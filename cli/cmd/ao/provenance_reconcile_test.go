package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// writeReconcileVerdict writes a minimal pawl-verdict JSON file into dir.
func writeReconcileVerdict(t *testing.T, dir, bead, sha, disp string) {
	t.Helper()
	body := `{"bead_id":"` + bead + `","head_sha":"` + sha + `","disposition":"` + disp + `"}`
	if err := os.WriteFile(filepath.Join(dir, bead+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write verdict %s: %v", bead, err)
	}
}

// TestScanReconcileVerdicts exercises the core detection: a CONFIRMED verdict with a bound edge
// reads BOUND; one without reads UNBOUND; a REFUTED verdict is skipped (not a desync); a malformed
// file surfaces as unbound. This is the primary reconcile value — finding the file/edge desync.
func TestScanReconcileVerdicts(t *testing.T) {
	dir := t.TempDir()
	ledgerDir := t.TempDir()
	ledger := filepath.Join(ledgerDir, "ledger.jsonl")

	const boundSHA = "aaaaaaa111111112222222333333344444445555"
	const unboundSHA = "bbbbbbb111111112222222333333344444445555"
	const refutedSHA = "ccccccc111111112222222333333344444445555"
	const reboundSHA = "ddddddd111111112222222333333344444445555"

	// A genuine CONFIRMED edge and a genuine REBOUND edge in the ledger (both "bound").
	store := provenancegraph.NewStore(ledger)
	if _, err := store.Append(provenancegraph.Edge{
		FromID: "age-bound@" + boundSHA[:7], FromType: "verdict",
		ToID: boundSHA, ToType: "commit", Relation: "wasDerivedFrom",
		EvidenceRef: "pawl-verdict age-bound disposition=CONFIRMED", TrustTier: "inferred",
		TS: "2026-07-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("append CONFIRMED edge: %v", err)
	}
	if _, err := store.Append(provenancegraph.Edge{
		FromID: "age-rebound@" + reboundSHA[:7], FromType: "verdict",
		ToID: reboundSHA, ToType: "commit", Relation: "wasDerivedFrom",
		EvidenceRef: "pawl-verdict age-rebound disposition=REBOUND", TrustTier: "inferred",
		TS: "2026-07-02T00:00:01Z",
	}); err != nil {
		t.Fatalf("append REBOUND edge: %v", err)
	}

	writeReconcileVerdict(t, dir, "age-bound", boundSHA, "CONFIRMED")     // CONFIRMED edge -> BOUND
	writeReconcileVerdict(t, dir, "age-rebound", reboundSHA, "REBOUND")   // REBOUND edge -> BOUND (Gemini-refute regression)
	writeReconcileVerdict(t, dir, "age-unbound", unboundSHA, "CONFIRMED") // no edge -> UNBOUND (the desync)
	writeReconcileVerdict(t, dir, "age-refuted", refutedSHA, "REFUTED")   // not authorizing -> skipped
	// A malformed verdict file -> surfaced as unbound, not a scan-abort.
	if err := os.WriteFile(filepath.Join(dir, "age-broken.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := scanReconcileVerdicts(dir, ledger)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	byBead := map[string]reconcileRow{}
	for _, r := range rows {
		byBead[r.BeadID] = r
	}
	// REFUTED is skipped entirely.
	if _, ok := byBead["age-refuted"]; ok {
		t.Errorf("REFUTED verdict must be skipped, not reported")
	}
	if r, ok := byBead["age-bound"]; !ok || !r.EdgeBound {
		t.Errorf("age-bound: want reported BOUND, got %+v (present=%v)", r, ok)
	}
	if r, ok := byBead["age-unbound"]; !ok || r.EdgeBound {
		t.Errorf("age-unbound: want reported UNBOUND, got %+v (present=%v)", r, ok)
	}
	// The malformed file is present as an unbound row (empty bead id -> keyed under "").
	if r, ok := byBead[""]; !ok || r.EdgeBound {
		t.Errorf("malformed verdict file must surface as an unbound row, got %+v (present=%v)", r, ok)
	}

	unbound := 0
	for _, r := range rows {
		if !r.EdgeBound {
			unbound++
		}
	}
	if unbound != 2 { // age-unbound + age-broken
		t.Errorf("unbound count = %d, want 2 (age-unbound + malformed)", unbound)
	}
}

// TestScanReconcileVerdicts_EmptyDir: no verdicts dir is clean (nil, no error), never a crash.
func TestScanReconcileVerdicts_EmptyDir(t *testing.T) {
	rows, err := scanReconcileVerdicts(filepath.Join(t.TempDir(), "does-not-exist"), filepath.Join(t.TempDir(), "ledger.jsonl"))
	if err != nil {
		t.Fatalf("missing dir should be clean, got err: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("missing dir should yield 0 rows, got %d", len(rows))
	}
}
