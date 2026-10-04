// Package resilience provides per-dependency timeouts, retries, and circuit breakers
// for notifications-service outbound calls.
package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	// DefaultFailureThreshold is the number of consecutive transport failures that open a breaker.
	DefaultFailureThreshold = 5
	// DefaultOpenCooldown is how long a breaker stays open before allowing one trial call.
	DefaultOpenCooldown = 15 * time.Second
)

// ErrCircuitOpen is returned when a breaker is open and the call is not attempted.
var ErrCircuitOpen = errors.New("circuit breaker open")

type breakerState int

const (
	stateClosed breakerState = iota
	stateOpen
	stateHalfOpen
)

// Breaker fails fast after repeated transport failures so callers do not queue on a dead dependency.
type Breaker struct {
	mu         sync.Mutex
	state      breakerState
	failures   int
	threshold  int
	cooldown   time.Duration
	openedAt   time.Time
	trialInUse bool
	now        func() time.Time
}

// NewBreaker returns a closed breaker. Non-positive threshold or cooldown use the defaults.
func NewBreaker(threshold int, cooldown time.Duration) *Breaker {
	if threshold < 1 {
		threshold = DefaultFailureThreshold
	}
	if cooldown <= 0 {
		cooldown = DefaultOpenCooldown
	}
	return &Breaker{
		threshold: threshold,
		cooldown:  cooldown,
		now:       time.Now,
	}
}

// Execute runs fn when the breaker allows it.
// isFailure reports whether err should count toward opening the breaker.
// Nil errors close the breaker. context.Canceled is ignored.
// Other errors that isFailure rejects do not change a closed breaker; in half-open they close it,
// because the dependency answered.
func (b *Breaker) Execute(fn func() error, isFailure func(error) bool) (err error) {
	if b == nil {
		return fn()
	}
	if allowErr := b.allow(); allowErr != nil {
		return allowErr
	}
	defer func() { b.record(err, isFailure) }()
	err = fn()
	return err
}

func (b *Breaker) allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case stateClosed:
		return nil
	case stateOpen:
		if b.now().Sub(b.openedAt) < b.cooldown {
			return ErrCircuitOpen
		}
		b.state = stateHalfOpen
		b.trialInUse = true
		return nil
	default:
		if b.trialInUse {
			return ErrCircuitOpen
		}
		b.trialInUse = true
		return nil
	}
}

func (b *Breaker) record(err error, isFailure func(error) bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.trialInUse = false

	if err == nil {
		b.failures = 0
		b.state = stateClosed
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	if isFailure != nil && isFailure(err) {
		b.failures++
		if b.state == stateHalfOpen || b.failures >= b.threshold {
			b.state = stateOpen
			b.openedAt = b.now()
		}
		return
	}
	if b.state == stateHalfOpen {
		b.failures = 0
		b.state = stateClosed
	}
}
