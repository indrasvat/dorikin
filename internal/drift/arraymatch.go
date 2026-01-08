package drift

import (
	"fmt"
	"strings"
)

// arrayKeyField defines the key field used to identify elements in named arrays.
// When comparing arrays at these paths, elements are matched by their key field
// rather than by index, preventing false positives when array order changes.
type arrayKeyField struct {
	pathSuffix string // Path suffix to match (e.g., ".containers", ".env")
	keyField   string // Field name to use as key (e.g., "name", "containerPort")
}

// namedArrays defines arrays that should be matched by content rather than index.
// Each entry specifies a path suffix and the field to use as the unique key.
var namedArrays = []arrayKeyField{
	// Pod spec arrays
	{".containers", "name"},
	{".initContainers", "name"},
	{".volumes", "name"},
	{".imagePullSecrets", "name"},

	// Container arrays
	{".volumeMounts", "name"},
	{".env", "name"},

	// Container ports - more specific patterns checked first
	{"].ports", "containerPort"}, // Match .containers[name].ports or .containers[0].ports

	// Service ports (direct under .spec, not nested in containers)
	{".spec.ports", "port"},

	// Volume sources (for specific volume types)
	{".configMap.items", "key"},
	{".secret.items", "key"},

	// Ingress rules
	{".rules", "host"},
	{".paths", "path"},

	// RBAC
	{".subjects", "name"},

	// Pod tolerations (match by key)
	{".tolerations", "key"},

	// Node affinity terms
	{".matchExpressions", "key"},
	{".matchFields", "key"},

	// TopologySpreadConstraints
	{".topologySpreadConstraints", "topologyKey"},
}

// getArrayKeyField returns the key field name for a named array path.
// Returns empty string if the path is not a named array.
func getArrayKeyField(path string) string {
	for _, na := range namedArrays {
		if strings.HasSuffix(path, na.pathSuffix) {
			return na.keyField
		}
	}
	return ""
}

// isNamedArray checks if an array at the given path should use content-based matching.
func isNamedArray(path string) bool {
	return getArrayKeyField(path) != ""
}

// extractKey extracts the key value from an array element.
// Returns the key value and true if found, empty string and false otherwise.
func extractKey(element any, keyField string) (string, bool) {
	m, ok := element.(map[string]any)
	if !ok {
		return "", false
	}

	val, exists := m[keyField]
	if !exists {
		return "", false
	}

	// Handle different value types
	switch v := val.(type) {
	case string:
		return v, true
	case int:
		return fmt.Sprintf("%d", v), true
	case int64:
		return fmt.Sprintf("%d", v), true
	case float64:
		// Handle integer-like floats
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), true
		}
		return fmt.Sprintf("%v", v), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

// buildKeyIndex builds a map from key values to array indices.
func buildKeyIndex(arr []any, keyField string) map[string]int {
	index := make(map[string]int)
	for i, elem := range arr {
		if key, ok := extractKey(elem, keyField); ok {
			index[key] = i
		}
	}
	return index
}

// arrayMatchResult represents the result of matching two arrays by key.
type arrayMatchResult struct {
	matched        []matchedPair // Pairs of elements that matched by key
	addedIndices   []int         // Indices in actual that have no match in expected
	removedIndices []int         // Indices in expected that have no match in actual
}

// matchedPair represents a pair of matched array elements.
type matchedPair struct {
	key           string
	expectedIndex int
	actualIndex   int
}

// matchArraysByKey matches two arrays using a key field.
// Returns matched pairs, added indices (in actual), and removed indices (in expected).
func matchArraysByKey(expected, actual []any, keyField string) arrayMatchResult {
	result := arrayMatchResult{}

	expIndex := buildKeyIndex(expected, keyField)
	actIndex := buildKeyIndex(actual, keyField)

	// Find matched pairs
	for key, expIdx := range expIndex {
		if actIdx, found := actIndex[key]; found {
			result.matched = append(result.matched, matchedPair{
				key:           key,
				expectedIndex: expIdx,
				actualIndex:   actIdx,
			})
		} else {
			result.removedIndices = append(result.removedIndices, expIdx)
		}
	}

	// Find added elements (in actual but not in expected)
	for key, actIdx := range actIndex {
		if _, found := expIndex[key]; !found {
			result.addedIndices = append(result.addedIndices, actIdx)
		}
	}

	return result
}
