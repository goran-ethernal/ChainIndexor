package rpc

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RPC metrics
	rpcRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_rpc_requests_total",
			Help: "Total number of RPC requests by method",
		},
		[]string{"method"},
	)

	rpcErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_rpc_errors_total",
			Help: "Total number of RPC errors by method and type",
		},
		[]string{"method", "error_type"},
	)

	rpcDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chainindexor_rpc_request_duration_seconds",
			Help:    "Duration of RPC requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
)

func RPCMethodInc(method string) {
	rpcRequests.WithLabelValues(method).Inc()
}

func RPCMethodDuration(method string, duration time.Duration) {
	rpcDuration.WithLabelValues(method).Observe(duration.Seconds())
}

func RPCMethodError(method, errorType string) {
	rpcErrors.WithLabelValues(method, errorType).Inc()
}
