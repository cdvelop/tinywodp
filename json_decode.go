package tinywodp

import (
	. "github.com/cdvelop/tinystring"
	"unsafe"
)

// JSON decoding implementation for TinyString
// Uses our custom reflectlite integration for minimal binary size - NO standard reflect

// JsonDecode parses JSON data and populates the target struct/slice
//
// Usage patterns:
//
//	err := Convert(jsonBytes).JsonDecode(&user)
//	err := Convert(jsonString).JsonDecode(&users)  // []User slice
//	err := Convert(reader).JsonDecode(&data)
//
// Supports decoding into:
// - Structs with basic field types
// - Slices of structs
// - Basic types (string, int, float, bool)
//
// Field matching: Uses snake_case JSON keys to struct fields
// Example: {"user_name": "John"} -> UserName field
func (c *refValue) JsonDecode(target any) error {
	if target == nil {
		return Err(errInvalidJSON, "target cannot be nil")
	}

	// Get JSON data as string
	jsonStr := c.getString()
	if jsonStr == "" {
		return Err(errInvalidJSON, "empty JSON data")
	}

	// Delegate to jsonH for thread-safe operation
	jh := getJsonH(c.separator)
	defer putJsonH(jh)
	return jh.decode(jsonStr, target)
}

// parseJsonIntoTarget parses JSON string and populates the target value
func (c *refValue) parseJsonIntoTarget(jsonStr string, target any) error {
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
	return c.parseJsonValueWithRefReflect(jsonStr, elem)
}

// parseJsonValueWithRefReflect parses a JSON value using our custom reflection
func (c *refValue) parseJsonValueWithRefReflect(jsonStr string, target *refValue) error {
	// Trim whitespace
	jsonStr = Convert(jsonStr).Trim().String()
	if len(jsonStr) == 0 {
		return Err(errInvalidJSON, "empty JSON")
	}
	switch target.refKind() {
	case tpString:
		return c.parseJsonStringRef(jsonStr, target)
	case tpInt, tpInt8, tpInt16, tpInt32, tpInt64:
		return c.parseJsonIntRef(jsonStr, target)
	case tpUint, tpUint8, tpUint16, tpUint32, tpUint64:
		return c.parseJsonUintRef(jsonStr, target)
	case tpFloat32, tpFloat64:
		return c.parseJsonFloatRef(jsonStr, target)
	case tpBool:
		return c.parseJsonBoolRef(jsonStr, target)
	case tpStruct:
		return c.parseJsonStructRef(jsonStr, target)
	case tpSlice:
		return c.parseJsonSliceRef(jsonStr, target)
	case tpPointer:
		return c.parseJsonPointerRef(jsonStr, target)
	default:
		return Err(errUnsupportedType, "unsupported target type for JSON decoding: "+target.refKind().String())
	}
}

// Custom reflection-based parsing functions using our *refValue system

// parseJsonStringRef parses a JSON string using our custom reflection
func (c *refValue) parseJsonStringRef(jsonStr string, target *refValue) error {
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
	decoded, err := c.unescapeJsonString(unquoted)
	if err != nil {
		return err
	}
	target.refSetString(decoded)
	return nil
}

