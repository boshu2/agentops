package yieldledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// repoRoot returns the absolute repo root so tests can reference tracked
// fixtures and the schema without embedding duplicates.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file = .../cli/internal/yieldledger/yieldledger_test.go
	// climb: yieldledger/ -> internal/ -> cli/ -> repo root
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// fixturePath resolves a tracked fixture under tests/fixtures/yield-ledger.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "tests", "fixtures", "yield-ledger", name)
}

// TestLoadPath_Fixture verifies the tracked dogfood fixture loads through the
// structural validator with the expected event count and round-trips its body
// discriminator into the typed pointers.
func TestLoadPath_Fixture(t *testing.T) {
	ledger, err := LoadPath(fixturePath(t, "dogfood-chain.jsonl"))
	if err != nil {
		t.Fatalf("LoadPath error: %v", err)
	}
	if ledger.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", ledger.SchemaVersion, SchemaVersion)
	}
	if len(ledger.Events) != 5 {
		t.Fatalf("len(Events) = %d, want 5", len(ledger.Events))
	}
	// First event is a gate-verdict; its typed body must be populated and the
	// others nil.
	gv := ledger.Events[0]
	if gv.Event != EventGateVerdict {
		t.Fatalf("Events[0].Event = %q, want %q", gv.Event, EventGateVerdict)
	}
	if gv.GateVerdict == nil {
		t.Fatal("Events[0].GateVerdict is nil, want populated body")
	}
	if gv.Accept != nil || gv.Usage != nil {
		t.Error("Events[0] carries a foreign body")
	}
	if !gv.GateVerdict.CrossFamily {
		t.Error("Events[0].GateVerdict.CrossFamily = false, want true")
	}
	if gv.GateVerdict.PawlVerdictRef.HeadSHA != "abc1234" {
		t.Errorf("pawl_verdict_ref.head_sha = %q, want abc1234", gv.GateVerdict.PawlVerdictRef.HeadSHA)
	}
}

// TestProjections verifies bead-keyed projections filter by bead and event type
// in append order.
func TestProjections(t *testing.T) {
	ledger, err := LoadPath(fixturePath(t, "dogfood-chain.jsonl"))
	if err != nil {
		t.Fatalf("LoadPath error: %v", err)
	}

	if got := len(ledger.EventsFor("ag-grcz3")); got != 3 {
		t.Errorf("EventsFor(ag-grcz3) = %d, want 3", got)
	}
	if got := len(ledger.EventsFor("ag-qzinh")); got != 2 {
		t.Errorf("EventsFor(ag-qzinh) = %d, want 2", got)
	}
	if got := len(ledger.EventsFor("ag-missing")); got != 0 {
		t.Errorf("EventsFor(ag-missing) = %d, want 0", got)
	}

	if got := len(ledger.AcceptsFor("ag-grcz3")); got != 1 {
		t.Errorf("AcceptsFor(ag-grcz3) = %d, want 1", got)
	}
	if got := len(ledger.AcceptsFor("ag-qzinh")); got != 0 {
		t.Errorf("AcceptsFor(ag-qzinh) = %d, want 0 (never accepted)", got)
	}
	if got := len(ledger.GateVerdictsFor("ag-grcz3")); got != 1 {
		t.Errorf("GateVerdictsFor(ag-grcz3) = %d, want 1", got)
	}
	if got := len(ledger.UsageFor("ag-grcz3")); got != 1 {
		t.Errorf("UsageFor(ag-grcz3) = %d, want 1", got)
	}
	if got := len(ledger.UsageFor("ag-qzinh")); got != 1 {
		t.Errorf("UsageFor(ag-qzinh) = %d, want 1", got)
	}
}

