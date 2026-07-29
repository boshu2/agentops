package evalsubstrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ModelSpecPath is the single checked sink for model-spec paths: it validates
// and encodes the id, so there is no way to build a model-spec path from an
// unvalidated id. Model-spec ids may contain ':' (for example "ms:stable"),
// which is a Windows alternate-data-stream separator, so ':' is percent-encoded
// to "%3A" before it becomes a directory name. The encoding is one-way and
// internal: both CaptureModelSpec (write) and LoadModelSpec (read) resolve
// through here, so the raw ':' form never reaches the filesystem and is never
// decoded back — no decoder is needed. Returns an error for any id that is not a
// safe single path component after encoding.
func ModelSpecPath(evalsRoot, specID string) (string, error) {
	// '%' is reserved in raw ids so the ':'→"%3A" encoding stays injective:
	// otherwise the raw id "ms%3Astable" would alias the encoded form of
	// "ms:stable" and two distinct ids could address one on-disk entry.
	if strings.Contains(specID, "%") {
		return "", fmt.Errorf("model-spec id %q contains a reserved '%%' character", specID)
	}
	component := strings.ReplaceAll(specID, ":", "%3A")
	if err := ValidateID(component); err != nil {
		return "", fmt.Errorf("model-spec id %q: %w", specID, err)
	}
	return filepath.Join(evalsRoot, "models", component, "spec.yaml"), nil
}

// CaptureModelSpec writes the ModelSpec to disk after stamping content_hash.
// Returns (specID, contentHash) for Manifest.ModelSpecRef + Manifest.ModelSpecHash.
func CaptureModelSpec(evalsRoot string, spec *ModelSpec) (string, string, error) {
	dest, err := ModelSpecPath(evalsRoot, spec.ID)
	if err != nil {
		return "", "", fmt.Errorf("CaptureModelSpec: %w", err)
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
	if err := WriteAtomic(dest, finalCanon); err != nil {
		return "", "", fmt.Errorf("CaptureModelSpec: write: %w", err)
	}
	return spec.ID, hash, nil
}

func LoadModelSpec(evalsRoot, specID string) (*ModelSpec, error) {
	path, err := ModelSpecPath(evalsRoot, specID)
	if err != nil {
		return nil, fmt.Errorf("LoadModelSpec: %w", err)
	}
	raw, err := os.ReadFile(path)
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
