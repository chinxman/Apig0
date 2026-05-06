package middleware

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const maxRecentEvents = 500

// RequestEvent is a single captured request/response.
type RequestEvent struct {
	ID        int64   `json:"id"`
	Method    string  `json:"method"`
	Path      string  `json:"path"`
	Status    int     `json:"status"`
	LatencyMs float64 `json:"latency_ms"`
	ClientIP  string  `json:"client_ip"`
	User      string  `json:"user,omitempty"`
	Service   string  `json:"service,omitempty"`
	Timestamp int64   `json:"ts"`
}

// ServiceStats is the aggregated view of one backend service.
type ServiceStats struct {
	Name       string  `json:"name"`
	Backend    string  `json:"backend"`
	Requests   int64   `json:"requests"`
	Errors     int64   `json:"errors"`
	AvgLatency float64 `json:"avg_latency_ms"`
	LastStatus int     `json:"last_status"`
	LastSeen   int64   `json:"last_seen"`
}

// Snapshot is sent to the dashboard on SSE connect and via /stats.
type Snapshot struct {
	UptimeSec int64                    `json:"uptime_sec"`
	Total     int64                    `json:"total"`
	Errors    int64                    `json:"errors"`
	Services  map[string]*ServiceStats `json:"services"`
	Recent    []*RequestEvent          `json:"recent"`
}

type serviceCounter struct {
	backend      string
	requests     int64
	errors       int64
	totalLatency float64
	lastStatus   int
	lastSeen     int64
}

// Monitor captures request metrics and streams them to dashboard clients.
type Monitor struct {
	startTime time.Time
	nextID    atomic.Int64

	mu       sync.RWMutex
	ring     [maxRecentEvents]*RequestEvent
	ringHead int
	ringLen  int
	total    int64
	totalErr int64
	services map[string]*serviceCounter

	subMu       sync.RWMutex
	subscribers map[chan []byte]struct{}
}

func NewMonitor() *Monitor {
	return &Monitor{
		startTime:   time.Now(),
		services:    make(map[string]*serviceCounter),
		subscribers: make(map[chan []byte]struct{}),
	}
}

// RegisterService pre-registers a service so it appears in the dashboard
// even before any traffic arrives.
func (m *Monitor) RegisterService(name, backend string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.services[name]; !ok {
		m.services[name] = &serviceCounter{backend: backend}
	} else {
		m.services[name].backend = backend
	}
}

// Middleware captures method, path, status, latency, IP, and user for every
// request, stores it in the ring buffer, and broadcasts to SSE subscribers.
func (m *Monitor) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Don't monitor admin/UI endpoints (avoid feedback loop)
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/admin/") || path == "/dashboard" || path == "/portal" || path == "/" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		// Determine service from first path segment
		service := ""
		trimmed := strings.TrimPrefix(path, "/")
		if parts := strings.SplitN(trimmed, "/", 2); len(parts) > 0 && parts[0] != "" {
			service = parts[0]
		}

		// Read user set by session middleware (falls back to totp_user for compat)
		userStr := ""
		if u, ok := c.Get("session_user"); ok {
			userStr, _ = u.(string)
		} else if u, ok := c.Get("totp_user"); ok {
			userStr, _ = u.(string)
		}

		evt := &RequestEvent{
			ID:        m.nextID.Add(1),
			Method:    c.Request.Method,
			Path:      path,
			Status:    c.Writer.Status(),
			LatencyMs: float64(latency.Microseconds()) / 1000.0,
			ClientIP:  c.ClientIP(),
			User:      userStr,
			Service:   service,
			Timestamp: time.Now().UnixMilli(),
		}

		m.record(evt)
		m.broadcast(evt)
	}
}

