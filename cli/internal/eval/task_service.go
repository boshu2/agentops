package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

type TaskRunWriter interface {
	Transition(evalsubstrate.RunStatus, func(*evalsubstrate.Manifest)) error
	Manifest() evalsubstrate.Manifest
	Path() string
}

type TaskRuntime interface {
	Root() string
	ReadFile(string) ([]byte, error)
	WriteAtomic(string, []byte) error
	ListDirectories(string) ([]string, error)
	UserHome() (string, error)
	SnapshotHarness(string, string, string) (*evalsubstrate.Harness, *evalsubstrate.HarnessLock, error)
	LoadModelSpec(string, string) (*evalsubstrate.ModelSpec, error)
	GenerateRunID(string) string
	OpenRun(string, string, evalsubstrate.Manifest) (TaskRunWriter, error)
	Now() time.Time
}

type TaskService struct{ Runtime TaskRuntime }
type TaskAddRequest struct{ SourcePath string }
type TaskAddResult struct {
	Task        evalsubstrate.Task
	Destination string
}
type TaskSummary struct {
	Task      *evalsubstrate.Task `json:"task,omitempty"`
	ID, Error string
}
type TaskListResult struct {
	Tasks []TaskSummary `json:"tasks"`
	Root  string        `json:"root"`
}
type TaskRunRequest struct {
	TaskID, SuiteRef, RigID, Seeds, HarnessRef, HarnessDir, ModelSpecID, GroundTruthRef string
	SampleSplit                                                                         string
	NSamples                                                                            int
	InspectVersion, InspectCommand                                                      string
	CrossSpec, AllowWeak, QuickSession, DryRun                                          bool
}
type TaskRunResult struct {
	DryRun   bool
	Manifest evalsubstrate.Manifest
	Path     string
}

func (service TaskService) Add(_ context.Context, request TaskAddRequest) (TaskAddResult, error) {
	raw, err := service.Runtime.ReadFile(request.SourcePath)
	if err != nil {
		return TaskAddResult{}, fmt.Errorf("eval task add: read %q: %w", request.SourcePath, err)
	}
	var task evalsubstrate.Task
	if err := yaml.Unmarshal(raw, &task); err != nil {
		return TaskAddResult{}, fmt.Errorf("eval task add: parse: %w", err)
	}
	if task.ID == "" {
		return TaskAddResult{}, fmt.Errorf("eval task add: missing required field id")
	}
	if task.Stats.MinNSamples <= 0 {
		return TaskAddResult{}, fmt.Errorf("eval task add: task %q has no stats.min_n_samples (gate #6 cannot enforce)", task.ID)
	}
	canonical, err := evalsubstrate.CanonicalizeYAML(raw)
	if err != nil {
		return TaskAddResult{}, fmt.Errorf("eval task add: canonicalize: %w", err)
	}
	destination := filepath.Join(service.Runtime.Root(), "tasks", task.ID, "task.yaml")
	if err := service.Runtime.WriteAtomic(destination, canonical); err != nil {
		return TaskAddResult{}, fmt.Errorf("eval task add: write: %w", err)
	}
	return TaskAddResult{Task: task, Destination: destination}, nil
}

func (service TaskService) List(_ context.Context) (TaskListResult, error) {
	root := filepath.Join(service.Runtime.Root(), "tasks")
	ids, err := service.Runtime.ListDirectories(root)
	if err != nil {
		return TaskListResult{}, fmt.Errorf("eval task list: %w", err)
	}
	sort.Strings(ids)
	result := TaskListResult{Root: root, Tasks: make([]TaskSummary, 0, len(ids))}
	for _, id := range ids {
		task, err := service.loadTask(id)
		if err != nil {
			result.Tasks = append(result.Tasks, TaskSummary{ID: id, Error: err.Error()})
			continue
		}
		result.Tasks = append(result.Tasks, TaskSummary{ID: id, Task: task})
	}
	return result, nil
}

