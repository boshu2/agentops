package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/extract"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/storage"
)

// withForgeTypedClient overrides the forgeTypedClient package var for the test
// duration and restores it, so production wiring isn't perturbed between tests.
func withForgeTypedClient(t *testing.T, c *extract.Client) {
	t.Helper()
	prev := forgeTypedClient
	forgeTypedClient = func() *extract.Client { return c }
	t.Cleanup(func() { forgeTypedClient = prev })
}

// TestForge_TypedWiredToCodexInProduction asserts the production seam returns a
// non-nil, BackendCodex-backed client (ac-b1.1). No live model is invoked — we
// only inspect the constructed client's backend, which must be on the LAW-0
// allowlist and never claude.
func TestForge_TypedWiredToCodexInProduction(t *testing.T) {
	client := forgeTypedClient()
	if client == nil {
		t.Fatal("forgeTypedClient() returned nil in production; the --typed wire is dead")
	}
	if client.Backend() != extract.BackendCodex {
		t.Fatalf("forgeTypedClient backend = %q, want %q", client.Backend(), extract.BackendCodex)
	}
	if !client.Backend().IsAllowed() {
		t.Fatalf("forgeTypedClient backend %q is not on the LAW-0 allowlist", client.Backend())
	}
	if string(client.Backend()) == "claude" {
		t.Fatal("LAW 0 violation: forgeTypedClient backed by claude")
	}
}

// TestForge_TypedEmitsViaInjectedFake exercises the --typed path end-to-end with
// an injected fake Generator (no live model): it must emit typed learning
// records parsed from the model output (ac-b1.2). The fake returns two entities
// and one valid PROV-O relation.
func TestForge_TypedEmitsViaInjectedFake(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:        "typed-functional-session",
		Date:      time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{"The pre-push gate rewrites the landed commit."},
		Decisions: []string{"Wire the typed extractor behind a value gate."},
	}

	fakeJSON := `{
	  "entities": [
	    {"node_type": "decision", "id": "decision-value-gate", "summary": "Wire typed extractor behind a value gate"},
	    {"node_type": "artifact", "id": "cli/cmd/ao/forge_typed.go", "summary": "Typed opt-in forge path"}
	  ],
	  "relations": [
	    {"from_id": "cli/cmd/ao/forge_typed.go", "relation": "wasGeneratedBy", "to_id": "decision-value-gate"}
	  ]
	}`

	client, err := extract.NewClientWithGenerator(extract.BackendBushidoLlama, &forgeFakeGen{response: fakeJSON})
	if err != nil {
		t.Fatalf("build fake client: %v", err)
	}
	withForgeTypedClient(t, client)

	n, err := writeTypedPendingLearnings(session, dir, forgeTypedClient())
	if err != nil {
		t.Fatalf("writeTypedPendingLearnings: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 typed records emitted, got %d", n)
	}

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		t.Fatalf("read pending dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 typed files, got %d", len(entries))
	}
}

