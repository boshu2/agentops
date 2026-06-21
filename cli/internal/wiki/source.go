package wiki

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// OpenKB source lifecycle (age-port-openkb-into-agentops-go-5qw.2): add/remove/
// recompile over the OpenKB workspace's raw/ sources + a registry. Every copy
// and delete is anchored at the symlink-resolved workspace root and routed
// through scaffoldSafeAbs, so a source operation can NEVER read or write outside
// the workspace or into .agents/.ao (the same boundary the scaffold enforces).

// SourceRegistryRelPath is the registry location relative to the workspace root.
const SourceRegistryRelPath = "wiki/sources/registry.json"

// supportedSourceExts are the text source types this slice handles. It is the
// SINGLE source of truth shared by `add` (what it accepts) and RawCandidates
// (what recompile --dry-run reports), and it MUST match what the real ingest
// (llmwiki.isIngestableExt) actually processes — .md/.txt — so add → dry-run →
// real recompile are consistent (cross-family REFUTE: `.markdown` was accepted +
// previewed but skipped by the real ingest). PDF/URL conversion is an adapter
// follow-up (bead risk note).
var supportedSourceExts = map[string]bool{".md": true, ".txt": true}

// SourceEntry is one registered raw source.
type SourceEntry struct {
	ID           string `json:"id"`            // slug derived from the source filename
	RawName      string `json:"raw_name"`      // basename under raw/
	OriginalPath string `json:"original_path"` // where it was added from
	SHA256       string `json:"sha256"`
	AddedAt      string `json:"added_at"`
}

// SourceRegistry is the persisted set of registered sources.
type SourceRegistry struct {
	Sources []SourceEntry `json:"sources"`
}

// safeRegistryName rejects a registry name that is not a bare filename — a
// non-basename (separators, "..", absolute, "."/empty) loaded from on-disk
// registry data could redirect a destructive delete to the wrong in-workspace
// file even while passing containment (cross-family REFUTE, age-port-openkb 5qw.2).
func safeRegistryName(name string) error {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) || strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator) {
		return fmt.Errorf("registry entry has an unsafe name %q (must be a bare filename)", name)
	}
	return nil
}

// find returns the index of the entry whose ID or RawName matches doc, or -1.
func (r *SourceRegistry) find(doc string) int {
	for i, e := range r.Sources {
		if e.ID == doc || e.RawName == doc {
			return i
		}
	}
	return -1
}

// LoadSourceRegistry reads the workspace registry (empty when absent).
func LoadSourceRegistry(workspace string) (*SourceRegistry, error) {
	realRoot, err := scaffoldRealRoot(workspace)
	if err != nil {
		return nil, err
	}
	path, err := scaffoldSafeAbs(realRoot, SourceRegistryRelPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SourceRegistry{}, nil
		}
		return nil, fmt.Errorf("read source registry: %w", err)
	}
	var reg SourceRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse source registry %s: %w", path, err)
	}
	return &reg, nil
}

// writeSourceRegistry atomically writes reg (sorted by ID) to the workspace.
func writeSourceRegistry(realRoot string, reg *SourceRegistry) error {
	sort.Slice(reg.Sources, func(i, j int) bool { return reg.Sources[i].ID < reg.Sources[j].ID })
	path, err := scaffoldSafeAbs(realRoot, SourceRegistryRelPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("registry dir: %w", err)
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".registry-*.tmp")
	if err != nil {
		return fmt.Errorf("registry temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write registry temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close registry temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic registry rename: %w", err)
	}
	return nil
}

// AddResult reports the outcome of AddSources.
type AddResult struct {
	Added   []SourceEntry
	Skipped []string // "path: reason" for unsupported / unreadable inputs
}

