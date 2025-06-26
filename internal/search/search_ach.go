package search

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/moov-io/rail-msg-sql/internal/storage"

	"github.com/moov-io/ach"
	"vitess.io/vitess/go/vt/sqlparser"
)

func (s *service) executeAchFileSelect(ctx context.Context, sel *sqlparser.Select, params storage.FilterParams) (*Results, error) {
	// Step 1: Extract selected columns and detect aggregate functions
	type columnInfo struct {
		SnakeName string // snake_case column name
		CamelName string // CamelCase field name
		Aggregate string // "MIN", "MAX", "SUM", or "" for non-aggregate
	}
	var columns []columnInfo
	var headerColumns []string
	isAggregateQuery := false

	for _, expr := range sel.SelectExprs.Exprs {
		if aliased, ok := expr.(*sqlparser.AliasedExpr); ok {
			var snakeName, camelName, aggregate string
			switch e := aliased.Expr.(type) {
			case *sqlparser.ColName:
				snakeName = strings.ToLower(sqlparser.String(e.Name))
				camelName = toCamelCase(snakeName)

			case *sqlparser.FuncExpr:
				// Handle aggregate functions (MIN, MAX, SUM)
				aggregate = strings.ToUpper(sqlparser.String(e.Name))
				if aggregate != "MIN" && aggregate != "MAX" && aggregate != "SUM" {
					return nil, fmt.Errorf("unsupported aggregate function: %s", aggregate)
				}
				isAggregateQuery = true
				if len(e.Exprs) != 1 {
					return nil, fmt.Errorf("aggregate function %s expects one argument", aggregate)
				}
				// Check if the argument is an AliasedExpr or directly a ColName
				var colName *sqlparser.ColName
				if cn, ok := e.Exprs[0].(*sqlparser.ColName); ok {
					colName = cn
				}
				if colName == nil {
					return nil, fmt.Errorf("invalid argument for %s: expected column name", aggregate)
				}
				snakeName = strings.ToLower(sqlparser.String(colName.Name))
				camelName = toCamelCase(snakeName)

			case *sqlparser.Sum:
				aggregate = "SUM"
				isAggregateQuery = true
				// Check if the argument is an AliasedExpr or directly a ColName
				var colName *sqlparser.ColName
				if cn, ok := e.Arg.(*sqlparser.ColName); ok {
					colName = cn
				}
				if colName == nil {
					return nil, fmt.Errorf("invalid argument for %T: expected column name", e)
				}
				snakeName = strings.ToLower(sqlparser.String(colName.Name))
				camelName = toCamelCase(snakeName)

			case *sqlparser.Min:
				aggregate = "MIN"
				isAggregateQuery = true
				// Check if the argument is an AliasedExpr or directly a ColName
				var colName *sqlparser.ColName
				if cn, ok := e.Arg.(*sqlparser.ColName); ok {
					colName = cn
				}
				if colName == nil {
					return nil, fmt.Errorf("invalid argument for %T: expected column name", e)
				}
				snakeName = strings.ToLower(sqlparser.String(colName.Name))
				camelName = toCamelCase(snakeName)

			case *sqlparser.Max:
				aggregate = "MAX"
				isAggregateQuery = true
				// Check if the argument is an AliasedExpr or directly a ColName
				var colName *sqlparser.ColName
				if cn, ok := e.Arg.(*sqlparser.ColName); ok {
					colName = cn
				}
				if colName == nil {
					return nil, fmt.Errorf("invalid argument for %T: expected column name", e)
				}
				snakeName = strings.ToLower(sqlparser.String(colName.Name))
				camelName = toCamelCase(snakeName)

			default:
				return nil, fmt.Errorf("unsupported expression type: %T", aliased.Expr)
			}
			columns = append(columns, columnInfo{
				SnakeName: snakeName,
				CamelName: camelName,
				Aggregate: aggregate,
			})
			// Use aliased name if provided, otherwise snake_case column name
			headerName := snakeName
			if !aliased.As.IsEmpty() {
				headerName = strings.ToLower(sqlparser.String(aliased.As))
			} else if aggregate != "" {
				headerName = fmt.Sprintf("%s(%s)", aggregate, snakeName)
			}
			headerColumns = append(headerColumns, headerName)
		}
	}

	// Step 2: Get all possible ACH file fields using reflection
	achFileFields := make(map[string]string) // snake_case -> CamelCase
	extractFieldNames(reflect.TypeOf(ach.File{}), achFileFields)

	// Validate selected fields
	for _, col := range columns {
		if _, exists := achFileFields[col.SnakeName]; !exists {
			return nil, fmt.Errorf("invalid column: %s", col.SnakeName)
		}
	}

	// Step 3: Fetch ACH files from storage
	files, err := s.fileStorage.ListAchFiles(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetching ACH files: %w", err)
	}

	// Step 4: Setup and apply WHERE clause filter
	var filteredEntries []struct {
		File  *ach.File
		Entry ach.EntryDetail
	}
	if sel.Where != nil {
		for _, file := range files {
			fileValue := reflect.ValueOf(file).Elem()
			// Iterate over Batches and their Entries
			batches := fileValue.FieldByName("Batches")
			if !batches.IsValid() || batches.IsNil() {
				continue
			}
			for i := 0; i < batches.Len(); i++ {
				batch := batches.Index(i)
				if batch.Kind() == reflect.Interface {
					batch = batch.Elem()
				}
				if batch.Kind() == reflect.Ptr {
					batch = batch.Elem()
				}
				entries := batch.FieldByName("Entries")
				if !entries.IsValid() || entries.IsNil() {
					continue
				}
				for j := 0; j < entries.Len(); j++ {
					entry := entries.Index(j)
					if entry.Kind() == reflect.Ptr {
						entry = entry.Elem()
					}
					if !entry.IsValid() {
						continue
					}
					// Evaluate WHERE clause against the EntryDetail
					matches, err := s.evaluateWhere(ctx, file, sel.Where.Expr)
					if err != nil {
						s.logger.Error().LogErrorf("evaluating WHERE clause: %v", err)
						continue
					}
					if matches {
						filteredEntries = append(filteredEntries, struct {
							File  *ach.File
							Entry ach.EntryDetail
						}{
							File:  file,
							Entry: entry.Interface().(ach.EntryDetail),
						})
					}
				}
			}
		}
	} else {
		// No WHERE clause, collect all entries
		for _, file := range files {
			fileValue := reflect.ValueOf(file).Elem()
			batches := fileValue.FieldByName("Batches")
			if !batches.IsValid() || batches.IsNil() {
				continue
			}
			for i := 0; i < batches.Len(); i++ {
				batch := batches.Index(i)
				if batch.Kind() == reflect.Interface {
					batch = batch.Elem()
				}
				if batch.Kind() == reflect.Ptr {
					batch = batch.Elem()
				}
				entries := batch.FieldByName("Entries")
				if !entries.IsValid() || entries.IsNil() {
					continue
				}
				for j := 0; j < entries.Len(); j++ {
					entry := entries.Index(j)
					if entry.Kind() == reflect.Ptr {
						entry = entry.Elem()
					}
					if !entry.IsValid() {
						continue
					}
					filteredEntries = append(filteredEntries, struct {
						File  *ach.File
						Entry ach.EntryDetail
					}{
						File:  file,
						Entry: entry.Interface().(ach.EntryDetail),
					})
				}
			}
		}
	}

	// Step 5: Build results with headers and rows
	results := &Results{
		Headers: Row{Columns: make([]interface{}, len(headerColumns))},
		Rows:    make([]Row, 0),
	}
	for i, name := range headerColumns {
		results.Headers.Columns[i] = name
	}

	if isAggregateQuery {
		// Handle aggregate query (single row with MIN, MAX, SUM results)
		aggResults := make([]interface{}, len(columns))
		for i, col := range columns {
			if col.Aggregate == "" {
				return nil, fmt.Errorf("non-aggregate column %s in aggregate query", col.SnakeName)
			}
			var min, max, sum float64
			var initialized bool
			var isInt bool

			for _, entry := range filteredEntries {
				entryValue := reflect.ValueOf(entry.Entry).Elem()
				fieldValue, err := getFieldByName(entryValue, col.SnakeName)
				if err != nil || !fieldValue.IsValid() {
					continue
				}

				var val float64
				switch fieldValue.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					val = float64(fieldValue.Int()) / 100.0 // Convert cents to dollars
					isInt = true
				case reflect.Float32, reflect.Float64:
					val = fieldValue.Float()
				default:
					return nil, fmt.Errorf("aggregate %s not supported for non-numeric field %s", col.Aggregate, col.SnakeName)
				}

				if !initialized {
					min, max, sum = val, val, val
					initialized = true
				} else {
					if val < min {
						min = val
					}
					if val > max {
						max = val
					}
					sum += val
				}
			}

			if !initialized {
				aggResults[i] = nil // No valid data found
			} else {
				switch col.Aggregate {
				case "MIN":
					if isInt {
						aggResults[i] = min // Keep as float64 for dollars
					} else {
						aggResults[i] = min
					}
				case "MAX":
					if isInt {
						aggResults[i] = max // Keep as float64 for dollars
					} else {
						aggResults[i] = max
					}
				case "SUM":
					if isInt {
						aggResults[i] = sum // Keep as float64 for dollars
					} else {
						aggResults[i] = sum
					}
				}
			}
		}
		results.Rows = append(results.Rows, Row{Columns: aggResults})
	} else {
		// Handle non-aggregate query (one row per entry)
		for _, entry := range filteredEntries {
			entryValue := reflect.ValueOf(&entry.Entry).Elem()
			row := Row{Columns: make([]interface{}, len(columns))}

			for i, col := range columns {
				fieldValue, err := getFieldByName(entryValue, col.SnakeName)
				if err != nil || !fieldValue.IsValid() {
					s.logger.Warn().LogErrorf("getting field %s: %v", col.SnakeName, err)
					row.Columns[i] = nil
					continue
				}

				// Store raw value based on type
				switch fieldValue.Kind() {
				case reflect.String:
					row.Columns[i] = fieldValue.String()
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					row.Columns[i] = float64(fieldValue.Int()) / 100.0 // Convert cents to dollars
				case reflect.Float32, reflect.Float64:
					row.Columns[i] = fieldValue.Float()
				case reflect.Bool:
					row.Columns[i] = fieldValue.Bool()
				case reflect.Slice:
					row.Columns[i] = fieldValue.Interface()
				case reflect.Struct:
					row.Columns[i] = fieldValue.Interface()
				default:
					s.logger.Warn().Logf("unsupported field type %s for %s", fieldValue.Kind(), col.SnakeName)
					row.Columns[i] = nil
				}
			}

			results.Rows = append(results.Rows, row)
		}
	}

	return results, nil
}

