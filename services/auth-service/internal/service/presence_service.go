package service

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"metarang/auth-service/internal/pubsub"
	"metarang/auth-service/internal/repository"
)

const (
	presenceOfflineAfter    = 2 * time.Minute
	presenceSweepInterval   = 15 * time.Second
	presenceSweepLookback   = 5 * time.Minute
	presenceSweepBatch      = 500
	presenceSweepMaxBatches = 20
	presenceWorkers         = 8
	presenceTouchQueue      = 4096
	presenceLockKey         = "presence:sweep:lock"
	presenceLockTTL         = 10 * time.Second
)

// PresenceStore is the persistence needed to track last_seen.
type PresenceStore interface {
	UpdateLastSeen(ctx context.Context, userID uint64) error
	ListUserIDsByLastSeen(ctx context.Context, after, until, cursorAt time.Time, cursorID uint64, limit int) ([]repository.LastSeenUser, error)
}

// PresenceService records authenticated activity and announces users who have gone quiet.
type PresenceService interface {
	Touch(userID uint64)
	Start(ctx context.Context)
	Sweep(ctx context.Context)
}

type presenceService struct {
	store     PresenceStore
	publisher pubsub.RedisPublisher
	redis     *redis.Client
	now       func() time.Time
	touches   chan uint64
}

// PresenceOption configures presence tracking.
type PresenceOption func(*presenceService)

// WithPresenceNow overrides the clock used by the offline sweeper.
func WithPresenceNow(now func() time.Time) PresenceOption {
	return func(s *presenceService) {
		if now != nil {
			s.now = now
		}
	}
}

// NewPresenceService tracks last_seen for authenticated users and sweeps offline users.
func NewPresenceService(store PresenceStore, publisher pubsub.RedisPublisher, redisClient *redis.Client, opts ...PresenceOption) PresenceService {
	s := &presenceService{
		store:     store,
		publisher: publisher,
		redis:     redisClient,
		now:       time.Now,
		touches:   make(chan uint64, presenceTouchQueue),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// Touch schedules a last_seen update and an online broadcast for this request.
// The request path does not wait on the database or Redis.
func (s *presenceService) Touch(userID uint64) {
	if s == nil || userID == 0 || s.store == nil {
		return
	}
	select {
	case s.touches <- userID:
	default:
		go s.handleTouch(userID)
	}
}

// Start processes activity updates and runs the offline sweeper until ctx is cancelled.
func (s *presenceService) Start(ctx context.Context) {
	if s == nil {
		return
	}
	go s.sweepLoop(ctx)
	for i := 1; i < presenceWorkers; i++ {
		go s.touchLoop(ctx)
	}
	s.touchLoop(ctx)
}

func (s *presenceService) touchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case userID := <-s.touches:
			s.handleTouch(userID)
		}
	}
}

func (s *presenceService) sweepLoop(ctx context.Context) {
	s.Sweep(ctx)
	ticker := time.NewTicker(presenceSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Sweep(ctx)
		}
	}
}

func (s *presenceService) handleTouch(userID uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.store.UpdateLastSeen(ctx, userID); err != nil {
		log.Printf("presence: update last_seen failed user=%d: %v", userID, err)
		return
	}
	if s.publisher == nil {
		return
	}
	if err := s.publisher.PublishUserStatusChanged(ctx, userID, true); err != nil {
		log.Printf("presence: publish online failed user=%d: %v", userID, err)
	}
}

// Sweep announces offline for users whose last_seen fell outside the 2-minute window.
// A Redis lock keeps a single replica working when several auth-service processes are running.
func (s *presenceService) Sweep(ctx context.Context) {
	if s == nil || s.store == nil {
		return
	}
	if s.redis != nil {
		ok, err := s.redis.SetNX(ctx, presenceLockKey, "1", presenceLockTTL).Result()
		if err != nil {
			log.Printf("presence sweep: lock failed: %v", err)
			return
		}
		if !ok {
			return
		}
	}

	now := s.now()
	until := now.Add(-presenceOfflineAfter)
	after := until.Add(-presenceSweepLookback)
	// cursorAt == after and cursorID == 0 selects the first page: last_seen > after.
	cursorAt := after
	var cursorID uint64

	for batch := 0; batch < presenceSweepMaxBatches; batch++ {
		if err := ctx.Err(); err != nil {
			return
		}
		rows, err := s.store.ListUserIDsByLastSeen(ctx, after, until, cursorAt, cursorID, presenceSweepBatch)
		if err != nil {
			log.Printf("presence sweep: list failed: %v", err)
			return
		}
		if len(rows) == 0 {
			return
		}
		for _, row := range rows {
			if s.publisher == nil {
				continue
			}
			if err := s.publisher.PublishUserStatusChanged(ctx, row.ID, false); err != nil {
				log.Printf("presence sweep: publish offline failed user=%d: %v", row.ID, err)
			}
		}
		last := rows[len(rows)-1]
		cursorAt = last.LastSeen
		cursorID = last.ID
		if len(rows) < presenceSweepBatch {
			return
		}
	}
}