// parseJsonIntRef parses a JSON integer using our custom reflection
func (c *refValue) parseJsonIntRef(jsonStr string, target *refValue) error {
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
func (c *refValue) parseJsonUintRef(jsonStr string, target *refValue) error {
	val, err := Convert(jsonStr).ToInt64() // Convert to int64 first, then cast to uint64
	if err != nil {
		return err
	}
	target.refSetUint(uint64(val))
	return nil
}

// parseJsonFloatRef parses a JSON float using our custom reflection
func (c *refValue) parseJsonFloatRef(jsonStr string, target *refValue) error {
	val, err := Convert(jsonStr).ToFloat()
	if err != nil {
		return err
	}
	target.refSetFloat(val)
	return nil
}

// parseJsonBoolRef parses a JSON boolean using our custom reflection
func (c *refValue) parseJsonBoolRef(jsonStr string, target *refValue) error {
	jsonStr = Convert(jsonStr).Trim().String()

	// Strict validation: must be exactly true or false
	if len(jsonStr) > 0 && jsonStr[0] == '"' {
		return Err(errInvalidJSON, "expected boolean but got string: "+jsonStr)
	}
	if len(jsonStr) > 0 && (jsonStr[0] >= '0' && jsonStr[0] <= '9' || jsonStr[0] == '-') {
		return Err(errInvalidJSON, "expected boolean but got number: "+jsonStr)
	}
	if len(jsonStr) > 0 && (jsonStr[0] == '[' || jsonStr[0] == '{') {
		return Err(errInvalidJSON, "expected boolean but got complex type")
	}

	switch jsonStr {
	case "true":
		target.refSetBool(true)
	case "false":
		target.refSetBool(false)
	default:
		return Err(errInvalidJSON, "invalid JSON boolean: "+jsonStr)
	}
	return nil
}

// parseJsonStructRef parses a JSON object into a struct using our custom reflection
func (c *refValue) parseJsonStructRef(jsonStr string, target *refValue) error {
	if target.refKind() != tpStruct {
		return Err(errUnsupportedType, "target is not a struct")
	}

	// Basic validation - must start with { and end with }
	jsonStr = Convert(jsonStr).Trim().String()
	if len(jsonStr) < 2 || jsonStr[0] != '{' || jsonStr[len(jsonStr)-1] != '}' {
		return Err(errInvalidJSON, "invalid JSON object format")
	}

	// Handle empty object
	if jsonStr == "{}" {
		return nil // empty object, nothing to set
	} // Get struct information
	var structInfo refStructType
	getStructType(target.Type(), &structInfo)
	if structInfo.refType == nil {
		return Err(errUnsupportedType, "cannot get struct information")
	}

	// Simple JSON parsing - remove outer braces and split by commas
	content := jsonStr[1 : len(jsonStr)-1] // Remove { }
	return c.parseJsonObjectContent(content, target, &structInfo)
}

// parseJsonSliceRef parses a JSON array into a slice using our custom reflection
func (c *refValue) parseJsonSliceRef(jsonStr string, target *refValue) error {
	if target.refKind() != tpSlice {
		return Err(errUnsupportedType, "target is not a slice")
	}

	// Basic validation - must start with [ and end with ]
	jsonStr = Convert(jsonStr).Trim().String()
	if len(jsonStr) < 2 || jsonStr[0] != '[' || jsonStr[len(jsonStr)-1] != ']' {
		return Err(errInvalidJSON, "invalid JSON array format")
	}

	elemType := target.Type().Elem()

	// Handle empty array
	if jsonStr == "[]" {
		switch elemType.Kind() {
		case tpString:
			target.refSet(refValueOf([]string{}))
		case tpStruct:
			// Create empty slice of structs using unsafe operations
			target.refSet(refValueOf([]interface{}{}))
		default:
			return Err(errUnsupportedType, "unsupported slice element type: "+elemType.Kind().String())
		}
		return nil
	}

	content := jsonStr[1 : len(jsonStr)-1] // Remove [ ]

	// Split array elements
	elements := c.splitJsonArrayElements(content)

	// Handle different element types
	switch elemType.Kind() {
	case tpString:
		return c.parseStringSlice(elements, target)
	case tpStruct:
		return c.parseStructSlice(elements, target, elemType)
	case tpInt, tpInt64:
		return c.parseIntSlice(elements, target)
	case tpFloat64:
		return c.parseFloatSlice(elements, target)
	case tpBool:
		return c.parseBoolSlice(elements, target)
	default:
		return Err(errUnsupportedType, "slice decoding only supports string, struct, int, float, and bool slices currently")
	}
}

// parseStringSlice parses a slice of JSON strings
func (c *refValue) parseStringSlice(elements []string, target *refValue) error {
	var stringSlice []string
	for _, elem := range elements {
		// Parse string element
		elemStr := Convert(elem).Trim().String()
		if len(elemStr) >= 2 && elemStr[0] == '"' && elemStr[len(elemStr)-1] == '"' {
			unquoted := elemStr[1 : len(elemStr)-1]
			decoded, err := c.unescapeJsonString(unquoted)
			if err != nil {
				return err
			}
			stringSlice = append(stringSlice, decoded)
		} else {
			return Err(errInvalidJSON, "invalid string element in array: "+elem)
		}
	}
	target.refSet(refValueOf(stringSlice))
	return nil
}

// parseStructSlice parses JSON array elements into a struct slice
func (c *refValue) parseStructSlice(elements []string, target *refValue, elemType *refType) error {
	if elemType.Kind() != tpStruct {
		return Err(errUnsupportedType, "element type is not a struct")
	}

	sliceLen := len(elements)

	if sliceLen == 0 {
		// Create empty slice using reflection to avoid memory issues
		emptySlice := refMakeSlice(target.Type(), 0, 0)
		target.refSet(emptySlice)
		return nil
	}

	// Create slice with proper capacity using reflection
	slice := refMakeSlice(target.Type(), sliceLen, sliceLen)
	target.refSet(slice)

	// Parse each element into the slice
	for i, elem := range elements {
		// Get the i-th element of the slice
		elemValue := target.refIndex(i)
		if !elemValue.refIsValid() {
			return Err(errInvalidJSON, "cannot access slice element at index "+Convert(i).String())
		}

		// Parse the JSON object into the struct element
		err := c.parseJsonStructRef(elem, elemValue)
		if err != nil {
			return Err(errInvalidJSON, "failed to parse element "+Convert(i).String()+": "+err.Error())
		}
	}

	return nil
}

// mallocSliceData allocates memory for slice data
func mallocSliceData(elemSize uintptr, count int) unsafe.Pointer {
	if count == 0 {
		return nil
	}
	totalSize := elemSize * uintptr(count)
	// Use make to allocate properly initialized memory
	data := make([]byte, totalSize)
	return unsafe.Pointer(&data[0])
}

// parseIntSlice, parseFloatSlice, parseBoolSlice implementations
func (c *refValue) parseIntSlice(elements []string, target *refValue) error {
	var intSlice []int
	for _, elem := range elements {
		// Parse int element
		elemStr := Convert(elem).Trim().String()
		intVal, err := Convert(elemStr).ToInt()
		if err != nil {
			return Err(errInvalidJSON, "invalid int element in array: "+elem)
		}
		intSlice = append(intSlice, intVal)
	}
	target.refSet(refValueOf(intSlice))
	return nil
}

func (c *refValue) parseFloatSlice(elements []string, target *refValue) error {
	var floatSlice []float64
	for _, elem := range elements {
		// Parse float element
		elemStr := Convert(elem).Trim().String()
		floatVal, err := Convert(elemStr).ToFloat()
		if err != nil {
			return Err(errInvalidJSON, "invalid float element in array: "+elem)
		}
		floatSlice = append(floatSlice, floatVal)
	}
	target.refSet(refValueOf(floatSlice))
	return nil
}

func (c *refValue) parseBoolSlice(elements []string, target *refValue) error {
	var boolSlice []bool
	for _, elem := range elements {
		// Parse bool element
		elemStr := Convert(elem).Trim().String()
		switch elemStr {
		case "true":
			boolSlice = append(boolSlice, true)
		case "false":
			boolSlice = append(boolSlice, false)
		default:
			return Err(errInvalidJSON, "invalid bool element in array: "+elem)
		}
	}
	target.refSet(refValueOf(boolSlice))
	return nil
}

// splitJsonArrayElements splits JSON array content into individual elements
func (c *refValue) splitJsonArrayElements(content string) []string {
	var elements []string
	current := Builder()
	inQuotes := false
	braceLevel := 0
	bracketLevel := 0

	for i, char := range content {
		switch char {
		case '"':
			if i == 0 || content[i-1] != '\\' {
				inQuotes = !inQuotes
			}
			current.appendRune(char)
		case '{':
			if !inQuotes {
				braceLevel++
			}
			current.appendRune(char)
		case '}':
			if !inQuotes {
				braceLevel--
			}
			current.appendRune(char)
		case '[':
			if !inQuotes {
				bracketLevel++
			}
			current.appendRune(char)
		case ']':
			if !inQuotes {
				bracketLevel--
			}
			current.appendRune(char)
		case ',':
			if !inQuotes && braceLevel == 0 && bracketLevel == 0 {
				elem := Convert(current.String()).Trim().String()
				if len(elem) > 0 {
					elements = append(elements, elem)
				}
				current.reset()
			} else {
				current.appendRune(char)
			}
		default:
			current.appendRune(char)
		}
	}

	if current.length() > 0 {
		elem := Convert(current.String()).Trim().String()
		if len(elem) > 0 {
			elements = append(elements, elem)
		}
	}

	return elements
}

// unescapeJsonString unescapes a JSON string value
func (c *refValue) unescapeJsonString(s string) (string, error) {
	// Simple implementation - just handle basic escapes for now
	// This could be expanded to handle all JSON escape sequences
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '"':
				result = append(result, '"')
			case '\\':
				result = append(result, '\\')
			case 'n':
				result = append(result, '\n')
			case 'r':
				result = append(result, '\r')
			case 't':
				result = append(result, '\t')
			default:
				result = append(result, s[i], s[i+1])
			}
			i++ // Skip next character
		} else {
			result = append(result, s[i])
		}
	}
	return string(result), nil
}

