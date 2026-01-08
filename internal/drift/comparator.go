package drift

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/google/go-cmp/cmp"

	"github.com/indrasvat/dorikin/pkg/api"
)

// Comparator compares Kubernetes resources.
type Comparator struct {
	filter *Filter
}

// NewComparator creates a new Comparator.
func NewComparator(ignorePaths []string) *Comparator {
	return &Comparator{
		filter: NewFilter(ignorePaths),
	}
}

// Compare compares expected and actual resource states.
// Returns a list of field differences.
func (c *Comparator) Compare(expected, actual map[string]any) []api.FieldDiff {
	return c.CompareWithDynamicIgnore(expected, actual, nil)
}

// CompareWithDynamicIgnore compares resources with additional dynamic ignore paths.
// This is used for HPA-managed resources where spec.replicas should be ignored.
func (c *Comparator) CompareWithDynamicIgnore(expected, actual map[string]any, dynamicIgnore []string) []api.FieldDiff {
	// Use the appropriate filter
	filter := c.filter
	if len(dynamicIgnore) > 0 {
		filter = c.filter.WithAdditionalPaths(dynamicIgnore)
	}

	// Filter both maps
	filteredExpected := filter.FilterMap(expected, "")
	filteredActual := filter.FilterMap(actual, "")

	// Collect differences
	var diffs []api.FieldDiff
	c.compareValues("", filteredExpected, filteredActual, &diffs)

	// Sort diffs by path for consistent output
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Path < diffs[j].Path
	})

	return diffs
}

// compareValues recursively compares two values.
func (c *Comparator) compareValues(path string, expected, actual any, diffs *[]api.FieldDiff) {
	if expected == nil && actual == nil {
		return
	}

	if expected == nil {
		*diffs = append(*diffs, api.FieldDiff{
			Path:   path,
			Actual: actual,
			Type:   api.DiffTypeAdded,
		})
		return
	}

	if actual == nil {
		*diffs = append(*diffs, api.FieldDiff{
			Path:     path,
			Expected: expected,
			Type:     api.DiffTypeRemoved,
		})
		return
	}

	// Check for quantity fields (e.g., cpu: "500m" vs "0.5")
	if isQuantityPath(path) {
		if compareQuantities(expected, actual) {
			return // Quantities are equivalent
		}
		// Fall through to normal comparison if not valid quantities
	}

	// Normalize numeric types for comparison
	expected = normalizeNumeric(expected)
	actual = normalizeNumeric(actual)

	expectedType := reflect.TypeOf(expected)
	actualType := reflect.TypeOf(actual)

	// Type mismatch (after normalization)
	if expectedType != actualType {
		*diffs = append(*diffs, api.FieldDiff{
			Path:     path,
			Expected: expected,
			Actual:   actual,
			Type:     api.DiffTypeModified,
		})
		return
	}

	switch exp := expected.(type) {
	case map[string]any:
		act := actual.(map[string]any)
		c.compareMaps(path, exp, act, diffs)
	case []any:
		act := actual.([]any)
		c.compareSlices(path, exp, act, diffs)
	default:
		// Use go-cmp for deep equality
		if !cmp.Equal(expected, actual) {
			*diffs = append(*diffs, api.FieldDiff{
				Path:     path,
				Expected: expected,
				Actual:   actual,
				Type:     api.DiffTypeModified,
			})
		}
	}
}

// normalizeNumeric converts all numeric types to float64 for comparison.
// JSON unmarshaling returns all numbers as float64, while YAML keeps integers as int.
// Normalizing to float64 ensures consistent comparison.
func normalizeNumeric(v any) any {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int8:
		return float64(n)
	case int16:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case uint:
		return float64(n)
	case uint8:
		return float64(n)
	case uint16:
		return float64(n)
	case uint32:
		return float64(n)
	case uint64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	default:
		return v
	}
}

// compareMaps compares two maps.
func (c *Comparator) compareMaps(basePath string, expected, actual map[string]any, diffs *[]api.FieldDiff) {
	// Check all expected keys
	for key, expValue := range expected {
		path := joinPath(basePath, key)
		actValue, exists := actual[key]

		if !exists {
			*diffs = append(*diffs, api.FieldDiff{
				Path:     path,
				Expected: expValue,
				Type:     api.DiffTypeRemoved,
			})
			continue
		}

		c.compareValues(path, expValue, actValue, diffs)
	}

	// Check for extra keys in actual
	for key, actValue := range actual {
		if _, exists := expected[key]; !exists {
			path := joinPath(basePath, key)
			*diffs = append(*diffs, api.FieldDiff{
				Path:   path,
				Actual: actValue,
				Type:   api.DiffTypeAdded,
			})
		}
	}
}

// compareSlices compares two slices.
// For named arrays (like containers, env, volumes), uses content-based matching by key.
// For other arrays, uses index-based comparison.
func (c *Comparator) compareSlices(basePath string, expected, actual []any, diffs *[]api.FieldDiff) {
	// Check if this is a named array that should be matched by key
	keyField := getArrayKeyField(basePath)
	if keyField != "" && len(expected) > 0 && len(actual) > 0 {
		c.compareNamedSlices(basePath, expected, actual, keyField, diffs)
		return
	}

	// Fall back to index-based comparison for non-named arrays or empty arrays
	if len(expected) != len(actual) {
		*diffs = append(*diffs, api.FieldDiff{
			Path:     basePath,
			Expected: expected,
			Actual:   actual,
			Type:     api.DiffTypeModified,
		})
		return
	}

	// Compare element by element
	for i := range expected {
		path := fmt.Sprintf("%s[%d]", basePath, i)
		c.compareValues(path, expected[i], actual[i], diffs)
	}
}

// compareNamedSlices compares arrays using content-based matching.
// Elements are matched by their key field (e.g., "name" for containers).
func (c *Comparator) compareNamedSlices(basePath string, expected, actual []any, keyField string, diffs *[]api.FieldDiff) {
	result := matchArraysByKey(expected, actual, keyField)

	// Compare matched pairs
	for _, pair := range result.matched {
		// Use the key in the path for clarity (e.g., ".containers[nginx]" instead of ".containers[0]")
		path := fmt.Sprintf("%s[%s]", basePath, pair.key)
		c.compareValues(path, expected[pair.expectedIndex], actual[pair.actualIndex], diffs)
	}

	// Report removed elements (in expected but not in actual)
	for _, idx := range result.removedIndices {
		key, _ := extractKey(expected[idx], keyField)
		path := fmt.Sprintf("%s[%s]", basePath, key)
		*diffs = append(*diffs, api.FieldDiff{
			Path:     path,
			Expected: expected[idx],
			Type:     api.DiffTypeRemoved,
		})
	}

	// Report added elements (in actual but not in expected)
	for _, idx := range result.addedIndices {
		key, _ := extractKey(actual[idx], keyField)
		path := fmt.Sprintf("%s[%s]", basePath, key)
		*diffs = append(*diffs, api.FieldDiff{
			Path:   path,
			Actual: actual[idx],
			Type:   api.DiffTypeAdded,
		})
	}
}

// joinPath joins path segments.
func joinPath(base, key string) string {
	if base == "" {
		return "." + key
	}
	return base + "." + key
}
