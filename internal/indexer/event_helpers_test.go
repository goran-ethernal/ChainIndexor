package indexer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFormatPeriodForTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		timestamp uint64
		interval  string
		expected  string
	}{
		{
			name:      "FormatDay",
			timestamp: 1705276800, // Jan 15, 2024 00:00:00 UTC
			interval:  "day",
			expected:  "01/15/2024",
		},
		{
			name:      "FormatHour",
			timestamp: 1705276800, // Jan 15, 2024 00:00:00 UTC
			interval:  "hour",
			expected:  "01/15/2024 00:00:00",
		},
		{
			name:      "FormatWeek",
			timestamp: 1705276800, // Jan 15, 2024 (week 3)
			interval:  "week",
			expected:  "2024-W03",
		},
		{
			name:      "DefaultToDay",
			timestamp: 1705276800,
			interval:  "unknown",
			expected:  "01/15/2024",
		},
		{
			name:      "FormatDayWithTime",
			timestamp: 1705320000, // Jan 15, 2024 12:00:00 UTC
			interval:  "day",
			expected:  "01/15/2024",
		},
		{
			name:      "FormatHourWithDifferentTime",
			timestamp: 1705320000, // Jan 15, 2024 12:00:00 UTC
			interval:  "hour",
			expected:  "01/15/2024 12:00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := FormatPeriodForTimestamp(tt.timestamp, tt.interval)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSampleBlockRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		minBlock  uint64
		maxBlock  uint64
		checkFunc func(samples []uint64)
	}{
		{
			name:     "SameBlock",
			minBlock: 1000,
			maxBlock: 1000,
			checkFunc: func(samples []uint64) {
				require.Equal(t, []uint64{1000}, samples)
			},
		},
		{
			name:     "SmallRange",
			minBlock: 1000,
			maxBlock: 1010,
			checkFunc: func(samples []uint64) {
				require.Greater(t, len(samples), 2, "should have at least min and max")
				require.Equal(t, uint64(1000), samples[0], "first should be min")
				require.Equal(t, uint64(1010), samples[len(samples)-1], "last should be max")
				// Check samples are in ascending order
				for i := 1; i < len(samples); i++ {
					require.Greater(t, samples[i], samples[i-1], "samples should be ascending")
				}
			},
		},
		{
			name:     "LargeRange",
			minBlock: 1000,
			maxBlock: 10000,
			checkFunc: func(samples []uint64) {
				// Should have min, max, and intermediate points
				require.GreaterOrEqual(t, len(samples), 3)
				require.Equal(t, uint64(1000), samples[0], "first should be min")
				require.Equal(t, uint64(10000), samples[len(samples)-1], "last should be max")
				// Check samples are distributed
				require.Greater(t, samples[2], samples[1], "intermediate points should be spread")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			samples := SampleBlockRange(tt.minBlock, tt.maxBlock)
			tt.checkFunc(samples)
		})
	}
}

func TestInterpolateTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		blockNum          uint64
		calibrationPoints []CalibrationPoint
		expectedTimestamp uint64
		description       string
	}{
		{
			name:              "EmptyCalibrationPoints",
			blockNum:          1000,
			calibrationPoints: []CalibrationPoint{},
			expectedTimestamp: 0,
			description:       "should return 0 for empty calibration points",
		},
		{
			name:     "BlockBeforeFirstPoint",
			blockNum: 900,
			calibrationPoints: []CalibrationPoint{
				{BlockNumber: 1000, Timestamp: 1000},
				{BlockNumber: 2000, Timestamp: 2000},
			},
			expectedTimestamp: 1000,
			description:       "should return first point's timestamp if block is before all points",
		},
		{
			name:     "BlockAfterLastPoint",
			blockNum: 3000,
			calibrationPoints: []CalibrationPoint{
				{BlockNumber: 1000, Timestamp: 1000},
				{BlockNumber: 2000, Timestamp: 2000},
			},
			expectedTimestamp: 2000,
			description:       "should return last point's timestamp if block is after all points",
		},
		{
			name:     "BlockAtFirstPoint",
			blockNum: 1000,
			calibrationPoints: []CalibrationPoint{
				{BlockNumber: 1000, Timestamp: 1000},
				{BlockNumber: 2000, Timestamp: 2000},
			},
			expectedTimestamp: 1000,
			description:       "should return exact timestamp at first point",
		},
		{
			name:     "BlockAtLastPoint",
			blockNum: 2000,
			calibrationPoints: []CalibrationPoint{
				{BlockNumber: 1000, Timestamp: 1000},
				{BlockNumber: 2000, Timestamp: 2000},
			},
			expectedTimestamp: 2000,
			description:       "should return exact timestamp at last point",
		},
		{
			name:     "LinearInterpolation",
			blockNum: 1500,
			calibrationPoints: []CalibrationPoint{
				{BlockNumber: 1000, Timestamp: 1000},
				{BlockNumber: 2000, Timestamp: 3000},
			},
			expectedTimestamp: 2000,
			description:       "should interpolate linearly between two points",
		},
		{
			name:     "LinearInterpolationQuarter",
			blockNum: 1250,
			calibrationPoints: []CalibrationPoint{
				{BlockNumber: 1000, Timestamp: 1000},
				{BlockNumber: 2000, Timestamp: 3000},
			},
			expectedTimestamp: 1500,
			description:       "should interpolate at 1/4 position",
		},
		{
			name:     "MultipleCalibrationPoints",
			blockNum: 2500,
			calibrationPoints: []CalibrationPoint{
				{BlockNumber: 1000, Timestamp: 1000},
				{BlockNumber: 2000, Timestamp: 2000},
				{BlockNumber: 3000, Timestamp: 4000},
			},
			expectedTimestamp: 3000,
			description:       "should interpolate between correct bracketing points",
		},
		{
			name:     "ConsecutiveInterpolation",
			blockNum: 1250,
			calibrationPoints: []CalibrationPoint{
				{BlockNumber: 1000, Timestamp: 1000},
				{BlockNumber: 1500, Timestamp: 2000},
				{BlockNumber: 2000, Timestamp: 3000},
			},
			expectedTimestamp: 1500,
			description:       "should interpolate correctly with multiple non-overlapping calibration points",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := InterpolateTimestamp(tt.blockNum, tt.calibrationPoints)
			require.Equal(t, tt.expectedTimestamp, result, tt.description)
		})
	}
}

func TestBuildUnionQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		metadata  map[string]*EventMetadata
		column    string
		checkFunc func(query string)
	}{
		{
			name:     "EmptyMetadata",
			metadata: map[string]*EventMetadata{},
			column:   "block_number",
			checkFunc: func(query string) {
				require.Equal(t, "", query, "should return empty string for empty metadata")
			},
		},
		{
			name: "SingleTable",
			metadata: map[string]*EventMetadata{
				"transfer": {
					Name:  "Transfer",
					Table: "transfers",
				},
			},
			column: "block_number",
			checkFunc: func(query string) {
				require.Equal(t, "SELECT block_number FROM transfers", query)
			},
		},
		{
			name: "MultipleTables",
			metadata: map[string]*EventMetadata{
				"transfer": {
					Name:  "Transfer",
					Table: "transfers",
				},
				"approval": {
					Name:  "Approval",
					Table: "approvals",
				},
			},
			column: "id",
			checkFunc: func(query string) {
				// Should contain both tables
				require.Contains(t, query, "SELECT id FROM transfers")
				require.Contains(t, query, "SELECT id FROM approvals")
				require.Contains(t, query, "UNION ALL")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			query := BuildUnionQuery(tt.metadata, tt.column)
			tt.checkFunc(query)
		})
	}
}

