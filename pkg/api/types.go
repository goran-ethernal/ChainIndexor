package api

import (
	"time"

	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
)

// Re-export indexer response types for API use
type StatsResponse = indexer.StatsResponse
type TimeseriesDataPoint = indexer.TimeseriesDataPoint
type MetricsResponse = indexer.MetricsResponse

// QueryParams represents common query parameters for event retrieval.
type QueryParams struct {
	// Pagination
	Limit  int `json:"limit" form:"limit"`
	Offset int `json:"offset" form:"offset"`

	// Block range filtering
	FromBlock *uint64 `json:"from_block,omitempty" form:"from_block"`
	ToBlock   *uint64 `json:"to_block,omitempty" form:"to_block"`

	// Address filtering (contract or participant address)
	Address string `json:"address,omitempty" form:"address"`

	// Time range filtering
	FromTime *time.Time `json:"from_time,omitempty" form:"from_time"`
	ToTime   *time.Time `json:"to_time,omitempty" form:"to_time"`

	// Sorting
	SortBy    string `json:"sort_by,omitempty" form:"sort_by"`       // Field to sort by
	SortOrder string `json:"sort_order,omitempty" form:"sort_order"` // "asc" or "desc"
}

// EventResponse represents a generic event response.
// @Description Response containing events and pagination information
type EventResponse struct {
	Events     interface{}      `json:"events" description:"Array of events"`
	Pagination PaginationResult `json:"pagination" description:"Pagination metadata"`
}

// PaginationResult contains pagination metadata.
// @Description Pagination information for paginated responses
type PaginationResult struct {
	Total   int  `json:"total" example:"1000" description:"Total number of items"`
	Limit   int  `json:"limit" example:"100" description:"Items per page"`
	Offset  int  `json:"offset" example:"0" description:"Current offset"`
	HasMore bool `json:"has_more" example:"true" description:"Whether more items are available"`
}

// ErrorResponse represents an error response.
// @Description Standard error response format
type ErrorResponse struct {
	Error   string `json:"error" description:"Error type"`
	Message string `json:"message,omitempty" description:"Detailed error message"`
	Code    int    `json:"code" example:"400" description:"HTTP status code"`
}

// HealthResponse represents a health check response.
// @Description Health status of the API and all indexers
type HealthResponse struct {
	Status    string          `json:"status" example:"healthy" description:"Overall health status"`
	Timestamp time.Time       `json:"timestamp" description:"Time of health check"`
	Indexers  []IndexerStatus `json:"indexers" description:"Status of each indexer"`
}

// IndexerStatus represents the status of a single indexer.
// @Description Status information for a single indexer
type IndexerStatus struct {
	Name        string `json:"name" description:"Indexer name"`
	Type        string `json:"type" description:"Indexer type"`
	LatestBlock uint64 `json:"latest_block" example:"19500000" description:"Latest indexed block"`
	EventCount  int64  `json:"event_count" example:"150000" description:"Total events indexed"`
	Healthy     bool   `json:"healthy" example:"true" description:"Whether indexer is healthy"`
}

// IndexerInfo represents information about an available indexer.
// @Description Metadata about an available indexer
type IndexerInfo struct {
	Type       string   `json:"type" description:"Indexer type"`
	Name       string   `json:"name" description:"Indexer name"`
	EventTypes []string `json:"event_types" description:"Supported event types"`
	Endpoints  []string `json:"endpoints" description:"Available API endpoints for this indexer"`
}
