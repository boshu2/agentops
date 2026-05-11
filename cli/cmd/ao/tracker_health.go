// practices: [dora-metrics, distributed-tracing]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const trackerProbeTimeout = 5 * time.Second

type trackerHealth struct {
	Command string `json:"command,omitempty"`
	Mode    string `json:"mode"`
	Healthy bool   `json:"healthy"`
	Reason  string `json:"reason,omitempty"`
	Error   string `json:"error,omitempty"`
}

func detectTrackerHealth(command string, lookPathFn gcLookFn) trackerHealth {
	command = effectiveBDCommand(command)
	executable, _ := splitRuntimeCommand(command)
	if executable == "" {
		return trackerHealth{
			Command: command,
			Mode:    "tasklist",
			Healthy: false,
			Reason:  "tracker command is empty",
			Error:   "no bd command configured",
		}
	}

	if _, err := defaultLookPath(lookPathFn)(executable); err != nil {
		return trackerHealth{
			Command: command,
			Mode:    "tasklist",
			Healthy: false,
			Reason:  "bd executable not found",
			Error:   err.Error(),
		}
	}

	probes := [][]string{
		{"ready", "--json"},
		{"list", "--type", "epic", "--status", "open", "--json"},
	}

	for _, probeArgs := range probes {
		if err := runTrackerProbe(command, probeArgs...); err != nil {
			return trackerHealth{
				Command: command + " " + strings.Join(probeArgs, " "),
				Mode:    "tasklist",
				Healthy: false,
				Reason:  "tracker probe failed",
				Error:   err.Error(),
			}
		}
	}

	return trackerHealth{
		Command: command,
		Mode:    "beads",
		Healthy: true,
		Reason:  "tracker probes succeeded",
	}
}

func runTrackerProbe(command string, args ...string) error {
	executable, prefixArgs := splitRuntimeCommand(command)
	if executable == "" {
		return fmt.Errorf("empty tracker command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), trackerProbeTimeout)
	defer cancel()

	cmdArgs := append([]string{}, prefixArgs...)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, executable, cmdArgs...)
	cmd.Env = cleanEnvNoClaude()
	out, err := cmd.CombinedOutput()
	if context.Cause(ctx) == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("timed out after %s", trackerProbeTimeout)
	}
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, trimmed)
	}

	if len(out) == 0 {
		return fmt.Errorf("empty JSON output")
	}

	var payload any
	if err := json.Unmarshal(out, &payload); err != nil {
		return fmt.Errorf("invalid JSON output: %w", err)
	}
	return nil
}
