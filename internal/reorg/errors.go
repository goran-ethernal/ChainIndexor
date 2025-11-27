package reorg

import "fmt"

// ErrReorgDetected is returned when a blockchain reorganization is detected.
type ErrReorgDetected struct {
	FirstReorgBlock uint64
	Details         string
}

func (e *ErrReorgDetected) Error() string {
	return fmt.Sprintf("reorg detected at block %d: %s", e.FirstReorgBlock, e.Details)
}

// NewReorgError creates a new ErrReorgDetected error.
func NewReorgError(firstReorgBlock uint64, details string) error {
	return &ErrReorgDetected{
		FirstReorgBlock: firstReorgBlock,
		Details:         details,
	}
}
