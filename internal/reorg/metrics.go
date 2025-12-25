package reorg

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ReorgsDetected = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "chainindexor_reorgs_detected_total",
			Help: "Total number of blockchain reorganizations detected",
		},
	)

	ReorgDepth = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "chainindexor_reorg_depth_blocks",
			Help:    "Depth of blockchain reorganizations in blocks",
			Buckets: []float64{1, 2, 5, 10, 20, 50, 100},
		},
	)

	ReorgLastDetected = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chainindexor_reorg_last_detected_timestamp",
			Help: "Unix timestamp of last reorg detection",
		},
	)

	ReorgFromBlock = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name: "chainindexor_reorg_from_block",
			Help: "Block numbers where reorgs started",
		},
	)
)

func ReorgDetectedLog(depth, fromBlock uint64) {
	ReorgsDetected.Inc()
	ReorgDepth.Observe(float64(depth))
	ReorgLastDetected.Set(float64(time.Now().UTC().Unix()))
	ReorgFromBlock.Observe(float64(fromBlock))
}
