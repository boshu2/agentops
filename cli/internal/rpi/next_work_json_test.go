package rpi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNextWorkItem_ConsumedMarkersRoundTrip locks the first-class per-item
// consumed markers (age-tkxq): consumed_note and consumed_ref survive a
// Marshal/Unmarshal round trip alongside the authoritative consumed boolean, and
// stay omitted when empty so legacy rows are byte-unchanged.
func TestNextWorkItem_ConsumedMarkersRoundTrip(t *testing.T) {
	in := NextWorkItem{
		Title:        "wire per-item consumed markers",
		Type:         "task",
		Severity:     "medium",
		Source:       "retro-learning",
		Description:  "d",
		Consumed:     true,
		ConsumedNote: "landed as age-tkxq; validator hardened",
		ConsumedRef:  "age-tkxq",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"consumed":true`,
		`"consumed_note":"landed as age-tkxq; validator hardened"`,
		`"consumed_ref":"age-tkxq"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in marshaled item; json = %s", want, got)
		}
	}

	var out NextWorkItem
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Consumed {
		t.Errorf("Consumed round-trip = false, want true")
	}
	if out.ConsumedNote != in.ConsumedNote {
		t.Errorf("ConsumedNote round-trip = %q, want %q", out.ConsumedNote, in.ConsumedNote)
	}
	if out.ConsumedRef != in.ConsumedRef {
		t.Errorf("ConsumedRef round-trip = %q, want %q", out.ConsumedRef, in.ConsumedRef)
	}

	bare := NextWorkItem{Title: "x", Type: "task", Severity: "low", Source: "evolve-generator"}
	bb, err := json.Marshal(bare)
	if err != nil {
		t.Fatalf("marshal bare: %v", err)
	}
	for _, k := range []string{"consumed_note", "consumed_ref"} {
		if strings.Contains(string(bb), k) {
			t.Errorf("expected %q to be omitted when empty; json = %s", k, bb)
		}
	}
}

// TestNextWorkItem_MalformedConsumedNoteTypeRejected proves a wrong-typed
// consumed_note (a JSON number) is rejected at decode rather than silently
// coerced or dropped. RewriteNextWorkFile preserves such a line verbatim as
// "malformed"; the row validator (validate-next-work.sh) is what fails it.
func TestNextWorkItem_MalformedConsumedNoteTypeRejected(t *testing.T) {
	bad := `{"title":"x","type":"task","severity":"low","source":"evolve-generator","description":"d","consumed_note":123}`
	var item NextWorkItem
	if err := json.Unmarshal([]byte(bad), &item); err == nil {
		t.Fatalf("expected decode error for numeric consumed_note, got item=%+v", item)
	}
}

// TestNextWorkEntry_MarshalByteIdenticalWhenNoExtra is the regression guard that
// the Extra passthrough does not reorder or otherwise change output for rows with
// no unknown fields: marshal is idempotent across a round trip and Extra stays
// nil for a fully-declared entry.
func TestNextWorkEntry_MarshalByteIdenticalWhenNoExtra(t *testing.T) {
	line := `{"source_epic":"na-fr0","timestamp":"2026-03-08T17:30:00Z","items":[{"title":"t","type":"tech-debt","severity":"high","source":"council-finding","description":"d","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}`
	var entry NextWorkEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry.Extra != nil {
		t.Errorf("Extra should be nil for a fully-declared entry, got %v", entry.Extra)
	}
	if len(entry.Items) == 1 && entry.Items[0].Extra != nil {
		t.Errorf("item Extra should be nil for a fully-declared item, got %v", entry.Items[0].Extra)
	}
	first, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var again NextWorkEntry
	if err := json.Unmarshal(first, &again); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	second, err := json.Marshal(again)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("marshal not idempotent:\n first = %s\nsecond = %s", first, second)
	}
}

// TestRewriteNextWorkFile_PreservesUnknownAndConsumedFields is the load-bearing
// fixture-fidelity test for the reverted-flag / dropped-note hazard. It writes a
// REAL persisted batch-28-style row — a batch-level free-text consumed_note plus
// an undeclared forward-compat "notes" key plus an item carrying its own
// undeclared "id" — then rewrites the line through the production writer to claim
// item 0 with first-class per-item markers, and asserts EVERY field survived by
// reading the file back.
func TestRewriteNextWorkFile_PreservesUnknownAndConsumedFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")

	// Real persisted shape: batch-level consumed_note (the operator's hand-encoded
	// per-item summary), a forward-compat "notes" key the struct does not model,
	// and an item with an undeclared "id".
	original := `{"source_epic":"age-batch-28","timestamp":"2026-07-02T09:00:00Z","items":[{"id":"itm-0","title":"first item","type":"task","severity":"medium","source":"retro-learning","description":"do the first thing"},{"title":"second item","type":"task","severity":"low","source":"retro-learning","description":"do the second thing"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null,"consumed_note":"[0] pending; [1] pending","notes":"forward-compat batch annotation"}` + "\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Consume item 0 with first-class per-item markers, leaving item 1 available.
	err := RewriteNextWorkFile(path, func(idx int, entry *NextWorkEntry) error {
		if idx != 0 {
			return nil
		}
		if len(entry.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(entry.Items))
		}
		entry.Items[0].Consumed = true
		entry.Items[0].ClaimStatus = "consumed"
		entry.Items[0].ConsumedNote = "landed on main"
		entry.Items[0].ConsumedRef = "abc1234"
		return nil
	})
	if err != nil {
		t.Fatalf("RewriteNextWorkFile: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	rawStr := string(raw)

	// Unknown / batch-level fields the struct did not itself set must survive.
	for _, want := range []string{
		`"consumed_note":"[0] pending; [1] pending"`, // batch-level operator note
		`"notes":"forward-compat batch annotation"`,  // undeclared forward-compat key
		`"id":"itm-0"`, // undeclared item-level key
	} {
		if !strings.Contains(rawStr, want) {
			t.Errorf("rewrite dropped preserved field %s\nfile = %s", want, rawStr)
		}
	}

	// The transform's first-class per-item markers must be present.
	for _, want := range []string{
		`"consumed_note":"landed on main"`,
		`"consumed_ref":"abc1234"`,
	} {
		if !strings.Contains(rawStr, want) {
			t.Errorf("rewrite lost first-class per-item marker %s\nfile = %s", want, rawStr)
		}
	}

	// Structural: read the file through the production reader and confirm the
	// lifecycle actually advanced (item 0 consumed, item 1 still selectable).
	entry, err := ParseNextWorkEntryLine(strings.TrimSpace(rawStr))
	if err != nil {
		t.Fatalf("re-parse rewritten line: %v", err)
	}
	if len(entry.Items) != 2 {
		t.Fatalf("expected 2 items after rewrite, got %d", len(entry.Items))
	}
	if !entry.Items[0].Consumed || entry.Items[0].ConsumedRef != "abc1234" {
		t.Errorf("item 0 not consumed with ref after rewrite: %+v", entry.Items[0])
	}
	if !IsQueueItemSelectable(entry.Items[1]) {
		t.Errorf("item 1 should remain selectable after consuming only item 0")
	}
	if entry.ConsumedNote != "[0] pending; [1] pending" {
		t.Errorf("batch consumed_note round-trip = %q", entry.ConsumedNote)
	}
}
