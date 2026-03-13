package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pulse_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	GRPCRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_grpc_requests_total",
			Help: "Total number of gRPC requests.",
		},
		[]string{"method", "status"},
	)

	CommandsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_commands_total",
			Help: "Total commands created.",
		},
		[]string{"type", "status"},
	)

	ConnectedAgents = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pulse_connected_agents",
			Help: "Number of currently connected agents.",
		},
	)

	ContainersTracked = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pulse_containers_tracked",
			Help: "Number of active (non-removed) containers.",
		},
	)
)

// Handler returns a Gin handler that serves Prometheus metrics.
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// Register registers all custom metrics with the default registry.
func Register() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		GRPCRequestsTotal,
		CommandsTotal,
		ConnectedAgents,
		ContainersTracked,
	)
	// Add Go runtime and process collectors (already registered by default,
	// but ensure they're present).
	prometheus.Unregister(collectors.NewGoCollector())
	prometheus.MustRegister(collectors.NewGoCollector())
}
