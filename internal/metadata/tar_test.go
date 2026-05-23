package metadata

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractManifestFromTarball(t *testing.T) {
	manifest := PackageManifest{
		Name:        "test-crew",
		Version:     "v1.2.3",
		Description: "A test agent crew",
		Type:        "crew",
		Dependencies: []string{"base-agent"},
	}

	manifestBytes, _ := json.Marshal(manifest)

	// Build a tar.gz in memory
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add manifest.json at root
	hdr := &tar.Header{
		Name: "manifest.json",
		Mode: 0600,
		Size: int64(len(manifestBytes)),
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write(manifestBytes)
	require.NoError(t, err)

	// Add a dummy file
	dummy := []byte("some agent code")
	hdr2 := &tar.Header{Name: "agent.py", Mode: 0600, Size: int64(len(dummy))}
	require.NoError(t, tw.WriteHeader(hdr2))
	_, err = tw.Write(dummy)
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	// Now extract
	m, raw, err := ExtractManifestFromTarball(&buf)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, "test-crew", m.Name)
	require.Equal(t, "v1.2.3", m.Version)
	require.Equal(t, "crew", m.Type)
	require.NotEmpty(t, raw)

	// Verify roundtrip
	var back PackageManifest
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Equal(t, manifest.Name, back.Name)
}

func TestExtractManifestFromTarball_NoManifest(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{Name: "readme.md", Mode: 0600, Size: 5}
	require.NoError(t, tw.WriteHeader(hdr))
	_, _ = tw.Write([]byte("hello"))

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	m, _, err := ExtractManifestFromTarball(&buf)
	require.NoError(t, err)
	require.Nil(t, m) // no manifest found is not an error
}
