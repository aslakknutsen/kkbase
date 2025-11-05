package agent

import (
	"sync"
	"time"

	"github.com/kagenti/kkbase/pkg/agenttypes"
)

// Deduplicator prevents duplicate events from being processed
type Deduplicator struct {
	seen map[string]time.Time
	ttl  time.Duration
	mu   sync.RWMutex
}

// NewDeduplicator creates a new deduplicator with the given TTL
func NewDeduplicator(ttl time.Duration) *Deduplicator {
	d := &Deduplicator{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}

	// Start cleanup goroutine
	go d.cleanupLoop()

	return d
}

// IsDuplicate checks if an event has been seen recently
func (d *Deduplicator) IsDuplicate(event agenttypes.Event) bool {
	d.mu.RLock()
	lastSeen, exists := d.seen[event.ID]
	d.mu.RUnlock()

	if !exists {
		// New event, mark as seen
		d.mu.Lock()
		d.seen[event.ID] = time.Now()
		d.mu.Unlock()
		return false
	}

	// Check if TTL has expired
	if time.Since(lastSeen) > d.ttl {
		// TTL expired, consider it new
		d.mu.Lock()
		d.seen[event.ID] = time.Now()
		d.mu.Unlock()
		return false
	}

	return true
}

// cleanupLoop periodically removes expired entries
func (d *Deduplicator) cleanupLoop() {
	ticker := time.NewTicker(d.ttl)
	defer ticker.Stop()

	for range ticker.C {
		d.cleanup()
	}
}

// cleanup removes expired entries
func (d *Deduplicator) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for id, lastSeen := range d.seen {
		if now.Sub(lastSeen) > d.ttl {
			delete(d.seen, id)
		}
	}
}
