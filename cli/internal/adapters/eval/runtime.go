package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/scenario"
	"github.com/boshu2/agentops/cli/internal/scenarioresults"
)

type Runtime struct{}

func (Runtime) RunSuite(options aoeval.RunOptions) (*aoeval.RunRecord, error) {
	return aoeval.RunSuite(options)
}
func (Runtime) RunBaselineAB(options aoeval.RunOptions) (aoeval.DeltaScorecard, *aoeval.RunRecord, *aoeval.RunRecord, error) {
	return aoeval.RunBaselineAB(options)
}
func (Runtime) WriteDeltaScorecard(card aoeval.DeltaScorecard, path string) error {
	return aoeval.WriteDeltaScorecard(card, path)
}
func (Runtime) RunContextAB(options aoeval.RunOptions, contextOptions aoeval.ContextABOptions) (aoeval.ContextDeltaScorecard, *aoeval.RunRecord, *aoeval.RunRecord, error) {
	return aoeval.RunContextAB(options, contextOptions)
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
	entries, err := os.ReadDir(filepath.Join(root, "runs"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (Runtime) LoadManifest(root, id string) (*evalsubstrate.Manifest, error) {
	return evalsubstrate.LoadManifest(evalsubstrate.ManifestPath(root, id))
}

func (Runtime) Transition(root, id string, next evalsubstrate.RunStatus, reason string, now time.Time) error {
	path := evalsubstrate.ManifestPath(root, id)
	manifest, err := evalsubstrate.LoadManifest(path)
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
	return evalsubstrate.WriteAtomic(path, append(data, '\n'))
}

func legalCleanupTransition(current, next evalsubstrate.RunStatus) bool {
	return current == evalsubstrate.StatusPending && next == evalsubstrate.StatusAborted || current == evalsubstrate.StatusRunning && next == evalsubstrate.StatusFailed
}

func (Runtime) DeleteRun(root, id string) error { return os.RemoveAll(filepath.Join(root, "runs", id)) }
func (Runtime) SweepTempFiles(root string, age int64) ([]string, error) {
	return evalsubstrate.SweepTempFiles(root, age)
}
func (Runtime) Now() time.Time { return time.Now().UTC() }

func (Runtime) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (Runtime) WriteAtomic(path string, data []byte) error {
	return evalsubstrate.WriteAtomic(path, data)
}
func (Runtime) ListDirectories(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
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

func (runtime Runtime) RunStats(args []string) ([]byte, error) {
	python := runtime.pythonBinary()
	if python == "" {
		return nil, fmt.Errorf("eval suite: substrate venv not found; provision via `python3 -m venv ~/.agents/evals/.venv && pip install numpy scipy`")
	}
	command := exec.Command(python, args...)
	command.Env = append(os.Environ(), "PYTHONPATH="+runtime.Root())
	output, err := command.Output()
	if err != nil {
		stderr := ""
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			stderr = string(exit.Stderr)
		}
		return nil, fmt.Errorf("eval suite: stats CLI failed: %w (stderr: %s)", err, stderr)
	}
	return output, nil
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
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return fmt.Errorf("write burn ledger temp: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("commit burn ledger: %w", err)
	}
	return nil
}
func (Runtime) WriteOutcomesManifest(dir, runID string, record aoeval.RunRecord) (string, error) {
	runDir := filepath.Join(dir, runID)
	if err := os.MkdirAll(runDir, 0o750); err != nil {
		return "", fmt.Errorf("create manifest dir %s: %w", runDir, err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode eval-run manifest: %w", err)
	}
	path := filepath.Join(runDir, "manifest.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write eval-run manifest %s: %w", path, err)
	}
	return path, nil
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
