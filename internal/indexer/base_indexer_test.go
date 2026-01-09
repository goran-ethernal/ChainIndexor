package indexer

import (
	"database/sql"
	"testing"

	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

// MockMetadataProvider implements MetadataProvider for testing.
type MockMetadataProvider struct {
	metadata map[string]*EventMetadata
}

func (m *MockMetadataProvider) InitEventMetadata() map[string]*EventMetadata {
	return m.metadata
}

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create test tables
	schema := `
	CREATE TABLE transfers (
		id INTEGER PRIMARY KEY,
		block_number INTEGER NOT NULL,
		tx_index INTEGER NOT NULL,
		log_index INTEGER NOT NULL,
		tx_hash TEXT,
		block_hash TEXT,
		from_address TEXT,
		to_address TEXT,
		value TEXT
	);
	
	CREATE TABLE approvals (
		id INTEGER PRIMARY KEY,
		block_number INTEGER NOT NULL,
		tx_index INTEGER NOT NULL,
		log_index INTEGER NOT NULL,
		tx_hash TEXT,
		block_hash TEXT,
		owner TEXT,
		spender TEXT,
		value TEXT
	);
	`

	_, err = db.Exec(schema)
	require.NoError(t, err)

	return db
}

// createTestMetadata creates test event metadata.
func createTestMetadata(t *testing.T) map[string]*EventMetadata {
	t.Helper()

	return map[string]*EventMetadata{
		"transfer": {
			Name:           "Transfer",
			Table:          "transfers",
			EventType:      nil,
			AddressColumns: []string{"from_address", "to_address"},
		},
		"approval": {
			Name:           "Approval",
			Table:          "approvals",
			EventType:      nil,
			AddressColumns: []string{"owner", "spender"},
		},
	}
}

func TestNewBaseIndexer(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{
		Type:       "test",
		Name:       "test-indexer",
		StartBlock: 1000,
	}

	idx := NewBaseIndexer(db, log, cfg)

	require.NotNil(t, idx)
	require.Equal(t, db, idx.DB)
}

func TestGetType(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{
		Type: "erc20",
		Name: "test-indexer",
	}

	idx := NewBaseIndexer(db, log, cfg)
	require.Equal(t, "erc20", idx.GetType())
}

func TestGetName(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{
		Type: "erc20",
		Name: "my-token-indexer",
	}

	idx := NewBaseIndexer(db, log, cfg)
	require.Equal(t, "my-token-indexer", idx.GetName())
}

func TestStartBlock(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{
		Type:       "erc20",
		Name:       "test",
		StartBlock: 12345,
	}

	idx := NewBaseIndexer(db, log, cfg)
	require.Equal(t, uint64(12345), idx.StartBlock())
}

func TestGetEventTypes(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{Type: "test", Name: "test"}
	bi := NewBaseIndexer(db, log, cfg)

	provider := &MockMetadataProvider{
		metadata: createTestMetadata(t),
	}

	types := bi.GetEventTypes(provider)

	require.Len(t, types, 2)
	require.Contains(t, types, "Transfer")
	require.Contains(t, types, "Approval")
}

func TestGetStats(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	// Insert test data
	_, err := db.Exec(`
	INSERT INTO transfers (block_number, tx_index, log_index, from_address, to_address, value)
	VALUES (100, 1, 0, '0xaaa', '0xbbb', '1000'),
	       (101, 2, 0, '0xccc', '0xddd', '2000'),
	       (102, 1, 0, '0xeee', '0xfff', '3000');
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
	INSERT INTO approvals (block_number, tx_index, log_index, owner, spender, value)
	VALUES (101, 1, 0, '0xaaa', '0x111', '5000');
	`)
	require.NoError(t, err)

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{Type: "test", Name: "test"}
	bi := NewBaseIndexer(db, log, cfg)

	provider := &MockMetadataProvider{
		metadata: createTestMetadata(t),
	}

	ctx := t.Context()
	stats, err := bi.GetStats(ctx, provider)
	require.NoError(t, err)

	statsMap, ok := stats.(map[string]interface{})
	require.True(t, ok, "stats should be a map[string]interface{}")

	require.Equal(t, int64(4), statsMap["total_events"].(int64))       //nolint:forcetypeassert
	require.Equal(t, uint64(100), statsMap["earliest_block"].(uint64)) //nolint:forcetypeassert
	require.Equal(t, uint64(102), statsMap["latest_block"].(uint64))   //nolint:forcetypeassert

	eventCounts, ok := statsMap["event_counts"].(map[string]int64)
	require.True(t, ok, "event_counts should be a map[string]int64")
	require.Equal(t, int64(3), eventCounts["Transfer"])
	require.Equal(t, int64(1), eventCounts["Approval"])
}

