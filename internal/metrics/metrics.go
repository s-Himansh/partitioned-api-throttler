package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// counts allowed vs. denied requests, labeled by partition.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "api_throttler_requests_total",
		Help: "Total number of requests processed by the throttler.",
	}, []string{"status", "partition"})

	// the rolling denied/total ratio per partition.
	ThrottleRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "api_throttler_throttle_rate",
		Help: "Ratio of denied requests to total requests per partition (0..1).",
	}, []string{"partition"})

	// histogram of end-to-end handler latency.
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "api_throttler_request_duration_seconds",
		Help:    "End-to-end request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"endpoint"})

	// tracks the total number of unique IPs being throttled.
	ActiveIPs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "api_throttler_active_ips",
		Help: "Number of unique client IPs currently tracked.",
	})
)
