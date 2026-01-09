package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
	"github.com/russross/meddler"
)

// BaseIndexer provides generic query implementations for all event indexers.
// Embed this in your indexer struct and implement InitEventMetadata().
type BaseIndexer struct {
	log *logger.Logger
	cfg config.IndexerConfig

	DB *sql.DB
}

func NewBaseIndexer(db *sql.DB, log *logger.Logger, cfg config.IndexerConfig) *BaseIndexer {
	return &BaseIndexer{
		DB:  db,
		log: log,
		cfg: cfg,
	}
}

// MetadataProvider defines the interface for indexers to provide event metadata.
type MetadataProvider interface {
	InitEventMetadata() map[string]*EventMetadata
}

// getEventMetadata retrieves metadata for an event type.
func (b *BaseIndexer) getEventMetadata(provider MetadataProvider, eventType string) (*EventMetadata, error) {
	metadata := provider.InitEventMetadata()
	meta, ok := metadata[strings.ToLower(eventType)]
	if !ok {
		validTypes := make([]string, 0, len(metadata))
		for k := range metadata {
			validTypes = append(validTypes, k)
		}
		return nil, fmt.Errorf("unknown event type: %s (valid types: %s)", eventType, strings.Join(validTypes, ", "))
	}
	return meta, nil
}

// GetEventTypes returns the list of event type names this indexer handles.
func (b *BaseIndexer) GetEventTypes(provider MetadataProvider) []string {
	metadata := provider.InitEventMetadata()
	types := make([]string, 0, len(metadata))
	for _, meta := range metadata {
		types = append(types, meta.Name)
	}
	return types
}

// QueryEvents retrieves events based on the provided query parameters.
func (b *BaseIndexer) QueryEvents(
	ctx context.Context,
	provider MetadataProvider,
	qp indexer.QueryParams,
) (interface{}, int, error) {
	meta, err := b.getEventMetadata(provider, qp.EventType)
	if err != nil {
		return nil, 0, err
	}

	// Build query
	//nolint:gosec // Table name comes from trusted metadata, not user input
	query := "SELECT * FROM " + meta.Table
	args := []interface{}{}
	var conditions []string

	if qp.FromBlock != nil {
		conditions = append(conditions, "block_number >= ?")
		args = append(args, *qp.FromBlock)
	}
	if qp.ToBlock != nil {
		conditions = append(conditions, "block_number <= ?")
		args = append(args, *qp.ToBlock)
	}
	if qp.Address != "" && len(meta.AddressColumns) > 0 {
		addrConditions := make([]string, len(meta.AddressColumns))
		for i, col := range meta.AddressColumns {
			addrConditions[i] = col + " = ?"
			args = append(args, qp.Address)
		}
		conditions = append(conditions, "("+strings.Join(addrConditions, " OR ")+")")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Get total count
	countQuery := strings.Replace(query, "SELECT *", "SELECT COUNT(*)", 1)
	var total int
	if err := b.DB.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	// Apply sorting with whitelist to prevent SQL injection
	allowedSortColumns := map[string]bool{
		"block_number": true,
		"tx_index":     true,
		"log_index":    true,
	}

	sortBy := "block_number" // default
	if qp.SortBy != "" && allowedSortColumns[qp.SortBy] {
		sortBy = qp.SortBy
	}

	sortOrder := "DESC" // default
	if strings.ToLower(qp.SortOrder) == "asc" {
		sortOrder = "ASC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s LIMIT ? OFFSET ?", sortBy, sortOrder)
	args = append(args, qp.Limit, qp.Offset)

	// Execute query and scan using meddler
	rows, err := b.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s events: %w", meta.Name, err)
	}
	defer rows.Close()

	// Create a slice pointer to hold event pointers using reflection
	sliceType := reflect.SliceOf(meta.EventType)
	slicePtr := reflect.New(sliceType)
	slice := slicePtr.Elem()

	if err := meddler.ScanAll(rows, slicePtr.Interface()); err != nil {
		return nil, 0, fmt.Errorf("failed to scan %s events: %w", meta.Name, err)
	}

	return slice.Interface(), total, nil
}

