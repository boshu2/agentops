package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManifestLiteralDeclaredRoots(t *testing.T) {
	for _, tc := range []struct {
		name, declared, file string
		wantDeletions        int
	}{
		{"brackets", "a[1].md", "a[1].md", 0},
		{"star", "report*.md", "report*.md", 1},
		{"question", "what?.md", "what?.md", 1},
		{"nested", "dir[1]/sub?*", "dir[1]/sub?*/value.md", 0},
		{"dot", ".", "nested/value.md", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, tc.file)
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("literal subject\n"), 0600); err != nil {
				t.Fatal(err)
			}
			base, err := BuildManifest(root, []string{tc.declared}, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(base.Entries) != 1 || base.Entries[0].Path != tc.file {
				t.Fatalf("literal file missing from manifest: %+v", base.Entries)
			}
			if err := VerifyManifest(root, base, nil); err != nil {
				t.Errorf("unchanged literal subject rejected: %v", err)
			}
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
			deleted, err := BuildManifest(root, []string{tc.declared}, nil, base, nil)
			if err != nil {
				t.Fatalf("build deletion manifest: %v", err)
			}
			// Preserve the Python reference's historical base-deletion identity;
			// literal live-root membership does not redefine that rule.
			if len(deleted.Entries) != tc.wantDeletions {
				t.Fatalf("reference-compatible deletion count: got %+v, want %d", deleted.Entries, tc.wantDeletions)
			}
			for _, entry := range deleted.Entries {
				if entry.Path != tc.file || entry.Kind != "deletion" {
					t.Fatalf("unexpected deletion entry: %+v", entry)
				}
			}
			if err := VerifyManifest(root, deleted, base); err != nil {
				t.Errorf("literal deletion manifest rejected: %v", err)
			}
		})
	}
}

func TestManifestLiteralDeclaredRootRejectsOutsideEntries(t *testing.T) {
	for _, tc := range []struct {
		name, declared, entry string
	}{
		{"bracket expansion", "a[1].md", "a1.md"},
		{"star expansion", "report*.md", "report-other.md"},
		{"question expansion", "what?.md", "whatX.md"},
		{"nested expansion", "dir[1]/sub?*", "dir1/subXYZ/value.md"},
		{"sibling prefix", "dir", "directory/value.md"},
		{"outside", "dir", "other/value.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{
				SchemaVersion: "subject-manifest.v1",
				DeclaredRoots: []string{tc.declared}, Exclusions: []string{},
				Entries: []Entry{{Path: tc.entry, Kind: "file", Digest: strings.Repeat("a", 64)}},
			}
			var err error
			m.Digest, err = manifestDigest(m)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := object(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseManifest(raw); err == nil {
				t.Fatalf("entry %q admitted under literal root %q", tc.entry, tc.declared)
			}
		})
	}
}

