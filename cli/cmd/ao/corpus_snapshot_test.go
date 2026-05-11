// practices: [corpus-durability, ai-assisted-dev]
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWriteSnapshotRoundTrip exercises the write+extract round-trip with a
// small synthetic corpus and asserts: file count, byte total, manifest sha,
// and that every original byte survives the tar.gz round-trip.
func TestWriteSnapshotRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src", ".agents")
	if err := os.MkdirAll(filepath.Join(src, "learnings"), 0o755); err != nil {
		t.Fatalf("mkdir src/learnings: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(src, "research"), 0o755); err != nil {
		t.Fatalf("mkdir src/research: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "learnings", "foo.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write foo.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "research", "bar.md"), []byte("world!"), 0o644); err != nil {
		t.Fatalf("write bar.md: %v", err)
	}

	tarPath := filepath.Join(tmp, "snap.tar.gz")
	count, total, sum, err := writeSnapshot(tarPath, src)
	if err != nil {
		t.Fatalf("writeSnapshot: %v", err)
	}
	if count != 2 {
		t.Errorf("file_count: got %d, want 2", count)
	}
	if total != int64(len("hello")+len("world!")) {
		t.Errorf("total_bytes: got %d, want %d", total, int64(len("hello")+len("world!")))
	}
	if len(sum) != 64 {
		t.Errorf("sha256 hex: got %d chars, want 64", len(sum))
	}

	dest := filepath.Join(tmp, "restored")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	restoredCount, restoredTotal, err := extractSnapshot(tarPath, filepath.Join(dest, ".agents"))
	if err != nil {
		t.Fatalf("extractSnapshot: %v", err)
	}
	if restoredCount != count {
		t.Errorf("restored file count: got %d, want %d", restoredCount, count)
	}
	if restoredTotal != total {
		t.Errorf("restored bytes: got %d, want %d", restoredTotal, total)
	}
	want := map[string]string{
		".agents/learnings/foo.md": "hello",
		".agents/research/bar.md":  "world!",
	}
	for rel, expected := range want {
		got, err := os.ReadFile(filepath.Join(dest, rel))
		if err != nil {
			t.Fatalf("read restored %s: %v", rel, err)
		}
		if string(got) != expected {
			t.Errorf("restored %s: got %q, want %q", rel, string(got), expected)
		}
	}
}

// TestExtractSnapshotRefusesPathTraversal verifies the extractor rejects
// tarball entries whose cleaned path escapes the destination root.
func TestExtractSnapshotRefusesPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	tarPath := filepath.Join(tmp, "evil.tar.gz")
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("create tar: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte("payload")
	hdr := &tar.Header{
		Name:     "../escaped.txt",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("write body: %v", err)
	}
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()

	dest := filepath.Join(tmp, "out", ".agents")
	_, _, err = extractSnapshot(tarPath, dest)
	if err == nil {
		t.Fatalf("expected path-traversal error, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected 'path traversal' in error; got %q", err.Error())
	}
}

// TestFindLatestSnapshotPicksNewest seeds a directory with three tarballs of
// staggered mtimes and asserts findLatestSnapshot returns the newest.
func TestFindLatestSnapshotPicksNewest(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "a-20260101T000000Z.tar.gz")
	middle := filepath.Join(dir, "a-20260201T000000Z.tar.gz")
	newer := filepath.Join(dir, "a-20260301T000000Z.tar.gz")
	for _, p := range []string{older, middle, newer} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	now := time.Now()
	_ = os.Chtimes(older, now.Add(-72*time.Hour), now.Add(-72*time.Hour))
	_ = os.Chtimes(middle, now.Add(-24*time.Hour), now.Add(-24*time.Hour))
	_ = os.Chtimes(newer, now, now)

	got, err := findLatestSnapshot(dir)
	if err != nil {
		t.Fatalf("findLatestSnapshot: %v", err)
	}
	if got != newer {
		t.Errorf("findLatestSnapshot: got %q, want %q", got, newer)
	}
}

// TestFindLatestSnapshotEmptyDir asserts an empty dir errors clearly rather
// than returning an empty path or silently succeeding.
func TestFindLatestSnapshotEmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, err := findLatestSnapshot(dir)
	if err == nil {
		t.Fatalf("expected error on empty dir")
	}
	if !strings.Contains(err.Error(), "no *.tar.gz snapshots") {
		t.Errorf("expected 'no *.tar.gz snapshots' in error; got %q", err.Error())
	}
}

// TestSnapshotManifestShape pins the JSON manifest structure so downstream
// consumers (restore tooling, freshness gate) don't drift.
func TestSnapshotManifestShape(t *testing.T) {
	m := snapshotManifest{
		SnapshotPath: "/tmp/foo.tar.gz",
		Repo:         "agentops",
		Source:       "/tmp/foo/.agents",
		FileCount:    3,
		TotalBytes:   123,
		SHA256:       "abc",
		CreatedAt:    time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC),
	}
	buf, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"snapshot_path", "repo", "source", "file_count", "total_bytes", "sha256", "created_at"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("manifest missing key %q", key)
		}
	}
}

// TestResolveSnapshotDirEnvOverride asserts the env var beats the home
// fallback, and that ~ in user-supplied paths is expanded.
func TestResolveSnapshotDirEnvOverride(t *testing.T) {
	t.Setenv(defaultSnapshotDirEnv, "/explicit/path")
	got, err := resolveSnapshotDir("")
	if err != nil {
		t.Fatalf("resolveSnapshotDir: %v", err)
	}
	if got != "/explicit/path" {
		t.Errorf("env override: got %q, want %q", got, "/explicit/path")
	}

	home, _ := os.UserHomeDir()
	t.Setenv(defaultSnapshotDirEnv, "")
	got, err = resolveSnapshotDir("~/snapshots")
	if err != nil {
		t.Fatalf("resolveSnapshotDir tilde: %v", err)
	}
	wantPrefix := filepath.Join(home, "snapshots")
	if got != wantPrefix {
		t.Errorf("tilde expansion: got %q, want %q", got, wantPrefix)
	}
}

// TestWriteSnapshotCreatesDeterministicHeader makes sure regular files survive
// with their byte contents and a reasonable size header (no zero-byte truncation).
func TestWriteSnapshotPreservesFileSize(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, ".agents")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	payload := []byte("the corpus must compound")
	if err := os.WriteFile(filepath.Join(src, "fact.md"), payload, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tarPath := filepath.Join(tmp, "snap.tar.gz")
	_, _, _, err := writeSnapshot(tarPath, src)
	if err != nil {
		t.Fatalf("writeSnapshot: %v", err)
	}
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	tr := tar.NewReader(gz)
	var found bool
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if strings.HasSuffix(hdr.Name, "fact.md") {
			found = true
			if hdr.Size != int64(len(payload)) {
				t.Errorf("fact.md size: got %d, want %d", hdr.Size, len(payload))
			}
		}
	}
	if !found {
		t.Fatal("fact.md not found in tarball")
	}
}
