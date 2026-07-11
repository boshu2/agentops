// `ao beads resume <id>` — slice 3 of soc-vuu6.27 (fungible-swarm death
// recovery). Atomically transfers an in_progress claim from a previous
// (likely stale) agent to the current one via `br update <id> --claim`,
// then appends a stale-claim-event (event_type="claim_transferred") to
// docs/provenance/ledger.jsonl so the audit trail records who picked up
// whose work.
//
// Slice 2 (`stale-claims`) surfaces candidates. This slice acts on them.
// Slice 4 (daemon job) will wrap both for periodic re-dispatch.
//
// practices: [agile-manifesto, continuous-delivery, dora-metrics]

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

var (
	beadsResumeAgentID     string
	beadsResumeLedgerPath  string
	beadsResumeJSON        bool
	beadsResumeNowOverride string // test seam
)

var beadsResumeCmd = &cobra.Command{
	Use:   "resume <bead-id>",
	Short: "Atomically transfer an in_progress claim from a stale agent to this one",
	Long: `Transfers a stale claim via 'br update <bead-id> --claim', then appends a
claim_transferred event (matching schemas/stale-claim-event.v1.schema.json)
to docs/provenance/ledger.jsonl. The bead's prior + new revision (assignee
and updated_at hash) is captured in the event for audit.

Use 'ao beads stale-claims' (slice 2) to find candidates first.

--agent: explicit new claimant id. Defaults to BEADS_ACTOR env var, else "ao-beads-resume".
--ledger: provenance ledger path (default docs/provenance/ledger.jsonl).
--json: emit the event to stdout in addition to the ledger.`,
	Args: cobra.ExactArgs(1),
	RunE: runBeadsResume,
}

func init() {
	beadsResumeCmd.Flags().StringVar(&beadsResumeAgentID, "agent", "",
		"New claimant id (defaults to BEADS_ACTOR env var, else ao-beads-resume).")
	beadsResumeCmd.Flags().StringVar(&beadsResumeLedgerPath, "ledger",
		"docs/provenance/ledger.jsonl",
		"Path to the provenance ledger (relative to repo root).")
	beadsResumeCmd.Flags().BoolVar(&beadsResumeJSON, "json", false,
		"Emit the claim_transferred event to stdout (always written to ledger).")
}

// beadsResumeShowFunc is the test seam for fetching a bead's current state
// (assignee + updated_at) BEFORE the claim transfer, so we can record the
// prior revision. Production: shells out to `br show <id> --json`.
var beadsResumeShowFunc = func(ctx context.Context, beadID string) (staleBeadRecord, error) {
	out, err := beadsTrackerCommandContext(ctx, "show", beadID, "--json").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return staleBeadRecord{}, fmt.Errorf("br show %s exited %d: %s", beadID, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return staleBeadRecord{}, err
	}
	return beadsapp.ParseShownBead(out, beadID)
}

// beadsResumeClaimFunc is the test seam for performing the atomic update.
// Production: `br update <id> --claim --actor <agent>`.
var beadsResumeClaimFunc = func(ctx context.Context, beadID, agent string) error {
	args := []string{"update", beadID, "--claim"}
	if agent != "" {
		args = append(args, "--actor", agent)
	}
	cmd := beadsTrackerCommandContext(ctx, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("br update --claim failed: %w: %s", err, string(out))
	}
	return nil
}

// beadsResumeAppendLedger is the test seam for writing the provenance event.
// Production: appends one JSON object per line to the ledger file. Accepts
// `any` so the caller can pass the full claim_transferred shape (which
// extends staleEvent with new_claimant + transfer) without us introducing
// an interface.
var beadsResumeAppendLedger = func(ledgerPath string, event any) error {
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		return fmt.Errorf("mkdir ledger dir: %w", err)
	}
	f, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger: %w", err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	if err := enc.Encode(event); err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	return nil
}

func runBeadsResume(cmd *cobra.Command, args []string) error {
	beadID := args[0]
	if beadID == "" {
		return fmt.Errorf("bead id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Capture prior revision via `br show`.
	prior, err := beadsResumeShowFunc(ctx, beadID)
	if err != nil {
		return fmt.Errorf("fetch prior state: %w", err)
	}
	if prior.Status != "in_progress" {
		return fmt.Errorf("bead %s is %q, not in_progress — resume only handles in_progress claims", beadID, prior.Status)
	}

	// 2. Compute now (test override OK).
	now := time.Now().UTC()
	if beadsResumeNowOverride != "" {
		parsed, err := time.Parse(time.RFC3339, beadsResumeNowOverride)
		if err != nil {
			return fmt.Errorf("invalid now-override: %w", err)
		}
		now = parsed.UTC()
	}

	// 3. Resolve the new claimant id.
	agent := beadsResumeAgentID
	if agent == "" {
		agent = os.Getenv("BEADS_ACTOR")
	}
	if agent == "" {
		agent = "ao-beads-resume"
	}

	// 4. Perform the atomic claim transfer.
	if err := beadsResumeClaimFunc(ctx, beadID, agent); err != nil {
		return fmt.Errorf("claim transfer: %w", err)
	}

	// 5. Fetch posterior revision for the audit trail.
	posterior, err := beadsResumeShowFunc(ctx, beadID)
	if err != nil {
		// Claim succeeded but we can't read back — record what we know.
		posterior = staleBeadRecord{ID: beadID, Status: "in_progress", Assignee: agent, UpdatedAt: now.Format(time.RFC3339)}
	}

	// 6. Build + write the event.
	priorAgent := prior.Assignee
	if priorAgent == "" {
		priorAgent = "unknown"
	}
	transferred := beadsapp.BuildTransferredEvent(beadID, agent, prior, posterior, now)

	// 7. Resolve ledger path relative to repo root.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve cwd: %w", err)
	}
	root, err := repoRootForBeads(cwd)
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	ledger := beadsResumeLedgerPath
	if !filepath.IsAbs(ledger) {
		ledger = filepath.Join(root, ledger)
	}

	// Write the full claim_transferred shape (with new_claimant + transfer).
	if err := beadsResumeAppendLedger(ledger, transferred); err != nil {
		// Best-effort: include extra context but don't roll back the claim.
		return fmt.Errorf("append ledger (claim already transferred): %w", err)
	}

	// 8. Optional JSON-to-stdout.
	if beadsResumeJSON {
		raw, err := json.Marshal(transferred)
		if err != nil {
			return fmt.Errorf("marshal transferred claim: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(raw))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(),
			"ao beads resume: %s transferred from %q to %q (prior_rev=%s, new_rev=%s)\n",
			beadID, priorAgent, agent, transferred.Transfer.PriorRevision, transferred.Transfer.NewRevision)
	}
	return nil
}

// transferInfo mirrors the `transfer` sub-object in stale-claim-event.v1.
type transferInfo = beadsapp.TransferInfo

// fingerprint produces a compact, stable revision token from (assignee,
// updated_at). br itself does not expose an etag; (assignee, updated_at)
// changes on every claim/update so it serves as the audit fingerprint.
func fingerprint(r staleBeadRecord) string {
	return beadsapp.Fingerprint(r)
}
