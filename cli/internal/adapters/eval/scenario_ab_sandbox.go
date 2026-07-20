package eval

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/wiki"
)

// corpusMarkers are the dir names that mark a corpus root (and the per-root dirs
// an arm must not read).
var corpusMarkers = []string{".agents", ".ao"}

// corpusRoot walks up from startDir to the nearest ancestor containing a corpus
// marker (.agents or .ao), so the deny set is correct no matter which subdir the
// command was invoked from (refuter: os.Getwd() under cli/ left <repo>/.agents
// readable). Falls back to startDir.
func corpusRoot(startDir string) string {
	dir := startDir
	for {
		for _, m := range corpusMarkers {
			if fi, err := os.Stat(filepath.Join(dir, m)); err == nil && fi.IsDir() {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir
		}
		dir = parent
	}
}

// corpusDenyPaths returns the absolute corpus directories a scenario-ab arm must
// NOT read: the repo corpus (<corpusRoot>/.agents, /.ao — resolved from any subdir)
// AND the global corpus (~/.agents), each plus its symlink-resolved canonical path
// (Seatbelt matches the real path). A leak in ANY of these voids the A/B.
func corpusDenyPaths(startDir string) []string {
	root := corpusRoot(startDir)
	cand := []string{filepath.Join(root, ".agents"), filepath.Join(root, ".ao")}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		cand = append(cand, filepath.Join(home, ".agents"))
	}
	// Deny the ENV-resolved corpus dir too. AO_AGENTS_DIR / AO_HOME override where
	// `ao` reads the corpus (wiki.AgentsDirIn — the authoritative resolver); a
	// NON-DEFAULT override points OUTSIDE <root>/.agents, leaving it readable by the
	// control arm (age-58r). Resolving the deny path through the same resolver `ao`
	// uses (not a literal ".agents" join) closes that leak under any env config.
	cand = append(cand, envCorpusDenyPaths(root)...)
	// Deny the CONFIG-resolved global AND local corpus roots too — they may be
	// configured OUTSIDE ~/.agents / <root>/.agents, and `ao lookup` reads exactly
	// these (refuter r2; age-58r for the local LearningsDir/PatternsDir). Sourcing
	// the deny set from config (the authoritative resolver) — not a hand-enumerated
	// list — is what closes the whack-a-mole of "another corpus path".
	if cfg, err := config.Load(nil); err == nil && cfg != nil {
		cand = append(cand, configCorpusDenyPaths(cfg.Paths.GlobalLearningsDir, cfg.Paths.GlobalPatternsDir)...)
		cand = append(cand, localCorpusDenyPaths(root, cfg.Paths.LearningsDir, cfg.Paths.PatternsDir)...)
	}
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range cand {
		add(p)
		add(resolveSymlink(p))
	}
	// Nested-symlink Seatbelt gap: a directory symlink INSIDE a corpus root (e.g.
	// .agents/learnings -> /external/dir) canonicalizes OUTSIDE every denied subpath,
	// so Seatbelt (which matches the real, canonical path) would ALLOW a read through
	// it, voiding the A/B. Walk each resolved deny root and deny the canonical target
	// of any directory symlink found within it. Iterate a snapshot so the appends below
	// don't extend the loop.
	for _, root := range append([]string(nil), out...) {
		for _, target := range nestedSymlinkDenyTargets(root) {
			add(target)
		}
	}
	return out
}

// maxSymlinkWalkDepth bounds the nested-symlink descent (directory levels below a
// deny root). Corpus dirs are small, so this caps the walk cheaply; filepath.WalkDir
// does not follow symlinks (Lstat), so symlink cycles cannot inflate it — the bound
// only guards against an unexpectedly deep real tree.
const maxSymlinkWalkDepth = 8

// nestedSymlinkDenyTargets walks root and returns the canonical (symlink-resolved)
// targets of every DIRECTORY symlink found within it, up to maxSymlinkWalkDepth levels.
// These are the paths a read INSIDE the corpus canonicalizes to; denying them closes
// the nested-symlink escape (a top-level resolveSymlink on the root does not reach a
// symlink one or more levels down). A non-existent root walks to nothing.
func nestedSymlinkDenyTargets(root string) []string {
	var out []string
	cleanRoot := filepath.Clean(root)
	rootDepth := strings.Count(cleanRoot, string(filepath.Separator))
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.Count(filepath.Clean(path), string(filepath.Separator))-rootDepth > maxSymlinkWalkDepth {
			return filepath.SkipDir
		}
		if d.Type()&os.ModeSymlink == 0 {
			return nil
		}
		target, rerr := filepath.EvalSymlinks(path)
		if rerr != nil {
			return nil
		}
		if fi, serr := os.Stat(target); serr != nil || !fi.IsDir() {
			return nil
		}
		out = append(out, target)
		return nil
	})
	return out
}

// configCorpusDenyPaths returns the global corpus directories `ao lookup` resolves
// from config — the global learnings dir, the DERIVED global findings dir
// (Dir(learnings)/findings, exactly as lookup computes it), and the global patterns
// dir. Denying precisely these (sourced from config, not hand-enumerated) closes the
// gap where a global corpus configured outside ~/.agents stayed readable (refuter r2).
// Pure (config values passed in) so it is unit-testable without loading from disk.
func configCorpusDenyPaths(globalLearningsDir, globalPatternsDir string) []string {
	var out []string
	if d := strings.TrimSpace(globalLearningsDir); d != "" {
		out = append(out, d, filepath.Join(filepath.Dir(d), "findings"))
	}
	if d := strings.TrimSpace(globalPatternsDir); d != "" {
		out = append(out, d)
	}
	return out
}

