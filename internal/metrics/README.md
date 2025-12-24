# Metrics Package

This package provides Prometheus metrics for monitoring ChainIndexor performance and health.

## Overview

The metrics package exposes Prometheus metrics directly without abstraction layers. Metrics are distributed across different packages based on their domain. All metrics are package-level variables that can be accessed and updated from anywhere in the application.

## Configuration

Add to your config file:

```yaml
metrics:
  enabled: true
  listen_address: ":9090"
  path: "/metrics"
```

## Starting the Metrics Server

```go
import "github.com/goran-ethernal/ChainIndexor/internal/metrics"

// Create and start metrics server
metricsServer := metrics.NewServer(config.Metrics)
if err := metricsServer.Start(); err != nil {
    log.Fatal(err)
}
defer metricsServer.Stop(context.Background())
```

## Available Metrics

### Indexing Metrics (5 metrics)

**Package**: `internal/metrics`

| Metric | Type | Labels | Description |
| ------ | ---- | ------ | ----------- |
| `chainindexor_last_indexed_block` | Gauge | indexer | The last block number successfully indexed |
| `chainindexor_blocks_processed_total` | Counter | indexer | Total number of blocks processed |
| `chainindexor_logs_indexed_total` | Counter | indexer | Total number of logs indexed |
| `chainindexor_block_processing_duration_seconds` | Histogram | indexer | Time taken to process a batch of blocks |
| `chainindexor_indexing_rate_blocks_per_second` | Gauge | indexer | Current indexing rate in blocks per second |

**Usage**:

```go
import "github.com/goran-ethernal/ChainIndexor/internal/metrics"

// Update last indexed block
metrics.LastIndexedBlockInc("my-indexer", 12345)

// Increment blocks processed
metrics.BlocksProcessedInc("my-indexer", 10)

// Record logs indexed
metrics.LogsIndexedInc("my-indexer", 100)

// Measure block processing time
metrics.BlockProcessingTimeLog("my-indexer", duration)

// Update indexing rate
metrics.IndexingRateLog("my-indexer", 150.5)
```

### Finalized Block Metric (1 metric)

**Package**: `internal/fetcher`

| Metric | Type | Labels | Description |
| ------ | ---- | ------ | ----------- |
| `chainindexor_finalized_block` | Gauge | - | The current finalized block number from RPC |

**Usage**:

```go
import "github.com/goran-ethernal/ChainIndexor/internal/fetcher"

// Update finalized block
fetcher.FinalizedBlockLogSet(12350)
```

### RPC Metrics (5 metrics)

**Package**: `internal/rpc`

| Metric | Type | Labels | Description |
| ------ | ---- | ------ | ----------- |
| `chainindexor_rpc_requests_total` | Counter | method | Total number of RPC requests by method |
| `chainindexor_rpc_errors_total` | Counter | method, error_type | Total number of RPC errors by method and type |
| `chainindexor_rpc_request_duration_seconds` | Histogram | method | Duration of RPC requests |
| `chainindexor_rpc_retries_total` | Counter | method | Total number of RPC request retries |

**Usage**:

```go
import "github.com/goran-ethernal/ChainIndexor/internal/rpc"

// Track RPC request
rpc.RPCMethodInc("eth_getLogs")

// Track RPC errors
rpc.RPCMethodError("eth_getLogs", "timeout")

// Measure RPC duration
rpc.RPCMethodDuration("eth_getLogs", duration)

// Direct access to metrics
rpc.RPCRetries.WithLabelValues("eth_getLogs").Inc()
```

### Database Metrics (4 metrics)

**Package**: `internal/metrics` and `internal/db`

| Metric | Type | Labels | Description |
| ------ | ---- | ------ | ----------- |
| `chainindexor_db_queries_total` | Counter | db, operation | Total number of database queries |
| `chainindexor_db_query_duration_seconds` | Histogram | db, operation | Duration of database queries |
| `chainindexor_db_errors_total` | Counter | db, error_type | Total number of database errors |
| `chainindexor_db_size_bytes` | Gauge | type | Database file size in bytes |

**Usage**:

```go
import "github.com/goran-ethernal/ChainIndexor/internal/metrics"

// Track queries
metrics.DBQueryInc("logs", "insert")

// Measure query time
metrics.DBQueryDuration("logs", "insert", duration)

// Track errors
metrics.DBErrorsInc("logs", "lock_timeout")
```

### Maintenance Metrics (7 metrics)

**Package**: `internal/db`

| Metric | Type | Labels | Description |
| ------ | ---- | ------ | ----------- |
| `chainindexor_maintenance_runs_total` | Counter | - | Total number of maintenance operations |
| `chainindexor_maintenance_outcomes_total` | Counter | status | Total number of maintenance operations by outcome (success/error) |
| `chainindexor_maintenance_duration_seconds` | Histogram | - | Duration of maintenance operations |
| `chainindexor_maintenance_last_run_timestamp` | Gauge | - | Unix timestamp of last maintenance run |
| `chainindexor_maintenance_space_reclaimed_bytes` | Gauge | - | Bytes reclaimed by last maintenance operation |
| `chainindexor_wal_checkpoint_total` | Counter | mode | Total number of WAL checkpoint operations |
| `chainindexor_vacuum_total` | Counter | - | Total number of VACUUM operations |

