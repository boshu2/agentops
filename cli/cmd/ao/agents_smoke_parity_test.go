// practices: [wiki-knowledge-surface, design-by-contract]
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// TestAgentsWriteSurfaces_GoShellScannerParity asserts that the Go scanner
// (scanProductionAgentsReferences in agents_smoke_test.go) and the shell
// scanner (scripts/check-agents-write-surfaces.sh) detect the same set of
// `.agents/<X>` references when given the same fixture tree.
//
// Closes finding f-2026-04-27-004: "Duplicated contract scanners drift when
// a new accepted syntax is added to one validator without paired fixture
// coverage in the mirror validator." The fixture covers every recognized
// syntax form (literal in .go, filepath.Join in .go, literal in .sh,
// literal in .bash) plus one form each scanner should reject (test files,
// non-production directories, false-prefix matches).
func TestAgentsWriteSurfaces_GoShellScannerParity(t *testing.T) {
	repoRoot := findRepoRootForParity(t)
	scriptPath := filepath.Join(repoRoot, "scripts", "check-agents-write-surfaces.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script %s missing: %v", scriptPath, err)
	}

	tmp := buildParityFixture(t)

	goRefs, err := scanProductionAgentsReferences(tmp)
	if err != nil {
		t.Fatalf("Go scanner error: %v", err)
	}

	shellRefs := runShellScanner(t, scriptPath, tmp)

	want := keys(goRefs)
	got := keys(shellRefs)
	if !equalStringSets(want, got) {
		t.Fatalf("scanner parity drift:\n  go-only:    %v\n  shell-only: %v\n  Go saw:     %v\n  shell saw:  %v",
			diff(want, got), diff(got, want), want, got)
	}

	// Pin the expected fixture set so a future fixture edit that shrinks
	// either scanner without touching the other is caught here too.
	expected := []string{"learnings", "memory", "packets", "releases", "wiki"}
	if !equalStringSets(want, expected) {
		t.Fatalf("fixture coverage drifted: scanners saw %v, expected %v", want, expected)
	}
}

// buildParityFixture writes a synthetic repo layout containing every
// recognized syntax form plus the contract doc the shell script needs.
func buildParityFixture(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	mustMkdir(t, filepath.Join(tmp, "cli", "cmd", "ao"))
	mustMkdir(t, filepath.Join(tmp, "scripts"))
	mustMkdir(t, filepath.Join(tmp, "hooks"))
	mustMkdir(t, filepath.Join(tmp, "lib"))
	mustMkdir(t, filepath.Join(tmp, "docs", "contracts"))
	mustMkdir(t, filepath.Join(tmp, "skills"))

	// Production .go: literal + filepath.Join. Test file (excluded) and
	// a similar-looking false prefix to confirm both scanners reject it.
	goSrc := []byte(`package main

import "path/filepath"

const _ = ".agents/learnings/foo.md"
var _ = filepath.Join(cwd, ".agents", "packets", "promoted")
var _ = filepath.Join(".agents", "wiki", "sources")
const _ = "not-dot-agents/skipme/x"
`)
	mustWrite(t, filepath.Join(tmp, "cli", "cmd", "ao", "thing.go"), goSrc)

	goTest := []byte(`package main
const _ = ".agents/test-only-surface/x"
`)
	mustWrite(t, filepath.Join(tmp, "cli", "cmd", "ao", "thing_test.go"), goTest)

	// Production shell: .sh and .bash both recognized.
	mustWrite(t, filepath.Join(tmp, "scripts", "ship.sh"),
		[]byte("#!/usr/bin/env bash\necho .agents/releases/run.json\n"))
	mustWrite(t, filepath.Join(tmp, "lib", "store.bash"),
		[]byte("#!/usr/bin/env bash\nstore=.agents/memory/cache\n"))

	// Contract doc: minimal typed table plus an intentionally narrow allowlist.
	// The parity test only cares about the referenced set; allowlist and
	// classification failures are expected and tolerated as exit 1.
	contract := []byte(`# Test fixture

| Surface | Lifecycle | Allowed writers | Mutation lane | Purpose |
|---|---|---|---|---|
| ` + "`ao`" + ` | persistent | cli | runtime-state | Fixture row |

<!-- BEGIN agents-write-surfaces-allowlist -->
ao
<!-- END agents-write-surfaces-allowlist -->
`)
	mustWrite(t, filepath.Join(tmp, "docs", "contracts", "agents-write-surfaces.md"), contract)

	return tmp
}

func runShellScanner(t *testing.T, scriptPath, repoRoot string) map[string]bool {
	t.Helper()
	cmd := exec.Command("bash", scriptPath, "--json")
	cmd.Env = append(os.Environ(), "AGENTS_WRITE_SURFACES_REPO_ROOT="+repoRoot)
	// --json writes payload to stdout even when exit=1 (undocumented
	// surfaces present, expected here because the fixture's allowlist is
	// intentionally empty). Capture stdout directly and tolerate exit=1.
	out, err := cmd.Output()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() == 2 {
			t.Fatalf("running shell scanner: %v\nstderr=%s", err, exitErr.Stderr)
		}
	}
	if len(out) == 0 {
		t.Fatal("shell scanner produced no JSON output")
	}

	var payload struct {
		SourceLocations map[string][]string `json:"source_locations"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parsing shell scanner JSON: %v\nraw=%s", err, string(out))
	}
	got := map[string]bool{}
	for k := range payload.SourceLocations {
		got[k] = true
	}
	return got
}

func findRepoRootForParity(t *testing.T) string {
	t.Helper()
	// Reuse findContractRoot from agents_smoke_test.go — it walks up to
	// docs/contracts/agents-write-surfaces.md, which is exactly the layout
	// we need.
	root, _ := findContractRoot(t)
	return root
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func diff(a, b []string) []string {
	bset := map[string]bool{}
	for _, s := range b {
		bset[s] = true
	}
	var out []string
	for _, s := range a {
		if !bset[s] {
			out = append(out, s)
		}
	}
	return out
}
