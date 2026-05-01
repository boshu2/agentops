package evalsubstrate

import (
	"path/filepath"
	"testing"
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
	dest := ModelSpecPath(root, spec.ID)
	if dest != filepath.Join(root, "models", spec.ID, "spec.yaml") {
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
