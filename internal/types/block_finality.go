package types

import "fmt"

// BlockFinality represents the finality mode for block confirmation.
type BlockFinality string

const (
	// FinalityFinalized uses the finalized block tag (highest level of finality)
	FinalityFinalized BlockFinality = "finalized"

	// FinalitySafe uses the safe block tag (medium level of finality)
	FinalitySafe BlockFinality = "safe"

	// FinalityLatest uses the latest block tag (no finality guarantees)
	FinalityLatest BlockFinality = "latest"
)

// String returns the string representation of BlockFinality.
func (f BlockFinality) String() string {
	return string(f)
}

// IsValid checks if the BlockFinality value is valid.
func (f BlockFinality) IsValid() bool {
	switch f {
	case FinalityFinalized, FinalitySafe, FinalityLatest:
		return true
	default:
		return false
	}
}

// ParseBlockFinality parses a string into a BlockFinality type.
func ParseBlockFinality(s string) (BlockFinality, error) {
	f := BlockFinality(s)
	if !f.IsValid() {
		return "", fmt.Errorf("invalid block finality: %s (must be one of: finalized, safe, latest)", s)
	}
	return f, nil
}
