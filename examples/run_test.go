package examples

import (
	"context"
	"testing"
	"time"

	"github.com/goran-ethernal/ChainIndexor/examples/indexers/erc20"
	"github.com/goran-ethernal/ChainIndexor/internal/common"
	"github.com/goran-ethernal/ChainIndexor/internal/config"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/downloader"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/metrics"
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

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ethClient, err := rpc.NewClient(ctx, cfg.Downloader.RPCURL, cfg.Downloader.Retry) // Example RPC URL
	if err != nil {
		t.Fatalf("failed to create RPC client: %v", err)
	}

	// Initialize and start metrics server if enabled
	var metricsServer *metrics.Server
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		metricsServer = metrics.NewServer(cfg.Metrics)
		if err := metricsServer.Start(ctx); err != nil {
			t.Fatalf("failed to start metrics server: %v", err)
		}
		defer func() {
			if err := metricsServer.Stop(ctx); err != nil {
				t.Logf("failed to stop metrics server: %v", err)
			}
		}()
		t.Logf("Metrics server started on %s%s", cfg.Metrics.ListenAddress, cfg.Metrics.Path)
	}

	err = downloadermig.RunMigrations(cfg.Downloader.DB)
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	database, err := db.NewSQLiteDBFromConfig(cfg.Downloader.DB)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	dbMaintainance := db.NewMaintenanceCoordinator(
		cfg.Downloader.DB.Path,
		database,
		cfg.Downloader.Maintenance,
		logger.NewComponentLoggerFromConfig(common.ComponentMaintenance, cfg.Logging),
	)

	reorgDetector, err := reorg.NewReorgDetector(
		database, ethClient,
		logger.NewComponentLoggerFromConfig(common.ComponentReorgDetector, cfg.Logging),
		dbMaintainance,
	)
	if err != nil {
		t.Fatalf("failed to create reorg detector: %v", err)
	}

	syncManager, err := downloader.NewSyncManager(
		database,
		logger.NewComponentLoggerFromConfig(common.ComponentSyncManager, cfg.Logging),
		dbMaintainance,
	)
	if err != nil {
		t.Fatalf("failed to create sync manager: %v", err)
	}

	downloader, err := downloader.New(
		cfg.Downloader,
		ethClient,
		reorgDetector,
		syncManager,
		dbMaintainance,
		logger.NewComponentLoggerFromConfig(common.ComponentDownloader, cfg.Logging),
	)
	if err != nil {
		t.Fatalf("failed to create downloader: %v", err)
	}

	erc20Indexer, err := erc20.NewERC20TokenIndexer(cfg.Indexers[0], logger.GetDefaultLogger())
	if err != nil {
		t.Fatalf("failed to create ERC20 indexer: %v", err)
	}

	downloader.RegisterIndexer(erc20Indexer)

	errCh := make(chan error, 1)
	go func() {
		errCh <- downloader.Download(ctx, *cfg)
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
