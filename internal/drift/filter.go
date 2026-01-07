// Package drift provides drift detection functionality.
package drift

import (
	"strings"
)

// Filter defines which fields to ignore during comparison.
type Filter struct {
	ignorePaths map[string]bool
}

// NewFilter creates a new Filter with the given ignore paths.
func NewFilter(ignorePaths []string) *Filter {
	paths := make(map[string]bool, len(ignorePaths))
	for _, p := range ignorePaths {
		paths[normalizeJSONPath(p)] = true
	}
	return &Filter{ignorePaths: paths}
}

// ShouldIgnore returns true if the given path should be ignored.
func (f *Filter) ShouldIgnore(path string) bool {
	normalized := normalizeJSONPath(path)

	// Check exact match
	if f.ignorePaths[normalized] {
		return true
	}

	// Check if any ignore path is a prefix (for nested fields)
	for ignorePath := range f.ignorePaths {
		if strings.HasPrefix(normalized, ignorePath+".") ||
			strings.HasPrefix(normalized, ignorePath+"[") {
			return true
		}
	}

	return false
}

// normalizeJSONPath normalizes a JSON path for comparison.
func normalizeJSONPath(path string) string {
	// Ensure path starts with dot
	if !strings.HasPrefix(path, ".") {
		path = "." + path
	}
	return path
}

// FilterMap filters a map, removing ignored fields.
func (f *Filter) FilterMap(m map[string]any, basePath string) map[string]any {
	result := make(map[string]any)

	for key, value := range m {
		path := basePath + "." + key

		if f.ShouldIgnore(path) {
			continue
		}

		switch v := value.(type) {
		case map[string]any:
			filtered := f.FilterMap(v, path)
			if len(filtered) > 0 {
				result[key] = filtered
			}
		case []any:
			filtered := f.filterSlice(v, path)
			if len(filtered) > 0 {
				result[key] = filtered
			}
		default:
			result[key] = value
		}
	}

	return result
}

// filterSlice filters a slice, removing ignored fields from nested maps.
func (f *Filter) filterSlice(s []any, basePath string) []any {
	result := make([]any, 0, len(s))

	for i, item := range s {
		path := basePath + "[" + itoa(i) + "]"

		if f.ShouldIgnore(path) {
			continue
		}

		switch v := item.(type) {
		case map[string]any:
			filtered := f.FilterMap(v, path)
			result = append(result, filtered)
		default:
			result = append(result, item)
		}
	}

	return result
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
