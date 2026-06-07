// practices: [hexagonal-architecture, durable-provenance]
package nextworkmaterialize

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

const (
	nextWorkQueueRel        = ".agents/rpi/next-work.jsonl"
	DefaultMaterializedBy   = "next-work-materialize"
	materializeBeadLabels   = "next-work,materialized"
	materializeProvFromFile = "next-work.jsonl"
)

// Options carries the CLI flags and dependency ports for next-work
// materialization. ExecBD and BDAvailable are injected so tests do not need the
// real bd binary.
type Options struct {
	File           string
	DryRun         bool
	JSON           bool
	SourceEpic     string
	MaterializedBy string
	Out            io.Writer
	ErrOut         io.Writer
	BDAvailable    func() bool
	ExecBD         func(args ...string) ([]byte, error)
}

// materializeCandidate is one queue item eligible to become a bead, tagged with
// the parseable-entry/item indices so the bead_id back-reference can be stamped.
type materializeCandidate struct {
	EntryIndex int
	ItemIndex  int
	SourceEpic string
	Item       rpi.NextWorkItem
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
// It rides bd's native metadata field; the real provenance edge is deferred to
// `ao provenance add`.
type materializeProvenance struct {
	MaterializedFrom string                `json:"materialized_from"`
	SourceEpic       string                `json:"source_epic,omitempty"`
	HarvestSource    string                `json:"harvest_source,omitempty"`
	NextWorkType     string                `json:"nextwork_type,omitempty"`
	Severity         string                `json:"severity,omitempty"`
	ProofRef         *rpi.NextWorkProofRef `json:"proof_ref,omitempty"`
}

// Run reads unmaterialized items from next-work.jsonl, creates durable beads,
// and stamps each created bead ID back onto the source queue item.
func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	errOut := opts.ErrOut
	if errOut == nil {
		errOut = io.Discard
	}
	if opts.MaterializedBy == "" {
		opts.MaterializedBy = DefaultMaterializedBy
	}

	path, err := ResolveNextWorkPath(opts.File)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(path); statErr != nil {
		fmt.Fprintf(out, "no next-work queue at %s — nothing to materialize\n", path)
		return nil
	}

	if !opts.DryRun && opts.BDAvailable != nil && !opts.BDAvailable() {
		fmt.Fprintln(errOut, "WARN: bd not on PATH — skipping materialize (graceful degradation)")
		return nil
	}

	candidates, err := enumerateMaterializeCandidates(path, opts.SourceEpic)
	if err != nil {
		return err
	}

	results := make([]materializeResult, 0, len(candidates))
	for _, c := range candidates {
		results = append(results, materializeOne(c, opts.MaterializedBy, opts.DryRun, opts.ExecBD))
	}

	if !opts.DryRun {
		if err := stampBeadIDs(path, results); err != nil {
			return fmt.Errorf("stamp bead_id back-references: %w", err)
		}
	}

	return writeMaterializeSummary(out, results, opts.DryRun, opts.JSON)
}

// ResolveNextWorkPath returns the explicit --file path or the cwd default.
func ResolveNextWorkPath(explicit string) (string, error) {
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
// materialization. Parseable entry indices come from
// rpi.ForEachParseableNextWorkEntry (same rules as rpi.RewriteNextWorkFile).
// Legacy flat entries are skipped because bead_id stamping writes into
// entry.Items by index, which requires items[].
func enumerateMaterializeCandidates(path, sourceEpicFilter string) ([]materializeCandidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var candidates []materializeCandidate
	err = rpi.ForEachParseableNextWorkEntry(data, func(idx int, entry rpi.NextWorkEntry) error {
		if sourceEpicFilter != "" && entry.SourceEpic != sourceEpicFilter {
			return nil
		}
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
// entry level. Items inside such an entry must not be materialized, even though
// their per-item consumed flags may be unset.
func isEntryConsumed(entry *rpi.NextWorkEntry) bool {
	return entry.Consumed || rpi.NormalizeClaimStatus(entry.Consumed, entry.ClaimStatus) == "consumed"
}

// isMaterializable reports whether an item should become a durable bead.
func isMaterializable(item rpi.NextWorkItem) bool {
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
func materializeOne(c materializeCandidate, materializedBy string, dryRun bool, execBD func(args ...string) ([]byte, error)) materializeResult {
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
	if execBD == nil {
		res.Status = "error"
		res.Error = "bd executor is not configured"
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
		"--type", MapNextWorkTypeToBeadType(c.Item.Type),
		"--priority", MapSeverityToPriority(c.Item.Severity),
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

// MapNextWorkTypeToBeadType maps a next-work item type onto a bd issue type.
// bd accepts bug|feature|task|epic|chore|decision; next-work's finer-grained
// process categories collapse to task.
func MapNextWorkTypeToBeadType(t string) string {
	switch t {
	case "feature":
		return "feature"
	case "bug":
		return "bug"
	case "chore":
		return "chore"
	case "task":
		return "task"
	default:
		return "task"
	}
}

// MapSeverityToPriority maps next-work severity onto a bd priority string.
// P0 is reserved for operator-critical work, so high maps to P1.
func MapSeverityToPriority(severity string) string {
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
	return rpi.RewriteNextWorkFile(path, func(idx int, entry *rpi.NextWorkEntry) error {
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
