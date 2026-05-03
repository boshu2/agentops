// Package paths is the canonical Go-side state-path resolver for AgentOps.
//
// It mirrors lib/ao-paths.sh: both honor the same environment-variable
// precedence so a script and a Go binary running under identical env produce
// identical paths.
//
// Env precedence (highest first):
//
//	AO_HOME              explicit override (treated as the .agents directory itself)
//	CLAUDE_PLUGIN_DATA   Claude plugin data dir (resolves to $CLAUDE_PLUGIN_DATA/.agents)
//	default              $REPO_ROOT/.agents (git rev-parse) or ${cwd}/.agents
//
// Per-subdir overrides (AO_AGENTS_DIR, AO_KNOWLEDGE_ROOT, …) win over the
// default layout once the home is resolved.
//
// Knowledge separation: KnowledgeRoot defaults to <AgentsDir>/wiki — the
// internal compiled wiki under .agents/. The external raw+wiki knowledge tree
// at the vault root is intentionally NOT modeled here; agentops keeps the two
// trees separate by design (see f-2026-05-01-005).
package paths

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Paths bundles every resolved AgentOps state root.
type Paths struct {
	Home          string
	AgentsDir     string
	KnowledgeRoot string
	HooksDir      string
	ScopeLock     string
	RPIDir        string
	FindingsDir   string
	PlansDir      string
	CouncilDir    string
	LearningsDir  string
	PatternsDir   string
	DecisionsDir  string
}

// Resolve reads environment variables and returns the canonical AgentOps
// state-path layout. It never returns an error — missing env values fall back
// to the documented defaults silently. The cwd is used as the starting point
// when neither AO_HOME nor CLAUDE_PLUGIN_DATA is set.
func Resolve() *Paths {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return resolveFrom(cwd)
}

// ResolveFromRepo resolves relative to the git repository root containing the
// current working directory. If git is unavailable or cwd is not inside a
// repo, behavior is identical to Resolve().
func ResolveFromRepo() *Paths {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return ResolveFromRoot(cwd)
}

// ResolveFromRoot resolves relative to the git repository root containing dir.
// If dir is not inside a git repository, dir itself is used as the fallback
// root. Environment overrides keep the same precedence as Resolve().
func ResolveFromRoot(dir string) *Paths {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	root := repoRoot(dir)
	if root == "" {
		root = dir
	}
	return resolveFrom(root)
}

// resolveFrom is the env+layout core, parameterized by the directory used as
// the fallback "repo root" when no explicit AO_HOME / CLAUDE_PLUGIN_DATA env
// is set.
func resolveFrom(repoRootDir string) *Paths {
	home := envHome(repoRootDir)

	agentsDir := envOr("AO_AGENTS_DIR", home)
	knowledge := envOr("AO_KNOWLEDGE_ROOT", filepath.Join(agentsDir, "wiki"))
	hooks := envOr("AO_HOOKS_DIR", filepath.Join(agentsDir, "hooks"))
	scopeLock := envOr("AO_SCOPE_LOCK", filepath.Join(agentsDir, "scope.lock"))
	rpi := envOr("AO_RPI_DIR", filepath.Join(agentsDir, "rpi"))
	findings := envOr("AO_FINDINGS_DIR", filepath.Join(agentsDir, "findings"))
	plans := envOr("AO_PLANS_DIR", filepath.Join(agentsDir, "plans"))
	council := envOr("AO_COUNCIL_DIR", filepath.Join(agentsDir, "council"))
	learnings := envOr("AO_LEARNINGS_DIR", filepath.Join(agentsDir, "learnings"))
	patterns := envOr("AO_PATTERNS_DIR", filepath.Join(agentsDir, "patterns"))
	decisions := envOr("AO_DECISIONS_DIR", filepath.Join(agentsDir, "decisions"))

	return &Paths{
		Home:          home,
		AgentsDir:     agentsDir,
		KnowledgeRoot: knowledge,
		HooksDir:      hooks,
		ScopeLock:     scopeLock,
		RPIDir:        rpi,
		FindingsDir:   findings,
		PlansDir:      plans,
		CouncilDir:    council,
		LearningsDir:  learnings,
		PatternsDir:   patterns,
		DecisionsDir:  decisions,
	}
}

// envHome resolves AO_HOME using the documented precedence.
func envHome(repoRootDir string) string {
	if v := strings.TrimSpace(os.Getenv("AO_HOME")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("CLAUDE_PLUGIN_DATA")); v != "" {
		return filepath.Join(strings.TrimRight(v, "/"), ".agents")
	}
	if repoRootDir == "" {
		repoRootDir = "."
	}
	return filepath.Join(repoRootDir, ".agents")
}

// envOr returns the env var when set+non-empty, else the fallback.
func envOr(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return fallback
}

// repoRoot returns the git repository top-level for the given dir, or "" if
// not in a repo / git unavailable.
func repoRoot(dir string) string {
	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	cmd := exec.Command(git, "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Validate ensures every directory root either exists already or can be
// created. It returns the joined error of every dir that fails. ScopeLock is
// a file path — its parent dir must be creatable, but the file itself is not
// required to exist.
func (p *Paths) Validate() error {
	if p == nil {
		return errors.New("paths: nil receiver")
	}
	dirs := []struct {
		name, path string
	}{
		{"Home", p.Home},
		{"AgentsDir", p.AgentsDir},
		{"KnowledgeRoot", p.KnowledgeRoot},
		{"HooksDir", p.HooksDir},
		{"RPIDir", p.RPIDir},
		{"FindingsDir", p.FindingsDir},
		{"PlansDir", p.PlansDir},
		{"CouncilDir", p.CouncilDir},
		{"LearningsDir", p.LearningsDir},
		{"PatternsDir", p.PatternsDir},
		{"DecisionsDir", p.DecisionsDir},
	}
	var errs []error
	for _, d := range dirs {
		if err := ensureDir(d.path); err != nil {
			errs = append(errs, fmt.Errorf("%s (%s): %w", d.name, d.path, err))
		}
	}
	if p.ScopeLock != "" {
		if err := ensureDir(filepath.Dir(p.ScopeLock)); err != nil {
			errs = append(errs, fmt.Errorf("ScopeLock parent (%s): %w", filepath.Dir(p.ScopeLock), err))
		}
	}
	return errors.Join(errs...)
}

// ensureDir reports nil if dir already exists or was created (perm 0o755).
func ensureDir(dir string) error {
	if dir == "" {
		return errors.New("empty path")
	}
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s: exists but is not a directory", dir)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}