// parseJsonObjectContent parses the content of a JSON object (without outer braces)
func (c *refValue) parseJsonObjectContent(content string, target *refValue, structInfo *refStructType) error {
	if content == "" {
		return nil // empty content
	}

	// Simple field parsing - split by commas (note: this is simplified and doesn't handle nested objects properly)
	pairs := c.splitJsonFields(content)

	for _, pair := range pairs {
		if err := c.parseJsonFieldPair(pair, target, structInfo); err != nil {
			return err
		}
	}

	return nil
}

// splitJsonFields splits JSON object content into field pairs (simplified)
func (c *refValue) splitJsonFields(content string) []string {
	var pairs []string
	current := Builder() // Use our custom string builder
	inQuotes := false
	braceLevel := 0
	bracketLevel := 0

	for i, char := range content {
		switch char {
		case '"':
			if i == 0 || content[i-1] != '\\' {
				inQuotes = !inQuotes
			}
			current.appendRune(char)
		case '{':
			if !inQuotes {
				braceLevel++
			}
			current.appendRune(char)
		case '}':
			if !inQuotes {
				braceLevel--
			}
			current.appendRune(char)
		case '[':
			if !inQuotes {
				bracketLevel++
			}
			current.appendRune(char)
		case ']':
			if !inQuotes {
				bracketLevel--
			}
			current.appendRune(char)
		case ',':
			if !inQuotes && braceLevel == 0 && bracketLevel == 0 {
				pairs = append(pairs, current.String())
				current.reset()
			} else {
				current.appendRune(char)
			}
		default:
			current.appendRune(char)
		}
	}

	if current.length() > 0 {
		pairs = append(pairs, current.String())
	}

	return pairs
}

