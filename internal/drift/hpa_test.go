package drift

import (
	"math"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/indrasvat/dorikin/pkg/api"
)

func TestNewHPATargetIndex(t *testing.T) {
	idx := NewHPATargetIndex()

	if idx == nil {
		t.Fatal("NewHPATargetIndex() returned nil")
	}
	if idx.targets == nil {
		t.Error("targets map is nil")
	}
	if len(idx.targets) != 0 {
		t.Errorf("targets map has %d entries, want 0", len(idx.targets))
	}
}

func TestHPATargetIndex_Add(t *testing.T) {
	idx := NewHPATargetIndex()

	key := HPATargetKey{
		Namespace: "default",
		Kind:      "Deployment",
		Name:      "nginx",
	}
	info := HPATargetInfo{
		HPAName:     "nginx-hpa",
		MinReplicas: 2,
		MaxReplicas: 10,
	}

	idx.Add(key, info)

	if len(idx.targets) != 1 {
		t.Errorf("targets map has %d entries, want 1", len(idx.targets))
	}
	got, ok := idx.targets[key]
	if !ok {
		t.Error("target not found after Add")
	}
	if got.HPAName != "nginx-hpa" {
		t.Errorf("HPAName = %q, want %q", got.HPAName, "nginx-hpa")
	}
	if got.MinReplicas != 2 {
		t.Errorf("MinReplicas = %d, want 2", got.MinReplicas)
	}
	if got.MaxReplicas != 10 {
		t.Errorf("MaxReplicas = %d, want 10", got.MaxReplicas)
	}
}

func TestHPATargetIndex_Merge(t *testing.T) {
	idx1 := NewHPATargetIndex()
	idx1.Add(HPATargetKey{Namespace: "default", Kind: "Deployment", Name: "app1"}, HPATargetInfo{HPAName: "hpa1", MinReplicas: 1, MaxReplicas: 5})
	idx1.Add(HPATargetKey{Namespace: "default", Kind: "Deployment", Name: "app2"}, HPATargetInfo{HPAName: "hpa2", MinReplicas: 2, MaxReplicas: 6})

	idx2 := NewHPATargetIndex()
	idx2.Add(HPATargetKey{Namespace: "default", Kind: "Deployment", Name: "app2"}, HPATargetInfo{HPAName: "hpa2-updated", MinReplicas: 3, MaxReplicas: 10}) // Overwrites
	idx2.Add(HPATargetKey{Namespace: "default", Kind: "Deployment", Name: "app3"}, HPATargetInfo{HPAName: "hpa3", MinReplicas: 1, MaxReplicas: 8})

	idx1.Merge(idx2)

	if len(idx1.targets) != 3 {
		t.Errorf("targets map has %d entries, want 3", len(idx1.targets))
	}

	// Verify overwrite
	app2Key := HPATargetKey{Namespace: "default", Kind: "Deployment", Name: "app2"}
	if info, ok := idx1.targets[app2Key]; !ok {
		t.Error("app2 not found after merge")
	} else if info.HPAName != "hpa2-updated" {
		t.Errorf("app2 HPAName = %q, want %q (overwritten value)", info.HPAName, "hpa2-updated")
	}
}

func TestHPATargetIndex_Merge_Nil(t *testing.T) {
	idx := NewHPATargetIndex()
	idx.Add(HPATargetKey{Namespace: "default", Kind: "Deployment", Name: "app1"}, HPATargetInfo{HPAName: "hpa1"})

	// Should not panic
	idx.Merge(nil)

	if len(idx.targets) != 1 {
		t.Errorf("targets map has %d entries, want 1 (unchanged)", len(idx.targets))
	}
}

func TestIsHPAManaged(t *testing.T) {
	idx := NewHPATargetIndex()
	idx.Add(HPATargetKey{Namespace: "default", Kind: "Deployment", Name: "nginx"}, HPATargetInfo{HPAName: "nginx-hpa"})

	tests := []struct {
		name string
		ref  api.ResourceRef
		want bool
	}{
		{
			name: "managed deployment",
			ref:  api.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "nginx"},
			want: true,
		},
		{
			name: "unmanaged deployment",
			ref:  api.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "apache"},
			want: false,
		},
		{
			name: "wrong namespace",
			ref:  api.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "nginx"},
			want: false,
		},
		{
			name: "wrong kind",
			ref:  api.ResourceRef{Namespace: "default", Kind: "StatefulSet", Name: "nginx"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idx.IsHPAManaged(tt.ref)
			if got != tt.want {
				t.Errorf("IsHPAManaged(%v) = %v, want %v", tt.ref, got, tt.want)
			}
		})
	}
}

