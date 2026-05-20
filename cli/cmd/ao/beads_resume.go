// `ao beads resume <id>` — slice 3 of soc-vuu6.27 (fungible-swarm death
// recovery). Atomically transfers an in_progress claim from a previous
// (likely stale) agent to the current one via `bd update <id> --claim`,
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
	Long: `Transfers a stale claim via 'bd update <bead-id> --claim', then appends a
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
// prior revision. Production: shells out to `bd show <id> --json`.
var beadsResumeShowFunc = func(ctx context.Context, beadID string) (staleBeadRecord, error) {
	out, err := exec.CommandContext(ctx, "bd", "show", beadID, "--json").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return staleBeadRecord{}, fmt.Errorf("bd show %s exited %d: %s", beadID, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return staleBeadRecord{}, err
	}
	// bd show <id> --json may emit either an object or a 1-element array.
	trimmed := bytes_trim_leading_ws(out)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var arr []staleBeadRecord
		if err := json.Unmarshal(out, &arr); err != nil {
			return staleBeadRecord{}, fmt.Errorf("parse bd show array: %w", err)
		}
		if len(arr) == 0 {
			return staleBeadRecord{}, fmt.Errorf("bd show %s returned empty array", beadID)
		}
		return arr[0], nil
	}
	var rec staleBeadRecord
	if err := json.Unmarshal(out, &rec); err != nil {
		return staleBeadRecord{}, fmt.Errorf("parse bd show object: %w", err)
	}
	return rec, nil
}

// beadsResumeClaimFunc is the test seam for performing the atomic update.
// Production: `bd update <id> --claim --actor <agent>`.
var beadsResumeClaimFunc = func(ctx context.Context, beadID, agent string) error {
	args := []string{"update", beadID, "--claim"}
	if agent != "" {
		args = append(args, "--actor", agent)
	}
	cmd := exec.CommandContext(ctx, "bd", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd update --claim failed: %w: %s", err, string(out))
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
	defer f.Close()
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

	// 1. Capture prior revision via `bd show`.
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
	event := staleEvent{
		SchemaVersion:    1,
		EventType:        "claim_transferred",
		BeadID:           beadID,
		DetectedAt:       now.Format(time.RFC3339),
		OriginalClaimant: staleAgent{ID: priorAgent},
		Evidence: staleEvidence{
			LastTouchTS:       prior.UpdatedAt,
			LastEvidenceEvent: "bd update --claim",
		},
	}
	// Extra fields for the transferred variant are emitted via a wrapper —
	// stale_detected and claim_transferred share the same Go type plus
	// these post-fields.
	transferred := struct {
		staleEvent
		NewClaimant staleAgent   `json:"new_claimant"`
		Transfer    transferInfo `json:"transfer"`
	}{
		staleEvent:  event,
		NewClaimant: staleAgent{ID: agent},
		Transfer: transferInfo{
			PriorRevision: fingerprint(prior),
			NewRevision:   fingerprint(posterior),
			NotesAppended: false,
		},
	}

	// 7. Resolve ledger path relative to repo root.
	root, err := repoRootForBeads()
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
		raw, _ := json.Marshal(transferred)
		fmt.Fprintln(cmd.OutOrStdout(), string(raw))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(),
			"ao beads resume: %s transferred from %q to %q (prior_rev=%s, new_rev=%s)\n",
			beadID, priorAgent, agent, transferred.Transfer.PriorRevision, transferred.Transfer.NewRevision)
	}
	return nil
}

// transferInfo mirrors the `transfer` sub-object in stale-claim-event.v1.
type transferInfo struct {
	PriorRevision string `json:"prior_revision"`
	NewRevision   string `json:"new_revision"`
	NotesAppended bool   `json:"notes_appended"`
}

// fingerprint produces a compact, stable revision token from (assignee,
// updated_at). bd itself does not expose an etag; (assignee, updated_at)
// changes on every claim/update so it serves as the audit fingerprint.
func fingerprint(r staleBeadRecord) string {
	if r.Assignee == "" && r.UpdatedAt == "" {
		return "unset"
	}
	a := r.Assignee
	if a == "" {
		a = "_"
	}
	u := r.UpdatedAt
	if u == "" {
		u = "_"
	}
	return a + "@" + u
}

// repoRootForBeads finds the git repo root, falling back to cwd. Kept local
// so this file doesn't depend on internal helpers being in scope.
func repoRootForBeads() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return "", cwdErr
		}
		return cwd, nil
	}
	return string(bytes_trim_trailing_ws(out)), nil
}

// bytes_trim_leading_ws / trailing_ws — tiny local helpers to avoid pulling
// extra packages. Whitespace-trim only (no full unicode).
func bytes_trim_leading_ws(b []byte) []byte {
	i := 0
	for i < len(b) {
		c := b[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		break
	}
	return b[i:]
}
func bytes_trim_trailing_ws(b []byte) []byte {
	j := len(b)
	for j > 0 {
		c := b[j-1]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			j--
			continue
		}
		break
	}
	return b[:j]
}
