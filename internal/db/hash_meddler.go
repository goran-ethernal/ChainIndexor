package db

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/russross/meddler"
)

func init() {
	// Register custom meddler converter for common.Hash
	meddler.Register("hash", HashMeddler{})
}

// HashMeddler handles conversion between common.Hash and database string representation.
type HashMeddler struct{}

func (h HashMeddler) PreRead(fieldAddr interface{}) (scanTarget interface{}, err error) {
	// Provide a string pointer to scan the database value into
	return new(string), nil
}

func (h HashMeddler) PostRead(fieldAddr, scanTarget interface{}) error {
	// Convert the scanned string to common.Hash
	s := scanTarget.(*string)
	hash := common.HexToHash(*s)

	// Set the value in the field
	ptr := fieldAddr.(*common.Hash)
	*ptr = hash
	return nil
}

func (h HashMeddler) PreWrite(field interface{}) (saveValue interface{}, err error) {
	// Convert common.Hash to string for database storage
	if hash, ok := field.(common.Hash); ok {
		return hash.Hex(), nil
	}
	return "", fmt.Errorf("expected common.Hash, got %T", field)
}
