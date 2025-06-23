package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moov-io/ach"
	"vitess.io/vitess/go/vt/sqlparser"
)

// ACHRecord represents a simplified ACH record for querying
type ACHRecord struct {
	TransactionCode string
	Amount          float64
	AccountNumber   string
	RoutingNumber   string
	Addenda         string
}

// ACHStore manages ACH files and their parsed data
type ACHStore struct {
	records []ACHRecord
}

// NewACHStore creates a new store and loads files from a local directory
func NewACHStore(dirPath string) (*ACHStore, error) {
	store := &ACHStore{}

	// Validate directory existence
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", dirPath)
	}

	// Walk the directory to find ACH files
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %v", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".ach" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			log.Printf("failed to open file %s: %v", path, err)
			return nil // Continue with other files
		}
		defer file.Close()

		achFile, err := parseACHFile(file)
		if err != nil {
			log.Printf("failed to parse ACH file %s: %v", path, err)
			return nil // Continue with other files
		}

		store.addRecordsFromACH(achFile)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %v", err)
	}

	if len(store.records) == 0 {
		log.Printf("warning: no valid ACH records found in %s", dirPath)
	}

	return store, nil
}

// parseACHFile parses an ACH file using moov-io/ach
func parseACHFile(reader io.Reader) (*ach.File, error) {
	achFile, err := ach.NewReader(reader).Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read ACH file: %v", err)
	}
	if err := achFile.Validate(); err != nil {
		return nil, fmt.Errorf("ACH file validation failed: %v", err)
	}
	return &achFile, nil
}

// addRecordsFromACH converts ACH entries to queryable records
func (s *ACHStore) addRecordsFromACH(file *ach.File) {
	for _, batch := range file.Batches {
		entries := batch.GetEntries()
		for _, entry := range entries {
			record := ACHRecord{
				TransactionCode: fmt.Sprintf("%d", entry.TransactionCode),
				Amount:          float64(entry.Amount) / 100.0, // Convert cents to dollars
				AccountNumber:   entry.DFIAccountNumber,
				RoutingNumber:   entry.RDFIIdentification + entry.CheckDigit,
				Addenda:         getAddendaString(entry),
			}
			s.records = append(s.records, record)
		}
	}
}

// getAddendaString extracts addenda information
func getAddendaString(entry *ach.EntryDetail) string {
	if len(entry.Addenda05) > 0 {
		return entry.Addenda05[0].PaymentRelatedInformation
	}
	return ""
}

// Query executes a SQL query against the ACH records
func (s *ACHStore) Query(sql string) ([]map[string]interface{}, error) {
	// Initialize the parser with options
	var opts sqlparser.Options
	p, err := sqlparser.New(opts)
	if err != nil {
		return nil, fmt.Errorf("creating sqlparser: %w", err)
	}

	// Parse the SQL query
	stmt, err := p.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL: %v", err)
	}

	sel, ok := stmt.(*sqlparser.Select)
	if !ok {
		return nil, fmt.Errorf("only SELECT queries are supported")
	}

	if len(sel.From) != 1 {
		return nil, fmt.Errorf("exactly one table expected")
	}
	tableName := sqlparser.String(sel.From[0].(*sqlparser.AliasedTableExpr).Expr)
	if tableName != "ach_records" {
		return nil, fmt.Errorf("unknown table: %s", tableName)
	}

	// Process WHERE clause
	filter := func(record ACHRecord) bool {
		if sel.Where == nil {
			return true
		}
		return evaluateWhere(sel.Where.Expr, record)
	}

	// Process SELECT expressions
	columns := getSelectedColumns(sel.SelectExprs)
	if len(columns) == 0 {
		return nil, fmt.Errorf("no columns selected")
	}

	// Validate selected columns
	validColumns := map[string]bool{
		"transaction_code": true,
		"amount":           true,
		"account_number":   true,
		"routing_number":   true,
		"addenda":          true,
	}
	for _, col := range columns {
		if !validColumns[strings.ToLower(col)] {
			return nil, fmt.Errorf("invalid column: %s", col)
		}
	}

	// Apply filter and select columns
	var results []map[string]interface{}
	for _, record := range s.records {
		if filter(record) {
			result := make(map[string]interface{})
			for _, col := range columns {
				result[col] = getRecordField(record, col)
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// evaluateWhere evaluates a WHERE clause condition
func evaluateWhere(expr sqlparser.Expr, record ACHRecord) bool {
	switch e := expr.(type) {
	case *sqlparser.ComparisonExpr:
		left := sqlparser.String(e.Left)
		rightLit, ok := e.Right.(*sqlparser.Literal)
		if !ok {
			return false // Only handle literal values for simplicity
		}
		right := rightLit.Val
		value := getRecordField(record, left)
		switch e.Operator {
		case sqlparser.EqualOp:
			return value == right
		case sqlparser.GreaterThanOp:
			leftVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return false
			}
			rightVal, err := strconv.ParseFloat(right, 64)
			if err != nil {
				return false
			}
			return leftVal > rightVal
		case sqlparser.LessThanOp:
			leftVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return false
			}
			rightVal, err := strconv.ParseFloat(right, 64)
			if err != nil {
				return false
			}
			return leftVal < rightVal
		default:
			return false // Unsupported operator
		}
	case *sqlparser.AndExpr:
		return evaluateWhere(e.Left, record) && evaluateWhere(e.Right, record)
	case *sqlparser.OrExpr:
		return evaluateWhere(e.Left, record) || evaluateWhere(e.Right, record)
	default:
		return true // No condition or unsupported expression
	}
}

// getRecordField extracts a field value from an ACHRecord
func getRecordField(record ACHRecord, field string) string {
	switch strings.ToLower(field) {
	case "transaction_code":
		return record.TransactionCode
	case "amount":
		return fmt.Sprintf("%.2f", record.Amount)
	case "account_number":
		return record.AccountNumber
	case "routing_number":
		return record.RoutingNumber
	case "addenda":
		return record.Addenda
	default:
		return ""
	}
}

// getSelectedColumns extracts column names from SELECT expressions
func getSelectedColumns(exprs *sqlparser.SelectExprs) []string {
	var columns []string
	for _, expr := range exprs.Exprs {
		switch e := expr.(type) {
		case *sqlparser.StarExpr:
			return []string{"transaction_code", "amount", "account_number", "routing_number", "addenda"}
		case *sqlparser.AliasedExpr:
			if colName, ok := e.Expr.(*sqlparser.ColName); ok {
				columns = append(columns, sqlparser.String(colName))
			}
		}
	}
	return columns
}

func main() {
	where, err := filepath.Abs(filepath.Join("testdata", "ach")) // Directory containing ACH files
	if err != nil {
		log.Fatalf("problem locating files in %v", where)
	}

	store, err := NewACHStore(where)
	if err != nil {
		log.Fatalf("failed to create ACH store: %v", err)
	}

	// Example SQL query
	query := "SELECT transaction_code, amount FROM ach_records WHERE amount > 10.00 AND amount < 250.00 AND (transaction_code = 22 OR transaction_code = 27);"
	results, err := store.Query(query)
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}

	// Print results
	fmt.Printf("\n\n")
	for _, result := range results {
		fmt.Printf("Result: %+v\n", result)
	}
}
