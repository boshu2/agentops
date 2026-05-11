// practices: [agile-manifesto, dora-metrics]
package main

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// --- Phase summaries ---

// writePhaseSummary writes a fallback summary only if Claude didn't write one.
func writePhaseSummary(cwd string, state *phasedState, phaseNum int) {
	rpiDir := filepath.Join(cwd, ".agents", "rpi")
	path := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-summary.md", phaseNum))

	// If Claude already wrote a summary, keep it (it's richer than our mechanical one)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("Phase %d: Claude-written summary found\n", phaseNum)
		return
	}
	fmt.Printf("Phase %d: no Claude summary found, writing fallback\n", phaseNum)

	if err := os.MkdirAll(rpiDir, 0750); err != nil {
		VerbosePrintf("Warning: could not create rpi dir for summary: %v\n", err)
		return
	}

	summary := generatePhaseSummary(state, phaseNum)
	if summary == "" {
		return
	}

	if err := os.WriteFile(path, []byte(summary), 0600); err != nil {
		VerbosePrintf("Warning: could not write phase summary: %v\n", err)
	}
}

// generatePhaseSummary produces a concise summary of what a phase accomplished.
func generatePhaseSummary(state *phasedState, phaseNum int) string {
	switch phaseNum {
	case 1: // Discovery (research + plan + pre-mortem)
		summary := fmt.Sprintf("Discovery completed for goal: %s\n", state.Goal)
		summary += "Research: see .agents/research/ for findings.\n"
		if state.EpicID != "" {
			summary += fmt.Sprintf("Plan: epic %s", state.EpicID)
			if state.FastPath {
				summary += " (micro-epic, fast path)"
			}
			summary += "\n"
		}
		verdict := state.Verdicts["pre_mortem"]
		if verdict != "" {
			summary += fmt.Sprintf("Pre-mortem verdict: %s\nSee .agents/council/*pre-mortem*.md for details.", verdict)
		}
		return summary
	case 2: // Implementation (crank)
		return fmt.Sprintf("Crank completed for epic %s.\nCheck bd children %s for issue statuses.", state.EpicID, state.EpicID)
	case 3: // Validation (vibe + post-mortem)
		summary := ""
		vibeVerdict := state.Verdicts["vibe"]
		if vibeVerdict != "" {
			summary += fmt.Sprintf("Vibe verdict: %s\nSee .agents/council/*vibe*.md for details.\n", vibeVerdict)
		}
		pmVerdict := state.Verdicts["post_mortem"]
		if pmVerdict != "" {
			summary += fmt.Sprintf("Post-mortem verdict: %s\n", pmVerdict)
		}
		summary += "See .agents/council/*post-mortem*.md and .agents/learnings/ for extracted knowledge."
		return summary
	}
	return ""
}

// handoffDetected checks if a phase wrote a handoff file (context degradation signal).
func handoffDetected(cwd string, phaseNum int) bool {
	path := filepath.Join(cwd, ".agents", "rpi", fmt.Sprintf("phase-%d-handoff.json", phaseNum))
	_, err := os.Stat(path)
	return err == nil
}

// cleanPhaseSummaries removes stale phase summaries and handoffs from a prior run.
func cleanPhaseSummaries(stateDir string) {
	for i := 1; i <= len(phases); i++ {
		path := filepath.Join(stateDir, fmt.Sprintf("phase-%d-summary.md", i))
		os.Remove(path) //nolint:errcheck // #nosec G104
		handoffPath := filepath.Join(stateDir, fmt.Sprintf("phase-%d-handoff.md", i))
		os.Remove(handoffPath) //nolint:errcheck // #nosec G104
		jsonHandoffPath := filepath.Join(stateDir, fmt.Sprintf("phase-%d-handoff.json", i))
		os.Remove(jsonHandoffPath) //nolint:errcheck // #nosec G104
		resultPath := filepath.Join(stateDir, fmt.Sprintf("phase-%d-result.json", i))
		os.Remove(resultPath) //nolint:errcheck // #nosec G104
	}
}

// --- Ratchet and logging ---

// recordRatchetCheckpoint records a ratchet checkpoint for a phase.
func recordRatchetCheckpoint(step, aoCommand string) {
	cmd := exec.Command(effectiveAOCommand(aoCommand), "ratchet", "record", step)
	if err := cmd.Run(); err != nil {
		VerbosePrintf("Warning: ratchet record %s: %v\n", step, err)
	}
}

// logPhaseTransition appends a log entry to the orchestration log.
func logPhaseTransition(logPath, runID, phase, details string) {
	var entry string
	if runID != "" {
		entry = fmt.Sprintf("[%s] [%s] %s: %s\n", time.Now().Format(time.RFC3339), runID, phase, details)
	} else {
		entry = fmt.Sprintf("[%s] %s: %s\n", time.Now().Format(time.RFC3339), phase, details)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		VerbosePrintf("Warning: could not write orchestration log: %v\n", err)
		return
	}
	defer func() { _ = f.Close() }() //nolint:errcheck

	if _, err := f.WriteString(entry); err != nil {
		VerbosePrintf("Warning: could not write log entry: %v\n", err)
		return
	}

	// Mirror transitions into append-only ledger and refresh per-run cache.
	// This keeps mutable status files as cache while preserving provenance.
	maybeAppendRPILedgerTransition(logPath, runID, phase, details)
}

func maybeAppendRPILedgerTransition(logPath, runID, phase, details string) {
	if runID == "" {
		return
	}
	rootDir, ok := deriveRepoRootFromRPIOrchestrationLog(logPath)
	if !ok {
		return
	}

	event := rpiLedgerEvent{
		RunID:  runID,
		Phase:  phase,
		Action: ledgerActionFromDetails(details),
		Details: map[string]any{
			"details": details,
		},
	}

	if _, err := appendRPILedgerEvent(rootDir, event); err != nil {
		VerbosePrintf("Warning: could not append RPI ledger event: %v\n", err)
		return
	}
	if err := materializeRPIRunCache(rootDir, runID); err != nil {
		VerbosePrintf("Warning: could not materialize RPI run cache: %v\n", err)
	}
}

func deriveRepoRootFromRPIOrchestrationLog(logPath string) (string, bool) {
	rpiDir := filepath.Dir(filepath.Clean(logPath))
	if filepath.Base(rpiDir) != "rpi" {
		return "", false
	}
	agentsDir := filepath.Dir(rpiDir)
	if filepath.Base(agentsDir) != ".agents" {
		return "", false
	}
	return filepath.Dir(agentsDir), true
}

var ledgerPrefixActions = []struct {
	prefix string
	action string
}{
	{"started", "started"},
	{"completed", "completed"},
	{"failed:", "failed"},
	{"fatal:", "fatal"},
	{"retry", "retry"},
	{"dry-run", "dry-run"},
	{"handoff", "handoff"},
	{"epic=", "summary"},
}

func ledgerActionFromDetails(details string) string {
	normalized := strings.ToLower(strings.TrimSpace(details))
	if normalized == "" {
		return "event"
	}
	for _, pa := range ledgerPrefixActions {
		if strings.HasPrefix(normalized, pa.prefix) {
			return pa.action
		}
	}
	fields := strings.Fields(normalized)
	return cmp.Or(strings.Trim(fields[0], ":"), "event")
}

// logFailureContext records actionable remediation context when a phase fails.
func logFailureContext(logPath, runID, phase string, err error) {
	logPhaseTransition(logPath, runID, phase, fmt.Sprintf("FAILURE_CONTEXT: %v | action: check .agents/rpi/ for phase artifacts, review .agents/council/ for verdicts", err))
}