// GetStats returns statistics about the indexed data.
func (b *BaseIndexer) GetStats(ctx context.Context, provider MetadataProvider) (interface{}, error) {
	stats := make(map[string]interface{})
	eventCounts := make(map[string]int64)
	var totalEvents int64
	var earliestBlock, latestBlock uint64

	metadata := provider.InitEventMetadata()

	// Query counts and block ranges for all events in a single pass
	for _, meta := range metadata {
		var count int64
		if err := b.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM "+meta.Table).Scan(&count); err != nil {
			return nil, fmt.Errorf("failed to get %s count: %w", meta.Name, err)
		}
		eventCounts[meta.Name] = count
		totalEvents += count

		var earliest, latest uint64
		if err := b.DB.QueryRowContext(ctx,
			"SELECT COALESCE(MIN(block_number), 0), COALESCE(MAX(block_number), 0) FROM "+meta.Table).
			Scan(&earliest, &latest); err != nil {
			return nil, fmt.Errorf("failed to get %s block range: %w", meta.Name, err)
		}

		if earliest == 0 && latest == 0 {
			continue // Skip empty tables
		}

		if earliestBlock == 0 || (earliest > 0 && earliest < earliestBlock) {
			earliestBlock = earliest
		}
		if latest > latestBlock {
			latestBlock = latest
		}
	}

	stats["total_events"] = totalEvents
	stats["event_counts"] = eventCounts
	stats["earliest_block"] = earliestBlock
	stats["latest_block"] = latestBlock

	return stats, nil
}

// QueryEventsTimeseries retrieves time-series aggregated event data.
func (b *BaseIndexer) QueryEventsTimeseries(
	ctx context.Context,
	provider MetadataProvider,
	tp indexer.TimeseriesParams,
) (interface{}, error) {
	blocksPerPeriod := GetBlocksPerPeriod(tp.Interval)

	// Build filter conditions
	var filterConditions string
	var filterArgs []interface{}
	if tp.FromBlock != nil || tp.ToBlock != nil {
		var conditions []string
		if tp.FromBlock != nil {
			conditions = append(conditions, "block_number >= ?")
			filterArgs = append(filterArgs, *tp.FromBlock)
		}
		if tp.ToBlock != nil {
			conditions = append(conditions, "block_number <= ?")
			filterArgs = append(filterArgs, *tp.ToBlock)
		}
		filterConditions = " AND " + strings.Join(conditions, " AND ")
	}

	// Aggregate events by block range buckets
	var periodResults []TimeseriesPeriodData

	// Query each event type
	metadata := provider.InitEventMetadata()
	for _, meta := range metadata {
		// Skip if filtering by event type and this isn't it
		if tp.EventType != "" && !strings.EqualFold(tp.EventType, meta.Name) {
			continue
		}

		//nolint:gosec // Table name comes from trusted metadata, not user input
		query := fmt.Sprintf(`
			SELECT 
				MIN(block_number) as min_block,
				MAX(block_number) as max_block,
				? as event_type,
				COUNT(*) as count
			FROM %s
			WHERE 1=1%s
			GROUP BY (block_number / ?)
			ORDER BY min_block ASC`, meta.Table, filterConditions)

		args := make([]interface{}, 0, len(filterArgs)+2) //nolint:mnd // +2 for event_type and blocksPerPeriod
		args = append(args, meta.Name)
		args = append(args, filterArgs...)
		args = append(args, blocksPerPeriod)

		rows, err := b.DB.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to query %s timeseries: %w", meta.Name, err)
		}

		for rows.Next() {
			var data TimeseriesPeriodData
			if err := rows.Scan(&data.MinBlock, &data.MaxBlock, &data.EventType, &data.Count); err != nil {
				rows.Close()
				return nil, err
			}
			periodResults = append(periodResults, data)
		}

		rows.Close()
	}

	if len(periodResults) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Find block range across all results
	var minBlock, maxBlock uint64
	for i, result := range periodResults {
		if i == 0 || result.MinBlock < minBlock {
			minBlock = result.MinBlock
		}
		if result.MaxBlock > maxBlock {
			maxBlock = result.MaxBlock
		}
	}

	// Sample representative blocks for timestamp calibration
	sampleBlocks := SampleBlockRange(minBlock, maxBlock)

	// Fetch headers for sample blocks only
	rpcClient := RPCClientFromContext(ctx)
	if rpcClient == nil {
		return nil, fmt.Errorf("RPC client not available in context: ensure the API/indexer server was initialized " +
			"with an RPC client and that the request context is populated via RPCClientFromContext")
	}

	headers, err := rpcClient.BatchGetBlockHeaders(ctx, sampleBlocks)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block headers: %w", err)
	}

	// Build calibration points from headers
	var calibrationPoints []CalibrationPoint
	for _, header := range headers {
		if header != nil {
			calibrationPoints = append(calibrationPoints, CalibrationPoint{
				BlockNumber: header.Number.Uint64(),
				Timestamp:   header.Time,
			})
		}
	}

	if len(calibrationPoints) == 0 {
		return nil, fmt.Errorf("failed to get any timestamp calibration points")
	}

	// Aggregate events by period with interpolated timestamps
	aggregatedData := make(map[TimeseriesPeriodKey]struct {
		Count    int64
		MinBlock uint64
		MaxBlock uint64
	})

	for _, result := range periodResults {
		midBlock := (result.MinBlock + result.MaxBlock) / 2 //nolint:mnd
		timestamp := InterpolateTimestamp(midBlock, calibrationPoints)
		period := FormatPeriodForTimestamp(timestamp, tp.Interval)

		key := TimeseriesPeriodKey{Period: period, EventType: result.EventType}
		data := aggregatedData[key]
		data.Count += result.Count

		if data.MinBlock == 0 || result.MinBlock < data.MinBlock {
			data.MinBlock = result.MinBlock
		}
		if result.MaxBlock > data.MaxBlock {
			data.MaxBlock = result.MaxBlock
		}

		aggregatedData[key] = data
	}

	// Convert to result format
	results := make([]map[string]interface{}, 0, len(aggregatedData))
	for key, data := range aggregatedData {
		results = append(results, map[string]interface{}{
			"period":     key.Period,
			"event_type": key.EventType,
			"count":      data.Count,
			"min_block":  data.MinBlock,
			"max_block":  data.MaxBlock,
		})
	}

	return results, nil
}

