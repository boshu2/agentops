package turnstate

import (
	"errors"
	"reflect"
	"testing"
)

// buildLog seals a sequence of (artifactID, from, to) steps into one chained
// log so tests can describe a scenario without hand-linking prev_hash. The ts
// is monotonically increasing per step so canonical replay order is the build
// order.
func buildLog(t *testing.T, steps [][3]string) []Transition {
	t.Helper()
	var log []Transition
	tsBase := []string{
		"2026-05-31T00:00:00Z",
		"2026-05-31T00:00:01Z",
		"2026-05-31T00:00:02Z",
		"2026-05-31T00:00:03Z",
		"2026-05-31T00:00:04Z",
		"2026-05-31T00:00:05Z",
	}
	for i, s := range steps {
		if i >= len(tsBase) {
			t.Fatalf("buildLog: too many steps (%d), extend tsBase", len(steps))
		}
		var err error
		log, err = Append(log, Transition{
			ArtifactID: s[0],
			FromState:  s[1],
			ToState:    s[2],
			TS:         tsBase[i],
		})
		if err != nil {
			t.Fatalf("Append step %d: %v", i, err)
		}
	}
	return log
}

func TestSeal_StampsSchemaAndHashes(t *testing.T) {
	sealed, err := Seal(Transition{
		ArtifactID: "art-1",
		FromState:  InitialState,
		ToState:    "drafted",
		TS:         "2026-05-31T00:00:00Z",
	}, "")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if sealed.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %q, want %q", sealed.SchemaVersion, SchemaVersion)
	}
	if sealed.PrevHash != "" {
		t.Errorf("genesis prev_hash = %q, want empty", sealed.PrevHash)
	}
	wantPayload, wantHash, _ := ComputeHashes(sealed)
	if sealed.PayloadHash != wantPayload {
		t.Errorf("payload_hash = %q, want %q", sealed.PayloadHash, wantPayload)
	}
	if sealed.Hash != wantHash {
		t.Errorf("hash = %q, want %q", sealed.Hash, wantHash)
	}
}

