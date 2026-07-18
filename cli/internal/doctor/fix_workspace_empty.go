package doctor

// Workspace subsystem: fm-ws-empty-dirs (auto-fixable).
//
// Flags empty immediate subdirectories of the workspace root `.agents/` —
// directories PROVABLY holding nothing: zero transitive regular files, zero
// non-regular entries (symlinks, FIFOs, ...), and zero unreadable subpaths
// (truly empty, or holding only empty substructure). These are abandoned
// scaffolding from crashed or
// half-finished lanes: they clutter inventory output and mislead tooling that
// treats directory presence as a signal.
//
// Three name classes are deliberately NOT claimed here:
//
//   - "ao" — the structured knowledge-store root. Its (empty) substructure is
//     the knowledge subsystem's contract (fm-knowledge-missing-substructure).
//   - canonical directory names (the VALUES of workspaceCanonicalAliases:
//     postmortem, pre-mortem-checks, handoff, retro, proofs, tests) — these may
//     legitimately sit empty awaiting their first write.
//   - stale/retry-named directories (isWorkspaceStaleDirName) — owned by the
//     workspace GC failure mode (fm-ws-stale-queue-dirs); no double-claim.
//
// The detector is PURE (stat + readdir only). The fixer's only user-state
// disk write is a Rename into the run's quarantine directory — never a
// remove — performed by quarantineEmptyDir, the directory adaptation of the
// Mutate eight-step discipline (Mutate itself is byte-oriented and cannot
// hash or back up a directory).

import (
	"fmt"
	"os"
	"strings"
)

// fmWorkspaceEmptyDirsID is the shared detector/fixer ID for this failure mode.
const fmWorkspaceEmptyDirsID = "fm-ws-empty-dirs"

func init() {
	RegisterDetector(workspaceEmptyDirsDetector{})
	RegisterFixer(workspaceEmptyDirsFixer{})
}

// workspaceEmptyDirClaimed reports whether name is excluded from the
// empty-dirs failure mode because another owner claims it: the knowledge
// store root, a canonical directory name, or a stale/retry-named directory.
func workspaceEmptyDirClaimed(name string) bool {
	if name == "ao" {
		return true
	}
	for _, canonical := range workspaceCanonicalAliases {
		if name == canonical {
			return true
		}
	}
	return isWorkspaceStaleDirName(name)
}

// workspaceEmptyDirInfoIsEmpty reports whether an inventoried directory is
// PROVABLY empty: zero regular files, zero non-regular entries (a dir of only
// symlinks/FIFOs is not empty), and zero walk errors (an unreadable subtree
// means content is unknown, and unknown is never empty).
func workspaceEmptyDirInfoIsEmpty(info workspaceDirInfo) bool {
	return info.FileCount == 0 && info.OtherEntries == 0 && info.WalkErrs == 0
}

