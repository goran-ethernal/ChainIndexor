package helpers

import (
	"database/sql"
	"path"
	"testing"

	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/migrations"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/stretchr/testify/require"
)

// NewTestDB creates a new temporary SQLite database for testing purposes
func NewTestDB(t *testing.T, dbName string) *sql.DB {
	t.Helper()

	tmpDBPath := path.Join(t.TempDir(), dbName)

	dbConfig := config.DatabaseConfig{Path: tmpDBPath}
	dbConfig.ApplyDefaults()

	require.NoError(t, migrations.RunMigrations(dbConfig))

	database, err := db.NewSQLiteDBFromConfig(dbConfig)
	require.NoError(t, err)

	return database
}
