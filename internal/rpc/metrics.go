package rpc

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RPC metrics
	RPCRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_rpc_requests_total",
			Help: "Total number of RPC requests by method",
		},
		[]string{"method"},
	)

	RPCErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_rpc_errors_total",
			Help: "Total number of RPC errors by method and type",
		},
		[]string{"method", "error_type"},
	)

	RPCDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chainindexor_rpc_request_duration_seconds",
			Help:    "Duration of RPC requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
)

func RPCMethodInc(method string) {
	RPCRequests.WithLabelValues(method).Inc()
}

func RPCMethodDuration(method string, duration time.Duration) {
	RPCDuration.WithLabelValues(method).Observe(duration.Seconds())
}

func RPCMethodError(method, errorType string) {
	RPCErrors.WithLabelValues(method, errorType).Inc()
}
