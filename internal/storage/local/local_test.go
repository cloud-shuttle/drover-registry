package local

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloud-shuttle/drover-registry/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestLocal_PutGetDelete(t *testing.T) {
	tmp := t.TempDir()
	l, err := New(tmp)
	require.NoError(t, err)

	data := []byte("hello drover registry crew template v1")
	sum := sha256.Sum256(data)
	checksum := "sha256:" + hex.EncodeToString(sum[:])

	ref := storage.PackageRef{
		TenantID: "test-org",
		Name:     "test-crew",
		Version:  "v0.0.1-test",
		Digest:   checksum,
	}

	// Put
	info, err := l.Put(context.Background(), ref, bytes.NewReader(data), int64(len(data)), checksum)
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), info.Size)
	require.Equal(t, checksum, info.Checksum)

	// Exists
	ok, err := l.Exists(context.Background(), ref)
	require.NoError(t, err)
	require.True(t, ok)

	// Get
	rc, gotInfo, err := l.Get(context.Background(), ref)
	require.NoError(t, err)
	require.Equal(t, info.Size, gotInfo.Size)

	read, err := os.ReadFile(filepath.Join(tmp, l.key(ref))) // direct for verification
	require.NoError(t, err)
	require.Equal(t, data, read)
	rc.Close()

	// Delete
	require.NoError(t, l.Delete(context.Background(), ref))
	ok, _ = l.Exists(context.Background(), ref)
	require.False(t, ok)

	// Put with bad checksum should fail
	badRef := ref
	badRef.Digest = "sha256:deadbeef"
	_, err = l.Put(context.Background(), badRef, bytes.NewReader(data), int64(len(data)), badRef.Digest)
	require.ErrorIs(t, err, ErrChecksumMismatch)
}