// TestValidateEvent verifies structural rejection of malformed events across
// each event type and the envelope.
func TestValidateEvent(t *testing.T) {
	goodRef := PawlVerdictRef{BeadID: "ag-x", HeadSHA: "abc1234"}
	cases := []struct {
		name    string
		event   Event
		wantBad bool
	}{
		{
			name:    "valid accept",
			event:   newAcceptEvent(AcceptInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), MergeSHA: "abc1234", MergedBy: "orch", GateVerdictRef: goodRef}),
			wantBad: false,
		},
		{
			name:    "valid gate-verdict",
			event:   newGateVerdictEvent(GateVerdictInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), Difficulty: 2, PawlVerdictRef: goodRef, Disposition: DispositionConfirmed, HeadSHA: "abc1234", Attempt: 1, AuthorContextID: "ctx-1", AuthorFamily: "claude"}),
			wantBad: false,
		},
		{
			name:    "valid usage",
			event:   newUsageEvent(UsageInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), TokensIn: 10, TokensOut: 5, CostUSD: 0.1, WallClockS: 3, Model: "m", Phase: PhaseImplement}),
			wantBad: false,
		},
		{
			name:    "missing bead_id",
			event:   newUsageEvent(UsageInput{BeadID: "", RunID: "r1", TS: time.Now(), Model: "m", Phase: PhaseImplement}),
			wantBad: true,
		},
		{
			name:    "missing run_id",
			event:   newUsageEvent(UsageInput{BeadID: "ag-x", RunID: "", TS: time.Now(), Model: "m", Phase: PhaseImplement}),
			wantBad: true,
		},
		{
			name:    "accept short merge_sha",
			event:   newAcceptEvent(AcceptInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), MergeSHA: "abc", MergedBy: "orch", GateVerdictRef: goodRef}),
			wantBad: true,
		},
		{
			name:    "accept invalid ref",
			event:   newAcceptEvent(AcceptInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), MergeSHA: "abc1234", MergedBy: "orch", GateVerdictRef: PawlVerdictRef{BeadID: "ag-x", HeadSHA: "short"}}),
			wantBad: true,
		},
		{
			name:    "valid gate-verdict deterministic tier (age-srl)",
			event:   newGateVerdictEvent(GateVerdictInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), Difficulty: 1, PawlVerdictRef: goodRef, Disposition: DispositionRefuted, HeadSHA: "abc1234", Attempt: 1, Mode: ModeDeterministic, AuthorContextID: "pre-push-gate", AuthorFamily: "deterministic-gate"}),
			wantBad: false,
		},
		{
			name:    "gate-verdict bad mode",
			event:   newGateVerdictEvent(GateVerdictInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), Difficulty: 1, PawlVerdictRef: goodRef, Disposition: DispositionRefuted, HeadSHA: "abc1234", Attempt: 1, Mode: "telepathy", AuthorContextID: "ctx-1", AuthorFamily: "claude"}),
			wantBad: true,
		},
		{
			name:    "gate-verdict bad disposition",
			event:   newGateVerdictEvent(GateVerdictInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), Difficulty: 2, PawlVerdictRef: goodRef, Disposition: "MAYBE", HeadSHA: "abc1234", Attempt: 1, AuthorContextID: "ctx-1", AuthorFamily: "claude"}),
			wantBad: true,
		},
		{
			name:    "gate-verdict attempt zero",
			event:   newGateVerdictEvent(GateVerdictInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), Difficulty: 2, PawlVerdictRef: goodRef, Disposition: DispositionConfirmed, HeadSHA: "abc1234", Attempt: 0, AuthorContextID: "ctx-1", AuthorFamily: "claude"}),
			wantBad: true,
		},
		{
			name:    "gate-verdict missing author_family",
			event:   newGateVerdictEvent(GateVerdictInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), Difficulty: 2, PawlVerdictRef: goodRef, Disposition: DispositionConfirmed, HeadSHA: "abc1234", Attempt: 1, AuthorContextID: "ctx-1", AuthorFamily: ""}),
			wantBad: true,
		},
		{
			name:    "usage bad phase",
			event:   newUsageEvent(UsageInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), Model: "m", Phase: "bogus"}),
			wantBad: true,
		},
		{
			name:    "usage negative cost",
			event:   newUsageEvent(UsageInput{BeadID: "ag-x", RunID: "r1", TS: time.Now(), CostUSD: -1, Model: "m", Phase: PhaseImplement}),
			wantBad: true,
		},
		{
			name:    "unknown event type",
			event:   Event{Event: "bogus", BeadID: "ag-x", RunID: "r1", TS: time.Now().UTC().Format(time.RFC3339)},
			wantBad: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defect := validateEvent(tc.event)
			if tc.wantBad && defect == "" {
				t.Error("validateEvent accepted a malformed event, want defect")
			}
			if !tc.wantBad && defect != "" {
				t.Errorf("validateEvent rejected a valid event: %s", defect)
			}
		})
	}
}

