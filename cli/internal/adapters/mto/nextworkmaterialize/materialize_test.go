package nextworkmaterialize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

// writeMaterializeQueue writes lines (already JSON-encoded) to a temp next-work.jsonl
// and returns its path.
func writeMaterializeQueue(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write queue file: %v", err)
	}
	return path
}

// batchLine builds one next-work.jsonl batch entry line.
func batchLine(t *testing.T, sourceEpic string, items ...rpi.NextWorkItem) string {
	t.Helper()
	entry := rpi.NextWorkEntry{
		SourceEpic:  sourceEpic,
		Timestamp:   "2026-05-30T12:00:00Z",
		Items:       items,
		ClaimStatus: "available",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}
	return string(data)
}

// matOpts carries the flag-equivalent inputs for a materialize test run.
type matOpts struct {
	file       string
	sourceEpic string
	dryRun     bool
}

// runMaterialize installs a fake bd port, invokes Run, and returns combined
// output plus the captured bd-create argv list.
func runMaterialize(t *testing.T, beadIDs []string, o matOpts) (string, [][]string, error) {
	t.Helper()

	var calls [][]string
	idx := 0
	execBD := func(a ...string) ([]byte, error) {
		calls = append(calls, a)
		id := fmt.Sprintf("ag-mat%d", idx)
		if idx < len(beadIDs) {
			id = beadIDs[idx]
		}
		idx++
		return []byte(id + "\n"), nil
	}

	var out bytes.Buffer
	err := Run(Options{
		File:           o.file,
		DryRun:         o.dryRun,
		SourceEpic:     o.sourceEpic,
		MaterializedBy: DefaultMaterializedBy,
		Out:            &out,
		ErrOut:         &out,
		BDAvailable:    func() bool { return true },
		ExecBD:         execBD,
	})
	return out.String(), calls, err
}

func readQueueItems(t *testing.T, path string) []rpi.NextWorkItem {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	var items []rpi.NextWorkItem
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		entry, err := rpi.ParseNextWorkEntryLine(line)
		if err != nil {
			continue
		}
		items = append(items, entry.Items...)
	}
	return items
}

// TestNextWorkMaterialize_CreatesDurableBeadWithProvenance is the Gherkin
// contract / first failing test for ag-9jle.3:
//
//	Given a completed wave with a harvested follow-up
//	When materialize runs
//	Then a durable bead exists (via bd create) carrying source_epic + proof_ref,
//	     not only a next-work.jsonl queue line.
func TestNextWorkMaterialize_CreatesDurableBeadWithProvenance(t *testing.T) {
	item := rpi.NextWorkItem{
		Title:       "Fix the cwd bug in ao goals measure",
		Type:        "bug",
		Severity:    "high",
		Source:      "post-mortem-finding",
		Description: "ao goals measure fails when invoked outside repo root.",
		ProofRef:    &rpi.NextWorkProofRef{Kind: "completed_run", RunID: "run-abc123"},
	}
	path := writeMaterializeQueue(t, batchLine(t, "ag-9jle", item))

	out, calls, err := runMaterialize(t, []string{"ag-real1"}, matOpts{file: path})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 bd create call, got %d: %v", len(calls), calls)
	}
	argv := calls[0]
	if argv[0] != "create" {
		t.Errorf("expected first arg 'create', got %q", argv[0])
	}
	joined := strings.Join(argv, "\x00")
	// Durable bead carries the harvested title.
	if !strings.Contains(joined, "Fix the cwd bug in ao goals measure") {
		t.Errorf("bd create args missing harvested title: %v", argv)
	}
	// bug severity high -> bd type bug, priority 1.
	assertFlag(t, argv, "--type", "bug")
	assertFlag(t, argv, "--priority", "1")
	// Provenance rides native --metadata: source_epic + proof_ref present.
	meta := flagValue(t, argv, "--metadata")
	if !strings.Contains(meta, "ag-9jle") {
		t.Errorf("metadata missing source_epic ag-9jle: %s", meta)
	}
	if !strings.Contains(meta, "run-abc123") || !strings.Contains(meta, "proof_ref") {
		t.Errorf("metadata missing proof_ref: %s", meta)
	}

	// The item is stamped with its durable bead_id (back-reference), proving
	// the queue line is no longer the only record of the work.
	items := readQueueItems(t, path)
	if len(items) != 1 || items[0].BeadID != "ag-real1" {
		t.Fatalf("expected item stamped with bead_id ag-real1, got %+v", items)
	}

	if !strings.Contains(out, "ag-real1") {
		t.Errorf("summary should report created bead, got: %s", out)
	}
}

func TestNextWorkMaterialize_Idempotent(t *testing.T) {
	item := rpi.NextWorkItem{
		Title: "Tidy the docs links", Type: "docs", Severity: "low",
		Source: "post-mortem-finding", Description: "Fix link rot.",
	}
	path := writeMaterializeQueue(t, batchLine(t, "ag-9jle", item))

	if _, calls, err := runMaterialize(t, []string{"ag-real7"}, matOpts{file: path}); err != nil || len(calls) != 1 {
		t.Fatalf("first run: err=%v calls=%d (want 1)", err, len(calls))
	}
	// Second run: the item now has a bead_id, so nothing is created.
	out, calls, err := runMaterialize(t, nil, matOpts{file: path})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("idempotency broken: second run made %d bd calls", len(calls))
	}
	if !strings.Contains(out, "no unmaterialized items") {
		t.Errorf("expected no-op summary, got: %s", out)
	}
}