func TestManifestLiteralRootsPreserveWildcardExclusions(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dir[1]"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"keep.md", "skip.tmp"} {
		if err := os.WriteFile(filepath.Join(root, "dir[1]", name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	base, err := BuildManifest(root, []string{"."}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildManifest(root, []string{"dir[1]"}, []string{"*.tmp"}, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 1 || m.Entries[0].Path != "dir[1]/keep.md" {
		t.Fatalf("wildcard exclusion changed: %+v", m.Entries)
	}
	if err := VerifyManifest(root, m, base); err != nil {
		t.Fatal(err)
	}
}

// This uses the retained, unmodified Python reference as the compatibility
// oracle, including its historical fnmatch rule for base-deletion membership.
func TestManifestPythonLiteralRootParity(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable; cross-language manifest parity not checked")
	}
	reference := filepath.Join("..", "..", "..", "skills", "validate", "tests", "validate.py")
	if _, err := os.Stat(reference); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, declared, baseDeclared string
		files, remove, exclusions    []string
	}{
		{"bracketed file", "a[1].md", "", []string{"a[1].md"}, []string{"a[1].md"}, nil},
		{"bracketed directory", "dir[1]", "", []string{"dir[1]/keep.md", "dir[1]/remove.md"}, []string{"dir[1]/remove.md"}, nil},
		{"broad base narrowed to literal root", "a[1].md", ".", []string{"a1.md", "a[1].md"}, []string{"a1.md"}, nil},
		{"ordinary file", "plain.md", "", []string{"plain.md"}, []string{"plain.md"}, nil},
		{"excluded base deletion", ".", "", []string{"keep.md", "remove.md", "skip.tmp"}, []string{"remove.md", "skip.tmp"}, []string{"*.tmp"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, outputs := t.TempDir(), t.TempDir()
			for _, name := range tc.files {
				file := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte("synthetic parity subject\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			runPython := func(args ...string) []byte {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, python, append([]string{"-B", reference}, args...)...)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("Python reference %v: %v\n%s", args, err, out)
				}
				return out
			}
			compare := func(phase string, exclusions []string, base *Manifest, basePath string) (*Manifest, string) {
				t.Helper()
				declared := tc.declared
				if phase == "base" && tc.baseDeclared != "" {
					declared = tc.baseDeclared
				}
				got, err := BuildManifest(root, []string{declared}, exclusions, base, nil)
				if err != nil {
					t.Fatal(err)
				}
				args := []string{"manifest", "--root", root, "--include", declared}
				for _, exclusion := range exclusions {
					args = append(args, "--exclude", exclusion)
				}
				if basePath != "" {
					args = append(args, "--base-manifest", basePath)
				}
				pyBytes := runPython(args...)
				pyPath := filepath.Join(outputs, phase+"-python.json")
				if err := os.WriteFile(pyPath, pyBytes, 0600); err != nil {
					t.Fatal(err)
				}
				pyManifest, err := LoadManifest(pyPath)
				if err != nil {
					t.Fatalf("load Python %s manifest: %v", phase, err)
				}
				goBytes, err := Canonical(got)
				if err != nil {
					t.Fatal(err)
				}
				pyCanonical, err := Canonical(pyManifest)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(goBytes, pyCanonical) || got.Digest != pyManifest.Digest {
					t.Errorf("%s canonical manifest fork\nGo: %s\nPython: %s", phase, goBytes, pyCanonical)
				}
				if err := VerifyManifest(root, pyManifest, base); err != nil {
					t.Errorf("Go rejected Python %s manifest: %v", phase, err)
				}
				goPath := filepath.Join(outputs, phase+"-go.json")
				if err := os.WriteFile(goPath, goBytes, 0600); err != nil {
					t.Fatal(err)
				}
				verifyArgs := []string{"verify-manifest", "--root", root, "--manifest", goPath}
				if basePath != "" {
					verifyArgs = append(verifyArgs, "--base-manifest", basePath)
				}
				var verified struct {
					Result string `json:"result"`
				}
				if err := json.Unmarshal(runPython(verifyArgs...), &verified); err != nil {
					t.Fatal(err)
				}
				if verified.Result != "PASS" {
					t.Fatalf("Python rejected Go %s manifest: %s", phase, verified.Result)
				}
				return pyManifest, pyPath
			}
			base, basePath := compare("base", nil, nil, "")
			for _, name := range tc.remove {
				if err := os.Remove(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			}
			compare("deleted", tc.exclusions, base, basePath)
		})
	}
}

// Structural deletion membership follows the legacy reference, but the exact
// base and recomputation must still establish every claimed deletion.
func TestManifestLegacyDeletionRequiresExactBase(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a[1].md"), []byte("live subject\n"), 0600); err != nil {
		t.Fatal(err)
	}
	base, err := BuildManifest(root, []string{"."}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	current, err := BuildManifest(root, []string{"a[1].md"}, nil, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifest(root, current, base); err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifest(root, current, nil); err == nil {
		t.Fatal("missing base accepted")
	}
	wrongBase, err := BuildManifest(root, []string{"a[1].md"}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if wrongBase.Digest == base.Digest {
		t.Fatal("fixture did not create distinct base identities")
	}
	if err := VerifyManifest(root, current, wrongBase); err == nil {
		t.Fatal("wrong base accepted")
	}

	forged := *current
	forged.Entries = append(append([]Entry{}, current.Entries...), Entry{Path: "a1.md", Kind: "deletion"})
	forged.Digest, err = manifestDigest(&forged)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := object(&forged)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(raw); err != nil {
		t.Fatalf("legacy deletion membership rejected structurally: %v", err)
	}
	if err := VerifyManifest(root, &forged, base); err == nil {
		t.Fatal("forged deletion absent from exact base accepted")
	}
	forged.BaseDigest = ""
	forged.Digest, err = manifestDigest(&forged)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifest(root, &forged, nil); err == nil {
		t.Fatal("unbound forged deletion accepted")
	}
}

func TestManifestLegacyDeletionDoesNotWidenLiveScope(t *testing.T) {
	for _, kind := range []string{"file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			m := &Manifest{SchemaVersion: "subject-manifest.v1", DeclaredRoots: []string{"a[1].md"}, Exclusions: []string{}, Entries: []Entry{{Path: "a1.md", Kind: kind, Digest: strings.Repeat("a", 64)}}}
			var err error
			m.Digest, err = manifestDigest(m)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := object(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseManifest(raw); err == nil {
				t.Fatalf("glob-matched live %s outside literal root accepted", kind)
			}
		})
	}
	for _, name := range []string{"skip.tmp", "../outside.md"} {
		t.Run(name, func(t *testing.T) {
			m := &Manifest{SchemaVersion: "subject-manifest.v1", DeclaredRoots: []string{"."}, Exclusions: []string{"*.tmp"}, Entries: []Entry{{Path: name, Kind: "deletion"}}}
			var err error
			m.Digest, err = manifestDigest(m)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := object(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseManifest(raw); err == nil {
				t.Fatalf("excluded or escaping deletion accepted: %s", name)
			}
		})
	}
	if _, err := BuildManifest(t.TempDir(), []string{"../outside.md"}, nil, nil, nil); err == nil {
		t.Fatal("escaping traversal accepted")
	}
}
