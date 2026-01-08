// Package drift provides drift detection functionality.
package drift

import (
	"math"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/indrasvat/dorikin/pkg/api"
)

// clampToInt32 safely converts int64 to int32 with bounds checking.
// Values outside int32 range are clamped to min/max.
func clampToInt32(v int64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

// HPATargetKey uniquely identifies a scalable resource.
type HPATargetKey struct {
	Namespace string
	Kind      string // Deployment, StatefulSet, ReplicaSet
	Name      string
}

// HPATargetInfo contains HPA configuration for a target.
type HPATargetInfo struct {
	HPAName     string
	MinReplicas int32
	MaxReplicas int32
}

// HPATargetIndex maps scalable resources to their HPA configurations.
// Thread-safe for read operations (built once, read many).
type HPATargetIndex struct {
	targets map[HPATargetKey]HPATargetInfo
}

// NewHPATargetIndex creates an empty index.
func NewHPATargetIndex() *HPATargetIndex {
	return &HPATargetIndex{
		targets: make(map[HPATargetKey]HPATargetInfo),
	}
}

// Add registers an HPA target.
func (idx *HPATargetIndex) Add(key HPATargetKey, info HPATargetInfo) {
	idx.targets[key] = info
}

// Merge combines another index into this one.
// If the same target exists in both, the other index's value takes precedence.
func (idx *HPATargetIndex) Merge(other *HPATargetIndex) {
	if other == nil {
		return
	}
	for key, info := range other.targets {
		idx.targets[key] = info
	}
}

// IsHPAManaged returns true if the resource is managed by an HPA.
func (idx *HPATargetIndex) IsHPAManaged(ref api.ResourceRef) bool {
	if idx == nil {
		return false
	}
	key := HPATargetKey{
		Namespace: ref.Namespace,
		Kind:      ref.Kind,
		Name:      ref.Name,
	}
	_, ok := idx.targets[key]
	return ok
}

// GetHPAInfo returns HPA info if the resource is managed, nil otherwise.
func (idx *HPATargetIndex) GetHPAInfo(ref api.ResourceRef) *HPATargetInfo {
	if idx == nil {
		return nil
	}
	key := HPATargetKey{
		Namespace: ref.Namespace,
		Kind:      ref.Kind,
		Name:      ref.Name,
	}
	if info, ok := idx.targets[key]; ok {
		return &info
	}
	return nil
}

// Size returns the number of HPA targets in the index.
func (idx *HPATargetIndex) Size() int {
	if idx == nil {
		return 0
	}
	return len(idx.targets)
}

// ExtractHPATargetsFromManifests scans loaded resources for HPAs
// and builds an index of their targets.
func ExtractHPATargetsFromManifests(resources []api.Resource) *HPATargetIndex {
	idx := NewHPATargetIndex()
	for _, res := range resources {
		if !isHPA(res.Object) {
			continue
		}
		key, info, ok := parseHPA(res.Object)
		if ok {
			idx.Add(key, info)
		}
	}
	return idx
}

// ExtractHPATargetsFromUnstructured builds an index from unstructured HPA objects.
// Used for cluster-queried HPAs.
func ExtractHPATargetsFromUnstructured(hpas []*unstructured.Unstructured) *HPATargetIndex {
	idx := NewHPATargetIndex()
	for _, hpa := range hpas {
		key, info, ok := parseHPA(hpa)
		if ok {
			idx.Add(key, info)
		}
	}
	return idx
}

// isHPA checks if an unstructured object is an HPA.
func isHPA(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}
	kind := obj.GetKind()
	apiVersion := obj.GetAPIVersion()
	return kind == "HorizontalPodAutoscaler" && strings.HasPrefix(apiVersion, "autoscaling/")
}

// parseHPA extracts target key and info from an HPA object.
// Handles both autoscaling/v1 and autoscaling/v2.
func parseHPA(obj *unstructured.Unstructured) (HPATargetKey, HPATargetInfo, bool) {
	namespace := obj.GetNamespace()

	// Extract scaleTargetRef
	scaleTargetRef, found, err := unstructured.NestedMap(obj.Object, "spec", "scaleTargetRef")
	if !found || err != nil {
		return HPATargetKey{}, HPATargetInfo{}, false
	}

	targetKind, _, _ := unstructured.NestedString(scaleTargetRef, "kind")
	targetName, _, _ := unstructured.NestedString(scaleTargetRef, "name")

	if targetKind == "" || targetName == "" {
		return HPATargetKey{}, HPATargetInfo{}, false
	}

	key := HPATargetKey{
		Namespace: namespace,
		Kind:      targetKind,
		Name:      targetName,
	}

	// Extract min/max replicas
	minReplicas, found, _ := unstructured.NestedInt64(obj.Object, "spec", "minReplicas")
	if !found {
		minReplicas = 1 // Kubernetes default
	}
	maxReplicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "maxReplicas")

	info := HPATargetInfo{
		HPAName:     obj.GetName(),
		MinReplicas: clampToInt32(minReplicas),
		MaxReplicas: clampToInt32(maxReplicas),
	}

	return key, info, true
}

// ScalableKinds are the Kubernetes kinds that HPAs can target.
var ScalableKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"ReplicaSet":  true,
}

// IsScalableKind returns true if the kind can be scaled by an HPA.
func IsScalableKind(kind string) bool {
	return ScalableKinds[kind]
}
