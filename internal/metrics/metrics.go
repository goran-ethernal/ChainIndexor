package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Database metrics
	dbQueries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"db", "operation"},
	)

	dbQueryTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chainindexor_db_query_duration_seconds",
			Help:    "Duration of database queries",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"db", "operation"},
	)

	dbErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_db_errors_total",
			Help: "Total number of database errors",
		},
		[]string{"db", "error_type"},
	)

	// Indexing metrics
	LastIndexedBlock = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chainindexor_last_indexed_block",
			Help: "The last block number successfully indexed",
		},
		[]string{"indexer"},
	)

	BlocksProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_blocks_processed_total",
			Help: "Total number of blocks processed",
		},
		[]string{"indexer"},
	)

	LogsIndexed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_logs_indexed_total",
			Help: "Total number of logs indexed",
		},
		[]string{"indexer"},
	)

	BlockProcessingTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chainindexor_block_processing_duration_seconds",
			Help:    "Time taken to process a batch of blocks",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"indexer"},
	)

	IndexingRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chainindexor_indexing_rate_blocks_per_second",
			Help: "Current indexing rate in blocks per second",
		},
		[]string{"indexer"},
	)

	// System metrics
	Uptime = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chainindexor_uptime_seconds",
			Help: "Application uptime in seconds",
		},
	)

	Errors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_errors_total",
			Help: "Total number of errors by component and severity",
		},
		[]string{"component", "severity"},
	)

	ComponentHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chainindexor_component_health",
			Help: "Component health status (1=healthy, 0=unhealthy)",
		},
		[]string{"component"},
	)

	Goroutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chainindexor_goroutines",
			Help: "Number of active goroutines",
		},
	)

	MemoryUsage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chainindexor_memory_usage_bytes",
			Help: "Memory usage statistics",
		},
		[]string{"type"},
	)

	startTime = time.Now()
)

func DBQueryInc(db string, operation string) {
	dbQueries.WithLabelValues(db, operation).Inc()
}

func DBQueryDuration(db string, operation string, duration time.Duration) {
	dbQueryTime.WithLabelValues(db, operation).Observe(duration.Seconds())
}

func DBErrorsInc(db string, errorType string) {
	dbErrors.WithLabelValues(db, errorType).Inc()
}

func BlockProcessingTimeLog(indexer string, duration time.Duration) {
	BlockProcessingTime.WithLabelValues(indexer).Observe(duration.Seconds())
}

func LastIndexedBlockInc(indexer string, blockNum uint64) {
	LastIndexedBlock.WithLabelValues(indexer).Set(float64(blockNum))
}

func BlocksProcessedInc(indexer string, count uint64) {
	BlocksProcessed.WithLabelValues(indexer).Add(float64(count))
}

func LogsIndexedInc(indexer string, count int) {
	LogsIndexed.WithLabelValues(indexer).Add(float64(count))
}

func IndexingRateLog(indexer string, rate float64) {
	IndexingRate.WithLabelValues(indexer).Set(rate)
}

func ComponentHealthSet(component string, healthy bool) {
	boolAsFloat := float64(1)
	if !healthy {
		boolAsFloat = 0
	}

	ComponentHealth.WithLabelValues(component).Set(boolAsFloat)
}

// UpdateSystemMetrics updates runtime system metrics.
// This should be called periodically (e.g., every 15 seconds).
func UpdateSystemMetrics() {
	// Update uptime
	Uptime.Set(time.Since(startTime).Seconds())

	// Update goroutine count
	Goroutines.Set(float64(runtime.NumGoroutine()))

	// Update memory statistics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	MemoryUsage.WithLabelValues("alloc").Set(float64(m.Alloc))
	MemoryUsage.WithLabelValues("total_alloc").Set(float64(m.TotalAlloc))
	MemoryUsage.WithLabelValues("sys").Set(float64(m.Sys))
	MemoryUsage.WithLabelValues("heap_inuse").Set(float64(m.HeapInuse))
}
