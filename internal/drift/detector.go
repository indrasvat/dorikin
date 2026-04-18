package drift

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/indrasvat/dorikin/internal/config"
	"github.com/indrasvat/dorikin/internal/k8s"
	"github.com/indrasvat/dorikin/internal/loader"
	"github.com/indrasvat/dorikin/internal/logcapture"
	"github.com/indrasvat/dorikin/pkg/api"
)

// Detector performs drift detection between manifests and cluster state.
type Detector struct {
	client     *k8s.Client
	loader     loader.Loader
	comparator *Comparator
	config     *config.Config
}

// DetectorOption is a functional option for configuring a Detector.
type DetectorOption func(*Detector)

// WithLoader sets a custom loader for the Detector.
// By default, the FileLoader is used.
func WithLoader(l loader.Loader) DetectorOption {
	return func(d *Detector) {
		d.loader = l
	}
}

// WithConfig sets the configuration for the Detector.
// The config enables resource-type-specific ignore paths.
func WithConfig(cfg *config.Config) DetectorOption {
	return func(d *Detector) {
		d.config = cfg
	}
}

// NewDetector creates a new Detector with the given ignore paths and options.
func NewDetector(client *k8s.Client, ignorePaths []string, opts ...DetectorOption) *Detector {
	d := &Detector{
		client:     client,
		loader:     loader.NewFileLoader(),
		comparator: NewComparator(ignorePaths),
		config:     nil,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// NewDetectorWithConfig creates a new Detector with configuration support.
//
// Deprecated: Use NewDetector with WithConfig option instead.
func NewDetectorWithConfig(client *k8s.Client, cfg *config.Config, ignorePaths []string) *Detector {
	return NewDetector(client, ignorePaths, WithConfig(cfg))
}

// Scan performs a complete drift scan.
func (d *Detector) Scan(ctx context.Context, opts api.ScanOptions) (*api.ScanResult, error) {
	startTime := time.Now()

	// Load manifests
	resources, err := d.loader.Load(ctx, opts.ManifestPaths, opts.Recursive)
	if err != nil {
		return nil, err
	}

	// Filter by namespace(s) if specified
	// Support both old Namespace field (single) and new Namespaces field (multiple)
	namespaces := opts.Namespaces
	if len(namespaces) == 0 && opts.Namespace != "" { //nolint:staticcheck // backward compat
		namespaces = []string{opts.Namespace} //nolint:staticcheck // backward compat
	}
	if len(namespaces) > 0 {
		resources = filterByNamespaces(resources, namespaces)
	}

	// Filter by kind if specified
	if len(opts.Kinds) > 0 || len(opts.ExcludeKinds) > 0 {
		resources = filterByKind(resources, opts.Kinds, opts.ExcludeKinds)
	}

	// Build HPA target index for replica-aware comparison
	var hpaIndex *HPATargetIndex
	if opts.HPAAware != api.HPAAwareModeDisabled {
		// Extract HPAs from manifests (always done unless disabled)
		hpaIndex = ExtractHPATargetsFromManifests(resources)

		// Optionally also query cluster for HPAs
		if opts.HPAAware == api.HPAAwareModeCluster {
			namespaces := extractNamespaces(resources)
			clusterHPAs, err := d.client.ListHPAs(ctx, namespaces)
			if err == nil {
				clusterIndex := ExtractHPATargetsFromUnstructured(clusterHPAs)
				hpaIndex.Merge(clusterIndex)
			} else {
				// Log warning so users know cluster HPA discovery failed
				logcapture.Warn("HPA cluster discovery failed, using manifest-only mode: %v", err)
			}
		}
	}

	// Detect drift for all resources
	reports := d.detectDrift(ctx, resources, hpaIndex)

	// Detect extra resources if requested
	if opts.IncludeExtra {
		extraReports := d.detectExtraResources(ctx, resources, namespaces)
		reports = append(reports, extraReports...)
	}

	// Calculate summary
	summary := calculateSummary(reports)

	endTime := time.Now()
	return &api.ScanResult{
		Reports:     reports,
		Summary:     summary,
		StartedAt:   startTime,
		CompletedAt: endTime,
		Duration:    endTime.Sub(startTime),
	}, nil
}

// detectDrift detects drift for a list of resources.
func (d *Detector) detectDrift(ctx context.Context, resources []api.Resource, hpaIndex *HPATargetIndex) []api.DriftReport {
	// Extract refs for batch fetching
	refs := make([]api.ResourceRef, len(resources))
	for i, res := range resources {
		refs[i] = res.Ref()
	}

	// Fetch all resources from cluster concurrently
	// Uses Go 1.25's WaitGroup.Go() under the hood!
	fetchResults := d.client.FetchResources(ctx, refs)

	// Compare each resource
	reports := make([]api.DriftReport, len(resources))
	var wg sync.WaitGroup

	for i, res := range resources {
		idx := i
		resource := res
		fetchResult := fetchResults[i]

		// Go 1.25 Feature: WaitGroup.Go()
		wg.Go(func() {
			report := d.compareResource(resource, fetchResult, hpaIndex)

			reports[idx] = report
		})
	}

	wg.Wait()
	return reports
}

// compareResource compares a single resource.
func (d *Detector) compareResource(resource api.Resource, fetchResult k8s.FetchResult, hpaIndex *HPATargetIndex) api.DriftReport {
	report := api.DriftReport{
		Resource:   resource.Ref(),
		CheckedAt:  time.Now(),
		SourceFile: resource.SourceFile,
	}

	// Handle fetch errors
	if fetchResult.Error != nil {
		report.Status = api.StatusError
		report.Error = fetchResult.Error.Error()
		return report
	}

	// Handle missing resources
	if !fetchResult.Found {
		report.Status = api.StatusMissing
		// Store manifest object for Manifest tab viewing
		report.ManifestObject = deepCopyMap(resource.Object.Object)
		return report
	}

	// Compare the resources
	expected := resource.Object.Object
	actual := fetchResult.Object.Object

	// Build dynamic ignore paths
	var dynamicIgnore []string
	ref := resource.Ref()

	// Add HPA-managed scalable resources' replica field
	if IsScalableKind(ref.Kind) && hpaIndex != nil && hpaIndex.IsHPAManaged(ref) {
		dynamicIgnore = append(dynamicIgnore, ".spec.replicas")
	}

	// Add resource-type-specific ignore paths from config
	if d.config != nil && d.config.Ignore.Resources != nil {
		if kindPaths, ok := d.config.Ignore.Resources[ref.Kind]; ok {
			for _, p := range kindPaths {
				dynamicIgnore = append(dynamicIgnore, "."+p)
			}
		}
	}

	diffs := d.comparator.CompareWithDynamicIgnore(expected, actual, dynamicIgnore)

	if len(diffs) == 0 {
		report.Status = api.StatusInSync
	} else {
		report.Status = api.StatusDrifted
		report.Diffs = diffs
		// Store full objects for Manifest/Cluster tab viewing
		report.ManifestObject = deepCopyMap(expected)
		report.ClusterObject = deepCopyMap(actual)
	}

	return report
}

// filterByNamespace filters resources by a single namespace.
//
// Deprecated: Use filterByNamespaces for multi-namespace support.
func filterByNamespace(resources []api.Resource, namespace string) []api.Resource {
	return filterByNamespaces(resources, []string{namespace})
}

// filterByNamespaces filters resources by namespace(s).
// Cluster-scoped resources (empty namespace) are always included.
func filterByNamespaces(resources []api.Resource, namespaces []string) []api.Resource {
	if len(namespaces) == 0 {
		return resources
	}

	nsSet := toSet(namespaces)
	filtered := make([]api.Resource, 0, len(resources))
	for _, res := range resources {
		ns := res.Object.GetNamespace()
		// Include cluster-scoped resources (empty namespace) or resources in matching namespaces
		if ns == "" || nsSet[ns] {
			filtered = append(filtered, res)
		}
	}
	return filtered
}

// filterByKind filters resources by kind.
// If include is non-empty, only resources with kinds in include are returned.
// If exclude is non-empty, resources with kinds in exclude are filtered out.
func filterByKind(resources []api.Resource, include, exclude []string) []api.Resource {
	includeSet := toSet(include)
	excludeSet := toSet(exclude)

	filtered := make([]api.Resource, 0, len(resources))
	for _, res := range resources {
		kind := res.Object.GetKind()

		// If include filter is specified, kind must be in the include set
		if len(includeSet) > 0 && !includeSet[kind] {
			continue
		}

		// If kind is in exclude set, skip it
		if excludeSet[kind] {
			continue
		}

		filtered = append(filtered, res)
	}
	return filtered
}

// toSet converts a string slice to a set (map[string]bool).
func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// extractNamespaces returns unique namespaces from resources.
func extractNamespaces(resources []api.Resource) []string {
	seen := make(map[string]bool)
	var namespaces []string
	for _, res := range resources {
		ns := res.Object.GetNamespace()
		if ns != "" && !seen[ns] {
			seen[ns] = true
			namespaces = append(namespaces, ns)
		}
	}
	return namespaces
}

// calculateSummary calculates scan summary from reports.
func calculateSummary(reports []api.DriftReport) api.ScanSummary {
	summary := api.ScanSummary{
		TotalResources: len(reports),
	}

	for i := range reports {
		switch reports[i].Status {
		case api.StatusInSync:
			summary.InSync++
		case api.StatusDrifted:
			summary.Drifted++
		case api.StatusMissing:
			summary.Missing++
		case api.StatusExtra:
			summary.Extra++
		case api.StatusError:
			summary.Errors++
		}
	}

	return summary
}

// deepCopyMap creates a deep copy of a map[string]any.
// This is used to store full objects without retaining references.
func deepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = deepCopyValue(v)
	}
	return dst
}

