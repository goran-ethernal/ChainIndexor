package downloader

import (
	"database/sql"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/migrations"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	tmpDB := t.TempDir() + "/test_downloader.db"

	// Create database config
	dbConfig := config.DatabaseConfig{
		Path: tmpDB,
	}
	dbConfig.ApplyDefaults()

	err := migrations.RunMigrations(dbConfig)
	require.NoError(t, err)

	db, err := db.NewSQLiteDBFromConfig(dbConfig)
	require.NoError(t, err)

	cleanup := func() {
		os.Remove(tmpDB)
		db.Close()
	}

	return db, cleanup
}

func TestSyncManager(t *testing.T) {
	// Create a temporary database file
	tmpDB, cleanup := setupTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	// Create SyncManager
	sm, err := NewSyncManager(tmpDB, log)
	require.NoError(t, err)
	defer sm.Close()

	// Test initial state
	state, err := sm.GetState()
	require.NoError(t, err)
	require.Equal(t, uint64(0), state.LastIndexedBlock)
	require.Equal(t, fetcher.ModeBackfill, state.GetMode())

	// Test GetLastIndexedBlock
	lastBlock, err := sm.GetLastIndexedBlock()
	require.NoError(t, err)
	require.Equal(t, uint64(0), lastBlock)

	// Test SaveCheckpoint
	testHash := common.HexToHash("0xabc123")
	err = sm.SaveCheckpoint(100, testHash, fetcher.ModeBackfill)
	require.NoError(t, err)

	lastBlock, err = sm.GetLastIndexedBlock()
	require.NoError(t, err)
	require.Equal(t, uint64(100), lastBlock)

	state, err = sm.GetState()
	require.NoError(t, err)
	require.Equal(t, uint64(100), state.LastIndexedBlock)
	require.Equal(t, testHash, state.LastIndexedBlockHash)
	require.Equal(t, fetcher.ModeBackfill, state.GetMode())
	require.Greater(t, state.LastIndexedTimestamp, int64(0))

	// Test mode change
	testHash2 := common.HexToHash("0xdef456")
	err = sm.SaveCheckpoint(200, testHash2, fetcher.ModeLive)
	require.NoError(t, err)

	state, err = sm.GetState()
	require.NoError(t, err)
	require.Equal(t, uint64(200), state.LastIndexedBlock)
	require.Equal(t, testHash2, state.LastIndexedBlockHash)
	require.Equal(t, fetcher.ModeLive, state.GetMode())

	// Test SetMode
	err = sm.SetMode(fetcher.ModeBackfill)
	require.NoError(t, err)

	state, err = sm.GetState()
	require.NoError(t, err)
	require.Equal(t, uint64(200), state.LastIndexedBlock)   // Block unchanged
	require.Equal(t, testHash2, state.LastIndexedBlockHash) // Hash unchanged
	require.Equal(t, fetcher.ModeBackfill, state.GetMode()) // Mode changed

	// Test Reset
	err = sm.Reset(50)
	require.NoError(t, err)

	state, err = sm.GetState()
	require.NoError(t, err)
	require.Equal(t, uint64(50), state.LastIndexedBlock)
	require.Equal(t, common.Hash{}, state.LastIndexedBlockHash) // Hash cleared on reset
	require.Equal(t, fetcher.ModeBackfill, state.GetMode())
}

func TestSyncManagerPersistence(t *testing.T) {
	// Create a temporary database file
	tmpDB, cleanup := setupTestDB(t)
	defer cleanup()

	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	// Create SyncManager and save a checkpoint
	sm, err := NewSyncManager(tmpDB, log)
	require.NoError(t, err)

	persistHash := common.HexToHash("0x123abc")
	err = sm.SaveCheckpoint(500, persistHash, fetcher.ModeLive)
	require.NoError(t, err)

	// Create a new SyncManager with the same database
	sm2, err := NewSyncManager(tmpDB, log)
	require.NoError(t, err)

	// Verify the checkpoint was persisted
	state, err := sm2.GetState()
	require.NoError(t, err)
	require.Equal(t, uint64(500), state.LastIndexedBlock)
	require.Equal(t, persistHash, state.LastIndexedBlockHash)
	require.Equal(t, fetcher.ModeLive, state.GetMode())
}