func TestValidateFields_Rejects(t *testing.T) {
	valid := Transition{
		SchemaVersion: SchemaVersion,
		ArtifactID:    "art-1",
		ToState:       "drafted",
		TS:            "2026-05-31T00:00:00Z",
	}
	tests := []struct {
		name    string
		mutate  func(Transition) Transition
		wantErr bool
	}{
		{"valid", func(tr Transition) Transition { return tr }, false},
		{"bad schema", func(tr Transition) Transition { tr.SchemaVersion = "v0"; return tr }, true},
		{"empty artifact_id", func(tr Transition) Transition { tr.ArtifactID = "  "; return tr }, true},
		{"empty to_state", func(tr Transition) Transition { tr.ToState = ""; return tr }, true},
		{"empty ts", func(tr Transition) Transition { tr.TS = ""; return tr }, true},
		{"non-utc ts", func(tr Transition) Transition { tr.TS = "2026-05-31T00:00:00+02:00"; return tr }, true},
		{"unparseable ts", func(tr Transition) Transition { tr.TS = "yesterday"; return tr }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFields(tc.mutate(valid))
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateFields err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestFold_DerivesCurrentState(t *testing.T) {
	log := buildLog(t, [][3]string{
		{"art-1", InitialState, "drafted"},
		{"art-1", "drafted", "reviewed"},
		{"art-1", "reviewed", "merged"},
		{"art-2", InitialState, "drafted"},
	})
	states, err := Fold(log)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	want := map[string]string{
		"art-1": "merged",
		"art-2": "drafted",
	}
	if !reflect.DeepEqual(states, want) {
		t.Errorf("Fold = %v, want %v", states, want)
	}
}

func TestFold_EmptyLogIsEmptyMap(t *testing.T) {
	states, err := Fold(nil)
	if err != nil {
		t.Fatalf("Fold(nil): %v", err)
	}
	if len(states) != 0 {
		t.Errorf("Fold(nil) = %v, want empty map", states)
	}
}

// TestFold_DeterministicAcrossInputOrder is the core property: state is a fold,
// so the same set of transitions yields identical state regardless of the order
// the slice happens to be in. Reversed input must replay to the same state.
func TestFold_DeterministicAcrossInputOrder(t *testing.T) {
	log := buildLog(t, [][3]string{
		{"art-1", InitialState, "drafted"},
		{"art-1", "drafted", "reviewed"},
		{"art-1", "reviewed", "merged"},
	})
	reversed := make([]Transition, len(log))
	for i, tr := range log {
		reversed[len(log)-1-i] = tr
	}

	forward, err := Fold(log)
	if err != nil {
		t.Fatalf("Fold forward: %v", err)
	}
	backward, err := Fold(reversed)
	if err != nil {
		t.Fatalf("Fold reversed: %v", err)
	}
	if !reflect.DeepEqual(forward, backward) {
		t.Errorf("Fold not order-independent: forward=%v reversed=%v", forward, backward)
	}
	if forward["art-1"] != "merged" {
		t.Errorf("art-1 state = %q, want merged", forward["art-1"])
	}
}

// TestFold_RejectsBrokenStateChain proves the in-band guard: a transition whose
// from_state does not match the current folded state is rejected, so state
// cannot jump without a contiguous transition.
func TestFold_RejectsBrokenStateChain(t *testing.T) {
	// art-1 goes Initial->drafted, then a transition claims from "merged"
	// which was never the current state.
	t0, err := Seal(Transition{ArtifactID: "art-1", FromState: InitialState, ToState: "drafted", TS: "2026-05-31T00:00:00Z"}, "")
	if err != nil {
		t.Fatalf("seal t0: %v", err)
	}
	t1, err := Seal(Transition{ArtifactID: "art-1", FromState: "merged", ToState: "archived", TS: "2026-05-31T00:00:01Z"}, t0.Hash)
	if err != nil {
		t.Fatalf("seal t1: %v", err)
	}
	_, err = Fold([]Transition{t0, t1})
	var fe *FoldError
	if !errors.As(err, &fe) {
		t.Fatalf("Fold err = %v, want *FoldError", err)
	}
	if fe.Want != "drafted" || fe.Got != "merged" {
		t.Errorf("FoldError = {want:%q got:%q}, expected {want:drafted got:merged}", fe.Want, fe.Got)
	}
	if fe.Index != 2 {
		t.Errorf("FoldError.Index = %d, want 2", fe.Index)
	}
}

func TestFold_RejectsGenesisNotFromInitial(t *testing.T) {
	t0, err := Seal(Transition{ArtifactID: "art-1", FromState: "drafted", ToState: "reviewed", TS: "2026-05-31T00:00:00Z"}, "")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	_, err = Fold([]Transition{t0})
	var fe *FoldError
	if !errors.As(err, &fe) {
		t.Fatalf("Fold err = %v, want *FoldError for non-Initial genesis", err)
	}
	if fe.Want != InitialState {
		t.Errorf("FoldError.Want = %q, want InitialState", fe.Want)
	}
}

func TestVerifyChain_DetectsTamper(t *testing.T) {
	log := buildLog(t, [][3]string{
		{"art-1", InitialState, "drafted"},
		{"art-1", "drafted", "reviewed"},
	})
	if idx, err := VerifyChain(log); err != nil {
		t.Fatalf("clean chain VerifyChain = (%d,%v), want (0,nil)", idx, err)
	}

	// Tamper with the second record's to_state without re-sealing.
	tampered := append([]Transition(nil), log...)
	tampered[1].ToState = "merged"
	idx, err := VerifyChain(tampered)
	if err == nil {
		t.Fatal("VerifyChain accepted tampered chain")
	}
	if idx != 2 {
		t.Errorf("VerifyChain first-broken index = %d, want 2", idx)
	}
}

func TestVerifyChain_DetectsBrokenPrevHashLink(t *testing.T) {
	log := buildLog(t, [][3]string{
		{"art-1", InitialState, "drafted"},
		{"art-1", "drafted", "reviewed"},
	})
	broken := append([]Transition(nil), log...)
	broken[1].PrevHash = "deadbeef"
	idx, err := VerifyChain(broken)
	if err == nil {
		t.Fatal("VerifyChain accepted broken prev_hash link")
	}
	if idx != 2 {
		t.Errorf("first-broken index = %d, want 2", idx)
	}
}

// TestFoldVerified_RejectsTamperBeforeFold proves the integrity-checked
// projection refuses to derive state from a tampered chain.
func TestFoldVerified_RejectsTamperBeforeFold(t *testing.T) {
	log := buildLog(t, [][3]string{
		{"art-1", InitialState, "drafted"},
		{"art-1", "drafted", "reviewed"},
	})
	if _, err := FoldVerified(log); err != nil {
		t.Fatalf("clean FoldVerified: %v", err)
	}
	tampered := append([]Transition(nil), log...)
	tampered[1].ToState = "merged"
	if _, err := FoldVerified(tampered); err == nil {
		t.Fatal("FoldVerified accepted tampered chain")
	}
}

func TestStateOf_FoundAndAbsent(t *testing.T) {
	log := buildLog(t, [][3]string{
		{"art-1", InitialState, "drafted"},
	})
	state, found, err := StateOf(log, "art-1")
	if err != nil {
		t.Fatalf("StateOf art-1: %v", err)
	}
	if !found || state != "drafted" {
		t.Errorf("StateOf(art-1) = (%q,%v), want (drafted,true)", state, found)
	}
	_, found, err = StateOf(log, "art-missing")
	if err != nil {
		t.Fatalf("StateOf missing: %v", err)
	}
	if found {
		t.Error("StateOf(art-missing) found = true, want false")
	}
}

// TestAppend_DoesNotMutateInput guards the immutability contract: Append builds
// a new slice and never writes through to the caller's log.
func TestAppend_DoesNotMutateInput(t *testing.T) {
	orig := buildLog(t, [][3]string{
		{"art-1", InitialState, "drafted"},
	})
	before := len(orig)
	_, err := Append(orig, Transition{ArtifactID: "art-1", FromState: "drafted", ToState: "reviewed", TS: "2026-05-31T00:00:01Z"})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if len(orig) != before {
		t.Errorf("Append mutated input length: %d != %d", len(orig), before)
	}
}

// TestFold_TieBreakByHashIsDeterministic proves replay order is total even when
// two transitions for the same artifact share a timestamp: the hash tie-break
// must impose one stable order, so Fold either succeeds identically every run
// or fails identically — never flaps.
func TestFold_TieBreakByHashIsDeterministic(t *testing.T) {
	// Two same-ts transitions from the same Initial state to different targets
	// is an illegal fork; the point is that the OUTCOME is stable, not random.
	a, err := Seal(Transition{ArtifactID: "art-1", FromState: InitialState, ToState: "alpha", TS: "2026-05-31T00:00:00Z"}, "")
	if err != nil {
		t.Fatalf("seal a: %v", err)
	}
	b, err := Seal(Transition{ArtifactID: "art-1", FromState: InitialState, ToState: "beta", TS: "2026-05-31T00:00:00Z"}, "")
	if err != nil {
		t.Fatalf("seal b: %v", err)
	}
	_, err1 := Fold([]Transition{a, b})
	_, err2 := Fold([]Transition{b, a})
	if (err1 == nil) != (err2 == nil) {
		t.Errorf("Fold tie-break not deterministic: err1=%v err2=%v", err1, err2)
	}
}
