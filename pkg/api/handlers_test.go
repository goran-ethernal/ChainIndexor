package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	apimocks "github.com/goran-ethernal/ChainIndexor/internal/api/mocks"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	rpcmocks "github.com/goran-ethernal/ChainIndexor/internal/rpc/mocks"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
	indexermocks "github.com/goran-ethernal/ChainIndexor/pkg/indexer/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockQueryableIndexer is a composite mock that implements both Indexer and Queryable interfaces
type mockQueryableIndexer struct {
	*indexermocks.Indexer
	*indexermocks.Queryable
}

// newMockQueryableIndexer creates a new composite mock
func newMockQueryableIndexer(t *testing.T) *mockQueryableIndexer {
	t.Helper()

	return &mockQueryableIndexer{
		Indexer:   indexermocks.NewIndexer(t),
		Queryable: indexermocks.NewQueryable(t),
	}
}

func TestRespondJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		status         int
		data           any
		expectedBody   string
		expectedStatus int
	}{
		{
			name:           "success with simple data",
			status:         http.StatusOK,
			data:           map[string]string{"message": "success"},
			expectedBody:   `{"message":"success"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "success with array",
			status:         http.StatusOK,
			data:           []string{"item1", "item2"},
			expectedBody:   `["item1","item2"]`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "success with nil",
			status:         http.StatusOK,
			data:           nil,
			expectedBody:   "null",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "error status",
			status:         http.StatusBadRequest,
			data:           map[string]string{"error": "bad request"},
			expectedBody:   `{"error":"bad request"}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			respondJSON(w, tt.status, tt.data)

			require.Equal(t, tt.expectedStatus, w.Code)
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))
			require.JSONEq(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestRespondJSON_EncodingError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	// Channel cannot be JSON encoded
	respondJSON(w, http.StatusOK, make(chan int))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "Failed to encode response")
}

func TestRespondError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		status         int
		message        string
		expectedCode   int
		expectedError  string
		expectedStatus int
	}{
		{
			name:           "bad request error",
			status:         http.StatusBadRequest,
			message:        "invalid input",
			expectedCode:   http.StatusBadRequest,
			expectedError:  "Bad Request",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "not found error",
			status:         http.StatusNotFound,
			message:        "resource not found",
			expectedCode:   http.StatusNotFound,
			expectedError:  "Not Found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "internal server error",
			status:         http.StatusInternalServerError,
			message:        "something went wrong",
			expectedCode:   http.StatusInternalServerError,
			expectedError:  "Internal Server Error",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			respondError(w, tt.status, tt.message)

			require.Equal(t, tt.expectedStatus, w.Code)
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			require.Equal(t, tt.expectedCode, response.Code)
			require.Equal(t, tt.expectedError, response.Error)
			require.Equal(t, tt.message, response.Message)
		})
	}
}

func TestParseQueryParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		queryString string
		validate    func(t *testing.T, params *indexer.QueryParams, err error)
	}{
		{
			name:        "default params",
			queryString: "",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.NoError(t, err)
				require.Equal(t, 100, params.Limit)
				require.Equal(t, 0, params.Offset)
				require.Equal(t, "", params.SortBy)
				require.Equal(t, "desc", params.SortOrder)
			},
		},
		{
			name:        "custom limit and offset",
			queryString: "limit=50&offset=100",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.NoError(t, err)
				require.Equal(t, 50, params.Limit)
				require.Equal(t, 100, params.Offset)
			},
		},
		{
			name:        "block range",
			queryString: "from_block=1000&to_block=2000",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.NoError(t, err)
				require.NotNil(t, params.FromBlock)
				require.NotNil(t, params.ToBlock)
				require.Equal(t, uint64(1000), *params.FromBlock)
				require.Equal(t, uint64(2000), *params.ToBlock)
			},
		},
		{
			name:        "address filter",
			queryString: "address=0x1234567890abcdef",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.NoError(t, err)
				require.Equal(t, "0x1234567890abcdef", params.Address)
			},
		},
		{
			name:        "event type filter",
			queryString: "event_type=Transfer",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.NoError(t, err)
				require.Equal(t, "Transfer", params.EventType)
			},
		},
		{
			name:        "sort parameters",
			queryString: "sort_by=tx_index&sort_order=asc",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.NoError(t, err)
				require.Equal(t, "tx_index", params.SortBy)
				require.Equal(t, "asc", params.SortOrder)
			},
		},
		{
			name:        "sort order uppercase",
			queryString: "sort_order=DESC",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.NoError(t, err)
				require.Equal(t, "desc", params.SortOrder)
			},
		},
		{
			name:        "all parameters",
			queryString: "limit=25&offset=50&from_block=100&to_block=200&address=0xabc&event_type=Approval&sort_by=log_index&sort_order=asc",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.NoError(t, err)
				require.Equal(t, 25, params.Limit)
				require.Equal(t, 50, params.Offset)
				require.NotNil(t, params.FromBlock)
				require.Equal(t, uint64(100), *params.FromBlock)
				require.NotNil(t, params.ToBlock)
				require.Equal(t, uint64(200), *params.ToBlock)
				require.Equal(t, "0xabc", params.Address)
				require.Equal(t, "Approval", params.EventType)
				require.Equal(t, "log_index", params.SortBy)
				require.Equal(t, "asc", params.SortOrder)
			},
		},
		{
			name:        "invalid limit - too small",
			queryString: "limit=0",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid limit")
			},
		},
		{
			name:        "invalid limit - too large",
			queryString: "limit=1001",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid limit")
			},
		},
		{
			name:        "invalid limit - not a number",
			queryString: "limit=abc",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid limit")
			},
		},
		{
			name:        "invalid offset - negative",
			queryString: "offset=-1",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid offset")
			},
		},
		{
			name:        "invalid offset - not a number",
			queryString: "offset=xyz",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid offset")
			},
		},
		{
			name:        "invalid from_block",
			queryString: "from_block=abc",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid from_block")
			},
		},
		{
			name:        "invalid to_block",
			queryString: "to_block=xyz",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid to_block")
			},
		},
		{
			name:        "invalid sort_order",
			queryString: "sort_order=invalid",
			validate: func(t *testing.T, params *indexer.QueryParams, err error) {
				t.Helper()

				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid sort_order")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/?"+tt.queryString, nil)
			params, err := parseQueryParams(req)
			tt.validate(t, params, err)
		})
	}
}

