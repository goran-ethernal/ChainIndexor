package indexer

import (
	"fmt"
	"strings"
	"sync"

	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
)

// Factory is a function that creates a new indexer instance.
type Factory func(cfg config.IndexerConfig, log *logger.Logger) (Indexer, error)

var (
	registry = make(map[string]Factory)
	mu       sync.RWMutex
)

// Register registers an indexer factory with the given type name.
// This is typically called in init() functions of indexer packages.
// The type name is case-insensitive and will be stored in lowercase.
func Register(indexerType string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	name := strings.ToLower(indexerType)
	if _, exists := registry[name]; exists {
		logger.GetDefaultLogger().Infof("indexer with name %s already in indexer registry."+
			"It will be overwritten.", name)
	}

	registry[name] = factory
}

// GetFactory returns the factory for the given indexer type.
// Returns nil if the type is not registered.
// The lookup is case-insensitive.
func GetFactory(indexerType string) Factory {
	mu.RLock()
	defer mu.RUnlock()
	return registry[strings.ToLower(indexerType)]
}

// ListRegistered returns a list of all registered indexer types.
func ListRegistered() []string {
	mu.RLock()
	defer mu.RUnlock()

	types := make([]string, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	return types
}

// Create creates a new indexer instance using the registered factory.
// Returns an error if the type is not registered or if creation fails.
// The type lookup is case-insensitive.
func Create(indexerType string, cfg config.IndexerConfig, log *logger.Logger) (Indexer, error) {
	factory := GetFactory(indexerType)
	if factory == nil {
		return nil, fmt.Errorf("unknown indexer type: %s (registered types: %v)", indexerType, ListRegistered())
	}

	return factory(cfg, log)
}
