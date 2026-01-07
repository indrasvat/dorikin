package k8s

import (
	"context"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/indrasvat/dorikin/pkg/api"
)

// FetchResult contains the result of fetching a resource.
type FetchResult struct {
	Ref    api.ResourceRef
	Object *unstructured.Unstructured
	Found  bool
	Error  error
}

// FetchResources fetches multiple resources concurrently.
// This uses Go 1.25's new sync.WaitGroup.Go() method!
func (c *Client) FetchResources(ctx context.Context, refs []api.ResourceRef) []FetchResult {
	results := make([]FetchResult, len(refs))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, ref := range refs {
		idx := i
		r := ref

		// Go 1.25 Feature: WaitGroup.Go()
		// No more wg.Add(1) + defer wg.Done() boilerplate!
		wg.Go(func() {
			result := c.fetchSingleResource(ctx, r)

			mu.Lock()
			results[idx] = result
			mu.Unlock()
		})
	}

	wg.Wait()
	return results
}

// fetchSingleResource fetches a single resource from the cluster.
func (c *Client) fetchSingleResource(ctx context.Context, ref api.ResourceRef) FetchResult {
	gvk := schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)

	obj, err := c.Get(ctx, gvk, ref.Namespace, ref.Name)
	if err != nil {
		// Check if it's a "not found" error
		if apierrors.IsNotFound(err) {
			return FetchResult{
				Ref:   ref,
				Found: false,
			}
		}
		return FetchResult{
			Ref:   ref,
			Error: err,
		}
	}

	return FetchResult{
		Ref:    ref,
		Object: obj,
		Found:  true,
	}
}
