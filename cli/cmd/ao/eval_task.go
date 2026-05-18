// practices: [llm-eval-harness, dora-metrics]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	evalsub "github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

// AGENTOPS_EVALS_ROOT lets callers (and tests) override the substrate
// root directory. Default ~/.agents/evals.
func evalsRoot() string {
	if r := strings.TrimSpace(os.Getenv("AGENTOPS_EVALS_ROOT")); r != "" {
		return r
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agents/evals"
	}
	return filepath.Join(home, ".agents", "evals")
}

var (
	evalTaskRunSuiteRef       string
	evalTaskRunRigID          string
	evalTaskRunSeeds          string
	evalTaskRunHarnessRef     string
	evalTaskRunHarnessDir     string
	evalTaskRunModelSpecID    string
	evalTaskRunGTRef          string
	evalTaskRunSampleSplit    string
	evalTaskRunNSamples       int
	evalTaskRunInspectVersion string
	evalTaskRunInspectCommand string
	evalTaskRunCrossSpec      bool
	evalTaskRunAllowWeak      bool
	evalTaskRunQuickSession   bool
	evalTaskRunDryRun         bool
)

var evalTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage evaluation Tasks (add, list, show, run)",
	Long: `Operate on the §3 Task primitive of the eval substrate.

Tasks live under $AGENTOPS_EVALS_ROOT/tasks/<id>/task.yaml and define the
input/output contract a Run will be evaluated against. The command surface
exposes:

  ao eval task add <task.yaml>        Register a Task by copying its file
  ao eval task list                   List registered Task ids
  ao eval task show <task-id>         Print a Task summary
  ao eval task run <task-id> ...      Open a new Run manifest under runs/

Run-time gates 1, 6, 7, 8, 9 (per SCHEMA.md §6) refuse the Run before any
Inspect launch when their preconditions fail. Refusal messages match §6
format and are checked by ao eval doctor.`,
}

var evalTaskAddCmd = &cobra.Command{
	Use:   "add <task.yaml>",
	Short: "Register a Task by copying its yaml + samples into the substrate",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := args[0]
		raw, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("eval task add: read %q: %w", src, err)
		}
		var task evalsub.Task
		if err := yaml.Unmarshal(raw, &task); err != nil {
			return fmt.Errorf("eval task add: parse: %w", err)
		}
		if task.ID == "" {
			return fmt.Errorf("eval task add: missing required field id")
		}
		if task.Stats.MinNSamples <= 0 {
			return fmt.Errorf("eval task add: task %q has no stats.min_n_samples (gate #6 cannot enforce)", task.ID)
		}
		dest := filepath.Join(evalsRoot(), "tasks", task.ID, "task.yaml")
		canon, err := evalsub.CanonicalizeYAML(raw)
		if err != nil {
			return fmt.Errorf("eval task add: canonicalize: %w", err)
		}
		if err := evalsub.WriteAtomic(dest, canon); err != nil {
			return fmt.Errorf("eval task add: write: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Task registered: %s\n  path:        %s\n  min_n_samples: %d\n", task.ID, dest, task.Stats.MinNSamples)
		return nil
	},
}

var evalTaskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered Task ids",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := filepath.Join(evalsRoot(), "tasks")
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "No tasks registered")
				return nil
			}
			return fmt.Errorf("eval task list: %w", err)
		}
		var ids []string
		for _, e := range entries {
			if e.IsDir() {
				ids = append(ids, e.Name())
			}
		}
		sort.Strings(ids)
		if GetOutput() == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]interface{}{"tasks": ids, "root": root})
		}
		if len(ids) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No tasks registered")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Tasks under %s:\n", root)
		for _, id := range ids {
			task, err := loadTask(id)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\t<unreadable: %v>\n", id, err)
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\tschema_version=%d min_n_samples=%d metric=%s\n",
				task.ID, task.SchemaVersion, task.Stats.MinNSamples, task.Stats.Metric)
		}
		return nil
	},
}

