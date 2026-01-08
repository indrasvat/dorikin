package drift

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/indrasvat/dorikin/pkg/api"
)

func TestNormalizeNumeric(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  any
	}{
		{"int", int(42), float64(42)},
		{"int8", int8(42), float64(42)},
		{"int16", int16(42), float64(42)},
		{"int32", int32(42), float64(42)},
		{"int64", int64(42), float64(42)},
		{"uint", uint(42), float64(42)},
		{"uint8", uint8(42), float64(42)},
		{"uint16", uint16(42), float64(42)},
		{"uint32", uint32(42), float64(42)},
		{"uint64", uint64(42), float64(42)},
		{"float32", float32(3.14), float64(float32(3.14))},
		{"float64", float64(3.14), float64(3.14)},
		{"string", "hello", "hello"},
		{"nil", nil, nil},
		{"bool", true, true},
		{"map", map[string]any{"k": "v"}, map[string]any{"k": "v"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeNumeric(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("normalizeNumeric() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCompare_IdenticalMaps(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "nginx",
			"namespace": "default",
		},
		"spec": map[string]any{
			"replicas": float64(3),
		},
	}

	actual := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "nginx",
			"namespace": "default",
		},
		"spec": map[string]any{
			"replicas": float64(3),
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 0 {
		t.Errorf("Compare() returned %d diffs, want 0: %v", len(diffs), diffs)
	}
}

func TestCompare_ModifiedValue(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"replicas": float64(3),
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"replicas": float64(5),
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1", len(diffs))
	}

	want := api.FieldDiff{
		Path:     ".spec.replicas",
		Expected: float64(3),
		Actual:   float64(5),
		Type:     api.DiffTypeModified,
	}

	if diff := cmp.Diff(want, diffs[0]); diff != "" {
		t.Errorf("Compare() diff mismatch (-want +got):\n%s", diff)
	}
}

func TestCompare_AddedKey(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"replicas": float64(3),
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"replicas":       float64(3),
			"minReadySeconds": float64(10),
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1", len(diffs))
	}

	if diffs[0].Type != api.DiffTypeAdded {
		t.Errorf("diff type = %v, want %v", diffs[0].Type, api.DiffTypeAdded)
	}
	if diffs[0].Path != ".spec.minReadySeconds" {
		t.Errorf("diff path = %q, want %q", diffs[0].Path, ".spec.minReadySeconds")
	}
}

func TestCompare_RemovedKey(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"replicas":       float64(3),
			"minReadySeconds": float64(10),
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"replicas": float64(3),
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1", len(diffs))
	}

	if diffs[0].Type != api.DiffTypeRemoved {
		t.Errorf("diff type = %v, want %v", diffs[0].Type, api.DiffTypeRemoved)
	}
	if diffs[0].Path != ".spec.minReadySeconds" {
		t.Errorf("diff path = %q, want %q", diffs[0].Path, ".spec.minReadySeconds")
	}
}

func TestCompare_NestedMaps(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						"app": "nginx",
					},
				},
			},
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						"app": "apache",
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1", len(diffs))
	}

	if diffs[0].Path != ".spec.template.metadata.labels.app" {
		t.Errorf("diff path = %q, want %q", diffs[0].Path, ".spec.template.metadata.labels.app")
	}
}

func TestCompare_Slices_LengthMismatch(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx"},
			},
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "nginx"},
				map[string]any{"name": "sidecar"},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1", len(diffs))
	}

	if diffs[0].Path != ".spec.containers" {
		t.Errorf("diff path = %q, want %q", diffs[0].Path, ".spec.containers")
	}
	if diffs[0].Type != api.DiffTypeModified {
		t.Errorf("diff type = %v, want %v", diffs[0].Type, api.DiffTypeModified)
	}
}

func TestCompare_Slices_ElementDiff(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx:1.25",
				},
			},
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx:1.26",
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1", len(diffs))
	}

	if diffs[0].Path != ".spec.containers[0].image" {
		t.Errorf("diff path = %q, want %q", diffs[0].Path, ".spec.containers[0].image")
	}
}

