package indexer

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/stretchr/testify/require"
)

// mockIndexerForFactory is a simple mock indexer for testing factory registration
type mockIndexerForFactory struct {
	name string
	typ  string
}

func (m *mockIndexerForFactory) GetName() string                   { return m.name }
func (m *mockIndexerForFactory) GetType() string                   { return m.typ }
func (m *mockIndexerForFactory) StartBlock() uint64                { return 0 }
func (m *mockIndexerForFactory) HandleLogs(logs []types.Log) error { return nil }
func (m *mockIndexerForFactory) EventsToIndex() map[common.Address]map[common.Hash]struct{} {
	return make(map[common.Address]map[common.Hash]struct{})
}
func (m *mockIndexerForFactory) HandleReorg(blockNum uint64) error { return nil }

// resetRegistry clears the factory registry for testing
func resetRegistry() {
	mu.Lock()
	defer mu.Unlock()
	registry = make(map[string]Factory)
}

func TestRegister(t *testing.T) {
	// Cannot use t.Parallel() because it modifies the global registry

	tests := []struct {
		name          string
		indexerType   string
		factory       Factory
		setupExisting func()
		validate      func(t *testing.T)
	}{
		{
			name:        "register new indexer type",
			indexerType: "test-indexer",
			factory: func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
				return &mockIndexerForFactory{name: "test", typ: "test-indexer"}, nil
			},
			validate: func(t *testing.T) {
				t.Helper()

				factory := GetFactory("test-indexer")
				require.NotNil(t, factory)

				idx, err := factory(config.IndexerConfig{}, logger.NewNopLogger())
				require.NoError(t, err)
				require.Equal(t, "test-indexer", idx.GetType())
			},
		},
		{
			name:        "register with uppercase - stored as lowercase",
			indexerType: "ERC20-Indexer",
			factory: func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
				return &mockIndexerForFactory{name: "erc20", typ: "erc20-indexer"}, nil
			},
			validate: func(t *testing.T) {
				t.Helper()

				// Should be retrievable with lowercase (how it was registered)
				factory := GetFactory("erc20-indexer")
				require.NotNil(t, factory)

				// Should also be retrievable with different casing
				factory2 := GetFactory("ERC20-INDEXER")
				require.NotNil(t, factory2)
			},
		},
		{
			name:        "overwrite existing registration",
			indexerType: "duplicate",
			factory: func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
				return &mockIndexerForFactory{name: "new", typ: "duplicate"}, nil
			},
			setupExisting: func() {
				Register("duplicate", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{name: "old", typ: "duplicate"}, nil
				})
			},
			validate: func(t *testing.T) {
				t.Helper()

				factory := GetFactory("duplicate")
				require.NotNil(t, factory)

				idx, err := factory(config.IndexerConfig{}, logger.NewNopLogger())
				require.NoError(t, err)
				require.Equal(t, "new", idx.GetName())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Can't use t.Parallel() here because we're modifying global registry
			resetRegistry()

			if tt.setupExisting != nil {
				tt.setupExisting()
			}

			Register(tt.indexerType, tt.factory)
			tt.validate(t)
		})
	}
}

func TestGetFactory(t *testing.T) {
	// Cannot use t.Parallel() because it depends on global registry state

	tests := []struct {
		name        string
		setup       func()
		indexerType string
		expectNil   bool
	}{
		{
			name: "get existing factory",
			setup: func() {
				Register("test-type", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				})
			},
			indexerType: "test-type",
			expectNil:   false,
		},
		{
			name: "get with different case",
			setup: func() {
				Register("CamelCase", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				})
			},
			indexerType: "camelcase",
			expectNil:   false,
		},
		{
			name: "get with uppercase",
			setup: func() {
				Register("lowercase", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				})
			},
			indexerType: "LOWERCASE",
			expectNil:   false,
		},
		{
			name:        "get non-existent factory",
			setup:       func() {},
			indexerType: "does-not-exist",
			expectNil:   true,
		},
		{
			name:        "get from empty registry",
			setup:       func() { resetRegistry() },
			indexerType: "any-type",
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Can't use t.Parallel() here because we're modifying global registry
			resetRegistry()
			tt.setup()

			factory := GetFactory(tt.indexerType)

			if tt.expectNil {
				require.Nil(t, factory)
			} else {
				require.NotNil(t, factory)
			}
		})
	}
}

