# Custom Indexer Example

This example demonstrates how to use ChainIndexor as a library to build your own application with custom indexers.

## Structure

```text
custom-indexer/
├── main.go              # Application entry point
├── go.mod               # Go module definition
└── indexers/
    └── mycontract/      # Generated indexer (via indexer-gen)
```

## Usage

### 1. Initialize your project

```bash
mkdir my-indexer-project
cd my-indexer-project
go mod init myproject
```

### 2. Install ChainIndexor

```bash
go get github.com/goran-ethernal/ChainIndexor
```

### 3. Generate your indexer

```bash
# Install the code generator
go install github.com/goran-ethernal/ChainIndexor/cmd/indexer-gen@latest

# Generate your custom indexer
indexer-gen \
  --name MyContract \
  --event "MyEvent(address indexed user, uint256 amount)" \
  --output ./indexers/mycontract
```

### 4. Create your main.go

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/goran-ethernal/ChainIndexor/internal/common"
    "github.com/goran-ethernal/ChainIndexor/internal/config"
    "github.com/goran-ethernal/ChainIndexor/internal/db"
    "github.com/goran-ethernal/ChainIndexor/internal/downloader"
    "github.com/goran-ethernal/ChainIndexor/internal/logger"
    downloadermig "github.com/goran-ethernal/ChainIndexor/internal/migrations"
    "github.com/goran-ethernal/ChainIndexor/internal/reorg"
    "github.com/goran-ethernal/ChainIndexor/internal/rpc"

    // Import your generated indexer
    "myproject/indexers/mycontract"
)

func main() {
    // Load configuration
    cfg, err := config.LoadFromFile("config.yaml")
    if err != nil {
        panic(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-sigCh
        cancel()
    }()

    // Initialize logger
    log := logger.GetDefaultLogger()

    // Initialize RPC client
    ethClient, err := rpc.NewClient(ctx, cfg.Downloader.RPCURL, cfg.Downloader.Retry)
    if err != nil {
        log.Fatalf("Failed to create RPC client: %v", err)
    }

    // Run migrations
    if err := downloadermig.RunMigrations(cfg.Downloader.DB); err != nil {
        log.Fatalf("Failed to run migrations: %v", err)
    }

    // Initialize database
    database, err := db.NewSQLiteDBFromConfig(cfg.Downloader.DB)
    if err != nil {
        log.Fatalf("Failed to create database: %v", err)
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
        log.Fatalf("Failed to create reorg detector: %v", err)
    }

    // Initialize sync manager
    syncManager, err := downloader.NewSyncManager(
        database,
        logger.NewComponentLoggerFromConfig(common.ComponentSyncManager, cfg.Logging),
        dbMaintenance,
    )
    if err != nil {
        log.Fatalf("Failed to create sync manager: %v", err)
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
        log.Fatalf("Failed to create downloader: %v", err)
    }
    defer dl.Close()

    // Create and register your custom indexer
    myIndexer, err := mycontract.NewMyContractIndexer(cfg.Indexers[0], log)
    if err != nil {
        log.Fatalf("Failed to create indexer: %v", err)
    }
    dl.RegisterIndexer(myIndexer)

    log.Info("Starting indexer...")

    // Start indexing
    if err := dl.Download(ctx, *cfg); err != nil {
        log.Fatalf("Downloader failed: %v", err)
    }

    log.Info("Indexer stopped successfully")
}
```

### 5. Create config.yaml

```yaml
downloader:
  rpc_url: "https://eth-mainnet.g.alchemy.com/v2/YOUR_KEY"
  chunk_size: 5000
  finality: "finalized"
  db:
    path: "./data/downloader.sqlite"
  maintenance:
    enabled: true
    check_interval: "5m"

indexers:
  - name: "MyContractIndexer"
    # Note: No 'type' field needed - you're wiring it up manually
    start_block: 0
    db:
      path: "./data/mycontract.sqlite"
    contracts:
      - address: "0xYourContractAddress"
        events:
          - "MyEvent(address,uint256)"

logging:
  default_level: "info"
```

### 6. Run your indexer

```bash
go run main.go
```

## Advantages of This Approach

- ✅ **Full Control**: Complete control over initialization and wiring
- ✅ **Custom Logic**: Add custom processing, validation, or transformations
- ✅ **Flexibility**: Mix multiple custom indexers, built-in indexers, and custom components
- ✅ **Integration**: Easy to integrate with existing Go applications
- ✅ **Type Safety**: All benefits of static typing and compile-time checks

## Next Steps

- Add custom processing logic in your indexer
- Implement additional event handlers
- Add custom database queries
- Integrate with your application's business logic
- Add monitoring and alerting
