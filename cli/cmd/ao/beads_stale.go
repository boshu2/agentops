// Package main / cmd ao.
//
// `ao beads stale-claims` — slice 2 of soc-vuu6.27 (fungible-swarm death
// recovery). Reads `br list --status in_progress --json`, derives a
// staleness signal per bead from its last activity timestamp, and emits a
// table or a JSON record array conforming to
// schemas/stale-claim-event.v1.schema.json (event_type: "stale_detected").
//
// Read-only. Slice 3 (`ao beads resume`) handles the atomic transfer.
//
// practices: [agile-manifesto, dora-metrics]

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

var (
	beadsStaleThresholdHours float64
	beadsStaleJSON           bool
	beadsStaleNowOverride    string // used by tests to make detected_at deterministic
)

var beadsStaleCmd = &cobra.Command{
	Use:   "stale-claims",
	Short: "List in_progress beads whose claim looks stale",
	Long: `Lists in_progress beads whose claim activity is older than --threshold.

A bead is "stale" when its updated_at timestamp is older than the threshold
(default 4h). Future signals (worktree quietness, session-bootstrap heartbeat
expiry) will be added when the enabling primitives (soc-vuu6.25) ship.

Output: human-readable table by default, or JSON array (matching
schemas/stale-claim-event.v1.schema.json shape) with --json. Read-only:
this command DOES NOT transfer claims. Use 'ao beads resume <id>' for that
(soc-vuu6.27 slice 3).`,
	RunE: runBeadsStale,
}

func init() {
	beadsStaleCmd.Flags().Float64Var(&beadsStaleThresholdHours, "threshold", 4.0,
		"Staleness threshold in hours (claim updated more than N hours ago).")
	beadsStaleCmd.Flags().BoolVar(&beadsStaleJSON, "json", false,
		"Emit JSON array conforming to stale-claim-event.v1 (event_type: stale_detected).")
}

// staleBeadRecord is the subset of `br list --json` output we care about.
type staleBeadRecord = beadsapp.StaleBeadRecord

// staleEvent mirrors schemas/stale-claim-event.v1.schema.json for
// event_type="stale_detected". JSON tags lowercase + snake_case to match.
type staleEvent = beadsapp.StaleEvent
type staleAgent = beadsapp.StaleAgent
type staleEvidence = beadsapp.StaleEvidence

func runBeadsStale(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	raw, err := beadsStaleFetchCmd(ctx)
	if err != nil {
		return fmt.Errorf("br list: %w", err)
	}
	var beads []staleBeadRecord
	if err := json.Unmarshal(raw, &beads); err != nil {
		return fmt.Errorf("parse br list: %w", err)
	}

	now := time.Now().UTC()
	if beadsStaleNowOverride != "" {
		parsed, err := time.Parse(time.RFC3339, beadsStaleNowOverride)
		if err != nil {
			return fmt.Errorf("invalid now-override: %w", err)
		}
		now = parsed.UTC()
	}

	events := computeStaleEvents(beads, now, beadsStaleThresholdHours)

	if beadsStaleJSON {
		out, err := json.Marshal(events)
		if err != nil {
			return fmt.Errorf("marshal events: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}

	// Human-readable.
	if len(events) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(),
			"ao beads stale-claims: none — all in_progress beads touched within %.1fh\n",
			beadsStaleThresholdHours)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"ao beads stale-claims: %d in_progress bead(s) stale (threshold %.1fh)\n",
		len(events), beadsStaleThresholdHours)
	for _, e := range events {
		fmt.Fprintf(cmd.OutOrStdout(),
			"  %-22s claim_age=%.1fh last_touch=%s claimant=%s\n",
			e.BeadID, e.Evidence.ClaimAgeHours, e.Evidence.LastTouchTS, e.OriginalClaimant.ID)
	}
	return nil
}

// beadsStaleFetchCmd is the seam for tests. Tests overwrite it to inject
// canned `br list` output without touching a real br binary.
var beadsStaleFetchCmd = func(ctx context.Context) ([]byte, error) {
	cmd := beadsTrackerCommandContext(ctx, "list", "--status", "in_progress", "--json", "--limit", "500")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("br list exited %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, err
	}
	return out, nil
}

// computeStaleEvents derives the stale_detected event set for a slice of
// in_progress beads. Pure function — no exec, no clock, no FS — so it's
// trivially table-testable.
func computeStaleEvents(beads []staleBeadRecord, now time.Time, thresholdHours float64) []staleEvent {
	return beadsapp.ComputeStaleEvents(beads, now, thresholdHours)
}
