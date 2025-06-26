package search

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/moov-io/ach"
)

// toSnakeCase converts a CamelCase string to snake_case.
// Example: TransactionCode -> transaction_code, RDFIIdentification -> rdfi_identification
func toSnakeCase(str string) string {
	if str == "" {
		return ""
	}
	var result strings.Builder
	lastWasUpper := false
	for i, r := range str {
		if unicode.IsUpper(r) {
			// Add underscore before uppercase if not the first character, not following an underscore,
			// and either the previous character was lowercase or the next character is lowercase
			if i > 0 && str[i-1] != '_' && (!lastWasUpper || (i+1 < len(str) && unicode.IsLower(rune(str[i+1])))) {
				result.WriteByte('_')
			}
			result.WriteRune(unicode.ToLower(r))
			lastWasUpper = true
		} else {
			result.WriteRune(r)
			lastWasUpper = false
		}
	}
	return result.String()
}

// toCamelCase converts a snake_case string back to CamelCase.
// Example: transaction_code -> TransactionCode, rdfi_identification -> RDFIIdentification
func toCamelCase(str string) string {
	if str == "" {
		return ""
	}
	// If already CamelCase, return as is
	if !strings.Contains(str, "_") && strings.ToLower(str[:1]) != str[:1] {
		return str
	}
	words := strings.Split(str, "_")
	var result strings.Builder
	for _, word := range words {
		if word != "" {
			// Preserve sequences of uppercase letters, only capitalize first letter if all lowercase
			if strings.ToLower(word) == word {
				switch word {
				case "odfi", "rdfi":
					result.WriteString(strings.ToUpper(word))
				default:
					result.WriteString(strings.ToUpper(string(word[0])) + strings.ToLower(word[1:]))
				}
			} else {
				result.WriteString(word)
			}
		}
	}
	return result.String()
}

var (
	concreteAchBatchTypes = []reflect.Type{
		reflect.TypeOf(ach.Batch{}),
		reflect.TypeOf(ach.BatchACK{}),
		reflect.TypeOf(ach.BatchADV{}),
		reflect.TypeOf(ach.BatchARC{}),
		reflect.TypeOf(ach.BatchATX{}),
		reflect.TypeOf(ach.BatchBOC{}),
		reflect.TypeOf(ach.BatchCCD{}),
		reflect.TypeOf(ach.BatchCIE{}),
		reflect.TypeOf(ach.BatchCOR{}),
		reflect.TypeOf(ach.BatchCTX{}),
		reflect.TypeOf(ach.BatchDNE{}),
		reflect.TypeOf(ach.BatchENR{}),
		// reflect.TypeOf(ach.IATBatch{}),
		reflect.TypeOf(ach.BatchMTE{}),
		reflect.TypeOf(ach.BatchPOP{}),
		reflect.TypeOf(ach.BatchPOS{}),
		reflect.TypeOf(ach.BatchPPD{}),
		reflect.TypeOf(ach.BatchRCK{}),
		reflect.TypeOf(ach.BatchSHR{}),
		reflect.TypeOf(ach.BatchTEL{}),
		reflect.TypeOf(ach.BatchTRC{}),
		reflect.TypeOf(ach.BatchTRX{}),
		reflect.TypeOf(ach.BatchWEB{}),
		reflect.TypeOf(ach.BatchXCK{}),
	}
)

// extractFieldNames recursively extracts field names from a struct using reflection,
// converting them to snake_case without prefixes.
func extractFieldNames(t reflect.Type, fields map[string]string) {
	// Dereference pointers and slices
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name in snake_case
		snakeName := toSnakeCase(field.Name)
		// Store mapping: snake_case -> OriginalFieldName (only if not already set to avoid overwriting)
		if _, exists := fields[snakeName]; !exists {
			fields[snakeName] = field.Name
		}

		// Handle nested structs
		fieldType := field.Type
		// Dereference pointers and slices
		for fieldType.Kind() == reflect.Ptr || fieldType.Kind() == reflect.Slice {
			fieldType = fieldType.Elem()
		}
		// Recurse into structs, including interfaces (e.g., Batcher)
		if fieldType.Kind() == reflect.Struct || fieldType.Kind() == reflect.Interface {
			// For Batcher interface, use known concrete types
			if fieldType == reflect.TypeOf((*ach.Batcher)(nil)).Elem() {
				// reflect.TypeOf((*ach.Batcher)(nil)).Elem() gets the type of the Batcher interface.
				// Since Batcher is an interface, we can't reflect on its fields directly.
				// Instead, we process known concrete types that implement Batcher.
				for _, concreteType := range concreteAchBatchTypes {
					extractFieldNames(concreteType, fields)
				}
			} else if fieldType.Kind() == reflect.Struct {
				extractFieldNames(fieldType, fields)
			}
		}
	}
}