func TestHandler_ListIndexers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMocks     func(registry *apimocks.IndexerRegistry)
		expectedStatus int
		validate       func(t *testing.T, response []byte)
	}{
		{
			name: "empty indexer list",
			setupMocks: func(registry *apimocks.IndexerRegistry) {
				registry.EXPECT().ListAll().Return([]indexer.Indexer{})
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response []byte) {
				t.Helper()

				var infos []IndexerInfo
				err := json.Unmarshal(response, &infos)
				require.NoError(t, err)
				require.Empty(t, infos)
			},
		},
		{
			name: "single queryable indexer",
			setupMocks: func(registry *apimocks.IndexerRegistry) {
				mockIdx := newMockQueryableIndexer(t)
				mockIdx.Indexer.EXPECT().GetType().Return("ERC20")
				mockIdx.Indexer.EXPECT().GetName().Return("erc20-indexer")
				mockIdx.Queryable.EXPECT().GetEventTypes().Return([]string{"Transfer", "Approval"})

				registry.EXPECT().ListAll().Return([]indexer.Indexer{mockIdx})
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response []byte) {
				t.Helper()

				var infos []IndexerInfo
				err := json.Unmarshal(response, &infos)
				require.NoError(t, err)
				require.Len(t, infos, 1)

				info := infos[0]
				require.Equal(t, "ERC20", info.Type)
				require.Equal(t, "erc20-indexer", info.Name)
				require.Equal(t, []string{"Transfer", "Approval"}, info.EventTypes)
				require.Len(t, info.Endpoints, 2)
				require.Contains(t, info.Endpoints[0], "/api/v1/indexers/erc20-indexer/events")
				require.Contains(t, info.Endpoints[1], "/api/v1/indexers/erc20-indexer/stats")
			},
		},
		{
			name: "multiple queryable indexers",
			setupMocks: func(registry *apimocks.IndexerRegistry) {
				mockIdx1 := newMockQueryableIndexer(t)
				mockIdx1.Indexer.EXPECT().GetType().Return("ERC20")
				mockIdx1.Indexer.EXPECT().GetName().Return("erc20-indexer")
				mockIdx1.Queryable.EXPECT().GetEventTypes().Return([]string{"Transfer", "Approval"})

				mockIdx2 := newMockQueryableIndexer(t)
				mockIdx2.Indexer.EXPECT().GetType().Return("ERC721")
				mockIdx2.Indexer.EXPECT().GetName().Return("erc721-indexer")
				mockIdx2.Queryable.EXPECT().GetEventTypes().Return([]string{"Transfer"})

				registry.EXPECT().ListAll().Return([]indexer.Indexer{mockIdx1, mockIdx2})
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response []byte) {
				t.Helper()

				var infos []IndexerInfo
				err := json.Unmarshal(response, &infos)
				require.NoError(t, err)
				require.Len(t, infos, 2)
			},
		},
		{
			name: "non-queryable indexer filtered out",
			setupMocks: func(registry *apimocks.IndexerRegistry) {
				mockQueryableIdx := newMockQueryableIndexer(t)
				mockQueryableIdx.Indexer.EXPECT().GetType().Return("ERC20")
				mockQueryableIdx.Indexer.EXPECT().GetName().Return("erc20-indexer")
				mockQueryableIdx.Queryable.EXPECT().GetEventTypes().Return([]string{"Transfer"})

				mockNonQueryableIdx := indexermocks.NewIndexer(t)

				registry.EXPECT().ListAll().Return([]indexer.Indexer{mockQueryableIdx, mockNonQueryableIdx})
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response []byte) {
				t.Helper()

				var infos []IndexerInfo
				err := json.Unmarshal(response, &infos)
				require.NoError(t, err)
				require.Len(t, infos, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := apimocks.NewIndexerRegistry(t)
			tt.setupMocks(registry)

			log := logger.NewNopLogger()
			handler := NewHandler(registry, rpcmocks.NewEthClient(t), log)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/indexers", nil)
			w := httptest.NewRecorder()

			handler.ListIndexers(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))
			tt.validate(t, w.Body.Bytes())
		})
	}
}

