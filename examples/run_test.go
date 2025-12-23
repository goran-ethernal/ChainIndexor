package examples

import (
	"context"
	"testing"
	"time"

	"github.com/goran-ethernal/ChainIndexor/examples/indexers/erc20"
	"github.com/goran-ethernal/ChainIndexor/internal/config"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/downloader"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	downloadermig "github.com/goran-ethernal/ChainIndexor/internal/migrations"
	"github.com/goran-ethernal/ChainIndexor/internal/reorg"
	"github.com/goran-ethernal/ChainIndexor/internal/rpc"
)

func TestRun(t *testing.T) {
	t.Skip("Exploratory example test - uncomment this and change the example config to run")

	// Uncomment to clean up data directory before every test run
	// require.NoError(t, os.RemoveAll("./data"))

	configPath := "../config.example.yaml"

	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ethClient, err := rpc.NewClient(t.Context(), cfg.Downloader.RPCURL) // Example RPC URL
	if err != nil {
		t.Fatalf("failed to create RPC client: %v", err)
	}

	err = downloadermig.RunMigrations(cfg.Downloader.DB)
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	database, err := db.NewSQLiteDBFromConfig(cfg.Downloader.DB)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	dbMaintainance := db.NewMaintenanceCoordinator(cfg.Downloader.DB.Path, database,
		cfg.Downloader.Maintenance, logger.GetDefaultLogger())

	reorgDetector, err := reorg.NewReorgDetector(database, ethClient, logger.GetDefaultLogger(), dbMaintainance)
	if err != nil {
		t.Fatalf("failed to create reorg detector: %v", err)
	}

	syncManager, err := downloader.NewSyncManager(database, logger.GetDefaultLogger(), dbMaintainance)
	if err != nil {
		t.Fatalf("failed to create sync manager: %v", err)
	}

	downloader, err := downloader.New(
		cfg.Downloader,
		ethClient,
		reorgDetector,
		syncManager,
		dbMaintainance,
		logger.GetDefaultLogger(),
	)
	if err != nil {
		t.Fatalf("failed to create downloader: %v", err)
	}

	erc20Indexer, err := erc20.NewERC20TokenIndexer(cfg.Indexers[0], logger.GetDefaultLogger())
	if err != nil {
		t.Fatalf("failed to create ERC20 indexer: %v", err)
	}

	downloader.RegisterIndexer(erc20Indexer)

	context, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- downloader.Download(context)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("downloader failed: %v", err)
		}
	case <-time.After(1 * time.Hour): // change to desired run time
		cancel()
		downloader.Close()
	}
}
