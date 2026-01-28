package loader

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func kustomizeAvailable() bool {
	_, err := exec.LookPath("kustomize")
	return err == nil
}

func TestNewKustomizeLoader(t *testing.T) {
	l := NewKustomizeLoader()
	if l == nil {
		t.Fatal("NewKustomizeLoader() returned nil")
	}
}

func TestNewKustomizeLoader_WithOptions(t *testing.T) {
	l := NewKustomizeLoader(
		WithKustomizePath("/custom/kustomize"),
	)

	if l.kustomizePath != "/custom/kustomize" {
		t.Errorf("kustomizePath = %q, want %q", l.kustomizePath, "/custom/kustomize")
	}
}

func TestKustomizeLoader_Load_ValidDirectory(t *testing.T) {
	if !kustomizeAvailable() {
		t.Skip("kustomize not available in PATH")
	}

	l := NewKustomizeLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/kustomize/valid"}, false)
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

	if !kinds["Deployment"] {
		t.Error("missing Deployment resource")
	}
	if !kinds["Service"] {
		t.Error("missing Service resource")
	}
}

func TestKustomizeLoader_Load_Namespace(t *testing.T) {
	if !kustomizeAvailable() {
		t.Skip("kustomize not available in PATH")
	}

	l := NewKustomizeLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/kustomize/valid"}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check that resources have the namespace set by kustomization.yaml
	for _, res := range resources {
		if res.Object.GetNamespace() != "default" {
			t.Errorf("resource %s namespace = %q, want %q",
				res.Object.GetName(), res.Object.GetNamespace(), "default")
		}
	}
}

func TestKustomizeLoader_Load_SourcePath(t *testing.T) {
	if !kustomizeAvailable() {
		t.Skip("kustomize not available in PATH")
	}

	l := NewKustomizeLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/kustomize/valid"}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for _, res := range resources {
		if res.SourceFile == "" {
			t.Error("SourceFile not set")
		}
		if res.SourceFile != "testdata/kustomize/valid" {
			t.Errorf("SourceFile = %q, want %q", res.SourceFile, "testdata/kustomize/valid")
		}
	}
}

func TestKustomizeLoader_Load_InvalidDirectory(t *testing.T) {
	if !kustomizeAvailable() {
		t.Skip("kustomize not available in PATH")
	}

	l := NewKustomizeLoader()
	ctx := context.Background()

	// testdata/kustomize/invalid has no kustomization.yaml
	_, err := l.Load(ctx, []string{"testdata/kustomize/invalid"}, false)
	if err == nil {
		t.Error("Load() should return error for invalid kustomize directory")
	}
}

func TestKustomizeLoader_Load_NonexistentPath(t *testing.T) {
	if !kustomizeAvailable() {
		t.Skip("kustomize not available in PATH")
	}

	l := NewKustomizeLoader()
	ctx := context.Background()

	_, err := l.Load(ctx, []string{"testdata/kustomize/nonexistent"}, false)
	if err == nil {
		t.Error("Load() should return error for nonexistent path")
	}
}

func TestKustomizeLoader_Load_FileNotDirectory(t *testing.T) {
	if !kustomizeAvailable() {
		t.Skip("kustomize not available in PATH")
	}

	l := NewKustomizeLoader()
	ctx := context.Background()

	// Pass a file instead of a directory
	_, err := l.Load(ctx, []string{"testdata/kustomize/valid/kustomization.yaml"}, false)
	if err == nil {
		t.Error("Load() should return error when path is not a directory")
	}
}

func TestKustomizeLoader_Load_ContextCancellation(t *testing.T) {
	if !kustomizeAvailable() {
		t.Skip("kustomize not available in PATH")
	}

	l := NewKustomizeLoader()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := l.Load(ctx, []string{"testdata/kustomize/valid"}, false)
	if err == nil {
		t.Error("Load() should return error for cancelled context")
	}
}

func TestKustomizeLoader_Load_MultiplePaths(t *testing.T) {
	if !kustomizeAvailable() {
		t.Skip("kustomize not available in PATH")
	}

	l := NewKustomizeLoader()
	ctx := context.Background()

	// Load the same directory twice
	resources, err := l.Load(ctx, []string{
		"testdata/kustomize/valid",
		"testdata/kustomize/valid",
	}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have 4 resources (2 from each load)
	if len(resources) != 4 {
		t.Errorf("Load() returned %d resources, want 4", len(resources))
	}
}

func TestKustomizeLoader_FindKustomize_CustomPath(t *testing.T) {
	l := NewKustomizeLoader(WithKustomizePath("/nonexistent/kustomize"))

	_, err := l.findKustomize()
	if err == nil {
		t.Error("findKustomize() should return error for nonexistent custom path")
	}
}

func TestKustomizeLoader_FindKustomize_InPath(t *testing.T) {
	if !kustomizeAvailable() {
		t.Skip("kustomize not available in PATH")
	}

	l := NewKustomizeLoader()
	path, err := l.findKustomize()
	if err != nil {
		t.Fatalf("findKustomize() error = %v", err)
	}
	if path == "" {
		t.Error("findKustomize() returned empty path")
	}
}

func TestKustomizeLoader_ValidateDir_KustomizationYaml(t *testing.T) {
	l := NewKustomizeLoader()

	err := l.validateKustomizationDir("testdata/kustomize/valid")
	if err != nil {
		t.Errorf("validateKustomizationDir() error = %v", err)
	}
}

func TestKustomizeLoader_ValidateDir_NoKustomization(t *testing.T) {
	l := NewKustomizeLoader()

	err := l.validateKustomizationDir("testdata/kustomize/invalid")
	if err == nil {
		t.Error("validateKustomizationDir() should return error for directory without kustomization.yaml")
	}
}

func TestKustomizeLoader_ValidateDir_NotDirectory(t *testing.T) {
	l := NewKustomizeLoader()

	err := l.validateKustomizationDir("testdata/kustomize/valid/kustomization.yaml")
	if err == nil {
		t.Error("validateKustomizationDir() should return error for non-directory path")
	}
}

func TestKustomizeLoader_WithTimeout(t *testing.T) {
	l := NewKustomizeLoader(WithKustomizeTimeout(30 * time.Second))
	if l.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want %v", l.timeout, 30*time.Second)
	}
}

func TestKustomizeLoader_DefaultTimeout(t *testing.T) {
	l := NewKustomizeLoader()
	if l.timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want %v", l.timeout, 60*time.Second)
	}
}
