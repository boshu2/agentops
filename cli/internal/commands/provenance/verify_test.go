// practices: [design-by-contract, in-toto-provenance]
package provenance

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// TestProvenanceVerify_IntactLedgerPasses: three appended edges form an intact
// chain; verify exits nil and reports OK with the record count.
func TestProvenanceVerify_IntactLedgerPasses(t *testing.T) {
	ledger := testLedger(t)
	seedLedger(t, ledger)

	out, err := execProv(t, ledger, "verify")
	if err != nil {
		t.Fatalf("verify intact ledger returned error: %v", err)
	}
	if !strings.Contains(out, "OK:") || !strings.Contains(out, "3 record") {
		t.Fatalf("verify summary = %q, want OK with 3 records", out)
	}
}

// TestProvenanceVerify_EmptyLedgerPasses: a fresh path with no ledger file must
// not fail the gate.
func TestProvenanceVerify_EmptyLedgerPasses(t *testing.T) {
	ledger := testLedger(t)

	out, err := execProv(t, ledger, "verify")
	if err != nil {
		t.Fatalf("verify empty ledger returned error: %v", err)
	}
	if !strings.Contains(out, "0 record") {
		t.Fatalf("verify empty = %q, want 0 records", out)
	}
}

// TestProvenanceVerify_TamperedLedgerFails is the CLI-level windshield test:
// flip a committed payload field WITHOUT recomputing hashes and verify must
// return a non-nil error naming the broken line.
func TestProvenanceVerify_TamperedLedgerFails(t *testing.T) {
	ledger := testLedger(t)
	seedLedger(t, ledger)

	raw, err := os.ReadFile(ledger)
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
	if err := os.WriteFile(ledger, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("rewrite ledger: %v", err)
	}

	out, err := execProv(t, ledger, "verify")
	if err == nil {
		t.Fatal("TAMPER NOT CAUGHT: verify returned nil on a tampered ledger")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error = %q, want it to name line 2", err.Error())
	}
	if !strings.Contains(out, "BROKEN") || !strings.Contains(out, "line 2") {
		t.Fatalf("stdout = %q, want BROKEN at line 2", out)
	}
}

// TestProvenanceVerify_JSONResult: --json emits the structured VerifyResult.
func TestProvenanceVerify_JSONResult(t *testing.T) {
	ledger := testLedger(t)
	seedLedger(t, ledger)

	out, err := execProv(t, ledger, "verify", "--json")
	if err != nil {
		t.Fatalf("verify --json: %v", err)
	}
	var res provenancegraph.VerifyResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("--json not a VerifyResult: %v\n%s", err, out)
	}
	if !res.Pass || res.RecordCount != 3 {
		t.Fatalf("VerifyResult = %+v, want Pass=true count=3", res)
	}
}
