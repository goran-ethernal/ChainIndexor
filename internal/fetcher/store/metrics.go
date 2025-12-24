package store

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Retention metrics
	retentionBlocksPruned = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_retention_blocks_pruned_total",
			Help: "Total number of blocks pruned by retention policy",
		},
		[]string{"db"},
	)

	retentionLogsPruned = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_retention_logs_pruned_total",
			Help: "Total number of logs pruned by retention policy",
		},
		[]string{"db"},
	)
)

func RetentionBlocksPrunedInc(db string, count uint64) {
	retentionBlocksPruned.WithLabelValues(db).Add(float64(count))
}

func RetentionLogsPrunedInc(db string, count uint64) {
	retentionLogsPruned.WithLabelValues(db).Add(float64(count))
}
