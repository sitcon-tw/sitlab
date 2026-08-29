package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	requests          *prometheus.CounterVec
	duration          *prometheus.HistogramVec
	webhooks          *prometheus.CounterVec
	webhookProcessing *prometheus.HistogramVec
	sseConnections    prometheus.Gauge
	sseEvents         prometheus.Counter
	webhookQueue      *prometheus.GaugeVec
	webhookOldest     prometheus.Gauge
	registry          *prometheus.Registry
}

func NewMetrics() *Metrics {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_server_requests_total", Help: "Completed HTTP requests."}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_server_request_duration_seconds", Help: "HTTP request duration.", Buckets: prometheus.DefBuckets}, []string{"method", "route", "status"})
	webhooks := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gitlab_webhook_deliveries_total", Help: "GitLab webhook deliveries by scope and result."}, []string{"scope", "result"})
	webhookProcessing := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "gitlab_webhook_processing_duration_seconds", Help: "Durable GitLab webhook processing duration.", Buckets: prometheus.DefBuckets}, []string{"kind", "result"})
	sseConnections := prometheus.NewGauge(prometheus.GaugeOpts{Name: "bootstrap_sse_connections", Help: "Active authenticated bootstrap event streams."})
	sseEvents := prometheus.NewCounter(prometheus.CounterOpts{Name: "bootstrap_sse_events_total", Help: "Bootstrap revision events written to streams."})
	webhookQueue := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "gitlab_webhook_queue_deliveries", Help: "Current durable GitLab webhook deliveries by state."}, []string{"state"})
	webhookOldest := prometheus.NewGauge(prometheus.GaugeOpts{Name: "gitlab_webhook_oldest_pending_age_seconds", Help: "Age of the oldest pending or processing GitLab webhook."})
	registry := prometheus.NewRegistry()
	registry.MustRegister(requests, duration, webhooks, webhookProcessing, sseConnections, sseEvents, webhookQueue, webhookOldest)
	return &Metrics{
		requests: requests, duration: duration, webhooks: webhooks,
		webhookProcessing: webhookProcessing, sseConnections: sseConnections,
		sseEvents: sseEvents, webhookQueue: webhookQueue, webhookOldest: webhookOldest, registry: registry,
	}
}

func (m *Metrics) WebhookDelivery(scope, result string) {
	m.webhooks.WithLabelValues(scope, result).Inc()
}

func (m *Metrics) WebhookProcessed(kind, result string, duration time.Duration) {
	m.webhookProcessing.WithLabelValues(kind, result).Observe(duration.Seconds())
}

func (m *Metrics) SSEConnected()    { m.sseConnections.Inc() }
func (m *Metrics) SSEDisconnected() { m.sseConnections.Dec() }
func (m *Metrics) SSEEvent()        { m.sseEvents.Inc() }

func (m *Metrics) SetWebhookQueue(pending, dead int64, oldestSeconds float64) {
	m.webhookQueue.WithLabelValues("pending").Set(float64(pending))
	m.webhookQueue.WithLabelValues("dead").Set(float64(dead))
	m.webhookOldest.Set(oldestSeconds)
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		capture := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(capture, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(capture.status)
		m.requests.WithLabelValues(r.Method, route, status).Inc()
		m.duration.WithLabelValues(r.Method, route, status).Observe(time.Since(start).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
