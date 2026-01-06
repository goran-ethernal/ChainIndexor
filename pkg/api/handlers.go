package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
)

// IndexerRegistry defines the interface for accessing registered indexers.
type IndexerRegistry interface {
	GetByName(name string) indexer.Indexer
	ListAll() []indexer.Indexer
}

// Handler handles HTTP requests for the API.
type Handler struct {
	registry IndexerRegistry
	log      *logger.Logger
}

// NewHandler creates a new API handler.
func NewHandler(registry IndexerRegistry, log *logger.Logger) *Handler {
	return &Handler{
		registry: registry,
		log:      log,
	}
}

// ListIndexers returns a list of all registered indexers.
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

// Health returns the health status of the API and all indexers.
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
				if statsMap, ok := stats.(map[string]any); ok {
					if latestBlock, ok := statsMap["latest_block"].(uint64); ok {
						status.LatestBlock = latestBlock
					}
					if eventCount, ok := statsMap["event_count"].(int64); ok {
						status.EventCount = eventCount
					}
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
