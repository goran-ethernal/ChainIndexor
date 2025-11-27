package fetcher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchModeString(t *testing.T) {
	require.Equal(t, "backfill", ModeBackfill.String())
	require.Equal(t, "live", ModeLive.String())
}