func (service TaskService) Show(_ context.Context, id string) (*evalsubstrate.Task, error) {
	return service.loadTask(id)
}

func (service TaskService) Run(_ context.Context, request TaskRunRequest) (TaskRunResult, error) {
	task, err := service.loadTask(request.TaskID)
	if err != nil {
		return TaskRunResult{}, err
	}
	if request.SuiteRef == "" {
		return TaskRunResult{}, fmt.Errorf("eval task run: --suite is required")
	}
	suite, err := service.loadSuite(request.SuiteRef)
	if err != nil {
		return TaskRunResult{}, err
	}
	seeds, err := parseTaskSeeds(request.Seeds)
	if err != nil {
		return TaskRunResult{}, err
	}
	if len(seeds) < 3 {
		return TaskRunResult{}, fmt.Errorf("eval task run: --seeds requires >=3 values, got %d (per §4 manifest required field)", len(seeds))
	}
	gateInputs := evalsubstrate.GateInputs{Suite: suite, Task: task, AllowWeak: request.AllowWeak, GTRequested: request.GroundTruthRef}
	if request.HarnessDir != "" {
		dir := request.HarnessDir
		if strings.HasPrefix(dir, "~") {
			if home, homeErr := service.Runtime.UserHome(); homeErr == nil {
				dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
			}
		}
		harness, lock, err := service.Runtime.SnapshotHarness(dir, request.HarnessRef, "ao eval task run")
		if err != nil {
			return TaskRunResult{}, fmt.Errorf("eval task run: harness snapshot: %w", err)
		}
		gateInputs.Harness, gateInputs.HarnessLock, gateInputs.HarnessDir = harness, lock, dir
	}
	var groundTruth []evalsubstrate.GroundTruthRow
	if request.GroundTruthRef != "" {
		groundTruth, _ = service.loadGroundTruth()
		gateInputs.GroundTruth = groundTruth
	}
	refusals := evalsubstrate.RunGates(gateInputs)
	if !refusals.Empty() {
		return TaskRunResult{}, &TaskGateError{Message: refusals.Format(), Count: len(refusals)}
	}
	if request.DryRun {
		return TaskRunResult{DryRun: true}, nil
	}
	rigID := request.RigID
	if rigID == "" {
		rigID = "unknown-rig"
	}
	manifest := service.taskManifest(request, task, suite, seeds, rigID, gateInputs, groundTruth)
	runID := service.Runtime.GenerateRunID(rigID)
	writer, err := service.Runtime.OpenRun(service.Runtime.Root(), runID, manifest)
	if err != nil {
		return TaskRunResult{}, fmt.Errorf("eval task run: open run: %w", err)
	}
	if err := writer.Transition(evalsubstrate.StatusRunning, func(manifest *evalsubstrate.Manifest) {
		manifest.ValidityGatesPassed = []string{"held_constant_declared", "min_n_samples", "ground_truth_immutable", "harness_lock_match", "multi_comparison_correction"}
	}); err != nil {
		return TaskRunResult{}, fmt.Errorf("eval task run: transition->running: %w", err)
	}
	return TaskRunResult{Manifest: writer.Manifest(), Path: writer.Path()}, nil
}

type TaskGateError struct {
	Message string
	Count   int
}

func (failure *TaskGateError) Error() string {
	return fmt.Sprintf("eval task run: %d gate refusal(s)", failure.Count)
}

func (service TaskService) loadTask(id string) (*evalsubstrate.Task, error) {
	path := filepath.Join(service.Runtime.Root(), "tasks", id, "task.yaml")
	raw, err := service.Runtime.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loadTask %q: %w", id, err)
	}
	var task evalsubstrate.Task
	if err := yaml.Unmarshal(raw, &task); err != nil {
		return nil, fmt.Errorf("loadTask %q: parse: %w", id, err)
	}
	return &task, nil
}

