package drift

import (
	"context"
	"sync"
	"time"

	"github.com/indrasvat/dorikin/internal/k8s"
	"github.com/indrasvat/dorikin/internal/loader"
	"github.com/indrasvat/dorikin/pkg/api"
)

// Detector performs drift detection between manifests and cluster state.
type Detector struct {
	client     *k8s.Client
	loader     loader.Loader
	comparator *Comparator
}

// NewDetector creates a new Detector.
func NewDetector(client *k8s.Client, ignorePaths []string) *Detector {
	return &Detector{
		client:     client,
		loader:     loader.NewFileLoader(),
		comparator: NewComparator(ignorePaths),
	}
}

// Scan performs a complete drift scan.
func (d *Detector) Scan(ctx context.Context, opts api.ScanOptions) (*api.ScanResult, error) {
	startTime := time.Now()

	// Load manifests
	resources, err := d.loader.Load(ctx, opts.ManifestPaths, opts.Recursive)
	if err != nil {
		return nil, err
	}

	// Filter by namespace if specified
	if opts.Namespace != "" {
		resources = filterByNamespace(resources, opts.Namespace)
	}

	// Detect drift for all resources
	reports := d.detectDrift(ctx, resources)

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
func (d *Detector) detectDrift(ctx context.Context, resources []api.Resource) []api.DriftReport {
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
	var mu sync.Mutex

	for i, res := range resources {
		idx := i
		resource := res
		fetchResult := fetchResults[i]

		// Go 1.25 Feature: WaitGroup.Go()
		wg.Go(func() {
			report := d.compareResource(resource, fetchResult)

			mu.Lock()
			reports[idx] = report
			mu.Unlock()
		})
	}

	wg.Wait()
	return reports
}

// compareResource compares a single resource.
func (d *Detector) compareResource(resource api.Resource, fetchResult k8s.FetchResult) api.DriftReport {
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
		return report
	}

	// Compare the resources
	expected := resource.Object.Object
	actual := fetchResult.Object.Object

	diffs := d.comparator.Compare(expected, actual)

	if len(diffs) == 0 {
		report.Status = api.StatusInSync
	} else {
		report.Status = api.StatusDrifted
		report.Diffs = diffs
	}

	return report
}

// filterByNamespace filters resources by namespace.
func filterByNamespace(resources []api.Resource, namespace string) []api.Resource {
	filtered := make([]api.Resource, 0, len(resources))
	for _, res := range resources {
		if res.Object.GetNamespace() == namespace || res.Object.GetNamespace() == "" {
			filtered = append(filtered, res)
		}
	}
	return filtered
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