// parseJsonFieldPair parses a single "key":"value" pair
func (c *refValue) parseJsonFieldPair(pair string, target *refValue, structInfo *refStructType) error {
	pair = Convert(pair).Trim().String()

	// Find the colon separator
	colonIndex := c.findJsonColon(pair)
	if colonIndex == -1 {
		return Err(errInvalidJSON, "invalid field pair format: "+pair)
	}

	keyPart := Convert(pair[:colonIndex]).Trim().String()
	valuePart := Convert(pair[colonIndex+1:]).Trim().String()

	// Parse key (remove quotes)
	if len(keyPart) < 2 || keyPart[0] != '"' || keyPart[len(keyPart)-1] != '"' {
		return Err(errInvalidJSON, "invalid key format: "+keyPart)
	}
	jsonKey := keyPart[1 : len(keyPart)-1]

	// Find matching struct field
	fieldIndex := c.findStructFieldByJsonName(jsonKey, structInfo)
	if fieldIndex == -1 {
		// Field not found, skip it
		return nil
	}

	// Get the target field
	field := target.refField(fieldIndex)
	if !field.refIsValid() {
		return Err(errInvalidJSON, "invalid field")
	}

	// Parse and set the value
	return c.parseJsonValueWithRefReflect(valuePart, field)
}

