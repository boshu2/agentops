package provenancegraph

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestStore_AppendThenReadRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docs", "provenance", "ledger.jsonl")
	store := NewStore(path)

	// Genesis append on an absent file.
	res, err := store.Append(validEdge())
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if res.Skipped {
		t.Fatal("first append should not be skipped")
	}
	if res.Edge.PrevHash != "" {
		t.Fatalf("genesis prev_hash = %q, want empty", res.Edge.PrevHash)
	}

	// A second, distinct edge chains onto the first.
	second := validEdge()
	second.ToID = "docs/provenance/ledger.jsonl"
	res2, err := store.Append(second)
	if err != nil {
		t.Fatalf("append 2: %v", err)
	}
	if res2.Edge.PrevHash != res.Edge.Hash {
		t.Fatalf("second prev_hash = %q, want %q", res2.Edge.PrevHash, res.Edge.Hash)
	}

	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("read %d edges, want 2", len(edges))
	}
	if idx, err := VerifyChain(edges); err != nil || idx != 0 {
		t.Fatalf("persisted chain: idx=%d err=%v, want 0/nil", idx, err)
	}
}

// TestStore_ConcurrentAppendDoesNotForkChain is the E3.CC contract
// (age-membrane-memory-arch-tz2s.4.5): the read-seal-append critical section
// must be serialized so concurrent appenders cannot both seal onto the same
// chain tip and FORK the hash chain. The provenance ledger is the membrane's
// verdict audit authority ("no verdict = not done"), and ml8's standing
// pawl-service is a new concurrent writer to it (concurrent routes -> concurrent
// emits). Each goroutine opens its own lock fd, so this exercises the same
// advisory-lock path a separate `ao` process would take.
func TestStore_ConcurrentAppendDoesNotForkChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docs", "provenance", "ledger.jsonl")
	store := NewStore(path)

	const n = 32
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := validEdge()
			// Distinct identity per goroutine so none is idempotently skipped.
			e.ToID = fmt.Sprintf("cli/cmd/ao/gen_%03d.go", i)
			if _, err := store.Append(e); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent append errored: %v", err)
	}

	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read after concurrent appends: %v", err)
	}
	// 1) No append lost or collided away.
	if len(edges) != n {
		t.Fatalf("after %d concurrent appends, ledger has %d edges (lost/collided)", n, len(edges))
	}
	// 2) The chain is intact in file order: each prev_hash links to the prior
	// hash. A fork (two edges sealed onto the same tip) breaks VerifyChain.
	if idx, err := VerifyChain(edges); err != nil || idx != 0 {
		t.Fatalf("chain forked by concurrent append: first break at record %d: %v", idx, err)
	}
	// 3) Every distinct identity is present exactly once.
	seen := make(map[string]int, n)
	for _, e := range edges {
		seen[e.ToID]++
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("cli/cmd/ao/gen_%03d.go", i)
		if seen[id] != 1 {
			t.Fatalf("identity %s present %d times, want exactly 1", id, seen[id])
		}
	}
}

func TestStore_AppendIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	store := NewStore(path)

	if _, err := store.Append(validEdge()); err != nil {
		t.Fatalf("append: %v", err)
	}
	// Same identity with a different timestamp -> skipped no-op.
	dup := validEdge()
	dup.TS = "2026-06-01T12:00:00Z"
	res, err := store.Append(dup)
	if err != nil {
		t.Fatalf("dup append: %v", err)
	}
	if !res.Skipped {
		t.Fatal("duplicate append should be skipped")
	}

	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("ledger has %d edges after dup, want 1", len(edges))
	}
}

func TestStore_AppendRejectsInvalidEdgeBeforeWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	store := NewStore(path)

	bad := validEdge()
	bad.Relation = "not_a_relation"
	if _, err := store.Append(bad); err == nil {
		t.Fatal("expected append to reject invalid relation")
	}
	// No file should have been created by a rejected write.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("ledger file created despite invalid edge: stat err = %v", err)
	}
}

func TestStore_ReadRejectsCorruptLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(path, []byte("{not json}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := NewStore(path).Read(); err == nil {
		t.Fatal("expected read to fail on corrupt JSON line")
	}
}

// TestStore_AppendEmitsSchemaValidEdges validates each appended edge against
// the merged schemas/agentops-sdlc-provenance.v1.schema.json via the committed
// validator (scripts/validate-provenance-ledger.sh). Skips cleanly if the
// validator or its python+jsonschema dependency is unavailable.
func TestStore_AppendEmitsSchemaValidEdges(t *testing.T) {
	repo := repoRootForTest(t)
	validator := filepath.Join(repo, "scripts", "validate-provenance-ledger.sh")
	if _, err := os.Stat(validator); err != nil {
		t.Skipf("validator not found: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	store := NewStore(path)

	if _, err := store.Append(validEdge()); err != nil {
		t.Fatalf("append: %v", err)
	}
	withEvidence := validEdge()
	withEvidence.ToID = "docs/provenance/ledger.jsonl"
	withEvidence.EvidenceRef = ".agents/council/2026-05-30-debate-provenance-substrate.md"
	withEvidence.TrustTier = "inferred"
	if _, err := store.Append(withEvidence); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Validate each emitted edge as a single event file.
	for i, e := range edges {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal edge %d: %v", i, err)
		}
		ef := filepath.Join(dir, "edge.json")
		if err := os.WriteFile(ef, b, 0o644); err != nil {
			t.Fatalf("write edge file: %v", err)
		}
		cmd := exec.Command("bash", validator, ef)
		out, runErr := cmd.CombinedOutput()
		so := string(out)
		// SKIP (python/jsonschema absent) exits 0 — treat as a pass.
		if runErr != nil {
			t.Fatalf("validator rejected emitted edge %d: %v\n%s", i, runErr, so)
		}
	}
}

// repoRootForTest walks up from the test's cwd to the repo root (dir holding
// both schemas/ and scripts/).
func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if isDirT(filepath.Join(dir, "schemas")) && isDirT(filepath.Join(dir, "scripts")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("repo root with schemas/+scripts/ not found")
	return ""
}

func isDirT(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
