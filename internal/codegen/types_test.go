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
		{"uint256", "string"},
		{"int", "string"},
		{"int8", "int64"},
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
		{"uint256", "TEXT"},
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
		{"HTTPSConnection", "h_t_t_p_s_connection"},
		{"tokenID", "token_i_d"},
		{"simple", "simple"},
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
		{"PoolCreated", "pool_createds"},
		{"Swap", "swaps"},
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
