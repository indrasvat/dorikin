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
	// Filter both maps
	filteredExpected := c.filter.FilterMap(expected, "")
	filteredActual := c.filter.FilterMap(actual, "")

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

	expectedType := reflect.TypeOf(expected)
	actualType := reflect.TypeOf(actual)

	// Type mismatch
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
func (c *Comparator) compareSlices(basePath string, expected, actual []any, diffs *[]api.FieldDiff) {
	// Simple length check first
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

// joinPath joins path segments.
func joinPath(base, key string) string {
	if base == "" {
		return "." + key
	}
	return base + "." + key
}
