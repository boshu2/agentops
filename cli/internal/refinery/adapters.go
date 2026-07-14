package refinery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

// NewProduction wires the refinery with git/bd/gates-backed adapters rooted at
// repoRoot. The registry is gates.Default (the seed registry).
func NewProduction(repoRoot string) *Refinery {
	runner := gates.NewScriptRunner(repoRoot)
	return &Refinery{
		Commits: &gitCommitSource{repoRoot: repoRoot},
		Gate:    &gatesChecker{repoRoot: repoRoot, runner: runner},
		Rerun:   &gatesRerunner{repoRoot: repoRoot, runner: runner},
		Beads:   &bdBeadFiler{repoRoot: repoRoot},
		Beacon:  &fileBeacon{repoRoot: repoRoot},
		Store:   &fileStateStore{path: filepath.Join(repoRoot, ".refinery-state")},
		RerunN:  3,
		Log:     func(s string) { fmt.Fprintln(os.Stderr, s) },
	}
}

// --- CommitSource: origin/main HEAD via git ---

type gitCommitSource struct{ repoRoot string }

func (g *gitCommitSource) MainHead(ctx context.Context) (string, error) {
	_, _ = run(ctx, g.repoRoot, "git", "fetch", "origin", "main", "--quiet") // best-effort
	out, err := run(ctx, g.repoRoot, "git", "rev-parse", "origin/main")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// --- GateChecker: the full gate over gates.Default ---

type gatesChecker struct {
	repoRoot string
	runner   ports.GateRunnerPort
}

func (g *gatesChecker) CheckFull(ctx context.Context) (*gates.Report, error) {
	o := gates.NewOrchestrator(gates.Default, g.runner, gates.NewGitChangedFiles(g.repoRoot), g.repoRoot)
	return o.Run(ctx, gates.RunOptions{Mode: gates.Full})
}

// --- Rerunner: re-run one check by ID ---

type gatesRerunner struct {
	repoRoot string
	runner   ports.GateRunnerPort
}

func (g *gatesRerunner) Rerun(ctx context.Context, checkID string) (ports.GateVerdict, error) {
	c, ok := gates.Default.Get(checkID)
	if !ok {
		return ports.GateVerdict{}, fmt.Errorf("refinery: unknown check %q", checkID)
	}
	if c.Run != nil {
		return c.Run(ctx, gates.RunContext{RepoRoot: g.repoRoot, Mode: gates.Full})
	}
	return g.runner.Run(ctx, ports.GateRunRequest{Name: ports.GateName(c.Backing)})
}

// --- BeadFiler: bd create ---

type bdBeadFiler struct{ repoRoot string }

func (b *bdBeadFiler) FileFixBead(ctx context.Context, sha string, checks []string) (string, error) {
	short := sha
	if len(short) > 8 {
		short = short[:8]
	}
	title := fmt.Sprintf("fix: main %s poisoned — deterministic gate failure (%s)", short, strings.Join(checks, ", "))
	resolution, err := trackerresolve.Resolve(b.repoRoot, os.Environ())
	if err != nil {
		return "", err
	}
	args := []string{"create", title, "--type", "task", "--labels", "refinery,blocking", "--json"}
	out, err := (trackerexec.Factory{}).Command(ctx, resolution, args, trackerexec.Streams{}).Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", resolution.Binary, strings.Join(args, " "), err)
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if jerr := json.Unmarshal(out, &parsed); jerr != nil {
		return "", nil // bead may have been created; ID just unparsed
	}
	return parsed.ID, nil
}

// --- Beacon: status file + best-effort git note ---

type fileBeacon struct{ repoRoot string }

type poisonFile struct {
	SHA    string   `json:"sha"`
	Checks []string `json:"checks"`
}

func (b *fileBeacon) path() string { return filepath.Join(b.repoRoot, ".refinery-poison") }

func (b *fileBeacon) Set(ctx context.Context, sha string, checks []string) error {
	data, err := json.MarshalIndent(poisonFile{SHA: sha, Checks: checks}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(b.path(), data, 0o644); err != nil { // #nosec G306 -- a beacon meant to be world-readable by any pusher
		return err
	}
	// best-effort git note so the poison travels with the commit
	_, _ = run(ctx, b.repoRoot, "git", "notes", "--ref=refinery", "add", "-f",
		"-m", "POISON: "+strings.Join(checks, ", "), sha)
	return nil
}

func (b *fileBeacon) Clear(ctx context.Context, sha string) error {
	if err := os.Remove(b.path()); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, _ = run(ctx, b.repoRoot, "git", "notes", "--ref=refinery", "remove", sha)
	return nil
}

// --- StateStore: JSON file ---

type fileStateStore struct{ path string }

func (s *fileStateStore) Load() (State, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("refinery: parse state %s: %w", s.path, err)
	}
	return st, nil
}

func (s *fileStateStore) Save(st State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644) // #nosec G306 -- non-secret refinery state
}

// run executes a command in dir and returns combined stdout.
func run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}
