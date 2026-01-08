package drift

import (
	"testing"
)

func TestIsQuantityPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Direct resource paths
		{".spec.template.spec.containers[0].resources.limits.cpu", true},
		{".spec.template.spec.containers[0].resources.limits.memory", true},
		{".spec.template.spec.containers[0].resources.requests.cpu", true},
		{".spec.template.spec.containers[0].resources.requests.memory", true},
		{".spec.template.spec.containers[0].resources.limits.ephemeral-storage", true},
		{".spec.template.spec.containers[0].resources.requests.ephemeral-storage", true},

		// Named container paths (content-based matching)
		{".spec.template.spec.containers[nginx].resources.limits.cpu", true},
		{".spec.template.spec.containers[nginx].resources.limits.memory", true},

		// Init containers
		{".spec.template.spec.initContainers[0].resources.limits.cpu", true},
		{".spec.template.spec.initContainers[init].resources.requests.memory", true},

		// PVC storage
		{".spec.resources.requests.storage", true},

		// Non-quantity paths
		{".spec.replicas", false},
		{".metadata.name", false},
		{".spec.template.spec.containers[0].image", false},
		{".spec.template.spec.containers[0].name", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isQuantityPath(tt.path)
			if got != tt.want {
				t.Errorf("isQuantityPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeQuantity(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		// CPU quantities
		{"cpu millicores", "500m", "500m"},
		{"cpu whole", "1", "1"},
		{"cpu decimal", "0.5", "500m"}, // Normalized to millicores

		// Memory quantities
		{"memory Mi", "128Mi", "128Mi"},
		{"memory Gi", "1Gi", "1Gi"},
		{"memory bytes", "134217728", "134217728"}, // k8s library keeps canonical bytes form

		// Non-quantity strings
		{"not a quantity", "nginx:latest", "nginx:latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeQuantity(tt.input)
			if got != tt.want {
				t.Errorf("normalizeQuantity(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeQuantity_NonString(t *testing.T) {
	// Non-string values should pass through unchanged
	tests := []struct {
		name  string
		input any
	}{
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeQuantity(tt.input)
			if got != tt.input {
				t.Errorf("normalizeQuantity(%v) = %v, want %v", tt.input, got, tt.input)
			}
		})
	}
}

func TestCompareQuantities(t *testing.T) {
	tests := []struct {
		name     string
		expected any
		actual   any
		want     bool
	}{
		// CPU equivalences
		{"cpu 500m == 0.5", "500m", "0.5", true},
		{"cpu 1 == 1000m", "1", "1000m", true},
		{"cpu 250m == 0.25", "250m", "0.25", true},

		// Memory equivalences
		{"memory 128Mi == bytes", "128Mi", "134217728", true},
		{"memory 1Gi == 1024Mi", "1Gi", "1024Mi", true},
		{"memory 512Mi == bytes", "512Mi", "536870912", true},

		// Non-equal quantities
		{"cpu 500m != 1", "500m", "1", false},
		{"memory 128Mi != 256Mi", "128Mi", "256Mi", false},

		// Non-quantity strings
		{"non-quantity strings", "hello", "world", false},

		// Invalid types
		{"int vs string", 500, "500m", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareQuantities(tt.expected, tt.actual)
			if got != tt.want {
				t.Errorf("compareQuantities(%v, %v) = %v, want %v", tt.expected, tt.actual, got, tt.want)
			}
		})
	}
}

func TestCompare_QuantityNormalization(t *testing.T) {
	c := NewComparator(nil)

	// Test CPU normalization in container resources
	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": "nginx",
							"resources": map[string]any{
								"limits": map[string]any{
									"cpu":    "500m",
									"memory": "128Mi",
								},
							},
						},
					},
				},
			},
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": "nginx",
							"resources": map[string]any{
								"limits": map[string]any{
									"cpu":    "0.5",       // Equivalent to 500m
									"memory": "134217728", // Equivalent to 128Mi
								},
							},
						},
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 0 {
		t.Errorf("Compare() returned %d diffs for equivalent quantities, want 0: %v", len(diffs), diffs)
	}
}

func TestCompare_QuantityMismatch(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": "nginx",
							"resources": map[string]any{
								"limits": map[string]any{
									"cpu": "500m",
								},
							},
						},
					},
				},
			},
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": "nginx",
							"resources": map[string]any{
								"limits": map[string]any{
									"cpu": "1", // Different from 500m
								},
							},
						},
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs for different quantities, want 1: %v", len(diffs), diffs)
	}

	// Should report the diff with the actual values
	if diffs[0].Expected != "500m" || diffs[0].Actual != "1" {
		t.Errorf("Expected diff to show original values, got expected=%v actual=%v", diffs[0].Expected, diffs[0].Actual)
	}
}
