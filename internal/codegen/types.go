package codegen

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const (
	addressType = "address"
	boolType    = "bool"
	stringType  = "string"
	bytesType   = "bytes"
	textType    = "TEXT"
)

// GoTypeName converts a Solidity type to a Go type name.
func GoTypeName(solidityType string) string {
	// Handle arrays first (before checking the base type)
	if strings.HasSuffix(solidityType, "[]") {
		// Dynamic arrays
		baseType := strings.TrimSuffix(solidityType, "[]")
		return "[]" + GoTypeName(baseType)
	}
	if regexp.MustCompile(`\[\d+\]$`).MatchString(solidityType) {
		// Fixed-size arrays (e.g., uint256[10])
		baseType := regexp.MustCompile(`\[\d+\]$`).ReplaceAllString(solidityType, "")
		return "[]" + GoTypeName(baseType) // Use slices in Go
	}

	// Now handle base types
	switch {
	case solidityType == addressType:
		return "common.Address"
	case solidityType == boolType:
		return boolType
	case solidityType == stringType:
		return stringType
	case solidityType == bytesType:
		return "[]byte"
	case strings.HasPrefix(solidityType, bytesType):
		// bytes1-bytes32 are fixed-size byte arrays, use common.Hash for bytes32
		if solidityType == "bytes32" {
			return "common.Hash"
		}
		return "[]byte"
	case strings.HasPrefix(solidityType, "uint"):
		size := strings.TrimPrefix(solidityType, "uint")
		if size == "" || size == "256" || size == "128" {
			return stringType // Large numbers need string representation
		}
		return "uint64"
	case strings.HasPrefix(solidityType, "int"):
		size := strings.TrimPrefix(solidityType, "int")
		if size == "" || size == "256" || size == "128" {
			return stringType
		}
		return "int64"
	default:
		return "interface{}"
	}
}

// DBTypeName converts a Solidity type to a database column type.
func DBTypeName(solidityType string) string {
	switch {
	case solidityType == addressType:
		return textType
	case solidityType == boolType:
		return "BOOLEAN"
	case solidityType == stringType:
		return textType
	case solidityType == bytesType:
		return "BLOB"
	case strings.HasPrefix(solidityType, bytesType):
		return textType // Store as hex string
	case strings.HasPrefix(solidityType, "uint") || strings.HasPrefix(solidityType, "int"):
		// Check if it fits in INTEGER (int64)
		size := strings.TrimPrefix(solidityType, "uint")
		size = strings.TrimPrefix(size, "int")
		if size == "" || size == "256" || size == "128" {
			return textType // Large numbers as text
		}
		return "INTEGER"
	case strings.HasSuffix(solidityType, "[]") || regexp.MustCompile(`\[\d+\]$`).MatchString(solidityType):
		return textType // Arrays stored as JSON
	default:
		return textType
	}
}

// MeddlerTag returns the meddler struct tag for a field.
func MeddlerTag(param EventParam) string {
	fieldName := DBFieldName(param.Name)
	goType := GoTypeName(param.Type)

	// Special tags for common types
	switch goType {
	case "common.Address":
		return fmt.Sprintf(`meddler:"%s,address"`, fieldName)
	case "common.Hash":
		return fmt.Sprintf(`meddler:"%s,hash"`, fieldName)
	default:
		return fmt.Sprintf(`meddler:"%s"`, fieldName)
	}
}

// DBFieldName converts a parameter name to a database field name.
// Examples: "from" -> "from_address", "to" -> "to_address", "value" -> "value"
func DBFieldName(paramName string) string {
	// Convert camelCase to snake_case
	snake := ToSnakeCase(paramName)

	// Add _address suffix for common address field names
	addressFields := []string{"from", "to", "owner", "spender", "sender", "recipient"}
	for _, field := range addressFields {
		if snake == field || strings.HasSuffix(snake, "_"+field) {
			return snake + "_address"
		}
	}

	return snake
}

// ToSnakeCase converts a string from camelCase or PascalCase to snake_case.
func ToSnakeCase(s string) string {
	result := make([]rune, 0, len(s)+len(s))
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result = append(result, '_')
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}

// ToPascalCase converts a string to PascalCase.
func ToPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})

	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}

	return strings.Join(parts, "")
}

// ToLowerCamelCase converts a string to lowerCamelCase.
func ToLowerCamelCase(s string) string {
	pascal := ToPascalCase(s)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// Pluralize returns a simple pluralized form of a word.
func Pluralize(word string) string {
	if strings.HasSuffix(word, "s") || strings.HasSuffix(word, "x") ||
		strings.HasSuffix(word, "z") || strings.HasSuffix(word, "ch") ||
		strings.HasSuffix(word, "sh") {
		return word + "es"
	}
	if strings.HasSuffix(word, "y") && len(word) > 1 {
		// Check if the character before 'y' is a consonant
		beforeY := rune(word[len(word)-2])
		if !isVowel(beforeY) {
			return word[:len(word)-1] + "ies"
		}
	}
	return word + "s"
}

func isVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}

// TableName generates a table name from an event name.
func TableName(eventName string) string {
	snake := ToSnakeCase(eventName)
	return Pluralize(snake)
}
