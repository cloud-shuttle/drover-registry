package metadata

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ExtractManifestFromTarball looks inside a .tar.gz (or .tar) for a file named
// manifest.json (or drover-registry.json / package.json) at the root and parses it.
func ExtractManifestFromTarball(r io.Reader) (*PackageManifest, []byte, error) {
	// Try gzip first, fall back to plain tar
	gr, err := gzip.NewReader(r)
	if err != nil {
		// Not gzipped? try as plain tar
		return extractFromTar(r)
	}
	defer gr.Close()

	return extractFromTar(gr)
}

func extractFromTar(r io.Reader) (*PackageManifest, []byte, error) {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("tar read error: %w", err)
		}

		name := hdr.Name
		// Accept manifest.json, ./manifest.json, drover-registry.json etc.
		base := strings.ToLower(strings.TrimPrefix(name, "./"))
		if base == "manifest.json" || base == "drover-registry.json" || base == "package.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read %s: %w", name, err)
			}
			var m PackageManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, nil, fmt.Errorf("invalid %s: %w", name, err)
			}
			return &m, data, nil
		}

		// Skip large files we don't care about for manifest discovery
		if hdr.Size > 2*1024*1024 {
			continue
		}
	}

	return nil, nil, nil // no manifest found (still valid for some artifact types)
}