// TestWriterAppendsWithoutClobber verifies each Append* preserves prior events,
// appends in order, and writes atomically (no leftover tmp).
func TestWriterAppendsWithoutClobber(t *testing.T) {
	root := t.TempDir()
	at := func(h int) time.Time { return time.Date(2026, 6, 14, h, 0, 0, 0, time.UTC) }
	w := Writer{Now: func() time.Time { return at(23) }}
	ref := PawlVerdictRef{BeadID: "ag-x", HeadSHA: "abc1234"}

	first, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-x", RunID: "r1", TS: at(10), Difficulty: 3, PawlVerdictRef: ref,
		Disposition: DispositionConfirmed, HeadSHA: "abc1234", Attempt: 1,
		AuthorContextID: "ctx-1", RefuterFamilies: []string{"claude", "gpt"},
		AuthorFamily: "claude", CrossFamily: true, AuthorNeReviewer: true, EvidencePresent: true,
	})
	if err != nil {
		t.Fatalf("AppendGateVerdict: %v", err)
	}
	if len(first.Events) != 1 {
		t.Fatalf("after gate-verdict len(Events) = %d, want 1", len(first.Events))
	}

	second, err := w.AppendUsage(root, UsageInput{
		BeadID: "ag-x", RunID: "r1", TS: at(11), TokensIn: 100, TokensOut: 20,
		CostUSD: 0.3, WallClockS: 60, Model: "claude-opus-4-8", Phase: PhaseImplement,
		CategoryHint: CategoryProductive,
	})
	if err != nil {
		t.Fatalf("AppendUsage: %v", err)
	}
	if len(second.Events) != 2 {
		t.Fatalf("after usage len(Events) = %d, want 2", len(second.Events))
	}

	third, err := w.AppendAccept(root, AcceptInput{
		BeadID: "ag-x", RunID: "r1", TS: at(12), MergeSHA: "def5678",
		MergedBy: "orchestrator", GateVerdictRef: ref,
	})
	if err != nil {
		t.Fatalf("AppendAccept: %v", err)
	}
	if len(third.Events) != 3 {
		t.Fatalf("after accept len(Events) = %d, want 3", len(third.Events))
	}

	// Reload from disk: prior events must survive every append, in order, with
	// their typed bodies round-tripped.
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load after appends: %v", err)
	}
	if len(loaded.Events) != 3 {
		t.Fatalf("reloaded len(Events) = %d, want 3", len(loaded.Events))
	}
	if loaded.Events[0].Event != EventGateVerdict || loaded.Events[0].GateVerdict == nil {
		t.Error("Events[0] is not a populated gate-verdict after reload")
	}
	if loaded.Events[1].Usage == nil || loaded.Events[1].Usage.CategoryHint != CategoryProductive {
		t.Error("Events[1] usage body did not round-trip")
	}
	if loaded.Events[2].Accept == nil || loaded.Events[2].Accept.MergeSHA != "def5678" {
		t.Error("Events[2] accept body did not round-trip")
	}
	if got := len(loaded.AcceptsFor("ag-x")); got != 1 {
		t.Errorf("AcceptsFor(ag-x) after reload = %d, want 1", got)
	}

	// No leftover tmp file from atomic write.
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ArtifactRelPath)) + ".tmp"); !os.IsNotExist(err) {
		t.Error("leftover .tmp file after atomic write")
	}
}

