package doctor

// Workspace dual-store and nested-tree detectors (report-only).
//
// Two detect-only findings, NO fixers registered:
//
//   - fm-ws-dual-store: BOTH <repo>/.agents/learnings and
//     <repo>/.agents/ao/learnings exist with content. The knowledge subsystem
//     already OWNS the repair for this split —
//     fm-knowledge-orphaned-flywheel-learnings consolidates the fallback dir
//     into the canonical one via per-file Rename. This workspace finding is a
//     broader report-only lens (RepoRoot-anchored, counts ALL regular files
//     transitively, not just top-level *.md/*.jsonl) whose remediation defers
//     to that existing fixer so the two subsystems never fight over the same
//     files.
//
//   - fm-ws-nested-tree: an accidental nested runtime tree
//     <repo>/<subdir>/.agents (e.g. cli/.agents), usually left behind by an
//     agent that ran with the wrong CWD. Merging a nested runtime tree into
//     the root one is a human call — content may belong to a genuinely nested
//     project — so this is manual-remediation only.
//
// The engine tolerates findings with no matching fixer by design:
// groupFindingsByFixer only buckets findings whose ID has a registered fixer,
// and applyFixers skips nil/non-auto-fixable entries, so a --fix pass over
// these IDs performs zero mutations and reports them as unfixed.
//
// Both detectors are PURE (lstat + readdir + WalkDir; symlinks never followed).

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Detector IDs for the workspace dual-store and nested-tree findings.
const (
	workspaceDualStoreID  = "fm-ws-dual-store"
	workspaceNestedTreeID = "fm-ws-nested-tree"
)

// workspaceNestedTreeSkipDirs are the immediate RepoRoot children never
// scanned for a nested `.agents` tree: VCS/vendor trees plus the root
// runtime dir itself (the root `.agents` is the canonical tree, not a nested
// one; `.claude` legitimately contains worktree checkouts with their own
// runtime trees).
var workspaceNestedTreeSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".claude":      true,
	".agents":      true,
}

func init() {
	RegisterDetector(workspaceDualStoreDetector{})
	RegisterDetector(workspaceNestedTreeDetector{})
	// Deliberately NO RegisterFixer for either ID: fm-ws-dual-store defers to
	// the knowledge subsystem's fm-knowledge-orphaned-flywheel-learnings fixer,
	// and fm-ws-nested-tree is a human-judgment merge.
}

// workspaceTransitiveFileCount reports whether dir is a real directory (lstat;
// a symlink to a directory does not count) and, if so, the transitive count of
// regular files under it. WalkDir never follows symlinks, and unreadable
// subtrees are skipped, mirroring workspaceDirInventory's best-effort read.
func workspaceTransitiveFileCount(dir string) (count int, isDir bool) {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return 0, false
	}
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			count++
		}
		return nil
	})
	return count, true
}

// ---------------------------------------------------------------------------
// FM: fm-ws-dual-store (report-only; fix owned by the knowledge subsystem)
// ---------------------------------------------------------------------------

// workspaceDualStoreDetector flags a learnings store split across the legacy
// <repo>/.agents/learnings and the canonical <repo>/.agents/ao/learnings.
type workspaceDualStoreDetector struct{}

func (workspaceDualStoreDetector) ID() string           { return workspaceDualStoreID }
func (workspaceDualStoreDetector) Subsystem() string    { return subsystemWorkspace }
func (workspaceDualStoreDetector) Severity() string     { return "P2" }
func (workspaceDualStoreDetector) EstimatedCostMS() int { return 5 }
func (workspaceDualStoreDetector) OnlineRequired() bool { return false }
func (workspaceDualStoreDetector) QuickPath() bool      { return true }
func (workspaceDualStoreDetector) Describe() string {
	return "learnings split across .agents/learnings and .agents/ao/learnings (fix owned by knowledge)"
}

