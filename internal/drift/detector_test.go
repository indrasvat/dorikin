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

func TestFilterByKind(t *testing.T) {
	resources := []api.Resource{
		{Object: makeUnstructured("default", "Deployment", "nginx")},
		{Object: makeUnstructured("default", "Service", "nginx-svc")},
		{Object: makeUnstructured("default", "ConfigMap", "config")},
		{Object: makeUnstructured("default", "Secret", "creds")},
		{Object: makeUnstructured("", "ClusterRole", "admin")},
	}

	tests := []struct {
		name      string
		include   []string
		exclude   []string
		wantLen   int
		wantKinds []string
	}{
		{
			name:      "include only Deployment",
			include:   []string{"Deployment"},
			exclude:   nil,
			wantLen:   1,
			wantKinds: []string{"Deployment"},
		},
		{
			name:      "include Deployment and Service",
			include:   []string{"Deployment", "Service"},
			exclude:   nil,
			wantLen:   2,
			wantKinds: []string{"Deployment", "Service"},
		},
		{
			name:      "exclude Secret",
			include:   nil,
			exclude:   []string{"Secret"},
			wantLen:   4,
			wantKinds: []string{"Deployment", "Service", "ConfigMap", "ClusterRole"},
		},
		{
			name:      "exclude multiple kinds",
			include:   nil,
			exclude:   []string{"Secret", "ConfigMap"},
			wantLen:   3,
			wantKinds: []string{"Deployment", "Service", "ClusterRole"},
		},
		{
			name:      "include and exclude combined",
			include:   []string{"Deployment", "Service", "ConfigMap"},
			exclude:   []string{"ConfigMap"},
			wantLen:   2,
			wantKinds: []string{"Deployment", "Service"},
		},
		{
			name:      "no filters returns all",
			include:   nil,
			exclude:   nil,
			wantLen:   5,
			wantKinds: []string{"Deployment", "Service", "ConfigMap", "Secret", "ClusterRole"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterByKind(resources, tt.include, tt.exclude)

			if len(got) != tt.wantLen {
				t.Errorf("len(filterByKind()) = %d, want %d", len(got), tt.wantLen)
			}

			gotKinds := make(map[string]bool)
			for _, r := range got {
				gotKinds[r.Object.GetKind()] = true
			}

			for _, wantKind := range tt.wantKinds {
				if !gotKinds[wantKind] {
					t.Errorf("filterByKind() missing kind %q", wantKind)
				}
			}
		})
	}
}

func TestFilterByKind_EmptyInput(t *testing.T) {
	got := filterByKind(nil, []string{"Deployment"}, nil)
	if len(got) != 0 {
		t.Errorf("filterByKind(nil) = %d items, want 0", len(got))
	}
}

func TestToSet(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  map[string]bool
	}{
		{
			name:  "nil input",
			items: nil,
			want:  nil,
		},
		{
			name:  "empty input",
			items: []string{},
			want:  nil,
		},
		{
			name:  "single item",
			items: []string{"Deployment"},
			want:  map[string]bool{"Deployment": true},
		},
		{
			name:  "multiple items",
			items: []string{"Deployment", "Service"},
			want:  map[string]bool{"Deployment": true, "Service": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toSet(tt.items)

			if tt.want == nil {
				if got != nil {
					t.Errorf("toSet() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("len(toSet()) = %d, want %d", len(got), len(tt.want))
			}

			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("toSet()[%q] = %v, want %v", k, got[k], v)
				}
			}
		})
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
