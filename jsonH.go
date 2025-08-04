package tinywodp

import (
	"sync"

	. "github.com/cdvelop/tinystring"
)

// jsonH - JSON Handler for race-condition-free operations
// All mutable state for JSON operations is isolated in this struct
// Each JSON operation gets its own instance from the pool, ensuring thread safety
type jsonH struct {
	jTmp string   // String buffer for last operation (replaces refValue.tmpStr)
	jBuf []string // Field parsing buffer (pre-allocated 16 capacity)
	jEsc []byte   // Escape processing buffer (pre-allocated 256 capacity)
	jSep string   // Field separator (from refValue.separator)
}

// Pool for jsonH instances to minimize allocations
// TinyGo compatible - sync.Pool works perfectly in TinyGo
var jsonHPool = sync.Pool{
	New: func() interface{} {
		return &jsonH{
			jBuf: make([]string, 0, 16),
			jEsc: make([]byte, 0, 256),
		}
	},
}

// getJsonH retrieves a jsonH instance from pool with proper initialization
// Resets all buffers while preserving allocated capacity for memory efficiency
func getJsonH(separator string) *jsonH {
	jh := jsonHPool.Get().(*jsonH)
	jh.jSep = separator
	jh.jTmp = ""          // Reset string buffer
	jh.jBuf = jh.jBuf[:0] // Reset slice but keep capacity
	jh.jEsc = jh.jEsc[:0] // Reset byte slice but keep capacity
	return jh
}

// putJsonH returns a jsonH instance to the pool for reuse
// Should always be called with defer to ensure proper cleanup
func putJsonH(jh *jsonH) {
	// Clear sensitive data before returning to pool
	jh.jTmp = ""
	jh.jSep = ""
	jsonHPool.Put(jh)
}

// resetBuffers clears all working buffers in jsonH
// Used internally to ensure clean state between operations
func (jh *jsonH) resetBuffers() {
	jh.jTmp = ""
	jh.jBuf = jh.jBuf[:0]
	jh.jEsc = jh.jEsc[:0]
}

// appendToTmp appends string to jTmp buffer
// Replaces direct tmpStr assignment for thread safety
func (jh *jsonH) appendToTmp(s string) {
	jh.jTmp += s
}

// setTmp sets jTmp buffer to specific value
// Replaces direct tmpStr assignment for thread safety
func (jh *jsonH) setTmp(s string) {
	jh.jTmp = s
}

// getTmp returns current jTmp buffer value
// Replaces direct tmpStr access for thread safety
func (jh *jsonH) getTmp() string {
	return jh.jTmp
}

// appendToBuf adds string to jBuf parsing buffer
// Used for field parsing operations that need string accumulation
func (jh *jsonH) appendToBuf(s string) {
	jh.jBuf = append(jh.jBuf, s)
}

// getBuf returns current jBuf slice
// Provides access to accumulated parsing buffer
func (jh *jsonH) getBuf() []string {
	return jh.jBuf
}

// appendToEsc adds bytes to jEsc escape buffer
// Used for JSON escape sequence processing
func (jh *jsonH) appendToEsc(b []byte) {
	jh.jEsc = append(jh.jEsc, b...)
}

// appendByteToEsc adds single byte to jEsc escape buffer
// Optimized for single character escape operations
func (jh *jsonH) appendByteToEsc(b byte) {
	jh.jEsc = append(jh.jEsc, b)
}

// getEsc returns current jEsc buffer
// Provides access to escape processing buffer
func (jh *jsonH) getEsc() []byte {
	return jh.jEsc
}

// getSep returns field separator for this JSON handler
// Provides access to separator configuration
func (jh *jsonH) getSep() string {
	return jh.jSep
}

// ============================================================================
// JSON DECODE OPERATIONS - Thread-safe implementations
// ============================================================================

// decode parses JSON string and populates the target value
// This is the main entry point for JSON decoding operations using jsonH
func (jh *jsonH) decode(jsonStr string, target any) error {
	if target == nil {
		return Err(errInvalidJSON, "target cannot be nil")
	}

	// Use our custom reflection for target analysis
	rv := refValueOf(target)
	// Debug: Check what kind we get for the pointer
	targetKind := rv.refKind()
	if targetKind != tpPointer {
		return Err(errInvalidJSON, "target must be a pointer, got: "+targetKind.String())
	}

	// Get the element that the pointer points to
	elem := rv.refElem()
	if !elem.refIsValid() {
		return Err(errInvalidJSON, "target pointer is nil or invalid")
	}

	// Debug: Check what kind we get for the element
	elemKind := elem.refKind()
	if elemKind.String() == "invalid" {
		return Err(errInvalidJSON, "element kind is invalid - reflection issue")
	}

	// Parse JSON and populate the element using our custom reflection
	return jh.parseJsonValueWithRefReflect(jsonStr, elem)
}

