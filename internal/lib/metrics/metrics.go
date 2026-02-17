package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	OrdersCreated   prometheus.Counter
	CacheHits       prometheus.Counter
	CacheMisses     prometheus.Counter
	requestDuration *prometheus.HistogramVec
	requestCount    *prometheus.CounterVec
}

func New() *Metrics {
	return &Metrics{
		OrdersCreated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "wb_orders_created_total",
			Help: "Total number of created orders",
		}),
		CacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "wb_cache_hits_total",
			Help: "Total number of cache hits",
		}),
		CacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "wb_cache_misses_total",
			Help: "Total number of cache misses",
		}),
		requestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		requestCount: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),
	}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		path := chi.RouteContext(r.Context()).RoutePattern()
		if path == "" {
			path = "unknown"
		}

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ww.Status())
		method := r.Method

		m.requestDuration.WithLabelValues(method, path).Observe(duration)
		m.requestCount.WithLabelValues(method, path, status).Inc()
	})
}

func NewTestMetrics() *Metrics {
	return &Metrics{
		OrdersCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_orders_created",
		}),
		CacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_cache_hits",
		}),
		CacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_cache_misses",
		}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "test_request_duration",
		}, []string{"method", "path"}),
		requestCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_request_count",
		}, []string{"method", "path", "status"}),
	}
}
