package throttler

import (
	"sync"
	"time"
)

type RequestEntry struct {
	Time       time.Time `json:"time"`
	IP         string    `json:"ip"`
	Endpoint   string    `json:"endpoint"`
	Status     string    `json:"status"` // "allowed" or "denied"
	LatencyMs  float64   `json:"latency_ms"`
	StatusCode int       `json:"status_code"`
}

type RequestLog struct {
	mu      sync.RWMutex
	entries []RequestEntry
	maxSize int
}

func NewRequestLog(maxSize int) *RequestLog {
	if maxSize <= 0 {
		maxSize = 200
	}
	return &RequestLog{
		entries: make([]RequestEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

func (rl *RequestLog) Add(entry RequestEntry) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if len(rl.entries) >= rl.maxSize {
		rl.entries = rl.entries[1:]
	}
	rl.entries = append(rl.entries, entry)
}

func (rl *RequestLog) Snapshot(n int) []RequestEntry {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	total := len(rl.entries)
	if n <= 0 || n > total {
		n = total
	}

	out := make([]RequestEntry, n)
	copy(out, rl.entries[total-n:])
	return out
}

func (rl *RequestLog) Len() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.entries)
}

func (rl *RequestLog) avgLatency() float64 {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if len(rl.entries) == 0 {
		return 0
	}

	var sum float64
	for _, e := range rl.entries {
		sum += e.LatencyMs
	}
	return sum / float64(len(rl.entries))
}
