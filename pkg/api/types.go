// Package api defines the public types for dorikin.
package api

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DriftStatus represents the drift state of a resource.
type DriftStatus string

const (
	// StatusInSync indicates the resource matches the desired state.
	StatusInSync DriftStatus = "IN_SYNC"
	// StatusDrifted indicates the resource has drifted from desired state.
	StatusDrifted DriftStatus = "DRIFTED"
	// StatusMissing indicates the resource exists in manifests but not in cluster.
	StatusMissing DriftStatus = "MISSING"
	// StatusExtra indicates the resource exists in cluster but not in manifests.
	StatusExtra DriftStatus = "EXTRA"
	// StatusError indicates an error occurred while checking the resource.
	StatusError DriftStatus = "ERROR"
)

// String returns the string representation of DriftStatus.
func (s DriftStatus) String() string {
	return string(s)
}

// Emoji returns a racing-themed emoji representation of DriftStatus.
func (s DriftStatus) Emoji() string {
	switch s {
	case StatusInSync:
		return "🏁" // Checkered flag - crossed the finish line
	case StatusDrifted:
		return "🚨" // Warning light - DORIFTO detected!
	case StatusMissing:
		return "🛑" // Stop sign - resource not found
	case StatusExtra:
		return "➕" // Unexpected pit crew addition
	case StatusError:
		return "💥" // Crash - something went wrong
	default:
		return "❓"
	}
}

// ResourceRef uniquely identifies a Kubernetes resource.
type ResourceRef struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Namespace  string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name       string `json:"name" yaml:"name"`
}

// String returns a human-readable representation of ResourceRef.
func (r ResourceRef) String() string {
	if r.Namespace != "" {
		return r.Namespace + "/" + r.Kind + "/" + r.Name
	}
	return r.Kind + "/" + r.Name
}

// ShortString returns a compact representation.
func (r ResourceRef) ShortString() string {
	return r.Kind + "/" + r.Name
}

// FieldDiff represents a difference in a single field.
type FieldDiff struct {
	// Path is the JSONPath to the field (e.g., ".spec.replicas").
	Path string `json:"path" yaml:"path"`
	// Expected is the value from the manifest.
	Expected any `json:"expected,omitempty" yaml:"expected,omitempty"`
	// Actual is the value from the cluster.
	Actual any `json:"actual,omitempty" yaml:"actual,omitempty"`
	// Type indicates the type of difference.
	Type DiffType `json:"type" yaml:"type"`
}

// DiffType indicates the type of field difference.
type DiffType string

const (
	// DiffTypeModified indicates the field value was changed.
	DiffTypeModified DiffType = "modified"
	// DiffTypeAdded indicates the field was added in actual.
	DiffTypeAdded DiffType = "added"
	// DiffTypeRemoved indicates the field was removed from actual.
	DiffTypeRemoved DiffType = "removed"
)

// RenderMode indicates how a diff should be rendered in the TUI.
type RenderMode int

const (
	// RenderModeSideBySide renders simple values in side-by-side columns.
	RenderModeSideBySide RenderMode = iota
	// RenderModeUnifiedDiff renders multi-line text changes as unified diff.
	RenderModeUnifiedDiff
	// RenderModeStructuralAdd renders objects/arrays added in cluster.
	RenderModeStructuralAdd
	// RenderModeStructuralDel renders objects/arrays removed from cluster.
	RenderModeStructuralDel
	// RenderModeReplacement renders >75% different content with toggle view.
	RenderModeReplacement
)

// DriftReport contains the drift analysis for a single resource.
type DriftReport struct {
	// Resource identifies the resource.
	Resource ResourceRef `json:"resource" yaml:"resource"`
	// Status is the drift status.
	Status DriftStatus `json:"status" yaml:"status"`
	// Diffs contains the field-level differences.
	Diffs []FieldDiff `json:"diffs,omitempty" yaml:"diffs,omitempty"`
	// CheckedAt is when the check was performed.
	CheckedAt time.Time `json:"checkedAt" yaml:"checkedAt"`
	// Error contains any error message.
	Error string `json:"error,omitempty" yaml:"error,omitempty"`
	// SourceFile is the manifest file this resource came from.
	SourceFile string `json:"sourceFile,omitempty" yaml:"sourceFile,omitempty"`

	// ManifestObject is the full object from the manifest.
	// Only populated for DRIFTED/MISSING resources to enable Manifest tab viewing.
	// Excluded from JSON/YAML serialization to avoid bloat.
	ManifestObject map[string]any `json:"-" yaml:"-"`
	// ClusterObject is the full object from the cluster.
	// Only populated for DRIFTED/EXTRA resources to enable Cluster tab viewing.
	// Excluded from JSON/YAML serialization to avoid bloat.
	ClusterObject map[string]any `json:"-" yaml:"-"`
}

