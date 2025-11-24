package downloader

import (
	"testing"

	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/stretchr/testify/require"
)

func TestFetchMode(t *testing.T) {
	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	lf := &LogFetcher{
		mode: ModeBackfill,
		log:  log.WithComponent("test"),
	}

	require.Equal(t, ModeBackfill, lf.GetMode())

	lf.SetMode(ModeLive)
	require.Equal(t, ModeLive, lf.GetMode())
}
