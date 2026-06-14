// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

func resetProvVerifyFlags() { provVerifyJSON = false }

// TestProvenanceVerify_IntactLedgerPasses: two appended events form an intact
// chain; verify exits nil and reports OK with the record count.
func TestProvenanceVerify_IntactLedgerPasses(t *testing.T) {
	chdirRepoFixture(t)
	seedLedger(t) // 3 distinct edges (shared helper from provenance_export_test.go)
	resetProvVerifyFlags()

	c, out := provTestCmd()
	if err := runProvenanceVerify(c, nil); err != nil {
		t.Fatalf("verify intact ledger returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "OK:") || !strings.Contains(got, "3 record") {
		t.Fatalf("verify summary = %q, want OK with 3 records", got)
	}
}

// TestProvenanceVerify_EmptyLedgerPasses: a fresh repo with no ledger file must
// not fail the gate.
func TestProvenanceVerify_EmptyLedgerPasses(t *testing.T) {
	chdirRepoFixture(t)
	resetProvVerifyFlags()

	c, out := provTestCmd()
	if err := runProvenanceVerify(c, nil); err != nil {
		t.Fatalf("verify empty ledger returned error: %v", err)
	}
	if !strings.Contains(out.String(), "0 record") {
		t.Fatalf("verify empty = %q, want 0 records", out.String())
	}
}

// TestProvenanceVerify_TamperedLedgerFails is the CLI-level windshield test:
// flip a committed payload field WITHOUT recomputing hashes and verify must
// return a non-nil error naming the broken line.
func TestProvenanceVerify_TamperedLedgerFails(t *testing.T) {
	path := chdirRepoFixture(t)
	seedLedger(t)
	resetProvVerifyFlags()

	// Read raw lines, tamper line 2's to_id, write back with stale hashes.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 ledger lines, got %d", len(lines))
	}
	var e provenancegraph.Edge
	if err := json.Unmarshal([]byte(lines[1]), &e); err != nil {
		t.Fatalf("unmarshal line 2: %v", err)
	}
	e.ToID = "tampered.go"
	b, _ := json.Marshal(e)
	lines[1] = string(b)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("rewrite ledger: %v", err)
	}

	c, out := provTestCmd()
	err = runProvenanceVerify(c, nil)
	if err == nil {
		t.Fatal("TAMPER NOT CAUGHT: verify returned nil on a tampered ledger")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error = %q, want it to name line 2", err.Error())
	}
	if !strings.Contains(out.String(), "BROKEN") || !strings.Contains(out.String(), "line 2") {
		t.Fatalf("stdout = %q, want BROKEN at line 2", out.String())
	}
}

// TestProvenanceVerify_JSONResult: --json emits the structured VerifyResult.
func TestProvenanceVerify_JSONResult(t *testing.T) {
	chdirRepoFixture(t)
	seedLedger(t)
	resetProvVerifyFlags()
	provVerifyJSON = true

	c, out := provTestCmd()
	if err := runProvenanceVerify(c, nil); err != nil {
		t.Fatalf("verify --json: %v", err)
	}
	var res provenancegraph.VerifyResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("--json not a VerifyResult: %v\n%s", err, out.String())
	}
	if !res.Pass || res.RecordCount != 3 {
		t.Fatalf("VerifyResult = %+v, want Pass=true count=3", res)
	}
}
