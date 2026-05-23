package gcs

import (
	"context"
	"errors"
	"io"

	"github.com/cloud-shuttle/drover-registry/internal/storage"
)

// GCS implements storage.Provider backed by Google Cloud Storage.
// TODO: full implementation using cloud.google.com/go/storage + checksums.
type GCS struct{}

func New(cfg interface{}) (*GCS, error) {
	return nil, errors.New("GCS backend not yet implemented (real impl in progress)")
}

func (g *GCS) Put(ctx context.Context, ref storage.PackageRef, r io.Reader, size int64, checksum string) (*storage.ObjectInfo, error) {
	return nil, errors.New("not implemented")
}
func (g *GCS) Get(ctx context.Context, ref storage.PackageRef) (io.ReadCloser, *storage.ObjectInfo, error) {
	return nil, nil, errors.New("not implemented")
}
func (g *GCS) Delete(ctx context.Context, ref storage.PackageRef) error {
	return errors.New("not implemented")
}
func (g *GCS) Exists(ctx context.Context, ref storage.PackageRef) (bool, error) {
	return false, errors.New("not implemented")
}
func (g *GCS) Head(ctx context.Context, ref storage.PackageRef) (*storage.ObjectInfo, error) {
	return nil, errors.New("not implemented")
}
func (g *GCS) ListVersions(ctx context.Context, tenantID, name string) ([]string, error) {
	return nil, errors.New("not implemented")
}
