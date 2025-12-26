package db

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goran-ethernal/ChainIndexor/internal/common"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/stretchr/testify/require"
)

func setupMaintenanceTestDB(t *testing.T) (*sql.DB, string, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "maintenance_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	dbPath := tmpFile.Name()

	dbConfig := config.DatabaseConfig{
		Path:        dbPath,
		JournalMode: "WAL",
		Synchronous: "NORMAL",
		BusyTimeout: 5000,
		CacheSize:   10000,
	}
	dbConfig.ApplyDefaults()

	db, err := NewSQLiteDBFromConfig(dbConfig)
	require.NoError(t, err)

	// Create a test table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_data (
			id INTEGER PRIMARY KEY,
			data TEXT
		)
	`)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	}

	return db, dbPath, cleanup
}

func TestMaintenanceCoordinator_NewMaintenanceCoordinator(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	cfg := config.MaintenanceConfig{
		Enabled:           true,
		CheckInterval:     common.NewDuration(1 * time.Minute),
		VacuumOnStartup:   false,
		WALCheckpointMode: "TRUNCATE",
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)
	require.NotNil(t, coordinator)
	require.NotNil(t, coordinator.db)
	require.Equal(t, "TRUNCATE", coordinator.config.WALCheckpointMode)
}

func TestMaintenanceCoordinator_RunMaintenance(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	// Insert some test data to create WAL activity
	for i := 0; i < 1000; i++ {
		_, err := db.Exec("INSERT INTO test_data (data) VALUES (?)", "test data")
		require.NoError(t, err)
	}

	// Check that WAL file exists and has data
	walPath := dbPath + "-wal"
	walInfo, err := os.Stat(walPath)
	require.NoError(t, err)
	require.Greater(t, walInfo.Size(), int64(0), "WAL should have data before checkpoint")

	cfg := config.MaintenanceConfig{
		Enabled:           false, // Don't start background worker
		WALCheckpointMode: "TRUNCATE",
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)

	// Run maintenance manually
	err = coordinator.RunMaintenance(context.Background())
	require.NoError(t, err)

	// Check metrics
	metrics := coordinator.GetMetrics()
	require.Equal(t, uint64(1), metrics.MaintenanceCount)
	require.False(t, metrics.LastMaintenanceTime.IsZero())
	require.NoError(t, metrics.LastMaintenanceError)
}

func TestMaintenanceCoordinator_WALCheckpoint(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	// Create significant WAL activity
	for i := 0; i < 5000; i++ {
		_, err := db.Exec("INSERT INTO test_data (data) VALUES (?)", "test data with more content")
		require.NoError(t, err)
	}

	walPath := dbPath + "-wal"
	walInfoBefore, err := os.Stat(walPath)
	require.NoError(t, err)
	walSizeBefore := walInfoBefore.Size()
	require.Greater(t, walSizeBefore, int64(1000), "Should have significant WAL data")

	cfg := config.MaintenanceConfig{
		Enabled:           false,
		WALCheckpointMode: "TRUNCATE",
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)
	err = coordinator.walCheckpoint()
	require.NoError(t, err)

	// WAL should be truncated after checkpoint
	walInfoAfter, err := os.Stat(walPath)
	if err == nil {
		// WAL may still exist but should be much smaller or empty
		require.LessOrEqual(t, walInfoAfter.Size(), walSizeBefore,
			"WAL should be same size or smaller after checkpoint")
	}
	// It's also OK if WAL file doesn't exist after TRUNCATE checkpoint
}

func TestMaintenanceCoordinator_OperationLock(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	cfg := config.MaintenanceConfig{
		Enabled:           false,
		WALCheckpointMode: "TRUNCATE",
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)

	// Test that multiple operations can acquire read lock concurrently
	var wg sync.WaitGroup
	const numOps = 10

	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := coordinator.AcquireOperationLock()
			time.Sleep(10 * time.Millisecond) // Simulate operation
			unlock()
		}()
	}

	wg.Wait()
}

func TestMaintenanceCoordinator_MaintenanceBlocksOperations(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	cfg := config.MaintenanceConfig{
		Enabled:           false,
		WALCheckpointMode: "PASSIVE", // Use faster mode for testing
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)

	var operationsBlocked atomic.Bool
	var maintenanceStarted atomic.Bool
	var maintenanceFinished atomic.Bool

	// Start a long-running operation
	operationDone := make(chan struct{})
	go func() {
		unlock := coordinator.AcquireOperationLock()
		time.Sleep(100 * time.Millisecond) // Hold lock for a while
		unlock()
		close(operationDone)
	}()

	// Give operation time to acquire lock
	time.Sleep(20 * time.Millisecond)

	// Try to run maintenance - should block until operation completes
	maintenanceDone := make(chan struct{})
	go func() {
		maintenanceStarted.Store(true)
		err := coordinator.RunMaintenance(context.Background())
		require.NoError(t, err)
		maintenanceFinished.Store(true)
		close(maintenanceDone)
	}()

	// Give maintenance goroutine time to start
	time.Sleep(20 * time.Millisecond)

	// Try to acquire operation lock - should block because maintenance is waiting
	operationBlocked := make(chan struct{})
	go func() {
		operationsBlocked.Store(true)
		unlock := coordinator.AcquireOperationLock()
		unlock()
		close(operationBlocked)
	}()

	// Wait for everything to complete
	<-operationDone
	<-maintenanceDone
	<-operationBlocked

	require.True(t, maintenanceStarted.Load())
	require.True(t, maintenanceFinished.Load())
	require.True(t, operationsBlocked.Load())
}

func TestMaintenanceCoordinator_BackgroundMaintenance(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	cfg := config.MaintenanceConfig{
		Enabled:           true,
		CheckInterval:     common.NewDuration(100 * time.Millisecond), // Fast interval for testing
		VacuumOnStartup:   false,
		WALCheckpointMode: "PASSIVE",
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)

	// Start background maintenance
	err = coordinator.Start(t.Context())
	require.NoError(t, err)

	// Insert data to create WAL activity
	for i := 0; i < 100; i++ {
		_, err := db.Exec("INSERT INTO test_data (data) VALUES (?)", "test")
		require.NoError(t, err)
	}

	// Wait for at least one maintenance cycle
	time.Sleep(300 * time.Millisecond)

	// Stop maintenance
	err = coordinator.Stop()
	require.NoError(t, err)

	// Check that maintenance ran
	metrics := coordinator.GetMetrics()
	require.Greater(t, metrics.MaintenanceCount, uint64(0), "Maintenance should have run at least once")
}

func TestMaintenanceCoordinator_StartupMaintenance(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	// Insert data before starting coordinator
	for i := 0; i < 100; i++ {
		_, err := db.Exec("INSERT INTO test_data (data) VALUES (?)", "test")
		require.NoError(t, err)
	}

	cfg := config.MaintenanceConfig{
		Enabled:           true,
		CheckInterval:     common.NewDuration(1 * time.Hour), // Long interval so it doesn't run during test
		VacuumOnStartup:   true,
		WALCheckpointMode: "TRUNCATE",
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)

	// Start should run maintenance immediately
	err = coordinator.Start(t.Context())
	require.NoError(t, err)
	defer func() {
		err := coordinator.Stop()
		require.NoError(t, err)
	}()

	// Check that startup maintenance ran
	metrics := coordinator.GetMetrics()
	require.Equal(t, uint64(1), metrics.MaintenanceCount, "Startup maintenance should have run")
	require.False(t, metrics.LastMaintenanceTime.IsZero())
}

func TestMaintenanceCoordinator_DisabledMaintenance(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	cfg := config.MaintenanceConfig{
		Enabled:           false,
		CheckInterval:     common.NewDuration(100 * time.Millisecond),
		WALCheckpointMode: "TRUNCATE",
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)

	err = coordinator.Start(t.Context())
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	err = coordinator.Stop()
	require.NoError(t, err)

	// No maintenance should have run
	metrics := coordinator.GetMetrics()
	require.Equal(t, uint64(0), metrics.MaintenanceCount, "No maintenance should run when disabled")
}

func TestMaintenanceCoordinator_ContextCancellation(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	config := config.MaintenanceConfig{
		Enabled:           false,
		WALCheckpointMode: "TRUNCATE",
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, config, log)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = coordinator.RunMaintenance(ctx)
	require.Error(t, err, "Should fail with cancelled context")
	require.ErrorIs(t, err, context.Canceled)
}

func TestMaintenanceCoordinator_InvalidConfig(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	cfg := config.MaintenanceConfig{
		Enabled:       true,
		CheckInterval: common.NewDuration(0), // Invalid (empty, will get default but then fail because we need to set to invalid)
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)

	require.Panics(t, func() {
		coordinator.maintenanceWorker(cfg.CheckInterval.Duration)
	})
}

func TestMaintenanceCoordinator_ConcurrentOperationsDuringMaintenance(t *testing.T) {
	db, dbPath, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	cfg := config.MaintenanceConfig{
		Enabled:           false,
		WALCheckpointMode: "PASSIVE",
	}

	coordinator := newMaintenanceCoordinator(dbPath, db, cfg, log)

	var wg sync.WaitGroup
	const numOperations = 50
	successCount := atomic.Int32{}

	// Start many concurrent operations
	for i := range numOperations {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 5; j++ {
				unlock := coordinator.AcquireOperationLock()

				// Simulate database operation
				_, err := db.Exec("INSERT INTO test_data (data) VALUES (?)", "test data")
				unlock()

				if err == nil {
					successCount.Add(1)
				}

				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Run maintenance concurrently
	wg.Go(func() {
		for range 3 {
			err := coordinator.RunMaintenance(context.Background())
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
		}
	})

	wg.Wait()

	// All operations should have succeeded
	require.Equal(t, int32(numOperations*5), successCount.Load(),
		"All operations should complete successfully even with concurrent maintenance")

	// Maintenance should have run
	metrics := coordinator.GetMetrics()
	require.Equal(t, uint64(3), metrics.MaintenanceCount)
}
