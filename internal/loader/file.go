package loader

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/indrasvat/dorikin/pkg/api"
)

// FileLoader loads manifests from files and directories.
type FileLoader struct{}

// NewFileLoader creates a new FileLoader.
func NewFileLoader() *FileLoader {
	return &FileLoader{}
}

// Load loads manifests from the given paths.
func (l *FileLoader) Load(ctx context.Context, paths []string, recursive bool) ([]api.Resource, error) {
	var allResources []api.Resource
	var mu sync.Mutex
	var wg sync.WaitGroup
	var loadErr error

	for _, path := range paths {
		p := path

		// Go 1.25 Feature: WaitGroup.Go()
		wg.Go(func() {
			resources, err := l.loadPath(ctx, p, recursive)

			mu.Lock()
			defer mu.Unlock()

			if err != nil && loadErr == nil {
				loadErr = err
				return
			}
			allResources = append(allResources, resources...)
		})
	}

	wg.Wait()

	if loadErr != nil {
		return nil, loadErr
	}

	return allResources, nil
}

// loadPath loads manifests from a single path.
func (l *FileLoader) loadPath(ctx context.Context, path string, recursive bool) ([]api.Resource, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	if info.IsDir() {
		return l.loadDirectory(ctx, path, recursive)
	}
	return l.loadFile(ctx, path)
}

// loadDirectory loads all YAML files from a directory.
func (l *FileLoader) loadDirectory(ctx context.Context, dir string, recursive bool) ([]api.Resource, error) {
	var resources []api.Resource

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories (but continue walking)
		if info.IsDir() {
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process YAML files
		if !isYAMLFile(path) {
			return nil
		}

		fileResources, err := l.loadFile(ctx, path)
		if err != nil {
			return fmt.Errorf("loading %s: %w", path, err)
		}
		resources = append(resources, fileResources...)
		return nil
	}

	if err := filepath.Walk(dir, walkFn); err != nil {
		return nil, err
	}

	return resources, nil
}

// loadFile loads resources from a single YAML file.
func (l *FileLoader) loadFile(ctx context.Context, path string) ([]api.Resource, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	return l.parseYAML(ctx, file, path)
}

// parseYAML parses a YAML stream into resources.
func (l *FileLoader) parseYAML(ctx context.Context, r io.Reader, sourcePath string) ([]api.Resource, error) {
	var resources []api.Resource
	var docBuffer bytes.Buffer

	scanner := bufio.NewScanner(bufio.NewReader(r))
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			resources = l.appendDocument(&docBuffer, sourcePath, resources)
			continue
		}
		docBuffer.WriteString(line)
		docBuffer.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning YAML: %w", err)
	}

	return l.appendDocument(&docBuffer, sourcePath, resources), nil
}

// appendDocument parses buffer content and appends to resources if valid.
func (l *FileLoader) appendDocument(buf *bytes.Buffer, sourcePath string, resources []api.Resource) []api.Resource {
	if buf.Len() == 0 {
		return resources
	}
	res, err := l.parseDocument(buf.Bytes(), sourcePath)
	buf.Reset()
	if err == nil && res != nil {
		return append(resources, *res)
	}
	return resources
}

// parseDocument parses a single YAML document.
func (l *FileLoader) parseDocument(data []byte, sourcePath string) (*api.Resource, error) {
	// Skip empty documents
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	// Parse into unstructured
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, &obj.Object); err != nil {
		return nil, fmt.Errorf("unmarshaling YAML: %w", err)
	}

	// Skip if no kind (e.g., comments-only documents)
	if obj.GetKind() == "" {
		return nil, nil
	}

	return &api.Resource{
		Object:     obj,
		SourceFile: sourcePath,
	}, nil
}

// isYAMLFile checks if a file is a YAML file.
func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}
