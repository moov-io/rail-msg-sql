package search

import (
	"errors"
	"reflect"
	"testing"

	"github.com/moov-io/ach"
	"github.com/stretchr/testify/require"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Basic CamelCase", "TransactionCode", "transaction_code"},
		{"Single Word", "Amount", "amount"},
		{"Multiple Uppercase", "RDFIIdentification", "rdfi_identification"},
		{"Already Snake", "company_name", "company_name"},
		{"Empty String", "", ""},
		{"Single Letter", "A", "a"},
		{"All Uppercase", "ODFI", "odfi"},
		{"Mixed Case", "AddendaRecordIndicator", "addenda_record_indicator"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toSnakeCase(tt.input)
			require.Equal(t, tt.expected, result, "toSnakeCase(%q) failed", tt.input)
		})
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Basic SnakeCase", "transaction_code", "TransactionCode"},
		{"Single Word", "amount", "Amount"},
		{"Multiple Words", "rdfi_identification", "RDFIIdentification"},
		{"Already Camel", "CompanyName", "CompanyName"},
		{"Empty String", "", ""},
		{"Single Letter", "a", "A"},
		{"All Lowercase", "odfi_identification", "ODFIIdentification"},
		{"Trailing Underscore", "addenda_record_", "AddendaRecord"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toCamelCase(tt.input)
			require.Equal(t, tt.expected, result, "toCamelCase(%q) failed", tt.input)
		})
	}
}

func TestExtractFieldNames(t *testing.T) {
	tests := []struct {
		name         string
		inputType    reflect.Type
		containsKeys []string
	}{
		{
			name:      "File",
			inputType: reflect.TypeOf(ach.File{}),
			containsKeys: []string{
				"immediate_origin",
				"company_name",
				"company_entry_description",
				"dfi_account_number",
				"trace_number",
				"foreign_correspondent_bank_id_number",
				"addenda05",
				"addenda98",
				"addenda99",
				"original_entry_trace_number",
				"corrected_data",
				"return_code",
				"contested_return_code",
				"dishonored_return_trace_number",
				"message_authentication_code",
				"total_debit_entry_dollar_amount_in_file",
			},
		},
		{
			name:      "FileHeader Struct",
			inputType: reflect.TypeOf(ach.FileHeader{}),
			containsKeys: []string{
				"immediate_destination",
				"file_creation_date",
				"immediate_destination_name",
				"reference_code",
			},
		},
		{
			name:      "EntryDetail Struct",
			inputType: reflect.TypeOf(ach.EntryDetail{}),
			containsKeys: []string{
				"transaction_code",
				"rdfi_identification",
				"type_code",
				"return_code",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := make(map[string]string)
			extractFieldNames(tt.inputType, fields)

			for _, key := range tt.containsKeys {
				_, exists := fields[key]
				require.True(t, exists, "expected key %s not found", key)
			}
		})
	}
}

func TestGetFieldByName(t *testing.T) {
	file := ach.File{
		Header: ach.FileHeader{
			ImmediateDestination: "123456789",
			FileCreationDate:     "20230624",
		},
		Batches: []ach.Batcher{
			&ach.Batch{
				Header: &ach.BatchHeader{
					ServiceClassCode: 200,
				},
				Entries: []*ach.EntryDetail{
					{
						TransactionCode: 22,
						Addenda99: &ach.Addenda99{
							ReturnCode: "R01",
						},
					},
				},
			},
		},
	}

	v := reflect.ValueOf(file)

	tests := []struct {
		name          string
		snakeName     string
		expected      interface{}
		expectedError error
	}{
		{
			name:      "Top-Level Field",
			snakeName: "immediate_destination",
			expected:  "123456789",
		},
		{
			name:      "Nested Batch Field",
			snakeName: "service_class_code",
			expected:  200,
		},
		{
			name:      "Nested Entry Field",
			snakeName: "transaction_code",
			expected:  22,
		},
		{
			name:      "Deeply Nested Addenda",
			snakeName: "return_code",
			expected:  "R01",
		},
		{
			name:          "Invalid Field",
			snakeName:     "non_existent",
			expectedError: errors.New("field non_existent not found"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			field, err := getFieldByName(v, tc.snakeName)
			if tc.expectedError != nil {
				require.ErrorContains(t, err, tc.expectedError.Error())
				return
			}

			require.NoError(t, err, "getFieldByName(%q) failed", tc.snakeName)
			require.True(t, field.IsValid(), "field %s is invalid", tc.snakeName)

			// Compare the actual value
			actual := field.Interface()
			require.Equal(t, tc.expected, actual)
		})
	}
}
