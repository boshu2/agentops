package evalsubstrate

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCaptureModelSpec_StampsHashAndPersists(t *testing.T) {
	root := t.TempDir()
	spec := &ModelSpec{
		ID:        "ms:test-2026-05-01",
		Provider:  "local-mlx",
		ModelName: "mlx-community/Qwen-test",
		SamplingDefaults: map[string]interface{}{
			"temperature": 0.0,
			"top_p":       1.0,
		},
		RigID: "bo-mac-m5",
	}
	id, hash, err := CaptureModelSpec(root, spec)
	if err != nil {
		t.Fatal(err)
	}
	if id != spec.ID {
		t.Fatalf("id mismatch: %s vs %s", id, spec.ID)
	}
	if hash == "" || hash[:7] != "sha256:" {
		t.Fatalf("bad hash: %s", hash)
	}
	if spec.ContentHash != hash {
		t.Fatalf("spec.ContentHash not stamped: %s vs %s", spec.ContentHash, hash)
	}
	dest, err := ModelSpecPath(root, spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dest != filepath.Join(root, "models", "ms%3Atest-2026-05-01", "spec.yaml") {
		t.Fatalf("unexpected path: %s", dest)
	}
	loaded, err := LoadModelSpec(root, spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ContentHash != hash {
		t.Fatalf("loaded hash mismatch: %s vs %s", loaded.ContentHash, hash)
	}
	if loaded.Provider != "local-mlx" {
		t.Fatalf("provider lost: %s", loaded.Provider)
	}
}

func TestCaptureModelSpec_StableAcrossRecaptures(t *testing.T) {
	root := t.TempDir()
	spec := &ModelSpec{
		ID:               "ms:stable",
		Provider:         "local-mlx",
		ModelName:        "test",
		SamplingDefaults: map[string]interface{}{"temperature": 0.0},
		CapturedAt:       "2026-05-01T00:00:00Z",
	}
	_, h1, err := CaptureModelSpec(root, spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.ContentHash = ""
	_, h2, err := CaptureModelSpec(root, spec)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("re-capture hash mismatch: %s vs %s", h1, h2)
	}
}

func TestCaptureModelSpecRejectsHostileIDs(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"../escape", `..\escape`, "/absolute", `C:\escape`, "model-1", "ms:Upper"} {
		spec := &ModelSpec{ID: id}
		if _, _, err := CaptureModelSpec(root, spec); err == nil {
			t.Fatalf("CaptureModelSpec(%q) unexpectedly succeeded", id)
		}
	}
}

func TestCaptureModelSpecRejectsSymlinkParentEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "models")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	spec := &ModelSpec{ID: "ms:escape"}
	if _, _, err := CaptureModelSpec(root, spec); err == nil {
		t.Fatal("CaptureModelSpec through escaping symlink unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(outside, "ms%3Aescape", "spec.yaml")); !os.IsNotExist(err) {
		t.Fatalf("outside model spec was created: %v", err)
	}
}

func TestLoadModelSpecReadsOneBoundedLegacyColonPath(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, "models", "ms:legacy")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(ModelSpec{ID: "ms:legacy", ContentHash: "sha256:legacy"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "spec.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := LoadModelSpec(root, "ms:legacy")
	if err != nil {
		t.Fatalf("LoadModelSpec legacy path: %v", err)
	}
	if spec.ID != "ms:legacy" {
		t.Fatalf("loaded id = %q", spec.ID)
	}
	if _, err := os.Stat(filepath.Join(root, "models", "ms%3Alegacy")); !os.IsNotExist(err) {
		t.Fatalf("compatibility read unexpectedly wrote canonical storage: %v", err)
	}
}

func TestModelSpecHashEqual(t *testing.T) {
	a := &ModelSpec{ContentHash: "sha256:aa"}
	b := &ModelSpec{ContentHash: "sha256:aa"}
	c := &ModelSpec{ContentHash: "sha256:bb"}
	empty := &ModelSpec{}

	if !ModelSpecHashEqual(a, b) {
		t.Fatal("a == b")
	}
	if ModelSpecHashEqual(a, c) {
		t.Fatal("a != c")
	}
	if ModelSpecHashEqual(empty, empty) {
		t.Fatal("empty hashes should not compare equal")
	}
}