var evalTaskShowCmd = &cobra.Command{
	Use:   "show <task-id>",
	Short: "Print a registered Task summary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := loadTask(args[0])
		if err != nil {
			return err
		}
		if GetOutput() == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(task)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Task: %s\n", task.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  schema_version: %d\n", task.SchemaVersion)
		fmt.Fprintf(cmd.OutOrStdout(), "  domain:         %s\n", task.Domain)
		fmt.Fprintf(cmd.OutOrStdout(), "  description:    %s\n", task.Description)
		fmt.Fprintf(cmd.OutOrStdout(), "  harness_ref:    %s\n", task.HarnessRef)
		fmt.Fprintf(cmd.OutOrStdout(), "  stats.metric:        %s\n", task.Stats.Metric)
		fmt.Fprintf(cmd.OutOrStdout(), "  stats.paired:        %v\n", task.Stats.Paired)
		fmt.Fprintf(cmd.OutOrStdout(), "  stats.min_n_samples: %d\n", task.Stats.MinNSamples)
		fmt.Fprintf(cmd.OutOrStdout(), "  stats.decision_rule: kind=%s confidence=%.2f\n",
			task.Stats.DecisionRule.Kind, task.Stats.DecisionRule.Confidence)
		return nil
	},
}

var evalTaskRunCmd = &cobra.Command{
	Use:   "run <task-id>",
	Short: "Open a new Run manifest for <task-id>; refuses on gate failure",
	Long: `Opens a new Run under $AGENTOPS_EVALS_ROOT/runs/<run-id>/manifest.json
in pending state, runs §6 manifest-checkable gates 1/6/7/8/9, and on pass
transitions the manifest to running.

This command opens and gates the Run manifest; it does not yet launch
Inspect itself. The atomic-write contract, manifest fields, and refusal
format are all fully exercised here. Use --dry-run to refuse-test without
creating the run directory.`,
	Args: cobra.ExactArgs(1),
	RunE: runEvalTaskRun,
}

func runEvalTaskRun(cmd *cobra.Command, args []string) error {
	gateInputs, suite, task, seeds, err := prepareTaskRunInputs(args[0])
	if err != nil {
		return err
	}

	refusals := evalsub.RunGates(gateInputs)
	if !refusals.Empty() {
		fmt.Fprintln(cmd.ErrOrStderr(), refusals.Format())
		return fmt.Errorf("eval task run: %d gate refusal(s)", len(refusals))
	}

	if evalTaskRunDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "Dry run: gates passed; no Run manifest written.")
		return nil
	}

	rigID := evalTaskRunRigID
	if rigID == "" {
		rigID = "unknown-rig"
	}
	runID := evalsub.GenerateRunID(rigID)
	manifest := buildTaskRunManifest(task, suite, seeds, rigID, gateInputs)

	w, err := evalsub.NewRunWriter(evalsRoot(), runID, manifest)
	if err != nil {
		return fmt.Errorf("eval task run: open run: %w", err)
	}
	if err := w.Transition(evalsub.StatusRunning, func(m *evalsub.Manifest) {
		m.ValidityGatesPassed = []string{
			"held_constant_declared",
			"min_n_samples",
			"ground_truth_immutable",
			"harness_lock_match",
			"multi_comparison_correction",
		}
	}); err != nil {
		return fmt.Errorf("eval task run: transition->running: %w", err)
	}

	return writeTaskRunOutput(cmd, w)
}

// prepareTaskRunInputs loads the task + suite, parses seeds, and assembles the
// gate-input bundle (harness snapshot + GT) for runEvalTaskRun.
func prepareTaskRunInputs(taskID string) (evalsub.GateInputs, *evalsub.Suite, *evalsub.Task, []int, error) {
	task, err := loadTask(taskID)
	if err != nil {
		return evalsub.GateInputs{}, nil, nil, nil, err
	}
	if evalTaskRunSuiteRef == "" {
		return evalsub.GateInputs{}, nil, nil, nil, fmt.Errorf("eval task run: --suite is required")
	}
	suite, err := loadSuite(evalTaskRunSuiteRef)
	if err != nil {
		return evalsub.GateInputs{}, nil, nil, nil, err
	}
	seeds, err := parseSeeds(evalTaskRunSeeds)
	if err != nil {
		return evalsub.GateInputs{}, nil, nil, nil, err
	}
	if len(seeds) < 3 {
		return evalsub.GateInputs{}, nil, nil, nil, fmt.Errorf("eval task run: --seeds requires >=3 values, got %d (per §4 manifest required field)", len(seeds))
	}

	gateInputs := evalsub.GateInputs{
		Suite:       suite,
		Task:        task,
		AllowWeak:   evalTaskRunAllowWeak,
		GTRequested: evalTaskRunGTRef,
	}
	if err := attachHarnessSnapshot(&gateInputs); err != nil {
		return evalsub.GateInputs{}, nil, nil, nil, err
	}
	if evalTaskRunGTRef != "" {
		if gtRows, gtErr := loadGroundTruthFile(); gtErr == nil {
			gateInputs.GroundTruth = gtRows
		}
	}
	return gateInputs, suite, task, seeds, nil
}

