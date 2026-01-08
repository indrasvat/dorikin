package drift

import (
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

// quantityPaths defines field paths that contain Kubernetes quantity values.
// These are normalized before comparison to handle equivalent representations
// like "128Mi" == "134217728" or "500m" == "0.5".
var quantityPaths = map[string]bool{
	// Container resources
	"resources.limits.cpu":      true,
	"resources.limits.memory":   true,
	"resources.requests.cpu":    true,
	"resources.requests.memory": true,

	// Ephemeral storage
	"resources.limits.ephemeral-storage":   true,
	"resources.requests.ephemeral-storage": true,

	// PVC storage
	"resources.requests.storage": true,

	// LimitRange items
	"default.cpu":           true,
	"default.memory":        true,
	"defaultRequest.cpu":    true,
	"defaultRequest.memory": true,
	"max.cpu":               true,
	"max.memory":            true,
	"min.cpu":               true,
	"min.memory":            true,

	// ResourceQuota
	"hard.cpu":                    true,
	"hard.memory":                 true,
	"hard.requests.cpu":           true,
	"hard.requests.memory":        true,
	"hard.limits.cpu":             true,
	"hard.limits.memory":          true,
	"hard.requests.storage":       true,
	"hard.persistentvolumeclaims": true,
}

// quantityPathSuffixes are path suffixes that indicate quantity fields.
// Used for dynamic detection of quantity fields in nested structures.
var quantityPathSuffixes = []string{
	".limits.cpu",
	".limits.memory",
	".limits.ephemeral-storage",
	".requests.cpu",
	".requests.memory",
	".requests.ephemeral-storage",
	".requests.storage",
}

// isQuantityPath checks if a path represents a Kubernetes quantity field.
func isQuantityPath(path string) bool {
	// Remove leading dot and array indices for matching
	cleanPath := cleanPathForQuantityMatch(path)

	// Check exact matches
	if quantityPaths[cleanPath] {
		return true
	}

	// Check suffix matches for nested paths
	for _, suffix := range quantityPathSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}

	return false
}

// cleanPathForQuantityMatch removes leading dots and array indices.
func cleanPathForQuantityMatch(path string) string {
	// Remove leading dot
	path = strings.TrimPrefix(path, ".")

	// Remove array indices like [0], [1], etc.
	re := regexp.MustCompile(`\[\d+\]`)
	path = re.ReplaceAllString(path, "")

	// Get only the last few segments for matching
	// e.g., ".spec.template.spec.containers.resources.limits.cpu" -> "resources.limits.cpu"
	parts := strings.Split(path, ".")
	if len(parts) > 3 {
		// Take last 3 parts for resource paths
		return strings.Join(parts[len(parts)-3:], ".")
	}

	return path
}

// normalizeQuantity attempts to normalize a value as a Kubernetes quantity.
// If successful, returns the canonical string representation.
// If the value is not a valid quantity, returns the original value unchanged.
func normalizeQuantity(v any) any {
	str, ok := v.(string)
	if !ok {
		return v
	}

	// Try to parse as a Kubernetes quantity
	q, err := resource.ParseQuantity(str)
	if err != nil {
		return v
	}

	// Return canonical string representation
	// This normalizes different representations:
	// - "128Mi" and "134217728" both become their canonical form
	// - "500m" and "0.5" both become their canonical form
	return q.String()
}

// compareQuantities compares two values as Kubernetes quantities.
// Returns true if they represent the same quantity, false otherwise.
func compareQuantities(expected, actual any) bool {
	expStr, expOk := expected.(string)
	actStr, actOk := actual.(string)

	if !expOk || !actOk {
		return false
	}

	expQ, expErr := resource.ParseQuantity(expStr)
	actQ, actErr := resource.ParseQuantity(actStr)

	if expErr != nil || actErr != nil {
		return false
	}

	return expQ.Cmp(actQ) == 0
}
