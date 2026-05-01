package evalsubstrate

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMiniHarness(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, data := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSnapshotHarness_ProducesStableContentHash(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skill")
	writeMiniHarness(t, dir, map[string][]byte{
		"SKILL.md":                     []byte("# Skill\n\nbody\n"),
		"references/iter-retrieval.md": []byte("# Reference\n"),
	})
	h1, _, err := SnapshotHarness(dir, "id-test", "ao-eval-snapshot-cli")
	if err != nil {
		t.Fatal(err)
	}
	h2, _, err := SnapshotHarness(dir, "id-test", "ao-eval-snapshot-cli")
	if err != nil {
		t.Fatal(err)
	}
	if h1.ContentHash != h2.ContentHash {
		t.Fatalf("hash unstable: %s vs %s", h1.ContentHash, h2.ContentHash)
	}
}

func TestSnapshotHarness_DetectsContentChange(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skill")
	writeMiniHarness(t, dir, map[string][]byte{
		"SKILL.md": []byte("v1\n"),
	})
	h1, _, err := SnapshotHarness(dir, "id", "x")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, _, err := SnapshotHarness(dir, "id", "x")
	if err != nil {
		t.Fatal(err)
	}
	if h1.ContentHash == h2.ContentHash {
		t.Fatalf("hash should differ after content change")
	}
}

func TestSnapshotHarness_CRLFNotChangeHash(t *testing.T) {
	root := t.TempDir()
	dir1 := filepath.Join(root, "lf")
	dir2 := filepath.Join(root, "crlf")
	writeMiniHarness(t, dir1, map[string][]byte{
		"config.yaml": []byte("foo: bar\nbaz: qux\n"),
	})
	writeMiniHarness(t, dir2, map[string][]byte{
		"config.yaml": []byte("foo: bar\r\nbaz: qux\r\n"),
	})
	h1, _, err := SnapshotHarness(dir1, "id", "x")
	if err != nil {
		t.Fatal(err)
	}
	h2, _, err := SnapshotHarness(dir2, "id", "x")
	if err != nil {
		t.Fatal(err)
	}
	if h1.ContentHash != h2.ContentHash {
		t.Fatalf("CRLF->LF should be hash-equivalent: %s vs %s", h1.ContentHash, h2.ContentHash)
	}
}

func TestVerifyHarnessLock_DetectsDrift(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skill")
	writeMiniHarness(t, dir, map[string][]byte{
		"SKILL.md": []byte("v1\n"),
	})
	_, lock, err := SnapshotHarness(dir, "id", "x")
	if err != nil {
		t.Fatal(err)
	}
	ok, got, err := VerifyHarnessLock(dir, lock)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected lock match: got=%s lock=%s", got, lock.ContentHash)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("v2-drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, _, err = VerifyHarnessLock(dir, lock)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("drift should fail verification (gate #8)")
	}
}

func TestWriteAndLoadHarnessLock_RoundTrips(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skill")
	writeMiniHarness(t, dir, map[string][]byte{
		"SKILL.md": []byte("v1\n"),
	})
	_, lock, err := SnapshotHarness(dir, "id", "x")
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteHarnessLock(dir, lock); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadHarnessLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ContentHash != lock.ContentHash {
		t.Fatalf("lock roundtrip mismatch: %s vs %s", loaded.ContentHash, lock.ContentHash)
	}
}
