package fetcher

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	finalizedBlock = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chainindexor_finalized_block",
			Help: "The current finalized block number from RPC",
		},
	)
)

func FinalizedBlockLogSet(blockNum uint64) {
	finalizedBlock.Set(float64(blockNum))
}