// Test GetEvents
func TestHandler_GetEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		indexerName    string
		queryString    string
		setupMocks     func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer)
		expectedStatus int
		validate       func(t *testing.T, response []byte, code int)
	}{
		{
			name:           "missing indexer name",
			indexerName:    "",
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var errResp ErrorResponse
				err := json.Unmarshal(response, &errResp)
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, errResp.Code)
				require.Contains(t, errResp.Message, "indexer name is required")
			},
		},
		{
			name:        "indexer not found",
			indexerName: "nonexistent",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("nonexistent").Return(nil)
			},
			expectedStatus: http.StatusNotFound,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var errResp ErrorResponse
				err := json.Unmarshal(response, &errResp)
				require.NoError(t, err)
				require.Equal(t, http.StatusNotFound, errResp.Code)
				require.Contains(t, errResp.Message, "not found")
			},
		},
		{
			name:        "indexer not queryable",
			indexerName: "non-queryable",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				mockIdx := indexermocks.NewIndexer(t)
				registry.EXPECT().GetByName("non-queryable").Return(mockIdx)
			},
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var errResp ErrorResponse
				err := json.Unmarshal(response, &errResp)
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, errResp.Code)
				require.Contains(t, errResp.Message, "does not support querying")
			},
		},
		{
			name:        "invalid query parameters",
			indexerName: "test-indexer",
			queryString: "limit=-1",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("test-indexer").Return(idx)
			},
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var errResp ErrorResponse
				err := json.Unmarshal(response, &errResp)
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, errResp.Code)
				require.Contains(t, errResp.Message, "invalid query parameters")
			},
		},
		{
			name:        "query error",
			indexerName: "test-indexer",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("test-indexer").Return(idx)
				idx.Queryable.EXPECT().QueryEvents(mock.Anything, mock.Anything).
					Return([]map[string]any{}, 0, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var errResp ErrorResponse
				err := json.Unmarshal(response, &errResp)
				require.NoError(t, err)
				require.Equal(t, http.StatusInternalServerError, errResp.Code)
				require.Contains(t, errResp.Message, "failed to query events")
			},
		},
		{
			name:        "successful query with empty results",
			indexerName: "test-indexer",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("test-indexer").Return(idx)
				idx.Queryable.EXPECT().QueryEvents(mock.Anything, mock.Anything).
					Return([]map[string]any{}, 0, nil)
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var eventResp EventResponse
				err := json.Unmarshal(response, &eventResp)
				require.NoError(t, err)
				require.Equal(t, 0, eventResp.Pagination.Total)
				require.Equal(t, 0, eventResp.Pagination.Offset)
				require.Equal(t, 100, eventResp.Pagination.Limit)
				require.False(t, eventResp.Pagination.HasMore)
			},
		},
		{
			name:        "successful query with results",
			indexerName: "test-indexer",
			queryString: "limit=10&offset=0",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("test-indexer").Return(idx)

				events := []map[string]any{
					{"block_number": uint64(100), "event": "Transfer"},
					{"block_number": uint64(101), "event": "Approval"},
				}

				idx.Queryable.EXPECT().QueryEvents(mock.Anything, mock.MatchedBy(func(params indexer.QueryParams) bool {
					return params.Limit == 10 && params.Offset == 0
				})).Return(events, 50, nil)
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var eventResp EventResponse
				err := json.Unmarshal(response, &eventResp)
				require.NoError(t, err)
				require.Equal(t, 50, eventResp.Pagination.Total)
				require.Equal(t, 0, eventResp.Pagination.Offset)
				require.Equal(t, 10, eventResp.Pagination.Limit)
				require.True(t, eventResp.Pagination.HasMore)

				// Verify events
				eventsSlice, ok := eventResp.Events.([]any)
				require.True(t, ok)
				require.Len(t, eventsSlice, 2)
			},
		},
		{
			name:        "pagination - last page",
			indexerName: "test-indexer",
			queryString: "limit=10&offset=90",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("test-indexer").Return(idx)

				events := []map[string]any{
					{"block_number": uint64(100)},
				}

				idx.Queryable.EXPECT().QueryEvents(mock.Anything, mock.MatchedBy(func(params indexer.QueryParams) bool {
					return params.Limit == 10 && params.Offset == 90
				})).Return(events, 91, nil)
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var eventResp EventResponse
				err := json.Unmarshal(response, &eventResp)
				require.NoError(t, err)
				require.Equal(t, 91, eventResp.Pagination.Total)
				require.Equal(t, 90, eventResp.Pagination.Offset)
				require.False(t, eventResp.Pagination.HasMore) // 90 + 1 = 91, no more
			},
		},
		{
			name:        "query with filters",
			indexerName: "test-indexer",
			queryString: "address=0x123&event_type=Transfer&from_block=100&to_block=200",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("test-indexer").Return(idx)

				idx.Queryable.EXPECT().QueryEvents(mock.Anything, mock.MatchedBy(func(params indexer.QueryParams) bool {
					return params.Address == "0x123" &&
						params.EventType == "Transfer" &&
						params.FromBlock != nil && *params.FromBlock == 100 &&
						params.ToBlock != nil && *params.ToBlock == 200
				})).Return([]map[string]any{}, 0, nil)
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var eventResp EventResponse
				err := json.Unmarshal(response, &eventResp)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := apimocks.NewIndexerRegistry(t)
			mockIdx := newMockQueryableIndexer(t)
			if tt.setupMocks != nil {
				tt.setupMocks(registry, mockIdx)
			}

			log := logger.NewNopLogger()
			handler := NewHandler(registry, rpcmocks.NewEthClient(t), log)

			url := fmt.Sprintf("/api/v1/indexers/%s/events", tt.indexerName)
			if tt.queryString != "" {
				url += "?" + tt.queryString
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetPathValue("name", tt.indexerName)
			w := httptest.NewRecorder()

			handler.GetEvents(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
			tt.validate(t, w.Body.Bytes(), w.Code)
		})
	}
}

