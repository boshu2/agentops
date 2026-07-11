// Package trackerresolve owns tracker selection, worktree-aware ledger
// discovery, binary resolution, and child-process context for both br and bd.
package trackerresolve

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	BR = "br"
	BD = "bd"

	SourceEnv    = "env"
	SourceConfig = "config"
	SourceLedger = "ledger"
	SourceBinary = "binary"

	LedgerSourceEnv       = "env"
	LedgerSourceGitCommon = "git-common-dir"
	LedgerSourceRepoRoot  = "repo-root"
	LedgerSourceCWD       = "cwd"
)

type Resolution struct {
	Tracker   string `json:"tracker"`
	Binary    string `json:"binary"`
	LedgerDir string `json:"ledger_dir"`
	Source    string `json:"source"`

	LedgerSource string   `json:"-"`
	RepoRoot     string   `json:"-"`
	GitCommonDir string   `json:"-"`
	WorkDir      string   `json:"-"`
	ChildEnv     []string `json:"-"`
}

type LedgerResolution struct {
	Path         string
	Source       string
	RepoRoot     string
	GitCommonDir string
}

type LookPath func(string) (string, error)

func Resolve(cwd string, env []string) (Resolution, error) {
	return ResolveWithLookPath(cwd, env, exec.LookPath)
}

func ResolveWithLookPath(cwd string, env []string, look LookPath) (Resolution, error) {
	cwd = normalizeCWD(cwd)
	if env == nil {
		env = os.Environ()
	}
	if look == nil {
		look = exec.LookPath
	}
	if raw, ok := envValue(env, "AGENTOPS_TRACKER"); ok {
		kind, err := normalize(raw)
		if err != nil {
			return Resolution{}, fmt.Errorf("AGENTOPS_TRACKER: %w", err)
		}
		return finish(kind, SourceEnv, cwd, env, look), nil
	}
	if raw, ok := configValue(cwd, env); ok {
		kind, err := normalize(raw)
		if err != nil {
			return Resolution{}, fmt.Errorf("tracker key in .agentops/config.yaml: %w", err)
		}
		return finish(kind, SourceConfig, cwd, env, look), nil
	}
	// Backend selection is based on the repository's natural ledgers. An
	// inherited BEADS_DIR tells a selected BR process where to operate; it must
	// not make an unrelated BR ledger outrank this repository's .beads ledger.
	brLedger := resolveNaturalLedger(cwd, BR)
	bdLedger := resolveNaturalLedger(cwd, BD)
	if isDir(brLedger.Path) {
		return finish(BR, SourceLedger, cwd, env, look), nil
	}
	if isDir(bdLedger.Path) {
		return finish(BD, SourceLedger, cwd, env, look), nil
	}
	if _, err := look(BR); err == nil {
		return finish(BR, SourceBinary, cwd, env, look), nil
	}
	if _, err := look(BD); err == nil {
		return finish(BD, SourceBinary, cwd, env, look), nil
	}
	return Resolution{}, fmt.Errorf(
		"cannot resolve a beads tracker from %s: no AGENTOPS_TRACKER override, no ledger "+
			"(_beads for br / .beads for bd), and neither 'br' nor 'bd' is on PATH.\n"+
			"Install one:\n"+
			"  br (beads_rust, local-first): brew install boshu2/agentops/br\n"+
			"  bd (beads, Go):               brew install beads",
		cwd,
	)
}

// ResolveLedger locates a backend's ledger without selecting a backend. BR
// honors BEADS_DIR; BD deliberately ignores it and discovers .beads from the
// canonical repository root.
func ResolveLedger(cwd string, env []string, tracker string) LedgerResolution {
	cwd = normalizeCWD(cwd)
	if env == nil {
		env = os.Environ()
	}
	workspace := resolveWorkspace(cwd)
	if tracker == BR {
		if value, ok := envValue(env, "BEADS_DIR"); ok {
			if !filepath.IsAbs(value) {
				value = filepath.Join(cwd, value)
			}
			return LedgerResolution{
				Path:         filepath.Clean(value),
				Source:       LedgerSourceEnv,
				RepoRoot:     workspace.repoRoot,
				GitCommonDir: workspace.gitCommonDir,
			}
		}
	}
	return naturalLedger(workspace, tracker)
}

func resolveNaturalLedger(cwd, tracker string) LedgerResolution {
	return naturalLedger(resolveWorkspace(normalizeCWD(cwd)), tracker)
}

