package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlockFinality(t *testing.T) {
	tests := []struct {
		name      string
		finality  BlockFinality
		wantValid bool
		wantStr   string
	}{
		{
			name:      "finalized",
			finality:  FinalityFinalized,
			wantValid: true,
			wantStr:   "finalized",
		},
		{
			name:      "safe",
			finality:  FinalitySafe,
			wantValid: true,
			wantStr:   "safe",
		},
		{
			name:      "latest",
			finality:  FinalityLatest,
			wantValid: true,
			wantStr:   "latest",
		},
		{
			name:      "invalid",
			finality:  BlockFinality("invalid"),
			wantValid: false,
			wantStr:   "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantValid, tt.finality.IsValid())
			require.Equal(t, tt.wantStr, tt.finality.String())
		})
	}
}

func TestParseBlockFinality(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      BlockFinality
		wantError bool
	}{
		{
			name:      "finalized",
			input:     "finalized",
			want:      FinalityFinalized,
			wantError: false,
		},
		{
			name:      "safe",
			input:     "safe",
			want:      FinalitySafe,
			wantError: false,
		},
		{
			name:      "latest",
			input:     "latest",
			want:      FinalityLatest,
			wantError: false,
		},
		{
			name:      "invalid",
			input:     "invalid",
			want:      "",
			wantError: true,
		},
		{
			name:      "empty",
			input:     "",
			want:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBlockFinality(tt.input)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}
