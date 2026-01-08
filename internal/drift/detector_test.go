package drift

import (
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/indrasvat/dorikin/pkg/api"
)

func TestFilterByNamespace(t *testing.T) {
	resources := []api.Resource{
		{Object: makeUnstructured("default", "Deployment", "nginx")},
		{Object: makeUnstructured("prod", "Deployment", "web")},
		{Object: makeUnstructured("default", "Service", "nginx-svc")},
		{Object: makeUnstructured("", "ClusterRole", "admin")}, // cluster-scoped
		{Object: makeUnstructured("staging", "ConfigMap", "config")},
	}

	tests := []struct {
		name      string
		namespace string
		wantLen   int
		wantNames []string
	}{
		{
			name:      "filter to default",
			namespace: "default",
			wantLen:   3, // 2 default + 1 cluster-scoped
			wantNames: []string{"nginx", "nginx-svc", "admin"},
		},
		{
			name:      "filter to prod",
			namespace: "prod",
			wantLen:   2, // 1 prod + 1 cluster-scoped
			wantNames: []string{"web", "admin"},
		},
		{
			name:      "filter to nonexistent namespace",
			namespace: "nonexistent",
			wantLen:   1, // only cluster-scoped
			wantNames: []string{"admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterByNamespace(resources, tt.namespace)

			if len(got) != tt.wantLen {
				t.Errorf("len(filterByNamespace()) = %d, want %d", len(got), tt.wantLen)
			}

			for _, wantName := range tt.wantNames {
				found := false
				for _, r := range got {
					if r.Object.GetName() == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("filterByNamespace() missing resource %q", wantName)
				}
			}
		})
	}
}

func TestFilterByNamespace_EmptyInput(t *testing.T) {
	got := filterByNamespace(nil, "default")
	if len(got) != 0 {
		t.Errorf("filterByNamespace(nil) = %d items, want 0", len(got))
	}
}

func TestExtractNamespaces(t *testing.T) {
	tests := []struct {
		name       string
		resources  []api.Resource
		wantNS     []string
		wantLen    int
		uniqueOnly bool
	}{
		{
			name: "multiple namespaces",
			resources: []api.Resource{
				{Object: makeUnstructured("default", "Deployment", "nginx")},
				{Object: makeUnstructured("prod", "Deployment", "web")},
				{Object: makeUnstructured("default", "Service", "nginx-svc")}, // duplicate
				{Object: makeUnstructured("staging", "ConfigMap", "config")},
			},
			wantNS:  []string{"default", "prod", "staging"},
			wantLen: 3,
		},
		{
			name: "excludes cluster-scoped",
			resources: []api.Resource{
				{Object: makeUnstructured("default", "Deployment", "nginx")},
				{Object: makeUnstructured("", "ClusterRole", "admin")},
			},
			wantNS:  []string{"default"},
			wantLen: 1,
		},
		{
			name:      "empty resources",
			resources: nil,
			wantNS:    nil,
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNamespaces(tt.resources)

			if len(got) != tt.wantLen {
				t.Errorf("len(extractNamespaces()) = %d, want %d", len(got), tt.wantLen)
			}

			for _, wantNS := range tt.wantNS {
				if !slices.Contains(got, wantNS) {
					t.Errorf("extractNamespaces() missing namespace %q", wantNS)
				}
			}
		})
	}
}

func TestCalculateSummary(t *testing.T) {
	tests := []struct {
		name    string
		reports []api.DriftReport
		want    api.ScanSummary
	}{
		{
			name:    "empty reports",
			reports: nil,
			want: api.ScanSummary{
				TotalResources: 0,
			},
		},
		{
			name: "all in sync",
			reports: []api.DriftReport{
				{Status: api.StatusInSync},
				{Status: api.StatusInSync},
				{Status: api.StatusInSync},
			},
			want: api.ScanSummary{
				TotalResources: 3,
				InSync:         3,
			},
		},
		{
			name: "mixed statuses",
			reports: []api.DriftReport{
				{Status: api.StatusInSync},
				{Status: api.StatusDrifted},
				{Status: api.StatusMissing},
				{Status: api.StatusExtra},
				{Status: api.StatusError},
			},
			want: api.ScanSummary{
				TotalResources: 5,
				InSync:         1,
				Drifted:        1,
				Missing:        1,
				Extra:          1,
				Errors:         1,
			},
		},
		{
			name: "multiple of each status",
			reports: []api.DriftReport{
				{Status: api.StatusInSync},
				{Status: api.StatusInSync},
				{Status: api.StatusDrifted},
				{Status: api.StatusDrifted},
				{Status: api.StatusDrifted},
				{Status: api.StatusError},
			},
			want: api.ScanSummary{
				TotalResources: 6,
				InSync:         2,
				Drifted:        3,
				Missing:        0,
				Extra:          0,
				Errors:         1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSummary(tt.reports)

			if got.TotalResources != tt.want.TotalResources {
				t.Errorf("TotalResources = %d, want %d", got.TotalResources, tt.want.TotalResources)
			}
			if got.InSync != tt.want.InSync {
				t.Errorf("InSync = %d, want %d", got.InSync, tt.want.InSync)
			}
			if got.Drifted != tt.want.Drifted {
				t.Errorf("Drifted = %d, want %d", got.Drifted, tt.want.Drifted)
			}
			if got.Missing != tt.want.Missing {
				t.Errorf("Missing = %d, want %d", got.Missing, tt.want.Missing)
			}
			if got.Extra != tt.want.Extra {
				t.Errorf("Extra = %d, want %d", got.Extra, tt.want.Extra)
			}
			if got.Errors != tt.want.Errors {
				t.Errorf("Errors = %d, want %d", got.Errors, tt.want.Errors)
			}
		})
	}
}

func TestNewDetector(t *testing.T) {
	ignorePaths := []string{".metadata.uid", ".status"}

	d := NewDetector(nil, ignorePaths) // nil client is OK for this test

	if d == nil {
		t.Fatal("NewDetector() returned nil")
	}
	if d.comparator == nil {
		t.Error("comparator is nil")
	}
	if d.loader == nil {
		t.Error("loader is nil")
	}
	// Verify ignore paths are applied by checking comparator behavior
	// The comparator should ignore .metadata.uid
	expected := map[string]any{"metadata": map[string]any{"uid": "abc", "name": "test"}}
	actual := map[string]any{"metadata": map[string]any{"uid": "def", "name": "test"}}
	diffs := d.comparator.Compare(expected, actual)
	if len(diffs) != 0 {
		t.Error("comparator should ignore .metadata.uid")
	}
}

func TestNewDetectorWithConfig(t *testing.T) {
	d := NewDetectorWithConfig(nil, nil, nil)

	if d == nil {
		t.Fatal("NewDetectorWithConfig() returned nil")
	}
	if d.comparator == nil {
		t.Error("comparator is nil")
	}
}

// makeUnstructured creates a minimal unstructured object for testing.
func makeUnstructured(namespace, kind, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       kind,
			"metadata": map[string]any{
				"name": name,
			},
		},
	}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	return obj
}
