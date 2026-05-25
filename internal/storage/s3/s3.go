package s3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/cloud-shuttle/drover-registry/internal/config"
	appstorage "github.com/cloud-shuttle/drover-registry/internal/storage"
)

// S3 implements appstorage.Provider using AWS S3 or S3-compatible (MinIO).
type S3 struct {
	client *s3.Client
	bucket string
	region string
}

func New(appCfg config.Config) (*S3, error) {
	if appCfg.S3Bucket == "" {
		return nil, errors.New("S3_BUCKET is required for s3 backend")
	}

	var opts []func(*awsconfig.LoadOptions) error

	opts = append(opts, awsconfig.WithRegion(appCfg.S3Region))

	if appCfg.S3AccessKeyID != "" && appCfg.S3SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				appCfg.S3AccessKeyID,
				appCfg.S3SecretAccessKey,
				"",
			),
		))
	}

	// For MinIO / custom endpoints
	if appCfg.S3Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               appCfg.S3Endpoint,
					SigningRegion:     appCfg.S3Region,
					HostnameImmutable: true,
				}, nil
			})
		opts = append(opts, awsconfig.WithEndpointResolverWithOptions(customResolver))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if appCfg.S3UsePathStyle {
			o.UsePathStyle = true
		}
	})

	// Quick bucket check (best effort)
	_, err = client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(appCfg.S3Bucket),
	})
	if err != nil {
		// Non-fatal in dev (bucket may be created later)
		fmt.Printf("warning: could not head S3 bucket %s: %v\n", appCfg.S3Bucket, err)
	}

	return &S3{
		client: client,
		bucket: appCfg.S3Bucket,
		region: appCfg.S3Region,
	}, nil
}

func (s *S3) key(ref appstorage.PackageRef) string {
	return fmt.Sprintf("%s/%s/%s/%s", ref.TenantID, ref.Name, ref.Version, ref.Digest)
}

func (s *S3) Put(ctx context.Context, ref appstorage.PackageRef, r io.Reader, size int64, checksum string) (*appstorage.ObjectInfo, error) {
	key := s.key(ref)

	h := sha256.New()
	tr := io.TeeReader(r, h)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   tr,
		// ContentLength: aws.Int64(size), // optional
	})
	if err != nil {
		return nil, fmt.Errorf("s3 put: %w", err)
	}

	got := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if got != checksum {
		// Clean up the invalid upload
		_ = s.Delete(ctx, ref)
		return nil, fmt.Errorf("%w: got %s want %s", appstorage.ErrChecksumMismatch, got, checksum)
	}

	// Best-effort: verify by head
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 head after put: %w", err)
	}

	storedSize := int64(0)
	if head.ContentLength != nil {
		storedSize = *head.ContentLength
	}
	info := &appstorage.ObjectInfo{
		Ref:        ref,
		Size:       storedSize,
		Checksum:   checksum,
		StoredAt:   time.Now().UTC(),
		StorageKey: key,
	}
	return info, nil
}

func (s *S3) Get(ctx context.Context, ref appstorage.PackageRef) (io.ReadCloser, *appstorage.ObjectInfo, error) {
	key := s.key(ref)

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, nil, appstorage.ErrNotFound
		}
		return nil, nil, fmt.Errorf("s3 get: %w", err)
	}

	storedSize := int64(0)
	if out.ContentLength != nil {
		storedSize = *out.ContentLength
	}
	info := &appstorage.ObjectInfo{
		Ref:        ref,
		Size:       storedSize,
		Checksum:   "",
		StoredAt:   time.Now().UTC(),
		StorageKey: key,
	}
	return out.Body, info, nil
}

func (s *S3) Delete(ctx context.Context, ref appstorage.PackageRef) error {
	key := s.key(ref)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3) Exists(ctx context.Context, ref appstorage.PackageRef) (bool, error) {
	key := s.key(ref)
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3) Head(ctx context.Context, ref appstorage.PackageRef) (*appstorage.ObjectInfo, error) {
	key := s.key(ref)
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, appstorage.ErrNotFound
		}
		return nil, err
	}
	storedSize := int64(0)
	if head.ContentLength != nil {
		storedSize = *head.ContentLength
	}
	return &appstorage.ObjectInfo{
		Ref:        ref,
		Size:       storedSize,
		Checksum:   "",
		StoredAt:   time.Now().UTC(),
		StorageKey: key,
	}, nil
}

func (s *S3) ListVersions(ctx context.Context, tenantID, name string) ([]string, error) {
	// For a full implementation we would use ListObjectsV2 with prefix.
	// Stub for now.
	return []string{}, nil
}
