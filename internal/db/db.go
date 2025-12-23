package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	_ "github.com/mattn/go-sqlite3"
)

const dbFolderPerm = 0755

// ensureDBFolder ensures the directory that contains dbPath exists.
// Example dbPath: "./data/mytokenindexer.sqlite"
func ensureDBFolder(dbPath string) error {
	dir := filepath.Dir(dbPath)
	return os.MkdirAll(dir, dbFolderPerm)
}

// NewSQLiteDBFromConfig creates a new SQLite DB with the given configuration.
func NewSQLiteDBFromConfig(cfg config.DatabaseConfig) (*sql.DB, error) {
	if err := ensureDBFolder(cfg.Path); err != nil {
		return nil, fmt.Errorf("failed to ensure DB folder: %w", err)
	}

	// Build connection string with configuration options
	foreignKeys := "off"
	if cfg.EnableForeignKeys {
		foreignKeys = "on"
	}

	connStr := fmt.Sprintf(
		"file:%s?_txlock=immediate&_foreign_keys=%s&_journal_mode=%s&_busy_timeout=%d",
		cfg.Path,
		foreignKeys,
		cfg.JournalMode,
		cfg.BusyTimeout,
	)

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Apply connection pool settings
	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)

	// Apply PRAGMA settings
	pragmas := []string{
		fmt.Sprintf("PRAGMA synchronous = %s", cfg.Synchronous),
		fmt.Sprintf("PRAGMA cache_size = %d", cfg.CacheSize),
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	return db, nil
}

// DBTotalSize returns the combined size of the SQLite main file + WAL + SHM.
// If WAL/SHM do not exist, they are simply ignored.
func DBTotalSize(dbPath string) (int64, error) {
	total := int64(0)

	// Check main database file
	if info, err := os.Stat(dbPath); err == nil {
		total += info.Size()
	} else if !os.IsNotExist(err) {
		return 0, err
	}

	// Add WAL + SHM if present
	for _, ext := range []string{"-wal", "-shm"} {
		p := dbPath + ext
		if info, err := os.Stat(p); err == nil {
			total += info.Size()
		} else if !os.IsNotExist(err) {
			return 0, err
		}
	}

	return total, nil
}