// evaluateWhere evaluates the WHERE clause expression against an ACH file
func (s *service) evaluateWhere(ctx context.Context, file *ach.File, expr sqlparser.Expr) (bool, error) {
	switch e := expr.(type) {
	case *sqlparser.ComparisonExpr:
		return s.evaluateComparison(ctx, file, e)
	case *sqlparser.AndExpr:
		left, err := s.evaluateWhere(ctx, file, e.Left)
		if err != nil || !left {
			return false, err
		}
		right, err := s.evaluateWhere(ctx, file, e.Right)
		return left && right, err
	case *sqlparser.OrExpr:
		left, err := s.evaluateWhere(ctx, file, e.Left)
		if err != nil {
			return false, err
		}
		if left {
			return true, nil
		}
		return s.evaluateWhere(ctx, file, e.Right)
	default:
		return false, fmt.Errorf("unsupported WHERE expression: %T", expr)
	}
}

// evaluateComparison evaluates a comparison expression (e.g., column = value)
func (s *service) evaluateComparison(ctx context.Context, file *ach.File, expr *sqlparser.ComparisonExpr) (bool, error) {
	if col, ok := expr.Left.(*sqlparser.ColName); ok {
		snakeName := strings.ToLower(sqlparser.String(col.Name))
		fileValue := reflect.ValueOf(file).Elem()
		fieldValue, err := getFieldByName(fileValue, snakeName)
		if err != nil {
			return false, fmt.Errorf("field %s (from %#v) not found: %w", snakeName, fileValue, err)
		}

		if val, ok := expr.Right.(*sqlparser.Literal); ok {
			switch expr.Operator {
			case sqlparser.EqualOp, sqlparser.LessThanOp, sqlparser.GreaterThanOp, sqlparser.LessEqualOp, sqlparser.GreaterEqualOp:
				switch val.Type {
				case sqlparser.IntVal:
					// Handle integer comparisons
					if fieldValue.Kind() == reflect.Int || fieldValue.Kind() == reflect.Int8 ||
						fieldValue.Kind() == reflect.Int16 || fieldValue.Kind() == reflect.Int32 ||
						fieldValue.Kind() == reflect.Int64 {
						queryVal, err := strconv.ParseInt(string(val.Val), 10, 64)
						if err != nil {
							return false, fmt.Errorf("invalid integer value for %s: %w", snakeName, err)
						}
						fieldInt := fieldValue.Int()
						switch expr.Operator {
						case sqlparser.EqualOp:
							return fieldInt == queryVal, nil
						case sqlparser.LessThanOp:
							return fieldInt < queryVal, nil
						case sqlparser.GreaterThanOp:
							return fieldInt > queryVal, nil
						case sqlparser.LessEqualOp:
							return fieldInt <= queryVal, nil
						case sqlparser.GreaterEqualOp:
							return fieldInt >= queryVal, nil
						}
					}
					// Handle float fields compared to integer literals
					if fieldValue.Kind() == reflect.Float32 || fieldValue.Kind() == reflect.Float64 {
						queryVal, err := strconv.ParseFloat(string(val.Val), 64)
						if err != nil {
							return false, fmt.Errorf("invalid float value for %s: %w", snakeName, err)
						}
						fieldFloat := fieldValue.Float()
						switch expr.Operator {
						case sqlparser.EqualOp:
							return fieldFloat == queryVal, nil
						case sqlparser.LessThanOp:
							return fieldFloat < queryVal, nil
						case sqlparser.GreaterThanOp:
							return fieldFloat > queryVal, nil
						case sqlparser.LessEqualOp:
							return fieldFloat <= queryVal, nil
						case sqlparser.GreaterEqualOp:
							return fieldFloat >= queryVal, nil
						}
					}
					return false, fmt.Errorf("field %s is not a number", snakeName)
				case sqlparser.FloatVal:
					// Handle float comparisons
					if fieldValue.Kind() == reflect.Float32 || fieldValue.Kind() == reflect.Float64 {
						queryVal, err := strconv.ParseFloat(string(val.Val), 64)
						if err != nil {
							return false, fmt.Errorf("invalid float value for %s: %w", snakeName, err)
						}
						fieldFloat := fieldValue.Float()
						switch expr.Operator {
						case sqlparser.EqualOp:
							return fieldFloat == queryVal, nil
						case sqlparser.LessThanOp:
							return fieldFloat < queryVal, nil
						case sqlparser.GreaterThanOp:
							return fieldFloat > queryVal, nil
						case sqlparser.LessEqualOp:
							return fieldFloat <= queryVal, nil
						case sqlparser.GreaterEqualOp:
							return fieldFloat >= queryVal, nil
						}
					}
					// Handle integer fields compared to float literals
					if fieldValue.Kind() == reflect.Int || fieldValue.Kind() == reflect.Int8 ||
						fieldValue.Kind() == reflect.Int16 || fieldValue.Kind() == reflect.Int32 ||
						fieldValue.Kind() == reflect.Int64 {
						queryVal, err := strconv.ParseFloat(string(val.Val), 64)
						if err != nil {
							return false, fmt.Errorf("invalid float value for %s: %w", snakeName, err)
						}
						fieldFloat := float64(fieldValue.Int())
						switch expr.Operator {
						case sqlparser.EqualOp:
							return fieldFloat == queryVal, nil
						case sqlparser.LessThanOp:
							return fieldFloat < queryVal, nil
						case sqlparser.GreaterThanOp:
							return fieldFloat > queryVal, nil
						case sqlparser.LessEqualOp:
							return fieldFloat <= queryVal, nil
						case sqlparser.GreaterEqualOp:
							return fieldFloat >= queryVal, nil
						}
					}
					return false, fmt.Errorf("field %s is not a number", snakeName)
				case sqlparser.StrVal:
					// Handle booleans as string literals ("true" or "false")
					if fieldValue.Kind() == reflect.Bool {
						queryVal, err := strconv.ParseBool(string(val.Val))
						if err != nil {
							return false, fmt.Errorf("invalid boolean value for %s: %w", snakeName, err)
						}
						if expr.Operator == sqlparser.EqualOp {
							return fieldValue.Bool() == queryVal, nil
						}
						return false, fmt.Errorf("operator %v not supported for boolean fields", expr.Operator)
					}
					return false, fmt.Errorf("field %s is not a boolean", snakeName)
				default:
					return false, fmt.Errorf("unsupported value type for %s: %v", snakeName, val.Type)
				}
			default:
				return false, fmt.Errorf("unsupported comparison operator: %v", expr.Operator)
			}
		}
		return false, fmt.Errorf("unsupported comparison value type: %T", expr.Right)
	}
	return false, fmt.Errorf("unsupported comparison expression: expected column name on left")
}

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
