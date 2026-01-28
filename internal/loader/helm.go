package loader

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/indrasvat/dorikin/internal/logcapture"
	"github.com/indrasvat/dorikin/pkg/api"
)

// HelmLoader loads manifests by running helm template.
type HelmLoader struct {
	helmPath    string        // path to helm binary (auto-detected if empty)
	releaseName string        // release name for helm template
	namespace   string        // namespace for helm template
	values      []string      // -f value files
	setValues   []string      // --set key=value
	timeout     time.Duration // timeout for helm commands (default: 60s)
}

// HelmOption is a functional option for configuring HelmLoader.
type HelmOption func(*HelmLoader)

// WithHelmPath sets a custom path to the helm binary.
func WithHelmPath(path string) HelmOption {
	return func(h *HelmLoader) {
		h.helmPath = path
	}
}

// WithRelease sets the release name for helm template.
func WithRelease(name string) HelmOption {
	return func(h *HelmLoader) {
		h.releaseName = name
	}
}

// WithNamespace sets the namespace for helm template.
func WithNamespace(ns string) HelmOption {
	return func(h *HelmLoader) {
		h.namespace = ns
	}
}

// WithValues sets the values files for helm template.
func WithValues(values []string) HelmOption {
	return func(h *HelmLoader) {
		h.values = values
	}
}

// WithSet sets the --set values for helm template.
func WithSet(setValues []string) HelmOption {
	return func(h *HelmLoader) {
		h.setValues = setValues
	}
}

// WithTimeout sets the timeout for helm commands.
// Default is 60 seconds.
func WithTimeout(d time.Duration) HelmOption {
	return func(h *HelmLoader) {
		h.timeout = d
	}
}

// NewHelmLoader creates a new HelmLoader.
func NewHelmLoader(opts ...HelmOption) *HelmLoader {
	h := &HelmLoader{
		releaseName: "release",        // default release name
		timeout:     60 * time.Second, // default timeout
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Load loads manifests from Helm chart directories.
func (h *HelmLoader) Load(ctx context.Context, paths []string, recursive bool) ([]api.Resource, error) {
	// Verify helm is available
	helmBin, err := h.findHelm()
	if err != nil {
		return nil, err
	}

	var allResources []api.Resource
	for _, path := range paths {
		resources, err := h.loadChart(ctx, helmBin, path)
		if err != nil {
			return nil, fmt.Errorf("loading chart %s: %w", path, err)
		}
		allResources = append(allResources, resources...)
	}

	return allResources, nil
}

// findHelm locates the helm binary.
func (h *HelmLoader) findHelm() (string, error) {
	if h.helmPath != "" {
		if _, err := os.Stat(h.helmPath); err != nil {
			return "", fmt.Errorf("helm binary not found at %s: %w", h.helmPath, err)
		}
		return h.helmPath, nil
	}

	// Try to find helm in PATH
	path, err := exec.LookPath("helm")
	if err != nil {
		return "", fmt.Errorf("helm not found in PATH. Install: https://helm.sh/docs/intro/install/")
	}
	return path, nil
}

// loadChart runs helm template and parses the output.
func (h *HelmLoader) loadChart(ctx context.Context, helmBin, chartPath string) ([]api.Resource, error) {
	// Validate Chart.yaml exists
	if err := h.validateChartDir(chartPath); err != nil {
		return nil, err
	}

	// Build helm template command
	args := []string{"template", h.releaseName, chartPath}

	// Add namespace if specified
	if h.namespace != "" {
		args = append(args, "--namespace", h.namespace)
	}

	// Add values files
	for _, f := range h.values {
		args = append(args, "--values", f)
	}

	// Add --set values
	for _, s := range h.setValues {
		args = append(args, "--set", s)
	}

	// Wrap context with timeout to prevent indefinite hangs
	timeoutCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	// Run helm template
	// #nosec G204 -- helmBin is validated to exist, args are user-provided chart options
	cmd := exec.CommandContext(timeoutCtx, helmBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		switch timeoutCtx.Err() {
		case context.DeadlineExceeded:
			return nil, fmt.Errorf("helm template timed out after %v", h.timeout)
		case context.Canceled:
			return nil, fmt.Errorf("helm template canceled")
		}
		return nil, fmt.Errorf("helm template failed: %w\n%s", err, stderr.String())
	}

	// Parse the rendered YAML
	return h.parseYAML(ctx, &stdout, chartPath)
}

// validateChartDir checks if the directory contains a Chart.yaml.
func (h *HelmLoader) validateChartDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	chartFile := filepath.Join(path, "Chart.yaml")
	if _, err := os.Stat(chartFile); err != nil {
		return fmt.Errorf("not a Helm chart: Chart.yaml not found in %s", path)
	}

	return nil
}

// parseYAML parses multi-document YAML output into resources.
func (h *HelmLoader) parseYAML(ctx context.Context, r *bytes.Buffer, sourcePath string) ([]api.Resource, error) {
	var resources []api.Resource
	var docBuffer bytes.Buffer

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			if res := h.parseDocument(docBuffer.Bytes(), sourcePath); res != nil {
				resources = append(resources, *res)
			}
			docBuffer.Reset()
			continue
		}
		docBuffer.WriteString(line)
		docBuffer.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning YAML: %w", err)
	}

	// Handle final document
	if res := h.parseDocument(docBuffer.Bytes(), sourcePath); res != nil {
		resources = append(resources, *res)
	}

	return resources, nil
}

// parseDocument parses a single YAML document.
// Returns nil for empty documents, documents without a Kind, or parse errors.
// Parse errors are logged so users know manifests are being skipped.
func (h *HelmLoader) parseDocument(data []byte, sourcePath string) *api.Resource {
	// Skip empty documents
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}

	// Parse into unstructured
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, &obj.Object); err != nil {
		logcapture.Warn("Failed to parse YAML document in %s: %v", sourcePath, err)
		return nil
	}

	// Skip if no kind (e.g., comments-only documents, Helm notes)
	if obj.GetKind() == "" {
		return nil
	}

	return &api.Resource{
		Object:     obj,
		SourceFile: sourcePath,
	}
}
