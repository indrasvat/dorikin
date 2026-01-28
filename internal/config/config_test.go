package config

import (
	"testing"
)

func TestLoadFrom_WithPaths(t *testing.T) {
	cfg, err := LoadFrom("testdata")
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	paths := cfg.EffectivePaths()
	if len(paths) != 2 {
		t.Fatalf("EffectivePaths() = %d paths, want 2", len(paths))
	}

	if paths[0] != "./manifests/" {
		t.Errorf("paths[0] = %q, want %q", paths[0], "./manifests/")
	}
	if paths[1] != "./k8s/" {
		t.Errorf("paths[1] = %q, want %q", paths[1], "./k8s/")
	}
}

func TestEffectivePaths_Empty(t *testing.T) {
	cfg := DefaultConfig()
	paths := cfg.EffectivePaths()
	if paths != nil {
		t.Errorf("EffectivePaths() = %v, want nil", paths)
	}
}

func TestDefaultConfig_NoPaths(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Paths != nil {
		t.Errorf("DefaultConfig().Paths = %v, want nil", cfg.Paths)
	}
}
