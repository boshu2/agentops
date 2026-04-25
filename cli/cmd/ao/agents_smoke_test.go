package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestAgentsWriteSurfaces_EachAllowlistEntryHasProductionReference is the
// read-side counterpart to scripts/check-agents-write-surfaces.sh. The
// shell script flags production code that references an undocumented
// surface; this test flags catalogued surfaces that have no production
// reference (stale allowlist entries).
//
// For each allowlist entry, this test scans cli/ (excluding *_test.go),
// scripts/, hooks/, and lib/ for at least one `.agents/<entry>` literal.
// Drives off the contract doc directly so the inventory and the gate
// stay synchronized as surfaces are added or retired.
func TestAgentsWriteSurfaces_EachAllowlistEntryHasProductionReference(t *testing.T) {
	repoRoot, contractPath := findContractRoot(t)
	contractData, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract %s: %v", contractPath, err)
	}

	allowlist := parseAgentsAllowlist(string(contractData))
	if len(allowlist) == 0 {
		t.Fatal("allowlist parsed empty — contract markers missing or malformed")
	}

	refs, err := scanProductionAgentsReferences(repoRoot)
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

// findContractRoot walks up from the test working directory looking for
// docs/contracts/agents-write-surfaces.md. Cannot reuse the shared
// findRepoRoot helper because this package contains a nested .agents/
// fixture dir that confuses the .agents-based probe.
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

// scanProductionAgentsReferences returns the set of subdir names referenced
// via .agents/<name> literals in production code (cli/**/*.go excluding
// _test.go, plus scripts/, hooks/, lib/ shell files). Mirrors the regex
// used by scripts/check-agents-write-surfaces.sh so the read-side and
// write-side gates agree on what counts as a reference.
func scanProductionAgentsReferences(repoRoot string) (map[string]bool, error) {
	literalRe := regexp.MustCompile(`\.agents/([a-z][a-zA-Z0-9_-]*)`)
	found := map[string]bool{}

	walkOne := func(rootDir string, isProductionFile func(path string, d fs.DirEntry) bool) error {
		root := filepath.Join(repoRoot, rootDir)
		if _, err := os.Stat(root); err != nil {
			return nil // directory absent is not a failure
		}
		return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !isProductionFile(path, d) {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			for _, m := range literalRe.FindAllSubmatch(data, -1) {
				found[string(m[1])] = true
			}
			return nil
		})
	}

	// Go production files: cli/**/*.go excluding *_test.go.
	if err := walkOne("cli", func(path string, d fs.DirEntry) bool {
		name := d.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}); err != nil {
		return nil, err
	}

	// Shell production files: scripts/, hooks/, lib/ — *.sh and *.bash.
	for _, dir := range []string{"scripts", "hooks", "lib"} {
		if err := walkOne(dir, func(path string, d fs.DirEntry) bool {
			name := d.Name()
			return strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".bash")
		}); err != nil {
			return nil, err
		}
	}

	return found, nil
}

// TestScanProductionAgentsReferences_FindsKnownLiteral pins the scanner
// behavior so the smoke test above can rely on it. Uses a fixture under
// t.TempDir() shaped like the production layout.
func TestScanProductionAgentsReferences_FindsKnownLiteral(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "cli", "cmd", "ao"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	goSrc := []byte(`package main
const _ = ".agents/learnings/foo.md"
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

	refs, err := scanProductionAgentsReferences(tmp)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !refs["learnings"] {
		t.Error("expected 'learnings' from production go file")
	}
	if !refs["releases"] {
		t.Error("expected 'releases' from shell script")
	}
	if refs["test-only-surface"] {
		t.Error("test files must be excluded — 'test-only-surface' should not appear")
	}
}
