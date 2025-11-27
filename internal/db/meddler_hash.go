//nolint:dupl
package db

import (
	"database/sql"
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
	// Use sql.NullString to handle NULL values
	return new(sql.NullString), nil
}

func (h HashMeddler) PostRead(fieldAddr, scanTarget interface{}) error {
	// Convert the scanned string to common.Hash
	ns, ok := scanTarget.(*sql.NullString)
	if !ok {
		return fmt.Errorf("expected *sql.NullString, got %T", scanTarget)
	}

	// Handle pointer to common.Hash
	if ptr, ok := fieldAddr.(**common.Hash); ok {
		if !ns.Valid {
			*ptr = nil
			return nil
		}
		hash := common.HexToHash(ns.String)
		*ptr = &hash
		return nil
	}

	// Handle common.Hash directly
	if ptr, ok := fieldAddr.(*common.Hash); ok {
		if !ns.Valid {
			*ptr = common.Hash{}
			return nil
		}
		*ptr = common.HexToHash(ns.String)
		return nil
	}

	return fmt.Errorf("expected *common.Hash or **common.Hash, got %T", fieldAddr)
}

func (h HashMeddler) PreWrite(field interface{}) (saveValue interface{}, err error) {
	// Handle pointer to common.Hash
	if ptr, ok := field.(*common.Hash); ok {
		if ptr == nil {
			return nil, nil
		}
		return ptr.Hex(), nil
	}

	// Handle common.Hash directly
	if hash, ok := field.(common.Hash); ok {
		return hash.Hex(), nil
	}

	return nil, fmt.Errorf("expected common.Hash or *common.Hash, got %T", field)
}
