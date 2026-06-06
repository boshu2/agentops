package canon

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTieredLearning(t *testing.T, dir, name, tier string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	fm := "---\nauthor: Alice\nauthor_email: alice@example.com\n"
	if tier != "" {
		fm += "canon_tier: " + tier + "\n"
	}
	fm += "---\n\nbody\n"
	if err := os.WriteFile(path, []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTierOf(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name, tier string
		want       Tier
	}{
		{"explicit heuristic", "heuristic", TierHeuristic},
		{"explicit falsifiable", "falsifiable", TierFalsifiable},
		{"unmarked defaults falsifiable", "", TierFalsifiable},
		{"garbage defaults falsifiable", "banana", TierFalsifiable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeTieredLearning(t, dir, tt.name+".md", tt.tier)
			if got := TierOf(p); got != tt.want {
				t.Errorf("TierOf = %q, want %q", got, tt.want)
			}
		})
	}
	if got := TierOf(filepath.Join(dir, "missing.md")); got != TierFalsifiable {
		t.Errorf("TierOf(missing) = %q, want falsifiable", got)
	}
}

func TestGateForThresholds(t *testing.T) {
	if g := GateFor(TierFalsifiable); g.MinCitations != 1 || g.MinVerifications != 1 {
		t.Errorf("falsifiable gate = %+v, want 1 cite / 1 verif", g)
	}
	if g := GateFor(TierHeuristic); g.MinCitations != 3 || g.MinVerifications != 0 {
		t.Errorf("heuristic gate = %+v, want 3 cites / 0 verif", g)
	}
}

// TestHeuristicEarnsOnCitationsAlone: a judgment learning can't be verified, so
// 3 cross-engineer citations (no verification) promote it; 2 do not.
func TestHeuristicEarnsOnCitationsAlone(t *testing.T) {
	dir := t.TempDir()
	entry := writeTieredLearning(t, dir, "h.md", "heuristic")
	cl := NewCitationLedger(filepath.Join(dir, "c.jsonl"))
	vl := NewVerificationLedger(filepath.Join(dir, "v.jsonl"))

	citers := []Identity{bob, carol, {Name: "Dave", Email: "dave@x"}}
	for i, who := range citers {
		if _, err := cl.Record("h", entry, "q", "s", who, clock); err != nil {
			t.Fatal(err)
		}
		d, err := EvaluateEntry("h", entry, cl, vl)
		if err != nil {
			t.Fatal(err)
		}
		wantEligible := i == 2 // eligible only at the 3rd distinct citation
		if d.Eligible != wantEligible {
			t.Errorf("after %d citations: eligible=%v want %v (%+v)", i+1, d.Eligible, wantEligible, d)
		}
		if d.Tier != TierHeuristic {
			t.Errorf("tier = %q, want heuristic", d.Tier)
		}
	}
}

// TestFalsifiableStillNeedsVerification: even with 3 citations, a falsifiable
// learning is NOT eligible without an independent verification.
func TestFalsifiableStillNeedsVerification(t *testing.T) {
	dir := t.TempDir()
	entry := writeTieredLearning(t, dir, "f.md", "falsifiable")
	cl := NewCitationLedger(filepath.Join(dir, "c.jsonl"))
	vl := NewVerificationLedger(filepath.Join(dir, "v.jsonl"))

	for _, who := range []Identity{bob, carol, {Name: "Dave", Email: "dave@x"}} {
		if _, err := cl.Record("f", entry, "q", "s", who, clock); err != nil {
			t.Fatal(err)
		}
	}
	d, err := EvaluateEntry("f", entry, cl, vl)
	if err != nil {
		t.Fatal(err)
	}
	if d.Eligible {
		t.Errorf("falsifiable with 3 citations but 0 verification should NOT be eligible: %+v", d)
	}

	// One independent verification clears it.
	if _, err := vl.Record("f", entry, "council", "gate.log:L1", VerdictConfirmed, bob, clock); err != nil {
		t.Fatal(err)
	}
	d, err = EvaluateEntry("f", entry, cl, vl)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Eligible {
		t.Errorf("falsifiable with citation + verification should be eligible: %+v", d)
	}
}

// TestUnknownPathDefaultsStrict: with no path (tier unknown), the strict
// falsifiable gate applies — unverifiable entries can't slip through.
func TestUnknownPathDefaultsStrict(t *testing.T) {
	dir := t.TempDir()
	entry := writeTieredLearning(t, dir, "h.md", "heuristic")
	cl := NewCitationLedger(filepath.Join(dir, "c.jsonl"))
	vl := NewVerificationLedger(filepath.Join(dir, "v.jsonl"))
	for _, who := range []Identity{bob, carol, {Name: "Dave", Email: "dave@x"}} {
		if _, err := cl.Record("h", entry, "q", "s", who, clock); err != nil {
			t.Fatal(err)
		}
	}
	// Path empty → falls back to falsifiable → needs verification despite 3 cites.
	d, err := EvaluateEntry("h", "", cl, vl)
	if err != nil {
		t.Fatal(err)
	}
	if d.Tier != TierFalsifiable || d.Eligible {
		t.Errorf("empty path must default to strict falsifiable, not eligible: %+v", d)
	}
}
