package wiki

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func srcNow() time.Time { return time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC) }

func newSrcWorkspace(t *testing.T) string {
	t.Helper()
	ws := filepath.Join(t.TempDir(), "ws")
	if _, err := Scaffold(ws, DefaultScaffoldConfig("test", "en")); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	return ws
}

// TestAddSources_CopiesAndRegisters: a supported file is copied into raw/ with a
// registry entry carrying its sha; an unsupported type is skipped.
func TestAddSources_CopiesAndRegisters(t *testing.T) {
	ws := newSrcWorkspace(t)
	srcDir := t.TempDir()
	md := filepath.Join(srcDir, "alpha.md")
	if err := os.WriteFile(md, []byte("# Alpha"), 0o600); err != nil {
		t.Fatal(err)
	}
	png := filepath.Join(srcDir, "image.png")
	if err := os.WriteFile(png, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := AddSources(ws, []string{md, png}, srcNow())
	if err != nil {
		t.Fatalf("AddSources: %v", err)
	}
	if len(res.Added) != 1 || res.Added[0].ID != "alpha" {
		t.Fatalf("Added = %+v, want 1 entry id=alpha", res.Added)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("Skipped = %v, want 1 (the .png)", res.Skipped)
	}
	if res.Added[0].SHA256 == "" {
		t.Error("entry missing sha256")
	}
	if _, err := os.Stat(filepath.Join(ws, "raw", "alpha.md")); err != nil {
		t.Errorf("raw copy missing: %v", err)
	}
	// Idempotent re-add updates, does not duplicate.
	if _, err := AddSources(ws, []string{md}, srcNow()); err != nil {
		t.Fatalf("re-add: %v", err)
	}
	if s, _ := RegisteredSources(ws); len(s) != 1 {
		t.Errorf("re-add duplicated registry: %d entries", len(s))
	}
}

// TestAddSources_RejectsSlugCollision: two raw files that slugify to the same id
// must not both register (they'd share one derived artifact); the collider is
// skipped+reported, keeping slug<->source 1:1 (cross-family REFUTE).
func TestAddSources_RejectsSlugCollision(t *testing.T) {
	ws := newSrcWorkspace(t)
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "alpha.md"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "alpha!.md"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := AddSources(ws, []string{srcDir}, srcNow())
	if err != nil {
		t.Fatalf("AddSources: %v", err)
	}
	if len(res.Added) != 1 {
		t.Fatalf("Added = %d, want 1 (slug collision skips the second)", len(res.Added))
	}
	if len(res.Skipped) == 0 {
		t.Error("collision was not reported as skipped")
	}
	if s, _ := RegisteredSources(ws); len(s) != 1 {
		t.Errorf("registry has %d entries, want 1 (slug-unique)", len(s))
	}
}

// TestAddSources_RawDirCollisionAfterKeepRaw: after remove --keep-raw orphans a
// raw file, adding a slug-collider is rejected by the raw/-directory check, so
// recompile can't clobber a shared artifact (cross-family REFUTE).
func TestAddSources_RawDirCollisionAfterKeepRaw(t *testing.T) {
	ws := newSrcWorkspace(t)
	srcDir := t.TempDir()
	alpha := filepath.Join(srcDir, "alpha.md")
	if err := os.WriteFile(alpha, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSources(ws, []string{alpha}, srcNow()); err != nil {
		t.Fatal(err)
	}
	// Drop the registry entry but keep the raw file (orphan in raw/).
	if _, err := RemoveSource(ws, "alpha", RemoveOptions{KeepRaw: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, "raw", "alpha.md")); err != nil {
		t.Fatalf("precondition: kept raw missing: %v", err)
	}
	// A different filename that also slugifies to "alpha" must be refused.
	collider := filepath.Join(srcDir, "alpha!.md")
	if err := os.WriteFile(collider, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := AddSources(ws, []string{collider}, srcNow())
	if err != nil {
		t.Fatalf("AddSources: %v", err)
	}
	if len(res.Added) != 0 {
		t.Errorf("added a slug-collider against an orphaned raw file: %+v", res.Added)
	}
	if len(res.Skipped) == 0 {
		t.Error("raw-dir collision not reported")
	}
	if _, err := os.Stat(filepath.Join(ws, "raw", "alpha!.md")); err == nil {
		t.Error("collider was copied into raw/ despite the collision")
	}
}

// TestRemoveSource_DryRun reports artifacts but deletes nothing and does not
// mutate the registry.
func TestRemoveSource_DryRun(t *testing.T) {
	ws := newSrcWorkspace(t)
	md := filepath.Join(t.TempDir(), "alpha.md")
	if err := os.WriteFile(md, []byte("# Alpha"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSources(ws, []string{md}, srcNow()); err != nil {
		t.Fatal(err)
	}
	res, err := RemoveSource(ws, "alpha", RemoveOptions{DryRun: true})
	if err != nil {
		t.Fatalf("RemoveSource dry-run: %v", err)
	}
	if res.Removed {
		t.Error("dry-run reported Removed=true")
	}
	if _, err := os.Stat(filepath.Join(ws, "raw", "alpha.md")); err != nil {
		t.Error("dry-run deleted the raw file")
	}
	if s, _ := RegisteredSources(ws); len(s) != 1 {
		t.Error("dry-run mutated the registry")
	}
}

// TestRemoveSource_ContainmentRejectsSymlinkArtifact is the destructive-safety
// core: a derived artifact that is a symlink (escape vector) is REFUSED by the
// containment guard, so remove can never delete through it outside the workspace.
func TestRemoveSource_ContainmentRejectsSymlinkArtifact(t *testing.T) {
	ws := newSrcWorkspace(t)
	md := filepath.Join(t.TempDir(), "alpha.md")
	if err := os.WriteFile(md, []byte("# Alpha"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSources(ws, []string{md}, srcNow()); err != nil {
		t.Fatal(err)
	}
	// Plant alpha's EXACT derived artifact "wiki/sources/alpha.md" as a symlink to
	// an OUTSIDE file. derivedMatches lists it (stem == "alpha"); the per-component
	// Lstat in scaffoldSafeAbs must refuse it rather than follow it.
	outside := filepath.Join(t.TempDir(), "victim.md")
	if err := os.WriteFile(outside, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}
	evil := filepath.Join(ws, "wiki", "sources", "alpha.md")
	if err := os.Symlink(outside, evil); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := RemoveSource(ws, "alpha", RemoveOptions{}); err == nil {
		t.Fatal("RemoveSource followed a symlink derived artifact (should refuse)")
	}
	// The outside victim must still exist.
	if _, err := os.Stat(outside); err != nil {
		t.Error("RemoveSource deleted an outside file through a symlink")
	}
	// All-or-nothing: a refused remove must NOT have deleted the raw file or
	// mutated the registry (cross-family REFUTE: partial deletion before refusal).
	if _, err := os.Stat(filepath.Join(ws, "raw", "alpha.md")); err != nil {
		t.Error("refused remove still deleted raw/alpha.md (not all-or-nothing)")
	}
	if s, _ := RegisteredSources(ws); len(s) != 1 {
		t.Errorf("refused remove mutated the registry: %d entries", len(s))
	}
}

// TestRemoveSource_DoesNotDeleteRegistry: a source whose id is "registry" must
// not cause its own removal to delete the registry control file.
func TestRemoveSource_DoesNotDeleteRegistry(t *testing.T) {
	ws := newSrcWorkspace(t)
	md := filepath.Join(t.TempDir(), "registry.md")
	if err := os.WriteFile(md, []byte("# Registry doc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSources(ws, []string{md}, srcNow()); err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveSource(ws, "registry", RemoveOptions{}); err != nil {
		t.Fatalf("RemoveSource(registry): %v", err)
	}
	// The registry control file must still exist (and be a valid, now-empty list).
	if _, err := os.Stat(filepath.Join(ws, SourceRegistryRelPath)); err != nil {
		t.Errorf("removing a source id'd 'registry' deleted the registry file: %v", err)
	}
	if s, err := RegisteredSources(ws); err != nil || len(s) != 0 {
		t.Errorf("registry not readable/empty after remove: %d, %v", len(s), err)
	}
}

// TestRemoveSource_RejectsCraftedRawName: a registry entry whose RawName is a
// non-basename (e.g. "../wiki/config.yaml") must be REFUSED before any delete —
// it cleans to an in-workspace path that passes containment but would delete the
// wrong control file (cross-family REFUTE).
func TestRemoveSource_RejectsCraftedRawName(t *testing.T) {
	ws := newSrcWorkspace(t)
	// Hand-craft a malicious registry entry pointing RawName at config.yaml.
	reg := &SourceRegistry{Sources: []SourceEntry{{
		ID: "evil", RawName: "../wiki/config.yaml", OriginalPath: "x", SHA256: "y", AddedAt: srcNow().Format("2006-01-02T15:04:05Z07:00"),
	}}}
	realRoot, err := scaffoldRealRoot(ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSourceRegistry(realRoot, reg); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(ws, "wiki", "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("precondition: config.yaml missing: %v", err)
	}
	if _, err := RemoveSource(ws, "evil", RemoveOptions{}); err == nil {
		t.Fatal("RemoveSource accepted a crafted non-basename RawName")
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Error("RemoveSource deleted wiki/config.yaml via a crafted RawName")
	}
}

// TestRemoveSource_CraftedIDDoesNotDeleteOthers: a crafted entry {RawName:evil.md,
// ID:beta} must delete ONLY evil's artifacts (derived from RawName), never a
// separate legitimate "beta" source's artifacts (cross-family REFUTE: trusted ID).
func TestRemoveSource_CraftedIDDoesNotDeleteOthers(t *testing.T) {
	ws := newSrcWorkspace(t)
	realRoot, err := scaffoldRealRoot(ws)
	if err != nil {
		t.Fatal(err)
	}
	// A real raw/evil.md + a legitimate beta artifact that must survive.
	if err := os.WriteFile(filepath.Join(ws, "raw", "evil.md"), []byte("e"), 0o600); err != nil {
		t.Fatal(err)
	}
	betaArtifact := filepath.Join(ws, "wiki", "sources", "beta.md")
	if err := os.WriteFile(betaArtifact, []byte("legit beta"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Crafted registry: ID claims "beta" but RawName is evil.md.
	reg := &SourceRegistry{Sources: []SourceEntry{{ID: "beta", RawName: "evil.md", OriginalPath: "x", SHA256: "y", AddedAt: srcNow().Format("2006-01-02T15:04:05Z07:00")}}}
	if err := writeSourceRegistry(realRoot, reg); err != nil {
		t.Fatal(err)
	}
	res, err := RemoveSource(ws, "beta", RemoveOptions{})
	if err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}
	if res.DocID != "evil" {
		t.Errorf("DocID = %q, want evil (derived from RawName, not trusted ID)", res.DocID)
	}
	// The legitimate beta artifact must survive.
	if _, err := os.Stat(betaArtifact); err != nil {
		t.Error("crafted ID deleted a different source's artifact (wiki/sources/beta.md)")
	}
	// evil's own raw file should be gone.
	if _, err := os.Stat(filepath.Join(ws, "raw", "evil.md")); err == nil {
		t.Error("remove did not delete the entry's actual raw file")
	}
}

// TestRemoveSource_NoSiblingOvermatch: removing "alpha" must NOT delete a sibling
// source "alpha-beta"'s artifact (cross-family REFUTE: prefix over-match).
func TestRemoveSource_NoSiblingOvermatch(t *testing.T) {
	ws := newSrcWorkspace(t)
	src := t.TempDir()
	for _, n := range []string{"alpha.md", "alpha-beta.md"} {
		if err := os.WriteFile(filepath.Join(src, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := AddSources(ws, []string{src}, srcNow()); err != nil {
		t.Fatal(err)
	}
	// Simulate ingest artifacts for both sources.
	sourcesDir := filepath.Join(ws, "wiki", "sources")
	for _, n := range []string{"alpha.md", "alpha-beta.md"} {
		if err := os.WriteFile(filepath.Join(sourcesDir, n), []byte("artifact"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := RemoveSource(ws, "alpha", RemoveOptions{}); err != nil {
		t.Fatalf("RemoveSource(alpha): %v", err)
	}
	// alpha's own artifact gone; the sibling alpha-beta's artifact survives.
	if _, err := os.Stat(filepath.Join(sourcesDir, "alpha.md")); err == nil {
		t.Error("alpha.md artifact not removed")
	}
	if _, err := os.Stat(filepath.Join(sourcesDir, "alpha-beta.md")); err != nil {
		t.Error("removing alpha over-deleted sibling alpha-beta.md")
	}
	// alpha-beta still registered.
	if s, _ := RegisteredSources(ws); len(s) != 1 || s[0].ID != "alpha-beta" {
		t.Errorf("sibling registry entry not preserved: %+v", s)
	}
}

// TestRemoveSource_KeepRaw leaves the raw file while dropping the registry entry.
func TestRemoveSource_KeepRaw(t *testing.T) {
	ws := newSrcWorkspace(t)
	md := filepath.Join(t.TempDir(), "alpha.md")
	if err := os.WriteFile(md, []byte("# Alpha"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSources(ws, []string{md}, srcNow()); err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveSource(ws, "alpha", RemoveOptions{KeepRaw: true}); err != nil {
		t.Fatalf("RemoveSource keep-raw: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, "raw", "alpha.md")); err != nil {
		t.Error("--keep-raw deleted the raw file")
	}
	if s, _ := RegisteredSources(ws); len(s) != 0 {
		t.Error("registry entry not dropped")
	}
}
