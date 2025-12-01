package migrations

import (
	_ "embed"

	"github.com/goran-ethernal/ChainIndexor/internal/db"
)

//go:embed 001_downloader_sync_manager_1.sql
var mig001 string

//go:embed 002_downloader_log_store_1.sql
var mig002 string

//go:embed 003_downloader_reorg_detector_1.sql
var mig003 string

func RunMigrations(dbPath string) error {
	migrations := []db.Migration{
		{
			ID:  "001_downloader_sync_manager_1.sql",
			SQL: mig001,
		},
		{
			ID:  "002_downloader_log_store_1.sql",
			SQL: mig002,
		},
		{
			ID:  "003_downloader_reorg_detector_1.sql",
			SQL: mig003,
		},
	}

	return db.RunMigrations(dbPath, migrations)
}
