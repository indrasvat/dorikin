// Package config handles dorikin configuration loading and defaults.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the dorikin configuration.
type Config struct {
	// Paths lists default manifest paths to scan.
	// Used when no paths are specified via CLI flags or arguments.
	Paths []string `yaml:"paths"`

	// Ignore configures field paths to skip during drift comparison.
	Ignore IgnoreConfig `yaml:"ignore"`

	// HPAAware controls HPA-aware drift detection mode.
	// Valid values: "manifests" (default), "cluster", "disabled"
	HPAAware string `yaml:"hpa_aware"`
}

// IgnoreConfig configures which field paths to ignore during comparison.
type IgnoreConfig struct {
	// ExtendDefaults when true (default), adds custom paths to built-in defaults.
	// When false, only uses the paths specified in Paths.
	ExtendDefaults *bool `yaml:"extend_defaults"`

	// Paths lists additional field paths to ignore globally (all resource types).
	// Supports exact matches (spec.replicas) and wildcards (spec.containers.resources).
	Paths []string `yaml:"paths"`

	// Resources specifies resource-type-specific ignore paths.
	// Keys are "Kind" (e.g., "Deployment", "ConfigMap", "MyCustomResource").
	// Values are lists of field paths to ignore for that resource type.
	Resources map[string][]string `yaml:"resources"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	extendDefaults := true
	return &Config{
		Ignore: IgnoreConfig{
			ExtendDefaults: &extendDefaults,
			Paths:          nil,
		},
		HPAAware: "manifests",
	}
}

// Load reads configuration from .dorikin.yaml in the current directory
// or any parent directory. Returns default config if no file found.
func Load() (*Config, error) {
	return LoadFrom("")
}

// LoadFrom reads configuration starting from the given directory.
// If dir is empty, uses the current working directory.
// Returns default config if no config file is found or directory cannot be determined.
func LoadFrom(dir string) (*Config, error) {
	if dir == "" {
		// Best effort to get cwd; if it fails, dir stays empty
		// and findConfigFile will return "" (no config found)
		dir, _ = os.Getwd()
	}

	// Walk up directory tree looking for .dorikin.yaml
	configPath := findConfigFile(dir)
	if configPath == "" {
		return DefaultConfig(), nil
	}

	return loadFile(configPath)
}

// findConfigFile walks up from dir looking for .dorikin.yaml
func findConfigFile(dir string) string {
	for {
		candidate := filepath.Join(dir, ".dorikin.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		// Also check .dorikin.yml
		candidate = filepath.Join(dir, ".dorikin.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			return ""
		}
		dir = parent
	}
}

// loadFile reads and parses a config file.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// EffectiveIgnorePaths returns the final list of global paths to ignore,
// combining defaults (if ExtendDefaults is true) with custom paths.
func (c *Config) EffectiveIgnorePaths() []string {
	extendDefaults := c.Ignore.ExtendDefaults == nil || *c.Ignore.ExtendDefaults

	if extendDefaults {
		// Start with defaults, append custom
		paths := make([]string, 0, len(DefaultIgnorePaths())+len(c.Ignore.Paths))
		paths = append(paths, DefaultIgnorePaths()...)
		paths = append(paths, c.Ignore.Paths...)
		return paths
	}

	// Custom paths only
	if c.Ignore.Paths == nil {
		return []string{}
	}
	return c.Ignore.Paths
}

// IgnorePathsForResource returns ignore paths for a specific resource kind.
// This combines global paths with resource-specific paths.
func (c *Config) IgnorePathsForResource(kind string) []string {
	global := c.EffectiveIgnorePaths()

	if c.Ignore.Resources == nil {
		return global
	}

	resourcePaths, ok := c.Ignore.Resources[kind]
	if !ok || len(resourcePaths) == 0 {
		return global
	}

	// Combine global + resource-specific
	combined := make([]string, 0, len(global)+len(resourcePaths))
	combined = append(combined, global...)
	combined = append(combined, resourcePaths...)
	return combined
}

// EffectiveHPAAware returns the HPA awareness mode, defaulting to "manifests".
func (c *Config) EffectiveHPAAware() string {
	if c.HPAAware == "" {
		return "manifests"
	}
	return c.HPAAware
}

// EffectivePaths returns the configured manifest paths.
// Returns nil if no paths are configured.
func (c *Config) EffectivePaths() []string {
	return c.Paths
}