// AddSources copies each supported source file under srcPaths into the
// workspace's raw/ and records a registry entry (atomic). A directory is walked
// for supported files; unsupported types are skipped and reported, never failed.
// Re-adding a source updates its entry (idempotent by RawName).
func AddSources(workspace string, srcPaths []string, now time.Time) (*AddResult, error) {
	realRoot, err := scaffoldRealRoot(workspace)
	if err != nil {
		return nil, err
	}
	reg, err := LoadSourceRegistry(workspace)
	if err != nil {
		return nil, err
	}
	res := &AddResult{}
	for _, sp := range srcPaths {
		files, skips, err := expandSourceInput(sp)
		if err != nil {
			return nil, err
		}
		res.Skipped = append(res.Skipped, skips...)
		for _, f := range files {
			entry, skip, err := addOneSource(realRoot, reg, f, now)
			if err != nil {
				return nil, err
			}
			if skip != "" {
				res.Skipped = append(res.Skipped, skip)
				continue
			}
			res.Added = append(res.Added, entry)
		}
	}
	if len(res.Added) > 0 {
		if err := writeSourceRegistry(realRoot, reg); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// expandSourceInput returns the supported files for a path (a file or a dir),
// plus skip messages for unsupported entries.
func expandSourceInput(p string) (files, skips []string, err error) {
	fi, err := os.Stat(p)
	if err != nil {
		return nil, []string{p + ": cannot stat: " + err.Error()}, nil
	}
	if !fi.IsDir() {
		if supportedSourceExts[strings.ToLower(filepath.Ext(p))] {
			return []string{p}, nil, nil
		}
		return nil, []string{p + ": unsupported type (supported: .md/.txt; PDF/URL are adapter follow-ups)"}, nil
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, nil, fmt.Errorf("read dir %s: %w", p, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		full := filepath.Join(p, e.Name())
		if supportedSourceExts[strings.ToLower(filepath.Ext(e.Name()))] {
			files = append(files, full)
		} else {
			skips = append(skips, full+": unsupported type")
		}
	}
	sort.Strings(files)
	return files, skips, nil
}

// addOneSource copies f into raw/ (contained) and upserts its registry entry
// keyed by SLUG. It returns a non-empty skip reason (and adds nothing) when f's
// slug COLLIDES with an already-registered DIFFERENT raw file: two files that
// slugify to the same id would share one derived artifact (wiki/sources/<id>.md)
// — they'd clobber each other at ingest and make removal delete the sibling's
// artifact. Enforcing slug uniqueness keeps slug<->source 1:1 (cross-family
// REFUTE). Re-adding the SAME raw file (same slug+name) updates idempotently.
func addOneSource(realRoot string, reg *SourceRegistry, f string, now time.Time) (entry SourceEntry, skip string, err error) {
	rawName := filepath.Base(f)
	id := slugify(strings.TrimSuffix(rawName, filepath.Ext(rawName)))
	for _, e := range reg.Sources {
		if e.ID == id && e.RawName != rawName {
			return SourceEntry{}, fmt.Sprintf("%s: slug %q collides with already-registered source %q — rename to add both", f, id, e.RawName), nil
		}
	}
	// Also enforce uniqueness over the raw/ DIRECTORY — the ingest's actual input
	// — not just the registry. `remove --keep-raw` can orphan a raw file (entry
	// dropped, raw kept); without this check a later add of a slug-collider would
	// leave two same-slug files in raw/ that recompile then clobbers into one
	// shared artifact, so remove would hit the wrong source (cross-family REFUTE).
	if rawDir, derr := scaffoldSafeAbs(realRoot, "raw"); derr == nil {
		if entries, rerr := os.ReadDir(rawDir); rerr == nil {
			for _, e := range entries {
				if e.IsDir() || e.Name() == rawName {
					continue
				}
				if slugify(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))) == id {
					return SourceEntry{}, fmt.Sprintf("%s: slug %q collides with existing raw file %q — rename to add both", f, id, e.Name()), nil
				}
			}
		}
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return SourceEntry{}, "", fmt.Errorf("read source %s: %w", f, err)
	}
	dst, err := scaffoldSafeAbs(realRoot, filepath.ToSlash(filepath.Join("raw", rawName)))
	if err != nil {
		return SourceEntry{}, "", err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return SourceEntry{}, "", fmt.Errorf("raw dir: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return SourceEntry{}, "", fmt.Errorf("copy source to raw: %w", err)
	}
	sum := sha256.Sum256(data)
	entry = SourceEntry{
		ID:           id,
		RawName:      rawName,
		OriginalPath: f,
		SHA256:       hex.EncodeToString(sum[:]),
		AddedAt:      now.UTC().Format(time.RFC3339),
	}
	for i, e := range reg.Sources { // upsert by slug (collisions already rejected)
		if e.ID == id {
			reg.Sources[i] = entry
			return entry, "", nil
		}
	}
	reg.Sources = append(reg.Sources, entry)
	return entry, "", nil
}

// RemoveOptions tunes RemoveSource.
type RemoveOptions struct {
	DryRun  bool // report artifacts, delete nothing
	KeepRaw bool // do not delete the raw/ source file
}

// RemoveResult reports what RemoveSource removed (or would remove on DryRun).
type RemoveResult struct {
	DocID     string
	Artifacts []string // workspace-relative paths
	Removed   bool     // false on DryRun
}

// derivedArtifactDirs are the wiki/ subdirs whose <id>* files are derived from a
// source and so are removed with it.
var derivedArtifactDirs = []string{"wiki/sources", "wiki/summaries", "wiki/concepts", "wiki/entities", "wiki/explorations", "wiki/reports"}

// RemoveSource removes a registered source and its derived artifacts. With
// DryRun it only REPORTS the workspace-relative artifacts that would be removed.
// Every target is containment-checked (scaffoldSafeAbs): a delete can never
// escape the workspace. Returns an error if the doc is not registered.
func RemoveSource(workspace, doc string, opts RemoveOptions) (*RemoveResult, error) {
	realRoot, err := scaffoldRealRoot(workspace)
	if err != nil {
		return nil, err
	}
	reg, err := LoadSourceRegistry(workspace)
	if err != nil {
		return nil, err
	}
	idx := reg.find(doc)
	if idx < 0 {
		return nil, fmt.Errorf("source %q is not registered", doc)
	}
	entry := reg.Sources[idx]
	// The registry is on-disk data that a destructive op must not trust blindly:
	// a crafted/hand-edited RawName like "../wiki/config.yaml" cleans to an
	// in-workspace path that passes containment yet deletes the WRONG file
	// (cross-family REFUTE). Require RawName to be a bare basename before it is
	// ever used to build a delete target.
	if err := safeRegistryName(entry.RawName); err != nil {
		return nil, err
	}
	// Derive the deletion id from the VALIDATED RawName, not the trusted stored
	// entry.ID: a crafted registry entry like {RawName:"evil.md", ID:"beta"} must
	// NOT let removing it delete a different legitimate "beta" source's derived
	// artifacts (cross-family REFUTE). For a clean entry this equals entry.ID
	// (addOneSource computes the same slug). slugify never returns "" (→"untitled"),
	// so the empty-id dotfile over-match cannot occur.
	effectiveID := slugify(strings.TrimSuffix(entry.RawName, filepath.Ext(entry.RawName)))
	res := &RemoveResult{DocID: effectiveID}

	// Candidate artifacts: the raw file (unless KeepRaw) + derived <id>* files.
	var rels []string
	if !opts.KeepRaw {
		rels = append(rels, filepath.ToSlash(filepath.Join("raw", entry.RawName)))
	}
	for _, dir := range derivedArtifactDirs {
		matches, err := derivedMatches(realRoot, dir, effectiveID)
		if err != nil {
			return nil, err
		}
		rels = append(rels, matches...)
	}
	sort.Strings(rels)

	// PHASE 1 — validate EVERY target's containment and collect existing ones,
	// WITHOUT deleting anything. A containment failure (e.g. a planted symlink
	// artifact) must abort the whole remove with nothing deleted — deleting as we
	// validate would leave earlier artifacts gone after a later target is refused
	// (cross-family REFUTE: partial deletion). This makes remove all-or-nothing on
	// the safety check.
	type removeTarget struct{ rel, abs string }
	var targets []removeTarget
	for _, rel := range rels {
		abs, err := scaffoldSafeAbs(realRoot, rel) // containment: never delete outside the workspace
		if err != nil {
			return nil, err // refuse the ENTIRE remove before any deletion
		}
		if _, statErr := os.Lstat(abs); statErr != nil {
			continue // artifact absent — nothing to report/remove
		}
		targets = append(targets, removeTarget{rel: rel, abs: abs})
		res.Artifacts = append(res.Artifacts, rel)
	}
	// PHASE 2 — all targets validated; now delete (non-dry-run).
	if !opts.DryRun {
		for _, t := range targets {
			if err := os.Remove(t.abs); err != nil {
				return nil, fmt.Errorf("remove %s: %w", t.rel, err)
			}
		}
	}
	// The registry entry itself is always part of the removal.
	res.Artifacts = append(res.Artifacts, SourceRegistryRelPath+" (entry "+entry.ID+")")
	if !opts.DryRun {
		reg.Sources = append(reg.Sources[:idx], reg.Sources[idx+1:]...)
		if err := writeSourceRegistry(realRoot, reg); err != nil {
			return nil, err
		}
		res.Removed = true
	}
	return res, nil
}

// derivedMatches returns workspace-relative paths in dir whose filename equals
// <id> or starts with "<id>-"/"<id>." (a derived artifact for that source).
func derivedMatches(realRoot, dir, id string) ([]string, error) {
	abs, err := scaffoldSafeAbs(realRoot, dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		rel := filepath.ToSlash(filepath.Join(dir, name))
		// NEVER classify the registry control file as a derived artifact — a
		// source whose id is "registry" must not delete the registry itself
		// (cross-family REFUTE).
		if rel == SourceRegistryRelPath {
			continue
		}
		// EXACT stem match only. A prefix heuristic ("id-"/"id.") over-deletes: a
		// sibling source's slug like "alpha-beta" starts with "alpha-", so removing
		// "alpha" would delete alpha-beta's artifact (cross-family REFUTE). Slugs
		// legitimately contain "-"/".", so no prefix is a safe namespace boundary.
		// The ingest names each source's artifact exactly "<id>.md"; sibling stages
		// that emit multiple per-source artifacts must record their paths in the
		// registry (exact tracking) rather than rely on a name prefix.
		if stem := strings.TrimSuffix(name, filepath.Ext(name)); stem == id {
			out = append(out, rel)
		}
	}
	return out, nil
}

// RawCandidates returns the regular-file names under the workspace's raw/ dir —
// the ACTUAL input the recompile/ingest stage processes. recompile --dry-run
// reports these (not the registry) so the preview matches the real run: after
// `remove --keep-raw` an orphaned raw file still recompiles, and dry-run must
// show it rather than lie based on the registry (cross-family REFUTE).
func RawCandidates(workspace string) ([]string, error) {
	realRoot, err := scaffoldRealRoot(workspace)
	if err != nil {
		return nil, err
	}
	rawDir, err := scaffoldSafeAbs(realRoot, "raw")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read raw dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		// Only count files the real ingest will process, so dry-run == real run.
		if !e.IsDir() && supportedSourceExts[strings.ToLower(filepath.Ext(e.Name()))] {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// RegisteredSources returns the registered source entries (sorted by ID).
func RegisteredSources(workspace string) ([]SourceEntry, error) {
	reg, err := LoadSourceRegistry(workspace)
	if err != nil {
		return nil, err
	}
	sort.Slice(reg.Sources, func(i, j int) bool { return reg.Sources[i].ID < reg.Sources[j].ID })
	return reg.Sources, nil
}
