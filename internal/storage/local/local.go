package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cloud-shuttle/drover-registry/internal/storage"
)

// Local implements storage.Provider using the local filesystem.
// Layout: <root>/<tenantID>/<name>/<version>/<digest>.tar.gz  (or .tar)
type Local struct {
	root string
}

// New creates a Local storage provider rooted at the given directory.
func New(root string) (*Local, error) {
	if root == "" {
		return nil, errors.New("local storage root cannot be empty")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create root dir: %w", err)
	}
	return &Local{root: filepath.Clean(root)}, nil
}

func (l *Local) key(ref storage.PackageRef) string {
	// tenant/name/version/digest  (digest already includes algo prefix usually)
	d := strings.TrimPrefix(ref.Digest, "sha256:")
	return filepath.Join(ref.TenantID, sanitize(ref.Name), sanitize(ref.Version), d)
}

func sanitize(s string) string {
	// very basic sanitization; real impl should be stricter
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, s)
}

func (l *Local) fullPath(ref storage.PackageRef) string {
	return filepath.Join(l.root, l.key(ref))
}

func (l *Local) Put(ctx context.Context, ref storage.PackageRef, r io.Reader, size int64, checksum string) (*storage.ObjectInfo, error) {
	if ref.TenantID == "" || ref.Name == "" || ref.Version == "" || checksum == "" {
		return nil, errors.New("ref fields and checksum are required")
	}

	p := l.fullPath(ref)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	// Write to temp then rename for atomicity
	tmp := p + ".tmp." + hex.EncodeToString([]byte(time.Now().String()))[:8]
	f, err := os.Create(tmp)
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmp) // cleanup on error paths
	}()

	h := sha256.New()
	w := io.MultiWriter(f, h)

	n, err := io.Copy(w, r)
	if err != nil {
		return nil, fmt.Errorf("write stream: %w", err)
	}
	if size > 0 && n != size {
		return nil, fmt.Errorf("size mismatch: wrote %d, expected %d", n, size)
	}

	got := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if got != checksum {
		return nil, fmt.Errorf("%w: got %s want %s", storage.ErrChecksumMismatch, got, checksum)
	}

	if err := f.Sync(); err != nil {
		return nil, fmt.Errorf("fsync: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmp, p); err != nil {
		return nil, fmt.Errorf("rename: %w", err)
	}

	info := &storage.ObjectInfo{
		Ref:        ref,
		Size:       n,
		Checksum:   checksum,
		StoredAt:   time.Now().UTC(),
		StorageKey: l.key(ref),
	}
	return info, nil
}

func (l *Local) Get(ctx context.Context, ref storage.PackageRef) (io.ReadCloser, *storage.ObjectInfo, error) {
	p := l.fullPath(ref)
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, storage.ErrNotFound
		}
		return nil, nil, fmt.Errorf("open: %w", err)
	}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("stat: %w", err)
	}

	info := &storage.ObjectInfo{
		Ref:        ref,
		Size:       stat.Size(),
		Checksum:   "", // caller can compute or we can store sidecar
		StoredAt:   stat.ModTime().UTC(),
		StorageKey: l.key(ref),
	}
	return f, info, nil
}

func (l *Local) Delete(ctx context.Context, ref storage.PackageRef) error {
	p := l.fullPath(ref)
	err := os.Remove(p)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove: %w", err)
	}
	// best-effort cleanup of empty parent dirs
	_ = os.Remove(filepath.Dir(p))
	_ = os.Remove(filepath.Dir(filepath.Dir(p)))
	return nil
}

func (l *Local) Exists(ctx context.Context, ref storage.PackageRef) (bool, error) {
	p := l.fullPath(ref)
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (l *Local) Head(ctx context.Context, ref storage.PackageRef) (*storage.ObjectInfo, error) {
	p := l.fullPath(ref)
	stat, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return &storage.ObjectInfo{
		Ref:        ref,
		Size:       stat.Size(),
		Checksum:   "", // TODO: read .sha256 sidecar or store in DB
		StoredAt:   stat.ModTime().UTC(),
		StorageKey: l.key(ref),
	}, nil
}

func (l *Local) ListVersions(ctx context.Context, tenantID, name string) ([]string, error) {
	dir := filepath.Join(l.root, sanitize(tenantID), sanitize(name))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	vers := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			vers = append(vers, e.Name())
		}
	}
	sort.Strings(vers)
	return vers, nil
}
