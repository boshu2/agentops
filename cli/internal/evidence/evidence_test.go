package evidence

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

func write(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := Canonical(v)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, b)
}
func draft() map[string]any {
	return map[string]any{
		"acceptance_digest": strings.Repeat("a", 64), "subject_manifest_digest": strings.Repeat("b", 64),
		"verdict": "PASS", "criteria": []any{map[string]any{"id": "c1", "result": "PASS", "evidence_refs": []string{"test:receipt"}}},
		"findings": []any{}, "evidence_refs": []string{"test:receipt"}, "checked": []string{"value"}, "not_checked": []string{}, "validated_at": "2026-07-14T00:00:00Z",
	}
}
func fixture(t *testing.T) (StoreOptions, *Manifest) {
	t.Helper()
	root := t.TempDir()
	out := t.TempDir()
	write(t, filepath.Join(root, "value"), []byte("subject bytes\n"))
	m, err := BuildManifest(root, []string{"value"}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(out, "manifest.json"), m)
	writeJSON(t, filepath.Join(out, "draft.json"), draft())
	write(t, filepath.Join(out, "intent"), []byte("independent source-support acceptance\n"))
	return StoreOptions{Root: root, EvidenceRoot: out, SubjectManifest: filepath.Join(out, "manifest.json"), Draft: filepath.Join(out, "draft.json"), IntentSource: filepath.Join(out, "intent"), Facts: RuntimeFacts{"author", "judge", "runtime", "runtime-1", "PASS"}}, m
}
func TestStoreRoundTripAndIndependentIntents(t *testing.T) {
	o, m := fixture(t)
	first, err := StoreVerdict(o)
	if err != nil {
		t.Fatal(err)
	}
	if first.Verdict != "PASS" || first.Idempotent {
		t.Fatalf("%+v", first)
	}
	if err = VerifySubject(o.Root, o.SubjectManifest, "", first.Path, o.IntentSource); err != nil {
		t.Fatal(err)
	}
	again, err := StoreVerdict(o)
	if err != nil || !again.Idempotent || !again.IntentSnapshotIdempotent || first.Path != again.Path {
		t.Fatalf("idempotence: %+v %v", again, err)
	}
	b, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err = verdictcheck.VerifyArtifact(b, first.ArtifactDigest); err != nil {
		t.Fatal(err)
	}
	raw, err := verdictcheck.DecodeObject(b)
	if err != nil {
		t.Fatal(err)
	}
	if raw["subject_manifest_digest"] != m.Digest {
		t.Fatal("wrong subject")
	}
	other := filepath.Join(o.EvidenceRoot, "disclosure-intent")
	write(t, other, []byte("independent destination-disclosure policy\n"))
	if err = VerifySubject(o.Root, o.SubjectManifest, "", first.Path, other); err == nil {
		t.Fatal("support PASS admitted disclosure intent")
	}
	o.IntentSource = other
	second, err := StoreVerdict(o)
	if err != nil {
		t.Fatal(err)
	}
	if second.AcceptanceDigest == first.AcceptanceDigest {
		t.Fatal("intents collapsed")
	}
	if err = VerifySubject(o.Root, o.SubjectManifest, "", second.Path, other); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(o.Root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "value" {
		t.Fatalf("workspace mutated: %v %v", entries, err)
	}
}
func TestStoreIdentityScopeAndEvidenceFloors(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*StoreOptions, map[string]any)
		want string
	}{
		{"collision", func(o *StoreOptions, d map[string]any) { o.Facts.ValidatorContextID = o.Facts.AuthorContextID }, "NOT_PROVEN"},
		{"missing author", func(o *StoreOptions, d map[string]any) { o.Facts.AuthorContextID = "" }, "NOT_PROVEN"},
		{"missing scope", func(o *StoreOptions, d map[string]any) { o.Facts.ScopeResult = "" }, "NOT_PROVEN"},
		{"scope fail", func(o *StoreOptions, d map[string]any) { o.Facts.ScopeResult = "FAIL" }, "FAIL"},
		{"scope unproven", func(o *StoreOptions, d map[string]any) { o.Facts.ScopeResult = "NOT_PROVEN" }, "NOT_PROVEN"},
		{"missing freshness", func(o *StoreOptions, d map[string]any) { o.Facts.FreshnessAttesterID = "" }, "NOT_PROVEN"},
		{"not checked", func(o *StoreOptions, d map[string]any) { d["not_checked"] = []string{"unverified acceptance"} }, "NOT_PROVEN"},
		{"empty evidence", func(o *StoreOptions, d map[string]any) { d["evidence_refs"] = []string{} }, "NOT_PROVEN"},
		{"empty checked", func(o *StoreOptions, d map[string]any) { d["checked"] = []string{} }, "NOT_PROVEN"},
		{"failed criterion", func(o *StoreOptions, d map[string]any) { d["criteria"].([]any)[0].(map[string]any)["result"] = "FAIL" }, "NOT_PROVEN"},
		{"honest fail", func(o *StoreOptions, d map[string]any) { d["verdict"] = "FAIL" }, "FAIL"},
		{"honest null", func(o *StoreOptions, d map[string]any) { d["verdict"] = "NOT_PROVEN" }, "NOT_PROVEN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, _ := fixture(t)
			d := draft()
			tc.edit(&o, d)
			writeJSON(t, o.Draft, d)
			r, err := StoreVerdict(o)
			if err != nil {
				t.Fatal(err)
			}
			if r.Verdict != tc.want {
				t.Fatalf("got %s want %s", r.Verdict, tc.want)
			}
			if err = VerifySubject(o.Root, o.SubjectManifest, "", r.Path, o.IntentSource); err == nil {
				t.Fatal("admitted non-PASS")
			}
		})
	}
}
func TestRejectBeforeAnyStorage(t *testing.T) {
	for _, name := range []string{"unknown", "nested unknown", "duplicate", "missing", "null list", "wrong subject", "permission", "symlink", "empty manifest", "missing root", "Git root", "nested Git output", "symlink output"} {
		t.Run(name, func(t *testing.T) {
			o, _ := fixture(t)
			dest := t.TempDir()
			o.EvidenceRoot = dest
			d := draft()
			switch name {
			case "unknown":
				d["next_action"] = "repair"
				writeJSON(t, o.Draft, d)
			case "nested unknown":
				d["criteria"].([]any)[0].(map[string]any)["confidence"] = "high"
				writeJSON(t, o.Draft, d)
			case "duplicate":
				b, _ := os.ReadFile(o.Draft)
				write(t, o.Draft, append([]byte(`{"verdict":"FAIL",`), b[1:]...))
			case "missing":
				delete(d, "checked")
				writeJSON(t, o.Draft, d)
			case "null list":
				d["findings"] = nil
				writeJSON(t, o.Draft, d)
			case "wrong subject":
				write(t, filepath.Join(o.Root, "value"), []byte("changed"))
			case "permission":
				if err := os.Chmod(filepath.Join(o.Root, "value"), 0700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(filepath.Join(o.Root, "value")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(o.IntentSource, filepath.Join(o.Root, "value")); err != nil {
					t.Fatal(err)
				}
			case "empty manifest":
				m, err := BuildManifest(o.Root, []string{"missing"}, nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				writeJSON(t, o.SubjectManifest, m)
			case "missing root":
				o.EvidenceRoot = filepath.Join(dest, "missing")
			case "Git root":
				if err := os.Mkdir(filepath.Join(dest, ".git"), 0700); err != nil {
					t.Fatal(err)
				}
			case "nested Git output":
				if err := os.MkdirAll(filepath.Join(dest, "verdicts", ".git"), 0700); err != nil {
					t.Fatal(err)
				}
			case "symlink output":
				if err := os.Symlink(t.TempDir(), filepath.Join(dest, "verdicts")); err != nil {
					t.Fatal(err)
				}
			}
			before := tree(t, dest)
			if _, err := StoreVerdict(o); err == nil {
				t.Fatal("invalid input stored")
			}
			if !bytes.Equal(before, tree(t, dest)) {
				t.Fatal("wrote before rejection")
			}
		})
	}
}
func tree(t *testing.T, root string) []byte {
	t.Helper()
	records := map[string]any{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(name)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		record := map[string]any{"mode": info.Mode().String()}
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			record["digest"] = Hash(b)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(name)
			if err != nil {
				return err
			}
			record["target"] = target
		}
		records[relative] = record
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Canonical(records)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStorageIntegrityCollision(t *testing.T) {
	o, _ := fixture(t)
	first, err := StoreVerdict(o)
	if err != nil {
		t.Fatal(err)
	}
	write(t, first.Path, []byte("corrupt\n"))
	second, err := StoreVerdict(o)
	if err != nil {
		t.Fatal(err)
	}
	if second.Verdict != "NOT_PROVEN" || second.Path == first.Path {
		t.Fatal("collision did not preserve integrity failure")
	}
	preserved, err := os.ReadFile(first.Path)
	if err != nil || string(preserved) != "corrupt\n" {
		t.Fatal("corrupt evidence overwritten")
	}
	if err = VerifyStored(second.Path); err != nil {
		t.Fatal(err)
	}
	again, err := StoreVerdict(o)
	if err != nil || !again.Idempotent || again.Path != second.Path {
		t.Fatalf("collision recovery not idempotent: %+v %v", again, err)
	}
}
func TestManifestSymlinkDeletionAndMetadata(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "target"), []byte("x"))
	if err := os.Symlink("target", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	base, err := BuildManifest(root, []string{"."}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "target")); err != nil {
		t.Fatal(err)
	}
	current, err := BuildManifest(root, []string{"."}, nil, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	if current.Entries[0].Kind != "symlink" || current.Entries[1].Kind != "deletion" {
		t.Fatalf("%+v", current)
	}
	if err = VerifyManifest(root, current, base); err != nil {
		t.Fatal(err)
	}
	if err = VerifyManifest(root, current, nil); err == nil {
		t.Fatal("missing base admitted")
	}
	commit := "descriptive"
	withMeta, err := BuildManifest(root, []string{"."}, nil, base, map[string]*string{"commit": &commit})
	if err != nil || current.Digest != withMeta.Digest {
		t.Fatal("metadata changes identity")
	}
	if err = os.Remove(filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink("other", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err = VerifyManifest(root, current, base); err == nil {
		t.Fatal("symlink change admitted")
	}
	if err = os.Symlink(t.TempDir(), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err = BuildManifest(root, []string{"alias/file"}, nil, nil, nil); err == nil {
		t.Fatal("followed intermediate symlink")
	}
}

// The same shared corpus covers the strict Go read path and writer seal path.
// Binding runtime facts is tested separately: it deliberately replaces identity
// fields and downgrades invalid PASS claims instead of certifying supplied bytes.
func TestGoldenCorpusWriterReader(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "tests", "fixtures", "verdict-contract", "cases")
	cases, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 10 {
		t.Fatal("missing corpus")
	}
	for _, entry := range cases {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var c struct {
			Name, Expected, FilenameDigest, Raw string
			Artifact                            json.RawMessage
		}
		var wire map[string]json.RawMessage
		if err = json.Unmarshal(b, &wire); err != nil {
			t.Fatal(err)
		}
		json.Unmarshal(wire["name"], &c.Name)
		json.Unmarshal(wire["expected"], &c.Expected)
		json.Unmarshal(wire["filename_digest"], &c.FilenameDigest)
		json.Unmarshal(wire["raw"], &c.Raw)
		c.Artifact = wire["artifact"]
		t.Run(c.Name, func(t *testing.T) {
			payload := c.Artifact
			if c.Raw != "" {
				payload = []byte(c.Raw)
			}
			readErr := verdictcheck.VerifyArtifact(payload, c.FilenameDigest)
			if (readErr == nil) != (c.Expected == "valid") {
				t.Fatalf("reader: %v", readErr)
			}
			if c.Expected == "valid" {
				raw, err := verdictcheck.DecodeObject(payload)
				if err != nil {
					t.Fatal(err)
				}
				sealed, err := sealVerdict(raw)
				if err != nil {
					t.Fatal(err)
				}
				if sealed["artifact_digest"] != c.FilenameDigest {
					t.Fatal("writer canonical identity fork")
				}
				payload, err := Canonical(sealed)
				if err != nil {
					t.Fatal(err)
				}
				stored, err := storeBytes(t.TempDir(), "verdicts/sha256/"+c.FilenameDigest+".json", append(payload, '\n'))
				if err != nil {
					t.Fatal(err)
				}
				if err = VerifyStored(stored.Path); err != nil {
					t.Fatal(err)
				}

			}
		})
	}
}

func TestSnapshotConcurrentPublication(t *testing.T) {
	root := t.TempDir()
	type result struct {
		snapshot *SnapshotResult
		err      error
	}
	results := make(chan result, 16)
	for i := 0; i < 16; i++ {
		go func() {
			snapshot, err := SnapshotIntent(root, []byte("same immutable bytes"))
			results <- result{snapshot, err}
		}()
	}
	created := 0
	for i := 0; i < 16; i++ {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		if !r.snapshot.Idempotent {
			created++
		}
		payload, err := os.ReadFile(r.snapshot.IntentRef)
		if err != nil || string(payload) != "same immutable bytes" {
			t.Fatalf("partial read: %q %v", payload, err)
		}
	}
	if created != 1 {
		t.Fatalf("publications=%d, want 1", created)
	}
	entries, err := os.ReadDir(filepath.Join(root, "intents", "sha256"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("left temporary files: %v %v", entries, err)
	}
	info, err := entries[0].Info()
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("snapshot permissions: %v %v", info, err)
	}
}
