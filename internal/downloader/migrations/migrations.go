package migrations

import (
	_ "embed"

	"github.com/goran-ethernal/ChainIndexor/internal/db"
)

//go:embed 001_initial.sql
var mig0001 string

//go:embed 002_log_store.sql
var mig0002 string

// RunMigrations runs all migrations for the downloader database.
func RunMigrations(dbPath string) error {
	migrations := []db.Migration{
		{
			ID:  "001_initial.sql",
			SQL: mig0001,
		},
		{
			ID:  "002_log_store.sql",
			SQL: mig0002,
		},
	}

	return db.RunMigrations(dbPath, migrations)
}
