package evalsubstrate

import (
	"os"
	"path/filepath"
	"strings"
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
	dest, err := ModelSpecPath(root, spec.ID)
	if err != nil {
		t.Fatalf("ModelSpecPath: %v", err)
	}
	wantDest := filepath.Join(root, "models", strings.ReplaceAll(spec.ID, ":", "%3A"), "spec.yaml")
	if dest != wantDest {
		t.Fatalf("unexpected path: %s (want %s)", dest, wantDest)
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

func TestCaptureModelSpec_RejectsTraversalIDWithoutWriting(t *testing.T) {
	root := t.TempDir()
	spec := &ModelSpec{ID: "../../escape", Provider: "local-mlx", ModelName: "test"}
	if _, _, err := CaptureModelSpec(root, spec); err == nil {
		t.Fatal("CaptureModelSpec accepted a traversal id; want rejection")
	}
	// The would-be escape target (root/../escape/spec.yaml) must not exist.
	escaped := filepath.Join(root, "..", "escape", "spec.yaml")
	if _, err := os.Stat(escaped); err == nil {
		t.Fatalf("traversal id escaped the eval root: %s exists", escaped)
	}
}

func TestLoadModelSpec_RejectsTraversalID(t *testing.T) {
	if _, err := LoadModelSpec(t.TempDir(), "../../../etc/passwd"); err == nil {
		t.Fatal("LoadModelSpec accepted a traversal id; want rejection")
	}
}

func TestModelSpecPath_IsCheckedAndEncodesColon(t *testing.T) {
	// The colon in a legitimate model-spec id is encoded to a path-safe token, so
	// no raw ':' (a Windows alternate-data-stream separator) reaches the filesystem.
	got, err := ModelSpecPath("/root", "ms:stable")
	if err != nil {
		t.Fatalf("ModelSpecPath(ms:stable) = %v, want success", err)
	}
	want := filepath.Join("/root", "models", "ms%3Astable", "spec.yaml")
	if got != want {
		t.Fatalf("ModelSpecPath encoding: got %q, want %q", got, want)
	}
	if strings.ContainsRune(got, ':') && filepath.VolumeName(got) == "" {
		t.Fatalf("encoded model-spec path still contains a raw ':': %q", got)
	}
	// The sink is the only constructor and it rejects unsafe ids: there is no
	// unchecked join to bypass. "ms%3Astable" must be rejected, not aliased:
	// accepting it would let a raw id collide with the encoded form of
	// "ms:stable" (the encoding must stay injective).
	for _, bad := range []string{"../../escape", "..", "a/b", "", "ms%3Astable", "50%"} {
		if _, err := ModelSpecPath("/root", bad); err == nil {
			t.Errorf("ModelSpecPath(%q) = nil error; the checked sink must reject it", bad)
		}
	}
}

func TestCaptureLoadModelSpec_ColonIDRoundTripsThroughEncoding(t *testing.T) {
	root := t.TempDir()
	spec := &ModelSpec{ID: "ms:stable", Provider: "local-mlx", ModelName: "test"}
	if _, _, err := CaptureModelSpec(root, spec); err != nil {
		t.Fatalf("CaptureModelSpec(ms:stable): %v", err)
	}
	// On-disk directory is the encoded form; the raw-colon path never exists.
	if _, err := os.Stat(filepath.Join(root, "models", "ms%3Astable", "spec.yaml")); err != nil {
		t.Fatalf("encoded spec file missing: %v", err)
	}
	if _, err := LoadModelSpec(root, "ms:stable"); err != nil {
		t.Fatalf("LoadModelSpec(ms:stable) did not round-trip through the encoder: %v", err)
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
