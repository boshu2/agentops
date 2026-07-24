package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/scenario"
	"github.com/boshu2/agentops/cli/internal/scenarioresults"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/subprocess"
)

type Runtime struct{}

func (Runtime) RunSuiteContext(ctx context.Context, options aoeval.RunOptions) (*aoeval.RunRecord, error) {
	return aoeval.RunSuiteContext(ctx, options)
}
func (Runtime) RunBaselineABContext(ctx context.Context, options aoeval.RunOptions) (aoeval.DeltaScorecard, *aoeval.RunRecord, *aoeval.RunRecord, error) {
	return aoeval.RunBaselineABContext(ctx, options)
}
func (Runtime) WriteDeltaScorecard(card aoeval.DeltaScorecard, path string) error {
	return aoeval.WriteDeltaScorecard(card, path)
}
func (Runtime) RunContextABContext(ctx context.Context, options aoeval.RunOptions, contextOptions aoeval.ContextABOptions) (aoeval.ContextDeltaScorecard, *aoeval.RunRecord, *aoeval.RunRecord, error) {
	return aoeval.RunContextABContext(ctx, options, contextOptions)
}
func (Runtime) WriteContextDeltaScorecard(card aoeval.ContextDeltaScorecard, path string) error {
	return aoeval.WriteContextDeltaScorecard(card, path)
}
func (Runtime) LoadRun(path string) (*aoeval.RunRecord, error) { return aoeval.LoadRun(path) }
func (Runtime) CompareRuns(candidate, baseline *aoeval.RunRecord, options aoeval.CompareOptions) (*aoeval.RunRecord, error) {
	return aoeval.CompareRuns(candidate, baseline, options)
}
func (Runtime) WorkDir() (string, error)        { return os.Getwd() }
func (Runtime) Abs(path string) (string, error) { return filepath.Abs(path) }
func (Runtime) PromoteBaseline(run *aoeval.RunRecord, options aoeval.BaselineOptions) (*aoeval.RunRecord, error) {
	return aoeval.PromoteBaseline(run, options)
}
func (Runtime) AuditBaselinePolicy(options aoeval.BaselineAuditOptions) (*aoeval.BaselineAuditReport, error) {
	return aoeval.AuditBaselinePolicy(options)
}
func (Runtime) BuildScorecard(candidate, baseline *aoeval.RunRecord, options aoeval.ScorecardOptions) (*aoeval.Scorecard, error) {
	return aoeval.BuildScorecard(candidate, baseline, options)
}
func (Runtime) WriteScorecard(path string, card *aoeval.Scorecard) error {
	return aoeval.WriteScorecard(path, card)
}
func (Runtime) BuildCoverageReport(options aoeval.CoverageOptions) (*aoeval.CoverageReport, error) {
	return aoeval.BuildCoverageReport(options)
}

func (Runtime) Root() string {
	if root := strings.TrimSpace(os.Getenv("AGENTOPS_EVALS_ROOT")); root != "" {
		return root
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agents/evals"
	}
	return filepath.Join(home, ".agents", "evals")
}