// getFieldByName retrieves a struct field by its snake_case name, searching recursively.
func getFieldByName(v reflect.Value, snakeName string) (reflect.Value, error) {
	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, fmt.Errorf("nil pointer for field %s", snakeName)
		}
		v = v.Elem()
	}

	// Handle slices by iterating over elements
	if v.Kind() == reflect.Slice && !v.IsNil() {
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			if elem.Kind() == reflect.Ptr && !elem.IsNil() {
				elem = elem.Elem()
			}
			if result, err := getFieldByName(elem, snakeName); err == nil {
				return result, nil
			}
		}
		return reflect.Value{}, fmt.Errorf("field %s not found in slice", snakeName)
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("field %s not found in value", snakeName)
	}

	// Try direct field match
	camelName := toCamelCase(snakeName)
	if field := v.FieldByName(camelName); field.IsValid() && field.CanInterface() {
		return field, nil
	}

	// Recursively search nested structs and interfaces
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.IsValid() || !field.CanInterface() {
			continue
		}
		fieldType := v.Type().Field(i).Type

		// Handle interfaces (e.g., Batcher)
		if fieldType.Kind() == reflect.Interface && !field.IsNil() {
			concreteValue := field.Elem()
			if concreteValue.Kind() == reflect.Ptr && !concreteValue.IsNil() {
				concreteValue = concreteValue.Elem()
			}
			if concreteValue.IsValid() {
				if result, err := getFieldByName(concreteValue, snakeName); err == nil {
					return result, nil
				}
			}
		}

		// Special handling for Batcher interface fields
		if fieldType == reflect.TypeOf((*ach.Batcher)(nil)).Elem() && field.Kind() == reflect.Slice && !field.IsNil() {

			// Try known concrete types implementing Batcher
			for _, concreteType := range concreteAchBatchTypes {
				// Check if field exists in the concrete type
				tempValue := reflect.New(concreteType).Elem()
				if tempValue.Kind() == reflect.Ptr {
					tempValue = tempValue.Elem()
				}

				if _, err := getFieldByName(tempValue, snakeName); err == nil {
					// If field exists, search in actual slice elements
					for j := 0; j < field.Len(); j++ {
						sliceElem := field.Index(j)
						if sliceElem.Kind() == reflect.Ptr && !sliceElem.IsNil() {
							sliceElem = sliceElem.Elem()
						}
						if sliceElem.IsValid() {
							if actualResult, err := getFieldByName(sliceElem, snakeName); err == nil {
								return actualResult, nil
							}
						}
					}
				}
			}
		}

		// Dereference pointers
		for fieldType.Kind() == reflect.Ptr {
			if field.IsNil() {
				break
			}
			fieldType = fieldType.Elem()
			field = field.Elem()
		}

		// Handle slices
		if fieldType.Kind() == reflect.Slice && field.Kind() == reflect.Slice && !field.IsNil() {
			for j := 0; j < field.Len(); j++ {
				sliceElem := field.Index(j)

				// Handle interfaces
				if sliceElem.Kind() == reflect.Interface && !sliceElem.IsNil() {
					sliceElem = sliceElem.Elem()
				}

				// Handle pointers
				if sliceElem.Kind() == reflect.Ptr && !sliceElem.IsNil() {
					sliceElem = sliceElem.Elem()
				}

				if sliceElem.IsValid() && sliceElem.Kind() == reflect.Struct {
					if result, err := getFieldByName(sliceElem, snakeName); err == nil {
						return result, nil
					}
				}
			}
		}

		// Recurse into structs
		if fieldType.Kind() == reflect.Struct && field.Kind() == reflect.Struct && field.IsValid() {
			if result, err := getFieldByName(field, snakeName); err == nil {
				return result, nil
			}
		}
	}

	return reflect.Value{}, fmt.Errorf("field %s not found", snakeName)
}
