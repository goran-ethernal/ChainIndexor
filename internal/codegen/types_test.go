package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoTypeName(t *testing.T) {
	tests := []struct {
		solidityType string
		want         string
	}{
		{"address", "common.Address"},
		{"bool", "bool"},
		{"string", "string"},
		{"bytes", "[]byte"},
		{"bytes32", "common.Hash"},
		{"bytes4", "[]byte"},
		{"uint", "string"},
		{"uint8", "uint64"},
		{"uint64", "uint64"},
		{"uint72", "string"},  // > 64 bits, needs string
		{"uint80", "string"},  // > 64 bits, needs string
		{"uint96", "string"},  // > 64 bits, needs string
		{"uint120", "string"}, // > 64 bits, needs string
		{"uint128", "string"},
		{"uint256", "string"},
		{"int", "string"},
		{"int8", "int64"},
		{"int64", "int64"},
		{"int72", "string"},  // > 64 bits, needs string
		{"int80", "string"},  // > 64 bits, needs string
		{"int96", "string"},  // > 64 bits, needs string
		{"int120", "string"}, // > 64 bits, needs string
		{"int128", "string"},
		{"int256", "string"},
		{"address[]", "[]common.Address"},
		{"uint256[]", "[]string"},
		{"uint256[10]", "[]string"},
	}

	for _, tt := range tests {
		t.Run(tt.solidityType, func(t *testing.T) {
			got := GoTypeName(tt.solidityType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDBTypeName(t *testing.T) {
	tests := []struct {
		solidityType string
		want         string
	}{
		{"address", "TEXT"},
		{"bool", "BOOLEAN"},
		{"string", "TEXT"},
		{"bytes", "BLOB"},
		{"bytes32", "TEXT"},
		{"uint8", "INTEGER"},
		{"uint64", "INTEGER"},
		{"uint72", "TEXT"},  // > 64 bits, needs TEXT
		{"uint80", "TEXT"},  // > 64 bits, needs TEXT
		{"uint96", "TEXT"},  // > 64 bits, needs TEXT
		{"uint120", "TEXT"}, // > 64 bits, needs TEXT
		{"uint128", "TEXT"},
		{"uint256", "TEXT"},
		{"int64", "INTEGER"},
		{"int72", "TEXT"},  // > 64 bits, needs TEXT
		{"int80", "TEXT"},  // > 64 bits, needs TEXT
		{"int96", "TEXT"},  // > 64 bits, needs TEXT
		{"int120", "TEXT"}, // > 64 bits, needs TEXT
		{"int128", "TEXT"},
		{"int256", "TEXT"},
		{"address[]", "TEXT"},
	}

	for _, tt := range tests {
		t.Run(tt.solidityType, func(t *testing.T) {
			got := DBTypeName(tt.solidityType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDBFieldName(t *testing.T) {
	tests := []struct {
		paramName string
		want      string
	}{
		{"from", "from_address"},
		{"to", "to_address"},
		{"owner", "owner_address"},
		{"spender", "spender_address"},
		{"sender", "sender_address"},
		{"recipient", "recipient_address"},
		{"value", "value"},
		{"tokenId", "token_id"},
		{"isActive", "is_active"},
		{"myParam", "my_param"},
	}

	for _, tt := range tests {
		t.Run(tt.paramName, func(t *testing.T) {
			got := DBFieldName(tt.paramName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"camelCase", "camel_case"},
		{"PascalCase", "pascal_case"},
		{"already_snake", "already_snake"},
		{"HTTPSConnection", "https_connection"},
		{"tokenID", "token_id"},
		{"simple", "simple"},
		{"ERC20", "erc20"},
		{"ERC20Token", "erc20_token"},
		{"HTTPTest", "http_test"},
		{"myHTTPServer", "my_http_server"},
		{"parseHTML", "parse_html"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToSnakeCase(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"snake_case", "SnakeCase"},
		{"kebab-case", "KebabCase"},
		{"camelCase", "Camelcase"},
		{"UPPERCASE", "Uppercase"},
		{"simple", "Simple"},
		{"multi_word_example", "MultiWordExample"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToPascalCase(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToLowerCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"snake_case", "snakeCase"},
		{"PascalCase", "pascalcase"},
		{"kebab-case", "kebabCase"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToLowerCamelCase(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"transfer", "transfers"},
		{"approval", "approvals"},
		{"box", "boxes"},
		{"class", "classes"},
		{"branch", "branches"},
		{"baby", "babies"},
		{"day", "days"},
		{"activity", "activities"},
		// Past-tense forms (common in events) should not be pluralized
		{"created", "created"},
		{"transferred", "transferred"},
		{"approved", "approved"},
		{"swapped", "swapped"},
		{"given", "given"},
		{"taken", "taken"},
		{"withdrawn", "withdrawn"},
		{"shown", "shown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Pluralize(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTableName(t *testing.T) {
	tests := []struct {
		eventName string
		want      string
	}{
		{"Transfer", "transfers"},
		{"Approval", "approvals"},
		{"PoolCreated", "pool_created"},
		{"Swap", "swaps"},
		{"TokensTransferred", "tokens_transferred"},
		{"AmountGiven", "amount_given"},
		{"FundsWithdrawn", "funds_withdrawn"},
	}

	for _, tt := range tests {
		t.Run(tt.eventName, func(t *testing.T) {
			got := TableName(tt.eventName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMeddlerTag(t *testing.T) {
	tests := []struct {
		name  string
		param EventParam
		want  string
	}{
		{
			name:  "address type",
			param: EventParam{Name: "from", Type: "address"},
			want:  `meddler:"from_address,address"`,
		},
		{
			name:  "bytes32 type",
			param: EventParam{Name: "hash", Type: "bytes32"},
			want:  `meddler:"hash,hash"`,
		},
		{
			name:  "uint256 type",
			param: EventParam{Name: "value", Type: "uint256"},
			want:  `meddler:"value"`,
		},
		{
			name:  "bool type",
			param: EventParam{Name: "enabled", Type: "bool"},
			want:  `meddler:"enabled"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MeddlerTag(tt.param)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsIntSizeLargerThan64(t *testing.T) {
	tests := []struct {
		name         string
		solidityType string
		intType      string
		want         bool
	}{
		{
			name:         "uint256 is larger than 64",
			solidityType: "uint256",
			intType:      "uint",
			want:         true,
		},
		{
			name:         "uint128 is larger than 64",
			solidityType: "uint128",
			intType:      "uint",
			want:         true,
		},
		{
			name:         "uint64 is not larger than 64",
			solidityType: "uint64",
			intType:      "uint",
			want:         false,
		},
		{
			name:         "uint32 is not larger than 64",
			solidityType: "uint32",
			intType:      "uint",
			want:         false,
		},
		{
			name:         "uint8 is not larger than 64",
			solidityType: "uint8",
			intType:      "uint",
			want:         false,
		},
		{
			name:         "int256 is larger than 64",
			solidityType: "int256",
			intType:      "int",
			want:         true,
		},
		{
			name:         "int128 is larger than 64",
			solidityType: "int128",
			intType:      "int",
			want:         true,
		},
		{
			name:         "int64 is not larger than 64",
			solidityType: "int64",
			intType:      "int",
			want:         false,
		},
		{
			name:         "int32 is not larger than 64",
			solidityType: "int32",
			intType:      "int",
			want:         false,
		},
		{
			name:         "wrong prefix returns false",
			solidityType: "uint256",
			intType:      "int",
			want:         false,
		},
		{
			name:         "uint72 is larger than 64",
			solidityType: "uint72",
			intType:      "uint",
			want:         true,
		},
		{
			name:         "uint80 is larger than 64",
			solidityType: "uint80",
			intType:      "uint",
			want:         true,
		},
		{
			name:         "uint96 is larger than 64",
			solidityType: "uint96",
			intType:      "uint",
			want:         true,
		},
		{
			name:         "uint120 is larger than 64",
			solidityType: "uint120",
			intType:      "uint",
			want:         true,
		},
		{
			name:         "empty size after prefix returns false",
			solidityType: "uint",
			intType:      "uint",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIntSizeLargerThan64(tt.solidityType, tt.intType)
			assert.Equal(t, tt.want, got)
		})
	}
}
