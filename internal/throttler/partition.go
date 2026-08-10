package throttler

import (
	"sync"
	"time"
)

// this is a sharded map of IP -> sliding window.
// each partition owns its own mutex, so contention is bounded to
// IPs that hash to the same partition.
type Partition struct {
	mu  sync.RWMutex
	ips map[string]*slidingWindow
}

func newPartition() *Partition {
	return &Partition{ips: make(map[string]*slidingWindow)}
}

// returns the sliding window for an IP, creating it if missing.
// Uses double-checked locking to avoid taking the write lock on the hot path.
func (p *Partition) windowFor(ip string, limit int, window time.Duration) *slidingWindow {
	p.mu.RLock()
	sw, ok := p.ips[ip]
	p.mu.RUnlock()

	if ok {
		return sw
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if sw, ok := p.ips[ip]; ok {
		return sw
	}

	sw = newSlidingWindow(limit, window)

	p.ips[ip] = sw

	return sw
}

// returns the number of IPs tracked by this partition.
func (p *Partition) snapshotLen() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.ips)
}

// removes IPs whose last activity is older than `maxAge`.
func (p *Partition) evictIfStale(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)

	evicted := 0

	p.mu.Lock()
	defer p.mu.Unlock()

	for ip, sw := range p.ips {
		last := sw.LastSeen()

		if last.IsZero() || last.Before(cutoff) {
			delete(p.ips, ip)

			evicted++
		}
	}

	return evicted
}
