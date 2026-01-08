package drift

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/indrasvat/dorikin/pkg/api"
)

func TestGetArrayKeyField(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		// Container arrays
		{".spec.template.spec.containers", "name"},
		{".spec.template.spec.initContainers", "name"},
		{".spec.containers", "name"},

		// Volume arrays
		{".spec.template.spec.volumes", "name"},
		{".spec.volumes", "name"},

		// Container sub-arrays
		{".spec.template.spec.containers[0].volumeMounts", "name"},
		{".spec.template.spec.containers[nginx].volumeMounts", "name"},
		{".spec.template.spec.containers[0].env", "name"},
		{".spec.template.spec.containers[0].ports", "containerPort"},

		// Service ports
		{".spec.ports", "port"},

		// Non-named arrays
		{".spec.template.spec.containers[0].args", ""},
		{".spec.template.spec.containers[0].command", ""},
		{".data", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := getArrayKeyField(tt.path)
			if got != tt.want {
				t.Errorf("getArrayKeyField(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsNamedArray(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".spec.template.spec.containers", true},
		{".spec.template.spec.volumes", true},
		{".spec.template.spec.containers[0].env", true},
		{".spec.template.spec.containers[0].args", false},
		{".metadata.labels", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isNamedArray(tt.path)
			if got != tt.want {
				t.Errorf("isNamedArray(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractKey(t *testing.T) {
	tests := []struct {
		name     string
		element  any
		keyField string
		wantKey  string
		wantOk   bool
	}{
		{
			name:     "string key",
			element:  map[string]any{"name": "nginx", "image": "nginx:latest"},
			keyField: "name",
			wantKey:  "nginx",
			wantOk:   true,
		},
		{
			name:     "int key",
			element:  map[string]any{"containerPort": 8080, "protocol": "TCP"},
			keyField: "containerPort",
			wantKey:  "8080",
			wantOk:   true,
		},
		{
			name:     "float64 key (integer-like)",
			element:  map[string]any{"port": float64(80), "targetPort": float64(8080)},
			keyField: "port",
			wantKey:  "80",
			wantOk:   true,
		},
		{
			name:     "missing key",
			element:  map[string]any{"image": "nginx:latest"},
			keyField: "name",
			wantKey:  "",
			wantOk:   false,
		},
		{
			name:     "non-map element",
			element:  "string-element",
			keyField: "name",
			wantKey:  "",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotOk := extractKey(tt.element, tt.keyField)
			if gotKey != tt.wantKey || gotOk != tt.wantOk {
				t.Errorf("extractKey() = (%q, %v), want (%q, %v)", gotKey, gotOk, tt.wantKey, tt.wantOk)
			}
		})
	}
}

func TestMatchArraysByKey(t *testing.T) {
	tests := []struct {
		name            string
		expected        []any
		actual          []any
		keyField        string
		wantMatchedKeys []string
		wantAdded       int
		wantRemoved     int
	}{
		{
			name: "identical arrays",
			expected: []any{
				map[string]any{"name": "nginx"},
				map[string]any{"name": "redis"},
			},
			actual: []any{
				map[string]any{"name": "nginx"},
				map[string]any{"name": "redis"},
			},
			keyField:        "name",
			wantMatchedKeys: []string{"nginx", "redis"},
			wantAdded:       0,
			wantRemoved:     0,
		},
		{
			name: "reordered arrays",
			expected: []any{
				map[string]any{"name": "nginx"},
				map[string]any{"name": "redis"},
			},
			actual: []any{
				map[string]any{"name": "redis"},
				map[string]any{"name": "nginx"},
			},
			keyField:        "name",
			wantMatchedKeys: []string{"nginx", "redis"},
			wantAdded:       0,
			wantRemoved:     0,
		},
		{
			name: "added element",
			expected: []any{
				map[string]any{"name": "nginx"},
			},
			actual: []any{
				map[string]any{"name": "nginx"},
				map[string]any{"name": "sidecar"},
			},
			keyField:        "name",
			wantMatchedKeys: []string{"nginx"},
			wantAdded:       1,
			wantRemoved:     0,
		},
		{
			name: "removed element",
			expected: []any{
				map[string]any{"name": "nginx"},
				map[string]any{"name": "redis"},
			},
			actual: []any{
				map[string]any{"name": "nginx"},
			},
			keyField:        "name",
			wantMatchedKeys: []string{"nginx"},
			wantAdded:       0,
			wantRemoved:     1,
		},
		{
			name: "replaced element",
			expected: []any{
				map[string]any{"name": "nginx"},
			},
			actual: []any{
				map[string]any{"name": "apache"},
			},
			keyField:        "name",
			wantMatchedKeys: []string{},
			wantAdded:       1,
			wantRemoved:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchArraysByKey(tt.expected, tt.actual, tt.keyField)

			// Check matched count
			if len(result.matched) != len(tt.wantMatchedKeys) {
				t.Errorf("matched count = %d, want %d", len(result.matched), len(tt.wantMatchedKeys))
			}

			// Check added count
			if len(result.addedIndices) != tt.wantAdded {
				t.Errorf("added count = %d, want %d", len(result.addedIndices), tt.wantAdded)
			}

			// Check removed count
			if len(result.removedIndices) != tt.wantRemoved {
				t.Errorf("removed count = %d, want %d", len(result.removedIndices), tt.wantRemoved)
			}
		})
	}
}

func TestCompare_ContentBasedArrayMatching_Reordered(t *testing.T) {
	c := NewComparator(nil)

	// Same containers, different order - should report NO diff
	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx:1.25"},
						map[string]any{"name": "redis", "image": "redis:7"},
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
						map[string]any{"name": "redis", "image": "redis:7"},
						map[string]any{"name": "nginx", "image": "nginx:1.25"},
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 0 {
		t.Errorf("Compare() returned %d diffs for reordered containers, want 0: %v", len(diffs), diffs)
	}
}

func TestCompare_ContentBasedArrayMatching_Modified(t *testing.T) {
	c := NewComparator(nil)

	// Same containers (reordered), but one has different image
	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx:1.25"},
						map[string]any{"name": "redis", "image": "redis:7"},
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
						map[string]any{"name": "redis", "image": "redis:7"},
						map[string]any{"name": "nginx", "image": "nginx:1.26"}, // Changed!
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1: %v", len(diffs), diffs)
	}

	// Should identify the nginx container's image change
	if diffs[0].Path != ".spec.template.spec.containers[nginx].image" {
		t.Errorf("diff path = %q, want .spec.template.spec.containers[nginx].image", diffs[0].Path)
	}
	if diffs[0].Expected != "nginx:1.25" || diffs[0].Actual != "nginx:1.26" {
		t.Errorf("diff values = %v -> %v, want nginx:1.25 -> nginx:1.26", diffs[0].Expected, diffs[0].Actual)
	}
}

func TestCompare_ContentBasedArrayMatching_Added(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx:1.25"},
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
						map[string]any{"name": "nginx", "image": "nginx:1.25"},
						map[string]any{"name": "sidecar", "image": "envoy:latest"}, // Added
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1: %v", len(diffs), diffs)
	}

	if diffs[0].Type != api.DiffTypeAdded {
		t.Errorf("diff type = %v, want %v", diffs[0].Type, api.DiffTypeAdded)
	}
	if diffs[0].Path != ".spec.template.spec.containers[sidecar]" {
		t.Errorf("diff path = %q, want .spec.template.spec.containers[sidecar]", diffs[0].Path)
	}
}

func TestCompare_ContentBasedArrayMatching_Removed(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx:1.25"},
						map[string]any{"name": "sidecar", "image": "envoy:latest"},
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
						map[string]any{"name": "nginx", "image": "nginx:1.25"},
						// sidecar removed
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 1 {
		t.Fatalf("Compare() returned %d diffs, want 1: %v", len(diffs), diffs)
	}

	if diffs[0].Type != api.DiffTypeRemoved {
		t.Errorf("diff type = %v, want %v", diffs[0].Type, api.DiffTypeRemoved)
	}
	if diffs[0].Path != ".spec.template.spec.containers[sidecar]" {
		t.Errorf("diff path = %q, want .spec.template.spec.containers[sidecar]", diffs[0].Path)
	}
}

func TestCompare_EnvVarsReordered(t *testing.T) {
	c := NewComparator(nil)

	// Same env vars, different order - should report NO diff
	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": "app",
							"env": []any{
								map[string]any{"name": "DB_HOST", "value": "localhost"},
								map[string]any{"name": "DB_PORT", "value": "5432"},
								map[string]any{"name": "DEBUG", "value": "false"},
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
							"name": "app",
							"env": []any{
								map[string]any{"name": "DEBUG", "value": "false"},
								map[string]any{"name": "DB_HOST", "value": "localhost"},
								map[string]any{"name": "DB_PORT", "value": "5432"},
							},
						},
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 0 {
		t.Errorf("Compare() returned %d diffs for reordered env vars, want 0: %v", len(diffs), diffs)
	}
}

func TestCompare_VolumeMountsReordered(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": "app",
							"volumeMounts": []any{
								map[string]any{"name": "config", "mountPath": "/etc/config"},
								map[string]any{"name": "data", "mountPath": "/data"},
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
							"name": "app",
							"volumeMounts": []any{
								map[string]any{"name": "data", "mountPath": "/data"},
								map[string]any{"name": "config", "mountPath": "/etc/config"},
							},
						},
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 0 {
		t.Errorf("Compare() returned %d diffs for reordered volumeMounts, want 0: %v", len(diffs), diffs)
	}
}

func TestCompare_VolumesReordered(t *testing.T) {
	c := NewComparator(nil)

	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"volumes": []any{
						map[string]any{"name": "config", "configMap": map[string]any{"name": "app-config"}},
						map[string]any{"name": "data", "emptyDir": map[string]any{}},
					},
				},
			},
		},
	}

	actual := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"volumes": []any{
						map[string]any{"name": "data", "emptyDir": map[string]any{}},
						map[string]any{"name": "config", "configMap": map[string]any{"name": "app-config"}},
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	if len(diffs) != 0 {
		t.Errorf("Compare() returned %d diffs for reordered volumes, want 0: %v", len(diffs), diffs)
	}
}

