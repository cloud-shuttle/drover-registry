package s3

import (
	"context"
	"errors"
	"io"

	"github.com/cloud-shuttle/drover-registry/internal/storage"
)

// S3 implements storage.Provider backed by AWS S3 or S3-compatible (MinIO).
// TODO: full implementation using aws-sdk-go-v2 + checksum verification.
type S3 struct {
	// config, client etc.
}

func New(cfg storage.Config) (*S3, error) {
	return nil, errors.New("S3 backend not yet implemented (coming in dreg-001)")
}

func (s *S3) Put(ctx context.Context, ref storage.PackageRef, r io.Reader, size int64, checksum string) (*storage.ObjectInfo, error) {
	return nil, errors.New("not implemented")
}
func (s *S3) Get(ctx context.Context, ref storage.PackageRef) (io.ReadCloser, *storage.ObjectInfo, error) {
	return nil, nil, errors.New("not implemented")
}
func (s *S3) Delete(ctx context.Context, ref storage.PackageRef) error {
	return errors.New("not implemented")
}
func (s *S3) Exists(ctx context.Context, ref storage.PackageRef) (bool, error) {
	return false, errors.New("not implemented")
}
func (s *S3) Head(ctx context.Context, ref storage.PackageRef) (*storage.ObjectInfo, error) {
	return nil, errors.New("not implemented")
}
func (s *S3) ListVersions(ctx context.Context, tenantID, name string) ([]string, error) {
	return nil, errors.New("not implemented")
}