func (m *Monitor) record(evt *RequestEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ring[m.ringHead] = evt
	m.ringHead = (m.ringHead + 1) % maxRecentEvents
	if m.ringLen < maxRecentEvents {
		m.ringLen++
	}

	m.total++
	if evt.Status >= 400 {
		m.totalErr++
	}

	svc := evt.Service
	if svc == "" {
		svc = "_gateway"
	}
	sc, ok := m.services[svc]
	if !ok {
		sc = &serviceCounter{}
		m.services[svc] = sc
	}
	sc.requests++
	if evt.Status >= 400 {
		sc.errors++
	}
	sc.totalLatency += evt.LatencyMs
	sc.lastStatus = evt.Status
	sc.lastSeen = evt.Timestamp

	appendMonitorEvent(evt)
}

func (m *Monitor) broadcast(evt *RequestEvent) {
	data, _ := json.Marshal(evt)
	msg := fmt.Sprintf("event: request\ndata: %s\n\n", data)
	raw := []byte(msg)

	m.subMu.RLock()
	defer m.subMu.RUnlock()
	for ch := range m.subscribers {
		select {
		case ch <- raw:
		default:
			// subscriber too slow, drop event
		}
	}
}

// SSEHandler streams live request events to the dashboard via Server-Sent Events.
// On connect it sends a "snapshot" with current stats + recent events,
// then streams individual "request" events in real-time.
func (m *Monitor) SSEHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")

		// Send initial snapshot
		snap := m.snapshot()
		snapData, _ := json.Marshal(snap)
		fmt.Fprintf(c.Writer, "event: snapshot\ndata: %s\n\n", snapData)
		c.Writer.Flush()

		// Subscribe to live events
		ch := make(chan []byte, 256)
		m.subMu.Lock()
		m.subscribers[ch] = struct{}{}
		m.subMu.Unlock()

		defer func() {
			m.subMu.Lock()
			delete(m.subscribers, ch)
			m.subMu.Unlock()
		}()

		ctx := c.Request.Context()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case msg := <-ch:
				_, err := c.Writer.Write(msg)
				if err != nil {
					return
				}
				c.Writer.Flush()
			case <-heartbeat.C:
				_, err := fmt.Fprintf(c.Writer, ": heartbeat\n\n")
				if err != nil {
					return
				}
				c.Writer.Flush()
			case <-ctx.Done():
				return
			}
		}
	}
}

// StatsHandler returns a JSON snapshot of current metrics.
func (m *Monitor) StatsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, m.snapshot())
	}
}

func (m *Monitor) Snapshot() *Snapshot {
	return m.snapshot()
}

func (m *Monitor) snapshot() *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	services := make(map[string]*ServiceStats, len(m.services))
	for name, sc := range m.services {
		// Skip internal _gateway counter in the service cards
		if name == "_gateway" {
			continue
		}
		avg := 0.0
		if sc.requests > 0 {
			avg = sc.totalLatency / float64(sc.requests)
		}
		services[name] = &ServiceStats{
			Name:       name,
			Backend:    sc.backend,
			Requests:   sc.requests,
			Errors:     sc.errors,
			AvgLatency: avg,
			LastStatus: sc.lastStatus,
			LastSeen:   sc.lastSeen,
		}
	}

	return &Snapshot{
		UptimeSec: int64(time.Since(m.startTime).Seconds()),
		Total:     m.total,
		Errors:    m.totalErr,
		Services:  services,
		Recent:    m.recentEvents(),
	}
}

func (m *Monitor) recentEvents() []*RequestEvent {
	result := make([]*RequestEvent, 0, m.ringLen)
	start := m.ringHead - m.ringLen
	if start < 0 {
		start += maxRecentEvents
	}
	for i := 0; i < m.ringLen; i++ {
		idx := (start + i) % maxRecentEvents
		if m.ring[idx] != nil {
			result = append(result, m.ring[idx])
		}
	}
	return result
}

func MonitorEventLogPath() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_MONITOR_PATH")); path != "" {
		return path
	}
	return filepath.Join(os.TempDir(), "apig0-monitor.jsonl")
}

func appendMonitorEvent(evt *RequestEvent) {
	raw, err := json.Marshal(evt)
	if err != nil {
		return
	}
	path := MonitorEventLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil && filepath.Dir(path) != "." {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(raw, '\n'))
}