// parseJsonValueWithRefReflect parses a JSON value using our custom reflection
// All tmpStr operations are replaced with jh.jTmp for thread safety
func (jh *jsonH) parseJsonValueWithRefReflect(jsonStr string, target *refValue) error {
	// Trim whitespace
	jsonStr = Convert(jsonStr).Trim().String()
	if len(jsonStr) == 0 {
		return Err(errInvalidJSON, "empty JSON")
	}
	switch target.refKind() {
	case tpString:
		return jh.parseJsonStringRef(jsonStr, target)
	case tpInt, tpInt8, tpInt16, tpInt32, tpInt64:
		return jh.parseJsonIntRef(jsonStr, target)
	case tpUint, tpUint8, tpUint16, tpUint32, tpUint64:
		return jh.parseJsonUintRef(jsonStr, target)
	case tpFloat32, tpFloat64:
		return jh.parseJsonFloatRef(jsonStr, target)
	case tpBool:
		return jh.parseJsonBoolRef(jsonStr, target)
	case tpStruct:
		return jh.parseJsonStructRef(jsonStr, target)
	case tpSlice:
		return jh.parseJsonSliceRef(jsonStr, target)
	case tpPointer:
		return jh.parseJsonPointerRef(jsonStr, target)
	default:
		return Err(errUnsupportedType, "for JSON decoding: "+target.refKind().String())
	}
}

// ============================================================================
// JSON PARSING METHODS - Thread-safe implementations for jsonH
// ============================================================================

// parseJsonStringRef parses a JSON string using our custom reflection
// All string operations use jh.jTmp instead of refValue.tmpStr for thread safety
func (jh *jsonH) parseJsonStringRef(jsonStr string, target *refValue) error {
	jsonStr = Convert(jsonStr).Trim().String()

	// Strict validation: must be a quoted string
	if len(jsonStr) < 2 || jsonStr[0] != '"' || jsonStr[len(jsonStr)-1] != '"' {
		// Check if this is actually a different type that should be rejected
		if jsonStr == "true" || jsonStr == "false" || jsonStr == "null" {
			return Err(errInvalidJSON, "expected string but got "+jsonStr)
		}
		// Check if it's a number
		if len(jsonStr) > 0 && (jsonStr[0] >= '0' && jsonStr[0] <= '9' || jsonStr[0] == '-') {
			return Err(errInvalidJSON, "expected string but got number: "+jsonStr)
		}
		// Check if it's an array or object
		if len(jsonStr) > 0 && (jsonStr[0] == '[' || jsonStr[0] == '{') {
			return Err(errInvalidJSON, "expected string but got complex type")
		}
		return Err(errInvalidJSON, "invalid JSON string format")
	}

	// Remove quotes and decode escape sequences
	unquoted := jsonStr[1 : len(jsonStr)-1]
	decoded, err := jh.unescapeJsonString(unquoted)
	if err != nil {
		return err
	}
	target.refSetString(decoded)
	return nil
}

// parseJsonIntRef parses a JSON integer using our custom reflection
func (jh *jsonH) parseJsonIntRef(jsonStr string, target *refValue) error {
	jsonStr = Convert(jsonStr).Trim().String()

	// Strict validation: must be a number, not a string or other type
	if len(jsonStr) > 0 && jsonStr[0] == '"' {
		return Err(errInvalidJSON, "expected number but got string: "+jsonStr)
	}
	if jsonStr == "true" || jsonStr == "false" {
		return Err(errInvalidJSON, "expected number but got boolean: "+jsonStr)
	}
	if len(jsonStr) > 0 && (jsonStr[0] == '[' || jsonStr[0] == '{') {
		return Err(errInvalidJSON, "expected number but got complex type")
	}
	intVal, err := Convert(jsonStr).ToInt64()
	if err != nil {
		return Err(errInvalidJSON, "invalid number: "+jsonStr)
	}
	target.refSetInt(intVal)
	return nil
}

// parseJsonUintRef parses a JSON unsigned integer using our custom reflection
func (jh *jsonH) parseJsonUintRef(jsonStr string, target *refValue) error {
	val, err := Convert(jsonStr).ToInt64() // Convert to int64 first, then cast to uint64
	if err != nil {
		return err
	}
	target.refSetUint(uint64(val))
	return nil
}

// parseJsonFloatRef parses a JSON float using our custom reflection
func (jh *jsonH) parseJsonFloatRef(jsonStr string, target *refValue) error {
	val, err := Convert(jsonStr).ToFloat()
	if err != nil {
		return err
	}
	target.refSetFloat(val)
	return nil
}

