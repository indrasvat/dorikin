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

// KustomizeLoader loads manifests by running kustomize build.
type KustomizeLoader struct {
	kustomizePath string        // path to kustomize binary (auto-detected if empty)
	timeout       time.Duration // timeout for kustomize commands (default: 60s)
}

// KustomizeOption is a functional option for configuring KustomizeLoader.
type KustomizeOption func(*KustomizeLoader)

// WithKustomizePath sets a custom path to the kustomize binary.
func WithKustomizePath(path string) KustomizeOption {
	return func(k *KustomizeLoader) {
		k.kustomizePath = path
	}
}

// WithKustomizeTimeout sets the timeout for kustomize commands.
// Default is 60 seconds.
func WithKustomizeTimeout(d time.Duration) KustomizeOption {
	return func(k *KustomizeLoader) {
		k.timeout = d
	}
}

// NewKustomizeLoader creates a new KustomizeLoader.
func NewKustomizeLoader(opts ...KustomizeOption) *KustomizeLoader {
	k := &KustomizeLoader{
		timeout: 60 * time.Second, // default timeout
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

// Load loads manifests from Kustomize directories.
func (k *KustomizeLoader) Load(ctx context.Context, paths []string, recursive bool) ([]api.Resource, error) {
	// Verify kustomize is available
	kustomizeBin, err := k.findKustomize()
	if err != nil {
		return nil, err
	}

	var allResources []api.Resource
	for _, path := range paths {
		resources, err := k.loadKustomization(ctx, kustomizeBin, path)
		if err != nil {
			return nil, fmt.Errorf("loading kustomization %s: %w", path, err)
		}
		allResources = append(allResources, resources...)
	}

	return allResources, nil
}

// findKustomize locates the kustomize binary.
func (k *KustomizeLoader) findKustomize() (string, error) {
	if k.kustomizePath != "" {
		if _, err := os.Stat(k.kustomizePath); err != nil {
			return "", fmt.Errorf("kustomize binary not found at %s: %w", k.kustomizePath, err)
		}
		return k.kustomizePath, nil
	}

	// Try to find kustomize in PATH
	path, err := exec.LookPath("kustomize")
	if err != nil {
		return "", fmt.Errorf("kustomize not found in PATH. Install: https://kubectl.docs.kubernetes.io/installation/kustomize/")
	}
	return path, nil
}

// loadKustomization runs kustomize build and parses the output.
func (k *KustomizeLoader) loadKustomization(ctx context.Context, kustomizeBin, path string) ([]api.Resource, error) {
	// Validate kustomization.yaml exists
	if err := k.validateKustomizationDir(path); err != nil {
		return nil, err
	}

	// Wrap context with timeout to prevent indefinite hangs
	timeoutCtx, cancel := context.WithTimeout(ctx, k.timeout)
	defer cancel()

	// Run kustomize build
	cmd := exec.CommandContext(timeoutCtx, kustomizeBin, "build", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		switch timeoutCtx.Err() {
		case context.DeadlineExceeded:
			return nil, fmt.Errorf("kustomize build timed out after %v", k.timeout)
		case context.Canceled:
			return nil, fmt.Errorf("kustomize build canceled")
		}
		return nil, fmt.Errorf("kustomize build failed: %w\n%s", err, stderr.String())
	}

	// Parse the rendered YAML
	return k.parseYAML(ctx, &stdout, path)
}

// validateKustomizationDir checks if the directory contains a kustomization file.
func (k *KustomizeLoader) validateKustomizationDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	// Check for kustomization.yaml or kustomization.yml
	kustomizationFiles := []string{
		filepath.Join(path, "kustomization.yaml"),
		filepath.Join(path, "kustomization.yml"),
		filepath.Join(path, "Kustomization"),
	}

	for _, f := range kustomizationFiles {
		if _, err := os.Stat(f); err == nil {
			return nil
		}
	}

	return fmt.Errorf("not a kustomize directory: kustomization.yaml not found in %s", path)
}

// parseYAML parses multi-document YAML output into resources.
func (k *KustomizeLoader) parseYAML(ctx context.Context, r *bytes.Buffer, sourcePath string) ([]api.Resource, error) {
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
			if res := k.parseDocument(docBuffer.Bytes(), sourcePath); res != nil {
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
	if res := k.parseDocument(docBuffer.Bytes(), sourcePath); res != nil {
		resources = append(resources, *res)
	}

	return resources, nil
}

// parseDocument parses a single YAML document.
// Returns nil for empty documents, documents without a Kind, or parse errors.
// Parse errors are logged so users know manifests are being skipped.
func (k *KustomizeLoader) parseDocument(data []byte, sourcePath string) *api.Resource {
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

	// Skip if no kind (e.g., comments-only documents)
	if obj.GetKind() == "" {
		return nil
	}

	return &api.Resource{
		Object:     obj,
		SourceFile: sourcePath,
	}
}
