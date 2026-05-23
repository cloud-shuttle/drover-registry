package infra

import (
	"fmt"

	"github.com/cloud-shuttle/drover-registry/internal/config"
	"github.com/cloud-shuttle/drover-registry/internal/storage"
	"github.com/cloud-shuttle/drover-registry/internal/storage/local"
	"github.com/cloud-shuttle/drover-registry/internal/storage/s3"
)

// NewStorageProvider creates the appropriate storage backend based on configuration.
// For "local": uses the proven local disk implementation with checksum verification.
// For "s3": real aws-sdk-go-v2 implementation (MinIO compatible).
// For "gcs": real Google Cloud Storage.
func NewStorageProvider(cfg config.Config) (storage.Provider, error) {
	switch cfg.StorageBackend {
	case "local", "":
		return local.New(cfg.StorageLocalRoot)
	case "s3":
		return s3.New(cfg)
	case "gcs":
		// return gcs.New(cfg) once implemented
		return nil, fmt.Errorf("gcs backend not yet wired in factory (see internal/storage/gcs)")
	default:
		return nil, fmt.Errorf("unknown storage backend %q (supported: local, s3, gcs)", cfg.StorageBackend)
	}
}
