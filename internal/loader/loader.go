// Package loader provides manifest loading functionality.
package loader

import (
	"context"

	"github.com/indrasvat/dorikin/pkg/api"
)

// Loader is the interface for loading Kubernetes manifests.
type Loader interface {
	// Load loads manifests from the given paths.
	Load(ctx context.Context, paths []string, recursive bool) ([]api.Resource, error)
}

// Type identifies the type of loader.
type Type string

const (
	TypeFile      Type = "file"
	TypeKustomize Type = "kustomize"
	TypeHelm      Type = "helm"
)
