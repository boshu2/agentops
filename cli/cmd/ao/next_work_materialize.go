// Package main — `ao next-work materialize` subcommand (ag-9jle.3).
//
// Closes the missing half of the BDD-Gherkin-wave loop: lessons -> beads.
// Harvested follow-ups land in .agents/rpi/next-work.jsonl as a queue but never
// become durable beads, so the flywheel executes without compounding into the
// tracker. This command reads unmaterialized items from the queue and creates
// one durable bead per item via `bd create`, carrying provenance
// (source_epic + proof_ref) on the bead's native metadata.
//
// Design (locked, ag-9jle.3 + handoff 2026-05-30):
//   - CLI owns the deterministic core; skills (post-mortem harvest, crank
//     wave-close) just CALL `ao next-work materialize`.
//   - Idempotent: an item is materialized once. The per-item bead_id field
//     (rpi.NextWorkItem.BeadID) is the back-reference; a set bead_id means the
//     item already has a durable bead and is skipped on re-run.
//   - Provenance rides bd's native --metadata (no forked provenance format).
//     The actual graph edge is deferred to ag-x31t.4's future `ao provenance
//     add`; this command only records the anchor fields.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/spf13/cobra"
)

const (
	nextWorkQueueRel        = ".agents/rpi/next-work.jsonl"
	defaultMaterializedBy   = "next-work-materialize"
	materializeBeadLabels   = "next-work,materialized"
	materializeProvFromFile = "next-work.jsonl"
)

var (
	nextWorkMaterializeFile       string
	nextWorkMaterializeDryRun     bool
	nextWorkMaterializeJSON       bool
	nextWorkMaterializeSourceEpic string
	nextWorkMaterializeMaterialBy string
)

var nextWorkCmd = &cobra.Command{
	Use:   "next-work",
	Short: "Operate on the harvested next-work queue (.agents/rpi/next-work.jsonl)",
	Long: `Commands that act on the carry-forward next-work queue that /post-mortem
harvests into and /evolve and /rpi loop consume from.`,
}

var nextWorkMaterializeCmd = &cobra.Command{
	Use:   "materialize",
	Short: "Create durable beads from unmaterialized harvested follow-ups",
	Long: `Read .agents/rpi/next-work.jsonl and create one durable bead per
unmaterialized item via 'bd create', closing the lessons -> beads half of the
loop so harvested work compounds into the tracker instead of living only in a
queue.

Each created bead carries provenance (source_epic + proof_ref) on its native
bd metadata, plus the labels 'next-work,materialized'. The item's bead_id field
is set as a back-reference, so re-running is idempotent: already-materialized,
already-consumed, and held-for-review items are skipped.

When bd is not on PATH the command degrades gracefully (warns, exits 0) unless
--dry-run is set.

Examples:
  ao next-work materialize
  ao next-work materialize --dry-run
  ao next-work materialize --json
  ao next-work materialize --source-epic ag-9jle`,
	Args: cobra.NoArgs,
	RunE: runNextWorkMaterialize,
}

func init() {
	f := nextWorkMaterializeCmd.Flags()
	f.StringVar(&nextWorkMaterializeFile, "file", "", "Path to next-work.jsonl (default: <cwd>/.agents/rpi/next-work.jsonl)")
	f.BoolVar(&nextWorkMaterializeDryRun, "dry-run", false, "Show what would be created without creating beads or mutating the queue")
	f.BoolVar(&nextWorkMaterializeJSON, "json", false, "Emit a machine-readable JSON summary")
	f.StringVar(&nextWorkMaterializeSourceEpic, "source-epic", "", "Only materialize items whose batch source_epic equals this value")
	f.StringVar(&nextWorkMaterializeMaterialBy, "materialized-by", defaultMaterializedBy, "Actor recorded in provenance metadata")
	nextWorkCmd.AddCommand(nextWorkMaterializeCmd)
	rootCmd.AddCommand(nextWorkCmd)
}

// materializeCandidate is one queue item eligible to become a bead, tagged with
// the parseable-entry/item indices so the bead_id back-reference can be stamped.
type materializeCandidate struct {
	EntryIndex int
	ItemIndex  int
	SourceEpic string
	Item       nextWorkItem
}

// materializeResult is the per-item outcome surfaced to the operator and JSON.
type materializeResult struct {
	Title      string `json:"title"`
	SourceEpic string `json:"source_epic,omitempty"`
	BeadID     string `json:"bead_id,omitempty"`
	Status     string `json:"status"` // created | would-create | error
	Error      string `json:"error,omitempty"`
	entryIndex int
	itemIndex  int
}

// materializeProvenance is the JSON payload handed to `bd create --metadata`.
// It rides bd's native metadata field — NOT a forked provenance format. The
// real provenance edge is deferred to ag-x31t.4's `ao provenance add`.
type materializeProvenance struct {
	MaterializedFrom string            `json:"materialized_from"`
	SourceEpic       string            `json:"source_epic,omitempty"`
	HarvestSource    string            `json:"harvest_source,omitempty"`
	NextWorkType     string            `json:"nextwork_type,omitempty"`
	Severity         string            `json:"severity,omitempty"`
	ProofRef         *nextWorkProofRef `json:"proof_ref,omitempty"`
}

