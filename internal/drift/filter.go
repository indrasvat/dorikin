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

// WithAdditionalPaths returns a new Filter that includes additional paths.
// The original Filter is not modified.
func (f *Filter) WithAdditionalPaths(additionalPaths []string) *Filter {
	// Copy existing paths
	paths := make(map[string]bool, len(f.ignorePaths)+len(additionalPaths))
	for p := range f.ignorePaths {
		paths[p] = true
	}
	// Add new paths
	for _, p := range additionalPaths {
		paths[normalizeJSONPath(p)] = true
	}
	return &Filter{ignorePaths: paths}
}

// ShouldIgnore returns true if the given path should be ignored.
func (f *Filter) ShouldIgnore(path string) bool {
	normalized := normalizeJSONPath(path)
	// Also create a version without array indices for wildcard matching
	withoutIndices := stripArrayIndices(normalized)

	// Check exact match
	if f.ignorePaths[normalized] || f.ignorePaths[withoutIndices] {
		return true
	}

	// Check if any ignore path is a prefix (for nested fields)
	for ignorePath := range f.ignorePaths {
		if strings.HasPrefix(normalized, ignorePath+".") ||
			strings.HasPrefix(normalized, ignorePath+"[") ||
			strings.HasPrefix(withoutIndices, ignorePath+".") ||
			strings.HasPrefix(withoutIndices, ignorePath+"[") {
			return true
		}
	}

	return false
}

// stripArrayIndices removes array indices from a path.
// e.g., ".spec.containers[0].ports[1].protocol" -> ".spec.containers.ports.protocol"
func stripArrayIndices(path string) string {
	var result strings.Builder
	inBracket := false
	for _, c := range path {
		if c == '[' {
			inBracket = true
			continue
		}
		if c == ']' {
			inBracket = false
			continue
		}
		if !inBracket {
			result.WriteRune(c)
		}
	}
	return result.String()
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
