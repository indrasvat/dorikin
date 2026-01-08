package drift

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewFilter(t *testing.T) {
	tests := []struct {
		name        string
		ignorePaths []string
		wantPaths   []string
	}{
		{
			name:        "empty paths",
			ignorePaths: nil,
			wantPaths:   nil,
		},
		{
			name:        "single path",
			ignorePaths: []string{".metadata.uid"},
			wantPaths:   []string{".metadata.uid"},
		},
		{
			name:        "normalizes paths without leading dot",
			ignorePaths: []string{"metadata.uid", "spec.replicas"},
			wantPaths:   []string{".metadata.uid", ".spec.replicas"},
		},
		{
			name:        "preserves paths with leading dot",
			ignorePaths: []string{".metadata.uid", ".spec.replicas"},
			wantPaths:   []string{".metadata.uid", ".spec.replicas"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.ignorePaths)

			for _, want := range tt.wantPaths {
				if !f.ignorePaths[want] {
					t.Errorf("NewFilter() missing expected path %q", want)
				}
			}
		})
	}
}

func TestShouldIgnore_ExactMatch(t *testing.T) {
	tests := []struct {
		name        string
		ignorePaths []string
		testPath    string
		want        bool
	}{
		{
			name:        "exact match with leading dot",
			ignorePaths: []string{".metadata.uid"},
			testPath:    ".metadata.uid",
			want:        true,
		},
		{
			name:        "exact match without leading dot in test path",
			ignorePaths: []string{".metadata.uid"},
			testPath:    "metadata.uid",
			want:        true,
		},
		{
			name:        "exact match without leading dot in ignore path",
			ignorePaths: []string{"metadata.uid"},
			testPath:    ".metadata.uid",
			want:        true,
		},
		{
			name:        "no match",
			ignorePaths: []string{".metadata.uid"},
			testPath:    ".metadata.name",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.ignorePaths)
			got := f.ShouldIgnore(tt.testPath)
			if got != tt.want {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.testPath, got, tt.want)
			}
		})
	}
}

func TestShouldIgnore_PrefixMatch(t *testing.T) {
	tests := []struct {
		name        string
		ignorePaths []string
		testPath    string
		want        bool
	}{
		{
			name:        "prefix match with dot",
			ignorePaths: []string{".metadata"},
			testPath:    ".metadata.uid",
			want:        true,
		},
		{
			name:        "prefix match nested",
			ignorePaths: []string{".spec.template"},
			testPath:    ".spec.template.metadata.labels.app",
			want:        true,
		},
		{
			name:        "prefix match with array",
			ignorePaths: []string{".spec.containers"},
			testPath:    ".spec.containers[0].image",
			want:        true,
		},
		{
			name:        "partial name not a prefix",
			ignorePaths: []string{".spec.rep"},
			testPath:    ".spec.replicas",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.ignorePaths)
			got := f.ShouldIgnore(tt.testPath)
			if got != tt.want {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.testPath, got, tt.want)
			}
		})
	}
}

func TestShouldIgnore_ArrayIndexStripping(t *testing.T) {
	tests := []struct {
		name        string
		ignorePaths []string
		testPath    string
		want        bool
	}{
		{
			name:        "path with index matches path without index in ignore",
			ignorePaths: []string{".spec.containers"},
			testPath:    ".spec.containers[0]",
			want:        true,
		},
		{
			name:        "nested array path",
			ignorePaths: []string{".spec.containers.ports"},
			testPath:    ".spec.containers[0].ports[1]",
			want:        true,
		},
		{
			name:        "deep nesting with arrays",
			ignorePaths: []string{".spec.containers.env.valueFrom"},
			testPath:    ".spec.containers[0].env[2].valueFrom",
			want:        true,
		},
		{
			name:        "array in ignore path matches literal",
			ignorePaths: []string{".spec.containers[0]"},
			testPath:    ".spec.containers[0]",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.ignorePaths)
			got := f.ShouldIgnore(tt.testPath)
			if got != tt.want {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.testPath, got, tt.want)
			}
		})
	}
}

