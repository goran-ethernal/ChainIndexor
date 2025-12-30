package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEventSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		want      *EventSignature
		wantErr   bool
	}{
		{
			name:      "ERC20 Transfer - canonical form",
			signature: "Transfer(address,address,uint256)",
			want: &EventSignature{
				Raw:  "Transfer(address,address,uint256)",
				Name: "Transfer",
				Params: []EventParam{
					{Name: "param0", Type: "address", Indexed: false},
					{Name: "param1", Type: "address", Indexed: false},
					{Name: "param2", Type: "uint256", Indexed: false},
				},
			},
			wantErr: false,
		},
		{
			name:      "ERC20 Transfer - with names",
			signature: "Transfer(address from, address to, uint256 value)",
			want: &EventSignature{
				Raw:  "Transfer(address from, address to, uint256 value)",
				Name: "Transfer",
				Params: []EventParam{
					{Name: "from", Type: "address", Indexed: false},
					{Name: "to", Type: "address", Indexed: false},
					{Name: "value", Type: "uint256", Indexed: false},
				},
			},
			wantErr: false,
		},
		{
			name:      "ERC20 Transfer - with indexed",
			signature: "Transfer(address indexed from, address indexed to, uint256 value)",
			want: &EventSignature{
				Raw:  "Transfer(address indexed from, address indexed to, uint256 value)",
				Name: "Transfer",
				Params: []EventParam{
					{Name: "from", Type: "address", Indexed: true},
					{Name: "to", Type: "address", Indexed: true},
					{Name: "value", Type: "uint256", Indexed: false},
				},
			},
			wantErr: false,
		},
		{
			name:      "ERC721 Transfer",
			signature: "Transfer(address indexed from, address indexed to, uint256 indexed tokenId)",
			want: &EventSignature{
				Raw:  "Transfer(address indexed from, address indexed to, uint256 indexed tokenId)",
				Name: "Transfer",
				Params: []EventParam{
					{Name: "from", Type: "address", Indexed: true},
					{Name: "to", Type: "address", Indexed: true},
					{Name: "tokenId", Type: "uint256", Indexed: true},
				},
			},
			wantErr: false,
		},
		{
			name:      "No parameters",
			signature: "Initialized()",
			want: &EventSignature{
				Raw:    "Initialized()",
				Name:   "Initialized",
				Params: []EventParam{},
			},
			wantErr: false,
		},
		{
			name:      "Different types",
			signature: "ComplexEvent(address indexed owner, bool enabled, bytes32 hash, string name)",
			want: &EventSignature{
				Raw:  "ComplexEvent(address indexed owner, bool enabled, bytes32 hash, string name)",
				Name: "ComplexEvent",
				Params: []EventParam{
					{Name: "owner", Type: "address", Indexed: true},
					{Name: "enabled", Type: "bool", Indexed: false},
					{Name: "hash", Type: "bytes32", Indexed: false},
					{Name: "name", Type: "string", Indexed: false},
				},
			},
			wantErr: false,
		},
		{
			name:      "Array types",
			signature: "MultiTransfer(address[] recipients, uint256[] amounts)",
			want: &EventSignature{
				Raw:  "MultiTransfer(address[] recipients, uint256[] amounts)",
				Name: "MultiTransfer",
				Params: []EventParam{
					{Name: "recipients", Type: "address[]", Indexed: false},
					{Name: "amounts", Type: "uint256[]", Indexed: false},
				},
			},
			wantErr: false,
		},
		{
			name:      "Fixed-size array",
			signature: "FixedArray(uint256[10] values)",
			want: &EventSignature{
				Raw:  "FixedArray(uint256[10] values)",
				Name: "FixedArray",
				Params: []EventParam{
					{Name: "values", Type: "uint256[10]", Indexed: false},
				},
			},
			wantErr: false,
		},
		{
			name:      "Various uint sizes",
			signature: "UintSizes(uint8 a, uint16 b, uint32 c, uint64 d, uint128 e, uint256 f)",
			want: &EventSignature{
				Raw:  "UintSizes(uint8 a, uint16 b, uint32 c, uint64 d, uint128 e, uint256 f)",
				Name: "UintSizes",
				Params: []EventParam{
					{Name: "a", Type: "uint8", Indexed: false},
					{Name: "b", Type: "uint16", Indexed: false},
					{Name: "c", Type: "uint32", Indexed: false},
					{Name: "d", Type: "uint64", Indexed: false},
					{Name: "e", Type: "uint128", Indexed: false},
					{Name: "f", Type: "uint256", Indexed: false},
				},
			},
			wantErr: false,
		},
		{
			name:      "Bytes types",
			signature: "BytesTypes(bytes data, bytes32 hash, bytes4 selector)",
			want: &EventSignature{
				Raw:  "BytesTypes(bytes data, bytes32 hash, bytes4 selector)",
				Name: "BytesTypes",
				Params: []EventParam{
					{Name: "data", Type: "bytes", Indexed: false},
					{Name: "hash", Type: "bytes32", Indexed: false},
					{Name: "selector", Type: "bytes4", Indexed: false},
				},
			},
			wantErr: false,
		},
		{
			name:      "Extra whitespace",
			signature: "  Transfer  ( address  indexed  from ,  address  indexed  to  ,  uint256  value  )  ",
			want: &EventSignature{
				Raw:  "Transfer  ( address  indexed  from ,  address  indexed  to  ,  uint256  value  )",
				Name: "Transfer",
				Params: []EventParam{
					{Name: "from", Type: "address", Indexed: true},
					{Name: "to", Type: "address", Indexed: true},
					{Name: "value", Type: "uint256", Indexed: false},
				},
			},
			wantErr: false,
		},
		{
			name:      "Empty signature",
			signature: "",
			wantErr:   true,
		},
		{
			name:      "Missing opening parenthesis",
			signature: "TransferAddress,address,uint256)",
			wantErr:   true,
		},
		{
			name:      "Missing closing parenthesis",
			signature: "Transfer(address,address,uint256",
			wantErr:   true,
		},
		{
			name:      "Invalid event name - lowercase",
			signature: "transfer(address,address,uint256)",
			wantErr:   true,
		},
		{
			name:      "Invalid event name - starts with number",
			signature: "1Transfer(address,address,uint256)",
			wantErr:   true,
		},
		{
			name:      "Invalid type",
			signature: "Transfer(invalidType from, address to, uint256 value)",
			wantErr:   true,
		},
		{
			name:      "Duplicate parameter names",
			signature: "Transfer(address from, address from, uint256 value)",
			wantErr:   true,
		},
		{
			name:      "Invalid parameter name",
			signature: "Transfer(address 123invalid, address to, uint256 value)",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEventSignature(tt.signature)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.Raw, got.Raw)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, len(tt.want.Params), len(got.Params))

			for i, wantParam := range tt.want.Params {
				assert.Equal(t, wantParam.Name, got.Params[i].Name, "param %d name", i)
				assert.Equal(t, wantParam.Type, got.Params[i].Type, "param %d type", i)
				assert.Equal(t, wantParam.Indexed, got.Params[i].Indexed, "param %d indexed", i)
			}
		})
	}
}