func TestHandler_GetStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		indexerName    string
		setupMocks     func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer)
		expectedStatus int
		validate       func(t *testing.T, response []byte, code int)
	}{
		{
			name:           "missing indexer name",
			indexerName:    "",
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var errResp ErrorResponse
				err := json.Unmarshal(response, &errResp)
				require.NoError(t, err)
				require.Contains(t, errResp.Message, "indexer name is required")
			},
		},
		{
			name:        "indexer not found",
			indexerName: "nonexistent",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("nonexistent").Return(nil)
			},
			expectedStatus: http.StatusNotFound,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var errResp ErrorResponse
				err := json.Unmarshal(response, &errResp)
				require.NoError(t, err)
				require.Contains(t, errResp.Message, "not found")
			},
		},
		{
			name:        "indexer not queryable",
			indexerName: "non-queryable",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				mockIdx := indexermocks.NewIndexer(t)
				registry.EXPECT().GetByName("non-queryable").Return(mockIdx)
			},
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var errResp ErrorResponse
				err := json.Unmarshal(response, &errResp)
				require.NoError(t, err)
				require.Contains(t, errResp.Message, "does not support querying")
			},
		},
		{
			name:        "get stats error",
			indexerName: "test-indexer",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("test-indexer").Return(idx)
				idx.Queryable.EXPECT().GetStats(mock.Anything).Return(nil, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var errResp ErrorResponse
				err := json.Unmarshal(response, &errResp)
				require.NoError(t, err)
				require.Contains(t, errResp.Message, "failed to get stats")
			},
		},
		{
			name:        "successful stats retrieval",
			indexerName: "test-indexer",
			setupMocks: func(registry *apimocks.IndexerRegistry, idx *mockQueryableIndexer) {
				registry.EXPECT().GetByName("test-indexer").Return(idx)

				stats := map[string]any{
					"event_count":  int64(1234),
					"latest_block": uint64(5000),
					"first_block":  uint64(1000),
				}

				idx.Queryable.EXPECT().GetStats(mock.Anything).Return(stats, nil)
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, response []byte, code int) {
				t.Helper()

				var stats map[string]any
				err := json.Unmarshal(response, &stats)
				require.NoError(t, err)
				require.Equal(t, float64(1234), stats["event_count"])
				require.Equal(t, float64(5000), stats["latest_block"])
				require.Equal(t, float64(1000), stats["first_block"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := apimocks.NewIndexerRegistry(t)
			mockIdx := newMockQueryableIndexer(t)
			if tt.setupMocks != nil {
				tt.setupMocks(registry, mockIdx)
			}

			log := logger.NewNopLogger()
			handler := NewHandler(registry, rpcmocks.NewEthClient(t), log)

			url := fmt.Sprintf("/api/v1/indexers/%s/stats", tt.indexerName)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetPathValue("name", tt.indexerName)
			w := httptest.NewRecorder()

			handler.GetStats(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
			tt.validate(t, w.Body.Bytes(), w.Code)
		})
	}
}

func TestHandler_Health(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupMocks func(registry *apimocks.IndexerRegistry)
		validate   func(t *testing.T, response []byte)
	}{
		{
			name: "no indexers",
			setupMocks: func(registry *apimocks.IndexerRegistry) {
				registry.EXPECT().ListAll().Return([]indexer.Indexer{})
			},
			validate: func(t *testing.T, response []byte) {
				t.Helper()

				var healthResp HealthResponse
				err := json.Unmarshal(response, &healthResp)
				require.NoError(t, err)
				require.Equal(t, "ok", healthResp.Status)
				require.Empty(t, healthResp.Indexers)
			},
		},
		{
			name: "single healthy indexer",
			setupMocks: func(registry *apimocks.IndexerRegistry) {
				mockIdx := newMockQueryableIndexer(t)
				mockIdx.Indexer.EXPECT().GetName().Return("test-indexer")
				mockIdx.Indexer.EXPECT().GetType().Return("ERC20")
				mockIdx.Queryable.EXPECT().GetStats(mock.Anything).Return(map[string]any{
					"latest_block": uint64(1000),
					"event_count":  int64(500),
				}, nil)

				registry.EXPECT().ListAll().Return([]indexer.Indexer{mockIdx})
			},
			validate: func(t *testing.T, response []byte) {
				t.Helper()

				var healthResp HealthResponse
				err := json.Unmarshal(response, &healthResp)
				require.NoError(t, err)
				require.Equal(t, "ok", healthResp.Status)
				require.Len(t, healthResp.Indexers, 1)

				status := healthResp.Indexers[0]
				require.Equal(t, "test-indexer", status.Name)
				require.Equal(t, "ERC20", status.Type)
				require.True(t, status.Healthy)
				require.Equal(t, uint64(1000), status.LatestBlock)
				require.Equal(t, int64(500), status.EventCount)
			},
		},
		{
			name: "indexer with error",
			setupMocks: func(registry *apimocks.IndexerRegistry) {
				mockIdx := newMockQueryableIndexer(t)
				mockIdx.Indexer.EXPECT().GetName().Return("test-indexer")
				mockIdx.Indexer.EXPECT().GetType().Return("ERC20")
				mockIdx.Queryable.EXPECT().GetStats(mock.Anything).Return(nil, errors.New("database error"))

				registry.EXPECT().ListAll().Return([]indexer.Indexer{mockIdx})
			},
			validate: func(t *testing.T, response []byte) {
				t.Helper()

				var healthResp HealthResponse
				err := json.Unmarshal(response, &healthResp)
				require.NoError(t, err)
				require.Equal(t, "ok", healthResp.Status)
				require.Len(t, healthResp.Indexers, 1)

				status := healthResp.Indexers[0]
				require.Equal(t, "test-indexer", status.Name)
				require.False(t, status.Healthy)
				require.Equal(t, uint64(0), status.LatestBlock)
				require.Equal(t, int64(0), status.EventCount)
			},
		},
		{
			name: "multiple indexers with mixed health",
			setupMocks: func(registry *apimocks.IndexerRegistry) {
				mockIdx1 := newMockQueryableIndexer(t)
				mockIdx1.Indexer.EXPECT().GetName().Return("healthy-indexer")
				mockIdx1.Indexer.EXPECT().GetType().Return("ERC20")
				mockIdx1.Queryable.EXPECT().GetStats(mock.Anything).Return(map[string]any{
					"latest_block": uint64(2000),
					"event_count":  int64(1000),
				}, nil)

				mockIdx2 := newMockQueryableIndexer(t)
				mockIdx2.Indexer.EXPECT().GetName().Return("unhealthy-indexer")
				mockIdx2.Indexer.EXPECT().GetType().Return("ERC721")
				mockIdx2.Queryable.EXPECT().GetStats(mock.Anything).Return(nil, errors.New("error"))

				registry.EXPECT().ListAll().Return([]indexer.Indexer{mockIdx1, mockIdx2})
			},
			validate: func(t *testing.T, response []byte) {
				t.Helper()

				var healthResp HealthResponse
				err := json.Unmarshal(response, &healthResp)
				require.NoError(t, err)
				require.Equal(t, "ok", healthResp.Status)
				require.Len(t, healthResp.Indexers, 2)

				// Verify first indexer is healthy
				require.True(t, healthResp.Indexers[0].Healthy)
				require.Equal(t, "healthy-indexer", healthResp.Indexers[0].Name)

				// Verify second indexer is unhealthy
				require.False(t, healthResp.Indexers[1].Healthy)
				require.Equal(t, "unhealthy-indexer", healthResp.Indexers[1].Name)
			},
		},
		{
			name: "non-queryable indexers filtered out",
			setupMocks: func(registry *apimocks.IndexerRegistry) {
				mockQueryableIdx := newMockQueryableIndexer(t)
				mockQueryableIdx.Indexer.EXPECT().GetName().Return("queryable")
				mockQueryableIdx.Indexer.EXPECT().GetType().Return("ERC20")
				mockQueryableIdx.Queryable.EXPECT().GetStats(mock.Anything).Return(map[string]any{}, nil)

				mockNonQueryableIdx := indexermocks.NewIndexer(t)

				registry.EXPECT().ListAll().Return([]indexer.Indexer{mockQueryableIdx, mockNonQueryableIdx})
			},
			validate: func(t *testing.T, response []byte) {
				t.Helper()

				var healthResp HealthResponse
				err := json.Unmarshal(response, &healthResp)
				require.NoError(t, err)
				require.Len(t, healthResp.Indexers, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := apimocks.NewIndexerRegistry(t)
			tt.setupMocks(registry)

			log := logger.NewNopLogger()
			handler := NewHandler(registry, rpcmocks.NewEthClient(t), log)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()

			handler.Health(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))
			tt.validate(t, w.Body.Bytes())
		})
	}
}
