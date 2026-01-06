package common

const (
	ComponentDownloader    = "downloader"
	ComponentLogFetcher    = "log-fetcher"
	ComponentSyncManager   = "sync-manager"
	ComponentReorgDetector = "reorg-detector"
	ComponentLogStore      = "log-store"
	ComponentMaintenance   = "maintenance"
	ComponentAPI           = "api"
)

var AllComponents = map[string]struct{}{
	ComponentDownloader:    {},
	ComponentLogFetcher:    {},
	ComponentSyncManager:   {},
	ComponentReorgDetector: {},
	ComponentLogStore:      {},
	ComponentMaintenance:   {},
	ComponentAPI:           {},
}