// TestLoadMissingLedger verifies a missing ledger is an empty ledger, no error.
func TestLoadMissingLedger(t *testing.T) {
	ledger, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load(missing) error: %v", err)
	}
	if len(ledger.Events) != 0 {
		t.Errorf("len(Events) = %d, want 0", len(ledger.Events))
	}
	if got := len(ledger.EventsFor("ag-x")); got != 0 {
		t.Errorf("EventsFor on empty ledger = %d, want 0", got)
	}
}

// TestLoadMalformedIsError verifies a structurally invalid JSONL line fails
// closed (with its line number) rather than loading silently, and that blank
// lines between events are tolerated.
func TestLoadMalformedIsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, filepath.FromSlash(ArtifactRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	// One good line, a blank line, then a usage event with an invalid phase: the
	// bad line must be rejected and the error must cite line 3.
	good := `{"event":"usage","bead_id":"ag-x","run_id":"r1","ts":"2026-06-14T17:00:00Z","body":{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"implement"}}`
	bad := `{"event":"usage","bead_id":"ag-x","run_id":"r1","ts":"2026-06-14T17:00:00Z","body":{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"bogus"}}`
	if err := os.WriteFile(path, []byte(good+"\n\n"+bad+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load accepted a malformed line, want error")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("malformed-line error %q does not cite line 3", err)
	}

	// A line whose JSON is itself broken is also a hard error citing the line.
	if err := os.WriteFile(path, []byte("{not json}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("Load accepted broken JSON, want error")
	}
}

// TestRoundTripJSON verifies the wire envelope nests the typed body under `body`
// and decodes back to the same typed pointer.
func TestRoundTripJSON(t *testing.T) {
	ev := newAcceptEvent(AcceptInput{
		BeadID: "ag-x", RunID: "r1", TS: time.Date(2026, 6, 14, 18, 0, 0, 0, time.UTC),
		MergeSHA: "def5678", MergedBy: "orch", GateVerdictRef: PawlVerdictRef{BeadID: "ag-x", HeadSHA: "abc1234"},
	})
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// The wire form must carry a nested `body`, not flattened accept fields.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		t.Fatalf("probe unmarshal: %v", err)
	}
	if _, ok := probe["body"]; !ok {
		t.Fatalf("marshaled event has no `body` envelope field: %s", data)
	}

	var back Event
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.Accept == nil || back.Accept.MergeSHA != "def5678" {
		t.Errorf("accept body did not round-trip: %+v", back)
	}
}

// TestConcurrentAppend is the B1 regression: N goroutines each append one event
// to the SAME ledger, then Load must see ALL N events well-formed. This FAILS
// against a read-modify-write-one-doc writer (lost updates / shared-tmp
// collisions) and PASSES against O_APPEND JSONL. Run under -race.
func TestConcurrentAppend(t *testing.T) {
	root := t.TempDir()
	const n = 64
	ref := PawlVerdictRef{BeadID: "ag-x", HeadSHA: "abc1234"}
	ts := time.Date(2026, 6, 14, 17, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := Writer{}
			_, err := w.AppendGateVerdict(root, GateVerdictInput{
				BeadID: "ag-x", RunID: fmt.Sprintf("r-%03d", i), TS: ts,
				Difficulty: 1, PawlVerdictRef: ref, Disposition: DispositionConfirmed,
				HeadSHA: "abc1234", Attempt: 1, AuthorContextID: "ctx-1",
				RefuterFamilies: []string{"claude"}, AuthorFamily: "claude",
			})
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent append errored: %v", err)
	}

	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load after concurrent appends: %v", err)
	}
	if len(loaded.Events) != n {
		t.Fatalf("after %d concurrent appends len(Events) = %d, want %d (lost/collided events)", n, len(loaded.Events), n)
	}
	// Every run_id must be present exactly once and every body well-formed.
	seen := make(map[string]int, n)
	for _, ev := range loaded.Events {
		if ev.Event != EventGateVerdict || ev.GateVerdict == nil {
			t.Fatalf("event %q not a populated gate-verdict after concurrent append", ev.RunID)
		}
		seen[ev.RunID]++
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("r-%03d", i)
		if seen[id] != 1 {
			t.Errorf("run_id %s appeared %d times, want exactly 1", id, seen[id])
		}
	}
}

