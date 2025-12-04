package common

import (
	"testing"
)

func TestParseUint64orHex(t *testing.T) {
	tests := []struct {
		name    string
		input   *string
		want    uint64
		wantErr bool
	}{
		{
			name:    "nil input",
			input:   nil,
			want:    0,
			wantErr: false,
		},
		{
			name:    "decimal string",
			input:   strPtr("12345"),
			want:    12345,
			wantErr: false,
		},
		{
			name:    "hex string with 0x prefix",
			input:   strPtr("0x1a2b"),
			want:    0x1a2b,
			wantErr: false,
		},
		{
			name:    "hex string with 0x prefix and uppercase",
			input:   strPtr("0xDEADBEEF"),
			want:    0xDEADBEEF,
			wantErr: false,
		},
		{
			name:    "invalid decimal string",
			input:   strPtr("12abc"),
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid hex string",
			input:   strPtr("0xGHIJK"),
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   strPtr(""),
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUint64orHex(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseUint64orHex() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseUint64orHex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
