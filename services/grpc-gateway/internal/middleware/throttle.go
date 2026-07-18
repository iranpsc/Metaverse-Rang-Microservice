package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	authpkg "metarang/shared/pkg/auth"
)

// requestRecord tracks the number of requests and the window start time
type requestRecord struct {
	count       int
	windowStart time.Time
	mu          sync.Mutex
}

// throttleStore stores rate limit data per user
type throttleStore struct {
	records map[uint64]*requestRecord
	mu      sync.RWMutex
}

// newThrottleStore creates a new throttle store
func newThrottleStore() *throttleStore {
	store := &throttleStore{
		records: make(map[uint64]*requestRecord),
	}
	// Start cleanup goroutine to remove old entries
	go store.startCleanup()
	return store
}

// startCleanup periodically cleans up old entries to prevent memory leaks
func (ts *throttleStore) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ts.cleanupOldRecords()
	}
}

// cleanupOldRecords removes records older than 1 hour
func (ts *throttleStore) cleanupOldRecords() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	for userID, record := range ts.records {
		record.mu.Lock()
		if record.windowStart.Before(oneHourAgo) {
			delete(ts.records, userID)
		}
		record.mu.Unlock()
	}
}

// getOrCreateRecord gets or creates a request record for a user
func (ts *throttleStore) getOrCreateRecord(userID uint64) *requestRecord {
	ts.mu.RLock()
	record, exists := ts.records[userID]
	ts.mu.RUnlock()

	if exists {
		return record
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()
	// Double-check after acquiring write lock
	if record, exists := ts.records[userID]; exists {
		return record
	}
	record = &requestRecord{
		count:       0,
		windowStart: time.Now(),
	}
	ts.records[userID] = record
	return record
}

// checkAndIncrement checks if the user can make a request and increments the counter
// Returns true if allowed, false if rate limited
func (ts *throttleStore) checkAndIncrement(userID uint64, maxRequests int, period time.Duration) bool {
	record := ts.getOrCreateRecord(userID)

	record.mu.Lock()
	defer record.mu.Unlock()

	now := time.Now()
	// If the window has expired, reset it
	if now.Sub(record.windowStart) >= period {
		record.count = 1
		record.windowStart = now
		return true
	}

	// Check if limit is exceeded
	if record.count >= maxRequests {
		return false
	}

	// Increment and allow
	record.count++
	return true
}

// Global throttle store (one per application instance)
var globalThrottleStore = newThrottleStore()

// ThrottleMiddleware creates an HTTP middleware that rate limits requests per user.
// It requires authentication middleware to be applied before it (to get user ID from context).
// maxRequests: maximum number of requests allowed
// period: time window for the rate limit (e.g., time.Minute for per-minute limits)
func ThrottleMiddleware(maxRequests int, period time.Duration) func(http.Handler) http.Handler {
	if maxRequests <= 0 {
		maxRequests = 1
	}
	if period <= 0 {
		period = time.Minute
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context (set by auth middleware)
			userCtx, err := authpkg.GetUserFromContext(r.Context())
			if err != nil {
				// If no user context, we can't throttle per-user
				// Let the request pass (auth middleware should have handled this)
				next.ServeHTTP(w, r)
				return
			}

			// Check rate limit
			allowed := globalThrottleStore.checkAndIncrement(userCtx.UserID, maxRequests, period)
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", formatRetryAfter(period))
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}

			// Request allowed, proceed
			next.ServeHTTP(w, r)
		})
	}
}

// formatRetryAfter formats the period as seconds for Retry-After header
func formatRetryAfter(period time.Duration) string {
	seconds := int(period.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
