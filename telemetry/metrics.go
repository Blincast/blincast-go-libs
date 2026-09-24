package telemetry

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

// Returns an instance of *http.Server on port 2112 to expose metrics for Prometheus scraping.
// It must be called in a separate goroutine to avoid blocking the main application execution flow.
func NewMetricServer() *http.Server {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		httpRequestsTotal,
		httpRequestDuration,
	)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	server := &http.Server{
		Addr:         ":2112",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	return server
}

// Creates a middleware to configure collecting total requests and duration metrics.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		statusStr := fmt.Sprintf("%d", rw.statusCode)

		counter := httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr)
		observer := httpRequestDuration.WithLabelValues(r.Method, r.URL.Path)

		if traceID := GetTraceID(r.Context()); traceID != "" {
			if eo, ok := observer.(prometheus.ExemplarObserver); ok {
				eo.ObserveWithExemplar(duration, prometheus.Labels{"trace_id": traceID})
			} else {
				observer.Observe(duration)
			}
		} else {
			observer.Observe(duration)
		}
		counter.Inc()
	})
}

// Creates a middleware to configure collecting total requests and duration metrics with Gin.
func GinMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		ctx := c.Request.Context()

		counter := httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status)
		observer := httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath())

		if traceID := GetTraceID(ctx); traceID != "" {
			if eo, ok := observer.(prometheus.ExemplarObserver); ok {
				eo.ObserveWithExemplar(duration, prometheus.Labels{"trace_id": traceID})
			} else {
				observer.Observe(duration)
			}
		} else {
			observer.Observe(duration)
		}

		counter.Inc()
	}
}
