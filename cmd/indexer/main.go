package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	// Import built-in indexers to register them
	_ "github.com/goran-ethernal/ChainIndexor/examples/indexers/erc20"
	"github.com/goran-ethernal/ChainIndexor/internal/common"
	"github.com/goran-ethernal/ChainIndexor/internal/config"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/downloader"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/metrics"
	downloadermig "github.com/goran-ethernal/ChainIndexor/internal/migrations"
	"github.com/goran-ethernal/ChainIndexor/internal/reorg"
	"github.com/goran-ethernal/ChainIndexor/internal/rpc"
	"github.com/goran-ethernal/ChainIndexor/pkg/api"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
	"github.com/spf13/cobra"
)

const (
	version = "1.0.0"
	banner  = `
╔═══════════════════════════════════════════╗
║         ChainIndexor v%s               ║
║   Blockchain Event Indexing Framework     ║
╚═══════════════════════════════════════════╝
`
)

var (
	configPath string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "indexer",
	Short: "ChainIndexor - Blockchain event indexing framework",
	Long: `ChainIndexor is a production-ready framework for indexing blockchain events.
It provides real-time event processing, automatic reorg handling, and persistent
storage with support for multiple built-in indexers.`,
	Version: version,
	RunE:    runIndexer,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available indexer types",
	Long:  `List all registered indexer types that can be used in the configuration file.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available indexer types:")
		types := indexer.ListRegistered()
		if len(types) == 0 {
			fmt.Println("  (no indexers registered)")
			return
		}
		for _, t := range types {
			fmt.Printf("  - %s\n", t)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to configuration file")
	rootCmd.AddCommand(listCmd)
}

func runIndexer(cmd *cobra.Command, args []string) error {
	fmt.Printf(banner, version)

	// Load configuration
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\nShutting down gracefully...")
		cancel()
	}()

	// Initialize logger
	log := logger.NewComponentLoggerFromConfig(common.ComponentDownloader, cfg.Logging)

	// Initialize RPC client
	log.Info("Connecting to Ethereum node...")
	ethClient, err := rpc.NewClient(ctx, cfg.Downloader.RPCURL, cfg.Downloader.Retry)
	if err != nil {
		return fmt.Errorf("failed to create RPC client: %w", err)
	}
	log.Infof("Connected to Ethereum node: %s", cfg.Downloader.RPCURL)

	// Initialize metrics server if enabled
	var metricsServer *metrics.Server
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		metricsServer = metrics.NewServer(cfg.Metrics)
		if err := metricsServer.Start(ctx); err != nil {
			return fmt.Errorf("failed to start metrics server: %w", err)
		}
		defer func() {
			if err := metricsServer.Stop(ctx); err != nil {
				log.Warnf("Failed to stop metrics server: %v", err)
			}
		}()
		log.Infof("Metrics server started on %s%s", cfg.Metrics.ListenAddress, cfg.Metrics.Path)
	}

	// Run downloader migrations
	log.Info("Running database migrations...")
	if err := downloadermig.RunMigrations(cfg.Downloader.DB); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize database
	database, err := db.NewSQLiteDBFromConfig(cfg.Downloader.DB)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	// Initialize maintenance coordinator
	dbMaintenance := db.NewMaintenanceCoordinator(
		cfg.Downloader.DB.Path,
		database,
		cfg.Downloader.Maintenance,
		logger.NewComponentLoggerFromConfig(common.ComponentMaintenance, cfg.Logging),
	)

	// Initialize reorg detector
	reorgDetector, err := reorg.NewReorgDetector(
		database,
		ethClient,
		logger.NewComponentLoggerFromConfig(common.ComponentReorgDetector, cfg.Logging),
		dbMaintenance,
	)
	if err != nil {
		return fmt.Errorf("failed to create reorg detector: %w", err)
	}

	// Initialize sync manager
	syncManager, err := downloader.NewSyncManager(
		database,
		logger.NewComponentLoggerFromConfig(common.ComponentSyncManager, cfg.Logging),
		dbMaintenance,
	)
	if err != nil {
		return fmt.Errorf("failed to create sync manager: %w", err)
	}

	// Initialize downloader
	dl, err := downloader.New(
		cfg.Downloader,
		ethClient,
		reorgDetector,
		syncManager,
		dbMaintenance,
		logger.NewComponentLoggerFromConfig(common.ComponentDownloader, cfg.Logging),
	)
	if err != nil {
		return fmt.Errorf("failed to create downloader: %w", err)
	}
	defer dl.Close()

	// Register indexers from configuration
	log.Infof("Registering %d indexer(s)...", len(cfg.Indexers))
	if len(cfg.Indexers) == 0 {
		log.Warn("No indexers configured. Exiting.")
		return nil
	}

	for i, idxCfg := range cfg.Indexers {
		if idxCfg.Type == "" {
			return fmt.Errorf("indexer #%d (%s) is missing 'type' field in configuration", i+1, idxCfg.Name)
		}

		log.Infof("Creating indexer: %s (type: %s)", idxCfg.Name, idxCfg.Type)

		idx, err := indexer.Create(
			idxCfg.Type,
			idxCfg,
			logger.GetDefaultLogger(),
		)
		if err != nil {
			return fmt.Errorf("failed to create indexer %s: %w", idxCfg.Name, err)
		}

		dl.RegisterIndexer(idx)
		log.Infof("✓ Registered indexer: %s", idxCfg.Name)
	}

	// Start API server if enabled
	if cfg.API != nil && cfg.API.Enabled {
		apiServer := api.NewServer(
			cfg.API,
			dl.Coordinator(),
			logger.NewComponentLoggerFromConfig(common.ComponentAPI, cfg.Logging),
		)
		go func() {
			if err := apiServer.Start(ctx); err != nil {
				log.Errorf("API server error: %v", err)
			}
		}()
	}

	// Start indexing
	log.Info("Starting ChainIndexor...")

	if err := dl.Download(ctx, *cfg); err != nil {
		return fmt.Errorf("downloader failed: %w", err)
	}

	log.Info("ChainIndexor stopped successfully")
	return nil
}
