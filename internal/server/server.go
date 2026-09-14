package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"partitioned-api-throttler/internal/metrics"
	"partitioned-api-throttler/internal/throttler"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	throttler *throttler.Throttler
	wsClients sync.Map
}

func New(t *throttler.Throttler) *Server {
	return &Server{throttler: t}
}

// StartBroadcaster sends metrics to all connected WebSocket clients every 500ms.
func (s *Server) StartBroadcaster() {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			snap := s.throttler.Snapshot()
			data, _ := json.Marshal(map[string]any{
				"type": "metrics",
				"data": snap,
			})

			s.wsClients.Range(func(key, value any) bool {
				ws := key.(*websocket.Conn)
				if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
					ws.Close()
					s.wsClients.Delete(key)
				}
				return true
			})
		}
	}()
}

// returns the root HTTP handler with all routes wired up.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/", s.withMetrics(s.throttle(s.handleAPI)))
	mux.HandleFunc("GET /api/echo", s.withMetrics(s.throttle(s.handleEcho)))
	mux.HandleFunc("GET /api/metrics", s.cors(s.handleMetricsSnapshot))
	mux.HandleFunc("GET /ws", s.handleWebSocket)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return mux
}

func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleMetricsSnapshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.throttler.Snapshot())
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}

	s.wsClients.Store(conn, true)

	// Send initial snapshot
	snap := s.throttler.Snapshot()
	data, _ := json.Marshal(map[string]any{
		"type": "metrics",
		"data": snap,
	})
	_ = conn.WriteMessage(websocket.TextMessage, data)

	// Read pump (detect disconnect)
	go func() {
		defer func() {
			s.wsClients.Delete(conn)
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
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

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return strings.TrimSpace(xff[:i])
			}
		}
		return strings.TrimSpace(xff)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}

	return host
}

func partitionLabel(idx uint32) string { return strconv.FormatUint(uint64(idx), 10) }
