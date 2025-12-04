package rpc

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockDataError struct {
	data any
	msg  string
}

func (m *mockDataError) Error() string {
	return m.msg
}

func (m *mockDataError) ErrorData() any {
	return m.data
}

func TestIsTooManyResultsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantMatch bool
		wantData  string
	}{
		{
			name:      "nil error",
			err:       nil,
			wantMatch: false,
			wantData:  "",
		},
		{
			name:      "non-DataError error",
			err:       errors.New("some other error"),
			wantMatch: false,
			wantData:  "",
		},
		{
			name: "DataError with unrelated message",
			err: &mockDataError{
				data: "Some other error message",
				msg:  "Some other error message",
			},
			wantMatch: false,
			wantData:  "Some other error message",
		},
		{
			name: "DataError with too many results message",
			err: &mockDataError{
				data: "Query returned more than 20000 results. Try with this block range [0x7dfd25, 0x7e0fcc].",
				msg:  "Query returned more than 20000 results. Try with this block range [0x7dfd25, 0x7e0fcc].",
			},
			wantMatch: true,
			wantData:  "Query returned more than 20000 results. Try with this block range [0x7dfd25, 0x7e0fcc].",
		},
		{
			name: "DataError with similar but not matching message",
			err: &mockDataError{
				data: "Query returned less than 20000 results.",
				msg:  "Query returned less than 20000 results.",
			},
			wantMatch: false,
			wantData:  "Query returned less than 20000 results.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotMatch, gotData := IsTooManyResultsError(tt.err)

			require.Equal(t, tt.wantData, gotData)
			require.Equal(t, tt.wantMatch, gotMatch)
		})
	}
}

func TestParseSuggestedBlockRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		errMsg   string
		wantFrom uint64
		wantTo   uint64
		wantOK   bool
	}{
		{
			name:     "empty error string",
			errMsg:   "",
			wantFrom: 0,
			wantTo:   0,
			wantOK:   false,
		},
		{
			name:     "no block range in error",
			errMsg:   "Query returned more than 20000 results.",
			wantFrom: 0,
			wantTo:   0,
			wantOK:   false,
		},
		{
			name:     "valid block range",
			errMsg:   "Query returned more than 20000 results. Try with this block range [0x7dfd25, 0x7e0fcc].",
			wantFrom: 8256805,
			wantTo:   8261580,
			wantOK:   true,
		},
		{
			name:     "valid block range with extra spaces",
			errMsg:   "Try with this block range [0x1aBc,   0x2DEF].",
			wantFrom: 6844,
			wantTo:   11759,
			wantOK:   true,
		},
		{
			name:     "invalid hex in block range",
			errMsg:   "Try with this block range [0xZZZZ, 0x1234].",
			wantFrom: 0,
			wantTo:   0,
			wantOK:   false,
		},
		{
			name:     "missing block range brackets",
			errMsg:   "Try with this block range 0x1234, 0x5678.",
			wantFrom: 0,
			wantTo:   0,
			wantOK:   false,
		},
		{
			name:     "multiple block ranges, only first is parsed",
			errMsg:   "Try with these ranges [0x10, 0x20] and [0x30, 0x40].",
			wantFrom: 16,
			wantTo:   32,
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			from, to, ok := ParseSuggestedBlockRange(tt.errMsg)

			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.wantFrom, from)
			require.Equal(t, tt.wantTo, to)
		})
	}
}