func TestEventSignature_CanonicalSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		want      string
	}{
		{
			name:      "With parameter names",
			signature: "Transfer(address indexed from, address indexed to, uint256 value)",
			want:      "Transfer(address,address,uint256)",
		},
		{
			name:      "Already canonical",
			signature: "Transfer(address,address,uint256)",
			want:      "Transfer(address,address,uint256)",
		},
		{
			name:      "No parameters",
			signature: "Initialized()",
			want:      "Initialized()",
		},
		{
			name:      "Complex types",
			signature: "MultiTransfer(address[] recipients, uint256[] amounts, bytes data)",
			want:      "MultiTransfer(address[],uint256[],bytes)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseEventSignature(tt.signature)
			require.NoError(t, err)

			got := parsed.CanonicalSignature()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEventSignature_IndexedParams(t *testing.T) {
	sig := "Transfer(address indexed from, address indexed to, uint256 value)"
	parsed, err := ParseEventSignature(sig)
	require.NoError(t, err)

	indexed := parsed.IndexedParams()
	assert.Len(t, indexed, 2)
	assert.Equal(t, "from", indexed[0].Name)
	assert.Equal(t, "to", indexed[1].Name)
}

func TestEventSignature_NonIndexedParams(t *testing.T) {
	sig := "Transfer(address indexed from, address indexed to, uint256 value)"
	parsed, err := ParseEventSignature(sig)
	require.NoError(t, err)

	nonIndexed := parsed.NonIndexedParams()
	assert.Len(t, nonIndexed, 1)
	assert.Equal(t, "value", nonIndexed[0].Name)
	assert.Equal(t, "uint256", nonIndexed[0].Type)
}

func TestIsValidSolidityType(t *testing.T) {
	validTypes := []string{
		"address",
		"bool",
		"string",
		"bytes",
		"bytes1", "bytes16", "bytes32",
		"uint", "uint8", "uint16", "uint32", "uint64", "uint128", "uint256",
		"int", "int8", "int16", "int32", "int64", "int128", "int256",
		"address[]",
		"uint256[]",
		"uint256[10]",
		"bytes32[]",
	}

	invalidTypes := []string{
		"",
		"invalid",
		"uint257", // Too large
		"bytes33", // Too large
		"bytes0",  // Too small
		"uint7",   // Not a multiple of 8
	}

	for _, typ := range validTypes {
		t.Run("valid_"+typ, func(t *testing.T) {
			assert.True(t, isValidSolidityType(typ), "expected %s to be valid", typ)
		})
	}

	for _, typ := range invalidTypes {
		t.Run("invalid_"+typ, func(t *testing.T) {
			assert.False(t, isValidSolidityType(typ), "expected %s to be invalid", typ)
		})
	}
}