func TestListRegistered(t *testing.T) {
	// Cannot use t.Parallel() because it depends on global registry state

	tests := []struct {
		name          string
		setup         func()
		expectedCount int
		expectedTypes []string
	}{
		{
			name: "empty registry",
			setup: func() {
				resetRegistry()
			},
			expectedCount: 0,
			expectedTypes: []string{},
		},
		{
			name: "single registered type",
			setup: func() {
				resetRegistry()
				Register("erc20", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				})
			},
			expectedCount: 1,
			expectedTypes: []string{"erc20"},
		},
		{
			name: "multiple registered types",
			setup: func() {
				resetRegistry()
				Register("erc20", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				})
				Register("erc721", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				})
				Register("erc1155", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				})
			},
			expectedCount: 3,
			expectedTypes: []string{"erc20", "erc721", "erc1155"},
		},
		{
			name: "case normalization",
			setup: func() {
				resetRegistry()
				Register("ERC20", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				})
				Register("Erc721", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				})
			},
			expectedCount: 2,
			expectedTypes: []string{"erc20", "erc721"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Can't use t.Parallel() here because we're modifying global registry
			tt.setup()

			types := ListRegistered()
			require.Len(t, types, tt.expectedCount)

			if tt.expectedCount > 0 {
				for _, expectedType := range tt.expectedTypes {
					require.Contains(t, types, expectedType)
				}
			}
		})
	}
}

func TestCreate(t *testing.T) {
	// Cannot use t.Parallel() because tests modify the global registry

	tests := []struct {
		name        string
		setup       func()
		indexerType string
		config      config.IndexerConfig
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, idx Indexer)
	}{
		{
			name: "create successful indexer",
			setup: func() {
				Register("success-type", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{
						name: cfg.Name,
						typ:  "success-type",
					}, nil
				})
			},
			indexerType: "success-type",
			config: config.IndexerConfig{
				Name: "test-indexer",
			},
			expectError: false,
			validate: func(t *testing.T, idx Indexer) {
				t.Helper()

				require.Equal(t, "test-indexer", idx.GetName())
				require.Equal(t, "success-type", idx.GetType())
			},
		},
		{
			name: "create with case-insensitive type",
			setup: func() {
				Register("CamelCase", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{typ: "camelcase"}, nil
				})
			},
			indexerType: "CAMELCASE",
			config:      config.IndexerConfig{},
			expectError: false,
			validate: func(t *testing.T, idx Indexer) {
				t.Helper()

				require.Equal(t, "camelcase", idx.GetType())
			},
		},
		{
			name:        "create with unregistered type",
			setup:       func() { resetRegistry() },
			indexerType: "unregistered",
			config:      config.IndexerConfig{},
			expectError: true,
			errorMsg:    "unknown indexer type",
		},
		{
			name: "factory returns error",
			setup: func() {
				Register("error-type", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return nil, errors.New("factory initialization failed")
				})
			},
			indexerType: "error-type",
			config:      config.IndexerConfig{},
			expectError: true,
			errorMsg:    "factory initialization failed",
		},
		{
			name: "create with config passed to factory",
			setup: func() {
				Register("config-type", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{
						name: cfg.Name,
						typ:  cfg.Type,
					}, nil
				})
			},
			indexerType: "config-type",
			config: config.IndexerConfig{
				Name: "custom-name",
				Type: "custom-type",
			},
			expectError: false,
			validate: func(t *testing.T, idx Indexer) {
				t.Helper()

				require.Equal(t, "custom-name", idx.GetName())
				require.Equal(t, "custom-type", idx.GetType())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Can't use t.Parallel() here because we're modifying global registry
			resetRegistry()
			tt.setup()

			log := logger.NewNopLogger()
			idx, err := Create(tt.indexerType, tt.config, log)

			if tt.expectError {
				require.ErrorContains(t, err, tt.errorMsg)
				require.Nil(t, idx)
			} else {
				require.NoError(t, err)
				require.NotNil(t, idx)
				if tt.validate != nil {
					tt.validate(t, idx)
				}
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	// This test verifies thread-safety of the factory registry
	// Cannot use t.Parallel() because it modifies the global registry

	resetRegistry()

	// Number of goroutines
	const numGoroutines = 10
	const numTypes = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // Register, Get, List operations

	// Concurrent registrations - use fmt.Sprintf for proper integer formatting
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			typeID := id % numTypes
			Register(
				fmt.Sprintf("type-%d", typeID),
				func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
					return &mockIndexerForFactory{}, nil
				},
			)
		}(i)
	}

	// Concurrent reads (GetFactory)
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			typeID := id % numTypes
			GetFactory(fmt.Sprintf("type-%d", typeID))
		}(i)
	}

	// Concurrent list operations
	for range numGoroutines {
		go func() {
			defer wg.Done()
			ListRegistered()
		}()
	}

	wg.Wait()

	// Verify registry is still consistent - should have exactly numTypes entries
	types := ListRegistered()
	require.Equal(t, numTypes, len(types), "Should have exactly %d types registered", numTypes)
}

func TestRegistryIsolation(t *testing.T) {
	// This test ensures that the factory registry is properly shared
	// across the package (not isolated per test)
	t.Parallel()

	resetRegistry()

	// Register in "first" context
	Register("shared-type", func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
		return &mockIndexerForFactory{typ: "shared-type"}, nil
	})

	// Verify accessible from "second" context
	factory := GetFactory("shared-type")
	require.NotNil(t, factory)

	// Verify in list
	types := ListRegistered()
	require.Contains(t, types, "shared-type")
}
