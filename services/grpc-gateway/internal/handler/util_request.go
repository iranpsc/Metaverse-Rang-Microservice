// Package handler provides HTTP handlers for the gRPC gateway service.
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// Helper functions

// requestHasBody reports whether the request may carry a body.
// ContentLength -1 (chunked / unset) is common for API clients such as Bruno and must not be treated as empty.
func requestHasBody(r *http.Request) bool {
	if r.Body == nil {
		return false
	}
	if r.ContentLength == 0 {
		return false
	}
	return r.ContentLength > 0 || r.ContentLength < 0
}

// decodeRequest decodes request data from query parameters, JSON body, or form-data
// It tries query parameters first, then falls back to body (JSON or form-data)
// This allows handlers to accept data from multiple sources
func decodeRequest(r *http.Request, v interface{}) error {
	// First, try to populate from query parameters
	queryErr := decodeQueryParams(r, v)

	// Check if body exists and has content
	if !requestHasBody(r) {
		// If no body, return query params result (even if empty, that's OK)
		return queryErr
	}

	// If we have a body, try to decode it
	contentType := r.Header.Get("Content-Type")
	var bodyErr error

	// Handle JSON requests
	if strings.HasPrefix(contentType, "application/json") {
		bodyErr = decodeJSONBody(r, v)
	} else if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		// Handle form-data requests
		bodyErr = decodeFormData(r, v)
	} else {
		// Default to JSON if content type is not specified or unknown
		bodyErr = decodeJSONBody(r, v)
	}

	// If body decoding succeeded, use it (body takes precedence over query params for same fields)
	// If body decoding failed but query params succeeded, use query params
	if bodyErr == nil {
		return nil
	}

	// If body decoding failed but query params succeeded, return query params result
	if queryErr == nil {
		return nil
	}

	// Both failed, return body error (more specific)
	return bodyErr
}

// decodeRequestBody decodes request body from JSON or form-data (multipart/form-data or application/x-www-form-urlencoded)
// It automatically detects the content type and handles both formats
// If the body is empty, it will also check query string parameters
func decodeRequestBody(r *http.Request, v interface{}) error {
	if !requestHasBody(r) {
		return decodeQueryParams(r, v)
	}

	contentType := r.Header.Get("Content-Type")
	var bodyErr error

	// Handle JSON requests
	if strings.HasPrefix(contentType, "application/json") {
		bodyErr = decodeJSONBody(r, v)
	} else if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		// Handle form-data requests
		bodyErr = decodeFormData(r, v)
	} else {
		// Default to JSON if content type is not specified or unknown
		// This maintains backward compatibility
		bodyErr = decodeJSONBody(r, v)
	}

	// If body decoding succeeded, also merge query parameters (query params can supplement body data)
	if bodyErr == nil {
		// Try to merge query parameters (non-destructive - only sets fields that are zero values)
		mergeQueryParams(r, v)
		return nil
	}

	// If body decoding failed, try query parameters as fallback when present
	if len(r.URL.Query()) > 0 {
		if queryErr := decodeQueryParams(r, v); queryErr == nil {
			return nil
		}
	}

	// Both failed, return body error
	return bodyErr
}

// mergeQueryParams merges query parameters into a struct, only setting fields that are zero values
// This allows query params to supplement body data without overwriting existing values
func mergeQueryParams(r *http.Request, v interface{}) {
	queryValues := r.URL.Query()
	if len(queryValues) == 0 {
		return
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Only set if field is zero value (empty)
		if !isZeroValue(fieldValue) {
			continue
		}

		// Get the JSON tag name, or use the field name as fallback
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Check for "query" tag as well
		if queryTag := field.Tag.Get("query"); queryTag != "" && queryTag != "-" {
			parts := strings.Split(queryTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Convert field name to lowercase for matching
		fieldNameLower := strings.ToLower(fieldName)

		// Try to find the query value
		var values []string
		var found bool
		if vals, ok := queryValues[fieldName]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := queryValues[fieldNameLower]; ok && len(vals) > 0 {
			values = vals
			found = true
		}

		if !found || len(values) == 0 {
			continue
		}

		// Get the first value and set it
		value := values[0]
		_ = setFieldValue(fieldValue, value) // Ignore errors for merge
	}
}

// isZeroValue checks if a reflect.Value is the zero value for its type
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Slice, reflect.Map, reflect.Interface, reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}

// decodeQueryParams decodes query string parameters into a struct
func decodeQueryParams(r *http.Request, v interface{}) error {
	queryValues := r.URL.Query()

	// If no query parameters, return nil (not an error, just empty)
	if len(queryValues) == 0 {
		return nil
	}

	// Use reflection to populate the struct
	return populateStructFromQuery(queryValues, v)
}

// decodeJSONBody safely decodes JSON from request body, handling empty bodies
func decodeJSONBody(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return io.EOF
	}

	// Check if body is empty
	if r.ContentLength == 0 {
		return io.EOF
	}

	// Try to peek at the body to see if it's already consumed
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	// Restore body for potential subsequent reads
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if len(bodyBytes) == 0 {
		return io.EOF
	}

	return json.Unmarshal(bodyBytes, v)
}

