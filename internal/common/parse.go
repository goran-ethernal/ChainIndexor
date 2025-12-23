package common

import (
	"strconv"
	"strings"
)

// ParseUint64orHex converts the given uint64 string into the number.
// It can parse the string with 0x prefix as well.
func ParseUint64orHex(val *string) (uint64, error) {
	if val == nil {
		return 0, nil
	}

	str := *val
	base := 10

	if strings.HasPrefix(str, "0x") {
		str = str[2:]
		base = 16
	}

	return strconv.ParseUint(str, base, 64)
}

const bytesInMB = 1024 * 1024

func MBToBytes(mb uint64) uint64 {
	return mb * bytesInMB
}

func BytesToMB(bytes uint64) uint64 {
	return bytes / bytesInMB
}

func ToLowerWithTrim(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
