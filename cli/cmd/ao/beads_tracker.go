package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type beadsDirResolution struct {
	Path   string
	Source string
}

const (
	beadsDirSourceEnv       = "env"
	beadsDirSourceGitCommon = "git-common-dir"
	beadsDirSourceRepoRoot  = "repo-root"
	beadsDirSourceCWD       = "cwd"
)

func beadsTrackerCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd = "."
	}
	return beadsTrackerCommandContextInDir(ctx, cwd, args...)
}

func beadsTrackerCommandContextInDir(ctx context.Context, cwd string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "br", args...)
	cmd.Dir = cwd
	cmd.Env = beadsTrackerEnvForDir(cwd)
	return cmd
}

func beadsTrackerEnvForDir(cwd string) []string {
	if _, ok := beadsEnvValue(os.Environ()); ok {
		return os.Environ()
	}
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "BEADS_DIR=") {
			continue
		}
		env = append(env, entry)
	}
	resolved := resolveBeadsDir(cwd, os.Environ())
	return append(env, "BEADS_DIR="+resolved.Path)
}

func resolveBeadsDir(cwd string, env []string) beadsDirResolution {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil || cwd == "" {
			cwd = "."
		}
	}
	if dir, ok := beadsEnvValue(env); ok {
		if filepath.IsAbs(dir) {
			return beadsDirResolution{Path: filepath.Clean(dir), Source: beadsDirSourceEnv}
		}
		return beadsDirResolution{Path: filepath.Clean(filepath.Join(cwd, dir)), Source: beadsDirSourceEnv}
	}
	if path := beadsDirFromGitCommon(cwd); path != "" {
		return beadsDirResolution{Path: path, Source: beadsDirSourceGitCommon}
	}
	if root, err := repoRootForBeads(cwd); err == nil && root != "" {
		return beadsDirResolution{Path: filepath.Join(root, "_beads"), Source: beadsDirSourceRepoRoot}
	}
	return beadsDirResolution{Path: filepath.Join(cwd, "_beads"), Source: beadsDirSourceCWD}
}

func beadsEnvValue(env []string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		entry := env[i]
		if strings.HasPrefix(entry, "BEADS_DIR=") {
			value := strings.TrimSpace(strings.TrimPrefix(entry, "BEADS_DIR="))
			if value != "" {
				return value, true
			}
			return "", false
		}
	}
	return "", false
}

func beadsDirFromGitCommon(cwd string) string {
	return ledgerDirFromGitCommon(cwd, "_beads")
}

// ledgerDirFromGitCommon returns <repo-root>/<name> resolved through git's
// common dir so a linked worktree points at the canonical ledger rather than a
// per-worktree copy. Returns "" when git cannot resolve a common dir (not a
// repo). name is the ledger directory basename ("_beads" for br, ".beads" for
// bd).
func ledgerDirFromGitCommon(cwd, name string) string {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	common := strings.TrimSpace(string(out))
	if common == "" {
		return ""
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(cwd, common)
	}
	return filepath.Join(filepath.Dir(filepath.Clean(common)), name)
}

// repoRootForBeads finds the git repo root for dir, falling back to dir.
func repoRootForBeads(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		if dir != "" {
			return dir, nil
		}
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return "", cwdErr
		}
		return cwd, nil
	}
	return strings.TrimSpace(string(out)), nil
}

// ------------------------------------------------------------------------
// Dual-tracker detection + resolution (bd + br)  —  age-fvr8
//
// AgentOps is a product. Most end users track their beads with bd (beads, Go);
// this repo tracks with br (beads_rust). The shipped ao CLI + skills must work
// with either. resolveTracker is the load-bearing foundation: it names which
// tracker the current environment uses, its resolved binary, and its ledger
// directory — so later increments (a CRUD passthrough, then tracker-agnostic
// skills) can build on one source of truth instead of re-detecting everywhere.
// ------------------------------------------------------------------------