// decodeFormData decodes form-data (multipart/form-data or application/x-www-form-urlencoded) into a struct
func decodeFormData(r *http.Request, v interface{}) error {
	contentType := r.Header.Get("Content-Type")

	var formValues map[string][]string

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form
		err := r.ParseMultipartForm(32 << 20) // 32MB max memory
		if err != nil {
			return fmt.Errorf("failed to parse multipart form: %w", err)
		}
		formValues = r.MultipartForm.Value
	} else {
		// Parse URL-encoded form
		err := r.ParseForm()
		if err != nil {
			return fmt.Errorf("failed to parse form: %w", err)
		}
		formValues = r.PostForm
	}

	// Use reflection to populate the struct
	return populateStructFromForm(formValues, v)
}

// populateStructFromQuery populates a struct from query parameter values using reflection
// It uses JSON struct tags to map query parameter names to struct fields
func populateStructFromQuery(queryValues map[string][]string, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("v must be a pointer to a struct")
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Get the JSON tag name, or use the field name as fallback
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			// Remove options like "omitempty" from the tag
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Check for "query" tag as well
		if queryTag := field.Tag.Get("query"); queryTag != "" && queryTag != "-" {
			parts := strings.Split(queryTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Convert field name to lowercase for matching (common in query parameters)
		fieldNameLower := strings.ToLower(fieldName)

		// Handle array notation: try fieldName[] first, then fieldName
		var values []string
		var found bool

		// Try array notation first (e.g., points[])
		arrayNotation := fieldName + "[]"
		if vals, ok := queryValues[arrayNotation]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := queryValues[strings.ToLower(arrayNotation)]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := queryValues[fieldName]; ok && len(vals) > 0 {
			// Try exact field name
			values = vals
			found = true
		} else if vals, ok := queryValues[fieldNameLower]; ok && len(vals) > 0 {
			// Try lowercase field name
			values = vals
			found = true
		}

		if !found || len(values) == 0 {
			continue
		}

		// Handle slice/array fields specially - use all values
		if fieldValue.Kind() == reflect.Slice {
			// Create a new slice with the appropriate type
			sliceType := fieldValue.Type().Elem()
			slice := reflect.MakeSlice(fieldValue.Type(), len(values), len(values))

			for i, val := range values {
				elemValue := reflect.New(sliceType).Elem()
				if err := setFieldValue(elemValue, val); err != nil {
					return fmt.Errorf("failed to set slice element %s[%d]: %w", fieldName, i, err)
				}
				slice.Index(i).Set(elemValue)
			}

			fieldValue.Set(slice)
		} else {
			// For non-slice fields, get the first value
			value := values[0]
			if err := setFieldValue(fieldValue, value); err != nil {
				return fmt.Errorf("failed to set field %s: %w", fieldName, err)
			}
		}
	}

	return nil
}

// populateStructFromForm populates a struct from form values using reflection
// It uses JSON struct tags to map form field names to struct fields
func populateStructFromForm(formValues map[string][]string, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("v must be a pointer to a struct")
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Get the JSON tag name, or use the field name as fallback
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			// Remove options like "omitempty" from the tag
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Convert field name to lowercase for matching (common in form submissions)
		fieldNameLower := strings.ToLower(fieldName)

		// Try to find the form value by exact name first, then by lowercase
		var values []string
		var found bool
		if vals, ok := formValues[fieldName]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := formValues[fieldNameLower]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := formValues[field.Tag.Get("form")]; ok && len(vals) > 0 {
			// Also check for "form" tag
			values = vals
			found = true
		}

		if !found || len(values) == 0 {
			continue
		}

		// Get the first value (form fields can have multiple values, we take the first)
		value := values[0]

		// Set the field value based on its type
		if err := setFieldValue(fieldValue, value); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldName, err)
		}
	}

	return nil
}

// setFieldValue sets a reflect.Value from a string form value
func setFieldValue(fieldValue reflect.Value, value string) error {
	if !fieldValue.CanSet() {
		return fmt.Errorf("field is not settable")
	}

	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: %w", err)
		}
		fieldValue.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer value: %w", err)
		}
		fieldValue.SetUint(uintVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float value: %w", err)
		}
		fieldValue.SetFloat(floatVal)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			// Also accept "1", "true", "yes", "on" as true, and "0", "false", "no", "off" as false
			switch strings.ToLower(value) {
			case "1", "true", "yes", "on":
				boolVal = true
			case "0", "false", "no", "off":
				boolVal = false
			default:
				return fmt.Errorf("invalid boolean value: %w", err)
			}
		}
		fieldValue.SetBool(boolVal)
	default:
		return fmt.Errorf("unsupported field type: %s", fieldValue.Kind())
	}

	return nil
}