func runNextWorkMaterialize(cmd *cobra.Command, _ []string) error {
	path, err := resolveNextWorkPath(nextWorkMaterializeFile)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	if _, statErr := os.Stat(path); statErr != nil {
		fmt.Fprintf(out, "no next-work queue at %s — nothing to materialize\n", path)
		return nil
	}

	if !nextWorkMaterializeDryRun && !bdAvailable() {
		fmt.Fprintln(errOut, "WARN: bd not on PATH — skipping materialize (graceful degradation)")
		return nil
	}

	candidates, err := enumerateMaterializeCandidates(path, nextWorkMaterializeSourceEpic)
	if err != nil {
		return err
	}

	results := make([]materializeResult, 0, len(candidates))
	for _, c := range candidates {
		results = append(results, materializeOne(c, nextWorkMaterializeMaterialBy, nextWorkMaterializeDryRun))
	}

	if !nextWorkMaterializeDryRun {
		if err := stampBeadIDs(path, results); err != nil {
			return fmt.Errorf("stamp bead_id back-references: %w", err)
		}
	}

	return writeMaterializeSummary(out, results, nextWorkMaterializeDryRun, nextWorkMaterializeJSON)
}

// resolveNextWorkPath returns the explicit --file path or the cwd default.
func resolveNextWorkPath(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return filepath.Join(cwd, nextWorkQueueRel), nil
}

// enumerateMaterializeCandidates reads the queue and returns items eligible for
// materialization. Parseable entry indices come from forEachParseableNextWorkEntry
// (same rules as rewriteNextWorkFile). Legacy flat entries (no items[]) are skipped —
// the bead_id stamp writes into entry.Items by index, which requires items[].
func enumerateMaterializeCandidates(path, sourceEpicFilter string) ([]materializeCandidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var candidates []materializeCandidate
	err = forEachParseableNextWorkEntry(data, func(idx int, entry nextWorkEntry) error {
		if sourceEpicFilter != "" && entry.SourceEpic != sourceEpicFilter {
			return nil
		}
		// Consumption is recorded at the BATCH level in the real queue
		// (entry.consumed / consumed_by set after the items[] array); the
		// per-item consumed flag is rarely populated. Skip a batch-consumed
		// entry wholesale so historically-consumed work is not re-materialized
		// into stale duplicate beads.
		if isEntryConsumed(&entry) {
			return nil
		}
		for itemIdx, item := range entry.Items {
			if !isMaterializable(item) {
				continue
			}
			candidates = append(candidates, materializeCandidate{
				EntryIndex: idx,
				ItemIndex:  itemIdx,
				SourceEpic: entry.SourceEpic,
				Item:       item,
			})
		}
		return nil
	})
	return candidates, err
}

// isEntryConsumed reports whether a batch entry is already consumed at the
// entry level (the dominant case: the legacy evolve/rpi-loop consumer marks the
// whole entry consumed via entry.consumed / consumed_by). Items inside such an
// entry must not be materialized, even though their per-item consumed flags are
// unset. Mirrors the entry half of rpi.IsFullyConsumed.
func isEntryConsumed(entry *nextWorkEntry) bool {
	return entry.Consumed || rpi.NormalizeClaimStatus(entry.Consumed, entry.ClaimStatus) == "consumed"
}

// isMaterializable reports whether an item should become a durable bead.
// Already-materialized (bead_id set), already-consumed, held-for-review, and
// title-less items are skipped — this is what makes the command idempotent.
func isMaterializable(item nextWorkItem) bool {
	if strings.TrimSpace(item.BeadID) != "" {
		return false
	}
	if item.Consumed {
		return false
	}
	if strings.TrimSpace(item.Title) == "" {
		return false
	}
	return !rpi.IsQueueItemHeldForReview(item)
}

// materializeOne creates (or, in dry-run, plans) a single bead for a candidate.
func materializeOne(c materializeCandidate, materializedBy string, dryRun bool) materializeResult {
	res := materializeResult{
		Title:      c.Item.Title,
		SourceEpic: c.SourceEpic,
		entryIndex: c.EntryIndex,
		itemIndex:  c.ItemIndex,
	}
	args, err := buildBDCreateArgs(c, materializedBy)
	if err != nil {
		res.Status = "error"
		res.Error = err.Error()
		return res
	}
	if dryRun {
		res.Status = "would-create"
		return res
	}
	stdout, err := execBD(args...)
	if err != nil {
		res.Status = "error"
		res.Error = fmt.Sprintf("bd create: %v", err)
		return res
	}
	beadID := strings.TrimSpace(string(stdout))
	if beadID == "" {
		res.Status = "error"
		res.Error = "bd create returned an empty bead ID"
		return res
	}
	res.Status = "created"
	res.BeadID = beadID
	return res
}

