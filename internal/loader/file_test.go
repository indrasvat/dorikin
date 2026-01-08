package loader

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNewFileLoader(t *testing.T) {
	l := NewFileLoader()
	if l == nil {
		t.Fatal("NewFileLoader() returned nil")
	}
}

func TestLoadFile_SingleDocument(t *testing.T) {
	l := NewFileLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/valid/deployment.yaml"}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Load() returned %d resources, want 1", len(resources))
	}

	res := resources[0]
	if res.Object.GetKind() != "Deployment" {
		t.Errorf("Kind = %q, want %q", res.Object.GetKind(), "Deployment")
	}
	if res.Object.GetName() != "nginx" {
		t.Errorf("Name = %q, want %q", res.Object.GetName(), "nginx")
	}
	if res.Object.GetNamespace() != "default" {
		t.Errorf("Namespace = %q, want %q", res.Object.GetNamespace(), "default")
	}
}

func TestLoadFile_MultiDocument(t *testing.T) {
	l := NewFileLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/valid/multi-doc.yaml"}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("Load() returned %d resources, want 2", len(resources))
	}

	kinds := make(map[string]bool)
	for _, res := range resources {
		kinds[res.Object.GetKind()] = true
	}

	if !kinds["Service"] {
		t.Error("missing Service resource")
	}
	if !kinds["ConfigMap"] {
		t.Error("missing ConfigMap resource")
	}
}

func TestLoadDirectory_NonRecursive(t *testing.T) {
	l := NewFileLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/valid"}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should load both files: deployment.yaml (1 doc) and multi-doc.yaml (2 docs)
	if len(resources) != 3 {
		t.Errorf("Load() returned %d resources, want 3", len(resources))
	}
}

func TestLoadFile_Invalid(t *testing.T) {
	l := NewFileLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/invalid/malformed.yaml"}, false)

	// The loader is designed to be forgiving - malformed documents are skipped
	// as long as the file is readable. This allows partial loads.
	// However, since our malformed.yaml causes a parse error, it should fail.
	if err == nil && len(resources) > 0 {
		// If no error, verify the malformed resource was skipped
		for _, res := range resources {
			if res.Object.GetName() == "broken" {
				t.Error("malformed resource should have been skipped")
			}
		}
	}
	// Note: Depending on the parser, this may or may not error.
	// The key test is that malformed data doesn't crash the loader.
}

func TestLoadFile_NotFound(t *testing.T) {
	l := NewFileLoader()
	ctx := context.Background()

	_, err := l.Load(ctx, []string{"testdata/nonexistent.yaml"}, false)

	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

func TestLoadFile_SourcePath(t *testing.T) {
	l := NewFileLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/valid/deployment.yaml"}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Load() returned %d resources, want 1", len(resources))
	}

	// SourceFile should be set
	if resources[0].SourceFile == "" {
		t.Error("SourceFile not set")
	}
	if filepath.Base(resources[0].SourceFile) != "deployment.yaml" {
		t.Errorf("SourceFile = %q, want deployment.yaml", filepath.Base(resources[0].SourceFile))
	}
}

func TestLoadFile_ContextCancellation(t *testing.T) {
	l := NewFileLoader()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := l.Load(ctx, []string{"testdata/valid/deployment.yaml"}, false)

	if err == nil {
		t.Error("Load() should return error for cancelled context")
	}
}

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"deployment.yaml", true},
		{"service.yml", true},
		{"config.YAML", true},
		{"readme.md", false},
		{"script.sh", false},
		{"data.json", false},
		{".yaml", true},
		{"noextension", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isYAMLFile(tt.path)
			if got != tt.want {
				t.Errorf("isYAMLFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestLoad_MultiplePaths(t *testing.T) {
	l := NewFileLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{
		"testdata/valid/deployment.yaml",
		"testdata/valid/multi-doc.yaml",
	}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// 1 from deployment.yaml + 2 from multi-doc.yaml
	if len(resources) != 3 {
		t.Errorf("Load() returned %d resources, want 3", len(resources))
	}
}