// TestClosedSchemaRejectsUnknownBodyField is the B2 regression: an event whose
// body carries a field not in the closed schema (additionalProperties:false)
// must be REJECTED on decode, not silently dropped.
func TestClosedSchemaRejectsUnknownBodyField(t *testing.T) {
	// Unknown field "surprise" inside the usage body.
	line := `{"event":"usage","bead_id":"ag-x","run_id":"r1","ts":"2026-06-14T17:00:00Z","body":{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"implement","surprise":true}}`
	var ev Event
	if err := ev.UnmarshalJSON([]byte(line)); err == nil {
		t.Error("decoded an event with an unknown body field, want reject")
	}

	// Unknown field on the envelope itself must also reject.
	envLine := `{"event":"usage","bead_id":"ag-x","run_id":"r1","ts":"2026-06-14T17:00:00Z","bogus_top":1,"body":{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"implement"}}`
	var ev2 Event
	if err := ev2.UnmarshalJSON([]byte(envLine)); err == nil {
		t.Error("decoded an event with an unknown envelope field, want reject")
	}

	// And via Load (the script-facing path): a JSONL line with an unknown body
	// field is a hard error citing the line.
	dir := t.TempDir()
	path := filepath.Join(dir, filepath.FromSlash(ArtifactRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("Load accepted an unknown body field, want error")
	}
}

// TestRejectedAndHoldVerdictsRoundTrip is the B6c regression: a REFUTED and a
// HOLD gate-verdict (not just CONFIRMED) must append and Load back intact, so Q's
// denominator and E (escalate/hold) are computable from data.
func TestRejectedAndHoldVerdictsRoundTrip(t *testing.T) {
	root := t.TempDir()
	ref := PawlVerdictRef{BeadID: "ag-x", HeadSHA: "abc1234"}
	ts := time.Date(2026, 6, 14, 17, 0, 0, 0, time.UTC)
	w := Writer{}

	for _, disp := range []string{DispositionRefuted, DispositionHold} {
		if _, err := w.AppendGateVerdict(root, GateVerdictInput{
			BeadID: "ag-x", RunID: "r1", TS: ts, Difficulty: 2, PawlVerdictRef: ref,
			Disposition: disp, HeadSHA: "abc1234", Attempt: 1, AuthorContextID: "ctx-1",
			RefuterFamilies: []string{"claude", "gpt"}, AuthorFamily: "claude",
		}); err != nil {
			t.Fatalf("AppendGateVerdict(%s): %v", disp, err)
		}
	}

	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	gvs := loaded.GateVerdictsFor("ag-x")
	if len(gvs) != 2 {
		t.Fatalf("GateVerdictsFor(ag-x) = %d, want 2", len(gvs))
	}
	if gvs[0].GateVerdict.Disposition != DispositionRefuted {
		t.Errorf("first verdict disposition = %q, want REFUTED", gvs[0].GateVerdict.Disposition)
	}
	if gvs[1].GateVerdict.Disposition != DispositionHold {
		t.Errorf("second verdict disposition = %q, want HOLD", gvs[1].GateVerdict.Disposition)
	}
}