// envCorpusDenyPaths returns the corpus dir an AO_AGENTS_DIR / AO_HOME env override
// redirects `ao` to read from (resolved via wiki.AgentsDirIn — the same resolver the
// real code uses). It returns the override only when it resolves to something OTHER
// than the default <root>/.agents (which is already denied): a non-default override is
// the age-58r leak path. With no env override set the override equals the default and
// nothing is added.
func envCorpusDenyPaths(root string) []string {
	resolved := wiki.AgentsDirIn(root)
	if resolved == "" || resolved == filepath.Join(root, ".agents") {
		return nil
	}
	return []string{resolved}
}

// localCorpusDenyPaths returns the config-resolved LOCAL (per-repo) corpus dirs —
// cfg.Paths.LearningsDir / PatternsDir — that fall OUTSIDE the already-denied
// <root>/.agents subtree. These default to ".agents/learnings" / ".agents/patterns"
// (relative, under the denied .agents root), but a config can point them at an
// ABSOLUTE path elsewhere; that absolute local corpus stayed readable by the control
// arm (age-58r). A relative dir resolves under root/.agents and is dropped (already
// covered); an absolute dir not under <root>/.agents is denied. Pure (config values
// passed in) so it is unit-testable without loading from disk.
func localCorpusDenyPaths(root, learningsDir, patternsDir string) []string {
	var out []string
	for _, d := range []string{learningsDir, patternsDir} {
		d = strings.TrimSpace(d)
		if d == "" || !filepath.IsAbs(d) {
			// Relative dirs resolve under <root>/.agents (already denied).
			continue
		}
		if within(d, filepath.Join(root, ".agents")) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// within reports whether path p is the same as, or nested under, dir.
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func resolveSymlink(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return ""
}

// sandboxExecDenyProfile builds a macOS Seatbelt profile that allows everything
// EXCEPT reads of the given corpus dirs. A denylist (not allowlist) keeps codex
// functional (auth, binary, system libs, temp writes) while the corpus is
// unreadable (age-9a9). All paths must be absolute.
func sandboxExecDenyProfile(denyPaths []string) string {
	var b strings.Builder
	b.WriteString("(version 1)(allow default)(deny file-read*")
	for _, p := range denyPaths {
		fmt.Fprintf(&b, " (subpath %q)", p)
	}
	b.WriteString(")")
	return b.String()
}

// sandboxedCodexArgs returns the argv for sandbox-exec wrapping `codex <codexArgs>`
// with the given profile. Pure (no exec) so the wrapping is unit-testable.
func sandboxedCodexArgs(profile string, codexArgs []string) []string {
	return append([]string{"-p", profile, "codex"}, codexArgs...)
}

// macOSSandboxExec is the Seatbelt binary.
const macOSSandboxExec = "/usr/bin/sandbox-exec"

// sandboxExecGuard is the single fail-closed gate every scenario-ab arm executor
// (codex exec AND the agentic shell runner) shares: it validates that isolation is
// actually available for denyPaths and returns the Seatbelt deny profile to wrap the
// command with, or an error.
//
// FAIL-CLOSED: returns an error — never an empty profile a caller could run bare —
// when no corpus path was resolved or the platform isolation is unavailable. A
// control arm that silently reads the corpus is precisely the failure this guards.
// Callers must treat any error as "do not run this arm".
func sandboxExecGuard(denyPaths []string) (profile string, err error) {
	if len(denyPaths) == 0 {
		return "", fmt.Errorf("arm isolation resolved no corpus paths to deny; refusing to run a potentially-unisolated control arm")
	}
	switch runtime.GOOS {
	case "darwin":
		if _, statErr := os.Stat(macOSSandboxExec); statErr != nil {
			return "", fmt.Errorf("arm isolation requires %s (macOS Seatbelt); refusing to run an unisolated control arm: %w", macOSSandboxExec, statErr)
		}
		return sandboxExecDenyProfile(denyPaths), nil
	default:
		// Linux/WSL bwrap path is age-9a9 S2. Until it lands, fail closed rather
		// than run an unisolated (corpus-readable) control arm.
		return "", fmt.Errorf("scenario-ab arm isolation is not yet implemented for GOOS=%q (age-9a9 S2: bubblewrap); refusing to run an unisolated control arm", runtime.GOOS)
	}
}

// sandboxedCodexCmd builds the *exec.Cmd that runs codex CONFINED so it cannot read
// any of denyPaths. The inner codex runs --dangerously-bypass-approvals-and-sandbox
// (documented for externally-sandboxed environments — the OUTER sandbox-exec is the
// real confinement, and macOS Seatbelt is inherited by child processes).
//
// FAIL-CLOSED via sandboxExecGuard: returns an error — never a bare, unconfined codex
// command — when platform isolation is unavailable or no corpus path was resolved.
func sandboxedCodexCmd(ctx context.Context, denyPaths []string, codexArgs []string) (*exec.Cmd, error) {
	profile, err := sandboxExecGuard(denyPaths)
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, macOSSandboxExec, sandboxedCodexArgs(profile, codexArgs)...), nil
}

// sandboxedShellCmd builds the *exec.Cmd that runs a single emitted worker command
// under a non-login shell (`bash -c`), CONFINED by the SAME Seatbelt deny machinery
// as the codex arm so the control (without-gold) arm cannot read any of denyPaths.
// The caller sets cmd.Dir to the isolated workspace; the deny set covers only the
// corpus roots, so workspace reads/writes stay allowed.
//
// FAIL-CLOSED via sandboxExecGuard: returns an error — never a bare `bash -lc`
// command — when platform isolation is unavailable or no corpus path was resolved.
func sandboxedShellCmd(ctx context.Context, denyPaths []string, command string) (*exec.Cmd, error) {
	profile, err := sandboxExecGuard(denyPaths)
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, macOSSandboxExec, "-p", profile, "bash", "-c", command), nil
}
