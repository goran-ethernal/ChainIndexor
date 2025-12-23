package downloader

import (
	"context"

	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
)

// Downloader defines the interface for downloading and streaming blockchain logs.
type Downloader interface {
	// RegisterIndexer registers an indexer to receive logs.
	// The downloader will use the indexer's EventsToIndex method to determine
	// which logs to fetch and forward.
	RegisterIndexer(indexer indexer.Indexer)

	// Download starts the download process, streaming logs to registered indexers.
	// It continues until the context is cancelled or an error occurs.
	Download(ctx context.Context) error

	// Close gracefully stops the downloader, ensuring all resources are cleaned up.
	Close() error
}