func (Runtime) ListRunIDs(root string) ([]string, error) {
	store, err := evalsubstrate.OpenRootStore(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	entries, err := store.ReadDir("runs")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	seen := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			id, err := evalsubstrate.ParseStorageIdentifier(evalsubstrate.IdentifierRun, entry.Name())
			if err != nil {
				return nil, fmt.Errorf("list run ids: invalid directory %q: %w", entry.Name(), err)
			}
			if prior, ok := seen[id.String()]; ok {
				return nil, fmt.Errorf("list run ids: storage collision for logical run id %q: %q and %q", id.String(), prior, entry.Name())
			}
			seen[id.String()] = entry.Name()
			ids = append(ids, id.String())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (Runtime) LoadManifest(root, id string) (*evalsubstrate.Manifest, error) {
	return evalsubstrate.LoadManifest(root, id)
}

func (Runtime) Transition(root, id string, next evalsubstrate.RunStatus, reason string, now time.Time) error {
	manifest, err := evalsubstrate.LoadManifest(root, id)
	if err != nil {
		return fmt.Errorf("transitionStale: load: %w", err)
	}
	if !legalCleanupTransition(manifest.Status, next) {
		return nil
	}
	manifest.Status = next
	manifest.RetractionReason = reason
	manifest.FinishedAt = now.UTC().Format(time.RFC3339)
	manifest.FinishedAtUnixMs = now.UTC().UnixMilli()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("transitionStale: marshal: %w", err)
	}
	runID, err := evalsubstrate.ParseIdentifier(evalsubstrate.IdentifierRun, id)
	if err != nil {
		return fmt.Errorf("transitionStale: %w", err)
	}
	store, err := evalsubstrate.OpenRootStore(root)
	if err != nil {
		return fmt.Errorf("transitionStale: %w", err)
	}
	defer func() { _ = store.Close() }()
	storageName, err := storedRunDirectoryName(store, runID)
	if err != nil {
		return fmt.Errorf("transitionStale: %w", err)
	}
	path := filepath.ToSlash(filepath.Join("runs", storageName, "manifest.json"))
	return store.WriteAtomic(path, append(data, '\n'), 0o644)
}

func legalCleanupTransition(current, next evalsubstrate.RunStatus) bool {
	return current == evalsubstrate.StatusPending && next == evalsubstrate.StatusAborted || current == evalsubstrate.StatusRunning && next == evalsubstrate.StatusFailed
}

func (Runtime) DeleteRun(root, id string) error {
	runID, err := evalsubstrate.ParseIdentifier(evalsubstrate.IdentifierRun, id)
	if err != nil {
		return fmt.Errorf("delete run: %w", err)
	}
	store, err := evalsubstrate.OpenRootStore(root)
	if err != nil {
		return fmt.Errorf("delete run: %w", err)
	}
	defer func() { _ = store.Close() }()
	storageName, err := storedRunDirectoryName(store, runID)
	if err != nil {
		return fmt.Errorf("delete run: %w", err)
	}
	return store.RemoveAll(filepath.ToSlash(filepath.Join("runs", storageName)))
}
func (Runtime) SweepTempFiles(root string, age int64) ([]string, error) {
	return evalsubstrate.SweepTempFiles(root, age)
}
func (Runtime) Now() time.Time { return time.Now().UTC() }

func (runtime Runtime) ReadFile(path string) ([]byte, error) {
	relative, owned, err := evalOwnedRelative(runtime.Root(), path)
	if err != nil {
		return nil, err
	}
	if !owned {
		return os.ReadFile(path)
	}
	store, err := evalsubstrate.OpenRootStore(runtime.Root())
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return store.ReadFile(relative)
}
func (runtime Runtime) WriteAtomic(path string, data []byte) error {
	relative, owned, err := evalOwnedRelative(runtime.Root(), path)
	if err != nil {
		return err
	}
	if !owned {
		return fmt.Errorf("write eval artifact %q: path is outside eval root %q", path, runtime.Root())
	}
	store, err := evalsubstrate.CreateRootStore(runtime.Root(), 0o755)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	return store.WriteAtomic(relative, data, 0o644)
}
func (runtime Runtime) ListDirectories(path string) ([]string, error) {
	relative, owned, err := evalOwnedRelative(runtime.Root(), path)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, fmt.Errorf("list eval directories %q: path is outside eval root %q", path, runtime.Root())
	}
	store, err := evalsubstrate.OpenRootStore(runtime.Root())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	entries, err := store.ReadDir(relative)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
func (Runtime) UserHome() (string, error) { return os.UserHomeDir() }
func (Runtime) SnapshotHarness(dir, ref, source string) (*evalsubstrate.Harness, *evalsubstrate.HarnessLock, error) {
	return evalsubstrate.SnapshotHarness(dir, ref, source)
}
func (Runtime) LoadModelSpec(root, id string) (*evalsubstrate.ModelSpec, error) {
	return evalsubstrate.LoadModelSpec(root, id)
}
func (Runtime) GenerateRunID(rigID string) string { return evalsubstrate.GenerateRunID(rigID) }
func (Runtime) OpenRun(root, id string, manifest evalsubstrate.Manifest) (aoeval.TaskRunWriter, error) {
	return evalsubstrate.NewRunWriter(root, id, manifest)
}

func (runtime Runtime) RunStats(ctx context.Context, args []string) ([]byte, error) {
	python := runtime.pythonBinary()
	if python == "" {
		return nil, fmt.Errorf("eval suite: substrate venv not found; provision via `python3 -m venv ~/.agents/evals/.venv && pip install numpy scipy`")
	}
	result, err := subprocess.Run(ctx, subprocess.Command{
		Name:        python,
		Args:        args,
		Env:         append(os.Environ(), "PYTHONPATH="+runtime.Root()),
		StdoutLimit: subprocess.CaptureLimit{HeadBytes: 4 * 1024 * 1024},
		StderrLimit: subprocess.CaptureLimit{TailBytes: 64 * 1024},
	})
	if err != nil {
		return nil, fmt.Errorf("eval suite: stats CLI failed: %w (stderr: %s)", err, result.Stderr.String())
	}
	if result.Stdout.Truncated {
		return nil, fmt.Errorf("eval suite: stats CLI output exceeded 4 MiB capture bound (%d bytes)", result.Stdout.TotalBytes)
	}
	return result.Stdout.Bytes(), nil
}

func (runtime Runtime) pythonBinary() string {
	if override := strings.TrimSpace(os.Getenv("AGENTOPS_EVALS_VENV")); override != "" {
		return override
	}
	candidates := []string{filepath.Join(runtime.Root(), ".venv", "bin", "python"), filepath.Join(runtime.Root(), ".venv", "bin", "python3")}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".agents", "evals", ".venv", "bin", "python"), filepath.Join(home, ".agents", "evals", ".venv", "bin", "python3"))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func (Runtime) LoadBurnLedger(path string) (evalsubstrate.HoldoutBurnLedger, error) {
	var ledger evalsubstrate.HoldoutBurnLedger
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ledger, nil
	}
	if err != nil {
		return ledger, fmt.Errorf("read burn ledger %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &ledger); err != nil {
		return ledger, fmt.Errorf("parse burn ledger %s: %w", path, err)
	}
	return ledger, nil
}
func (Runtime) SaveBurnLedger(path string, ledger evalsubstrate.HoldoutBurnLedger) error {
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("encode burn ledger: %w", err)
	}
	// 0o600: holdout burn state is load-bearing for holdout secrecy; a world- or
	// group-readable ledger leaks which holdout scenarios have been spent
	// (age-6j9ee.3). storage.AtomicWriteFile writes through a fresh, unpredictably
	// named temp file (os.CreateTemp, 0o600), chmods it to 0o600, fsyncs, then
	// renames over the target and removes the temp on any error. So the final
	// ledger mode is 0o600 regardless of pre-existing state: a stale world-
	// readable "<path>.tmp" left by a crashed run can no longer bleed its mode
	// into the committed ledger (the old fixed-".tmp" scheme reused that leftover
	// inode, and os.WriteFile does not narrow the mode of an existing file), and
	// no reader ever observes a partial write.
	if err := storage.AtomicWriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write burn ledger %s: %w", path, err)
	}
	return nil
}
func (Runtime) WriteOutcomesManifest(dir, runID string, record aoeval.RunRecord) (string, error) {
	id, err := evalsubstrate.ParseIdentifier(evalsubstrate.IdentifierRun, runID)
	if err != nil {
		return "", fmt.Errorf("write outcomes manifest: %w", err)
	}
	store, err := evalsubstrate.CreateRootStore(dir, 0o750)
	if err != nil {
		return "", fmt.Errorf("create outcomes manifest root %s: %w", dir, err)
	}
	defer func() { _ = store.Close() }()
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode eval-run manifest: %w", err)
	}
	relative := filepath.ToSlash(filepath.Join(id.StorageName(), "manifest.json"))
	if err := store.WriteAtomic(relative, append(data, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write eval-run manifest %s: %w", relative, err)
	}
	path, err := store.Path(relative)
	if err != nil {
		return "", err
	}
	return path, nil
}

