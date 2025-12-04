package rpc

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/goran-ethernal/ChainIndexor/internal/common"
)

// IsTooManyResultsError checks if the error is an RPC "too many results" error (DataError with message in ErrorData).
func IsTooManyResultsError(err error) (bool, string) {
	if err == nil {
		return false, ""
	}

	var dataErr rpc.DataError
	if errors.As(err, &dataErr) {
		errData := fmt.Sprintf("%v", dataErr.ErrorData())
		// Match the actual error string format (single backslash for \d)
		return regexp.MustCompile(`Query returned more than \d+ results`).MatchString(errData), errData
	}

	return false, ""
}

// ParseSuggestedBlockRange attempts to extract the suggested block range from the error message.
// Returns the suggested fromBlock and toBlock, and true if successfully parsed.
// Expected format: "Query returned more than 20000 results. Try with this block range [0x7dfd25, 0x7e0fcc]."
func ParseSuggestedBlockRange(err string) (fromBlock, toBlock uint64, ok bool) {
	if err == "" {
		return 0, 0, false
	}

	// Pattern to match hex block ranges in square brackets
	re := regexp.MustCompile(`\[(0x[0-9a-fA-F]+),\s*(0x[0-9a-fA-F]+)\]`)
	matches := re.FindStringSubmatch(err)

	const expectedMatches = 3 // full match + 2 groups
	if len(matches) != expectedMatches {
		return 0, 0, false
	}

	// Parse hex strings to uint64
	from, err1 := common.ParseUint64orHex(&matches[1])
	to, err2 := common.ParseUint64orHex(&matches[2])

	if err1 != nil || err2 != nil {
		return 0, 0, false
	}

	return from, to, true
}