func TestGetBlocksPerPeriod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		interval string
		expected uint64
	}{
		{
			name:     "Hour",
			interval: "hour",
			expected: BlocksPerHour,
		},
		{
			name:     "Day",
			interval: "day",
			expected: BlocksPerDay,
		},
		{
			name:     "Week",
			interval: "week",
			expected: BlocksPerWeek,
		},
		{
			name:     "UnknownDefaultToDay",
			interval: "month",
			expected: BlocksPerDay,
		},
		{
			name:     "EmptyDefaultToDay",
			interval: "",
			expected: BlocksPerDay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetBlocksPerPeriod(tt.interval)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatPeriodForTimestampEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("Epoch", func(t *testing.T) {
		t.Parallel()
		result := FormatPeriodForTimestamp(0, "day")
		require.Equal(t, "01/01/1970", result)
	})

	t.Run("LeapYear", func(t *testing.T) {
		t.Parallel()
		// Feb 29, 2024 (leap year)
		leapYearTimestamp := uint64(time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC).Unix())
		result := FormatPeriodForTimestamp(leapYearTimestamp, "day")
		require.Equal(t, "02/29/2024", result)
	})

	t.Run("EndOfYear", func(t *testing.T) {
		t.Parallel()
		// Dec 31, 2024
		endOfYearTimestamp := uint64(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC).Unix())
		result := FormatPeriodForTimestamp(endOfYearTimestamp, "day")
		require.Equal(t, "12/31/2024", result)
	})

	t.Run("WeekBoundary", func(t *testing.T) {
		t.Parallel()
		// Monday start of week 1, 2024
		mondayTimestamp := uint64(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
		result := FormatPeriodForTimestamp(mondayTimestamp, "week")
		require.Equal(t, "2024-W01", result)
	})
}

func TestInterpolateTimestampPrecision(t *testing.T) {
	t.Parallel()

	t.Run("NoRounding", func(t *testing.T) {
		t.Parallel()

		calibrationPoints := []CalibrationPoint{
			{BlockNumber: 1000, Timestamp: 1000},
			{BlockNumber: 3000, Timestamp: 5000},
		}
		// At block 2000, should be exactly at 3000
		result := InterpolateTimestamp(2000, calibrationPoints)
		require.Equal(t, uint64(3000), result)
	})

	t.Run("IntegerDivision", func(t *testing.T) {
		t.Parallel()

		calibrationPoints := []CalibrationPoint{
			{BlockNumber: 100, Timestamp: 1000},
			{BlockNumber: 200, Timestamp: 1100},
		}
		// At block 150, should interpolate correctly
		result := InterpolateTimestamp(150, calibrationPoints)
		// (1100-1000) * 50 / 100 + 1000 = 50 + 1000 = 1050
		require.Equal(t, uint64(1050), result)
	})
}

func TestSampleBlockRangeDistribution(t *testing.T) {
	t.Parallel()

	t.Run("EvenDistribution", func(t *testing.T) {
		t.Parallel()

		samples := SampleBlockRange(0, 1000)
		// Should have min and max plus intermediate points
		require.GreaterOrEqual(t, len(samples), 3)
		// Check roughly even spacing (not exact due to integer division)
		if len(samples) >= 3 {
			spacing1 := samples[1] - samples[0]
			spacing2 := samples[2] - samples[1]
			// Spacings should be similar (allowing for rounding)
			require.InDelta(t, float64(spacing1), float64(spacing2), float64(spacing1)*0.1)
		}
	})

	t.Run("LargeRange", func(t *testing.T) {
		t.Parallel()

		samples := SampleBlockRange(1000000, 9000000)
		// Should have multiple intermediate samples
		require.Greater(t, len(samples), 5)
		// First and last should be min and max
		require.Equal(t, uint64(1000000), samples[0])
		require.Equal(t, uint64(9000000), samples[len(samples)-1])
	})
}
