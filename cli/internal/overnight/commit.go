package overnight

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/daemon"
)

type CommitStageJobOptions struct {
	Spec                   daemon.DreamStageJobSpec
	RunOptions             RunLoopOptions
	Checkpoint             *Checkpoint
	CheckpointManifestPath string
	Log                    io.Writer
	Now                    func() time.Time
}

type CommitStageJobResult struct {
	SchemaVersion          int               `json:"schema_version"`
	DreamRunID             string            `json:"dream_run_id"`
	IterationID            string            `json:"iteration_id,omitempty"`
	Stage                  string            `json:"stage"`
	Status                 string            `json:"status"`
	Error                  string            `json:"error,omitempty"`
	OutputDir              string            `json:"output_dir"`
	ResultPath             string            `json:"result_path"`
	CheckpointManifestPath string            `json:"checkpoint_manifest_path"`
	CheckpointPath         string            `json:"checkpoint_path"`
	PrevDir                string            `json:"prev_dir"`
	LiveDir                string            `json:"live_dir"`
	MarkerPath             string            `json:"marker_path"`
	MarkerState            string            `json:"marker_state,omitempty"`
	RecoveryRequired       bool              `json:"recovery_required"`
	StartedAt              string            `json:"started_at"`
	CompletedAt            string            `json:"completed_at"`
	DurationMillis         int64             `json:"duration_millis"`
	Degraded               []string          `json:"degraded,omitempty"`
	StageFailures          map[string]string `json:"stage_failures,omitempty"`
}

func RunCommitStageJob(ctx context.Context, opts CommitStageJobOptions) (CommitStageJobResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spec := opts.Spec
	if err := spec.Validate(); err != nil {
		return CommitStageJobResult{}, err
	}
	if spec.Stage != daemon.DreamStageCommit {
		return CommitStageJobResult{}, fmt.Errorf("overnight/commit: stage job has stage %q, want %q", spec.Stage, daemon.DreamStageCommit)
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	started := now().UTC()
	runOpts := opts.RunOptions
	if runOpts.Cwd == "" {
		return CommitStageJobResult{}, fmt.Errorf("overnight/commit: stage job requires RunOptions.Cwd")
	}
	if runOpts.OutputDir == "" {
		runOpts.OutputDir = spec.OutputDir
	}
	if runOpts.OutputDir == "" {
		return CommitStageJobResult{}, fmt.Errorf("overnight/commit: stage job requires output_dir")
	}
	if runOpts.RunID == "" {
		runOpts.RunID = spec.DreamRunID
	}
	if err := ctxCheck(ctx); err != nil {
		return CommitStageJobResult{}, err
	}
	cp, manifestPath, err := commitStageCheckpoint(spec, opts)
	if err != nil {
		return CommitStageJobResult{}, err
	}
	if opts.Log != nil {
		fmt.Fprintf(opts.Log, "overnight/commit: checkpoint commit start (%s)\n", cp.IterationID)
	}
	commitErr := cp.Commit()
	if opts.Log != nil && commitErr == nil {
		fmt.Fprintf(opts.Log, "overnight/commit: checkpoint commit done (%s)\n", cp.IterationID)
	}
	completed := now().UTC()
	stageResult := buildCommitStageJobResult(spec, runOpts.OutputDir, manifestPath, cp, started, completed, commitErr)
	stageResult.ResultPath = CommitStageJobResultPath(runOpts.OutputDir)
	path, writeErr := WriteCommitStageJobResult(runOpts.OutputDir, stageResult)
	stageResult.ResultPath = path
	if commitErr != nil {
		if writeErr != nil {
			return stageResult, fmt.Errorf("%w; write commit stage result: %v", commitErr, writeErr)
		}
		return stageResult, commitErr
	}
	if writeErr != nil {
		return stageResult, writeErr
	}
	return stageResult, nil
}

func CommitStageJobResultPath(outputDir string) string {
	return filepath.Join(outputDir, "stages", "commit-result.json")
}

func WriteCommitStageJobResult(outputDir string, result CommitStageJobResult) (string, error) {
	if outputDir == "" {
		return "", fmt.Errorf("overnight/commit: output_dir is required")
	}
	path := CommitStageJobResultPath(outputDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("overnight/commit: mkdir stage result dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("overnight/commit: marshal stage result: %w", err)
	}
	data = append(data, '\n')
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return "", fmt.Errorf("overnight/commit: write stage result: %w", err)
	}
	return path, nil
}

func commitStageCheckpoint(spec daemon.DreamStageJobSpec, opts CommitStageJobOptions) (*Checkpoint, string, error) {
	if opts.Checkpoint != nil {
		manifestPath := opts.CheckpointManifestPath
		if manifestPath == "" {
			manifestPath = filepath.Join(spec.OutputDir, "stages", CheckpointManifestFileName)
		}
		if err := WriteCheckpointManifest(manifestPath, opts.Checkpoint.Manifest()); err != nil {
			return nil, "", err
		}
		return opts.Checkpoint, manifestPath, nil
	}
	manifestPath := firstNonEmptyString(opts.CheckpointManifestPath, spec.CheckpointDir)
	if manifestPath == "" {
		return nil, "", fmt.Errorf("overnight/commit: checkpoint manifest is required")
	}
	manifestPath = resolveCheckpointManifestPath(manifestPath)
	manifest, err := ReadCheckpointManifest(manifestPath)
	if err != nil {
		return nil, "", err
	}
	cp, err := CheckpointFromManifest(manifest)
	return cp, manifestPath, err
}

func buildCommitStageJobResult(
	spec daemon.DreamStageJobSpec,
	outputDir string,
	manifestPath string,
	cp *Checkpoint,
	started time.Time,
	completed time.Time,
	commitErr error,
) CommitStageJobResult {
	status := "completed"
	if commitErr != nil {
		status = "failed"
	}
	result := CommitStageJobResult{
		SchemaVersion:          1,
		DreamRunID:             spec.DreamRunID,
		IterationID:            spec.IterationID,
		Stage:                  string(daemon.DreamStageCommit),
		Status:                 status,
		OutputDir:              outputDir,
		CheckpointManifestPath: manifestPath,
		StartedAt:              started.UTC().Format(time.RFC3339Nano),
		CompletedAt:            completed.UTC().Format(time.RFC3339Nano),
		DurationMillis:         completed.Sub(started).Milliseconds(),
	}
	if cp == nil {
		if commitErr != nil {
			result.Error = commitErr.Error()
		}
		return result
	}
	result.CheckpointPath = cp.StagingDir
	result.PrevDir = cp.PrevDir
	result.LiveDir = cp.LiveDir
	result.MarkerPath = cp.MarkerPath
	state, stateErr := readMarkerState(cp.MarkerPath)
	if stateErr == nil {
		result.MarkerState = state
		result.RecoveryRequired = state == markerStateReady
	} else if commitErr != nil {
		result.StageFailures = map[string]string{"marker-state": stateErr.Error()}
	}
	if commitErr != nil {
		result.Error = commitErr.Error()
	}
	return result
}