func TestIsHPAManaged_NilIndex(t *testing.T) {
	var idx *HPATargetIndex
	ref := api.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "nginx"}

	if idx.IsHPAManaged(ref) {
		t.Error("IsHPAManaged on nil index should return false")
	}
}

func TestGetHPAInfo(t *testing.T) {
	idx := NewHPATargetIndex()
	idx.Add(HPATargetKey{Namespace: "default", Kind: "Deployment", Name: "nginx"}, HPATargetInfo{HPAName: "nginx-hpa", MinReplicas: 2, MaxReplicas: 10})

	t.Run("found", func(t *testing.T) {
		ref := api.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "nginx"}
		info := idx.GetHPAInfo(ref)
		if info == nil {
			t.Fatal("GetHPAInfo() returned nil, want info")
		}
		if info.HPAName != "nginx-hpa" {
			t.Errorf("HPAName = %q, want %q", info.HPAName, "nginx-hpa")
		}
		if info.MinReplicas != 2 {
			t.Errorf("MinReplicas = %d, want 2", info.MinReplicas)
		}
		if info.MaxReplicas != 10 {
			t.Errorf("MaxReplicas = %d, want 10", info.MaxReplicas)
		}
	})

	t.Run("not found", func(t *testing.T) {
		ref := api.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "apache"}
		info := idx.GetHPAInfo(ref)
		if info != nil {
			t.Errorf("GetHPAInfo() = %v, want nil", info)
		}
	})
}

func TestGetHPAInfo_NilIndex(t *testing.T) {
	var idx *HPATargetIndex
	ref := api.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "nginx"}

	if idx.GetHPAInfo(ref) != nil {
		t.Error("GetHPAInfo on nil index should return nil")
	}
}

func TestHPATargetIndex_Size(t *testing.T) {
	tests := []struct {
		name    string
		entries int
	}{
		{"empty", 0},
		{"one entry", 1},
		{"multiple entries", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := NewHPATargetIndex()
			for i := 0; i < tt.entries; i++ {
				idx.Add(HPATargetKey{Namespace: "default", Kind: "Deployment", Name: string(rune('a' + i))}, HPATargetInfo{})
			}
			if got := idx.Size(); got != tt.entries {
				t.Errorf("Size() = %d, want %d", got, tt.entries)
			}
		})
	}
}

func TestHPATargetIndex_Size_Nil(t *testing.T) {
	var idx *HPATargetIndex
	if idx.Size() != 0 {
		t.Error("Size() on nil index should return 0")
	}
}

func TestClampToInt32(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  int32
	}{
		{"normal positive", 100, 100},
		{"normal negative", -100, -100},
		{"zero", 0, 0},
		{"max int32", math.MaxInt32, math.MaxInt32},
		{"min int32", math.MinInt32, math.MinInt32},
		{"above max int32", math.MaxInt32 + 1, math.MaxInt32},
		{"below min int32", math.MinInt32 - 1, math.MinInt32},
		{"large positive", math.MaxInt64, math.MaxInt32},
		{"large negative", math.MinInt64, math.MinInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampToInt32(tt.input)
			if got != tt.want {
				t.Errorf("clampToInt32(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsHPA(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		apiVersion string
		want       bool
	}{
		{"autoscaling/v1", "HorizontalPodAutoscaler", "autoscaling/v1", true},
		{"autoscaling/v2", "HorizontalPodAutoscaler", "autoscaling/v2", true},
		{"autoscaling/v2beta2", "HorizontalPodAutoscaler", "autoscaling/v2beta2", true},
		{"wrong kind", "Deployment", "autoscaling/v2", false},
		{"wrong apiVersion", "HorizontalPodAutoscaler", "apps/v1", false},
		{"empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetKind(tt.kind)
			obj.SetAPIVersion(tt.apiVersion)

			got := isHPA(obj)
			if got != tt.want {
				t.Errorf("isHPA() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsHPA_Nil(t *testing.T) {
	if isHPA(nil) {
		t.Error("isHPA(nil) should return false")
	}
}

func TestParseHPA_V2(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]any{
				"name":      "nginx-hpa",
				"namespace": "default",
			},
			"spec": map[string]any{
				"scaleTargetRef": map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "nginx",
				},
				"minReplicas": int64(2),
				"maxReplicas": int64(10),
			},
		},
	}

	key, info, ok := parseHPA(obj)

	if !ok {
		t.Fatal("parseHPA() returned ok=false")
	}
	if key.Namespace != "default" {
		t.Errorf("key.Namespace = %q, want %q", key.Namespace, "default")
	}
	if key.Kind != "Deployment" {
		t.Errorf("key.Kind = %q, want %q", key.Kind, "Deployment")
	}
	if key.Name != "nginx" {
		t.Errorf("key.Name = %q, want %q", key.Name, "nginx")
	}
	if info.HPAName != "nginx-hpa" {
		t.Errorf("info.HPAName = %q, want %q", info.HPAName, "nginx-hpa")
	}
	if info.MinReplicas != 2 {
		t.Errorf("info.MinReplicas = %d, want 2", info.MinReplicas)
	}
	if info.MaxReplicas != 10 {
		t.Errorf("info.MaxReplicas = %d, want 10", info.MaxReplicas)
	}
}

func TestParseHPA_DefaultMinReplicas(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "autoscaling/v1",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]any{
				"name":      "nginx-hpa",
				"namespace": "default",
			},
			"spec": map[string]any{
				"scaleTargetRef": map[string]any{
					"kind": "Deployment",
					"name": "nginx",
				},
				// minReplicas not specified
				"maxReplicas": int64(10),
			},
		},
	}

	_, info, ok := parseHPA(obj)

	if !ok {
		t.Fatal("parseHPA() returned ok=false")
	}
	if info.MinReplicas != 1 {
		t.Errorf("info.MinReplicas = %d, want 1 (k8s default)", info.MinReplicas)
	}
}

