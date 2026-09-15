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

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/", s.cors(s.withMetrics(s.throttle(s.handleAPI))))
	mux.HandleFunc("GET /api/echo", s.cors(s.withMetrics(s.throttle(s.handleEcho))))

	// Dashboard data
	mux.HandleFunc("GET /api/metrics", s.cors(s.handleMetricsSnapshot))
	mux.HandleFunc("GET /ws", s.handleWebSocket)

	// Admin API
	mux.HandleFunc("GET /admin/whitelist", s.cors(s.handleGetWhitelist))
	mux.HandleFunc("POST /admin/whitelist", s.cors(s.handleAddWhitelist))
	mux.HandleFunc("DELETE /admin/whitelist", s.cors(s.handleRemoveWhitelist))
	mux.HandleFunc("GET /admin/blacklist", s.cors(s.handleGetBlacklist))
	mux.HandleFunc("POST /admin/blacklist", s.cors(s.handleAddBlacklist))
	mux.HandleFunc("DELETE /admin/blacklist", s.cors(s.handleRemoveBlacklist))
	mux.HandleFunc("GET /admin/log", s.cors(s.handleGetLog))
	mux.HandleFunc("GET /admin/stats", s.cors(s.handleStats))

	// Prometheus + health
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// --- Admin handlers ---

func (s *Server) handleGetWhitelist(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"whitelist": s.throttler.AccessList().GetWhitelist(),
	})
}

func (s *Server) handleAddWhitelist(w http.ResponseWriter, r *http.Request) {
	var body struct{ IP string `json:"ip"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IP == "" {
		http.Error(w, `{"error":"ip required"}`, http.StatusBadRequest)
		return
	}
	s.throttler.AccessList().AddWhitelist(body.IP)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "added", "ip": body.IP})
}

func (s *Server) handleRemoveWhitelist(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, `{"error":"ip required"}`, http.StatusBadRequest)
		return
	}
	s.throttler.AccessList().RemoveWhitelist(ip)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "removed", "ip": ip})
}

func (s *Server) handleGetBlacklist(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"blacklist": s.throttler.AccessList().GetBlacklist(),
	})
}

func (s *Server) handleAddBlacklist(w http.ResponseWriter, r *http.Request) {
	var body struct{ IP string `json:"ip"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IP == "" {
		http.Error(w, `{"error":"ip required"}`, http.StatusBadRequest)
		return
	}
	s.throttler.AccessList().AddBlacklist(body.IP)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "added", "ip": body.IP})
}

func (s *Server) handleRemoveBlacklist(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, `{"error":"ip required"}`, http.StatusBadRequest)
		return
	}
	s.throttler.AccessList().RemoveBlacklist(ip)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "removed", "ip": ip})
}

func (s *Server) handleGetLog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"log": s.throttler.RequestLog().Snapshot(100),
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.throttler.Snapshot())
}

// --- Dashboard handlers ---

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

	snap := s.throttler.Snapshot()
	data, _ := json.Marshal(map[string]any{"type": "metrics", "data": snap})
	_ = conn.WriteMessage(websocket.TextMessage, data)

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

// --- Throttle + metrics middleware ---

func (s *Server) throttle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !s.throttler.Allow(ip, r.URL.Path) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

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
