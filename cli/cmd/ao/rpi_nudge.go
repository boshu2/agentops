package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

var (
	rpiNudgeRunID      string
	rpiNudgePhase      int
	rpiNudgeAllWorkers bool
	rpiNudgeWorker     int
	rpiNudgeMessage    string
)

func init() {
	nudgeCmd := &cobra.Command{
		Use:   "nudge [message]",
		Short: "Send a mid-flight nudge into tmux-backed RPI phase sessions",
		Long: `Send a message to an active tmux-backed RPI phase session.

Targets may be the mayor session (default), one worker (--worker), or every
worker session for a phase (--all-workers).`,
		RunE: runRPINudge,
	}
	nudgeCmd.Flags().StringVar(&rpiNudgeRunID, "run-id", "", "Run ID to target (defaults to latest phased state)")
	nudgeCmd.Flags().IntVar(&rpiNudgePhase, "phase", 0, "Phase number to target (1-3; defaults to run's current phase)")
	nudgeCmd.Flags().BoolVar(&rpiNudgeAllWorkers, "all-workers", false, "Send nudge to every worker session for the phase")
	nudgeCmd.Flags().IntVar(&rpiNudgeWorker, "worker", 0, "Send nudge to one worker (for example: 1)")
	nudgeCmd.Flags().StringVar(&rpiNudgeMessage, "message", "", "Nudge message to send")
	rpiCmd.AddCommand(nudgeCmd)
}

// rpiNudgeRecord is a thin alias for the internal NudgeRecord type.
type rpiNudgeRecord = cliRPI.NudgeRecord

type rpiNudgeContext struct {
	runID        string
	phase        int
	root         string
	tmuxBin      string
	phaseSession string
	targets      []string
}

func runRPINudge(cmd *cobra.Command, args []string) error {
	if err := validateRPINudgeTargetSelection(); err != nil {
		return err
	}
	message, err := resolveRPINudgeMessage(args)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	nudgeCtx, err := prepareRPINudgeContext(cwd)
	if err != nil {
		return err
	}
	if err := executeRPINudge(nudgeCtx, message); err != nil {
		return err
	}
	fmt.Printf("Nudged %d session(s): %s\n", len(nudgeCtx.targets), strings.Join(nudgeCtx.targets, ", "))
	return nil
}

func validateRPINudgeTargetSelection() error {
	if rpiNudgeAllWorkers && rpiNudgeWorker > 0 {
		return fmt.Errorf("use either --all-workers or --worker, not both")
	}
	return nil
}

func resolveRPINudgeMessage(args []string) (string, error) {
	message := strings.TrimSpace(rpiNudgeMessage)
	if message == "" && len(args) > 0 {
		message = strings.TrimSpace(strings.Join(args, " "))
	}
	if message == "" {
		return "", fmt.Errorf("provide a nudge message via --message or positional text")
	}
	return message, nil
}

func prepareRPINudgeContext(cwd string) (rpiNudgeContext, error) {
	runID, state, root, err := resolveNudgeRun(cwd, strings.TrimSpace(rpiNudgeRunID))
	if err != nil {
		return rpiNudgeContext{}, err
	}
	phase, err := resolveNudgePhase(state, rpiNudgePhase)
	if err != nil {
		return rpiNudgeContext{}, err
	}
	tmuxBin, err := resolveRPINudgeTmuxBinary()
	if err != nil {
		return rpiNudgeContext{}, err
	}
	phaseSession := tmuxSessionName(runID, phase)
	sessions, err := listTmuxSessions(tmuxBin)
	if err != nil {
		return rpiNudgeContext{}, fmt.Errorf("list tmux sessions: %w", err)
	}
	targets, err := resolveNudgeTargets(sessions, phaseSession, rpiNudgeAllWorkers, rpiNudgeWorker)
	if err != nil {
		return rpiNudgeContext{}, err
	}
	return rpiNudgeContext{
		runID:        runID,
		phase:        phase,
		root:         root,
		tmuxBin:      tmuxBin,
		phaseSession: phaseSession,
		targets:      targets,
	}, nil
}

func resolveRPINudgeTmuxBinary() (string, error) {
	toolchain, tcErr := resolveRPIToolchainDefaults()
	tmuxCommand := "tmux"
	if tcErr == nil && strings.TrimSpace(toolchain.TmuxCommand) != "" {
		tmuxCommand = strings.TrimSpace(toolchain.TmuxCommand)
	}
	tmuxBin, err := defaultLookPath(nil)(tmuxCommand)
	if err != nil {
		return "", fmt.Errorf("tmux binary %q not found: %w", tmuxCommand, err)
	}
	return tmuxBin, nil
}

