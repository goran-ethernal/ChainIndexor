package db

import (
	"database/sql"
	"fmt"

	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	_ "github.com/mattn/go-sqlite3"
)

// NewSQLiteDB creates a new SQLite DB
func NewSQLiteDB(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite3", fmt.Sprintf(
		"file:%s?_txlock=immediate&_foreign_keys=on&_journal_mode=WAL&_busy_timeout=30000",
		dbPath,
	))
}

// NewSQLiteDBFromConfig creates a new SQLite DB with the given configuration.
func NewSQLiteDBFromConfig(cfg config.DatabaseConfig) (*sql.DB, error) {
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
