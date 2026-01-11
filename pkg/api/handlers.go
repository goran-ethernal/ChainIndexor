package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
	"github.com/goran-ethernal/ChainIndexor/pkg/rpc"
)

// RPCClientContextKey is the context key for storing RPC client (exported for use in generated code)
type RPCClientContextKey struct{}

// IndexerRegistry defines the interface for accessing registered indexers.
type IndexerRegistry interface {
	GetByName(name string) indexer.Indexer
	ListAll() []indexer.Indexer
}

// Handler handles HTTP requests for the API.
type Handler struct {
	registry IndexerRegistry
	log      *logger.Logger
	rpc      rpc.EthClient
}

// NewHandler creates a new API handler.
func NewHandler(registry IndexerRegistry, rpcClient rpc.EthClient, log *logger.Logger) *Handler {
	return &Handler{
		registry: registry,
		log:      log,
		rpc:      rpcClient,
	}
}

// ListIndexers returns a list of all registered indexers.
// @Summary List all indexers
// @Description Get a list of all registered indexers with their event types and available endpoints
// @Tags Indexers
// @Produce json
// @Success 200 {array} IndexerInfo "List of indexers"
// @Router /indexers [get]
func (h *Handler) ListIndexers(w http.ResponseWriter, r *http.Request) {
	indexers := h.registry.ListAll()

	var infos []IndexerInfo
	for _, idx := range indexers {
		if queryable, ok := idx.(indexer.Queryable); ok {
			info := IndexerInfo{
				Type:       idx.GetType(),
				Name:       idx.GetName(),
				EventTypes: queryable.GetEventTypes(),
				Endpoints: []string{
					fmt.Sprintf("/api/v1/indexers/%s/events", idx.GetName()),
					fmt.Sprintf("/api/v1/indexers/%s/stats", idx.GetName()),
				},
			}
			infos = append(infos, info)
		}
	}

	respondJSON(w, http.StatusOK, infos)
}

// GetEvents retrieves events from a specific indexer.
// @Summary Get events from an indexer
// @Description Retrieve events from a specific indexer with optional filtering, pagination, and sorting
// @Tags Events
// @Produce json
// @Param name path string true "Indexer name"
// @Param event_type query string false "Event type to filter by"
// @Param limit query int false "Maximum number of events to return" default(100)
// @Param offset query int false "Number of events to skip" default(0)
// @Param from_block query integer false "Filter events from this block number"
// @Param to_block query integer false "Filter events up to this block number"
// @Param address query string false "Filter by address (contract or participant)"
// @Param sort_by query string false "Field to sort by"
// @Param sort_order query string false "Sort order: asc or desc" Enums(asc, desc)
// @Success 200 {object} EventResponse "List of events with pagination info"
// @Failure 400 {object} ErrorResponse "Invalid parameters"
// @Failure 404 {object} ErrorResponse "Indexer not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /indexers/{name}/events [get]
func (h *Handler) GetEvents(w http.ResponseWriter, r *http.Request) {
	indexerName := r.PathValue("name")
	if indexerName == "" {
		respondError(w, http.StatusBadRequest, "indexer name is required")
		return
	}

	// Get indexer from registry
	idx := h.registry.GetByName(indexerName)
	if idx == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("indexer '%s' not found", indexerName))
		return
	}

	// Check if indexer is queryable
	queryable, ok := idx.(indexer.Queryable)
	if !ok {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("indexer '%s' does not support querying", indexerName))
		return
	}

	// Parse query parameters
	params, err := parseQueryParams(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid query parameters: %v", err))
		return
	}

	// Query events
	events, total, err := queryable.QueryEvents(r.Context(), *params)
	if err != nil {
		h.log.Errorf("Failed to query events: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to query events")
		return
	}

	// Use reflection to get length since events could be any slice type
	eventsVal := reflect.ValueOf(events)
	if eventsVal.Kind() != reflect.Slice {
		h.log.Errorf("Invalid events type returned from indexer '%s': expected slice, got %T", indexerName, events)
		respondError(w, http.StatusInternalServerError, "invalid events type returned from indexer")
		return
	}

	// Build response
	response := EventResponse{
		Events: events,
		Pagination: PaginationResult{
			Total:   total,
			Limit:   params.Limit,
			Offset:  params.Offset,
			HasMore: params.Offset+eventsVal.Len() < total,
		},
	}

	respondJSON(w, http.StatusOK, response)
}