// deepCopyValue recursively copies a value.
func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyMap(val)
	case []any:
		dst := make([]any, len(val))
		for i, item := range val {
			dst[i] = deepCopyValue(item)
		}
		return dst
	default:
		// Primitive types (string, int, float, bool, nil) are safe to copy directly
		return v
	}
}

// CurrentContext returns the current kubectl context name.
func (d *Detector) CurrentContext() string {
	return d.client.CurrentContext()
}

// detectExtraResources finds resources in the cluster that are not in manifests.
// It queries the cluster concurrently for each unique GVK found in manifests
// and reports any resources that don't have a corresponding manifest entry.
// Errors listing specific resource types are logged but don't fail the scan.
func (d *Detector) detectExtraResources(ctx context.Context, manifests []api.Resource, namespaces []string) []api.DriftReport {
	// Build a set of manifest resource keys
	manifestKeys := make(map[string]bool)
	for _, r := range manifests {
		manifestKeys[resourceKey(r.Ref())] = true
	}

	// Extract unique GVKs from manifests
	gvks := extractUniqueGVKs(manifests)

	var reports []api.DriftReport
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, gvk := range gvks {
		wg.Go(func() {
			clusterResources, err := d.client.ListResources(ctx, gvk, namespaces)
			if err != nil {
				// Log warning so users know EXTRA detection may be incomplete
				logcapture.Warn("Failed to list %s resources for EXTRA detection: %v", gvk.Kind, err)
				return
			}

			for _, cr := range clusterResources {
				ref := api.ResourceRef{
					APIVersion: cr.GetAPIVersion(),
					Kind:       cr.GetKind(),
					Namespace:  cr.GetNamespace(),
					Name:       cr.GetName(),
				}

				// If this resource is not in manifests, it's EXTRA
				if !manifestKeys[resourceKey(ref)] {
					report := api.DriftReport{
						Resource:      ref,
						Status:        api.StatusExtra,
						ClusterObject: deepCopyMap(cr.Object),
						CheckedAt:     time.Now(),
					}

					mu.Lock()
					reports = append(reports, report)
					mu.Unlock()
				}
			}
		})
	}

	wg.Wait()
	return reports
}

// resourceKey generates a unique key for a resource reference.
func resourceKey(ref api.ResourceRef) string {
	return fmt.Sprintf("%s/%s/%s/%s", ref.APIVersion, ref.Kind, ref.Namespace, ref.Name)
}

// extractUniqueGVKs returns the unique GroupVersionKinds from a list of resources.
func extractUniqueGVKs(resources []api.Resource) []schema.GroupVersionKind {
	seen := make(map[schema.GroupVersionKind]bool)
	var gvks []schema.GroupVersionKind

	for _, r := range resources {
		gvk := r.Object.GroupVersionKind()
		if !seen[gvk] {
			seen[gvk] = true
			gvks = append(gvks, gvk)
		}
	}

	return gvks
}
