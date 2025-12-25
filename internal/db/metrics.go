package db

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Maintenance metrics
	maintenanceRuns = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "chainindexor_maintenance_runs_total",
			Help: "Total number of maintenance operations",
		},
	)

	maintenanceOutcomes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_maintenance_outcomes_total",
			Help: "Total number of maintenance operations by outcome",
		},
		[]string{"status"},
	)

	maintenanceDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "chainindexor_maintenance_duration_seconds",
			Help:    "Duration of maintenance operations",
			Buckets: prometheus.DefBuckets,
		},
	)

	maintenanceLastRun = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chainindexor_maintenance_last_run_timestamp",
			Help: "Unix timestamp of last maintenance run",
		},
	)

	maintenanceSpaceReclaimed = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chainindexor_maintenance_space_reclaimed_bytes",
			Help: "Bytes reclaimed by last maintenance operation",
		},
	)

	walCheckpoints = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chainindexor_wal_checkpoint_total",
			Help: "Total number of WAL checkpoint operations",
		},
		[]string{"mode"},
	)

	vacuumRuns = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "chainindexor_vacuum_total",
			Help: "Total number of VACUUM operations",
		},
	)

	dbSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chainindexor_db_size_bytes",
			Help: "Database file size in bytes",
		},
		[]string{"type"},
	)
)

func MaintenanceRunsInc() {
	maintenanceRuns.Inc()
}

func MaintenanceDurationLog(duration time.Duration) {
	maintenanceDuration.Observe(duration.Seconds())
}

func MaintenanceLastRunLog() {
	maintenanceLastRun.Set(float64(time.Now().UTC().Unix()))
}

func MaintenanceErrorInc() {
	maintenanceOutcomes.WithLabelValues("error").Inc()
}

func MaintenanceSuccessInc() {
	maintenanceOutcomes.WithLabelValues("success").Inc()
}

func MaintenanceSpaceReclaimedLog(bytesReclaimed uint64) {
	maintenanceSpaceReclaimed.Set(float64(bytesReclaimed))
}

func WALCheckpointInc(mode string) {
	walCheckpoints.WithLabelValues(mode).Inc()
}

func VacuumRunsInc() {
	vacuumRuns.Inc()
}

func DBSizeLog(sizeBytes int64) {
	dbSize.WithLabelValues("total").Set(float64(sizeBytes))
}
