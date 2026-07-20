// practices: [design-by-contract, in-toto-provenance]
package provenance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// fixedNow is the deterministic clock the test module uses for the default add
// timestamp, standing in for the host's time.Now seam.
var fixedNow = time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)

// testLedger returns a ledger path under a fresh temp dir; the store creates the
// intermediate directories on first append.
func testLedger(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), provenancegraph.LedgerRelativePath)
}

// newTestModule builds a provenance Module whose ledger-path and clock seams are
// pinned for the test, standing in for the package-main host wiring.
func newTestModule(ledger string) *Module {
	return NewModule(HostOptions{
		LedgerPath: func() string { return ledger },
		Now:        func() time.Time { return fixedNow },
	})
}

// execProv constructs a fresh provenance module (clean flag state) and runs the
// command tree with the given args, capturing stdout+stderr. It replaces the
// former cmd/ao executeCommand harness for this carved family.
func execProv(t *testing.T, ledger string, args ...string) (string, error) {
	t.Helper()
	root := newTestModule(ledger).Command()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	// Cobra prints "Error: ..." to stderr on a non-nil RunE; keep it out of the
	// captured stdout so callers parse only what the command emitted.
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestModuleConstructsCommandTree(t *testing.T) {
	root := newTestModule("x").Command()
	if root.Name() != "provenance" {
		t.Fatalf("root command = %q, want provenance", root.Name())
	}
	want := map[string]bool{
		"add": true, "list": true, "export": true, "show": true,
		"position": true, "trace": true, "verify": true, "mine-session": true,
	}
	got := map[string]bool{}
	for _, c := range root.Commands() {
		got[c.Name()] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("subcommand set = %v, want %v", got, want)
	}
}

func TestProvenanceAdd_ProducesSchemaValidEdgeAndReadsBack(t *testing.T) {
	ledger := testLedger(t)

	out, err := execProv(t, ledger, "add", "ag-x31t.4", "cli/cmd/ao/provenance_add.go",
		"--relation", "wasGeneratedBy", "--json")
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	var emitted provenancegraph.Edge
	if err := json.Unmarshal([]byte(out), &emitted); err != nil {
		t.Fatalf("add output not an Edge JSON: %v\n%s", err, out)
	}

	// Emitted edge must be field-valid against the v1 vocabulary and sealed.
	if err := provenancegraph.ValidateFields(emitted); err != nil {
		t.Fatalf("emitted edge invalid: %v", err)
	}
	if emitted.SchemaVersion != provenancegraph.SchemaVersion {
		t.Fatalf("schema_version = %q, want %q", emitted.SchemaVersion, provenancegraph.SchemaVersion)
	}
	if emitted.PrevHash != "" {
		t.Fatalf("genesis prev_hash = %q, want empty", emitted.PrevHash)
	}
	if len(emitted.Hash) != 64 || len(emitted.PayloadHash) != 64 {
		t.Fatalf("hash shape: payload=%d hash=%d, want 64", len(emitted.PayloadHash), len(emitted.Hash))
	}

	// The ledger file is at the resolved path.
	if _, err := os.Stat(ledger); err != nil {
		t.Fatalf("ledger not written at %s: %v", ledger, err)
	}

	// list reads it back.
	out2, err := execProv(t, ledger, "list", "--json")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var listed []provenancegraph.Edge
	if err := json.Unmarshal([]byte(out2), &listed); err != nil {
		t.Fatalf("list output not an Edge array: %v\n%s", err, out2)
	}
	if len(listed) != 1 {
		t.Fatalf("list returned %d edges, want 1", len(listed))
	}
	if listed[0].Hash != emitted.Hash {
		t.Fatalf("round-trip hash mismatch: %q vs %q", listed[0].Hash, emitted.Hash)
	}
}

func TestProvenanceAdd_Idempotent(t *testing.T) {
	ledger := testLedger(t)

	if _, err := execProv(t, ledger, "add", "ag-x31t.4", "out.go", "--relation", "wasGeneratedBy"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	// Re-add same edge with a different timestamp.
	out2, err := execProv(t, ledger, "add", "ag-x31t.4", "out.go", "--relation", "wasGeneratedBy",
		"--ts", "2026-06-01T00:00:00Z")
	if err != nil {
		t.Fatalf("second add: %v", err)
	}
	if !bytes.Contains([]byte(out2), []byte("idempotent")) {
		t.Fatalf("second add output = %q, want idempotent no-op notice", out2)
	}

	// list shows exactly one edge.
	out3, err := execProv(t, ledger, "list", "--json")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var listed []provenancegraph.Edge
	if err := json.Unmarshal([]byte(out3), &listed); err != nil {
		t.Fatalf("list parse: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("idempotent add left %d edges, want 1", len(listed))
	}
}

func TestProvenanceAdd_RejectsInvalidRelation(t *testing.T) {
	ledger := testLedger(t)
	if _, err := execProv(t, ledger, "add", "ag-x31t.4", "out.go", "--relation", "frobnicates"); err == nil {
		t.Fatal("expected error for invalid relation")
	}
}

func TestProvenanceList_FiltersByFromIDAndRelation(t *testing.T) {
	ledger := testLedger(t)

	// Edge 1: decision -> artifact.
	if _, err := execProv(t, ledger, "add", "ag-x31t.4", "a.go", "--relation", "wasGeneratedBy"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	// Edge 2: a different decision, different relation.
	if _, err := execProv(t, ledger, "add", "soc-byl.3", "ag-x31t",
		"--relation", "wasAssociatedWith", "--to-type", "bead"); err != nil {
		t.Fatalf("add 2: %v", err)
	}

	// Filter by from-id.
	out3, err := execProv(t, ledger, "list", "--json", "--from-id", "soc-byl.3")
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	var byID []provenancegraph.Edge
	if err := json.Unmarshal([]byte(out3), &byID); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(byID) != 1 || byID[0].FromID != "soc-byl.3" {
		t.Fatalf("from-id filter wrong: %+v", byID)
	}

	// Filter by relation.
	out4, err := execProv(t, ledger, "list", "--json", "--relation", "wasGeneratedBy")
	if err != nil {
		t.Fatalf("list rel: %v", err)
	}
	var byRel []provenancegraph.Edge
	if err := json.Unmarshal([]byte(out4), &byRel); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(byRel) != 1 || byRel[0].Relation != "wasGeneratedBy" {
		t.Fatalf("relation filter wrong: %+v", byRel)
	}
}
