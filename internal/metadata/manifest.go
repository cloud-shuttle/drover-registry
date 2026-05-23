package metadata

import (
	"encoding/json"
	"fmt"
	"io"
)

// PackageManifest is the minimal metadata file expected inside every published artifact
// (usually at the root of the tarball as manifest.json or drover-registry.json).
//
// This is the core of dreg-002 "Version indexing & metadata API parser".
type PackageManifest struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description,omitempty"`
	Type         string   `json:"type"` // "agent" | "crew" | "template"
	Entrypoint   string   `json:"entrypoint,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Labels       []string `json:"labels,omitempty"`
	// Checksum is of the whole tarball, recorded by the registry on upload
	// (not required in the manifest the client sends).
}

// ParseManifest reads a manifest.json from an io.Reader (usually from inside the uploaded tarball).
func ParseManifest(r io.Reader) (*PackageManifest, error) {
	var m PackageManifest
	dec := json.NewDecoder(r)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("invalid manifest.json: %w", err)
	}
	if m.Name == "" || m.Version == "" || m.Type == "" {
		return nil, fmt.Errorf("manifest must contain at minimum: name, version, type")
	}
	return &m, nil
}
