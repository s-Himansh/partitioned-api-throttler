package server

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"partitioned-api-throttler/internal/metrics"
	"partitioned-api-throttler/internal/throttler"
)

type Server struct {
	throttler *throttler.Throttler
}

func New(t *throttler.Throttler) *Server {
	return &Server{throttler: t}
}

// returns the root HTTP handler with all routes wired up.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/", s.withMetrics(s.throttle(s.handleAPI)))
	mux.HandleFunc("/api/echo", s.withMetrics(s.throttle(s.handleEcho)))
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return mux
}

// throttle is the rate-limiting middleware.
func (s *Server) throttle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		if !s.throttler.Allow(ip) {
			w.Header().Set("Retry-After", "1")

			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)

			return
		}

		next(w, r)
	}
}

// records end-to-end latency.
func (s *Server) withMetrics(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next(w, r)

		metrics.RequestDuration.WithLabelValues(r.URL.Path).Observe(time.Since(start).Seconds())
	}
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, _ = w.Write([]byte(`{"status":"ok","path":"` + r.URL.Path + `"}`))
}

func (s *Server) handleEcho(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, _ = fmt.Fprintf(w, `{"echo":"%s","time":"%s"}`, r.URL.RawQuery, time.Now().Format(time.RFC3339Nano))
}

// extracts the caller IP, honoring X-Forwarded-For when present.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}

		return xff
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	// Normalize IPv6 loopback
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}

	return host
}

// helper so callers can format partition labels without importing strconv elsewhere
func partitionLabel(idx uint32) string { return strconv.FormatUint(uint64(idx), 10) }
