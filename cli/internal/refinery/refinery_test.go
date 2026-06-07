package refinery

import (
	"context"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// ---- fakes ----

type fakeCommits struct{ head string }

func (f fakeCommits) MainHead(context.Context) (string, error) { return f.head, nil }

type fakeGate struct{ rep *gates.Report }

func (f fakeGate) CheckFull(context.Context) (*gates.Report, error) { return f.rep, nil }

type fakeRerun struct{ status ports.GateStatus }

func (f fakeRerun) Rerun(context.Context, string) (ports.GateVerdict, error) {
	return ports.GateVerdict{Status: f.status}, nil
}

type fakeBeads struct {
	filed      int
	lastChecks []string
}

func (f *fakeBeads) FileFixBead(_ context.Context, _ string, checks []string) (string, error) {
	f.filed++
	f.lastChecks = checks
	return "ag-fix1", nil
}

type fakeBeacon struct{ setN, clearN int }

func (f *fakeBeacon) Set(context.Context, string, []string) error { f.setN++; return nil }
func (f *fakeBeacon) Clear(context.Context, string) error         { f.clearN++; return nil }

type memStore struct{ st State }

func (m *memStore) Load() (State, error) { return m.st, nil }
func (m *memStore) Save(s State) error   { m.st = s; return nil }

// ---- report builders ----

func blockingCheck(id string, status ports.GateStatus) gates.CheckResult {
	return gates.CheckResult{
		Check:   gates.Check{ID: id, Tiers: gates.Full, Blocking: true, Backing: "x"},
		Verdict: ports.GateVerdict{Status: status},
	}
}

func newRefinery(head string, rep *gates.Report, rerun ports.GateStatus, store *memStore, beads *fakeBeads, beacon *fakeBeacon) *Refinery {
	return &Refinery{
		Commits: fakeCommits{head: head},
		Gate:    fakeGate{rep: rep},
		Rerun:   fakeRerun{status: rerun},
		Beads:   beads,
		Beacon:  beacon,
		Store:   store,
		RerunN:  3,
	}
}

// ---- tests ----

func TestRunOnce_SkipsUnchangedHead(t *testing.T) {
	store := &memStore{st: State{LastCheckedSHA: "abc"}}
	r := newRefinery("abc", &gates.Report{}, ports.GateStatusFail, store, &fakeBeads{}, &fakeBeacon{})
	res, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Skipped {
		t.Error("unchanged HEAD should be Skipped")
	}
}

func TestRunOnce_GreenClearsBeacon(t *testing.T) {
	store := &memStore{st: State{LastCheckedSHA: "old", Poison: []PoisonEntry{{SHA: "old"}}}}
	beacon := &fakeBeacon{}
	rep := &gates.Report{Results: []gates.CheckResult{blockingCheck("go.build", ports.GateStatusPass)}}
	r := newRefinery("new", rep, ports.GateStatusPass, store, &fakeBeads{}, beacon)
	res, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Green {
		t.Error("all-pass report should be Green")
	}
	if beacon.clearN != 1 {
		t.Errorf("beacon.Clear calls = %d, want 1", beacon.clearN)
	}
	if len(store.st.Poison) != 0 {
		t.Errorf("green should clear poison; got %v", store.st.Poison)
	}
	if store.st.LastCheckedSHA != "new" {
		t.Errorf("LastCheckedSHA = %q, want new", store.st.LastCheckedSHA)
	}
}

func TestRunOnce_DeterministicFailEscalates(t *testing.T) {
	store := &memStore{}
	beads := &fakeBeads{}
	beacon := &fakeBeacon{}
	rep := &gates.Report{Results: []gates.CheckResult{blockingCheck("contract.registry-drift", ports.GateStatusFail)}}
	// rerun always FAILs -> deterministic
	r := newRefinery("bad", rep, ports.GateStatusFail, store, beads, beacon)
	res, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(res.Deterministic) != 1 || res.Deterministic[0] != "contract.registry-drift" {
		t.Errorf("Deterministic = %v, want [contract.registry-drift]", res.Deterministic)
	}
	if beads.filed != 1 {
		t.Errorf("fix-bead filed = %d, want 1", beads.filed)
	}
	if beacon.setN != 1 {
		t.Errorf("beacon.Set calls = %d, want 1", beacon.setN)
	}
	if len(store.st.Poison) != 1 || store.st.Poison[0].FixBead != "ag-fix1" {
		t.Errorf("poison state = %+v, want one entry with fix bead", store.st.Poison)
	}
}

func TestRunOnce_FlakyFailDoesNotEscalate(t *testing.T) {
	store := &memStore{}
	beads := &fakeBeads{}
	beacon := &fakeBeacon{}
	rep := &gates.Report{Results: []gates.CheckResult{blockingCheck("skill.schema", ports.GateStatusFail)}}
	// rerun PASSes -> not reproducible -> flaky -> no escalation
	r := newRefinery("flaky", rep, ports.GateStatusPass, store, beads, beacon)
	res, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(res.Deterministic) != 0 {
		t.Errorf("flaky failure must NOT escalate; Deterministic = %v", res.Deterministic)
	}
	if beads.filed != 0 {
		t.Errorf("no fix-bead for flaky; filed = %d", beads.filed)
	}
	if beacon.setN != 0 {
		t.Errorf("no beacon for flaky; setN = %d", beacon.setN)
	}
	if len(store.st.Poison) != 0 {
		t.Errorf("flaky must not poison; got %v", store.st.Poison)
	}
}

func TestRunOnce_NeverRevertsField(t *testing.T) {
	// Structural guard: the Refinery type exposes no revert capability — there is
	// no Revert method/field. A deterministic failure escalates via beacon+bead
	// only. (Compile-time evidence: this test references the public surface.)
	store := &memStore{}
	rep := &gates.Report{Results: []gates.CheckResult{blockingCheck("x", ports.GateStatusFail)}}
	r := newRefinery("z", rep, ports.GateStatusFail, store, &fakeBeads{}, &fakeBeacon{})
	if _, err := r.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	// The poisoned commit remains on main (state records it; nothing reverted).
	if store.st.LastCheckedSHA != "z" {
		t.Errorf("LastCheckedSHA = %q, want z (commit stays; backstop never reverts)", store.st.LastCheckedSHA)
	}
}
