package api

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"api.audius.co/trashid"
)

// WriteCSVFromStructs writes a CSV string from a slice of structs using reflection
// It uses the provided headers to determine which fields to include and their order
func WriteCSVFromStructs(items interface{}, headers []string) (string, error) {
	val := reflect.ValueOf(items)
	if val.Kind() != reflect.Slice {
		return "", fmt.Errorf("input must be a slice")
	}

	if val.Len() == 0 {
		return "", nil
	}

	// Get the type of the first element to determine struct fields
	elemType := val.Type().Elem()
	if elemType.Kind() != reflect.Struct {
		return "", fmt.Errorf("slice elements must be structs")
	}

	// Map header names to field indices
	fieldIndices, err := mapHeadersToFields(elemType, headers)
	if err != nil {
		return "", err
	}
	if len(fieldIndices) == 0 {
		return "", fmt.Errorf("no matching fields found for headers")
	}

	// Create CSV writer
	var csvBuilder strings.Builder
	writer := csv.NewWriter(&csvBuilder)

	// Write headers
	if err := writer.Write(headers); err != nil {
		return "", err
	}

	// Write data rows
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		record := make([]string, len(headers))

		for j, header := range headers {
			if fieldIndex, exists := fieldIndices[header]; exists {
				field := item.Field(fieldIndex)
				record[j] = convertFieldToString(field)
			} else {
				record[j] = "" // Empty string for missing fields
			}
		}

		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return csvBuilder.String(), nil
}

// WriteCSVFromStructsAuto writes a CSV string from a slice of structs using reflection
// It automatically extracts field names from the 'csv' struct tag
// Header/column order will be determined by struct field order.
// Use WriteCSVFromStructs if you want to manually specify the headers.
func WriteCSVFromStructsAuto(items interface{}) (string, error) {
	// Get the reflection value of the input
	val := reflect.ValueOf(items)
	if val.Kind() != reflect.Slice {
		return "", fmt.Errorf("input must be a slice")
	}

	if val.Len() == 0 {
		return "", nil
	}

	// Get the type of the first element to determine struct fields
	elemType := val.Type().Elem()
	if elemType.Kind() != reflect.Struct {
		return "", fmt.Errorf("slice elements must be structs")
	}

	// Extract CSV headers from struct tags
	headers, fieldIndices := extractCSVHeaders(elemType)
	if len(headers) == 0 {
		return "", fmt.Errorf("no fields with csv tags found")
	}

	// Create CSV writer
	var csvBuilder strings.Builder
	writer := csv.NewWriter(&csvBuilder)

	// Write headers
	if err := writer.Write(headers); err != nil {
		return "", err
	}

	// Write data rows
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		record := make([]string, len(fieldIndices))

		for j, fieldIndex := range fieldIndices {
			field := item.Field(fieldIndex)
			record[j] = convertFieldToString(field)
		}

		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return csvBuilder.String(), nil
}

// extractCSVHeaders extracts CSV headers and field indices from struct tags
func extractCSVHeaders(structType reflect.Type) ([]string, []int) {
	var headers []string
	var fieldIndices []int

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		csvTag := field.Tag.Get("csv")

		// Skip fields without csv tag or with csv:"-"
		if csvTag == "" || csvTag == "-" {
			continue
		}

		headers = append(headers, csvTag)
		fieldIndices = append(fieldIndices, i)
	}

	return headers, fieldIndices
}

// mapHeadersToFields maps header names to field indices by looking up csv tags
func mapHeadersToFields(structType reflect.Type, headers []string) (map[string]int, error) {
	fieldIndices := make(map[string]int)

	// Build a map of csv tag names to field indices
	csvTagToFieldIndex := make(map[string]int)
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		csvTag := field.Tag.Get("csv")

		// Skip fields without csv tag or with csv:"-"
		if csvTag == "" || csvTag == "-" {
			continue
		}

		csvTagToFieldIndex[csvTag] = i
	}

	// Map provided headers to field indices and check for missing headers
	var missingHeaders []string
	for _, header := range headers {
		if fieldIndex, exists := csvTagToFieldIndex[header]; exists {
			fieldIndices[header] = fieldIndex
		} else {
			missingHeaders = append(missingHeaders, header)
		}
	}

	// Return error if any headers don't have corresponding fields
	if len(missingHeaders) > 0 {
		return nil, fmt.Errorf("no field found with csv tag for headers: %v", missingHeaders)
	}

	return fieldIndices, nil
}

// convertFieldToString converts a reflect.Value to its string representation
func convertFieldToString(field reflect.Value) string {
	if !field.IsValid() {
		return ""
	}

	// Handle pointer types
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return ""
		}
		field = field.Elem()
	}

	// Handle trashid.HashId specially - it needs to be encoded like in JSON marshaling
	if field.Type() == reflect.TypeOf(trashid.HashId(0)) {
		if hashId, ok := field.Interface().(trashid.HashId); ok {
			if encoded, err := trashid.EncodeHashId(int(hashId)); err == nil {
				return encoded
			}
		}
		return field.String()
	}

	switch field.Kind() {
	case reflect.String:
		return field.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(field.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(field.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(field.Float(), 'f', 6, 64)
	case reflect.Bool:
		return strconv.FormatBool(field.Bool())
	default:
		// Handle time.Time and other types that implement Stringer
		if field.Type() == reflect.TypeOf(time.Time{}) {
			if timeVal, ok := field.Interface().(time.Time); ok {
				return timeVal.Format(time.RFC3339)
			}
		}

		// Try to use String() method
		if stringer, ok := field.Interface().(fmt.Stringer); ok {
			return stringer.String()
		}

		// Fallback to default string representation
		return fmt.Sprintf("%v", field.Interface())
	}
}
