package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu             sync.Mutex
	httpTotal      map[string]int64
	httpDurationMs map[string]float64
	startedAt      time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{
		httpTotal:      make(map[string]int64),
		httpDurationMs: make(map[string]float64),
		startedAt:      time.Now().UTC(),
	}
}

func (m *Metrics) ObserveRequest(method, path string, statusCode int, duration time.Duration) {
	if m == nil {
		return
	}
	key := fmt.Sprintf("%s|%s|%d", strings.ToUpper(method), path, statusCode)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.httpTotal[key]++
	m.httpDurationMs[key] += float64(duration.Milliseconds())
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	m.mu.Lock()
	defer m.mu.Unlock()

	_, _ = fmt.Fprintf(w, "# HELP service_uptime_seconds Service uptime in seconds\n")
	_, _ = fmt.Fprintf(w, "# TYPE service_uptime_seconds gauge\n")
	_, _ = fmt.Fprintf(w, "service_uptime_seconds %.0f\n", time.Since(m.startedAt).Seconds())

	_, _ = fmt.Fprintf(w, "# HELP http_requests_total Total HTTP requests by method/path/status\n")
	_, _ = fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
	_, _ = fmt.Fprintf(w, "# HELP http_request_duration_ms_total Total HTTP duration in milliseconds by method/path/status\n")
	_, _ = fmt.Fprintf(w, "# TYPE http_request_duration_ms_total counter\n")

	keys := make([]string, 0, len(m.httpTotal))
	for key := range m.httpTotal {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		parts := strings.Split(key, "|")
		if len(parts) != 3 {
			continue
		}
		method := parts[0]
		path := parts[1]
		status := parts[2]
		count := m.httpTotal[key]
		dur := m.httpDurationMs[key]

		_, _ = fmt.Fprintf(w, "http_requests_total{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n", escapeLabel(method), escapeLabel(path), escapeLabel(status), count)
		_, _ = fmt.Fprintf(w, "http_request_duration_ms_total{method=\"%s\",path=\"%s\",status=\"%s\"} %.0f\n", escapeLabel(method), escapeLabel(path), escapeLabel(status), dur)
	}
}

func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\"", "\\\"")
	return v
}