func evalOwnedRelative(root, target string) (string, bool, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false, fmt.Errorf("resolve eval root %q: %w", root, err)
	}
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return "", false, fmt.Errorf("resolve eval path %q: %w", target, err)
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteTarget)
	if err != nil {
		return "", false, fmt.Errorf("relativize eval path %q to %q: %w", target, root, err)
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false, nil
	}
	return filepath.ToSlash(relative), true, nil
}

func storedRunDirectoryName(store *evalsubstrate.RootStore, id evalsubstrate.Identifier) (string, error) {
	for _, storageName := range id.CompatibilityStorageNames() {
		relative := filepath.ToSlash(filepath.Join("runs", storageName))
		if _, err := store.Stat(relative); err == nil {
			return storageName, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("run %q does not exist", id.String())
}

func (Runtime) Create(options scenario.CreateOptions) (*scenario.CreateResult, error) {
	return scenario.Create(options)
}
func (Runtime) MkdirAll(path string, mode uint32) error { return os.MkdirAll(path, os.FileMode(mode)) }
func (Runtime) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
func (Runtime) ReadDir(path string) ([]aoeval.ScenarioFileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]aoeval.ScenarioFileEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, aoeval.ScenarioFileEntry{Name: entry.Name(), IsDir: entry.IsDir()})
	}
	return result, nil
}
func (Runtime) WriteFile(path string, data []byte, mode uint32) error {
	return os.WriteFile(path, data, os.FileMode(mode))
}
func (Runtime) LoadDirectives(path, id string) ([]goals.ParsedDirective, error) {
	patcher, _, err := goals.LoadGoalsPatcher(path)
	if err != nil {
		return nil, err
	}
	return goals.FilterDirectives(patcher.Directives(), 0, id), nil
}
func (Runtime) LoadGates(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file, err := goals.ParseMarkdownGoals(data)
	if err != nil {
		return nil, err
	}
	gates := make(map[string]string, len(file.Goals))
	for _, goal := range file.Goals {
		gates[goal.ID] = goal.Check
	}
	return gates, nil
}
func (Runtime) ScenarioDirs() []string { return goals.DefaultScenarioDirs() }
func (Runtime) Measure(command string, timeout time.Duration) (string, string) {
	measurement := goals.MeasureOne(goals.Goal{ID: "scenario-check", Check: command}, timeout)
	return measurement.Result, measurement.Output
}
func (Runtime) CurrentIteration(root string) int {
	loaded, err := scenarioresults.Load(root, false)
	if err == nil && loaded.Artifact != nil {
		return loaded.Artifact.Iteration + 1
	}
	return 1
}
func (Runtime) AppendResults(root, runID string, iteration int, results []scenarioresults.ScenarioResult, now time.Time) error {
	_, err := (scenarioresults.Writer{Now: func() time.Time { return now }}).Append(root, runID, iteration, results)
	return err
}