func TestCompare_NonNamedArrayStillIndexBased(t *testing.T) {
	c := NewComparator(nil)

	// Command args are NOT named - should still be index-based
	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":    "app",
							"command": []any{"/bin/sh", "-c"},
							"args":    []any{"echo", "hello"},
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
							"name":    "app",
							"command": []any{"-c", "/bin/sh"}, // Reordered
							"args":    []any{"hello", "echo"}, // Reordered
						},
					},
				},
			},
		},
	}

	diffs := c.Compare(expected, actual)

	// Should detect differences because command/args are not named arrays
	// Element-by-element comparison: command[0], command[1], args[0], args[1] = 4 diffs
	if len(diffs) != 4 {
		t.Errorf("Compare() returned %d diffs for reordered command/args, want 4: %v", len(diffs), diffs)
	}
}

func TestCleanPathForQuantityMatch(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{".spec.template.spec.containers[0].resources.limits.cpu", "resources.limits.cpu"},
		{".spec.template.spec.containers[nginx].resources.limits.memory", "resources.limits.memory"},
		{".resources.requests.storage", "resources.requests.storage"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := cleanPathForQuantityMatch(tt.path)
			if got != tt.want {
				t.Errorf("cleanPathForQuantityMatch(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// Verify the integration works end-to-end
func TestCompare_CombinedFeatures(t *testing.T) {
	c := NewComparator(nil)

	// Test both features together: reordered containers with quantity values
	expected := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "nginx",
							"image": "nginx:1.25",
							"resources": map[string]any{
								"limits": map[string]any{
									"cpu":    "500m",
									"memory": "128Mi",
								},
							},
						},
						map[string]any{
							"name":  "redis",
							"image": "redis:7",
							"resources": map[string]any{
								"limits": map[string]any{
									"cpu":    "1",
									"memory": "256Mi",
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
						// Reordered + equivalent quantities
						map[string]any{
							"name":  "redis",
							"image": "redis:7",
							"resources": map[string]any{
								"limits": map[string]any{
									"cpu":    "1000m",     // Equivalent to "1"
									"memory": "268435456", // Equivalent to 256Mi
								},
							},
						},
						map[string]any{
							"name":  "nginx",
							"image": "nginx:1.25",
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
		for _, d := range diffs {
			t.Logf("Unexpected diff: path=%s type=%s expected=%v actual=%v", d.Path, d.Type, d.Expected, d.Actual)
		}
		t.Errorf("Compare() returned %d diffs for reordered containers with equivalent quantities, want 0", len(diffs))
	}
}

var _ = cmp.Diff // silence import warning
