// Package k8s provides Kubernetes client functionality.
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps Kubernetes client functionality.
type Client struct {
	dynamic   dynamic.Interface
	discovery discovery.DiscoveryInterface
	mapper    meta.RESTMapper
	config    *rest.Config
	context   string
}

// ClientOptions configures the Kubernetes client.
type ClientOptions struct {
	// KubeConfig is the path to kubeconfig file.
	// Empty uses default locations.
	KubeConfig string
	// Context is the kubectl context to use.
	// Empty uses current context.
	Context string
}

// NewClient creates a new Kubernetes client.
func NewClient(opts ClientOptions) (*Client, error) {
	config, currentContext, err := buildConfig(opts)
	if err != nil {
		return nil, fmt.Errorf("building config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating discovery client: %w", err)
	}

	// Use cached discovery for better performance
	cachedDiscovery := memory.NewMemCacheClient(discoveryClient)
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscovery)

	return &Client{
		dynamic:   dynamicClient,
		discovery: discoveryClient,
		mapper:    mapper,
		config:    config,
		context:   currentContext,
	}, nil
}

// buildConfig builds the Kubernetes client config.
func buildConfig(opts ClientOptions) (*rest.Config, string, error) {
	kubeconfig := opts.KubeConfig
	if kubeconfig == "" {
		kubeconfig = defaultKubeconfig()
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{
		ExplicitPath: kubeconfig,
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		configOverrides.CurrentContext = opts.Context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)

	// Get raw config to extract current context name
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("loading raw config: %w", err)
	}

	currentContext := rawConfig.CurrentContext
	if opts.Context != "" {
		currentContext = opts.Context
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		// Try in-cluster config as fallback
		inClusterConfig, inClusterErr := rest.InClusterConfig()
		if inClusterErr != nil {
			return nil, "", fmt.Errorf("neither kubeconfig nor in-cluster config available: %w", err)
		}
		return inClusterConfig, "in-cluster", nil
	}

	return config, currentContext, nil
}

// defaultKubeconfig returns the default kubeconfig path.
func defaultKubeconfig() string {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// Get retrieves a resource from the cluster.
func (c *Client) Get(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	gvr, err := c.gvkToGVR(gvk)
	if err != nil {
		return nil, fmt.Errorf("mapping GVK to GVR: %w", err)
	}

	var result *unstructured.Unstructured
	if namespace != "" {
		result, err = c.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		result, err = c.dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

// gvkToGVR converts GroupVersionKind to GroupVersionResource.
func (c *Client) gvkToGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}

// ServerVersion returns the Kubernetes server version.
func (c *Client) ServerVersion() (string, error) {
	version, err := c.discovery.ServerVersion()
	if err != nil {
		return "", err
	}
	return version.GitVersion, nil
}

// CurrentContext returns the current kubectl context name.
func (c *Client) CurrentContext() string {
	return c.context
}

// ListResources lists all resources of a given GVK in the specified namespaces.
// If namespaces is empty, lists resources in all namespaces.
func (c *Client) ListResources(ctx context.Context, gvk schema.GroupVersionKind, namespaces []string) ([]*unstructured.Unstructured, error) {
	gvr, err := c.gvkToGVR(gvk)
	if err != nil {
		return nil, fmt.Errorf("mapping GVK to GVR: %w", err)
	}

	var allResources []*unstructured.Unstructured

	// If no namespaces specified, list across all namespaces
	if len(namespaces) == 0 {
		result, err := c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing resources: %w", err)
		}
		for i := range result.Items {
			allResources = append(allResources, &result.Items[i])
		}
		return allResources, nil
	}

	// List resources in each namespace
	for _, ns := range namespaces {
		result, err := c.dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			// Skip this namespace if listing fails (e.g., namespace doesn't exist)
			continue
		}
		for i := range result.Items {
			allResources = append(allResources, &result.Items[i])
		}
	}

	return allResources, nil
}

// ListHPAs lists all HPAs in the specified namespaces.
// If namespaces is empty, lists HPAs in all namespaces.
func (c *Client) ListHPAs(ctx context.Context, namespaces []string) ([]*unstructured.Unstructured, error) {
	// Try autoscaling/v2 first (preferred), fall back to v1
	gvr := schema.GroupVersionResource{
		Group:    "autoscaling",
		Version:  "v2",
		Resource: "horizontalpodautoscalers",
	}

	var allHPAs []*unstructured.Unstructured

	// If no namespaces specified, list across all namespaces
	if len(namespaces) == 0 {
		result, err := c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			// Try v1 as fallback
			gvr.Version = "v1"
			result, err = c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing HPAs: %w", err)
			}
		}
		for i := range result.Items {
			allHPAs = append(allHPAs, &result.Items[i])
		}
		return allHPAs, nil
	}

	// List HPAs in each namespace
	for _, ns := range namespaces {
		result, err := c.dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			// Try v1 as fallback for this namespace
			gvr.Version = "v1"
			result, err = c.dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				// Skip this namespace if HPA listing fails
				continue
			}
		}
		for i := range result.Items {
			allHPAs = append(allHPAs, &result.Items[i])
		}
	}

	return allHPAs, nil
}
