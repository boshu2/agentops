package gates

import (
	"context"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func backingCheck(id string) Check {
	return Check{ID: id, Tiers: Fast | Full, Backing: "x"}
}

func TestRegistry_AllSortedByID(t *testing.T) {
	r := NewRegistry()
	for _, id := range []string{"go.vet", "a.b", "schema.x"} {
		if err := r.Add(backingCheck(id)); err != nil {
			t.Fatalf("Add(%q): %v", id, err)
		}
	}
	got := r.All()
	want := []string{"a.b", "go.vet", "schema.x"}
	if len(got) != len(want) {
		t.Fatalf("All() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("All()[%d].ID = %q, want %q", i, got[i].ID, want[i])
		}
	}
}

func TestRegistry_DuplicateIDRejected(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(backingCheck("dup")); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := r.Add(backingCheck("dup")); err == nil {
		t.Fatal("duplicate Add: want error, got nil")
	}
	if r.Len() != 1 {
		t.Fatalf("Len() = %d, want 1 (duplicate must not be stored)", r.Len())
	}
}

func TestRegistry_GetReturnsRegistered(t *testing.T) {
	r := NewRegistry()
	in := backingCheck("go.build")
	if err := r.Add(in); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, ok := r.Get("go.build")
	if !ok {
		t.Fatal("Get: want found")
	}
	if got.ID != "go.build" {
		t.Fatalf("Get().ID = %q, want go.build", got.ID)
	}
	if _, ok := r.Get("missing"); ok {
		t.Fatal("Get(missing): want not found")
	}
}

func TestCheck_Validate(t *testing.T) {
	native := func(context.Context, RunContext) (ports.GateVerdict, error) {
		return ports.GateVerdict{Status: ports.GateStatusPass}, nil
	}
	tests := []struct {
		name    string
		check   Check
		wantErr bool
	}{
		{"valid backing", Check{ID: "a", Tiers: Fast, Backing: "s"}, false},
		{"valid native run", Check{ID: "a", Tiers: Full, Run: native}, false},
		{"empty id", Check{Tiers: Fast, Backing: "s"}, true},
		{"no tiers", Check{ID: "a", Backing: "s"}, true},
		{"both backing and run", Check{ID: "a", Tiers: Fast, Backing: "s", Run: native}, true},
		{"neither backing nor run", Check{ID: "a", Tiers: Fast}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.check.validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestTier_Has(t *testing.T) {
	if !(Fast | Full).Has(Fast) {
		t.Fatal("Fast|Full should have Fast")
	}
	if !(Fast | Full).Has(Full) {
		t.Fatal("Fast|Full should have Full")
	}
	if Full.Has(Fast) {
		t.Fatal("Full alone should not have Fast")
	}
}

func TestCheck_AlwaysRun(t *testing.T) {
	if !(Check{Match: nil}).AlwaysRun() {
		t.Fatal("nil Match should be AlwaysRun")
	}
	if !(Check{Match: []string{}}).AlwaysRun() {
		t.Fatal("empty Match should be AlwaysRun")
	}
	if (Check{Match: []string{"*.go"}}).AlwaysRun() {
		t.Fatal("non-empty Match should not be AlwaysRun")
	}
}