// GetMetrics returns performance and processing metrics.
func (b *BaseIndexer) GetMetrics(ctx context.Context, provider MetadataProvider) (interface{}, error) {
	metrics := make(map[string]interface{})

	// Build UNION query for all event tables
	metadata := provider.InitEventMetadata()
	unionQuery := BuildUnionQuery(metadata, "block_number")
	if strings.TrimSpace(unionQuery) == "" {
		// No event metadata available; return empty metrics without executing invalid SQL.
		return metrics, nil
	}

	// Calculate processing rate from recent blocks
	var (
		recentEventsCount int64
		recentBlockCount  uint64
	)

	//nolint:gosec // Union query composed from trusted metadata tables
	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as event_count,
			MAX(block_number) - MIN(block_number) + 1 as block_range
		FROM (%s) as all_events
		WHERE block_number >= (SELECT MAX(block_number) - 1000 FROM (%s))`,
		unionQuery, unionQuery)

	if err := b.DB.QueryRowContext(ctx, query).Scan(&recentEventsCount, &recentBlockCount); err != nil {
		recentEventsCount = 0
		recentBlockCount = 0
	}

	if recentBlockCount > 0 {
		metrics["events_per_block"] = float64(recentEventsCount) / float64(recentBlockCount)
	} else {
		metrics["events_per_block"] = 0.0
	}

	// Calculate average events per day based on block range
	var totalEvents int64
	if err := b.DB.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM (%s) as all_events", unionQuery),
	).Scan(&totalEvents); err != nil {
		totalEvents = 0
	}

	var totalBlocks uint64
	if err := b.DB.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT MAX(block_number) - MIN(block_number) + 1 FROM (%s)", unionQuery)).Scan(&totalBlocks); err != nil {
		totalBlocks = 0
	}

	if totalBlocks > 0 {
		estimatedDays := float64(totalBlocks) / BlocksPerDay
		if estimatedDays > 0 {
			metrics["avg_events_per_day"] = float64(totalEvents) / estimatedDays
		} else {
			metrics["avg_events_per_day"] = 0.0
		}
	} else {
		metrics["avg_events_per_day"] = 0.0
	}

	metrics["recent_blocks_analyzed"] = recentBlockCount
	metrics["recent_events_count"] = recentEventsCount

	return metrics, nil
}

// GetType returns the type identifier of the indexer.
func (b *BaseIndexer) GetType() string {
	return b.cfg.Type
}

// GetName returns the configured name of the indexer instance.
func (b *BaseIndexer) GetName() string {
	return b.cfg.Name
}

// StartBlock returns the block number from which this indexer should start.
func (b *BaseIndexer) StartBlock() uint64 {
	return b.cfg.StartBlock
}

// Close closes the database connection.
func (b *BaseIndexer) Close() error {
	if b.DB != nil {
		return b.DB.Close()
	}
	return nil
}

// HandleReorg handles a blockchain reorganization by removing data from the reorg point.
// This is generic and works with any indexer.
func (b *BaseIndexer) HandleReorg(provider MetadataProvider, blockNum uint64) error {
	metadata := provider.InitEventMetadata()
	if len(metadata) == 0 {
		return nil
	}

	// Begin transaction
	tx, err := b.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			b.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	// Delete from each event table
	for _, meta := range metadata {
		//nolint:gosec // Table name comes from trusted metadata, not user input
		query := fmt.Sprintf("DELETE FROM %s WHERE block_number >= ?", meta.Table)
		_, err := tx.Exec(query, blockNum)
		if err != nil {
			return fmt.Errorf("failed to delete from %s: %w", meta.Table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	b.log.Infof("Handled reorg from block %d", blockNum)

	return nil
}
