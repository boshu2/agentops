package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type RPICLICommand struct {
	Root    string
	Script  string
	Args    []string
	Env     []string
	LogPath string
}

type RPICLICommandRunner func(context.Context, RPICLICommand) error

type RPICLIExecutorOptions struct {
	Root          string
	ScriptPath    string
	CommandRunner RPICLICommandRunner
}

type RPICLIExecutor struct {
	root          string
	scriptPath    string
	commandRunner RPICLICommandRunner
}

func NewRPICLIExecutor(opts RPICLIExecutorOptions) (*RPICLIExecutor, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		return nil, errors.New("daemon rpi cli executor: root is required")
	}
	scriptPath := strings.TrimSpace(opts.ScriptPath)
	if scriptPath == "" {
		scriptPath = filepath.Join(root, "scripts", "ao-rpi-autonomous-cycle.sh")
	}
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(root, scriptPath)
	}
	runner := opts.CommandRunner
	if runner == nil {
		runner = runRPICLICommand
	}
	return &RPICLIExecutor{
		root:          root,
		scriptPath:    filepath.Clean(scriptPath),
		commandRunner: runner,
	}, nil
}

func (e *RPICLIExecutor) JobTypes() []JobType {
	return []JobType{JobTypeRPIRun}
}

func (e *RPICLIExecutor) RunJob(ctx context.Context, claim QueueClaim) (JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if claim.Job.JobType != JobTypeRPIRun {
		return JobExecutionResult{}, fmt.Errorf("rpi cli executor does not support job type %s", claim.Job.JobType)
	}
	spec, err := RPIRunJobSpecFromPayload(claim.Job.Payload)
	if err != nil {
		return JobExecutionResult{}, err
	}
	artifacts := e.artifactsFor(claim, spec)
	if err := validateRPICLIFullRun(spec); err != nil {
		return JobExecutionResult{Artifacts: artifacts}, err
	}
	if err := validateRPICLIScript(e.scriptPath); err != nil {
		return JobExecutionResult{Artifacts: artifacts}, err
	}
	command := RPICLICommand{
		Root:    e.root,
		Script:  e.scriptPath,
		Args:    rpiCLIArgsForSpec(spec),
		Env:     rpiCLIEnvForClaim(claim, spec),
		LogPath: filepath.Join(e.root, filepath.FromSlash(artifacts["rpi_cli_log"])),
	}
	if err := e.commandRunner(ctx, command); err != nil {
		return JobExecutionResult{Artifacts: artifacts}, err
	}
	return JobExecutionResult{Artifacts: artifacts}, nil
}

func (e *RPICLIExecutor) artifactsFor(claim QueueClaim, spec RPIRunJobSpec) map[string]string {
	runID := sanitizeIDPart(spec.RunID)
	if runID == "" {
		runID = "unknown-run"
	}
	jobID := sanitizeIDPart(claim.Job.JobID)
	if jobID == "" {
		jobID = "unknown-job"
	}
	logPath := filepath.ToSlash(filepath.Join(".agents", "daemon", "rpi", "runs", runID, jobID, "rpi-cli.log"))
	return map[string]string{
		"executor_policy":     "cli-fallback",
		"backend":             string(RPIBackendGasCityCLI),
		"requested_backend":   string(spec.Backend),
		"run_id":              spec.RunID,
		"goal":                spec.Goal,
		"rpi_cli_script":      e.scriptPath,
		"rpi_cli_log":         logPath,
		"landing_policy":      "off",
		"rpi_wrapper_command": "ao rpi loop --supervisor",
	}
}

func validateRPICLIScript(scriptPath string) error {
	info, err := os.Stat(scriptPath)
	if err != nil {
		return fmt.Errorf("rpi cli executor script unavailable: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("rpi cli executor script is a directory: %s", scriptPath)
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("rpi cli executor script is not executable: %s", scriptPath)
	}
	return nil
}

func validateRPICLIFullRun(spec RPIRunJobSpec) error {
	if spec.StartPhase != 1 || spec.MaxPhase != 3 {
		return fmt.Errorf("rpi cli executor only supports full rpi.run cycles: start_phase=%d max_phase=%d", spec.StartPhase, spec.MaxPhase)
	}
	return nil
}

func rpiCLIArgsForSpec(spec RPIRunJobSpec) []string {
	args := []string{
		"--goal", spec.Goal,
		"--max-cycles", "1",
		"--gate-policy", "required",
		"--landing-policy", "off",
		"--failure-policy", "stop",
	}
	return args
}

func rpiCLIEnvForClaim(claim QueueClaim, spec RPIRunJobSpec) []string {
	env := []string{
		"AGENTOPS_DAEMON_JOB_ID=" + claim.Job.JobID,
		"AGENTOPS_DAEMON_REQUEST_ID=" + string(claim.Job.RequestID),
		"AGENTOPS_DAEMON_RPI_RUN_ID=" + spec.RunID,
		"AGENTOPS_DAEMON_RPI_BACKEND=" + string(RPIBackendGasCityCLI),
		"AGENTOPS_DAEMON_RPI_START_PHASE=" + strconv.Itoa(spec.StartPhase),
		"AGENTOPS_DAEMON_RPI_MAX_PHASE=" + strconv.Itoa(spec.MaxPhase),
	}
	if spec.EpicID != "" {
		env = append(env, "AGENTOPS_DAEMON_RPI_EPIC_ID="+spec.EpicID)
	}
	if spec.ExecutionPacketPath != "" {
		env = append(env, "AGENTOPS_DAEMON_RPI_EXECUTION_PACKET="+spec.ExecutionPacketPath)
	}
	return env
}

func runRPICLICommand(ctx context.Context, command RPICLICommand) error {
	if err := os.MkdirAll(filepath.Dir(command.LogPath), 0o700); err != nil {
		return fmt.Errorf("create rpi cli log dir: %w", err)
	}
	logFile, err := os.OpenFile(command.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open rpi cli log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.CommandContext(ctx, command.Script, command.Args...)
	cmd.Dir = command.Root
	cmd.Env = append(os.Environ(), command.Env...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run rpi cli executor: %w", err)
	}
	return nil
}

var _ JobExecutor = (*RPICLIExecutor)(nil)
