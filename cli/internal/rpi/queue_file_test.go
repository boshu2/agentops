package rpi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// seedNextWorkFixture writes a REAL persisted next-work.jsonl row (the batch
// shape production emits: batch-level lifecycle fields, one keyed item and one
// unkeyed item, plus forward-compat unknown keys at both levels), then proves
// fixture fidelity by round-tripping it through the production reader.
func seedNextWorkFixture(t *testing.T) (path string, stale NextWorkEntry) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "next-work.jsonl")

	original := `{"source_epic":"age-batch-31","timestamp":"2026-07-03T09:00:00Z","items":[{"id":"itm-0","title":"first item","type":"task","severity":"medium","source":"retro-learning","description":"do the first thing","claim_status":"available","fc_item":"item forward-compat"},{"title":"second item","type":"task","severity":"low","source":"retro-learning","description":"do the second thing"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null,"notes":"forward-compat batch annotation"}` + "\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Fixture fidelity: the production reader must accept the persisted shape,
	// and the production writer must reproduce it losslessly (serialize with
	// the production writer, read back with the production reader).
	parsed, err := ParseNextWorkEntryLine(strings.TrimSpace(original))
	if err != nil {
		t.Fatalf("production reader rejected fixture: %v", err)
	}
	reserialized, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("production writer rejected fixture: %v", err)
	}
	roundTripped, err := ParseNextWorkEntryLine(string(reserialized))
	if err != nil {
		t.Fatalf("re-parse production-writer output: %v", err)
	}
	if len(roundTripped.Items) != 2 || roundTripped.Items[0].ID != "itm-0" || roundTripped.Consumed {
		t.Fatalf("fixture round-trip changed shape: %+v", roundTripped)
	}

	return path, roundTripped
}

// readSingleEntry reads the single-row queue file back through the production
// reader.
func readSingleEntry(t *testing.T, path string) NextWorkEntry {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	entry, err := ParseNextWorkEntryLine(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("re-parse rewritten line: %v", err)
	}
	return entry
}