func (d workspaceDualStoreDetector) Detect(env *DetectEnv) ([]Finding, error) {
	base := workspaceAgentsDir(env)
	legacyCount, legacyIsDir := workspaceTransitiveFileCount(filepath.Join(base, "learnings"))
	canonicalCount, canonicalIsDir := workspaceTransitiveFileCount(filepath.Join(base, "ao", "learnings"))
	if !legacyIsDir || !canonicalIsDir || legacyCount == 0 || canonicalCount == 0 {
		return nil, nil
	}
	// The knowledge subsystem owns the repair: point remediation at its fixer
	// ID (live reference, so a rename there breaks the build here rather than
	// silently drifting) and stay non-auto-fixable so the two never fight.
	knowledgeFixerID := orphanedFlywheelLearningsFixer{}.ID()
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("dual learnings stores: %d file(s) in .agents/learnings, %d in .agents/ao/learnings", legacyCount, canonicalCount),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/learnings",
			Query: fmt.Sprintf("transitive regular-file counts: .agents/learnings=%d .agents/ao/learnings=%d", legacyCount, canonicalCount),
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + knowledgeFixerID,
			ExplainCommand:   "ao doctor explain " + knowledgeFixerID,
			AutoFixable:      false,
			EstimatedActions: 0,
		},
	}}, nil
}

// ---------------------------------------------------------------------------
// FM: fm-ws-nested-tree (report-only; merging is a human call)
// ---------------------------------------------------------------------------

// workspaceNestedTreeDetector flags accidental nested runtime trees
// <repo>/<subdir>/.agents at depth 1 under RepoRoot.
type workspaceNestedTreeDetector struct{}

func (workspaceNestedTreeDetector) ID() string           { return workspaceNestedTreeID }
func (workspaceNestedTreeDetector) Subsystem() string    { return subsystemWorkspace }
func (workspaceNestedTreeDetector) Severity() string     { return "P2" }
func (workspaceNestedTreeDetector) EstimatedCostMS() int { return 25 }
func (workspaceNestedTreeDetector) OnlineRequired() bool { return false }
func (workspaceNestedTreeDetector) QuickPath() bool      { return false }
func (workspaceNestedTreeDetector) Describe() string {
	return "nested .agents runtime tree under a repo subdirectory (manual merge)"
}

func (d workspaceNestedTreeDetector) Detect(env *DetectEnv) ([]Finding, error) {
	entries, err := os.ReadDir(env.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("doctor: read repo root %s: %w", env.RepoRoot, err)
	}
	var findings []Finding
	// os.ReadDir returns entries sorted by name, so findings are deterministic.
	for _, e := range entries {
		// lstat-style types: a symlinked child is ModeSymlink, not IsDir —
		// never follow it into a tree outside the repo.
		if !e.IsDir() || workspaceNestedTreeSkipDirs[e.Name()] {
			continue
		}
		rel := filepath.Join(e.Name(), ".agents")
		count, isDir := workspaceTransitiveFileCount(filepath.Join(env.RepoRoot, rel))
		if !isDir || count == 0 {
			continue
		}
		findings = append(findings, Finding{
			ID:        d.ID(),
			Severity:  d.Severity(),
			Subsystem: d.Subsystem(),
			Title:     fmt.Sprintf("nested runtime tree %s holds %d file(s)", rel, count),
			// Presence is factual, but "accidental" is inferred: the tree may
			// belong to a genuinely nested project.
			Confidence: 0.9,
			Evidence: Evidence{
				File:  rel,
				Query: fmt.Sprintf("transitive regular-file count %s=%d", rel, count),
			},
			Remediation: Remediation{
				Command: "Review " + rel + ". If its contents belong to the root workspace, " +
					"merge them into .agents/ by hand and remove the nested tree; " +
					"if it is an intentional nested project, leave it. Then re-run: ao doctor",
				ExplainCommand:   "ao doctor explain " + d.ID(),
				AutoFixable:      false,
				EstimatedActions: 0,
			},
		})
	}
	return findings, nil
}
