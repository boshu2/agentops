package eval

import (
	"context"
	"errors"
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
//
// FAIL-CLOSED: returns an error (never a partial deny set) when the symlink-reachability
// closure cannot be computed completely — a pathological symlink graph that exceeds the
// closure bounds, or an unreadable directory. The caller (sandboxExecGuard) then refuses
// to run the arm, exactly like the no-Seatbelt case. This is the property that kills the
// nested/chained-symlink escape class: there is no symlink shape that leaks, because any
// shape either lands FULLY in the closure or trips the refusal.
func corpusDenyPaths(startDir string) ([]string, error) {
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
	// Nested/chained-symlink Seatbelt gap: a symlink INSIDE a corpus root (e.g.
	// .agents/learnings -> /external/dir, or .agents/note.md -> /external/secret.txt)
	// canonicalizes OUTSIDE every denied subpath, so Seatbelt (which matches the real,
	// canonical path) would ALLOW a read through it, voiding the A/B. A single-pass walk
	// that only enumerates the FIRST hop is NOT a closure: .agents/learnings -> ext1 and
	// ext1/pivot.md -> ext2/secret.txt leaks ext2 because the walk never descends INTO the
	// resolved external target ext1 (the round-3 refute). The deny set is instead the full
	// symlink-reachability CLOSURE (fixpoint), computed with fail-closed bounds so no
	// symlink shape can escape: every shape either lands fully in the closure or refuses.
	closure, err := symlinkClosureDenyTargets(append([]string(nil), out...), maxSymlinkClosureDirs, maxSymlinkDenyEntries)
	if err != nil {
		return nil, err
	}
	for _, target := range closure {
		add(target)
	}
	return out, nil
}

// Bounds for the symlink-reachability closure. They are the fail-closed guard that kills
// the escape class: a pathological symlink graph (a fan-out, a deep chain, or a symlink to
// a huge external tree such as `/`) trips a bound and the whole arm refuses to run, rather
// than silently computing an incomplete deny set. The bounds apply to the EXTERNAL subtrees
// reached THROUGH symlinks — where explosion happens — not the legitimate corpus roots
// (which are already denied wholesale by subpath and are bounded by reality; the global
// ~/.agents is routinely tens of thousands of dirs, so a global dir cap would false-close
// every real run). symlinkClosureDenyTargets takes the caps as parameters so overflow is
// unit-testable with tiny caps + tiny fixtures; corpusDenyPaths passes these consts.
const (
	// maxSymlinkClosureDirs caps directories walked in symlink-reached external subtrees.
	// It must clear the REAL environment (measured ~129 external dirs on a machine whose
	// ~/.agents symlinks the repo skill corpus) with headroom, while a symlink to `/` (or
	// any large external tree) blows past it and fails closed. If a legitimate corpus ever
	// exceeds it, the arm safely refuses (never leaks); bump this const. Not a security
	// primitive — the fail-closed refusal is.
	maxSymlinkClosureDirs = 1024
	// maxSymlinkDenyEntries caps distinct canonical targets the closure may add (real
	// environment measured ~58; headroom below).
	maxSymlinkDenyEntries = 2048
)

// errSymlinkClosureOverflow is returned (wrapped) when the symlink closure exceeds its
// bounds. It makes the arm FAIL CLOSED via sandboxExecGuard — the same refusal path as an
// unavailable sandbox — because an incomplete deny set could leak the corpus.
var errSymlinkClosureOverflow = errors.New("symlink deny closure exceeded bounds (pathological corpus symlink graph); refusing to run a potentially-unisolated control arm")

// symlinkClosureDenyTargets computes the full symlink-reachability CLOSURE of the deny
// roots: the set of canonical paths a read that starts anywhere in the corpus can reach by
// following symlinks. It is a fixpoint over a worklist —
//
//  1. Walk each root. Every symlink found (file OR dir) is resolved (EvalSymlinks) to its
//     canonical target, which is added to the deny set. Every resolved DIRECTORY target is
//     ALSO pushed onto the worklist, so its interior symlinks get resolved too — this is
//     what closes a CHAINED escape (.agents/learnings -> ext1; ext1/pivot.md -> ext2).
//  2. Repeat until the worklist is empty. The result is the complete closure.
//
// Termination and completeness: there is NO depth bound — a depth cap can only either
// silently truncate the closure (omitting a deep symlink target — the leak this replaced) or
// false-close on a legitimately deep corpus, both of which break "complete closure OR refuse".
// The closure instead walks each subtree in FULL, and terminates on two guards: (1) a target
// is deduped via the seen-set BEFORE being pushed and each root is walked at most once
// (a->b->a terminates; filepath.WalkDir uses Lstat and does not follow symlinks, so no single
// walk can cycle); (2) directories reached THROUGH symlinks are dir-capped (maxDirs), so a
// pathological EXTERNAL graph — a wide fan-out OR a deep linear chain — is bounded and fails
// closed. Exceeding the dir cap, the deny-entry cap (maxEntries), or hitting an unreadable
// directory (a non-NotExist walk error) returns an error so the arm fails closed. Dangling
// symlinks (unresolvable chains) are skipped; device/socket/fifo targets are skipped (not a
// corpus leak). initialRoots (the legitimate corpus dirs) are walked in full but NOT dir-capped
// — they are bounded by reality and already denied by subpath. A missing/NotExist root walks to
// nothing. Caps are parameters so overflow is unit-testable with tiny caps; corpusDenyPaths
// passes the package consts.
func symlinkClosureDenyTargets(initialRoots []string, maxDirs, maxEntries int) ([]string, error) {
	var out []string
	denySeen := map[string]bool{} // canonical targets already added (output dedup)
	walked := map[string]bool{}   // roots already walked (cross-walk cycle guard)
	var worklist []string         // EXTERNAL dir targets reached via symlinks; dir-capped
	dirsWalked := 0               // dirs walked in symlink-reached subtrees only

	addTarget := func(target string, isDir bool) error {
		if denySeen[target] {
			return nil
		}
		denySeen[target] = true
		out = append(out, target)
		if len(out) > maxEntries {
			return errSymlinkClosureOverflow
		}
		if isDir {
			// Descend into the resolved external dir so a CHAINED escape is closed.
			worklist = append(worklist, target)
		}
		return nil
	}

	walkOne := func(root string, countDirs bool) error {
		return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					// A never-created corpus root (e.g. missing .ao): nothing to walk.
					return nil
				}
				// Unreadable dir / permission error: cannot prove the closure is complete.
				return err
			}
			if d.IsDir() {
				if countDirs {
					dirsWalked++
					if dirsWalked > maxDirs {
						return errSymlinkClosureOverflow
					}
				}
				return nil
			}
			if d.Type()&os.ModeSymlink == 0 {
				return nil
			}
			target, rerr := filepath.EvalSymlinks(path)
			if rerr != nil {
				// Dangling symlink or unresolvable chain — nothing to deny.
				return nil
			}
			fi, serr := os.Stat(target)
			if serr != nil {
				return nil
			}
			switch {
			case fi.IsDir():
				return addTarget(target, true)
			case fi.Mode().IsRegular():
				return addTarget(target, false)
			default:
				// device / socket / fifo — not a corpus leak; do not deny, do not descend.
				return nil
			}
		})
	}

	// Phase 1: the legitimate corpus roots — depth-bounded, NOT dir-capped.
	for _, r := range initialRoots {
		if walked[r] {
			continue
		}
		walked[r] = true
		if err := walkOne(r, false); err != nil {
			return nil, fmt.Errorf("symlink deny closure walk of %q: %w", r, err)
		}
	}
	// Phase 2: external subtrees reached via symlinks — dir-capped fixpoint.
	for len(worklist) > 0 {
		r := worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]
		if walked[r] {
			continue
		}
		walked[r] = true
		if err := walkOne(r, true); err != nil {
			return nil, fmt.Errorf("symlink deny closure walk of %q: %w", r, err)
		}
	}
	return out, nil
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
		out = appendWithCanonicalAo(out, d)
		out = appendWithCanonicalAo(out, filepath.Join(filepath.Dir(d), "findings"))
	}
	if d := strings.TrimSpace(globalPatternsDir); d != "" {
		out = appendWithCanonicalAo(out, d)
	}
	return out
}

// appendWithCanonicalAo appends the section dir d AND its canonical ao
// counterpart (<parent>/ao/<section>). The config Paths defaults are
// single-rooted LEGACY section dirs (<base>/learnings etc.), but doctor's
// split fixer migrates the corpus to the canonical <base>/ao/<section>; a deny
// set built only from the legacy dirs leaves the migrated canonical dirs
// readable by the control arm. A dir that is already canonical (its parent is
// named "ao") gets no ao/ao echo. Extra deny entries for a counterpart that
// does not exist on disk are harmless: the closure walk treats NotExist roots
// as empty and Seatbelt subpath denies of absent paths deny nothing.
func appendWithCanonicalAo(out []string, d string) []string {
	out = append(out, d)
	parent := filepath.Dir(d)
	if filepath.Base(parent) == "ao" {
		return out
	}
	return append(out, filepath.Join(parent, "ao", filepath.Base(d)))
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
		// Deny the canonical ao counterpart too (see appendWithCanonicalAo): the
		// config default is the legacy single-rooted dir, but a doctor-migrated
		// corpus lives at <parent>/ao/<section>.
		out = appendWithCanonicalAo(out, d)
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
