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
	"fmt"
	"time"

	"github.com/spf13/cobra"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

var (
	beadsStaleThresholdHours float64
	beadsStaleJSON           bool
	beadsStaleNowOverride    string // used by tests to make detected_at deterministic
)

// staleBeadRecord is the subset of `br list --json` output we care about.
type staleBeadRecord = beadsapp.StaleBeadRecord

// staleEvent mirrors schemas/stale-claim-event.v1.schema.json for
// event_type="stale_detected". JSON tags lowercase + snake_case to match.
type staleEvent = beadsapp.StaleEvent
type staleAgent = beadsapp.StaleAgent
type staleEvidence = beadsapp.StaleEvidence

func executeBeadsStale(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	now := time.Now().UTC()
	if beadsStaleNowOverride != "" {
		parsed, err := time.Parse(time.RFC3339, beadsStaleNowOverride)
		if err != nil {
			return fmt.Errorf("invalid now-override: %w", err)
		}
		now = parsed.UTC()
	}

	events, err := beadsapp.DetectStale(ctx, beadsapp.StaleSourceFunc(beadsStaleFetchCmd), now, beadsStaleThresholdHours)
	if err != nil {
		return fmt.Errorf("br list: %w", err)
	}

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
	return currentBeadsTracker().ListInProgress(ctx)
}

// computeStaleEvents derives the stale_detected event set for a slice of
// in_progress beads. Pure function — no exec, no clock, no FS — so it's
// trivially table-testable.
func computeStaleEvents(beads []staleBeadRecord, now time.Time, thresholdHours float64) []staleEvent {
	return beadsapp.ComputeStaleEvents(beads, now, thresholdHours)
}