func TestCompare_NilHandling(t *testing.T) {
	tests := []struct {
		name     string
		expected any
		actual   any
		wantType api.DiffType
	}{
		{
			name:     "nil expected, non-nil actual",
			expected: nil,
			actual:   "value",
			wantType: api.DiffTypeAdded,
		},
		{
			name:     "non-nil expected, nil actual",
			expected: "value",
			actual:   nil,
			wantType: api.DiffTypeRemoved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewComparator(nil)

			expected := map[string]any{"key": tt.expected}
			actual := map[string]any{"key": tt.actual}

			diffs := c.Compare(expected, actual)

			if len(diffs) != 1 {
				t.Fatalf("Compare() returned %d diffs, want 1", len(diffs))
			}

			if diffs[0].Type != tt.wantType {
				t.Errorf("diff type = %v, want %v", diffs[0].Type, tt.wantType)
			}
		})
	}
}

func TestCompare_BothNil(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{"key": nil}
	actual := map[string]any{"key": nil}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 0 {
		t.Errorf("Compare() returned %d diffs for nil==nil, want 0", len(diffs))
	}
}

func TestCompareWithDynamicIgnore(t *testing.T) {
	c := NewComparator([]string{".metadata.uid"})

	expected := map[string]any{
		"metadata": map[string]any{
			"name": "nginx",
			"uid":  "abc-123",
		},
		"spec": map[string]any{
			"replicas": float64(3),
		},
	}

	actual := map[string]any{
		"metadata": map[string]any{
			"name": "nginx",
			"uid":  "def-456",
		},
		"spec": map[string]any{
			"replicas": float64(5),
		},
	}

	// Without dynamic ignore, should see replicas diff
	diffs := c.Compare(expected, actual)
	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1", len(diffs))
	}
	if diffs[0].Path != ".spec.replicas" {
		t.Errorf("diff path = %q, want .spec.replicas", diffs[0].Path)
	}

	// With dynamic ignore for replicas, should see no diff
	diffs = c.CompareWithDynamicIgnore(expected, actual, []string{".spec.replicas"})
	if len(diffs) != 0 {
		t.Errorf("CompareWithDynamicIgnore() returned %d diffs, want 0", len(diffs))
	}
}

func TestCompare_NumericTypeNormalization(t *testing.T) {
	c := NewComparator(nil)

	// YAML typically parses integers as int, JSON as float64
	expected := map[string]any{
		"spec": map[string]any{
			"replicas": int(3), // YAML style
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"replicas": float64(3), // JSON style
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 0 {
		t.Errorf("Compare() returned %d diffs for int vs float64, want 0: %v", len(diffs), diffs)
	}
}

func TestCompare_TypeMismatch(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"key": "3", // string
	}

	actual := map[string]any{
		"key": float64(3), // number
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1", len(diffs))
	}

	if diffs[0].Type != api.DiffTypeModified {
		t.Errorf("diff type = %v, want %v", diffs[0].Type, api.DiffTypeModified)
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		base string
		key  string
		want string
	}{
		{"", "metadata", ".metadata"},
		{".metadata", "name", ".metadata.name"},
		{".spec.template", "spec", ".spec.template.spec"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := joinPath(tt.base, tt.key)
			if got != tt.want {
				t.Errorf("joinPath(%q, %q) = %q, want %q", tt.base, tt.key, got, tt.want)
			}
		})
	}
}

func TestCompare_IgnorePathsApplied(t *testing.T) {
	c := NewComparator([]string{
		".metadata.uid",
		".metadata.resourceVersion",
		".metadata.creationTimestamp",
	})

	expected := map[string]any{
		"metadata": map[string]any{
			"name":              "nginx",
			"uid":               "abc-123",
			"resourceVersion":   "12345",
			"creationTimestamp": "2024-01-01T00:00:00Z",
		},
	}

	actual := map[string]any{
		"metadata": map[string]any{
			"name":              "nginx",
			"uid":               "def-456",
			"resourceVersion":   "67890",
			"creationTimestamp": "2024-06-01T00:00:00Z",
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 0 {
		t.Errorf("Compare() returned %d diffs (should ignore server-generated fields), got: %v", len(diffs), diffs)
	}
}

func TestCompare_DiffsSorted(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"z": "z",
		"a": "a",
		"m": "m",
	}

	actual := map[string]any{
		"z": "z-changed",
		"a": "a-changed",
		"m": "m-changed",
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 3 {
		t.Fatalf("Compare() returned %d diffs, want 3", len(diffs))
	}

	// Diffs should be sorted by path
	if diffs[0].Path != ".a" || diffs[1].Path != ".m" || diffs[2].Path != ".z" {
		t.Errorf("diffs not sorted: %v, %v, %v", diffs[0].Path, diffs[1].Path, diffs[2].Path)
	}
}
