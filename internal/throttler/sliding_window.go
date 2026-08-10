package throttler

import (
	"sync"
	"time"
)

// this is a per-IP window that records request timestamps.
// It is not safe for concurrent use on its own; callers wrap it in a Partition.
type slidingWindow struct {
	mu         sync.Mutex
	timestamps []time.Time // ascending by construction (appends are monotonic)
	limit      int
	window     time.Duration
}

func newSlidingWindow(limit int, window time.Duration) *slidingWindow {
	return &slidingWindow{
		timestamps: make([]time.Time, 0, limit+1),
		limit:      limit,
		window:     window,
	}
}

// records a request at time `now` and returns true if it fits the budget.
func (sw *slidingWindow) Allow(now time.Time) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	cutoff := now.Add(-sw.window)

	// Drop timestamps outside the sliding window.
	i := 0

	for i < len(sw.timestamps) && sw.timestamps[i].Before(cutoff) {
		i++
	}

	if i > 0 {
		// Compact in-place to avoid unbounded slice growth.
		sw.timestamps = sw.timestamps[i:]
	}

	if len(sw.timestamps) >= sw.limit {
		return false
	}

	sw.timestamps = append(sw.timestamps, now)

	return true
}

// LastSeen returns the most recent timestamp, or zero if empty.
func (sw *slidingWindow) LastSeen() time.Time {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if len(sw.timestamps) == 0 {
		return time.Time{}
	}

	return sw.timestamps[len(sw.timestamps)-1]
}
