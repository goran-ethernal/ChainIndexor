package db

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T, journal string) (*sql.DB, string, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "logstore_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	dbPath := tmpFile.Name()

	// Create database
	dbConfig := config.DatabaseConfig{Path: dbPath, JournalMode: journal}
	dbConfig.ApplyDefaults()

	sqlDB, err := NewSQLiteDBFromConfig(dbConfig)
	require.NoError(t, err)

	// Insert test table
	_, err = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, value TEXT);`)
	require.NoError(t, err)

	// Insert test data
	for i := range 5000 {
		_, err = sqlDB.Exec(`INSERT INTO test_table (value) VALUES (?);`, fmt.Sprintf("value_%d", i))
		require.NoError(t, err)
	}

	cleanup := func() {
		sqlDB.Close()
		os.Remove(dbPath)
	}

	return sqlDB, dbPath, cleanup
}

func TestVacuum_Modes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		journalMode string
	}{
		{name: "WAL", journalMode: "WAL"},
		{name: "NonWAL", journalMode: "TRUNCATE"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, dbPath, cleanup := setupTestDB(t, tc.journalMode)
			defer cleanup()

			initialSize, err := DBTotalSize(dbPath)
			require.NoError(t, err)

			require.NoError(t, Vacuum(db))

			finalSize, err := DBTotalSize(dbPath)
			require.NoError(t, err)

			require.LessOrEqual(t, finalSize, initialSize)
		})
	}
}

func TestDBTotalSize(t *testing.T) {
	type fileSpec struct {
		name string
		data []byte
	}
	testCases := []struct {
		name        string
		files       []fileSpec // main file is always first, then optional -wal and -shm
		setup       func(paths []string) error
		expectSize  int64
		expectError bool
	}{
		{
			name: "MainOnly",
			files: []fileSpec{
				{name: "main", data: []byte("main-db-content")},
			},
			setup: func(paths []string) error {
				return os.WriteFile(paths[0], []byte("main-db-content"), 0644)
			},
			expectSize:  int64(len("main-db-content")),
			expectError: false,
		},
		{
			name: "WithWALAndSHM",
			files: []fileSpec{
				{name: "main", data: []byte("main-db")},
				{name: "wal", data: []byte("wal-content")},
				{name: "shm", data: []byte("shm-content")},
			},
			setup: func(paths []string) error {
				if err := os.WriteFile(paths[0], []byte("main-db"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(paths[1], []byte("wal-content"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(paths[2], []byte("shm-content"), 0644); err != nil {
					return err
				}
				return nil
			},
			expectSize:  int64(len("main-db") + len("wal-content") + len("shm-content")),
			expectError: false,
		},
		{
			name:        "MissingFiles",
			files:       []fileSpec{{name: "main", data: nil}},
			setup:       func(paths []string) error { return nil }, // don't create file
			expectSize:  0,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare temp files
			tmpDir := t.TempDir()
			mainPath := tmpDir + "/main.db"
			paths := []string{mainPath}
			if len(tc.files) > 1 {
				for _, f := range tc.files[1:] {
					paths = append(paths, mainPath+"-"+f.name)
				}
			}
			// Setup files as needed
			require.NoError(t, tc.setup(paths))
			// No permission cleanup needed for StatError (file is removed)
			// Remove files after test
			defer func() {
				for _, p := range paths {
					os.Remove(p)
				}
			}()

			size, err := DBTotalSize(mainPath)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectSize, size)
			}
		})
	}
}
