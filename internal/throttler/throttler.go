package throttler

import (
	"hash/fnv"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"partitioned-api-throttler/internal/metrics"
)

type EndpointConfig struct {
	Limit int           `json:"limit"`
	Window time.Duration `json:"window"`
}

type Throttler struct {
	partitions    []*Partition
	numPartitions uint32
	limit         int
	window        time.Duration
	totalAllowed  atomic.Int64
	totalDenied   atomic.Int64
	accessList    *AccessList
	requestLog    *RequestLog
	log           sync.Map // endpoint -> *atomic.Int64 (allowed)
	deniedLog     sync.Map // endpoint -> *atomic.Int64 (denied)
	adaptive      *AdaptiveConfig
}

type AdaptiveConfig struct {
	Enabled        bool
	TargetLatencyMs float64
	MinLimit       int
	MaxLimit       int
}

func New(numPartitions, limit int, window time.Duration) *Throttler {
	if numPartitions <= 0 {
		numPartitions = 64
	}

	t := &Throttler{
		partitions:    make([]*Partition, numPartitions),
		numPartitions: uint32(numPartitions),
		limit:         limit,
		window:        window,
		accessList:    NewAccessList(),
		requestLog:    NewRequestLog(500),
		adaptive: &AdaptiveConfig{
			Enabled:         false,
			TargetLatencyMs: 100,
			MinLimit:        1,
			MaxLimit:        1000,
		},
	}

	for i := 0; i < numPartitions; i++ {
		t.partitions[i] = newPartition()
	}

	return t
}

func (t *Throttler) AccessList() *AccessList {
	return t.accessList
}

func (t *Throttler) RequestLog() *RequestLog {
	return t.requestLog
}

func (t *Throttler) SetAdaptive(cfg AdaptiveConfig) {
	t.adaptive = &cfg
}

func (t *Throttler) GetAdaptive() AdaptiveConfig {
	return *t.adaptive
}

func (t *Throttler) PartitionIndex(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32() % t.numPartitions
}

func (t *Throttler) Allow(ip, endpoint string) bool {
	if t.accessList.IsBlacklisted(ip) {
		t.totalDenied.Add(1)
		t.requestLog.Add(RequestEntry{
			Time: time.Now(), IP: ip, Endpoint: endpoint,
			Status: "denied", StatusCode: 403,
		})
		return false
	}

	if t.accessList.IsWhitelisted(ip) {
		t.totalAllowed.Add(1)
		t.requestLog.Add(RequestEntry{
			Time: time.Now(), IP: ip, Endpoint: endpoint,
			Status: "allowed", StatusCode: 200,
		})
		return true
	}

	limit := t.limit
	if t.adaptive.Enabled {
		limit = t.computeAdaptiveLimit()
	}

	idx := t.PartitionIndex(ip)
	p := t.partitions[idx]
	sw := p.windowFor(ip, limit, t.window)
	allowed := sw.Allow(time.Now())

	label := strconv.FormatUint(uint64(idx), 10)
	if allowed {
		metrics.RequestsTotal.WithLabelValues("allowed", label).Inc()
		t.totalAllowed.Add(1)
	} else {
		metrics.RequestsTotal.WithLabelValues("denied", label).Inc()
		t.totalDenied.Add(1)
	}

	status := "denied"
	code := 429
	if allowed {
		status = "allowed"
		code = 200
	}
	t.requestLog.Add(RequestEntry{
		Time: time.Now(), IP: ip, Endpoint: endpoint,
		Status: status, StatusCode: code,
	})

	return allowed
}

func (t *Throttler) computeAdaptiveLimit() int {
	avg := t.requestLog.avgLatency()
	if avg <= 0 {
		return t.limit
	}

	ratio := t.adaptive.TargetLatencyMs / avg
	newLimit := int(float64(t.limit) * ratio)

	if newLimit < t.adaptive.MinLimit {
		newLimit = t.adaptive.MinLimit
	}
	if newLimit > t.adaptive.MaxLimit {
		newLimit = t.adaptive.MaxLimit
	}
	return newLimit
}

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
	grace := t.window
	for _, p := range t.partitions {
		_ = p.evictIfStale(grace)
	}
}

func (t *Throttler) Snapshot() map[string]any {
	var totalIPs int64
	partitions := make([]int, len(t.partitions))
	for i, p := range t.partitions {
		n := p.snapshotLen()
		partitions[i] = n
		totalIPs += int64(n)
	}

	allowed := t.totalAllowed.Load()
	denied := t.totalDenied.Load()
	total := allowed + denied
	throttleRate := 0.0
	if total > 0 {
		throttleRate = float64(denied) / float64(total)
	}

	var avgLatency float64
	if t.requestLog != nil {
		avgLatency = t.requestLog.avgLatency()
	}

	return map[string]any{
		"active_ips":     totalIPs,
		"total_allowed":  allowed,
		"total_denied":   denied,
		"total_requests": total,
		"throttle_rate":  throttleRate,
		"partitions":     partitions,
		"num_partitions": len(t.partitions),
		"limit":          t.limit,
		"window_seconds": int(t.window.Seconds()),
		"avg_latency_ms": avgLatency,
		"adaptive":       t.adaptive.Enabled,
		"whitelist":      t.accessList.GetWhitelist(),
		"blacklist":      t.accessList.GetBlacklist(),
		"log":            t.requestLog.Snapshot(50),
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