// TestRewriteNextWorkFile_ConcurrentStaleRewriteCannotRevertConsumed is the
// age-kbw4 regression: one lane marks an item consumed while a concurrent lane
// rewrites the same row from a snapshot taken BEFORE that marking (the stale
// read-modify-write that reverted 13 consumed flags wholesale). Whichever
// order the sidecar lock serializes the two writers into, consumed=true must
// survive with its note/ref intact, and unknown fields must keep round-tripping.
func TestRewriteNextWorkFile_ConcurrentStaleRewriteCannotRevertConsumed(t *testing.T) {
	path, stale := seedNextWorkFixture(t)

	consumedBy := "lane-a"
	consumedAt := "2026-07-03T10:00:00Z"

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	// Lane A: mark item 0 consumed (the production consume shape: consumed +
	// claim_status + note/ref/by/at together).
	go func() {
		defer wg.Done()
		errs[0] = RewriteNextWorkFile(path, func(idx int, entry *NextWorkEntry) error {
			entry.Items[0].Consumed = true
			entry.Items[0].ClaimStatus = "consumed"
			entry.Items[0].ConsumedNote = "landed on main"
			entry.Items[0].ConsumedRef = "abc1234"
			entry.Items[0].ConsumedBy = &consumedBy
			entry.Items[0].ConsumedAt = &consumedAt
			return nil
		})
	}()
	// Lane B: the stale lane — replaces the entry wholesale from its
	// pre-consume in-memory model.
	go func() {
		defer wg.Done()
		staleCopy := stale
		errs[1] = RewriteNextWorkFile(path, func(idx int, entry *NextWorkEntry) error {
			*entry = staleCopy
			return nil
		})
	}()
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("lane %d RewriteNextWorkFile: %v", i, err)
		}
	}

	entry := readSingleEntry(t, path)
	if len(entry.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(entry.Items))
	}
	item := entry.Items[0]
	if !item.Consumed {
		t.Fatalf("consumed=true was reverted by the stale concurrent rewrite: %+v", item)
	}
	if item.ConsumedNote != "landed on main" {
		t.Errorf("ConsumedNote = %q, want %q", item.ConsumedNote, "landed on main")
	}
	if item.ConsumedRef != "abc1234" {
		t.Errorf("ConsumedRef = %q, want %q", item.ConsumedRef, "abc1234")
	}
	if item.ClaimStatus != "consumed" {
		t.Errorf("ClaimStatus = %q, want %q", item.ClaimStatus, "consumed")
	}
	if item.ConsumedBy == nil || *item.ConsumedBy != consumedBy {
		t.Errorf("ConsumedBy = %v, want %q", item.ConsumedBy, consumedBy)
	}
	if item.ConsumedAt == nil || *item.ConsumedAt != consumedAt {
		t.Errorf("ConsumedAt = %v, want %q", item.ConsumedAt, consumedAt)
	}
	if !IsQueueItemSelectable(entry.Items[1]) {
		t.Errorf("item 1 should remain selectable, got %+v", entry.Items[1])
	}

	// No regression of the lossless round-trip (3429abe): unknown keys at both
	// levels survive both rewrites.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back raw: %v", err)
	}
	for _, want := range []string{
		`"notes":"forward-compat batch annotation"`,
		`"fc_item":"item forward-compat"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("rewrite dropped unknown field %s\nfile = %s", want, raw)
		}
	}
}

// TestRewriteNextWorkFile_MergeRestoresConsumedFromDisk is the deterministic
// merge case: the file already carries consumed=true (batch AND item level)
// and a transform overwrites the entry from a stale snapshot lacking it. The
// written result must keep consumed=true with the note/ref from the consumed
// (disk) side.
func TestRewriteNextWorkFile_MergeRestoresConsumedFromDisk(t *testing.T) {
	path, stale := seedNextWorkFixture(t)

	consumedBy := "lane-a"
	consumedAt := "2026-07-03T10:00:00Z"
	// Advance the file to the consumed state through the production writer.
	err := RewriteNextWorkFile(path, func(idx int, entry *NextWorkEntry) error {
		entry.Items[0].Consumed = true
		entry.Items[0].ClaimStatus = "consumed"
		entry.Items[0].ConsumedNote = "landed on main"
		entry.Items[0].ConsumedRef = "abc1234"
		entry.Items[0].ConsumedBy = &consumedBy
		entry.Items[0].ConsumedAt = &consumedAt
		entry.Consumed = true
		entry.ClaimStatus = "consumed"
		entry.ConsumedNote = "batch fully landed"
		entry.ConsumedRef = "age-kbw4"
		entry.ConsumedBy = &consumedBy
		entry.ConsumedAt = &consumedAt
		return nil
	})
	if err != nil {
		t.Fatalf("consume rewrite: %v", err)
	}

	// Stale lane rewrites wholesale from its pre-consume snapshot.
	err = RewriteNextWorkFile(path, func(idx int, entry *NextWorkEntry) error {
		staleCopy := stale
		*entry = staleCopy
		return nil
	})
	if err != nil {
		t.Fatalf("stale rewrite: %v", err)
	}

	entry := readSingleEntry(t, path)
	if !entry.Consumed {
		t.Fatalf("batch consumed=true was downgraded: %+v", entry)
	}
	if entry.ConsumedNote != "batch fully landed" {
		t.Errorf("batch ConsumedNote = %q, want %q", entry.ConsumedNote, "batch fully landed")
	}
	if entry.ConsumedRef != "age-kbw4" {
		t.Errorf("batch ConsumedRef = %q, want %q", entry.ConsumedRef, "age-kbw4")
	}
	if entry.ClaimStatus != "consumed" {
		t.Errorf("batch ClaimStatus = %q, want %q", entry.ClaimStatus, "consumed")
	}
	if entry.ConsumedBy == nil || *entry.ConsumedBy != consumedBy {
		t.Errorf("batch ConsumedBy = %v, want %q", entry.ConsumedBy, consumedBy)
	}
	item := entry.Items[0]
	if !item.Consumed || item.ConsumedNote != "landed on main" || item.ConsumedRef != "abc1234" {
		t.Errorf("item consumed markers not restored from disk side: %+v", item)
	}
	if item.ConsumedAt == nil || *item.ConsumedAt != consumedAt {
		t.Errorf("item ConsumedAt = %v, want %q", item.ConsumedAt, consumedAt)
	}
}

// TestMergeConsumedState_UpdatedConsumedSideWins locks the other half of the
// merge rule: when the TRANSFORM side is the one with consumed=true, its
// note/ref win untouched — the merge only ever prevents downgrades, it does
// not resurrect old markers over a fresh consume.
func TestMergeConsumedState_UpdatedConsumedSideWins(t *testing.T) {
	disk := NextWorkEntry{
		SourceEpic: "e",
		Items: []NextWorkItem{
			{ID: "itm-0", Title: "t", Type: "task", Severity: "low", Source: "s", Description: "d"},
		},
	}
	updated := disk
	updated.Items = append([]NextWorkItem(nil), disk.Items...)
	updated.Items[0].Consumed = true
	updated.Items[0].ConsumedNote = "fresh consume"
	updated.Items[0].ConsumedRef = "def5678"

	mergeConsumedState(disk, &updated)

	if !updated.Items[0].Consumed {
		t.Fatalf("fresh consume lost: %+v", updated.Items[0])
	}
	if updated.Items[0].ConsumedNote != "fresh consume" {
		t.Errorf("ConsumedNote = %q, want %q", updated.Items[0].ConsumedNote, "fresh consume")
	}
	if updated.Items[0].ConsumedRef != "def5678" {
		t.Errorf("ConsumedRef = %q, want %q", updated.Items[0].ConsumedRef, "def5678")
	}
}

// TestMergeConsumedState_KeyedItemMatchedAcrossReorder proves the item merge
// matches by ID when items carry one, so a stale lane that reordered keyed
// items still cannot revert a consumed flag; a keyed item with no disk
// counterpart is genuinely new and is left untouched.
func TestMergeConsumedState_KeyedItemMatchedAcrossReorder(t *testing.T) {
	consumedBy := "lane-a"
	disk := NextWorkEntry{
		Items: []NextWorkItem{
			{ID: "itm-0", Title: "a"},
			{ID: "itm-1", Title: "b", Consumed: true, ConsumedNote: "done", ConsumedRef: "ref-1", ConsumedBy: &consumedBy, ClaimStatus: "consumed"},
		},
	}
	updated := NextWorkEntry{
		Items: []NextWorkItem{
			{ID: "itm-1", Title: "b"}, // reordered, consumed dropped by stale model
			{ID: "itm-0", Title: "a"},
			{ID: "itm-new", Title: "c"}, // genuinely new
		},
	}

	mergeConsumedState(disk, &updated)

	got := updated.Items[0]
	if !got.Consumed || got.ConsumedNote != "done" || got.ConsumedRef != "ref-1" || got.ClaimStatus != "consumed" {
		t.Errorf("keyed consumed item not restored across reorder: %+v", got)
	}
	if got.ConsumedBy == nil || *got.ConsumedBy != consumedBy {
		t.Errorf("ConsumedBy = %v, want %q", got.ConsumedBy, consumedBy)
	}
	if updated.Items[1].Consumed {
		t.Errorf("unconsumed keyed item wrongly marked consumed: %+v", updated.Items[1])
	}
	if updated.Items[2].Consumed || updated.Items[2].ConsumedNote != "" {
		t.Errorf("new keyed item must be untouched: %+v", updated.Items[2])
	}
}
