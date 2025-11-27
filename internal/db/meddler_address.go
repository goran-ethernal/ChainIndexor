//nolint:dupl
package db

import (
	"database/sql"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/russross/meddler"
)

func init() {
	// Register custom meddler converter for common.Address
	meddler.Register("address", AddressMeddler{})
}

// AddressMeddler handles conversion between common.Address and database string representation.
type AddressMeddler struct{}

func (a AddressMeddler) PreRead(fieldAddr interface{}) (scanTarget interface{}, err error) {
	// Use sql.NullString to handle NULL values
	return new(sql.NullString), nil
}

func (a AddressMeddler) PostRead(fieldAddr, scanTarget interface{}) error {
	// Convert the scanned string to common.Address
	ns, ok := scanTarget.(*sql.NullString)
	if !ok {
		return fmt.Errorf("expected *sql.NullString, got %T", scanTarget)
	}

	// Handle pointer to common.Address
	if ptr, ok := fieldAddr.(**common.Address); ok {
		if !ns.Valid {
			*ptr = nil
			return nil
		}
		address := common.HexToAddress(ns.String)
		*ptr = &address
		return nil
	}

	// Handle common.Address directly
	if ptr, ok := fieldAddr.(*common.Address); ok {
		if !ns.Valid {
			*ptr = common.Address{}
			return nil
		}
		*ptr = common.HexToAddress(ns.String)
		return nil
	}

	return fmt.Errorf("expected *common.Address or **common.Address, got %T", fieldAddr)
}

func (a AddressMeddler) PreWrite(field interface{}) (saveValue interface{}, err error) {
	// Handle pointer to common.Address
	if ptr, ok := field.(*common.Address); ok {
		if ptr == nil {
			return nil, nil
		}
		return ptr.Hex(), nil
	}

	// Handle common.Address directly
	if address, ok := field.(common.Address); ok {
		return address.Hex(), nil
	}

	return nil, fmt.Errorf("expected common.Address or *common.Address, got %T", field)
}
