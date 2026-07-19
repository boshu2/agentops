// practices: [design-by-contract, in-toto-provenance]
package provenance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// dirExists reports whether p is an existing directory (local helper so this
// test file does not depend on the carved skills isDir).
func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// repoRoot walks up from the test's cwd to the repo root (dir containing both
// tests/ and cli/). Used to resolve the committed orphan fixtures.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if dirExists(filepath.Join(dir, "tests", "fixtures", "provenance")) && dirExists(filepath.Join(dir, "cli")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repo root from %s", dir)
	return ""
}

func fixturePath(t *testing.T, name string) string {
	return filepath.Join(repoRoot(t), "tests", "fixtures", "provenance", name)
}

// expectedOrphanFixtures mirrors tests/fixtures/provenance/expected-orphans.json.
type expectedOrphanFixtures struct {
	Fixtures []struct {
		File             string                        `json:"file"`
		OrphanArtifactID string                        `json:"orphan_artifact_id"`
		ExpectedFinding  provenancegraph.OrphanFinding `json:"expected_finding"`
	} `json:"fixtures"`
}

func loadExpected(t *testing.T) expectedOrphanFixtures {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, "expected-orphans.json"))
	if err != nil {
		t.Fatalf("read expected-orphans.json: %v", err)
	}
	var exp expectedOrphanFixtures
	if err := json.Unmarshal(b, &exp); err != nil {
		t.Fatalf("parse expected-orphans.json: %v", err)
	}
	if len(exp.Fixtures) != 3 {
		t.Fatalf("expected 3 seeded fixtures, got %d", len(exp.Fixtures))
	}
	return exp
}

// TestProvenanceTrace_StrictCatchesEachSeededOrphan is the L2 gate test: for
// every seeded orphan fixture, `--orphans --strict` must exit non-zero and emit
// exactly the finding declared in expected-orphans.json.
func TestProvenanceTrace_StrictCatchesEachSeededOrphan(t *testing.T) {
	exp := loadExpected(t)
	for _, fx := range exp.Fixtures {
		t.Run(fx.File, func(t *testing.T) {
			out, err := execProv(t, testLedger(t), "trace", "--orphans", "--strict", "--json",
				"--graph", fixturePath(t, fx.File))
			if err == nil {
				t.Fatalf("--strict on %s expected non-zero exit, got nil", fx.File)
			}

			var got []provenancegraph.OrphanFinding
			for _, ln := range strings.Split(strings.TrimSpace(out), "\n") {
				if strings.TrimSpace(ln) == "" {
					continue
				}
				var f provenancegraph.OrphanFinding
				if jerr := json.Unmarshal([]byte(ln), &f); jerr != nil {
					t.Fatalf("finding line not JSON: %v\n%s", jerr, ln)
				}
				got = append(got, f)
			}
			if len(got) != 1 {
				t.Fatalf("%s: want exactly 1 orphan finding, got %d: %+v", fx.File, len(got), got)
			}
			if got[0].OrphanArtifactID != fx.OrphanArtifactID {
				t.Fatalf("%s orphan id = %q, want %q", fx.File, got[0].OrphanArtifactID, fx.OrphanArtifactID)
			}
			if got[0].Severity != fx.ExpectedFinding.Severity ||
				got[0].Code != fx.ExpectedFinding.Code ||
				got[0].Path != fx.ExpectedFinding.Path ||
				got[0].Message != fx.ExpectedFinding.Message {
				t.Fatalf("%s finding mismatch:\n got  %+v\n want %+v", fx.File, got[0], fx.ExpectedFinding)
			}
		})
	}
}

// TestProvenanceTrace_PassesOnceEdgeAdded proves the gate flips green: take a
// seeded fixture, append an inbound edge wiring its orphan artifact back to a
// directive, and the audit reports zero orphans and exits 0 under --strict.
func TestProvenanceTrace_PassesOnceEdgeAdded(t *testing.T) {
	exp := loadExpected(t)
	fx := exp.Fixtures[0] // orphan-scenario-hash-stability.jsonl

	src, err := os.ReadFile(fixturePath(t, fx.File))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	// Wire the orphan artifact with an inbound edge.
	wired := string(src) +
		`{"record":"edge","edge_type":"bead_produced_artifact","from_id":"d-gherkin-acceptance","to_id":"` +
		fx.OrphanArtifactID + `","confidence":"high","evidence":"GOALS.md"}` + "\n"

	dir := t.TempDir()
	path := filepath.Join(dir, "wired.jsonl")
	if err := os.WriteFile(path, []byte(wired), 0o644); err != nil {
		t.Fatalf("write wired graph: %v", err)
	}

	out, err := execProv(t, testLedger(t), "trace", "--orphans", "--strict", "--graph", path)
	if err != nil {
		t.Fatalf("wired graph should pass --strict, got error: %v", err)
	}
	if !strings.Contains(out, "No provenance orphans found.") {
		t.Fatalf("wired graph output = %q, want no-orphans message", out)
	}
}

// TestProvenanceTrace_NonStrictReportsButExitsZero verifies advisory mode: an
// orphan is reported but the command exits 0 without --strict.
func TestProvenanceTrace_NonStrictReportsButExitsZero(t *testing.T) {
	exp := loadExpected(t)
	fx := exp.Fixtures[0]

	out, err := execProv(t, testLedger(t), "trace", "--orphans", "--graph", fixturePath(t, fx.File))
	if err != nil {
		t.Fatalf("non-strict should exit 0, got: %v", err)
	}
	if !strings.Contains(out, "1 orphan(s)") {
		t.Fatalf("non-strict output = %q, want orphan count", out)
	}
	if !strings.Contains(out, "pass --strict") {
		t.Fatalf("non-strict output = %q, want --strict hint", out)
	}
}

// TestProvenanceTrace_RequiresOrphansMode rejects an invocation without
// --orphans (the only supported mode).
func TestProvenanceTrace_RequiresOrphansMode(t *testing.T) {
	if _, err := execProv(t, testLedger(t), "trace", "--graph", "x"); err == nil {
		t.Fatal("want error when --orphans not set")
	}
}

// TestProvenanceTrace_RequiresGraph rejects --orphans without --graph.
func TestProvenanceTrace_RequiresGraph(t *testing.T) {
	if _, err := execProv(t, testLedger(t), "trace", "--orphans"); err == nil {
		t.Fatal("want error when --graph not provided")
	}
}