**Usage**:

```go
import "github.com/goran-ethernal/ChainIndexor/internal/db"

// Track maintenance run
db.MaintenanceRunsInc()

// Track outcome
db.MaintenanceSuccessInc()
db.MaintenanceErrorInc()

// Measure duration
db.MaintenanceDurationLog(duration)

// Update last run time
db.MaintenanceLastRunLog()

// Record space reclaimed
db.MaintenanceSpaceReclaimedLog(bytesReclaimed)
```

### Reorg Metrics (4 metrics)

**Package**: `internal/reorg`

| Metric | Type | Labels | Description |
| ------ | ---- | ------ | ----------- |
| `chainindexor_reorgs_detected_total` | Counter | - | Total number of blockchain reorganizations detected |
| `chainindexor_reorg_depth_blocks` | Histogram | - | Depth of blockchain reorganizations in blocks |
| `chainindexor_reorg_last_detected_timestamp` | Gauge | - | Unix timestamp of last reorg detection |
| `chainindexor_reorg_from_block` | Histogram | - | Block numbers where reorgs started |

**Usage**:

```go
import "github.com/goran-ethernal/ChainIndexor/internal/reorg"

// Detect reorg (logs all metrics at once)
reorg.ReorgDetectedLog(depth, fromBlock)

// Or manually
reorg.ReorgsDetected.Inc()
reorg.ReorgDepth.Observe(5)
reorg.ReorgLastDetected.Set(float64(time.Now().Unix()))
reorg.ReorgFromBlock.Observe(12340)
```

### Retention Metrics (2 metrics)

**Package**: `internal/fetcher/store`

| Metric | Type | Labels | Description |
| ------ | ---- | ------ | ----------- |
| `chainindexor_retention_blocks_pruned_total` | Counter | db | Total number of blocks pruned by retention policy |
| `chainindexor_retention_logs_pruned_total` | Counter | db | Total number of logs pruned by retention policy |

**Usage**:

```go
import "github.com/goran-ethernal/ChainIndexor/internal/fetcher/store"

// Record blocks pruned
store.RetentionBlocksPrunedInc("logs", 1000)

// Record logs pruned
store.RetentionLogsPrunedInc("logs", 50000)
```

### System Metrics (5 metrics)

**Package**: `internal/metrics`

| Metric | Type | Labels | Description |
| ------ | ---- | ------ | ----------- |
| `chainindexor_uptime_seconds` | Gauge | - | Application uptime in seconds |
| `chainindexor_errors_total` | Counter | component, severity | Total number of errors by component and severity |
| `chainindexor_component_health` | Gauge | component | Component health status (1=healthy, 0=unhealthy) |
| `chainindexor_goroutines` | Gauge | - | Number of active goroutines |
| `chainindexor_memory_usage_bytes` | Gauge | type | Memory usage statistics (alloc, total_alloc, sys, heap_inuse) |

**Usage**:

```go
import "github.com/goran-ethernal/ChainIndexor/internal/metrics"

// Update system metrics (automatically called every 15 seconds)
metrics.UpdateSystemMetrics()

// Track errors
metrics.Errors.WithLabelValues("downloader", "error").Inc()

// Update component health
metrics.ComponentHealthSet("downloader", true)  // healthy
metrics.ComponentHealthSet("logstore", false)   // unhealthy
```

## Metrics Summary

**Total: 33 metrics** across 7 categories

- **Indexing**: 5 metrics (blocks processed, logs indexed, processing time, rate)
- **Finalized Block**: 1 metric (current finalized block)
- **RPC**: 5 metrics (requests, errors, duration, connections, retries)
- **Database**: 4 metrics (queries, query duration, errors, size)
- **Maintenance**: 7 metrics (runs, outcomes, duration, last run, space reclaimed, WAL, vacuum)
- **Reorg**: 4 metrics (detected, depth, last detected, from block)
- **Retention**: 2 metrics (blocks pruned, logs pruned)
- **System**: 5 metrics (uptime, errors, component health, goroutines, memory)

## Accessing Metrics

Once the server is running, metrics are available at:

- Metrics: `http://localhost:9090/metrics`
- Health: `http://localhost:9090/health`

## Prometheus Configuration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'chainindexor'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
```

## Example Queries

### Monitor Indexing Progress

```promql
# Indexing rate (blocks per second)
rate(chainindexor_blocks_processed_total[5m])

# Last indexed block
chainindexor_last_indexed_block

# Block lag (calculated from finalized block)
chainindexor_finalized_block - chainindexor_last_indexed_block

# Time to process blocks (95th percentile)
histogram_quantile(0.95, rate(chainindexor_block_processing_duration_seconds_bucket[5m]))
```

### Monitor RPC Health

```promql
# RPC error rate
rate(chainindexor_rpc_errors_total[5m])

