package indexer

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/goran-ethernal/ChainIndexor/pkg/api"
	"github.com/goran-ethernal/ChainIndexor/pkg/rpc"
)

const (
	// Approximate number of Ethereum blocks per week (12s block time)
	BlocksPerWeek = 50400
	// Approximate number of Ethereum blocks per day (12s block time)
	BlocksPerDay = 7200
	// Approximate number of Ethereum blocks per hour (12s block time)
	BlocksPerHour = 300
)

// EventMetadata describes an event type for dynamic query handling.
type EventMetadata struct {
	Name           string       // Event name (e.g., "Transfer")
	Table          string       // Database table name (e.g., "transfers")
	EventType      reflect.Type // Reflection type for scanning
	AddressColumns []string     // Column names containing addresses
}

// CalibrationPoint represents a block number to timestamp mapping for interpolation.
type CalibrationPoint struct {
	BlockNumber uint64
	Timestamp   uint64
}

// FormatPeriodForTimestamp formats a Unix timestamp into a period string based on interval.
func FormatPeriodForTimestamp(timestamp uint64, interval string) string {
	t := time.Unix(int64(timestamp), 0).UTC()
	switch interval {
	case "hour":
		return t.Format("2006-01-02 15:00:00")
	case "week":
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	default: // day
		return t.Format("2006-01-02")
	}
}

// SampleBlockRange generates evenly distributed sample blocks across a range.
func SampleBlockRange(minBlock, maxBlock uint64) []uint64 {
	sampleBlocks := []uint64{minBlock}
	blockRange := maxBlock - minBlock

	if blockRange > 0 {
		// Add 3-5 evenly distributed sample points
		numSamples := 5
		for i := 1; i < numSamples; i++ {
			sampleBlock := minBlock + (blockRange * uint64(i) / uint64(numSamples))
			sampleBlocks = append(sampleBlocks, sampleBlock)
		}
		if maxBlock != minBlock {
			sampleBlocks = append(sampleBlocks, maxBlock)
		}
	}

	return sampleBlocks
}

// InterpolateTimestamp linearly interpolates a timestamp for a block number using calibration points.
func InterpolateTimestamp(blockNum uint64, calibrationPoints []CalibrationPoint) uint64 {
	if len(calibrationPoints) == 0 {
		return 0
	}

	// Boundary cases
	if blockNum <= calibrationPoints[0].BlockNumber {
		return calibrationPoints[0].Timestamp
	}
	if blockNum >= calibrationPoints[len(calibrationPoints)-1].BlockNumber {
		return calibrationPoints[len(calibrationPoints)-1].Timestamp
	}

	// Find bracketing points and interpolate
	for i := 0; i < len(calibrationPoints)-1; i++ {
		p1 := calibrationPoints[i]
		p2 := calibrationPoints[i+1]

		if blockNum >= p1.BlockNumber && blockNum <= p2.BlockNumber {
			blockDiff := p2.BlockNumber - p1.BlockNumber
			if blockDiff == 0 {
				return p1.Timestamp
			}

			timeDiff := int64(p2.Timestamp) - int64(p1.Timestamp)
			blockOffset := blockNum - p1.BlockNumber
			timeOffset := (timeDiff * int64(blockOffset)) / int64(blockDiff)
			return uint64(int64(p1.Timestamp) + timeOffset)
		}
	}

	return calibrationPoints[0].Timestamp
}

// BuildUnionQuery builds a UNION query across all event tables.
func BuildUnionQuery(metadata map[string]*EventMetadata, column string) string {
	parts := make([]string, 0, len(metadata))
	for _, meta := range metadata {
		parts = append(parts, fmt.Sprintf("SELECT %s FROM %s", column, meta.Table))
	}
	return strings.Join(parts, " UNION ALL ")
}

// TimeseriesPeriodData represents intermediate timeseries data from a query.
type TimeseriesPeriodData struct {
	MinBlock  uint64
	MaxBlock  uint64
	EventType string
	Count     int64
}

// TimeseriesPeriodKey groups timeseries data by period and event type.
type TimeseriesPeriodKey struct {
	Period    string
	EventType string
}

// GetBlocksPerPeriod returns the approximate number of blocks in the given interval.
func GetBlocksPerPeriod(interval string) uint64 {
	switch interval {
	case "hour":
		return BlocksPerHour
	case "week":
		return BlocksPerWeek
	default: // day
		return BlocksPerDay
	}
}

// RPCClientFromContext extracts the RPC client from context.
func RPCClientFromContext(ctx context.Context) rpc.EthClient {
	if rpcClient, ok := ctx.Value(api.RPCClientContextKey{}).(rpc.EthClient); ok {
		return rpcClient
	}
	return nil
}
