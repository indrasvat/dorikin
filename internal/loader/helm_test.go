package loader

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func helmAvailable() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}

func TestNewHelmLoader(t *testing.T) {
	l := NewHelmLoader()
	if l == nil {
		t.Fatal("NewHelmLoader() returned nil")
	}
}

func TestNewHelmLoader_WithOptions(t *testing.T) {
	l := NewHelmLoader(
		WithRelease("my-release"),
		WithNamespace("my-namespace"),
		WithValues([]string{"values.yaml"}),
		WithSet([]string{"key=value"}),
		WithHelmPath("/custom/helm"),
	)

	if l.releaseName != "my-release" {
		t.Errorf("releaseName = %q, want %q", l.releaseName, "my-release")
	}
	if l.namespace != "my-namespace" {
		t.Errorf("namespace = %q, want %q", l.namespace, "my-namespace")
	}
	if len(l.values) != 1 || l.values[0] != "values.yaml" {
		t.Errorf("values = %v, want [values.yaml]", l.values)
	}
	if len(l.setValues) != 1 || l.setValues[0] != "key=value" {
		t.Errorf("setValues = %v, want [key=value]", l.setValues)
	}
	if l.helmPath != "/custom/helm" {
		t.Errorf("helmPath = %q, want %q", l.helmPath, "/custom/helm")
	}
}

func TestHelmLoader_DefaultRelease(t *testing.T) {
	l := NewHelmLoader()
	if l.releaseName != "release" {
		t.Errorf("default releaseName = %q, want %q", l.releaseName, "release")
	}
}

func TestHelmLoader_Load_ValidChart(t *testing.T) {
	if !helmAvailable() {
		t.Skip("helm not available in PATH")
	}

	l := NewHelmLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/helm/valid"}, false)
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

func TestHelmLoader_Load_WithNamespace(t *testing.T) {
	if !helmAvailable() {
		t.Skip("helm not available in PATH")
	}

	l := NewHelmLoader(WithNamespace("custom-ns"))
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/helm/valid"}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check that resources have the custom namespace
	for _, res := range resources {
		if res.Object.GetNamespace() != "custom-ns" {
			t.Errorf("resource %s namespace = %q, want %q",
				res.Object.GetName(), res.Object.GetNamespace(), "custom-ns")
		}
	}
}

func TestHelmLoader_Load_SourcePath(t *testing.T) {
	if !helmAvailable() {
		t.Skip("helm not available in PATH")
	}

	l := NewHelmLoader()
	ctx := context.Background()

	resources, err := l.Load(ctx, []string{"testdata/helm/valid"}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for _, res := range resources {
		if res.SourceFile == "" {
			t.Error("SourceFile not set")
		}
		if res.SourceFile != "testdata/helm/valid" {
			t.Errorf("SourceFile = %q, want %q", res.SourceFile, "testdata/helm/valid")
		}
	}
}

func TestHelmLoader_Load_InvalidChart(t *testing.T) {
	if !helmAvailable() {
		t.Skip("helm not available in PATH")
	}

	l := NewHelmLoader()
	ctx := context.Background()

	// testdata/helm/invalid has no Chart.yaml
	_, err := l.Load(ctx, []string{"testdata/helm/invalid"}, false)
	if err == nil {
		t.Error("Load() should return error for invalid chart")
	}
}

func TestHelmLoader_Load_NonexistentPath(t *testing.T) {
	if !helmAvailable() {
		t.Skip("helm not available in PATH")
	}

	l := NewHelmLoader()
	ctx := context.Background()

	_, err := l.Load(ctx, []string{"testdata/helm/nonexistent"}, false)
	if err == nil {
		t.Error("Load() should return error for nonexistent path")
	}
}

func TestHelmLoader_Load_FileNotDirectory(t *testing.T) {
	if !helmAvailable() {
		t.Skip("helm not available in PATH")
	}

	l := NewHelmLoader()
	ctx := context.Background()

	// Pass a file instead of a directory
	_, err := l.Load(ctx, []string{"testdata/helm/valid/Chart.yaml"}, false)
	if err == nil {
		t.Error("Load() should return error when path is not a directory")
	}
}

func TestHelmLoader_Load_ContextCancellation(t *testing.T) {
	if !helmAvailable() {
		t.Skip("helm not available in PATH")
	}

	l := NewHelmLoader()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := l.Load(ctx, []string{"testdata/helm/valid"}, false)
	if err == nil {
		t.Error("Load() should return error for cancelled context")
	}
}

func TestHelmLoader_Load_MultiplePaths(t *testing.T) {
	if !helmAvailable() {
		t.Skip("helm not available in PATH")
	}

	l := NewHelmLoader()
	ctx := context.Background()

	// Load the same chart twice (simulating multiple charts)
	resources, err := l.Load(ctx, []string{
		"testdata/helm/valid",
		"testdata/helm/valid",
	}, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have 4 resources (2 from each load)
	if len(resources) != 4 {
		t.Errorf("Load() returned %d resources, want 4", len(resources))
	}
}

func TestHelmLoader_FindHelm_CustomPath(t *testing.T) {
	l := NewHelmLoader(WithHelmPath("/nonexistent/helm"))

	_, err := l.findHelm()
	if err == nil {
		t.Error("findHelm() should return error for nonexistent custom path")
	}
}

func TestHelmLoader_FindHelm_InPath(t *testing.T) {
	if !helmAvailable() {
		t.Skip("helm not available in PATH")
	}

	l := NewHelmLoader()
	path, err := l.findHelm()
	if err != nil {
		t.Fatalf("findHelm() error = %v", err)
	}
	if path == "" {
		t.Error("findHelm() returned empty path")
	}
}

func TestHelmLoader_WithTimeout(t *testing.T) {
	l := NewHelmLoader(WithTimeout(30 * time.Second))
	if l.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want %v", l.timeout, 30*time.Second)
	}
}

func TestHelmLoader_DefaultTimeout(t *testing.T) {
	l := NewHelmLoader()
	if l.timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want %v", l.timeout, 60*time.Second)
	}
}
