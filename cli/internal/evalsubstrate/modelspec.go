package evalsubstrate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

func ModelSpecPath(evalsRoot, specID string) string {
	return filepath.Join(evalsRoot, "models", specID, "spec.yaml")
}

// CaptureModelSpec writes the ModelSpec to disk after stamping content_hash.
// Returns (specID, contentHash) for Manifest.ModelSpecRef + Manifest.ModelSpecHash.
func CaptureModelSpec(evalsRoot string, spec *ModelSpec) (string, string, error) {
	if spec.ID == "" {
		return "", "", fmt.Errorf("CaptureModelSpec: empty id")
	}
	if spec.SchemaVersion == 0 {
		spec.SchemaVersion = SchemaVersion
	}
	if spec.CapturedAt == "" {
		spec.CapturedAt = timeNow().UTC().Format(time.RFC3339)
	}

	prev := spec.ContentHash
	spec.ContentHash = ""
	rawYAML, err := yaml.Marshal(spec)
	if err != nil {
		spec.ContentHash = prev
		return "", "", fmt.Errorf("CaptureModelSpec: marshal: %w", err)
	}
	canon, err := CanonicalizeYAML(rawYAML)
	if err != nil {
		spec.ContentHash = prev
		return "", "", fmt.Errorf("CaptureModelSpec: canonicalize: %w", err)
	}
	hash := ContentHash(canon)
	spec.ContentHash = hash

	finalYAML, err := yaml.Marshal(spec)
	if err != nil {
		return "", "", fmt.Errorf("CaptureModelSpec: re-marshal: %w", err)
	}
	finalCanon, err := CanonicalizeYAML(finalYAML)
	if err != nil {
		return "", "", fmt.Errorf("CaptureModelSpec: re-canonicalize: %w", err)
	}
	dest := ModelSpecPath(evalsRoot, spec.ID)
	if err := WriteAtomic(dest, finalCanon); err != nil {
		return "", "", fmt.Errorf("CaptureModelSpec: write: %w", err)
	}
	return spec.ID, hash, nil
}

func LoadModelSpec(evalsRoot, specID string) (*ModelSpec, error) {
	raw, err := os.ReadFile(ModelSpecPath(evalsRoot, specID))
	if err != nil {
		return nil, fmt.Errorf("LoadModelSpec: %w", err)
	}
	var spec ModelSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("LoadModelSpec: parse: %w", err)
	}
	return &spec, nil
}

func ModelSpecHashEqual(a, b *ModelSpec) bool {
	return a != nil && b != nil && a.ContentHash != "" && a.ContentHash == b.ContentHash
}