// TestForge_TypedSealsEdges asserts the extracted relations are sealed as PROV-O
// edges in the provenance ledger (ac-b1.3): after a typed run the ledger under
// baseDir contains the expected from/relation/to edge with a valid, intact hash
// chain (read back via store.VerifyFile + store.Read).
func TestForge_TypedSealsEdges(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:        "typed-seal-session",
		Date:      time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{"A relation between an artifact and a decision."},
	}

	fakeJSON := `{
	  "entities": [
	    {"node_type": "decision", "id": "decision-seal-edges", "summary": "Seal extracted relations as edges"},
	    {"node_type": "artifact", "id": "cli/cmd/ao/forge_typed.go", "summary": "Typed forge path"}
	  ],
	  "relations": [
	    {"from_id": "cli/cmd/ao/forge_typed.go", "relation": "wasDerivedFrom", "to_id": "decision-seal-edges"}
	  ]
	}`

	client, err := extract.NewClientWithGenerator(extract.BackendBushidoLlama, &forgeFakeGen{response: fakeJSON})
	if err != nil {
		t.Fatalf("build fake client: %v", err)
	}

	if _, err := writeTypedPendingLearnings(session, dir, client); err != nil {
		t.Fatalf("writeTypedPendingLearnings: %v", err)
	}

	ledgerPath := filepath.Join(dir, provenancegraph.LedgerRelativePath)
	store := provenancegraph.NewStore(ledgerPath)

	res, err := store.VerifyFile()
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if !res.Pass {
		t.Fatalf("ledger chain not intact: line %d: %s", res.FirstBrokenLine, res.Message)
	}
	if res.RecordCount < 1 {
		t.Fatalf("expected >=1 sealed edge, got %d", res.RecordCount)
	}

	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var found bool
	for _, e := range edges {
		if e.FromID == "cli/cmd/ao/forge_typed.go" && e.Relation == "wasDerivedFrom" && e.ToID == "decision-seal-edges" {
			found = true
			if e.TrustTier != "mined" {
				t.Errorf("sealed edge trust_tier = %q, want mined", e.TrustTier)
			}
			if e.Hash == "" || e.PayloadHash == "" {
				t.Errorf("sealed edge has empty hash: hash=%q payload_hash=%q", e.Hash, e.PayloadHash)
			}
			// endpoint node types resolved from the entities' node_type
			if e.FromType != "artifact" {
				t.Errorf("from_type = %q, want artifact", e.FromType)
			}
			if e.ToType != "decision" {
				t.Errorf("to_type = %q, want decision", e.ToType)
			}
		}
	}
	if !found {
		t.Fatalf("expected sealed edge cli/cmd/ao/forge_typed.go --wasDerivedFrom--> decision-seal-edges not found in ledger (%d edges)", len(edges))
	}
}

// TestNewCodexClient_SchemaBytesWrittenToTempFile asserts the codex path adapts
// the schema BYTES that Client.Generate passes into a readable temp FILE PATH
// handed to the raw codex turn function (the bytes-vs-path mismatch resolution).
// The injected raw asserts it received a path to a file whose contents are the
// schema JSON, not the JSON bytes used as a path.
func TestNewCodexClient_SchemaBytesWrittenToTempFile(t *testing.T) {
	const schemaJSON = `{"type":"object","properties":{"x":{"type":"string"}}}`

	var gotPath, gotContents string
	raw := func(_ context.Context, _ /*prompt*/ string, schemaPath string) (string, int, error) {
		gotPath = schemaPath
		data, err := os.ReadFile(schemaPath)
		if err != nil {
			t.Errorf("raw received an unreadable path %q (schema bytes passed as a path?): %v", schemaPath, err)
			return "ok", 0, nil
		}
		gotContents = string(data)
		return "ok", 0, nil
	}

	client := extract.NewCodexClient(raw)
	if client.Backend() != extract.BackendCodex {
		t.Fatalf("backend = %q, want codex", client.Backend())
	}

	out, err := client.Generate(context.Background(), "prompt", []byte(schemaJSON))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "ok" {
		t.Fatalf("Generate out = %q, want ok", out)
	}
	if gotPath == "" {
		t.Fatal("raw was never invoked with a schema path")
	}
	// The path must be a real file path, not the schema JSON itself.
	if strings.Contains(gotPath, "\"type\"") || gotPath == schemaJSON {
		t.Fatalf("raw received schema BYTES as the path argument, not a file path: %q", gotPath)
	}
	if gotContents != schemaJSON {
		t.Fatalf("temp file contents = %q, want the schema JSON %q", gotContents, schemaJSON)
	}
	// The temp file must be cleaned up after Generate returns.
	if _, err := os.Stat(gotPath); !os.IsNotExist(err) {
		t.Errorf("temp schema file %q not cleaned up after Generate", gotPath)
	}
}
