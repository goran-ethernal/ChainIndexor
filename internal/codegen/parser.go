package codegen

import (
	"fmt"
	"regexp"
	"strings"
)

// EventParam represents a parameter in an event signature.
type EventParam struct {
	Name    string // Parameter name (e.g., "from", "to", "value")
	Type    string // Solidity type (e.g., "address", "uint256")
	Indexed bool   // Whether the parameter is indexed
}

// EventSignature represents a parsed event signature.
type EventSignature struct {
	Raw    string       // Original signature string
	Name   string       // Event name (e.g., "Transfer")
	Params []EventParam // Event parameters
}

// ParseEventSignature parses an event signature string into structured data.
// Supported formats:
//   - "Transfer(address,address,uint256)"
//   - "Transfer(address indexed from, address indexed to, uint256 value)"
//   - "Transfer(address from, address to, uint256 value)"
func ParseEventSignature(sig string) (*EventSignature, error) {
	sig = strings.TrimSpace(sig)

	if sig == "" {
		return nil, fmt.Errorf("empty signature")
	}

	// Find the opening parenthesis
	openParen := strings.Index(sig, "(")
	if openParen == -1 {
		return nil, fmt.Errorf("invalid signature: missing opening parenthesis")
	}

	// Extract event name
	eventName := strings.TrimSpace(sig[:openParen])
	if eventName == "" {
		return nil, fmt.Errorf("invalid signature: empty event name")
	}

	// Validate event name (must start with uppercase letter)
	if !regexp.MustCompile(`^[A-Z][a-zA-Z0-9_]*$`).MatchString(eventName) {
		return nil, fmt.Errorf("invalid event name '%s': must start "+
			"with uppercase letter and contain only alphanumeric characters", eventName)
	}

	// Find the closing parenthesis
	closeParen := strings.LastIndex(sig, ")")
	if closeParen == -1 {
		return nil, fmt.Errorf("invalid signature: missing closing parenthesis")
	}

	if closeParen <= openParen {
		return nil, fmt.Errorf("invalid signature: malformed parentheses")
	}

	// Extract parameters string
	paramsStr := sig[openParen+1 : closeParen]

	// Parse parameters
	params, err := parseParameters(paramsStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	return &EventSignature{
		Raw:    sig,
		Name:   eventName,
		Params: params,
	}, nil
}

// parseParameters parses the parameter list from an event signature.
func parseParameters(paramsStr string) ([]EventParam, error) {
	paramsStr = strings.TrimSpace(paramsStr)

	// Empty parameter list
	if paramsStr == "" {
		return []EventParam{}, nil
	}

	// Split by comma, but be careful with nested types like tuples
	paramStrings := splitParameters(paramsStr)

	params := make([]EventParam, 0, len(paramStrings))
	paramNames := make(map[string]bool) // Track duplicate names

	for i, paramStr := range paramStrings {
		param, err := parseParameter(strings.TrimSpace(paramStr), i)
		if err != nil {
			return nil, fmt.Errorf("invalid parameter '%s': %w", paramStr, err)
		}

		// Check for duplicate parameter names (only if named)
		if param.Name != "" {
			if paramNames[param.Name] {
				return nil, fmt.Errorf("duplicate parameter name: %s", param.Name)
			}
			paramNames[param.Name] = true
		}

		params = append(params, param)
	}

	return params, nil
}

// splitParameters splits parameter string by commas, handling nested structures.
func splitParameters(paramsStr string) []string {
	var params []string
	var current strings.Builder
	depth := 0

	for _, ch := range paramsStr {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				params = append(params, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	// Add the last parameter
	if current.Len() > 0 {
		params = append(params, current.String())
	}

	return params
}

// parseParameter parses a single parameter string.
// Formats:
//   - "address" (type only)
//   - "address from" (type + name)
//   - "address indexed from" (type + indexed + name)
func parseParameter(paramStr string, index int) (EventParam, error) {
	if paramStr == "" {
		return EventParam{}, fmt.Errorf("empty parameter")
	}

	parts := strings.Fields(paramStr)
	if len(parts) == 0 {
		return EventParam{}, fmt.Errorf("empty parameter")
	}

	param := EventParam{}

	// First part is always the type
	param.Type = parts[0]

	// Validate Solidity type
	if !isValidSolidityType(param.Type) {
		return EventParam{}, fmt.Errorf("invalid Solidity type: %s", param.Type)
	}

	// Parse remaining parts
	switch len(parts) {
	case 1:
		// Type only: "address"
		param.Name = fmt.Sprintf("param%d", index)
		param.Indexed = false

	case 2: //nolint:mnd
		// Type + name OR type + indexed (without name)
		if parts[1] == "indexed" {
			param.Indexed = true
			param.Name = fmt.Sprintf("param%d", index)
		} else {
			param.Name = parts[1]
			param.Indexed = false
		}

	case 3: //nolint:mnd
		// Type + indexed + name
		if parts[1] != "indexed" {
			return EventParam{}, fmt.Errorf("expected 'indexed' keyword, got '%s'", parts[1])
		}
		param.Indexed = true
		param.Name = parts[2]

	default:
		return EventParam{}, fmt.Errorf("too many parts in parameter definition")
	}

	// Validate parameter name
	if param.Name != "" && !regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(param.Name) {
		return EventParam{}, fmt.Errorf("invalid parameter name: %s", param.Name)
	}

	return param, nil
}

// isValidSolidityType checks if a string is a valid Solidity type.
func isValidSolidityType(typ string) bool {
	// Basic types
	basicTypes := map[string]bool{
		"address": true,
		"bool":    true,
		"string":  true,
		"bytes":   true,
	}

	if basicTypes[typ] {
		return true
	}

	// Fixed-size bytes (bytes1 to bytes32)
	if matched, _ := regexp.MatchString(`^bytes([1-9]|[12][0-9]|3[0-2])$`, typ); matched {
		return true
	}

	// Unsigned integers (uint8 to uint256, in steps of 8)
	if matched, _ := regexp.MatchString(`^uint(8|16|24|32|40|48|56|64|72|80|88|96|104|112|120|128|136|144|152|160|168|176|184|192|200|208|216|224|232|240|248|256)?$`, typ); matched { //nolint:lll
		return true
	}

	// Signed integers (int8 to int256, in steps of 8)
	if matched, _ := regexp.MatchString(`^int(8|16|24|32|40|48|56|64|72|80|88|96|104|112|120|128|136|144|152|160|168|176|184|192|200|208|216|224|232|240|248|256)?$`, typ); matched { //nolint:lll
		return true
	}

	// Arrays (e.g., uint256[], address[3])
	if strings.HasSuffix(typ, "[]") {
		baseType := strings.TrimSuffix(typ, "[]")
		return isValidSolidityType(baseType)
	}

	if matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*\[\d+\]$`, typ); matched {
		// Fixed-size array (e.g., uint256[10])
		baseType := regexp.MustCompile(`\[\d+\]$`).ReplaceAllString(typ, "")
		return isValidSolidityType(baseType)
	}

	return false
}

// CanonicalSignature returns the canonical event signature without parameter names.
// Example: "Transfer(address,address,uint256)"
func (e *EventSignature) CanonicalSignature() string {
	if len(e.Params) == 0 {
		return e.Name + "()"
	}

	types := make([]string, len(e.Params))
	for i, param := range e.Params {
		types[i] = param.Type
	}

	return e.Name + "(" + strings.Join(types, ",") + ")"
}

// IndexedParams returns only the indexed parameters.
func (e *EventSignature) IndexedParams() []EventParam {
	var indexed []EventParam
	for _, param := range e.Params {
		if param.Indexed {
			indexed = append(indexed, param)
		}
	}
	return indexed
}

// NonIndexedParams returns only the non-indexed parameters.
func (e *EventSignature) NonIndexedParams() []EventParam {
	var nonIndexed []EventParam
	for _, param := range e.Params {
		if !param.Indexed {
			nonIndexed = append(nonIndexed, param)
		}
	}
	return nonIndexed
}
