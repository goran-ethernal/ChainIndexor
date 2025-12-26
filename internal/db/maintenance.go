package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/goran-ethernal/ChainIndexor/internal/common"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
)

type Maintenance interface {
	// Start begins background maintenance if enabled.
	Start(ctx context.Context) error
	// Stop stops background maintenance and waits for completion.
	Stop() error
	// AcquireOperationLock acquires a read lock for database operations.
	// Returns an unlock function that must be called when the operation completes.
	AcquireOperationLock() func()
	// GetMetrics returns current maintenance metrics.
	GetMetrics() MaintenanceMetrics
	// RunMaintenance performs database maintenance operations (for manual invocation).
	RunMaintenance(ctx context.Context) error
}

// NoOpMaintenance is a no-operation implementation of the Maintenance interface.
type NoOpMaintenance struct{}

// Start is a no-op.
func (m *NoOpMaintenance) Start(ctx context.Context) error {
	return nil
}

// Stop is a no-op.
func (m *NoOpMaintenance) Stop() error {
	return nil
}

// RunMaintenance is a no-op.
func (m *NoOpMaintenance) RunMaintenance(ctx context.Context) error {
	return nil
}

// AcquireOperationLock is a no-op that returns an empty unlock function.
func (m *NoOpMaintenance) AcquireOperationLock() func() {
	return func() {}
}

// GetMetrics returns empty maintenance metrics.
func (m *NoOpMaintenance) GetMetrics() MaintenanceMetrics {
	return MaintenanceMetrics{}
}

// MaintenanceCoordinator coordinates database maintenance operations across components.
// It uses a RWMutex where readers are normal operations and writer is maintenance.
// This ensures maintenance has exclusive access when needed while allowing concurrent operations.
type MaintenanceCoordinator struct {
	db     *sql.DB
	config config.MaintenanceConfig
	dbPath string
	log    *logger.Logger

	// RWMutex: readers = operations, writer = maintenance
	// Operations acquire read lock (shared, non-blocking with other operations)
	// Maintenance acquires write lock (exclusive, waits for all operations to complete)
	opLock sync.RWMutex

	// Background maintenance control
	maintenanceCtx    context.Context
	maintenanceCancel context.CancelFunc
	maintenanceWg     sync.WaitGroup

	// Metrics
	metricsLock         sync.Mutex
	lastMaintenanceTime time.Time
	maintenanceCount    uint64
	lastMaintenanceErr  error
}

// NewMaintenanceCoordinator creates a new maintenance coordinator.
func NewMaintenanceCoordinator(
	dbPath string,
	db *sql.DB,
	cfg *config.MaintenanceConfig,
	log *logger.Logger,
) Maintenance {
	if cfg == nil {
		return &NoOpMaintenance{}
	}

	return newMaintenanceCoordinator(dbPath, db, *cfg, log)
}

// newMaintenanceCoordinator is an internal constructor for MaintenanceCoordinator.
func newMaintenanceCoordinator(
	dbPath string,
	db *sql.DB,
	cfg config.MaintenanceConfig,
	log *logger.Logger,
) *MaintenanceCoordinator {
	return &MaintenanceCoordinator{
		db:     db,
		config: cfg,
		dbPath: dbPath,
		log:    log.WithComponent("db-maintenance"),
	}
}

// Start begins background maintenance if enabled.
func (m *MaintenanceCoordinator) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.log.Info("Background maintenance is disabled")
		return nil
	}

	m.maintenanceCtx, m.maintenanceCancel = context.WithCancel(ctx)

	// Run initial maintenance if configured
	if m.config.VacuumOnStartup {
		m.log.Info("Running startup maintenance")
		if err := m.RunMaintenance(m.maintenanceCtx); err != nil {
			m.log.Warnf("Startup maintenance failed: %v", err)
		}
	}

	// Start background worker
	m.maintenanceWg.Add(1)
	go m.maintenanceWorker(m.config.CheckInterval.Duration)

	m.log.Infof("Background maintenance started - interval: %v, checkpoint mode: %s",
		m.config.CheckInterval.Duration, m.config.WALCheckpointMode)

	return nil
}

// Stop stops background maintenance and waits for completion.
func (m *MaintenanceCoordinator) Stop() error {
	if m.maintenanceCancel == nil {
		return nil // Not started
	}

	m.log.Info("Stopping background maintenance...")
	m.maintenanceCancel()
	m.maintenanceWg.Wait()
	m.log.Info("Background maintenance stopped")

	return nil
}

// maintenanceWorker runs periodic maintenance in the background.
func (m *MaintenanceCoordinator) maintenanceWorker(checkInterval time.Duration) {
	defer m.maintenanceWg.Done()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.maintenanceCtx.Done():
			return

		case <-ticker.C:
			m.log.Debug("Running periodic maintenance")
			if err := m.RunMaintenance(m.maintenanceCtx); err != nil {
				m.log.Warnf("Periodic maintenance failed: %v", err)
			}
		}
	}
}