// buildBDCreateArgs assembles the `bd create` argv for a candidate, including
// the native-metadata provenance payload and the materialized-from footer.
func buildBDCreateArgs(c materializeCandidate, materializedBy string) ([]string, error) {
	prov := materializeProvenance{
		MaterializedFrom: materializeProvFromFile,
		SourceEpic:       c.SourceEpic,
		HarvestSource:    c.Item.Source,
		NextWorkType:     c.Item.Type,
		Severity:         c.Item.Severity,
		ProofRef:         c.Item.ProofRef,
	}
	metaJSON, err := json.Marshal(prov)
	if err != nil {
		return nil, fmt.Errorf("marshal provenance metadata: %w", err)
	}
	desc := c.Item.Description + materializeProvenanceFooter(c, materializedBy)
	return []string{
		"create", c.Item.Title,
		"--type", mapNextWorkTypeToBeadType(c.Item.Type),
		"--priority", mapSeverityToPriority(c.Item.Severity),
		"--description", desc,
		"--labels", materializeBeadLabels,
		"--metadata", string(metaJSON),
		"--silent",
	}, nil
}

// materializeProvenanceFooter renders a human-readable provenance footer so the
// origin is visible in `bd show` even without inspecting metadata.
func materializeProvenanceFooter(c materializeCandidate, materializedBy string) string {
	epic := c.SourceEpic
	if epic == "" {
		epic = "(none)"
	}
	src := c.Item.Source
	if src == "" {
		src = "(unknown)"
	}
	return fmt.Sprintf(
		"\n\n---\nMaterialized from %s by %s · source_epic: %s · harvest: %s",
		materializeProvFromFile, materializedBy, epic, src,
	)
}

// mapNextWorkTypeToBeadType maps a next-work item type onto a bd issue type.
// bd accepts bug|feature|task|epic|chore|decision; next-work's finer-grained
// process categories collapse to task.
func mapNextWorkTypeToBeadType(t string) string {
	switch t {
	case "feature":
		return "feature"
	case "bug":
		return "bug"
	case "chore":
		return "chore"
	case "task":
		return "task"
	default: // tech-debt, improvement, pattern-fix, process-improvement, docs
		return "task"
	}
}

// mapSeverityToPriority maps next-work severity onto a bd priority string.
// P0 is reserved for operator-critical work, so high maps to P1.
func mapSeverityToPriority(severity string) string {
	switch severity {
	case "high":
		return "1"
	case "medium":
		return "2"
	case "low":
		return "3"
	default:
		return "2"
	}
}

// stampBeadIDs writes each created bead's ID back onto its source item so a
// re-run skips it. Dry-run and errored items are left untouched.
func stampBeadIDs(path string, results []materializeResult) error {
	stamp := make(map[int]map[int]string)
	for _, r := range results {
		if r.Status != "created" || r.BeadID == "" {
			continue
		}
		if stamp[r.entryIndex] == nil {
			stamp[r.entryIndex] = make(map[int]string)
		}
		stamp[r.entryIndex][r.itemIndex] = r.BeadID
	}
	if len(stamp) == 0 {
		return nil
	}
	return rewriteNextWorkFile(path, func(idx int, entry *nextWorkEntry) error {
		items, ok := stamp[idx]
		if !ok {
			return nil
		}
		for itemIdx, beadID := range items {
			if itemIdx >= 0 && itemIdx < len(entry.Items) {
				entry.Items[itemIdx].BeadID = beadID
			}
		}
		return nil
	})
}

// writeMaterializeSummary emits the per-item outcomes as text or JSON.
func writeMaterializeSummary(w io.Writer, results []materializeResult, dryRun, asJSON bool) error {
	created, planned, errored := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "created":
			created++
		case "would-create":
			planned++
		case "error":
			errored++
		}
	}
	if asJSON {
		payload := struct {
			DryRun  bool                `json:"dry_run"`
			Created int                 `json:"created"`
			Planned int                 `json:"planned"`
			Errors  int                 `json:"errors"`
			Results []materializeResult `json:"results"`
		}{DryRun: dryRun, Created: created, Planned: planned, Errors: errored, Results: results}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return fmt.Errorf("encode json summary: %w", err)
		}
		if errored > 0 {
			return fmt.Errorf("%d item(s) failed to materialize", errored)
		}
		return nil
	}

	if len(results) == 0 {
		fmt.Fprintln(w, "next-work materialize: no unmaterialized items — queue already compounded into the tracker")
		return nil
	}
	for _, r := range results {
		switch r.Status {
		case "created":
			fmt.Fprintf(w, "  ✓ %s → %s\n", r.Title, r.BeadID)
		case "would-create":
			fmt.Fprintf(w, "  • [dry-run] would create bead for %q (epic %s)\n", r.Title, r.SourceEpic)
		case "error":
			fmt.Fprintf(w, "  ✗ %s — %s\n", r.Title, r.Error)
		}
	}
	verb := "created"
	if dryRun {
		verb = "would create"
	}
	fmt.Fprintf(w, "next-work materialize: %s %d bead(s), %d error(s)\n", verb, created+planned, errored)
	if errored > 0 {
		return fmt.Errorf("%d item(s) failed to materialize", errored)
	}
	return nil
}