// workspaceEmptyDirCandidates inventories base and returns the names of
// immediate subdirectories that are provably empty (see
// workspaceEmptyDirInfoIsEmpty), excluding claimed names, in deterministic
// (name-sorted) order. A missing base — or a base that is not a real
// directory, e.g. a symlinked `.agents` root — yields nil candidates and a
// nil error: nothing this failure mode may safely claim.
func workspaceEmptyDirCandidates(base string) ([]string, error) {
	if !workspaceRealDir(base) {
		return nil, nil
	}
	inv, err := workspaceDirInventory(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, info := range inv {
		if !workspaceEmptyDirInfoIsEmpty(info) {
			continue
		}
		if workspaceEmptyDirClaimed(info.Name) {
			continue
		}
		out = append(out, info.Name)
	}
	return out, nil
}

// quarantineEmptyDir moves the still-empty workspace directory <base>/<name>
// into quarantine through the shared workspace directory-rename chokepoint
// adapter (workspaceDirRename). The still-empty re-verification runs UNDER
// the per-path lock via the verify hook: a file may have landed since the
// caller's scan, and a dir that gained files is refused, not quarantined.
// Returns (true, nil) when the rename happened (or would happen, in dry-run),
// and (false, err) when it was refused.
func quarantineEmptyDir(ctx *MutateContext, base, name, dest string) (bool, error) {
	verify := func(path string) error {
		baseInv, err := workspaceDirInventory(base)
		if err != nil {
			return fmt.Errorf("doctor: re-inventory %s: %w", base, err)
		}
		for _, di := range baseInv {
			if di.Name == name {
				// Same bar as the detector: provably empty means zero regular
				// files AND zero other entries AND zero walk errors.
				if workspaceEmptyDirInfoIsEmpty(di) {
					return nil
				}
				break
			}
		}
		return fmt.Errorf("doctor: %s gained content (or became unreadable) since scan; refusing to quarantine (refused_unsafe)", path)
	}
	if err := workspaceQuarantineDirByName(ctx, base, name, dest, verify); err != nil {
		return false, err
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// FM: fm-ws-empty-dirs (auto-fixable)
// ---------------------------------------------------------------------------

// workspaceEmptyDirsDetector flags unclaimed empty top-level workspace
// directories in one summarizing finding.
type workspaceEmptyDirsDetector struct{}

func (workspaceEmptyDirsDetector) ID() string           { return fmWorkspaceEmptyDirsID }
func (workspaceEmptyDirsDetector) Subsystem() string    { return subsystemWorkspace }
func (workspaceEmptyDirsDetector) Severity() string     { return "P3" }
func (workspaceEmptyDirsDetector) EstimatedCostMS() int { return 4 }
func (workspaceEmptyDirsDetector) OnlineRequired() bool { return false }
func (workspaceEmptyDirsDetector) QuickPath() bool      { return true }
func (workspaceEmptyDirsDetector) Describe() string {
	return "empty unclaimed top-level .agents directories (abandoned scaffolding)"
}

func (d workspaceEmptyDirsDetector) Detect(env *DetectEnv) ([]Finding, error) {
	candidates, err := workspaceEmptyDirCandidates(workspaceAgentsDir(env))
	if err != nil {
		return nil, fmt.Errorf("doctor: inventory workspace: %w", err)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d empty workspace dir(s): %s", len(candidates), strings.Join(candidates, ", ")),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents",
			Query: "find .agents -mindepth 1 -maxdepth 1 -type d -empty (plus dirs with only empty subdirs)",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: len(candidates),
		},
	}}, nil
}

// workspaceEmptyDirsFixer quarantines each still-empty qualifying directory via
// a Rename into <RunDir>/quarantine/workspace/. It re-scans at fix time, so a
// directory that gained files between detect and fix is naturally skipped.
type workspaceEmptyDirsFixer struct{}

func (workspaceEmptyDirsFixer) ID() string { return fmWorkspaceEmptyDirsID }
func (workspaceEmptyDirsFixer) Preconditions() []string {
	return []string{
		".agents exists and is a directory",
		"directory still holds zero transitive regular files at fix time",
	}
}
func (workspaceEmptyDirsFixer) WritesTo() []string { return []string{".agents"} }
func (workspaceEmptyDirsFixer) Ops() []string      { return []string{"Rename"} }
func (workspaceEmptyDirsFixer) Reversible() bool   { return true }
func (workspaceEmptyDirsFixer) Idempotent() bool   { return true }
func (workspaceEmptyDirsFixer) AutoFixable() bool  { return true }

func (f workspaceEmptyDirsFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	base := workspaceAgentsDir(env)
	// Never mutate through a `.agents` root that is not a real directory: a
	// symlinked root would make every rename act on a tree OUTSIDE the repo
	// while the lexical scope check still passes.
	exists, err := workspaceRequireRealAgentsDir(base)
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: %w", f.ID(), err)
		return res, res.Err
	}
	if !exists {
		res.Fixed = true // nothing to fix
		return res, nil
	}
	candidates, err := workspaceEmptyDirCandidates(base)
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: inventory workspace: %w", f.ID(), err)
		return res, res.Err
	}
	if len(candidates) == 0 {
		res.Fixed = true
		return res, nil
	}
	for _, name := range candidates {
		ok, err := quarantineEmptyDir(ctx, base, name, workspaceQuarantineDest(ctx, name))
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: quarantine %s: %w", f.ID(), name, err)
			return res, res.Err
		}
		if ok {
			res.ActionsTaken++
		}
	}
	if !ctx.DryRun {
		remaining, err := workspaceEmptyDirCandidates(base)
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: post-fix inventory: %w", f.ID(), err)
			return res, res.Err
		}
		if len(remaining) != 0 {
			res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
			return res, res.Err
		}
	}
	res.Fixed = true
	return res, nil
}