func TestShouldIgnore_NoMatch(t *testing.T) {
	f := NewFilter([]string{
		".metadata.uid",
		".metadata.resourceVersion",
		".status",
	})

	paths := []string{
		".metadata.name",
		".metadata.namespace",
		".spec.replicas",
		".spec.template.spec.containers",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			if f.ShouldIgnore(path) {
				t.Errorf("ShouldIgnore(%q) = true, want false", path)
			}
		})
	}
}

func TestWithAdditionalPaths_Immutability(t *testing.T) {
	original := NewFilter([]string{".metadata.uid"})
	extended := original.WithAdditionalPaths([]string{".status"})

	// Original should not have the new path
	if original.ShouldIgnore(".status") {
		t.Error("original filter should not ignore .status")
	}

	// Extended should have both paths
	if !extended.ShouldIgnore(".metadata.uid") {
		t.Error("extended filter should ignore .metadata.uid")
	}
	if !extended.ShouldIgnore(".status") {
		t.Error("extended filter should ignore .status")
	}
}

func TestWithAdditionalPaths_NormalizesNewPaths(t *testing.T) {
	original := NewFilter([]string{".metadata.uid"})
	extended := original.WithAdditionalPaths([]string{"status"}) // no leading dot

	if !extended.ShouldIgnore(".status") {
		t.Error("extended filter should ignore .status (normalized)")
	}
}

func TestFilterMap_NestedMaps(t *testing.T) {
	f := NewFilter([]string{".metadata.uid", ".metadata.resourceVersion"})

	input := map[string]any{
		"metadata": map[string]any{
			"name":            "test",
			"namespace":       "default",
			"uid":             "abc-123",
			"resourceVersion": "12345",
		},
		"spec": map[string]any{
			"replicas": 3,
		},
	}

	want := map[string]any{
		"metadata": map[string]any{
			"name":      "test",
			"namespace": "default",
		},
		"spec": map[string]any{
			"replicas": 3,
		},
	}

	got := f.FilterMap(input, "")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FilterMap() mismatch (-want +got):\n%s", diff)
	}
}

func TestFilterMap_WithSlices(t *testing.T) {
	f := NewFilter([]string{".spec.containers.resources"})

	input := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx:1.25",
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu": "100m",
						},
					},
				},
				map[string]any{
					"name":  "sidecar",
					"image": "sidecar:latest",
					"resources": map[string]any{
						"limits": map[string]any{
							"memory": "128Mi",
						},
					},
				},
			},
		},
	}

	want := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx:1.25",
				},
				map[string]any{
					"name":  "sidecar",
					"image": "sidecar:latest",
				},
			},
		},
	}

	got := f.FilterMap(input, "")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FilterMap() mismatch (-want +got):\n%s", diff)
	}
}

func TestFilterMap_RemovesEmptyNestedMaps(t *testing.T) {
	f := NewFilter([]string{".metadata.annotations"})

	input := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"key": "value",
			},
		},
		"spec": map[string]any{
			"replicas": 1,
		},
	}

	// metadata should be empty after removing annotations, so it should be excluded
	want := map[string]any{
		"spec": map[string]any{
			"replicas": 1,
		},
	}

	got := f.FilterMap(input, "")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FilterMap() mismatch (-want +got):\n%s", diff)
	}
}

func TestNormalizeJSONPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"metadata.uid", ".metadata.uid"},
		{".metadata.uid", ".metadata.uid"},
		{"spec.replicas", ".spec.replicas"},
		{".spec.replicas", ".spec.replicas"},
		{"", "."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeJSONPath(tt.input)
			if got != tt.want {
				t.Errorf("normalizeJSONPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripArrayIndices(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{".spec.containers[0]", ".spec.containers"},
		{".spec.containers[0].ports[1]", ".spec.containers.ports"},
		{".spec.containers[0].env[2].valueFrom", ".spec.containers.env.valueFrom"},
		{".spec.containers[123].image", ".spec.containers.image"},
		{".metadata.name", ".metadata.name"},
		{".spec.volumes", ".spec.volumes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripArrayIndices(tt.input)
			if got != tt.want {
				t.Errorf("stripArrayIndices(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{9999, "9999"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := itoa(tt.input)
			if got != tt.want {
				t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
