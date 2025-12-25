package reorg

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	reorgsDetected = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "chainindexor_reorgs_detected_total",
			Help: "Total number of blockchain reorganizations detected",
		},
	)

	reorgDepth = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "chainindexor_reorg_depth_blocks",
			Help:    "Depth of blockchain reorganizations in blocks",
			Buckets: []float64{1, 2, 5, 10, 20, 50, 100},
		},
	)

	reorgLastDetected = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chainindexor_reorg_last_detected_timestamp",
			Help: "Unix timestamp of last reorg detection",
		},
	)

	reorgFromBlock = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "chainindexor_reorg_from_block",
			Help:    "Block numbers where reorgs started",
			Buckets: []float64{0, 1000000, 3000000, 5000000, 7000000, 9000000, 10000000},
		},
	)
)

func ReorgDetectedLog(depth, fromBlock uint64) {
	reorgsDetected.Inc()
	reorgDepth.Observe(float64(depth))
	reorgLastDetected.Set(float64(time.Now().UTC().Unix()))
	reorgFromBlock.Observe(float64(fromBlock))
}
