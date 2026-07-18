package doctor

// Workspace subsystem shared base.
//
// This file holds the helpers the workspace hygiene detectors and fixers are
// built on: the `.agents` root resolver, a symlink-safe directory inventory,
// the canonical-alias registry for drifted directory spellings, stale/retry
// name classification, the quarantine Rename destination, and the GC TTL.
// It registers NO detectors and NO fixers; those live in sibling files.
//
// Helpers here are PURE reads (stat + readdir only). Any future fixer disk
// write flows through Mutate; "deletion" of a workspace directory is a Rename
// into the run's quarantine/ directory (see workspaceQuarantineDest), never a
// remove.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// subsystemWorkspace is the Subsystem() name shared by all workspace
// detectors and fixers.
const subsystemWorkspace = "workspace"

// workspaceAgentsDir returns the workspace hygiene root, <RepoRoot>/.agents.
func workspaceAgentsDir(env *DetectEnv) string {
	return filepath.Join(env.RepoRoot, ".agents")
}

// workspaceDirInfo describes one immediate subdirectory of the workspace root.
type workspaceDirInfo struct {
	// Name is the directory's base name.
	Name string
	// FileCount is the transitive count of regular files under the directory.
	FileCount int
	// ByteSize is the transitive sum of regular-file sizes in bytes.
	ByteSize int64
	// NewestMTime is the newest regular-file modification time under the
	// directory; zero when the directory contains no regular files.
	NewestMTime time.Time
}

// workspaceDirInventory inventories the immediate subdirectories of base,
// returning one workspaceDirInfo per directory in deterministic name order.
// Symlinks are never followed: a top-level symlink (even to a directory) is
// not inventoried, and symlinks encountered during the walk contribute
// nothing. Unreadable entries are skipped, not fatal — the inventory is a
// best-effort read of a possibly messy workspace. A missing or unreadable
// base itself is the only error case.
func workspaceDirInventory(base string) ([]workspaceDirInfo, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	out := make([]workspaceDirInfo, 0, len(entries))
	for _, e := range entries {
		// os.ReadDir reports lstat-style types, so a symlink to a directory
		// is ModeSymlink here (not IsDir) and gets skipped — symlink-safe.
		if !e.IsDir() {
			continue
		}
		info := workspaceDirInfo{Name: e.Name()}
		// WalkDir never follows symlinks; the error callback path tolerates
		// permission failures by skipping the offending subtree.
		_ = filepath.WalkDir(filepath.Join(base, e.Name()), func(_ string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			fi, infoErr := d.Info()
			if infoErr != nil {
				return nil
			}
			info.FileCount++
			info.ByteSize += fi.Size()
			if fi.ModTime().After(info.NewestMTime) {
				info.NewestMTime = fi.ModTime()
			}
			return nil
		})
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// workspaceCanonicalAliases maps drifted spellings of top-level `.agents`
// directory names to their canonical names. The drift detector consumes this
// table verbatim: a key present as a top-level directory is a drift finding
// whose remediation is a merge/rename into the value directory.
var workspaceCanonicalAliases = map[string]string{
	"post-mortem":  "postmortem",
	"post-mortems": "postmortem",
	"pre-mortem":   "pre-mortem-checks",
	"pre-mortems":  "pre-mortem-checks",
	"handoffs":     "handoff",
	"mto-handoff":  "handoff",
	"retros":       "retro",
	"proof":        "proofs",
	"test":         "tests",
}

// workspaceStaleSuffixRe matches an explicit stale marker: a trailing
// `.stale-<timestamp>` suffix as written by land-queue staling
// (e.g. `land-queue-age-h433.19.stale-20260711T221032Z`).
var workspaceStaleSuffixRe = regexp.MustCompile(`\.stale-\d{8}T\d{6}Z$`)

// workspaceRetryChainRe matches a retry-chain directory: a trailing
// `-retry<N>` suffix, optionally itself staled
// (e.g. `land-queue-age-h433.22-native-retry2`).
var workspaceRetryChainRe = regexp.MustCompile(`-retry\d+(\.stale-\d{8}T\d{6}Z)?$`)

// workspaceStaleNameKind classifies a top-level workspace directory name.
// explicitStale reports a `.stale-<timestamp>` suffix; retryChain reports a
// `-retry<N>` suffix (possibly combined with a stale suffix). Both can be
// true at once for a staled retry directory.
func workspaceStaleNameKind(name string) (explicitStale, retryChain bool) {
	return workspaceStaleSuffixRe.MatchString(name), workspaceRetryChainRe.MatchString(name)
}

// isWorkspaceStaleDirName reports whether a top-level directory name is
// GC-eligible debris: explicitly staled, a retry-chain dir, or both.
func isWorkspaceStaleDirName(name string) bool {
	explicitStale, retryChain := workspaceStaleNameKind(name)
	return explicitStale || retryChain
}

// workspaceQuarantineDest returns the Rename target for quarantining a
// top-level workspace directory: <RunDir>/quarantine/workspace/<dirName>.
// This mirrors the run-quarantine scheme every other fixer uses (paths under
// <RunDir>/quarantine/, cf. RunArtifact.QuarantineDir and the knowledge
// fixer's staged-mkdir path): there is no deletion op, so a workspace GC
// fixer "deletes" via Mutate with Rename{To: workspaceQuarantineDest(...)}.
// Rename's executeAtomic MkdirAll's the destination parent.
func workspaceQuarantineDest(ctx *MutateContext, dirName string) string {
	return filepath.Join(ctx.RunDir, "quarantine", subsystemWorkspace, dirName)
}

// Workspace GC TTL configuration.
const (
	// workspaceTTLEnvVar overrides the workspace GC TTL, in whole days.
	workspaceTTLEnvVar = "AO_DOCTOR_WS_TTL_DAYS"
	// workspaceTTLDefaultDays is the default workspace GC TTL in days.
	workspaceTTLDefaultDays = 14
)

// workspaceGCTTL returns the workspace GC TTL: stale/retry directories whose
// newest content is older than this are GC candidates. Defaults to 14 days;
// AO_DOCTOR_WS_TTL_DAYS overrides with a positive integer day count, and any
// invalid value falls back to the default.
func workspaceGCTTL() time.Duration {
	if raw := os.Getenv(workspaceTTLEnvVar); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return time.Duration(n) * day
		}
	}
	return workspaceTTLDefaultDays * day
}
