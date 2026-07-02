//go:build flywheel

// practices: [wiki-knowledge-surface, resilience-patterns, ai-assisted-dev]
package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tarEntry is one crafted archive member. Body is only meaningful for TypeReg.
type tarEntry struct {
	name     string
	typeflag byte
	linkname string
	body     string
	mode     int64
}

// buildCraftedTarGz writes a tar.gz containing exactly the given entries and
// returns its path. It bypasses the production writer on purpose so the extractor
// can be exercised against hostile/unusual archives the writer would never emit.
func buildCraftedTarGz(t *testing.T, dir string, entries []tarEntry) string {
	t.Helper()
	tarPath := filepath.Join(dir, "crafted.tar.gz")
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("create crafted tar: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     mode,
			Typeflag: e.typeflag,
			Linkname: e.linkname,
		}
		if e.typeflag == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %q: %v", e.name, err)
		}
		if e.typeflag == tar.TypeReg && len(e.body) > 0 {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("write body %q: %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gz: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	return tarPath
}

// TestExtractSnapshotContainment drives the hardened extractor with crafted
// archives: absolute-path, traversal, symlink, hardlink, and unknown-typeflag
// entries must each be refused with an error naming the entry/flag, and NOTHING
// may be written outside the destination tree. A benign dir+reg archive must
// still extract cleanly. Table-driven; each case gets an isolated t.TempDir.
func TestExtractSnapshotContainment(t *testing.T) {
	cases := []struct {
		name        string
		entries     []tarEntry
		wantErrSub  []string // all must appear in the error (nil = expect success)
		wantFiles   map[string]string
		forbidPaths []string // absolute paths that must NOT exist after extraction
	}{
		{
			name: "absolute path entry refused",
			entries: []tarEntry{
				{name: "/etc/passwd", typeflag: tar.TypeReg, body: "root:x:0:0"},
			},
			wantErrSub: []string{"absolute-path", "/etc/passwd"},
		},
		{
			name: "traversal entry refused",
			entries: []tarEntry{
				{name: "../../escape.txt", typeflag: tar.TypeReg, body: "pwned"},
			},
			wantErrSub: []string{"path traversal", "escape.txt"},
		},
		{
			name: "symlink typeflag refused naming flag",
			entries: []tarEntry{
				{name: ".agents/link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"},
			},
			wantErrSub: []string{"unsupported tar entry", ".agents/link", "typeflag"},
		},
		{
			name: "hardlink typeflag refused naming flag",
			entries: []tarEntry{
				{name: ".agents/hard", typeflag: tar.TypeLink, linkname: ".agents/real"},
			},
			wantErrSub: []string{"unsupported tar entry", ".agents/hard", "typeflag"},
		},
		{
			name: "unknown typeflag refused",
			entries: []tarEntry{
				// TypeFifo is neither dir, reg, symlink nor hardlink.
				{name: ".agents/fifo", typeflag: tar.TypeFifo},
			},
			wantErrSub: []string{"unsupported tar entry", ".agents/fifo", "typeflag"},
		},
		{
			name: "benign dir and reg archive extracts",
			entries: []tarEntry{
				{name: ".agents", typeflag: tar.TypeDir, mode: 0o755},
				{name: ".agents/learnings", typeflag: tar.TypeDir, mode: 0o755},
				{name: ".agents/learnings/foo.md", typeflag: tar.TypeReg, body: "hello"},
				{name: ".agents/bar.md", typeflag: tar.TypeReg, body: "world!"},
			},
			wantErrSub: nil,
			wantFiles: map[string]string{
				".agents/learnings/foo.md": "hello",
				".agents/bar.md":           "world!",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			work := t.TempDir()
			tarPath := buildCraftedTarGz(t, work, tc.entries)
			dest := filepath.Join(work, "out", ".agents")

			_, _, err := extractSnapshot(tarPath, dest)

			if len(tc.wantErrSub) == 0 {
				if err != nil {
					t.Fatalf("expected clean extract, got error: %v", err)
				}
				for rel, want := range tc.wantFiles {
					got, rerr := os.ReadFile(filepath.Join(work, "out", rel))
					if rerr != nil {
						t.Fatalf("read restored %s: %v", rel, rerr)
					}
					if string(got) != want {
						t.Errorf("restored %s: got %q, want %q", rel, string(got), want)
					}
				}
				// Destination tree must contain ONLY the benign entries — no
				// stray files leaked outside .agents/.
				assertOnlyExpected(t, filepath.Join(work, "out"), tc.wantFiles)
				return
			}

			if err == nil {
				t.Fatalf("expected refusal error, got nil")
			}
			for _, sub := range tc.wantErrSub {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("error %q missing expected substring %q", err.Error(), sub)
				}
			}
			// Nothing must have been written outside the destination root.
			for _, forbidden := range tc.forbidPaths {
				if _, statErr := os.Lstat(forbidden); statErr == nil {
					t.Errorf("refused entry still wrote forbidden path %q", forbidden)
				}
			}
			// The absolute-path case in particular must not have created /etc/passwd
			// content anywhere reachable — assert the extraction root stayed empty of
			// unexpected regular files.
			assertNoRegularFilesOutside(t, filepath.Join(work, "out"))
		})
	}
}