func TestNextWorkMaterialize_SkipsConsumedAndHeld(t *testing.T) {
	fresh := rpi.NextWorkItem{
		Title: "Fresh actionable item", Type: "task", Severity: "medium",
		Source: "post-mortem-finding", Description: "Do the thing.",
	}
	consumed := rpi.NextWorkItem{
		Title: "Already done", Type: "task", Severity: "low",
		Source: "post-mortem-finding", Description: "Historical.", Consumed: true,
	}
	held := rpi.NextWorkItem{
		Title: "Needs human review", Type: "feature", Severity: "high",
		Source: "feature-suggestion", Description: "Held.", Requires: []string{"human-review"},
	}
	path := writeMaterializeQueue(t, batchLine(t, "ag-9jle", fresh, consumed, held))

	_, calls, err := runMaterialize(t, []string{"ag-real9"}, matOpts{file: path})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected only the fresh item materialized, got %d calls: %v", len(calls), calls)
	}
	if !strings.Contains(strings.Join(calls[0], "\x00"), "Fresh actionable item") {
		t.Errorf("wrong item materialized: %v", calls[0])
	}
}

// TestNextWorkMaterialize_SkipsBatchConsumedEntry is the ag-mjlg regression
// contract. The real next-work.jsonl marks consumption at the BATCH level
// (entry.consumed / consumed_by), not per item, so an entry can be fully
// consumed while its items carry no item-level consumed flag.
func TestNextWorkMaterialize_SkipsBatchConsumedEntry(t *testing.T) {
	consumedBy := "soc-xlw8"
	consumedAt := "2026-05-08T09:30:00-04:00"
	// Item itself looks fresh: no item-level consumed flag, no bead_id; exactly
	// how historical queue entries store their items.
	freshLooking := rpi.NextWorkItem{
		Title: "Historical work already handled by soc-xlw8", Type: "task",
		Severity: "medium", Source: "post-mortem-finding", Description: "Done long ago.",
	}
	entry := rpi.NextWorkEntry{
		SourceEpic:  "soc-9xn0",
		Timestamp:   "2026-05-07T21:45:29-04:00",
		Items:       []rpi.NextWorkItem{freshLooking},
		Consumed:    true,
		ClaimStatus: "consumed",
		ConsumedBy:  &consumedBy,
		ConsumedAt:  &consumedAt,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal consumed entry: %v", err)
	}
	path := writeMaterializeQueue(t, string(data))

	out, calls, err := runMaterialize(t, nil, matOpts{file: path})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("batch-consumed entry must yield zero bd-create calls, got %d: %v", len(calls), calls)
	}
	if !strings.Contains(out, "no unmaterialized items") {
		t.Errorf("expected no-op summary for a fully batch-consumed queue, got: %s", out)
	}
}

func TestNextWorkMaterialize_DryRunDoesNotMutate(t *testing.T) {
	item := rpi.NextWorkItem{
		Title: "Candidate item", Type: "improvement", Severity: "medium",
		Source: "retro-learning", Description: "Improve.",
	}
	path := writeMaterializeQueue(t, batchLine(t, "ag-9jle", item))

	out, calls, err := runMaterialize(t, nil, matOpts{file: path, dryRun: true})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("dry-run must not call bd create, got %d", len(calls))
	}
	if !strings.Contains(out, "would create") {
		t.Errorf("dry-run summary should say 'would create', got: %s", out)
	}
	items := readQueueItems(t, path)
	if len(items) != 1 || items[0].BeadID != "" {
		t.Errorf("dry-run must not stamp bead_id, got %+v", items)
	}
}

func TestNextWorkMaterialize_SourceEpicFilter(t *testing.T) {
	a := rpi.NextWorkItem{Title: "From epic A", Type: "task", Severity: "low", Source: "post-mortem-finding", Description: "A."}
	b := rpi.NextWorkItem{Title: "From epic B", Type: "task", Severity: "low", Source: "post-mortem-finding", Description: "B."}
	path := writeMaterializeQueue(t, batchLine(t, "epic-a", a), batchLine(t, "epic-b", b))

	_, calls, err := runMaterialize(t, []string{"ag-realA"}, matOpts{file: path, sourceEpic: "epic-a"})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("source-epic filter should yield 1 create, got %d", len(calls))
	}
	if !strings.Contains(strings.Join(calls[0], "\x00"), "From epic A") {
		t.Errorf("filter selected wrong epic: %v", calls[0])
	}
}

func TestMapNextWorkTypeToBeadType(t *testing.T) {
	cases := map[string]string{
		"feature": "feature", "bug": "bug", "chore": "chore", "task": "task",
		"tech-debt": "task", "improvement": "task", "pattern-fix": "task",
		"process-improvement": "task", "docs": "task", "": "task",
	}
	for in, want := range cases {
		if got := MapNextWorkTypeToBeadType(in); got != want {
			t.Errorf("MapNextWorkTypeToBeadType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMapSeverityToPriority(t *testing.T) {
	cases := map[string]string{"high": "1", "medium": "2", "low": "3", "": "2", "bogus": "2"}
	for in, want := range cases {
		if got := MapSeverityToPriority(in); got != want {
			t.Errorf("MapSeverityToPriority(%q) = %q, want %q", in, got, want)
		}
	}
}

// assertFlag fails unless argv contains flag immediately followed by want.
func assertFlag(t *testing.T, argv []string, flag, want string) {
	t.Helper()
	if got := flagValue(t, argv, flag); got != want {
		t.Errorf("flag %s = %q, want %q (argv=%v)", flag, got, want, argv)
	}
}

// flagValue returns the value following flag in argv, failing if absent.
func flagValue(t *testing.T, argv []string, flag string) string {
	t.Helper()
	for i := 0; i < len(argv)-1; i++ {
		if argv[i] == flag {
			return argv[i+1]
		}
	}
	t.Fatalf("flag %s not found in argv=%v", flag, argv)
	return ""
}
