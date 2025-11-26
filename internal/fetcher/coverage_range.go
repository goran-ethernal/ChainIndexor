package fetcher

// CoverageRange represents a block range that has been downloaded and stored.
type CoverageRange struct {
	FromBlock uint64
	ToBlock   uint64
}

// IsCovered checks if the entire range [from, to] is covered by the coverage ranges.
func IsCovered(from, to uint64, coverage []CoverageRange) bool {
	if len(coverage) == 0 {
		return false
	}

	// Sort and merge overlapping ranges for accurate coverage check
	for _, r := range coverage {
		if r.FromBlock <= from && r.ToBlock >= to {
			return true
		}
	}

	return false
}

// GetMissingRanges returns the block ranges that are not covered by the given coverage.
// This is useful for determining which ranges still need to be fetched from the RPC node.
func GetMissingRanges(from, to uint64, coverage []CoverageRange) []CoverageRange {
	if len(coverage) == 0 {
		return []CoverageRange{{FromBlock: from, ToBlock: to}}
	}

	var missing []CoverageRange
	currentStart := from

	for _, r := range coverage {
		// If there's a gap before this range
		if r.FromBlock > currentStart {
			missing = append(missing, CoverageRange{
				FromBlock: currentStart,
				ToBlock:   min(r.FromBlock-1, to),
			})
		}

		// Move past this covered range
		if r.ToBlock >= currentStart {
			currentStart = r.ToBlock + 1
		}

		// If we've covered the entire requested range, we're done
		if currentStart > to {
			break
		}
	}

	// If there's a gap after the last range
	if currentStart <= to {
		missing = append(missing, CoverageRange{
			FromBlock: currentStart,
			ToBlock:   to,
		})
	}

	return missing
}
