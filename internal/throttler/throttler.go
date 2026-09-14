package throttler

import (
	"hash/fnv"
	"strconv"
	"sync/atomic"
	"time"

	"partitioned-api-throttler/internal/metrics"
)

// this is a partitioned, sliding-window rate limiter.
type Throttler struct {
	partitions    []*Partition
	numPartitions uint32
	limit         int
	window        time.Duration
	totalAllowed  atomic.Int64
	totalDenied   atomic.Int64
}

// New constructs a Throttler with `numPartitions` shards.
func New(numPartitions, limit int, window time.Duration) *Throttler {
	if numPartitions <= 0 {
		numPartitions = 64
	}

	t := &Throttler{
		partitions:    make([]*Partition, numPartitions),
		numPartitions: uint32(numPartitions),
		limit:         limit,
		window:        window,
	}

	for i := 0; i < numPartitions; i++ {
		t.partitions[i] = newPartition()
	}

	return t
}

// returns the partition index for a given key using FNV-1a.
func (t *Throttler) PartitionIndex(key string) uint32 {
	h := fnv.New32a()

	_, _ = h.Write([]byte(key))

	return h.Sum32() % t.numPartitions
}

// Allow checks whether the IP may proceed.
func (t *Throttler) Allow(ip string) bool {
	idx := t.PartitionIndex(ip)

	p := t.partitions[idx]

	sw := p.windowFor(ip, t.limit, t.window)

	allowed := sw.Allow(time.Now())

	label := strconv.FormatUint(uint64(idx), 10)

	if allowed {
		metrics.RequestsTotal.WithLabelValues("allowed", label).Inc()
		t.totalAllowed.Add(1)
	} else {
		metrics.RequestsTotal.WithLabelValues("denied", label).Inc()
		t.totalDenied.Add(1)
	}

	return allowed
}

// launches a background goroutine that periodically evicts
// idle IP entries to bound memory usage.
func (t *Throttler) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)

		defer ticker.Stop()

		for range ticker.C {
			t.evictStale()

			t.publishGauges()
		}
	}()
}

func (t *Throttler) evictStale() {
	// Give IPs a grace period equal to the window before eviction.
	grace := t.window

	for _, p := range t.partitions {
		_ = p.evictIfStale(grace)
	}
}

func (t *Throttler) publishGauges() {
	var totalIPs int64
	for _, p := range t.partitions {
		n := p.snapshotLen()
		totalIPs += int64(n)
	}
	metrics.ActiveIPs.Set(float64(totalIPs))

	allowed := t.totalAllowed.Load()
	denied := t.totalDenied.Load()
	total := allowed + denied
	if total > 0 {
		metrics.ThrottleRate.WithLabelValues("global").Set(float64(denied) / float64(total))
	}
}