# RPC request rate by method
sum by (method) (rate(chainindexor_rpc_requests_total[5m]))

# Average RPC latency
rate(chainindexor_rpc_request_duration_seconds_sum[5m]) / 
rate(chainindexor_rpc_request_duration_seconds_count[5m])

# Active connections
chainindexor_rpc_connections_active
```

### Monitor Database

```promql
# Database size
chainindexor_db_size_bytes

# Query rate
sum by (operation) (rate(chainindexor_db_queries_total[5m]))

# Slow queries (95th percentile)
histogram_quantile(0.95, rate(chainindexor_db_query_duration_seconds_bucket[5m]))

# Database errors
rate(chainindexor_db_errors_total[5m])
```

### Monitor Maintenance

```promql
# Maintenance runs
rate(chainindexor_maintenance_runs_total[1h])

# Last maintenance run (time since)
time() - chainindexor_maintenance_last_run_timestamp

# Space reclaimed
chainindexor_maintenance_space_reclaimed_bytes

# Success rate
rate(chainindexor_maintenance_outcomes_total{status="success"}[1h]) /
rate(chainindexor_maintenance_runs_total[1h])
```

### Monitor System Health

```promql
# Component health (0 = down, 1 = up)
chainindexor_component_health

# Memory usage
chainindexor_memory_usage_bytes{type="heap_inuse"}

# Goroutine count
chainindexor_goroutines

# Error rate by component
rate(chainindexor_errors_total[5m])

# Application uptime
chainindexor_uptime_seconds
```

### Monitor Reorgs

```promql
# Reorg detection rate
rate(chainindexor_reorgs_detected_total[1h])

# Average reorg depth
rate(chainindexor_reorg_depth_blocks_sum[1h]) / 
rate(chainindexor_reorg_depth_blocks_count[1h])

# Time since last reorg
time() - chainindexor_reorg_last_detected_timestamp

# Reorg starting blocks
histogram_quantile(0.95, rate(chainindexor_reorg_from_block_bucket[1h]))
```

### Monitor Retention

```promql
# Blocks pruned rate
rate(chainindexor_retention_blocks_pruned_total[1h])

# Logs pruned rate
rate(chainindexor_retention_logs_pruned_total[1h])
```

## Integration Points

Metrics are automatically tracked by the following components:

1. **Downloader** (`internal/downloader/downloader.go`)
   - Uses: `internal/metrics` for indexing metrics
   - Uses: `internal/fetcher` for finalized block
   - Uses: `internal/reorg` for reorg tracking

2. **RPC Client** (`internal/rpc/client.go`)
   - Uses: `internal/rpc` metrics package
   - Tracks all RPC calls, errors, duration automatically

3. **LogStore** (`internal/fetcher/store/log_store.go`)
   - Uses: `internal/metrics` for database queries
   - Uses: `internal/fetcher/store` for retention metrics

4. **Maintenance Coordinator** (`internal/db/maintenance.go`)
   - Uses: `internal/db` metrics package
   - Tracks maintenance operations, WAL checkpoints, VACUUM

5. **Reorg Detector** (`internal/reorg/reorg_detector.go`)
   - Uses: `internal/reorg` metrics package
   - Tracks reorg detection events

## Grafana Dashboard

You can create a Grafana dashboard using these metrics. Recommended panels:

### Overview Dashboard

- **Last indexed block** vs **finalized block** (time series)
- **Block lag** gauge (finalized - last_indexed)
- **Indexing rate** graph (blocks/sec)
- **Component health** status panel

### Performance Dashboard

- **Block processing duration** histogram
- **RPC request latency** by method
- **Database query performance** by operation
- **Logs indexed rate** graph

### Errors & Health Dashboard

- **Error rate by component** (stacked area chart)
- **RPC errors** breakdown by type
- **Database errors** tracking
- **Component health** heatmap

### Resources Dashboard

- **Memory usage** over time (all types)
- **Goroutine count** graph
- **Database size** growth
- **Application uptime**

### Maintenance Dashboard

- **Maintenance run** frequency
- **Space reclaimed** per run
- **Maintenance duration** histogram
- **WAL checkpoints** by mode

### Reorg Dashboard

- **Reorgs detected** over time
- **Reorg depth** distribution
- **Time since last reorg**
- **Reorg starting blocks** distribution

## Performance Impact

The metrics implementation uses Prometheus client library best practices:

- Metrics stored in memory with minimal overhead
- No I/O operations for metric updates
- Automatic aggregation and efficient querying
- System metrics update only every 15 seconds

Expected overhead: < 1% CPU, < 10MB memory for typical workloads.

## Disabling Metrics

To disable metrics, set `enabled: false` in your configuration:

```yaml
metrics:
  enabled: false
```

When disabled, the metrics server will not start. However, metric recording calls throughout the code remain safe (metrics are still registered with Prometheus but not exposed via HTTP).