// Tracker kinds AgentOps understands.
const (
	trackerBR = "br" // beads_rust — this repo's tracker, the local-first successor
	trackerBD = "bd" // beads (Go) — what most product users track with
)

// How the tracker was selected, in precedence order (first match wins).
const (
	trackerSourceEnv    = "env"    // AGENTOPS_TRACKER override
	trackerSourceConfig = "config" // tracker: key in .agentops/config.yaml
	trackerSourceLedger = "ledger" // a _beads/ (br) or .beads/ (bd) dir present
	trackerSourceBinary = "binary" // command -v br / bd
)

// trackerEnvVar is the explicit tracker override. Honored over config and over
// any ledger/binary detection.
const trackerEnvVar = "AGENTOPS_TRACKER"

// trackerConfigKey is the top-level key in .agentops/config.yaml that pins the
// beads tracker, e.g. `tracker: bd`.
const trackerConfigKey = "tracker"

// trackerResolution is the resolved beads tracker for the current environment.
type trackerResolution struct {
	Tracker   string `json:"tracker"`    // "bd" | "br"
	Binary    string `json:"binary"`     // resolved absolute path, or bare name when not on PATH
	LedgerDir string `json:"ledger_dir"` // ledger directory for this tracker
	Source    string `json:"source"`     // env | config | ledger | binary
}

// trackerLookPath resolves a tracker binary on PATH. Overridable in tests so
// the binary-availability precedence branch can be exercised deterministically.
var trackerLookPath = exec.LookPath

// resolveTracker determines which beads tracker (bd or br) the current
// environment uses, plus its resolved binary and ledger directory.
//
// Precedence (first match wins):
//  1. AGENTOPS_TRACKER=bd|br            — explicit env override (over config)
//  2. tracker: <bd|br> in .agentops/config.yaml — project config over home config
//  3. Ledger present, resolved through git's common dir (worktree-aware):
//     a _beads/ dir ⇒ br, a .beads/ dir ⇒ bd. If BOTH exist, br wins
//     deterministically (this repo's case).
//  4. Available binary: br first (the local-first successor), then bd.
//  5. Otherwise an actionable error naming both install paths.
func resolveTracker(cwd string, env []string) (trackerResolution, error) {
	if env == nil {
		env = os.Environ()
	}
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil && wd != "" {
			cwd = wd
		} else {
			cwd = "."
		}
	}

	// (1) explicit env override — highest precedence.
	if v, ok := envLookup(env, trackerEnvVar); ok {
		kind, err := normalizeTracker(v)
		if err != nil {
			return trackerResolution{}, fmt.Errorf("%s: %w", trackerEnvVar, err)
		}
		return finishTracker(kind, trackerSourceEnv, cwd, env), nil
	}

	// (2) config key — project config wins over home config.
	if v, ok := configTrackerValue(cwd, env); ok {
		kind, err := normalizeTracker(v)
		if err != nil {
			return trackerResolution{}, fmt.Errorf("%s key in .agentops/config.yaml: %w", trackerConfigKey, err)
		}
		return finishTracker(kind, trackerSourceConfig, cwd, env), nil
	}

	// (3) ledger presence — br wins the tie when both dirs exist.
	if isExistingDir(brLedgerDirNatural(cwd)) {
		return finishTracker(trackerBR, trackerSourceLedger, cwd, env), nil
	}
	if isExistingDir(bdLedgerDir(cwd)) {
		return finishTracker(trackerBD, trackerSourceLedger, cwd, env), nil
	}

	// (4) available binary — br first.
	if path, err := trackerLookPath(trackerBR); err == nil {
		res := finishTracker(trackerBR, trackerSourceBinary, cwd, env)
		res.Binary = path
		return res, nil
	}
	if path, err := trackerLookPath(trackerBD); err == nil {
		res := finishTracker(trackerBD, trackerSourceBinary, cwd, env)
		res.Binary = path
		return res, nil
	}

	// (5) nothing resolvable — name both install paths.
	return trackerResolution{}, fmt.Errorf(
		"cannot resolve a beads tracker from %s: no %s override, no ledger "+
			"(_beads for br / .beads for bd), and neither 'br' nor 'bd' is on PATH.\n"+
			"Install one:\n"+
			"  br (beads_rust, local-first): brew install boshu2/agentops/br\n"+
			"  bd (beads, Go):               brew install beads",
		cwd, trackerEnvVar)
}