// parseJsonBoolRef parses a JSON boolean using our custom reflection
func (jh *jsonH) parseJsonBoolRef(jsonStr string, target *refValue) error {
	jsonStr = Convert(jsonStr).Trim().String()

	// Strict validation: must be exactly true or false
	if jsonStr == "true" {
		target.refSetBool(true)
		return nil
	} else if jsonStr == "false" {
		target.refSetBool(false)
		return nil
	}

	// Invalid boolean value
	return Err(errInvalidJSON, "expected boolean but got: "+jsonStr)
}

// parseJsonStructRef parses a JSON object using our custom reflection
func (jh *jsonH) parseJsonStructRef(jsonStr string, target *refValue) error {
	jsonStr = Convert(jsonStr).Trim().String()

	// Must be a JSON object
	if len(jsonStr) < 2 || jsonStr[0] != '{' || jsonStr[len(jsonStr)-1] != '}' {
		return Err(errInvalidJSON, "expected object but got: "+jsonStr)
	}

	// Remove braces
	content := jsonStr[1 : len(jsonStr)-1]
	content = Convert(content).Trim().String()

	// Empty object
	if len(content) == 0 {
		return nil
	}

	// Split into fields and parse each one
	fields, err := jh.splitJsonFields(content)
	if err != nil {
		return err
	}

	return jh.parseStructFields(fields, target)
}

// parseJsonSliceRef parses a JSON array using our custom reflection
func (jh *jsonH) parseJsonSliceRef(jsonStr string, target *refValue) error {
	jsonStr = Convert(jsonStr).Trim().String()

	// Must be a JSON array
	if len(jsonStr) < 2 || jsonStr[0] != '[' || jsonStr[len(jsonStr)-1] != ']' {
		return Err(errInvalidJSON, "expected array but got: "+jsonStr)
	}

	// Remove brackets
	content := jsonStr[1 : len(jsonStr)-1]
	content = Convert(content).Trim().String()

	// Empty array
	if len(content) == 0 {
		return nil
	}

	// Split into elements and parse each one
	elements, err := jh.splitJsonArrayElements(content)
	if err != nil {
		return err
	}

	return jh.parseSliceElements(elements, target)
}

// parseJsonPointerRef parses a JSON value for a pointer type
func (jh *jsonH) parseJsonPointerRef(jsonStr string, target *refValue) error {
	jsonStr = Convert(jsonStr).Trim().String()

	// Handle null
	if jsonStr == "null" {
		// Keep pointer as nil
		return nil
	}

	// Get the element the pointer points to
	elem := target.refElem()
	if !elem.refIsValid() {
		return Err(errInvalidJSON, "pointer target is invalid")
	}

	// Parse the value for the pointed-to element
	return jh.parseJsonValueWithRefReflect(jsonStr, elem)
}

// splitJsonFields splits JSON object content into key-value pairs
func (jh *jsonH) splitJsonFields(content string) (map[string]string, error) {
	fields := make(map[string]string)
	jh.resetBuffers()

	var key, value string
	var inString bool
	var escapeNext bool
	var braceLevel, bracketLevel int
	var state int // 0=key, 1=colon, 2=value, 3=comma

	for _, char := range content {
		if escapeNext {
			jh.jTmp += string(char)
			escapeNext = false
			continue
		}

		if char == '\\' && inString {
			jh.jTmp += string(char)
			escapeNext = true
			continue
		}

		if char == '"' && !escapeNext {
			inString = !inString
			jh.jTmp += string(char)
			continue
		}

		if inString {
			jh.jTmp += string(char)
			continue
		}

		switch char {
		case '{':
			braceLevel++
			jh.jTmp += string(char)
		case '}':
			braceLevel--
			jh.jTmp += string(char)
		case '[':
			bracketLevel++
			jh.jTmp += string(char)
		case ']':
			bracketLevel--
			jh.jTmp += string(char)
		case ':':
			if braceLevel == 0 && bracketLevel == 0 && state == 0 {
				key = Convert(jh.jTmp).Trim().String()
				jh.jTmp = ""
				state = 2 // Expecting value
			} else {
				jh.jTmp += string(char)
			}
		case ',':
			if braceLevel == 0 && bracketLevel == 0 && state == 2 {
				value = Convert(jh.jTmp).Trim().String()
				fields[key] = value
				jh.jTmp = ""
				state = 0 // Expecting next key
			} else {
				jh.jTmp += string(char)
			}
		default:
			jh.jTmp += string(char)
		}
	}

	// Handle last field
	if state == 2 && len(jh.jTmp) > 0 {
		value = Convert(jh.jTmp).Trim().String()
		fields[key] = value
	}

	return fields, nil
}