func (service TaskService) loadSuite(ref string) (*evalsubstrate.Suite, error) {
	path := ref
	if !strings.HasSuffix(ref, ".yaml") && !strings.HasSuffix(ref, ".yml") {
		path = filepath.Join(service.Runtime.Root(), "suites", ref, "suite.yaml")
	}
	raw, err := service.Runtime.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loadSuite %q: %w", ref, err)
	}
	var suite evalsubstrate.Suite
	if err := yaml.Unmarshal(raw, &suite); err != nil {
		return nil, fmt.Errorf("loadSuite %q: parse: %w", ref, err)
	}
	return &suite, nil
}

func (service TaskService) loadGroundTruth() ([]evalsubstrate.GroundTruthRow, error) {
	raw, err := service.Runtime.ReadFile(filepath.Join(service.Runtime.Root(), "ground-truth", "ground-truth.jsonl"))
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var rows []evalsubstrate.GroundTruthRow
	for decoder.More() {
		var row evalsubstrate.GroundTruthRow
		if err := decoder.Decode(&row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (service TaskService) taskManifest(request TaskRunRequest, task *evalsubstrate.Task, suite *evalsubstrate.Suite, seeds []int, rigID string, gates evalsubstrate.GateInputs, rows []evalsubstrate.GroundTruthRow) evalsubstrate.Manifest {
	manifest := evalsubstrate.Manifest{TaskRef: task.ID, SuiteRef: suite.ID, HarnessRef: request.HarnessRef, ModelSpecRef: request.ModelSpecID, GroundTruthRef: request.GroundTruthRef, SampleSplit: pickTaskSplit(suite, request.SampleSplit), NSamples: pickTaskSamples(suite, request.NSamples), Seeds: seeds, RigID: rigID, InspectCommand: request.InspectCommand, InspectVersion: request.InspectVersion, QuickSession: request.QuickSession}
	if gates.Harness != nil {
		manifest.HarnessContentHash = gates.Harness.ContentHash
	}
	if request.ModelSpecID != "" {
		if spec, err := service.Runtime.LoadModelSpec(service.Runtime.Root(), request.ModelSpecID); err == nil {
			manifest.ModelSpecHash = spec.ContentHash
		}
	}
	manifest.GroundTruthHash = taskGroundTruthHash(rows, request.GroundTruthRef)
	if count := len(suite.VariedAxis.Values); count > 2 {
		manifest.MultiComparisonMethod, manifest.ComparisonFamily, manifest.ReferenceArm, manifest.FamilySizeK = suite.Stats.MultiComparisonMethod, suite.Stats.ComparisonFamily, suite.Stats.ReferenceArm, evalsubstrate.FamilySizeK(suite.Stats.ComparisonFamily, count)
	}
	return manifest
}

func parseTaskSeeds(value string) ([]int, error) {
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	seeds := make([]int, 0, len(parts))
	for _, part := range parts {
		var seed int
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &seed); err != nil {
			return nil, fmt.Errorf("parseSeeds: bad seed %q: %w", part, err)
		}
		seeds = append(seeds, seed)
	}
	return seeds, nil
}
func pickTaskSplit(suite *evalsubstrate.Suite, override string) string {
	if override != "" {
		return override
	}
	if suite != nil && suite.SampleSplit != "" {
		return suite.SampleSplit
	}
	return "dev"
}
func pickTaskSamples(suite *evalsubstrate.Suite, override int) int {
	if override > 0 {
		return override
	}
	if suite != nil {
		return suite.NSamples
	}
	return 0
}
func taskGroundTruthHash(rows []evalsubstrate.GroundTruthRow, ref string) string {
	if ref == "" || len(rows) == 0 {
		return ""
	}
	var data []byte
	for _, row := range rows {
		if row.ID == ref || row.Supersedes == ref {
			encoded, _ := json.Marshal(row)
			canonical, err := evalsubstrate.CanonicalizeJSON(encoded)
			if err != nil {
				canonical = encoded
			}
			data = append(data, canonical...)
		}
	}
	if len(data) == 0 {
		return ""
	}
	return evalsubstrate.ContentHash(data)
}