// finishTracker fills LedgerDir + Binary for a chosen tracker/source.
func finishTracker(kind, source, cwd string, env []string) trackerResolution {
	res := trackerResolution{Tracker: kind, Source: source}
	if kind == trackerBD {
		res.LedgerDir = bdLedgerDir(cwd)
	} else {
		// br honors BEADS_DIR + git-common-dir exactly as `ao beads dir` does.
		res.LedgerDir = resolveBeadsDir(cwd, env).Path
	}
	if path, err := trackerLookPath(kind); err == nil {
		res.Binary = path
	} else {
		res.Binary = kind // bare name; the user may not have installed it yet
	}
	return res
}

// normalizeTracker maps a raw value to a canonical tracker kind or errors.
func normalizeTracker(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case trackerBR:
		return trackerBR, nil
	case trackerBD:
		return trackerBD, nil
	default:
		return "", fmt.Errorf("unknown tracker %q (want %q or %q)", v, trackerBD, trackerBR)
	}
}

// configTrackerValue reads the top-level `tracker:` key from
// .agentops/config.yaml, project config (cwd) taking precedence over home
// config (matching `ao config` precedence 3 > 4). Returns "" / false when the
// key is absent or the files don't exist.
func configTrackerValue(cwd string, env []string) (string, bool) {
	paths := []string{filepath.Join(cwd, ".agentops", "config.yaml")}
	if home, ok := envLookup(env, "HOME"); ok {
		paths = append(paths, filepath.Join(home, ".agentops", "config.yaml"))
	} else if home := os.Getenv("HOME"); home != "" {
		paths = append(paths, filepath.Join(home, ".agentops", "config.yaml"))
	}
	for _, p := range paths {
		data, err := os.ReadFile(p) // #nosec G304 -- fixed config filename under a known dir
		if err != nil {
			continue
		}
		var doc struct {
			Tracker string `yaml:"tracker"`
		}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}
		if v := strings.TrimSpace(doc.Tracker); v != "" {
			return v, true
		}
	}
	return "", false
}

// brLedgerDirNatural returns the canonical br ledger dir (_beads) for cwd,
// worktree-aware, ignoring any BEADS_DIR override. Used only for ledger-presence
// detection; the resolved LedgerDir for br honors BEADS_DIR via resolveBeadsDir.
func brLedgerDirNatural(cwd string) string {
	if dir := ledgerDirFromGitCommon(cwd, "_beads"); dir != "" {
		return dir
	}
	if root, err := repoRootForBeads(cwd); err == nil && root != "" {
		return filepath.Join(root, "_beads")
	}
	return filepath.Join(cwd, "_beads")
}

// bdLedgerDir returns the bd ledger dir (.beads) for cwd, worktree-aware.
func bdLedgerDir(cwd string) string {
	if dir := ledgerDirFromGitCommon(cwd, ".beads"); dir != "" {
		return dir
	}
	if root, err := repoRootForBeads(cwd); err == nil && root != "" {
		return filepath.Join(root, ".beads")
	}
	return filepath.Join(cwd, ".beads")
}

// isExistingDir reports whether path exists and is a directory.
func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// envLookup returns the last value of key in env (later entries win, matching
// the OS) and whether it was present and non-empty.
func envLookup(env []string, key string) (string, bool) {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(env[i], prefix))
			if val != "" {
				return val, true
			}
			return "", false
		}
	}
	return "", false
}
