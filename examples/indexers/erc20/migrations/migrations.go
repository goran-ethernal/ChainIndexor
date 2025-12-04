package migrations

import (
	_ "embed"

	"github.com/goran-ethernal/ChainIndexor/internal/db"
)

//go:embed 001_initial.sql
var mig0001 string

// RunMigrations runs all migrations for the ERC20 indexer database.
func RunMigrations(dbPath string) error {
	migrations := []db.Migration{
		{
			ID:  "001_initial.sql",
			SQL: mig0001,
		},
	}

	return db.RunMigrations(dbPath, migrations)
}