// GetStats retrieves statistics for a specific indexer.
// @Summary Get indexer statistics
// @Description Retrieve statistics and status information for a specific indexer
// @Tags Stats
// @Produce json
// @Param name path string true "Indexer name"
// @Success 200 {object} StatsResponse "Indexer statistics"
// @Failure 400 {object} ErrorResponse "Invalid parameters"
// @Failure 404 {object} ErrorResponse "Indexer not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /indexers/{name}/stats [get]
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	indexerName := r.PathValue("name")
	if indexerName == "" {
		respondError(w, http.StatusBadRequest, "indexer name is required")
		return
	}

	// Get indexer from registry
	idx := h.registry.GetByName(indexerName)
	if idx == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("indexer '%s' not found", indexerName))
		return
	}

	// Check if indexer is queryable
	queryable, ok := idx.(indexer.Queryable)
	if !ok {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("indexer '%s' does not support querying", indexerName))
		return
	}

	// Get stats
	stats, err := queryable.GetStats(r.Context())
	if err != nil {
		h.log.Errorf("Failed to get stats: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetEventsTimeseries retrieves time-series aggregated event data.
// @Summary Get timeseries event data
// @Description Retrieve events aggregated by time periods (hour, day, or week) with event counts
// @Tags Analytics
// @Produce json
// @Param name path string true "Indexer name"
// @Param interval query string false "Time period interval" Enums(hour, day, week) default(day)
// @Param event_type query string false "Filter by specific event type"
// @Param from_block query integer false "Filter events from this block number"
// @Param to_block query integer false "Filter events up to this block number"
// @Success 200 {array} TimeseriesDataPoint "Timeseries data points with event counts"
// @Failure 400 {object} ErrorResponse "Invalid parameters"
// @Failure 404 {object} ErrorResponse "Indexer not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /indexers/{name}/events/timeseries [get]
func (h *Handler) GetEventsTimeseries(w http.ResponseWriter, r *http.Request) {
	indexerName := r.PathValue("name")
	if indexerName == "" {
		respondError(w, http.StatusBadRequest, "indexer name is required")
		return
	}

	// Get indexer from registry
	idx := h.registry.GetByName(indexerName)
	if idx == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("indexer '%s' not found", indexerName))
		return
	}

	// Check if indexer is queryable
	queryable, ok := idx.(indexer.Queryable)
	if !ok {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("indexer '%s' does not support querying", indexerName))
		return
	}

	// Parse timeseries parameters
	params, err := parseTimeseriesParams(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid query parameters: %v", err))
		return
	}

	// Add RPC client to context so generated code can access it
	ctx := context.WithValue(r.Context(), RPCClientContextKey{}, h.rpc)

	// Query timeseries data
	data, err := queryable.QueryEventsTimeseries(ctx, *params)
	if err != nil {
		h.log.Errorf("Failed to query events timeseries: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to query events timeseries")
		return
	}

	respondJSON(w, http.StatusOK, data)
}

// GetMetrics retrieves performance and processing metrics.
// @Summary Get indexer metrics
// @Description Retrieve performance and processing metrics for a specific indexer
// @Tags Metrics
// @Produce json
// @Param name path string true "Indexer name"
// @Success 200 {object} MetricsResponse "Indexer metrics"
// @Failure 400 {object} ErrorResponse "Invalid parameters"
// @Failure 404 {object} ErrorResponse "Indexer not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /indexers/{name}/metrics [get]
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	indexerName := r.PathValue("name")
	if indexerName == "" {
		respondError(w, http.StatusBadRequest, "indexer name is required")
		return
	}

	// Get indexer from registry
	idx := h.registry.GetByName(indexerName)
	if idx == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("indexer '%s' not found", indexerName))
		return
	}

	// Check if indexer is queryable
	queryable, ok := idx.(indexer.Queryable)
	if !ok {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("indexer '%s' does not support querying", indexerName))
		return
	}

	// Get metrics
	metrics, err := queryable.GetMetrics(r.Context())
	if err != nil {
		h.log.Errorf("Failed to get metrics: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to get metrics")
		return
	}

	respondJSON(w, http.StatusOK, metrics)
}

