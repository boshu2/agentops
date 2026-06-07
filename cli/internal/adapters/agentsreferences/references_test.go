// practices: [wiki-knowledge-surface, design-by-contract]
package agentsreferences

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/boshu2/agentops/cli/internal/adapters/agentsurface"
)

// TestAgentsWriteSurfaces_EachAllowlistEntryHasProductionReference is the
// read-side counterpart to scripts/check-agents-write-surfaces.sh. The shell
// script flags production code that references an undocumented surface; this
// test flags catalogued surfaces that have no production reference.
func TestAgentsWriteSurfaces_EachAllowlistEntryHasProductionReference(t *testing.T) {
	repoRoot, contractPath := findContractRoot(t)
	contractData, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract %s: %v", contractPath, err)
	}

	allowlist := agentsurface.ParseAllowlist(string(contractData))
	if len(allowlist) == 0 {
		t.Fatal("allowlist parsed empty: contract markers missing or malformed")
	}

	refs, err := ScanProduction(repoRoot)
	if err != nil {
		t.Fatalf("scanning production refs: %v", err)
	}

	for _, entry := range allowlist {
		t.Run(entry, func(t *testing.T) {
			if !refs[entry] {
				t.Errorf(
					"allowlist entry %q has no production reference under cli/, scripts/, hooks/, or lib/. "+
						"Either add a write site or remove %q from docs/contracts/agents-write-surfaces.md.",
					entry, entry,
				)
			}
		})
	}
}

func TestScanProduction_FindsKnownLiteral(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "cli", "cmd", "ao"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	goSrc := []byte(`package main

import "path/filepath"

const _ = ".agents/learnings/foo.md"
var _ = filepath.Join(cwd, ".agents", "packets", "promoted")
var _ = filepath.Join(".agents", "wiki", "sources")
`)
	if err := os.WriteFile(filepath.Join(tmp, "cli", "cmd", "ao", "thing.go"), goSrc, 0o644); err != nil {
		t.Fatal(err)
	}

	goTest := []byte(`package main
const _ = ".agents/test-only-surface/x"
`)
	if err := os.WriteFile(filepath.Join(tmp, "cli", "cmd", "ao", "thing_test.go"), goTest, 0o644); err != nil {
		t.Fatal(err)
	}

	sh := []byte("#!/usr/bin/env bash\necho .agents/releases/run.json\n")
	if err := os.WriteFile(filepath.Join(tmp, "scripts", "ship.sh"), sh, 0o644); err != nil {
		t.Fatal(err)
	}

	refs, err := ScanProduction(tmp)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !refs["learnings"] {
		t.Error("expected 'learnings' from production go file")
	}
	if !refs["packets"] {
		t.Error("expected 'packets' from filepath.Join production go file")
	}
	if !refs["wiki"] {
		t.Error("expected 'wiki' from filepath.Join in production go file")
	}
	if !refs["releases"] {
		t.Error("expected 'releases' from shell script")
	}
	if refs["test-only-surface"] {
		t.Error("test files must be excluded; 'test-only-surface' should not appear")
	}
}

// TestAgentsWriteSurfaces_GoShellScannerParity asserts that the Go scanner and
// the shell scanner detect the same set of `.agents/<X>` references when given
// the same fixture tree.
func TestAgentsWriteSurfaces_GoShellScannerParity(t *testing.T) {
	repoRoot := findRepoRootForParity(t)
	scriptPath := filepath.Join(repoRoot, "scripts", "check-agents-write-surfaces.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script %s missing: %v", scriptPath, err)
	}

	tmp := buildParityFixture(t)

	goRefs, err := ScanProduction(tmp)
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

	expected := []string{"learnings", "memory", "packets", "releases", "wiki"}
	if !equalStringSets(want, expected) {
		t.Fatalf("fixture coverage drifted: scanners saw %v, expected %v", want, expected)
	}
}

func findContractRoot(t *testing.T) (string, string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		probe := filepath.Join(dir, "docs", "contracts", "agents-write-surfaces.md")
		if _, err := os.Stat(probe); err == nil {
			return dir, probe
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find docs/contracts/agents-write-surfaces.md walking up from %s", cwd)
	return "", ""
}

func buildParityFixture(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	mustMkdir(t, filepath.Join(tmp, "cli", "cmd", "ao"))
	mustMkdir(t, filepath.Join(tmp, "scripts"))
	mustMkdir(t, filepath.Join(tmp, "hooks"))
	mustMkdir(t, filepath.Join(tmp, "lib"))
	mustMkdir(t, filepath.Join(tmp, "docs", "contracts"))
	mustMkdir(t, filepath.Join(tmp, "skills"))

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

	mustWrite(t, filepath.Join(tmp, "scripts", "ship.sh"),
		[]byte("#!/usr/bin/env bash\necho .agents/releases/run.json\n"))
	mustWrite(t, filepath.Join(tmp, "lib", "store.bash"),
		[]byte("#!/usr/bin/env bash\nstore=.agents/memory/cache\n"))

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
