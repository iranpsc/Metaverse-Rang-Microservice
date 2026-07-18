package repository

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// VariableRepository handles variable rate lookups with caching
type VariableRepository struct {
	db    *sql.DB
	cache map[string]cachedRate
	mu    sync.RWMutex
	ttl   time.Duration
}

type cachedRate struct {
	value     float64
	expiresAt time.Time
}

// NewVariableRepository creates a new variable repository with caching
func NewVariableRepository(db *sql.DB) *VariableRepository {
	return &VariableRepository{
		db:    db,
		cache: make(map[string]cachedRate),
		ttl:   5 * time.Minute, // 5 minute TTL as per plan
	}
}

// GetRate retrieves a variable rate with caching
// Implements getVariableRate logic used throughout marketplace_service.go
func (r *VariableRepository) GetRate(ctx context.Context, asset string) float64 {
	// Check cache first
	r.mu.RLock()
	if cached, ok := r.cache[asset]; ok {
		if time.Now().Before(cached.expiresAt) {
			r.mu.RUnlock()
			return cached.value
		}
	}
	r.mu.RUnlock()

	// Cache miss or expired - fetch from DB
	var rate float64
	query := "SELECT value FROM variables WHERE `key` = ?"
	err := r.db.QueryRowContext(ctx, query, asset).Scan(&rate)
	if err != nil {
		// Default to 1.0 if not found (matching existing behavior)
		rate = 1.0
	}

	// Update cache
	r.mu.Lock()
	r.cache[asset] = cachedRate{
		value:     rate,
		expiresAt: time.Now().Add(r.ttl),
	}
	r.mu.Unlock()

	return rate
}

// GetRateWithCache is an alias for GetRate (for consistency with plan)
func (r *VariableRepository) GetRateWithCache(ctx context.Context, asset string) float64 {
	return r.GetRate(ctx, asset)
}

// ClearCache clears the entire cache (useful for testing or forced refresh)
func (r *VariableRepository) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[string]cachedRate)
}

// InvalidateCache invalidates a specific asset's cache entry
func (r *VariableRepository) InvalidateCache(asset string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, asset)
}