// assertOnlyExpected walks root and asserts the set of regular files exactly
// matches the expected relative paths (keyed off the parent of root/.agents).
func assertOnlyExpected(t *testing.T, root string, want map[string]string) {
	t.Helper()
	got := map[string]bool{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				return rerr
			}
			got[rel] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	for rel := range want {
		if !got[rel] {
			t.Errorf("expected file %q missing from extraction tree", rel)
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			t.Errorf("unexpected file %q in extraction tree (want only %v)", rel, keysOf(want))
		}
	}
}

// assertNoRegularFilesOutside asserts a refused extraction left no regular files
// under root (a partial/leaked write would show up here).
func assertNoRegularFilesOutside(t *testing.T, root string) {
	t.Helper()
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			// root may not exist at all on a refusal — that's fine.
			return nil
		}
		if info.Mode().IsRegular() {
			t.Errorf("refused extraction leaked a regular file: %q", p)
		}
		return nil
	})
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestExtractSnapshotProductionRoundTripStaysGreen is the fixture-fidelity guard:
// it builds a snapshot through the REAL production writer (createCorpusSnapshot →
// writeSnapshot) from a realistic files+dirs corpus and re-extracts it through the
// hardened extractor, proving the strict default-case does not reject any typeflag
// the writer actually emits for genuine corpus content (TypeDir/TypeReg only).
func TestExtractSnapshotProductionRoundTripStaysGreen(t *testing.T) {
	work := t.TempDir()
	// A realistic .agents/ corpus shape: nested dirs + regular markdown files.
	src := filepath.Join(work, "repo", corpusSourceDir)
	if err := os.MkdirAll(filepath.Join(src, "learnings"), 0o755); err != nil {
		t.Fatalf("mkdir learnings: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(src, "research", "deep"), 0o755); err != nil {
		t.Fatalf("mkdir research/deep: %v", err)
	}
	files := map[string]string{
		"learnings/one.md":       "learning one",
		"research/two.md":        "research two",
		"research/deep/three.md": "deep three",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(src, rel), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	outDir := filepath.Join(work, "snapshots")
	manifest, _, err := createCorpusSnapshot(filepath.Join(work, "repo"), outDir)
	if err != nil {
		t.Fatalf("createCorpusSnapshot: %v", err)
	}
	if manifest.FileCount != len(files) {
		t.Fatalf("manifest file count: got %d, want %d", manifest.FileCount, len(files))
	}

	dest := filepath.Join(work, "restored", corpusSourceDir)
	count, _, err := extractSnapshot(manifest.SnapshotPath, dest)
	if err != nil {
		t.Fatalf("extractSnapshot of production snapshot must stay green, got: %v", err)
	}
	if count != len(files) {
		t.Errorf("restored file count: got %d, want %d", count, len(files))
	}
	for rel, want := range files {
		got, rerr := os.ReadFile(filepath.Join(work, "restored", corpusSourceDir, rel))
		if rerr != nil {
			t.Fatalf("read restored %s: %v", rel, rerr)
		}
		if string(got) != want {
			t.Errorf("restored %s: got %q, want %q", rel, string(got), want)
		}
	}
}
