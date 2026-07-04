package mine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

// TestWriteWorkItems_AppendSurvivesConcurrentRewrite is the age-kbw4 append
// window: without the shared next-work sidecar lock, a line appended by
// WriteWorkItems between a concurrent RewriteNextWorkFile's read and its
// truncate+write is silently destroyed. Both writers now serialize on
// rpi.WithNextWorkFileLock, so whichever order they run in, BOTH facts must
// survive: the rewrite's consumed marking and the appended compile-mine row.
func TestWriteWorkItems_AppendSurvivesConcurrentRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")

	// Real persisted row, proven by production reader/writer round-trip.
	original := `{"source_epic":"age-batch-31","timestamp":"2026-07-03T09:00:00Z","items":[{"id":"itm-0","title":"first item","type":"task","severity":"medium","source":"retro-learning","description":"do the first thing","claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}` + "\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	seeded, err := rpi.ParseNextWorkEntryLine(strings.TrimSpace(original))
	if err != nil {
		t.Fatalf("production reader rejected fixture: %v", err)
	}
	if reserialized, err := json.Marshal(seeded); err != nil {
		t.Fatalf("production writer rejected fixture: %v", err)
	} else if _, err := rpi.ParseNextWorkEntryLine(string(reserialized)); err != nil {
		t.Fatalf("re-parse production-writer output: %v", err)
	}

	appended := WorkItemEmit{
		Title:       "Reduce complexity: parse in cli/foo.go (CC=17)",
		Type:        "refactor",
		Severity:    "high",
		Source:      "compile-mine",
		Description: "Function parse in cli/foo.go has cyclomatic complexity 17 with 3 recent edits. Extract helpers to reduce CC below 15.",
		Evidence:    "complexity=17 recent_edits=3",
		File:        "cli/foo.go",
		Func:        "parse",
	}
	appended.ID = WorkItemID(appended)
	const ts = "2026-07-03T11:00:00Z"

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	// Lane A: consume the seeded item through the production rewrite path.
	go func() {
		defer wg.Done()
		errs[0] = rpi.RewriteNextWorkFile(path, func(idx int, entry *rpi.NextWorkEntry) error {
			entry.Items[0].Consumed = true
			entry.Items[0].ClaimStatus = "consumed"
			entry.Items[0].ConsumedNote = "landed on main"
			entry.Items[0].ConsumedRef = "abc1234"
			return nil
		})
	}()
	// Lane B: the compile-mine producer appends a new batch concurrently.
	go func() {
		defer wg.Done()
		errs[1] = WriteWorkItems(path, []WorkItemEmit{appended}, ts)
	}()
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("lane %d: %v", i, err)
		}
	}

	// Read everything back through the production reader.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var entries []rpi.NextWorkEntry
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		entry, err := rpi.ParseNextWorkEntryLine(line)
		if err != nil {
			t.Fatalf("unparseable line after concurrent writers: %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (seeded + appended), got %d\nfile = %s", len(entries), raw)
	}

	var seededOut, minedOut *rpi.NextWorkEntry
	for i := range entries {
		switch entries[i].SourceEpic {
		case "age-batch-31":
			seededOut = &entries[i]
		case "compile-mine":
			minedOut = &entries[i]
		}
	}
	if seededOut == nil {
		t.Fatalf("seeded entry lost: %s", raw)
	}
	if minedOut == nil {
		t.Fatalf("appended compile-mine entry destroyed by concurrent rewrite: %s", raw)
	}

	// The rewrite's consumed marking survived, exactly.
	item := seededOut.Items[0]
	if !item.Consumed {
		t.Fatalf("seeded item consumed=true lost: %+v", item)
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

	// The appended row survived, exactly as WriteWorkItems emits it.
	if minedOut.Timestamp != ts {
		t.Errorf("appended Timestamp = %q, want %q", minedOut.Timestamp, ts)
	}
	if minedOut.Consumed {
		t.Errorf("appended entry must start unconsumed: %+v", minedOut)
	}
	if minedOut.ClaimStatus != "available" {
		t.Errorf("appended ClaimStatus = %q, want %q", minedOut.ClaimStatus, "available")
	}
	if len(minedOut.Items) != 1 {
		t.Fatalf("appended entry items = %d, want 1", len(minedOut.Items))
	}
	got := minedOut.Items[0]
	if got.ID != appended.ID {
		t.Errorf("appended item ID = %q, want %q", got.ID, appended.ID)
	}
	if got.Title != appended.Title {
		t.Errorf("appended item Title = %q, want %q", got.Title, appended.Title)
	}
	if got.Description != appended.Description {
		t.Errorf("appended item Description = %q, want %q", got.Description, appended.Description)
	}
}
