package canon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeLearning creates a learning file with the given author frontmatter and
// returns its path.
func writeLearning(t *testing.T, dir, name, authorName, authorEmail string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := "---\nauthor: " + authorName + "\nauthor_email: " + authorEmail + "\n---\n\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write learning: %v", err)
	}
	return path
}

var (
	alice = Identity{Name: "Alice", Email: "alice@example.com"}
	bob   = Identity{Name: "Bob", Email: "bob@example.com"}
	carol = Identity{Name: "Carol", Email: "carol@example.com"}
	clock = time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
)

func TestIdentitySameAs(t *testing.T) {
	tests := []struct {
		name string
		a, b Identity
		want bool
	}{
		{"same email", Identity{"Alice", "a@x.com"}, Identity{"Different", "a@x.com"}, true},
		{"email case-insensitive", Identity{"A", "A@X.com"}, Identity{"B", "a@x.com"}, true},
		{"different email", alice, bob, false},
		{"name match when no email", Identity{Name: "Alice"}, Identity{Name: "alice"}, true},
		{"two zero identities are not the same", Identity{}, Identity{}, false},
		{"zero vs named is not the same", Identity{}, alice, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.SameAs(tt.b); got != tt.want {
				t.Errorf("SameAs = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthorOf(t *testing.T) {
	dir := t.TempDir()
	path := writeLearning(t, dir, "l.md", "Alice", "alice@example.com")
	got := AuthorOf(path)
	if !got.SameAs(alice) {
		t.Errorf("AuthorOf = %+v, want Alice", got)
	}
	if missing := AuthorOf(filepath.Join(dir, "nope.md")); !missing.IsZero() {
		t.Errorf("AuthorOf(missing) = %+v, want zero", missing)
	}
}

// TestEarnedPromotion is the load-bearing L2 test: it drives the full gate
// across the realistic lifecycle and asserts the exact eligibility at each step.
func TestEarnedPromotion(t *testing.T) {
	dir := t.TempDir()
	entry := writeLearning(t, dir, "til.md", "Alice", "alice@example.com")
	cl := NewCitationLedger(filepath.Join(dir, "citations.jsonl"))
	vl := NewVerificationLedger(filepath.Join(dir, "verifications.jsonl"))
	gate := DefaultGate()

	mustEval := func() Decision {
		t.Helper()
		d, err := gate.Evaluate("til", cl, vl)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		return d
	}

	// Step 0: nothing recorded → not eligible, both signals missing.
	if d := mustEval(); d.Eligible || len(d.Unmet) != 2 {
		t.Fatalf("fresh entry: eligible=%v unmet=%v, want not-eligible with 2 unmet", d.Eligible, d.Unmet)
	}

	// Step 1: the AUTHOR cites and verifies her own entry. Self-attestations
	// must not move the gate at all.
	if _, err := cl.Record("til", entry, "q", "s1", alice, clock); err != nil {
		t.Fatal(err)
	}
	if _, err := vl.Record("til", entry, "manual", "ran it", VerdictConfirmed, alice, clock); err != nil {
		t.Fatal(err)
	}
	if d := mustEval(); d.Eligible || d.Citations != 0 || d.Verifications != 0 {
		t.Fatalf("self-attestation counted: %+v, want citations=0 verifications=0 not-eligible", d)
	}

	// Step 2: Bob cites it (useful) but no independent verification yet.
	if _, err := cl.Record("til", entry, "q", "s2", bob, clock); err != nil {
		t.Fatal(err)
	}
	if d := mustEval(); d.Eligible || d.Citations != 1 || d.Verifications != 0 {
		t.Fatalf("citation-only: %+v, want citations=1 verifications=0 not-eligible", d)
	}

	// Step 3: Carol independently confirms it → both signals present → earned.
	if _, err := vl.Record("til", entry, "ao-verify", "gate.log:L42", VerdictConfirmed, carol, clock); err != nil {
		t.Fatal(err)
	}
	if d := mustEval(); !d.Eligible || d.Citations != 1 || d.Verifications != 1 {
		t.Fatalf("earned: %+v, want citations=1 verifications=1 eligible", d)
	}
}

func TestRefutationHardBlocks(t *testing.T) {
	dir := t.TempDir()
	entry := writeLearning(t, dir, "bad.md", "Alice", "alice@example.com")
	cl := NewCitationLedger(filepath.Join(dir, "c.jsonl"))
	vl := NewVerificationLedger(filepath.Join(dir, "v.jsonl"))

	// Plenty of citations and a confirmation — would otherwise pass.
	if _, err := cl.Record("bad", entry, "", "", bob, clock); err != nil {
		t.Fatal(err)
	}
	if _, err := vl.Record("bad", entry, "manual", "r", VerdictConfirmed, carol, clock); err != nil {
		t.Fatal(err)
	}
	// But Bob independently refutes it.
	if _, err := vl.Record("bad", entry, "manual", "found counterexample", VerdictRefuted, bob, clock); err != nil {
		t.Fatal(err)
	}

	d, err := DefaultGate().Evaluate("bad", cl, vl)
	if err != nil {
		t.Fatal(err)
	}
	if d.Eligible || !d.Refuted {
		t.Errorf("refuted entry: eligible=%v refuted=%v, want not-eligible refuted", d.Eligible, d.Refuted)
	}
}

func TestDistinctEngineersRequired(t *testing.T) {
	dir := t.TempDir()
	entry := writeLearning(t, dir, "l.md", "Alice", "alice@example.com")
	cl := NewCitationLedger(filepath.Join(dir, "c.jsonl"))

	// Bob cites three times from different sessions — still ONE engineer.
	for _, s := range []string{"s1", "s2", "s3"} {
		if _, err := cl.Record("e", entry, "q", s, bob, clock); err != nil {
			t.Fatal(err)
		}
	}
	got, err := cl.CrossEngineerCitations("e")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Errorf("CrossEngineerCitations = %d, want 1 (same engineer cannot stack)", got)
	}
}

func TestRequireReceipt(t *testing.T) {
	dir := t.TempDir()
	entry := writeLearning(t, dir, "l.md", "Alice", "alice@example.com")
	vl := NewVerificationLedger(filepath.Join(dir, "v.jsonl"))

	// Bob confirms WITHOUT a receipt; Carol confirms WITH one.
	if _, err := vl.Record("e", entry, "manual", "", VerdictConfirmed, bob, clock); err != nil {
		t.Fatal(err)
	}
	if _, err := vl.Record("e", entry, "ao-verify", "gate.log:L9", VerdictConfirmed, carol, clock); err != nil {
		t.Fatal(err)
	}

	lenient, err := vl.IndependentConfirmations("e", false)
	if err != nil {
		t.Fatal(err)
	}
	if lenient != 2 {
		t.Errorf("lenient confirmations = %d, want 2", lenient)
	}
	strict, err := vl.IndependentConfirmations("e", true)
	if err != nil {
		t.Fatal(err)
	}
	if strict != 1 {
		t.Errorf("receipt-required confirmations = %d, want 1 (Bob's unreceipted dropped)", strict)
	}
}

func TestLedgerSurvivesMalformedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.jsonl")
	entry := writeLearning(t, dir, "l.md", "Alice", "alice@example.com")
	cl := NewCitationLedger(path)
	if _, err := cl.Record("e", entry, "q", "s", bob, clock); err != nil {
		t.Fatal(err)
	}
	// Corrupt the ledger with a junk line, then append a good one.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("{ this is not json\n")
	f.Close()
	if _, err := cl.Record("e", entry, "q", "s", carol, clock); err != nil {
		t.Fatal(err)
	}

	got, err := cl.CrossEngineerCitations("e")
	if err != nil {
		t.Fatalf("load after corruption: %v", err)
	}
	if got != 2 {
		t.Errorf("CrossEngineerCitations = %d, want 2 (malformed line skipped, good ones kept)", got)
	}
}