// RunMaintenance performs database maintenance operations.
// This acquires an exclusive lock, blocking all operations until complete.
func (m *MaintenanceCoordinator) RunMaintenance(ctx context.Context) error {
	m.log.Info("Starting database maintenance")
	start := time.Now().UTC()

	// Track maintenance run
	MaintenanceRunsInc()

	// Acquire write lock - blocks new operations and waits for ongoing ones to complete
	m.opLock.Lock()
	defer m.opLock.Unlock()

	// Check if context was cancelled while waiting for lock
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var maintenanceErr error

	initialDBSize, err := DBTotalSize(m.dbPath)
	if err != nil {
		m.log.Warnf("Failed to get initial DB size: %v", err)
	}

	// Step 1: WAL Checkpoint
	if err := m.walCheckpoint(); err != nil {
		m.log.Errorf("WAL checkpoint failed: %v", err)
		maintenanceErr = fmt.Errorf("WAL checkpoint failed: %w", err)
	}

	// Step 2: VACUUM (if not in WAL mode or if conditions allow)
	if err := m.vacuum(); err != nil {
		m.log.Warnf("VACUUM failed (may be expected in WAL mode): %v", err)
		if maintenanceErr == nil {
			maintenanceErr = fmt.Errorf("VACUUM failed: %w", err)
		}
	}

	finalDBSize, err := DBTotalSize(m.dbPath)
	if err != nil {
		m.log.Warnf("Failed to get final DB size: %v", err)
	}

	duration := time.Since(start)

	// Update internal metrics
	m.metricsLock.Lock()
	m.lastMaintenanceTime = time.Now().UTC()
	m.maintenanceCount++
	m.lastMaintenanceErr = maintenanceErr
	m.metricsLock.Unlock()

	// Update Prometheus metrics
	MaintenanceDurationLog(duration)
	MaintenanceLastRunLog()

	if maintenanceErr != nil {
		MaintenanceErrorInc()
		m.log.Warnf("Maintenance completed with errors in %v: %v", duration, maintenanceErr)
		return maintenanceErr
	}

	MaintenanceSuccessInc()
	m.log.Infof("Maintenance completed successfully in %v.", duration)

	if initialDBSize > finalDBSize {
		spaceReclaimed := uint64(initialDBSize - finalDBSize)
		MaintenanceSpaceReclaimedLog(spaceReclaimed)
		m.log.Infof("Maintenance cleaned: %d MB", common.BytesToMB(spaceReclaimed))
	}

	DBSizeLog(finalDBSize)

	return nil
}

// walCheckpoint performs a WAL checkpoint operation.
func (m *MaintenanceCoordinator) walCheckpoint() error {
	isWAL, err := m.isWALMode()
	if err != nil {
		return fmt.Errorf("failed to check journal mode: %w", err)
	}

	if !isWAL {
		m.log.Debug("Database not in WAL mode, skipping WAL checkpoint")
		return nil
	}

	checkpointSQL := fmt.Sprintf("PRAGMA wal_checkpoint(%s)", m.config.WALCheckpointMode)
	m.log.Debugf("Running: %s", checkpointSQL)

	var busyCount, logFrames, checkpointedFrames int
	err = m.db.QueryRow(checkpointSQL).Scan(&busyCount, &logFrames, &checkpointedFrames)
	if err != nil {
		return fmt.Errorf("failed to execute WAL checkpoint: %w", err)
	}

	m.log.Infof("WAL checkpoint complete - mode: %s, busy: %d, log_frames: %d, checkpointed: %d",
		m.config.WALCheckpointMode, busyCount, logFrames, checkpointedFrames)

	// Track checkpoint
	WALCheckpointInc(strings.ToLower(m.config.WALCheckpointMode))

	if busyCount > 0 {
		m.log.Warnf("WAL checkpoint encountered %d busy pages (some pages not checkpointed)", busyCount)
	}

	return nil
}

// vacuum performs a VACUUM operation to reclaim space.
// VACUUM works in both WAL and non-WAL modes, but serves different purposes:
// - WAL mode: Reclaims fragmented space within pages after deletes/updates
// - Non-WAL: Also compacts the database file
// Note: VACUUM requires exclusive access (which we have via write lock)
func (m *MaintenanceCoordinator) vacuum() error {
	m.log.Debug("Running VACUUM")

	_, err := m.db.Exec("VACUUM")
	if err != nil {
		if strings.Contains(err.Error(), "database is locked") {
			// This can happen if there are other active connections with transactions
			return fmt.Errorf("cannot vacuum: database is locked (retry later)")
		}
		return fmt.Errorf("vacuum failed: %w", err)
	}

	// Track vacuum
	VacuumRunsInc()
	m.log.Info("VACUUM completed successfully")
	return nil
}

// isWALMode checks if the database is in WAL journal mode.
func (m *MaintenanceCoordinator) isWALMode() (bool, error) {
	var mode string
	if err := m.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		return false, err
	}
	return strings.EqualFold(mode, "wal"), nil
}

// AcquireOperationLock acquires a read lock for database operations.
// Returns an unlock function that must be called when the operation completes.
// This allows normal operations to proceed concurrently while ensuring
// maintenance operations have exclusive access when needed.
func (m *MaintenanceCoordinator) AcquireOperationLock() func() {
	m.opLock.RLock()
	return m.opLock.RUnlock
}

// GetMetrics returns current maintenance metrics.
func (m *MaintenanceCoordinator) GetMetrics() MaintenanceMetrics {
	m.metricsLock.Lock()
	defer m.metricsLock.Unlock()

	return MaintenanceMetrics{
		LastMaintenanceTime:  m.lastMaintenanceTime,
		MaintenanceCount:     m.maintenanceCount,
		LastMaintenanceError: m.lastMaintenanceErr,
	}
}

// MaintenanceMetrics provides visibility into maintenance operations.
type MaintenanceMetrics struct {
	LastMaintenanceTime  time.Time
	MaintenanceCount     uint64
	LastMaintenanceError error
}