// attachHarnessSnapshot snapshots --harness-dir (when set) into the gate
// inputs, expanding ~ and recording the lock + content hash.
func attachHarnessSnapshot(gateInputs *evalsub.GateInputs) error {
	if evalTaskRunHarnessDir == "" {
		return nil
	}
	expanded := evalTaskRunHarnessDir
	if strings.HasPrefix(expanded, "~") {
		home, _ := os.UserHomeDir()
		expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~"))
	}
	harness, lock, err := evalsub.SnapshotHarness(expanded, evalTaskRunHarnessRef, "ao eval task run")
	if err != nil {
		return fmt.Errorf("eval task run: harness snapshot: %w", err)
	}
	gateInputs.Harness = harness
	gateInputs.HarnessLock = lock
	gateInputs.HarnessDir = expanded
	return nil
}

// buildTaskRunManifest assembles the §4 Run manifest from the gathered inputs,
// including model-spec/ground-truth hashes and multi-comparison fields when
// the suite arm count exceeds 2.
func buildTaskRunManifest(task *evalsub.Task, suite *evalsub.Suite, seeds []int, rigID string, gateInputs evalsub.GateInputs) evalsub.Manifest {
	manifest := evalsub.Manifest{
		TaskRef:        task.ID,
		SuiteRef:       suite.ID,
		HarnessRef:     evalTaskRunHarnessRef,
		ModelSpecRef:   evalTaskRunModelSpecID,
		GroundTruthRef: evalTaskRunGTRef,
		SampleSplit:    pickSampleSplit(suite, evalTaskRunSampleSplit),
		NSamples:       pickNSamples(suite, evalTaskRunNSamples),
		Seeds:          seeds,
		RigID:          rigID,
		InspectCommand: evalTaskRunInspectCommand,
		InspectVersion: evalTaskRunInspectVersion,
		QuickSession:   evalTaskRunQuickSession,
	}
	if gateInputs.Harness != nil {
		manifest.HarnessContentHash = gateInputs.Harness.ContentHash
	}
	if evalTaskRunModelSpecID != "" {
		if spec, err := evalsub.LoadModelSpec(evalsRoot(), evalTaskRunModelSpecID); err == nil {
			manifest.ModelSpecHash = spec.ContentHash
		}
	}
	if evalTaskRunGTRef != "" {
		manifest.GroundTruthHash = hashGroundTruthRef(gateInputs.GroundTruth, evalTaskRunGTRef)
	}
	if n := len(suite.VariedAxis.Values); n > 2 {
		manifest.MultiComparisonMethod = suite.Stats.MultiComparisonMethod
		manifest.ComparisonFamily = suite.Stats.ComparisonFamily
		manifest.ReferenceArm = suite.Stats.ReferenceArm
		manifest.FamilySizeK = evalsub.FamilySizeK(suite.Stats.ComparisonFamily, n)
	}
	return manifest
}

func writeTaskRunOutput(cmd *cobra.Command, w *evalsub.RunWriter) error {
	out := w.Manifest()
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Run opened: %s\n  status:    %s\n  manifest:  %s\n  rig_id:    %s\n  task_ref:  %s\n  suite_ref: %s\n",
		out.ID, out.Status, w.Path(), out.RigID, out.TaskRef, out.SuiteRef)
	return nil
}

// loadTask reads the Task by id from the substrate.
func loadTask(id string) (*evalsub.Task, error) {
	path := filepath.Join(evalsRoot(), "tasks", id, "task.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loadTask %q: %w", id, err)
	}
	var task evalsub.Task
	if err := yaml.Unmarshal(raw, &task); err != nil {
		return nil, fmt.Errorf("loadTask %q: parse: %w", id, err)
	}
	return &task, nil
}

// loadSuite resolves a Suite ref against $EVALS_ROOT/suites/<ref>/suite.yaml,
// or treats it as a literal path when it ends in .yaml.
func loadSuite(ref string) (*evalsub.Suite, error) {
	candidate := ref
	if !strings.HasSuffix(ref, ".yaml") && !strings.HasSuffix(ref, ".yml") {
		candidate = filepath.Join(evalsRoot(), "suites", ref, "suite.yaml")
	}
	raw, err := os.ReadFile(candidate)
	if err != nil {
		return nil, fmt.Errorf("loadSuite %q: %w", ref, err)
	}
	var s evalsub.Suite
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("loadSuite %q: parse: %w", ref, err)
	}
	return &s, nil
}

