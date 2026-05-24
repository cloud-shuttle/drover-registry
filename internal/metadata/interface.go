package metadata

import (
	"context"
)

type RegistryPackageInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Digest      string    `json:"digest"`
	SizeBytes   int64     `json:"size_bytes"`
	PublishedBy string    `json:"published_by"`
}

type FederatedCrewStore interface {
	PublishPackage(ctx context.Context, tenantID string, name string, version string, digest string, sizeBytes int64, storageKey string, manifest []byte, publishedBy string) error
	FetchPackage(ctx context.Context, tenantID string, name string, version string) (*RegistryPackageInfo, error)
	ListPackages(ctx context.Context, tenantID string) ([]RegistryPackageInfo, error)
}
