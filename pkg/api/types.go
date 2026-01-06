package api

import "time"

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
type EventResponse struct {
	Events     interface{}      `json:"events"`
	Pagination PaginationResult `json:"pagination"`
}

// PaginationResult contains pagination metadata.
type PaginationResult struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status    string          `json:"status"`
	Timestamp time.Time       `json:"timestamp"`
	Indexers  []IndexerStatus `json:"indexers"`
}

// IndexerStatus represents the status of a single indexer.
type IndexerStatus struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	LatestBlock uint64 `json:"latest_block"`
	EventCount  int64  `json:"event_count"`
	Healthy     bool   `json:"healthy"`
}

// IndexerInfo represents information about an available indexer.
type IndexerInfo struct {
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	EventTypes []string `json:"event_types"`
	Endpoints  []string `json:"endpoints"`
}
