// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// provTestCmd returns a fresh cobra command with captured stdout.
func provTestCmd() (*cobra.Command, *bytes.Buffer) {
	var out bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&out)
	c.SetErr(&out)
	return c, &out
}

// chdirRepoFixture creates a minimal repo-shaped tmp dir (docs/ + schemas/),
// chdirs into it, and returns the ledger path. Restores cwd on cleanup.
func chdirRepoFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"docs", "schemas"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return filepath.Join(dir, provenancegraph.LedgerRelativePath)
}

// resetProvAddFlags sets the add flags to a known baseline for a test.
func resetProvAddFlags() {
	provAddRelation = "wasGeneratedBy"
	provAddFromType = "decision"
	provAddToType = "artifact"
	provAddTrustTier = "authored"
	provAddEvidence = ""
	provAddTS = "2026-05-31T00:00:00Z"
	provAddJSON = false
}

func resetProvListFlags() {
	provListJSON = false
	provListFromID = ""
	provListRelation = ""
}

func resetProvVerifyFlags() {
	provVerifyJSON = false
}

func TestProvenanceAdd_ProducesSchemaValidEdgeAndReadsBack(t *testing.T) {
	ledger := chdirRepoFixture(t)
	resetProvAddFlags()
	provAddJSON = true

	c, out := provTestCmd()
	if err := runProvenanceAdd(c, []string{"ag-x31t.4", "cli/cmd/ao/provenance_add.go"}); err != nil {
		t.Fatalf("add: %v", err)
	}

	var emitted provenancegraph.Edge
	if err := json.Unmarshal(out.Bytes(), &emitted); err != nil {
		t.Fatalf("add output not an Edge JSON: %v\n%s", err, out.String())
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

	// The ledger file is at the resolved repo-relative path.
	if _, err := os.Stat(ledger); err != nil {
		t.Fatalf("ledger not written at %s: %v", ledger, err)
	}

	// list reads it back.
	resetProvListFlags()
	provListJSON = true
	c2, out2 := provTestCmd()
	if err := runProvenanceList(c2, nil); err != nil {
		t.Fatalf("list: %v", err)
	}
	var listed []provenancegraph.Edge
	if err := json.Unmarshal(out2.Bytes(), &listed); err != nil {
		t.Fatalf("list output not an Edge array: %v\n%s", err, out2.String())
	}
	if len(listed) != 1 {
		t.Fatalf("list returned %d edges, want 1", len(listed))
	}
	if listed[0].Hash != emitted.Hash {
		t.Fatalf("round-trip hash mismatch: %q vs %q", listed[0].Hash, emitted.Hash)
	}
}

func TestProvenanceAdd_Idempotent(t *testing.T) {
	chdirRepoFixture(t)
	resetProvAddFlags()

	c1, _ := provTestCmd()
	if err := runProvenanceAdd(c1, []string{"ag-x31t.4", "out.go"}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	// Re-add same edge with a different timestamp.
	provAddTS = "2026-06-01T00:00:00Z"
	c2, out2 := provTestCmd()
	if err := runProvenanceAdd(c2, []string{"ag-x31t.4", "out.go"}); err != nil {
		t.Fatalf("second add: %v", err)
	}
	if got := out2.String(); !bytes.Contains([]byte(got), []byte("idempotent")) {
		t.Fatalf("second add output = %q, want idempotent no-op notice", got)
	}

	// list shows exactly one edge.
	resetProvListFlags()
	provListJSON = true
	c3, out3 := provTestCmd()
	if err := runProvenanceList(c3, nil); err != nil {
		t.Fatalf("list: %v", err)
	}
	var listed []provenancegraph.Edge
	if err := json.Unmarshal(out3.Bytes(), &listed); err != nil {
		t.Fatalf("list parse: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("idempotent add left %d edges, want 1", len(listed))
	}
}

func TestProvenanceAdd_RejectsInvalidRelation(t *testing.T) {
	chdirRepoFixture(t)
	resetProvAddFlags()
	provAddRelation = "frobnicates"

	c, _ := provTestCmd()
	if err := runProvenanceAdd(c, []string{"ag-x31t.4", "out.go"}); err == nil {
		t.Fatal("expected error for invalid relation")
	}
}

func TestProvenanceList_FiltersByFromIDAndRelation(t *testing.T) {
	chdirRepoFixture(t)

	// Edge 1: decision -> artifact.
	resetProvAddFlags()
	c1, _ := provTestCmd()
	if err := runProvenanceAdd(c1, []string{"ag-x31t.4", "a.go"}); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	// Edge 2: a different decision, different relation.
	resetProvAddFlags()
	provAddRelation = "wasAssociatedWith"
	provAddToType = "bead"
	c2, _ := provTestCmd()
	if err := runProvenanceAdd(c2, []string{"soc-byl.3", "ag-x31t"}); err != nil {
		t.Fatalf("add 2: %v", err)
	}

	// Filter by from-id.
	resetProvListFlags()
	provListJSON = true
	provListFromID = "soc-byl.3"
	c3, out3 := provTestCmd()
	if err := runProvenanceList(c3, nil); err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	var byID []provenancegraph.Edge
	if err := json.Unmarshal(out3.Bytes(), &byID); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(byID) != 1 || byID[0].FromID != "soc-byl.3" {
		t.Fatalf("from-id filter wrong: %+v", byID)
	}

	// Filter by relation.
	resetProvListFlags()
	provListJSON = true
	provListRelation = "wasGeneratedBy"
	c4, out4 := provTestCmd()
	if err := runProvenanceList(c4, nil); err != nil {
		t.Fatalf("list rel: %v", err)
	}
	var byRel []provenancegraph.Edge
	if err := json.Unmarshal(out4.Bytes(), &byRel); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(byRel) != 1 || byRel[0].Relation != "wasGeneratedBy" {
		t.Fatalf("relation filter wrong: %+v", byRel)
	}
}

func TestProvenanceVerify_EmptyLedgerPasses(t *testing.T) {
	chdirRepoFixture(t)
	resetProvVerifyFlags()
	provVerifyJSON = true

	c, out := provTestCmd()
	if err := runProvenanceVerify(c, nil); err != nil {
		t.Fatalf("verify empty ledger: %v", err)
	}
	var report provenanceVerifyReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("verify output not JSON: %v\n%s", err, out.String())
	}
	if !report.OK || report.Records != 0 {
		t.Fatalf("empty ledger report = %+v, want ok with zero records", report)
	}
}

func TestProvenanceVerify_DetectsTamperedHashChain(t *testing.T) {
	ledger := chdirRepoFixture(t)

	resetProvAddFlags()
	c1, _ := provTestCmd()
	if err := runProvenanceAdd(c1, []string{"ag-1", "a.go"}); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	resetProvAddFlags()
	provAddTS = "2026-05-31T00:01:00Z"
	c2, _ := provTestCmd()
	if err := runProvenanceAdd(c2, []string{"ag-2", "b.go"}); err != nil {
		t.Fatalf("add 2: %v", err)
	}

	raw, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	tampered := strings.Replace(string(raw), `"to_id":"b.go"`, `"to_id":"tampered.go"`, 1)
	if tampered == string(raw) {
		t.Fatalf("test setup failed: did not tamper second edge\n%s", raw)
	}
	if err := os.WriteFile(ledger, []byte(tampered), 0o644); err != nil {
		t.Fatalf("write tampered ledger: %v", err)
	}

	resetProvVerifyFlags()
	provVerifyJSON = true
	c3, out := provTestCmd()
	err = runProvenanceVerify(c3, nil)
	if err == nil {
		t.Fatal("verify should fail on tampered ledger")
	}
	var report provenanceVerifyReport
	if jerr := json.Unmarshal(out.Bytes(), &report); jerr != nil {
		t.Fatalf("verify output not JSON: %v\n%s", jerr, out.String())
	}
	if report.OK || report.Index != 2 || !strings.Contains(report.Error, "payload_hash mismatch") {
		t.Fatalf("tamper report = %+v, want failed record 2 payload mismatch", report)
	}
}