// findJsonColon finds the position of the colon that separates key from value
func (c *refValue) findJsonColon(pair string) int {
	inQuotes := false
	for i, char := range pair {
		if char == '"' && (i == 0 || pair[i-1] != '\\') {
			inQuotes = !inQuotes
		} else if char == ':' && !inQuotes {
			return i
		}
	}
	return -1
}

// findStructFieldByJsonName finds the field index by JSON field name
func (c *refValue) findStructFieldByJsonName(jsonKey string, structInfo *refStructType) int {
	// First try to match using JSON tags
	for i, field := range structInfo.fields {
		if jsonName := field.tag.Get("json"); jsonName != "" {
			// Handle json:",omitempty" and similar tags
			if commaIndex := indexByte(jsonName, ','); commaIndex != -1 {
				jsonName = jsonName[:commaIndex]
			}
			if jsonName == jsonKey {
				return i
			}
		}
	}

	// Fallback to original field names (case-sensitive match)
	for i, field := range structInfo.fields {
		if field.name == jsonKey {
			return i
		}
	}

	// Fallback to case-insensitive match for common patterns
	for i, field := range structInfo.fields {
		// Convert PascalCase to snake_case for comparison
		snakeCase := toSnakeCase(field.name)
		if snakeCase == jsonKey {
			return i
		}
	}

	return -1
}

// indexByte returns the index of the first instance of c in s, or -1 if c is not present in s
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// toSnakeCase converts PascalCase to snake_case
func toSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	result := make([]byte, 0, len(s)+5) // Pre-allocate with some extra space
	for i, r := range s {
		// If uppercase and not first character, add underscore
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			// Convert to lowercase
			result = append(result, byte(r-'A'+'a'))
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}

// appendRune adds a rune to the current refValue value
func (c *refValue) appendRune(r rune) *refValue {
	current := c.getString()
	// Use the existing addRne2Buf method from convert.go
	buf := make([]byte, 0, len(current)+4) // 4 bytes max for UTF-8 rune
	buf = append(buf, current...)
	buf = addRne2Buf(buf, r)
	c.setString(string(buf))
	return c
}

// parseJsonPointerRef parses a JSON value into a pointer using our custom reflection
func (c *refValue) parseJsonPointerRef(jsonStr string, target *refValue) error {
	if target.refKind() != tpPointer {
		return Err(errUnsupportedType, "target is not a pointer")
	}

	// Handle null values
	jsonStr = Convert(jsonStr).Trim().String()
	if jsonStr == "null" {
		// Set pointer to nil - this is handled by not setting anything
		return nil
	}

	// Get the element type that the pointer points to
	elemType := target.Type().Elem()
	if elemType == nil {
		return Err(errUnsupportedType, "pointer element type is nil")
	}

	// Allocate memory for the element value
	elemSize := elemType.Size()
	if elemSize == 0 {
		return Err(errUnsupportedType, "element type has zero size")
	}

	// Allocate memory for the pointed-to value
	elemPtr := unsafe.Pointer(&make([]byte, elemSize)[0])
	memclr(elemPtr, elemSize)

	// Create a refValue representing the element value
	elemValue := &refValue{
		separator: "_",
		typ:       elemType,
		ptr:       elemPtr,
		flag:      refFlag(elemType.Kind()) | flagAddr,
	}

	// Parse the JSON into the element value
	err := c.parseJsonValueWithRefReflect(jsonStr, elemValue)
	if err != nil {
		return err
	}

	// Set the pointer to point to our allocated memory
	*(*unsafe.Pointer)(target.ptr) = elemPtr
	return nil
}