func TestGetStatsEmptyTables(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{Type: "test", Name: "test"}
	bi := NewBaseIndexer(db, log, cfg)

	provider := &MockMetadataProvider{
		metadata: createTestMetadata(t),
	}

	ctx := t.Context()
	stats, err := bi.GetStats(ctx, provider)
	require.NoError(t, err)

	statsMap, ok := stats.(map[string]interface{})
	require.True(t, ok, "stats should be a map[string]interface{}")

	require.Equal(t, int64(0), statsMap["total_events"].(int64))     //nolint:forcetypeassert
	require.Equal(t, uint64(0), statsMap["earliest_block"].(uint64)) //nolint:forcetypeassert
	require.Equal(t, uint64(0), statsMap["latest_block"].(uint64))   //nolint:forcetypeassert

	eventCounts, ok := statsMap["event_counts"].(map[string]int64)
	require.True(t, ok, "event_counts should be a map[string]int64")
	require.Equal(t, int64(0), eventCounts["Transfer"])
	require.Equal(t, int64(0), eventCounts["Approval"])
}

func TestGetMetadataUnknownEventType(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{Type: "test", Name: "test"}
	bi := NewBaseIndexer(db, log, cfg)

	provider := &MockMetadataProvider{
		metadata: createTestMetadata(t),
	}

	_, err = bi.getEventMetadata(provider, "UnknownEvent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown event type")
}

func TestHandleReorg(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	// Insert test data
	for i := range 10 {
		_, err := db.Exec(`
		INSERT INTO transfers (block_number, tx_index, log_index, from_address, to_address, value)
		VALUES (?, ?, ?, '0xaaa', '0xbbb', '1000')
		`, 100+i, 0, 0)
		require.NoError(t, err)
	}

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{Type: "test", Name: "test"}
	bi := NewBaseIndexer(db, log, cfg)

	provider := &MockMetadataProvider{
		metadata: createTestMetadata(t),
	}

	// Verify data exists
	var countBefore int
	err = db.QueryRow("SELECT COUNT(*) FROM transfers").Scan(&countBefore)
	require.NoError(t, err)
	require.Equal(t, 10, countBefore)

	// Handle reorg at block 105
	err = bi.HandleReorg(provider, 105)
	require.NoError(t, err)

	// Verify data >= 105 is removed
	var countAfter int
	err = db.QueryRow("SELECT COUNT(*) FROM transfers").Scan(&countAfter)
	require.NoError(t, err)
	require.Equal(t, 5, countAfter)

	// Verify remaining data is < 105
	var maxBlock uint64
	err = db.QueryRow("SELECT MAX(block_number) FROM transfers").Scan(&maxBlock)
	require.NoError(t, err)
	require.Equal(t, uint64(104), maxBlock)
}

func TestHandleReorgEmptyTables(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{Type: "test", Name: "test"}
	bi := NewBaseIndexer(db, log, cfg)

	provider := &MockMetadataProvider{
		metadata: createTestMetadata(t),
	}

	// Should not error on empty tables
	err = bi.HandleReorg(provider, 100)
	require.NoError(t, err)
}

func TestClose(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)

	log, err := logger.NewLogger("debug", true)
	require.NoError(t, err)
	cfg := config.IndexerConfig{Type: "test", Name: "test"}
	bi := NewBaseIndexer(db, log, cfg)

	// Close should not error
	err = bi.Close()
	require.NoError(t, err)

	// Subsequent operations should fail
	_, err = db.Query("SELECT 1")
	require.Error(t, err)
}
