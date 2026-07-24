package evalsubstrate

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

func ModelSpecPath(evalsRoot, specID string) (string, error) {
	relative, err := modelSpecRelative(specID)
	if err != nil {
		return "", err
	}
	return filepath.Join(evalsRoot, filepath.FromSlash(relative)), nil
}

func modelSpecRelative(specID string) (string, error) {
	id, err := ParseIdentifier(IdentifierModel, specID)
	if err != nil {
		return "", fmt.Errorf("model spec path: %w", err)
	}
	return filepath.ToSlash(filepath.Join("models", id.StorageName(), "spec.yaml")), nil
}

// CaptureModelSpec writes the ModelSpec to disk after stamping content_hash.
// Returns (specID, contentHash) for Manifest.ModelSpecRef + Manifest.ModelSpecHash.
func CaptureModelSpec(evalsRoot string, spec *ModelSpec) (string, string, error) {
	if spec == nil {
		return "", "", fmt.Errorf("CaptureModelSpec: nil spec")
	}
	id, err := ParseIdentifier(IdentifierModel, spec.ID)
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
	store, err := CreateRootStore(evalsRoot, 0o755)
	if err != nil {
		return "", "", fmt.Errorf("CaptureModelSpec: %w", err)
	}
	defer func() { _ = store.Close() }()
	dest := filepath.ToSlash(filepath.Join("models", id.StorageName(), "spec.yaml"))
	if err := store.WriteAtomic(dest, finalCanon, 0o644); err != nil {
		return "", "", fmt.Errorf("CaptureModelSpec: write: %w", err)
	}
	return spec.ID, hash, nil
}

func LoadModelSpec(evalsRoot, specID string) (*ModelSpec, error) {
	id, err := ParseIdentifier(IdentifierModel, specID)
	if err != nil {
		return nil, fmt.Errorf("LoadModelSpec: %w", err)
	}
	store, err := OpenRootStore(evalsRoot)
	if err != nil {
		return nil, fmt.Errorf("LoadModelSpec: %w", err)
	}
	defer func() { _ = store.Close() }()
	var raw []byte
	var used string
	for _, storageName := range id.CompatibilityStorageNames() {
		used = filepath.ToSlash(filepath.Join("models", storageName, "spec.yaml"))
		raw, err = store.ReadFile(used)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("LoadModelSpec: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("LoadModelSpec: %w", err)
	}
	var spec ModelSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("LoadModelSpec: parse: %w", err)
	}
	if spec.ID != id.String() {
		return nil, fmt.Errorf("LoadModelSpec: spec id %q does not match requested model id %q", spec.ID, id.String())
	}
	return &spec, nil
}

func ModelSpecHashEqual(a, b *ModelSpec) bool {
	return a != nil && b != nil && a.ContentHash != "" && a.ContentHash == b.ContentHash
}
