package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	_ "github.com/mattn/go-sqlite3"
	migrate "github.com/rubenv/sql-migrate"
)

const (
	UpDownSeparator     = "-- +migrate Up"
	dbPrefixReplacer    = "/*dbprefix*/"
	NoLimitMigrations   = 0 // indicate that there is no limit on the number of migrations to run
	migrationDirections = 2
)

type Migration struct {
	ID     string
	SQL    string
	Prefix string
}

// RunMigrations will execute pending migrations if needed to keep
// the database updated with the latest changes in either direction,
// up or down.
func RunMigrations(dbPath string, migrations []Migration) error {
	db, err := NewSQLiteDB(dbPath)
	if err != nil {
		return fmt.Errorf("error creating DB %w", err)
	}
	return RunMigrationsDB(logger.GetDefaultLogger(), db, migrations)
}

func RunMigrationsDB(logger *logger.Logger, db *sql.DB, migrationsParam []Migration) error {
	return RunMigrationsDBExtended(logger, db, migrationsParam, migrate.Up, NoLimitMigrations)
}

// RunMigrationsDBExtended is an extended version of RunMigrationsDB that allows
// dir: can be migrate.Up or migrate.Down
// maxMigrations: Will apply at most `max` migrations. Pass 0 for no limit (or use Exec)
func RunMigrationsDBExtended(logger *logger.Logger,
	db *sql.DB,
	migrationsParam []Migration,
	dir migrate.MigrationDirection,
	maxMigrations int) error {
	migs := &migrate.MemoryMigrationSource{Migrations: []*migrate.Migration{}}
	fullmigrations := migrationsParam
	// In case of partial execution we ignore the base migrations
	if maxMigrations != NoLimitMigrations {
		migrate.SetIgnoreUnknown(true)
	}

	for _, m := range fullmigrations {
		prefixed := strings.ReplaceAll(m.SQL, dbPrefixReplacer, m.Prefix)
		splitted := strings.Split(prefixed, UpDownSeparator)

		if len(splitted) < migrationDirections {
			return fmt.Errorf("migration %s missing '-- +migrate Up' separator", m.ID)
		}

		// splitted[0] = Down section (may include "-- +migrate Down" marker)
		// splitted[1] = Up section

		downSQL := splitted[0]
		upSQL := splitted[1]

		// Clean up Down section - remove the Down marker if present
		downMarker := "-- +migrate Down"
		if idx := strings.Index(downSQL, downMarker); idx != -1 {
			downSQL = strings.TrimSpace(downSQL[idx+len(downMarker):])
		} else {
			downSQL = strings.TrimSpace(downSQL)
		}

		upSQL = strings.TrimSpace(upSQL)

		migs.Migrations = append(migs.Migrations, &migrate.Migration{
			Id:   m.Prefix + m.ID,
			Up:   []string{upSQL},
			Down: []string{downSQL},
		})
	}

	var listMigrations strings.Builder
	for _, m := range migs.Migrations {
		listMigrations.WriteString(m.Id + ", ")
	}

	logger.Debugf("running migrations: (max %d/%d) migrations: %s", maxMigrations,
		len(migs.Migrations),
		listMigrations.String())
	nMigrations, err := migrate.ExecMax(db, "sqlite3", migs, dir, maxMigrations)
	if err != nil {
		return fmt.Errorf("error executing migration (max %d/%d) migrations: %s . Err: %w",
			maxMigrations, len(migs.Migrations), listMigrations.String(), err)
	}

	logger.Infof("successfully ran %d migrations from migrations: %s", nMigrations, listMigrations.String())
	return nil
}