func executeRPINudge(nudgeCtx rpiNudgeContext, message string) error {
	commandRecord, err := appendRPIC2Command(nudgeCtx.root, rpiC2CommandInput{
		RunID:    nudgeCtx.runID,
		Phase:    nudgeCtx.phase,
		Kind:     "nudge",
		Targets:  append([]string(nil), nudgeCtx.targets...),
		Message:  message,
		Deadline: time.Now().UTC().Add(30 * time.Second),
		Metadata: map[string]any{
			"phase_session": nudgeCtx.phaseSession,
			"all_workers":   rpiNudgeAllWorkers,
			"worker":        rpiNudgeWorker,
		},
	})
	if err != nil {
		return fmt.Errorf("append command log: %w", err)
	}
	if err := deliverRPINudges(nudgeCtx, message, commandRecord.CommandID); err != nil {
		return err
	}
	record := rpiNudgeRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		RunID:     nudgeCtx.runID,
		Phase:     nudgeCtx.phase,
		Targets:   append([]string(nil), nudgeCtx.targets...),
		Message:   message,
	}
	if err := appendRPINudgeAudit(nudgeCtx.root, nudgeCtx.runID, record); err != nil {
		VerbosePrintf("Warning: could not write nudge audit: %v\n", err)
	}
	return nil
}

func deliverRPINudges(nudgeCtx rpiNudgeContext, message, commandID string) error {
	for _, target := range nudgeCtx.targets {
		if err := sendTmuxNudge(nudgeCtx.tmuxBin, target, message); err != nil {
			emitErr := appendRPINudgeC2Event(nudgeCtx.root, nudgeCtx.runID, nudgeCtx.phase, commandID, target, "failed", err.Error())
			if emitErr != nil {
				VerbosePrintf("Warning: could not append failed nudge event: %v\n", emitErr)
			}
			return err
		}
		if err := appendRPINudgeC2Event(nudgeCtx.root, nudgeCtx.runID, nudgeCtx.phase, commandID, target, "ack", "nudge delivered"); err != nil {
			VerbosePrintf("Warning: could not append ack nudge event: %v\n", err)
		}
	}
	return nil
}

func appendRPINudgeC2Event(root, runID string, phase int, commandID, target, status, message string) error {
	eventType := "command.nudge." + strings.TrimSpace(status)
	_, err := appendRPIC2Event(root, rpiC2EventInput{
		RunID:     runID,
		CommandID: commandID,
		Phase:     phase,
		Backend:   "tmux",
		Source:    "rpi_nudge",
		Type:      eventType,
		Message:   message,
		Details: map[string]any{
			"target": target,
			"status": strings.TrimSpace(status),
		},
	})
	return err
}

func resolveNudgeRun(cwd, requestedRunID string) (runID string, state *phasedState, root string, err error) {
	runID = strings.TrimSpace(requestedRunID)
	if runID != "" {
		state, root, err = locateRunMetadata(cwd, runID)
		if err != nil {
			return "", nil, "", fmt.Errorf("locate run %s: %w", runID, err)
		}
		return runID, state, root, nil
	}

	state, err = loadPhasedState(cwd)
	if err != nil {
		return "", nil, "", fmt.Errorf("load latest phased state (or pass --run-id): %w", err)
	}
	runID = strings.TrimSpace(state.RunID)
	if runID == "" {
		return "", nil, "", fmt.Errorf("latest phased state has no run_id; pass --run-id")
	}
	state, root, err = locateRunMetadata(cwd, runID)
	if err != nil {
		return "", nil, "", fmt.Errorf("locate latest run %s: %w", runID, err)
	}
	return runID, state, root, nil
}

func resolveNudgePhase(state *phasedState, requestedPhase int) (int, error) {
	statePhase := 0
	if state != nil {
		statePhase = state.Phase
	}
	return cliRPI.ResolveNudgePhase(statePhase, requestedPhase)
}

func resolveNudgeTargets(sessions []string, phaseSession string, allWorkers bool, worker int) ([]string, error) {
	return cliRPI.ResolveNudgeTargets(sessions, phaseSession, allWorkers, worker)
}

func appendRPINudgeAudit(root, runID string, record rpiNudgeRecord) error {
	runDir := rpiRunRegistryDir(root, runID)
	return cliRPI.AppendNudgeAudit(runDir, record)
}