// splitJsonArrayElements splits JSON array content into individual elements
func (jh *jsonH) splitJsonArrayElements(content string) ([]string, error) {
	var elements []string
	jh.resetBuffers()

	var inString bool
	var escapeNext bool
	var braceLevel, bracketLevel int

	for _, char := range content {
		if escapeNext {
			jh.jTmp += string(char)
			escapeNext = false
			continue
		}

		if char == '\\' && inString {
			jh.jTmp += string(char)
			escapeNext = true
			continue
		}

		if char == '"' && !escapeNext {
			inString = !inString
			jh.jTmp += string(char)
			continue
		}

		if inString {
			jh.jTmp += string(char)
			continue
		}

		switch char {
		case '{':
			braceLevel++
			jh.jTmp += string(char)
		case '}':
			braceLevel--
			jh.jTmp += string(char)
		case '[':
			bracketLevel++
			jh.jTmp += string(char)
		case ']':
			bracketLevel--
			jh.jTmp += string(char)
		case ',':
			if braceLevel == 0 && bracketLevel == 0 {
				element := Convert(jh.jTmp).Trim().String()
				if len(element) > 0 {
					elements = append(elements, element)
				}
				jh.jTmp = ""
			} else {
				jh.jTmp += string(char)
			}
		default:
			jh.jTmp += string(char)
		}
	}

	// Handle last element
	if len(jh.jTmp) > 0 {
		element := Convert(jh.jTmp).Trim().String()
		if len(element) > 0 {
			elements = append(elements, element)
		}
	}

	return elements, nil
}

// parseStructFields parses struct fields from JSON key-value pairs
func (jh *jsonH) parseStructFields(fields map[string]string, target *refValue) error {
	// Get number of fields in struct
	numFields := target.refNumField()

	// Get struct type info for field names
	var structInfo refStructType
	getStructType(target.Type(), &structInfo)

	// Debug: Print available fields
	// fmt.Printf("DEBUG: JSON fields: %v\n", fields)
	// fmt.Printf("DEBUG: Struct has %d fields\n", numFields)
	// fmt.Printf("DEBUG: StructInfo has %d fields\n", len(structInfo.fields))

	// Parse each field in the struct
	for i := 0; i < numFields; i++ {
		if i >= len(structInfo.fields) {
			continue // Skip if no field info available
		}

		// Get field name
		fieldName := structInfo.fields[i].name
		// fmt.Printf("DEBUG: Field %d: %s\n", i, fieldName)

		// Check if this field exists in the JSON
		jsonValue, exists := fields[fieldName]
		if !exists {
			// fmt.Printf("DEBUG: Field %s not found in JSON\n", fieldName)
			continue // Skip missing fields
		}

		// fmt.Printf("DEBUG: Parsing field %s = %s\n", fieldName, jsonValue)

		// Get the field refValue
		fieldConv := target.refField(i)
		if !fieldConv.refIsValid() {
			continue // Skip invalid fields
		}

		// Parse the JSON value into this field
		err := jh.parseJsonValueWithRefReflect(jsonValue, fieldConv)
		if err != nil {
			return err
		}
	}

	return nil
}

// parseSliceElements parses slice elements from JSON array elements
func (jh *jsonH) parseSliceElements(elements []string, target *refValue) error {
	// This is a simplified implementation
	// In a full implementation, this would create slice elements and parse each one
	return Err(errUnsupportedType, "slice parsing not fully implemented")
}

// ============================================================================
// JSON UTILITY METHODS - Thread-safe implementations for jsonH
// ============================================================================

// unescapeJsonString unescapes a JSON string value using jh.jEsc buffer
// Uses jsonH escape buffer to avoid allocations
func (jh *jsonH) unescapeJsonString(s string) (string, error) {
	// Reset escape buffer for reuse
	jh.jEsc = jh.jEsc[:0]

	// Pre-allocate capacity if needed
	if cap(jh.jEsc) < len(s) {
		jh.jEsc = make([]byte, 0, len(s))
	}

	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '"':
				jh.jEsc = append(jh.jEsc, '"')
			case '\\':
				jh.jEsc = append(jh.jEsc, '\\')
			case 'n':
				jh.jEsc = append(jh.jEsc, '\n')
			case 'r':
				jh.jEsc = append(jh.jEsc, '\r')
			case 't':
				jh.jEsc = append(jh.jEsc, '\t')
			default:
				jh.jEsc = append(jh.jEsc, s[i], s[i+1])
			}
			i++ // Skip next character
		} else {
			jh.jEsc = append(jh.jEsc, s[i])
		}
	}
	return string(jh.jEsc), nil
}