// HasDrift returns true if the resource has drifted.
func (r DriftReport) HasDrift() bool {
	return r.Status == StatusDrifted || r.Status == StatusMissing || r.Status == StatusExtra
}

// ScanResult contains the results of a complete scan.
type ScanResult struct {
	// Reports contains individual resource reports.
	Reports []DriftReport `json:"reports" yaml:"reports"`
	// Summary contains aggregated statistics.
	Summary ScanSummary `json:"summary" yaml:"summary"`
	// StartedAt is when the scan started.
	StartedAt time.Time `json:"startedAt" yaml:"startedAt"`
	// CompletedAt is when the scan completed.
	CompletedAt time.Time `json:"completedAt" yaml:"completedAt"`
	// Duration is the scan duration.
	Duration time.Duration `json:"duration" yaml:"duration"`
}

// ScanSummary contains aggregated scan statistics.
type ScanSummary struct {
	TotalResources int `json:"totalResources" yaml:"totalResources"`
	InSync         int `json:"inSync" yaml:"inSync"`
	Drifted        int `json:"drifted" yaml:"drifted"`
	Missing        int `json:"missing" yaml:"missing"`
	Extra          int `json:"extra" yaml:"extra"`
	Errors         int `json:"errors" yaml:"errors"`
}

// HasIssues returns true if any resources have drift or errors.
func (s ScanSummary) HasIssues() bool {
	return s.Drifted > 0 || s.Missing > 0 || s.Extra > 0 || s.Errors > 0
}

// Resource represents a Kubernetes resource with its source.
type Resource struct {
	// Object is the unstructured Kubernetes object.
	Object *unstructured.Unstructured
	// SourceFile is the file this resource was loaded from.
	SourceFile string
}

// Ref returns the ResourceRef for this resource.
func (r Resource) Ref() ResourceRef {
	return ResourceRef{
		APIVersion: r.Object.GetAPIVersion(),
		Kind:       r.Object.GetKind(),
		Namespace:  r.Object.GetNamespace(),
		Name:       r.Object.GetName(),
	}
}

// ScanOptions configures a scan operation.
type ScanOptions struct {
	// ManifestPaths are paths to manifest files or directories.
	ManifestPaths []string
	// Namespace filters resources by namespace (empty = all).
	//
	// Deprecated: Use Namespaces for multi-namespace support.
	// For backward compatibility, if Namespaces is empty and Namespace is set,
	// Namespace is used as a single-element Namespaces slice.
	Namespace string
	// Namespaces filters resources by namespace(s) (empty = all).
	Namespaces []string
	// Kinds filters to include only these resource kinds.
	Kinds []string
	// ExcludeKinds filters to exclude these resource kinds.
	ExcludeKinds []string
	// KubeContext is the kubectl context to use.
	KubeContext string
	// KubeConfig is the path to kubeconfig file.
	KubeConfig string
	// IgnorePaths are JSONPaths to ignore during comparison.
	IgnorePaths []string
	// Recursive enables recursive directory scanning.
	Recursive bool
	// IncludeExtra includes resources in cluster but not in manifests.
	IncludeExtra bool
	// HPAAware controls HPA-aware replica comparison (default: manifests).
	HPAAware HPAAwareMode
}

// DefaultIgnorePaths returns the default paths to ignore during comparison.
func DefaultIgnorePaths() []string {
	return []string{
		".metadata.resourceVersion",
		".metadata.uid",
		".metadata.generation",
		".metadata.creationTimestamp",
		".metadata.managedFields",
		".metadata.annotations.kubectl.kubernetes.io/last-applied-configuration",
		".metadata.annotations.deployment.kubernetes.io/revision",
		".status",
	}
}

// OutputFormat specifies the output format.
type OutputFormat string

const (
	OutputFormatTable OutputFormat = "table"
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatYAML  OutputFormat = "yaml"
	OutputFormatQuiet OutputFormat = "quiet"
)

// HPAAwareMode controls how HPA awareness works during drift detection.
type HPAAwareMode string

const (
	// HPAAwareModeManifests extracts HPAs from manifest files only (default).
	HPAAwareModeManifests HPAAwareMode = "manifests"
	// HPAAwareModeCluster also queries the cluster for HPAs.
	HPAAwareModeCluster HPAAwareMode = "cluster"
	// HPAAwareModeDisabled disables HPA awareness entirely.
	HPAAwareModeDisabled HPAAwareMode = "disabled"
)