// hashGroundTruthRef computes a manifest-level ground_truth_hash by serializing
// every matching row id (id == requested OR supersedes-chain head pointing to
// it) in deterministic order, then hashing. Day-2 minimal hash; Day-4 graduates
// to Dolt-resolved supersession + supersedes-chain canonicalization.
func hashGroundTruthRef(rows []evalsub.GroundTruthRow, ref string) string {
	if ref == "" || len(rows) == 0 {
		return ""
	}
	var matches []evalsub.GroundTruthRow
	for _, r := range rows {
		if r.ID == ref || r.Supersedes == ref {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return ""
	}
	// Stable serialization: sort by id, marshal each as canonical JSON.
	var buf []byte
	for _, m := range matches {
		bs, _ := json.Marshal(m)
		canon, err := evalsub.CanonicalizeJSON(bs)
		if err != nil {
			canon = bs
		}
		buf = append(buf, canon...)
	}
	return evalsub.ContentHash(buf)
}

// loadGroundTruthFile loads ground-truth rows from the canonical jsonl path
// when --ground-truth is set on the run command. Day-2 keeps this thin; Day-4
// graduates to Dolt-resolved supersession chains.
func loadGroundTruthFile() ([]evalsub.GroundTruthRow, error) {
	path := filepath.Join(evalsRoot(), "ground-truth", "ground-truth.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var rows []evalsub.GroundTruthRow
	dec := json.NewDecoder(f)
	for dec.More() {
		var row evalsub.GroundTruthRow
		if err := dec.Decode(&row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseSeeds(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(p), "%d", &n); err != nil {
			return nil, fmt.Errorf("parseSeeds: bad seed %q: %w", p, err)
		}
		out = append(out, n)
	}
	return out, nil
}

func pickSampleSplit(s *evalsub.Suite, override string) string {
	if override != "" {
		return override
	}
	if s != nil && s.SampleSplit != "" {
		return s.SampleSplit
	}
	return "dev"
}

func pickNSamples(s *evalsub.Suite, override int) int {
	if override > 0 {
		return override
	}
	if s != nil && s.NSamples > 0 {
		return s.NSamples
	}
	return 0
}

func registerEvalTaskCmd() {
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunSuiteRef, "suite", "", "Suite id or path to suite.yaml (required)")
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunRigID, "rig-id", "", "Rig identifier stamped into the Run manifest")
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunSeeds, "seeds", "", "Comma-separated seeds (>=3, per §4)")
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunHarnessRef, "harness", "", "Harness id (recorded into manifest)")
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunHarnessDir, "harness-dir", "", "Path to harness source dir for snapshot + gate #8")
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunModelSpecID, "model-spec", "", "ModelSpec id (already captured via ao eval models capture)")
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunGTRef, "ground-truth", "", "Ground-truth row id (head of supersession chain)")
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunSampleSplit, "sample-split", "", "Sample split (dev|holdout); default from suite")
	evalTaskRunCmd.Flags().IntVar(&evalTaskRunNSamples, "n-samples", 0, "Override Suite.n_samples")
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunInspectVersion, "inspect-version", "0.3.216", "Inspect AI version stamped into manifest")
	evalTaskRunCmd.Flags().StringVar(&evalTaskRunInspectCommand, "inspect-command", "", "Inspect command recorded into the Run manifest (not executed yet)")
	evalTaskRunCmd.Flags().BoolVar(&evalTaskRunCrossSpec, "cross-spec", false, "Allow ModelSpec drift (gate #4)")
	evalTaskRunCmd.Flags().BoolVar(&evalTaskRunAllowWeak, "allow-weak-labels", false, "Allow runs against confidence=weak ground-truth rows (gate #7)")
	evalTaskRunCmd.Flags().BoolVar(&evalTaskRunQuickSession, "quick", false, "Mark Run as quick_session=true (excluded from --vs auto-baseline pool)")
	evalTaskRunCmd.Flags().BoolVar(&evalTaskRunDryRun, "dry-run", false, "Run gates and exit without writing a Run manifest")

	evalTaskCmd.AddCommand(evalTaskAddCmd, evalTaskListCmd, evalTaskShowCmd, evalTaskRunCmd)
}