// Health returns the health status of the API and all indexers.
// @Summary Health check
// @Description Check the health status of the API and all registered indexers
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse "API and indexer health status"
// @Router /health [get]
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	indexers := h.registry.ListAll()

	var statuses []IndexerStatus
	for _, idx := range indexers {
		if queryable, ok := idx.(indexer.Queryable); ok {
			stats, err := queryable.GetStats(r.Context())
			status := IndexerStatus{
				Name:    idx.GetName(),
				Type:    idx.GetType(),
				Healthy: err == nil,
			}

			if err == nil {
				status.LatestBlock = stats.LatestBlock
				// Sum all event counts
				for _, count := range stats.EventCounts {
					status.EventCount += count
				}
			}

			statuses = append(statuses, status)
		}
	}

	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Indexers:  statuses,
	}

	respondJSON(w, http.StatusOK, response)
}

// parseQueryParams parses HTTP query parameters into QueryParams.
func parseQueryParams(r *http.Request) (*indexer.QueryParams, error) {
	params := indexer.NewDefaultQueryParams()

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 1000 {
			return params, fmt.Errorf("invalid limit: must be between 1 and 1000")
		}
		params.Limit = limit
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return params, fmt.Errorf("invalid offset: must be non-negative")
		}
		params.Offset = offset
	}

	if fromBlockStr := r.URL.Query().Get("from_block"); fromBlockStr != "" {
		fromBlock, err := strconv.ParseUint(fromBlockStr, 10, 64)
		if err != nil {
			return params, fmt.Errorf("invalid from_block")
		}
		params.FromBlock = &fromBlock
	}

	if toBlockStr := r.URL.Query().Get("to_block"); toBlockStr != "" {
		toBlock, err := strconv.ParseUint(toBlockStr, 10, 64)
		if err != nil {
			return params, fmt.Errorf("invalid to_block")
		}
		params.ToBlock = &toBlock
	}

	if address := r.URL.Query().Get("address"); address != "" {
		params.Address = address
	}

	if eventType := r.URL.Query().Get("event_type"); eventType != "" {
		params.EventType = eventType
	}

	if sortBy := r.URL.Query().Get("sort_by"); sortBy != "" {
		params.SortBy = strings.ToLower(sortBy)
	}

	if sortOrder := r.URL.Query().Get("sort_order"); sortOrder != "" {
		sortOrder = strings.ToLower(sortOrder)
		if sortOrder != "asc" && sortOrder != "desc" {
			return params, fmt.Errorf("invalid sort_order: must be 'asc' or 'desc'")
		}
		params.SortOrder = sortOrder
	}

	return params, nil
}

// parseTimeseriesParams parses HTTP query parameters for timeseries queries.
func parseTimeseriesParams(r *http.Request) (*indexer.TimeseriesParams, error) {
	params := &indexer.TimeseriesParams{
		Interval: "day", // default
	}

	if interval := r.URL.Query().Get("interval"); interval != "" {
		interval = strings.ToLower(interval)
		if interval != "hour" && interval != "day" && interval != "week" {
			return params, fmt.Errorf("invalid interval: must be 'hour', 'day', or 'week'")
		}
		params.Interval = interval
	}

	if fromBlockStr := r.URL.Query().Get("from_block"); fromBlockStr != "" {
		fromBlock, err := strconv.ParseUint(fromBlockStr, 10, 64)
		if err != nil {
			return params, fmt.Errorf("invalid from_block")
		}
		params.FromBlock = &fromBlock
	}

	if toBlockStr := r.URL.Query().Get("to_block"); toBlockStr != "" {
		toBlock, err := strconv.ParseUint(toBlockStr, 10, 64)
		if err != nil {
			return params, fmt.Errorf("invalid to_block")
		}
		params.ToBlock = &toBlock
	}

	if params.FromBlock != nil && params.ToBlock != nil && *params.FromBlock > *params.ToBlock {
		return params, fmt.Errorf("from_block cannot be greater than to_block")
	}

	if eventType := r.URL.Query().Get("event_type"); eventType != "" {
		params.EventType = eventType
	}

	return params, nil
}

// respondJSON sends a JSON response.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")

	// Encode JSON first to catch any errors before writing status
	encoded, err := json.Marshal(data)
	if err != nil {
		// Log the error but we can still set proper status since headers haven't been sent
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	// Only write status after successful encoding
	w.WriteHeader(status)

	// Write the encoded JSON
	if _, err := w.Write(encoded); err != nil {
		// Headers already sent, can only log the error
		// The partial response may have been sent to client
		return
	}
}

// respondError sends an error response.
func respondError(w http.ResponseWriter, status int, message string) {
	response := ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	}
	respondJSON(w, status, response)
}
