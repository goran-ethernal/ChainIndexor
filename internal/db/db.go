package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// NewSQLiteDB creates a new SQLite DB
func NewSQLiteDB(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite3", fmt.Sprintf(
		"file:%s?_txlock=immediate&_foreign_keys=on&_journal_mode=WAL&_busy_timeout=30000",
		dbPath,
	))
}