func TestParseHPA_MissingScaleTargetRef(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]any{
				"name":      "nginx-hpa",
				"namespace": "default",
			},
			"spec": map[string]any{
				// no scaleTargetRef
				"maxReplicas": int64(10),
			},
		},
	}

	_, _, ok := parseHPA(obj)

	if ok {
		t.Error("parseHPA() should return ok=false for missing scaleTargetRef")
	}
}

func TestParseHPA_MissingTargetName(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]any{
				"name":      "nginx-hpa",
				"namespace": "default",
			},
			"spec": map[string]any{
				"scaleTargetRef": map[string]any{
					"kind": "Deployment",
					// no name
				},
				"maxReplicas": int64(10),
			},
		},
	}

	_, _, ok := parseHPA(obj)

	if ok {
		t.Error("parseHPA() should return ok=false for missing target name")
	}
}

func TestExtractHPATargetsFromManifests(t *testing.T) {
	hpaObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]any{
				"name":      "nginx-hpa",
				"namespace": "default",
			},
			"spec": map[string]any{
				"scaleTargetRef": map[string]any{
					"kind": "Deployment",
					"name": "nginx",
				},
				"minReplicas": int64(2),
				"maxReplicas": int64(10),
			},
		},
	}

	deployObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "nginx",
				"namespace": "default",
			},
		},
	}

	resources := []api.Resource{
		{Object: hpaObj},
		{Object: deployObj},
	}

	idx := ExtractHPATargetsFromManifests(resources)

	if idx.Size() != 1 {
		t.Errorf("Size() = %d, want 1", idx.Size())
	}

	ref := api.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "nginx"}
	if !idx.IsHPAManaged(ref) {
		t.Error("nginx deployment should be HPA-managed")
	}
}

func TestExtractHPATargetsFromUnstructured(t *testing.T) {
	hpa := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]any{
				"name":      "app-hpa",
				"namespace": "prod",
			},
			"spec": map[string]any{
				"scaleTargetRef": map[string]any{
					"kind": "StatefulSet",
					"name": "app",
				},
				"minReplicas": int64(3),
				"maxReplicas": int64(20),
			},
		},
	}

	hpas := []*unstructured.Unstructured{hpa}
	idx := ExtractHPATargetsFromUnstructured(hpas)

	if idx.Size() != 1 {
		t.Errorf("Size() = %d, want 1", idx.Size())
	}

	ref := api.ResourceRef{Namespace: "prod", Kind: "StatefulSet", Name: "app"}
	info := idx.GetHPAInfo(ref)
	if info == nil {
		t.Fatal("GetHPAInfo() returned nil")
	}
	if info.MinReplicas != 3 {
		t.Errorf("MinReplicas = %d, want 3", info.MinReplicas)
	}
	if info.MaxReplicas != 20 {
		t.Errorf("MaxReplicas = %d, want 20", info.MaxReplicas)
	}
}

func TestIsScalableKind(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"Deployment", true},
		{"StatefulSet", true},
		{"ReplicaSet", true},
		{"DaemonSet", false},
		{"Service", false},
		{"ConfigMap", false},
		{"Pod", false},
		{"Job", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got := IsScalableKind(tt.kind)
			if got != tt.want {
				t.Errorf("IsScalableKind(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}
