package gcs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/cloud-shuttle/drover-registry/internal/config"
	appstorage "github.com/cloud-shuttle/drover-registry/internal/storage"
)

// GCS implements appstorage.Provider backed by Google Cloud Storage.
type GCS struct {
	client *storage.Client
	bucket string
}

func New(ctx context.Context, appCfg config.Config) (*GCS, error) {
	if appCfg.GCSBucket == "" {
		return nil, errors.New("GCS_BUCKET is required for gcs backend")
	}

	var opts []option.ClientOption
	if appCfg.GCSCredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(appCfg.GCSCredentialsFile))
	}

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCS{
		client: client,
		bucket: appCfg.GCSBucket,
	}, nil
}

func (g *GCS) key(ref appstorage.PackageRef) string {
	return fmt.Sprintf("%s/%s/%s/%s", ref.TenantID, ref.Name, ref.Version, ref.Digest)
}

func (g *GCS) Put(ctx context.Context, ref appstorage.PackageRef, r io.Reader, size int64, checksum string) (*appstorage.ObjectInfo, error) {
	key := g.key(ref)
	obj := g.client.Bucket(g.bucket).Object(key)

	w := obj.NewWriter(ctx)
	h := sha256.New()
	tr := io.TeeReader(r, h)

	n, err := io.Copy(w, tr)
	if err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("gcs put stream: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gcs put close: %w", err)
	}

	got := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if got != checksum {
		_ = obj.Delete(ctx)
		return nil, fmt.Errorf("%w: got %s want %s", appstorage.ErrChecksumMismatch, got, checksum)
	}

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs attrs after put: %w", err)
	}

	info := &appstorage.ObjectInfo{
		Ref:        ref,
		Size:       n, // or attrs.Size
		Checksum:   checksum,
		StoredAt:   attrs.Updated,
		StorageKey: key,
	}
	return info, nil
}

func (g *GCS) Get(ctx context.Context, ref appstorage.PackageRef) (io.ReadCloser, *appstorage.ObjectInfo, error) {
	key := g.key(ref)
	obj := g.client.Bucket(g.bucket).Object(key)

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, nil, appstorage.ErrNotFound
		}
		return nil, nil, fmt.Errorf("gcs get attrs: %w", err)
	}

	r, err := obj.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, nil, appstorage.ErrNotFound
		}
		return nil, nil, fmt.Errorf("gcs get reader: %w", err)
	}

	info := &appstorage.ObjectInfo{
		Ref:        ref,
		Size:       attrs.Size,
		Checksum:   "",
		StoredAt:   attrs.Updated,
		StorageKey: key,
	}
	return r, info, nil
}

func (g *GCS) Delete(ctx context.Context, ref appstorage.PackageRef) error {
	key := g.key(ref)
	err := g.client.Bucket(g.bucket).Object(key).Delete(ctx)
	if err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
		return err
	}
	return nil
}

func (g *GCS) Exists(ctx context.Context, ref appstorage.PackageRef) (bool, error) {
	key := g.key(ref)
	_, err := g.client.Bucket(g.bucket).Object(key).Attrs(ctx)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	}
	return false, err
}

func (g *GCS) Head(ctx context.Context, ref appstorage.PackageRef) (*appstorage.ObjectInfo, error) {
	key := g.key(ref)
	attrs, err := g.client.Bucket(g.bucket).Object(key).Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, appstorage.ErrNotFound
		}
		return nil, err
	}

	return &appstorage.ObjectInfo{
		Ref:        ref,
		Size:       attrs.Size,
		Checksum:   "",
		StoredAt:   attrs.Updated,
		StorageKey: key,
	}, nil
}

func (g *GCS) ListVersions(ctx context.Context, tenantID, name string) ([]string, error) {
	// Stubbed for now, full implementation would use Query over prefix
	return []string{}, nil
}
