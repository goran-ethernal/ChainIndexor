package reorg

import "fmt"

// ReorgDetectedError is returned when a blockchain reorganization is detected.
type ReorgDetectedError struct {
	FirstReorgBlock uint64
	Details         string
}

func (e *ReorgDetectedError) Error() string {
	return fmt.Sprintf("reorg detected at block %d: %s", e.FirstReorgBlock, e.Details)
}

// NewReorgError creates a new ReorgDetectedError.
func NewReorgError(firstReorgBlock uint64, details string) error {
	return &ReorgDetectedError{
		FirstReorgBlock: firstReorgBlock,
		Details:         details,
	}
}
