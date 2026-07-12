package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
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
