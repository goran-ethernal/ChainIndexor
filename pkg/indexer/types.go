package indexer

// QueryParams represents common query parameters for event retrieval.
type QueryParams struct {
	// Event type to query (e.g., "Transfer", "Approval")
	EventType string

	// Pagination
	Limit  int
	Offset int

	// Block range filtering
	FromBlock *uint64
	ToBlock   *uint64

	// Address filtering
	Address string

	// Sorting
	SortBy    string
	SortOrder string // "asc" or "desc"
}

func NewDefaultQueryParams() *QueryParams {
	return &QueryParams{
		Limit:     defaultPageLimit,
		Offset:    0,
		SortOrder: "desc",
	}
}

// TimeseriesParams represents parameters for time-series queries.
type TimeseriesParams struct {
	// Interval for aggregation: "hour", "day", "week"
	Interval string

	// Block range filtering
	FromBlock *uint64
	ToBlock   *uint64

	// Event type filtering
	EventType string
}

// StatsResponse represents indexer statistics.
// @Description Statistics and status information for an indexer
type StatsResponse struct {
	TotalEvents   int64            `json:"total_events" example:"150000" description:"Total number of events indexed"`
	EventCounts   map[string]int64 `json:"event_counts" description:"Event count breakdown by event type"`
	EarliestBlock uint64           `json:"earliest_block" example:"19000000" description:"Earliest block number processed"`
	LatestBlock   uint64           `json:"latest_block" example:"19500000" description:"Latest block number processed"`
}

// TimeseriesDataPoint represents a single point in timeseries data.
// @Description A data point in a timeseries response
type TimeseriesDataPoint struct {
	Period    string `json:"period" example:"2024-01-15" description:"Time period (ISO 8601 format)"`
	EventType string `json:"event_type" example:"Transfer" description:"Event type"`
	Count     int64  `json:"count" example:"1250" description:"Number of events in this period"`
	MinBlock  uint64 `json:"min_block" example:"19500000" description:"Minimum block number in period"`
	MaxBlock  uint64 `json:"max_block" example:"19510000" description:"Maximum block number in period"`
}

// MetricsResponse represents performance and processing metrics.
// @Description Performance metrics for an indexer
type MetricsResponse struct {
	EventsPerBlock       float64 `json:"events_per_block" example:"12.5" description:"Average events per block"`
	AvgEventsPerDay      float64 `json:"avg_events_per_day" example:"1250.5" description:"Average events per day"`
	RecentBlocksAnalyzed uint64  `json:"recent_blocks_analyzed" example:"1000" description:"Number of recent blocks analyzed"`
	RecentEventsCount    int64   `json:"recent_events_count" example:"12500" description:"Event count in recent blocks"`
}