func naturalLedger(workspace workspaceResolution, tracker string) LedgerResolution {
	name := ".beads"
	if tracker == BR {
		name = "_beads"
	}
	return LedgerResolution{
		Path:         filepath.Join(workspace.repoRoot, name),
		Source:       workspace.source,
		RepoRoot:     workspace.repoRoot,
		GitCommonDir: workspace.gitCommonDir,
	}
}

// ChildEnvironment returns an isolated child env: BR receives exactly one
// canonical BEADS_DIR, while BD receives none so its own .beads discovery wins.
func ChildEnvironment(env []string, tracker, ledgerDir string) []string {
	if env == nil {
		env = os.Environ()
	}
	child := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, "BEADS_DIR=") {
			continue
		}
		child = append(child, entry)
	}
	if tracker == BR {
		child = append(child, "BEADS_DIR="+ledgerDir)
	}
	return child
}

func BeadsDirValue(env []string) (string, bool) {
	if env == nil {
		env = os.Environ()
	}
	return envValue(env, "BEADS_DIR")
}

func RepoRoot(cwd string) string {
	return resolveWorkspace(normalizeCWD(cwd)).repoRoot
}

func finish(kind, source, cwd string, env []string, look LookPath) Resolution {
	ledger := ResolveLedger(cwd, env, kind)
	binary := kind
	if path, err := look(kind); err == nil {
		binary = path
	}
	workDir := cwd
	if kind == BD {
		workDir = ledger.RepoRoot
	}
	return Resolution{
		Tracker:      kind,
		Binary:       binary,
		LedgerDir:    ledger.Path,
		Source:       source,
		LedgerSource: ledger.Source,
		RepoRoot:     ledger.RepoRoot,
		GitCommonDir: ledger.GitCommonDir,
		WorkDir:      workDir,
		ChildEnv:     ChildEnvironment(env, kind, ledger.Path),
	}
}

func normalize(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case BR:
		return BR, nil
	case BD:
		return BD, nil
	default:
		return "", fmt.Errorf("unknown tracker %q (want %q or %q)", raw, BD, BR)
	}
}

func configValue(cwd string, env []string) (string, bool) {
	workspace := resolveWorkspace(cwd)
	paths := []string{filepath.Join(cwd, ".agentops", "config.yaml")}
	if workspace.repoRoot != cwd {
		paths = append(paths, filepath.Join(workspace.repoRoot, ".agentops", "config.yaml"))
	}
	if home, ok := envValue(env, "HOME"); ok {
		paths = append(paths, filepath.Join(home, ".agentops", "config.yaml"))
	}
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if _, duplicate := seen[path]; duplicate {
			continue
		}
		seen[path] = struct{}{}
		data, err := os.ReadFile(path) // #nosec G304 -- fixed config path under cwd/repo/home.
		if err != nil {
			continue
		}
		var value struct {
			Tracker string `yaml:"tracker"`
		}
		if yaml.Unmarshal(data, &value) == nil && strings.TrimSpace(value.Tracker) != "" {
			return value.Tracker, true
		}
	}
	return "", false
}

func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for index := len(env) - 1; index >= 0; index-- {
		if strings.HasPrefix(env[index], prefix) {
			value := strings.TrimSpace(strings.TrimPrefix(env[index], prefix))
			return value, value != ""
		}
	}
	return "", false
}

type workspaceResolution struct {
	repoRoot     string
	gitCommonDir string
	source       string
}

func resolveWorkspace(cwd string) workspaceResolution {
	if common, ok := gitPath(cwd, "--git-common-dir"); ok {
		return workspaceResolution{
			repoRoot:     filepath.Dir(common),
			gitCommonDir: common,
			source:       LedgerSourceGitCommon,
		}
	}
	if root, ok := gitPath(cwd, "--show-toplevel"); ok {
		return workspaceResolution{repoRoot: root, source: LedgerSourceRepoRoot}
	}
	return workspaceResolution{repoRoot: cwd, source: LedgerSourceCWD}
}

func gitPath(cwd, flag string) (string, bool) {
	output, err := exec.Command("git", "-C", cwd, "rev-parse", flag).Output()
	if err != nil {
		return "", false
	}
	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	path = filepath.Clean(path)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return path, true
}

func normalizeCWD(cwd string) string {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if cwd == "" {
		cwd = "."
	}
	if absolute, err := filepath.Abs(cwd); err == nil {
		cwd = absolute
	}
	return filepath.Clean(cwd)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
