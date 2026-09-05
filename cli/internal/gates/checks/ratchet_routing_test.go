package checks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
)

// TestRatchetLibConsumersRouteLibEdits is the ratchet-lib routing closure
// (age-ratchet-lib-extraction-bv7d.1, premortem FM3): every scripts/check-*.sh
// that sources scripts/lib/ratchet.sh must (a) be a registered Backing in the
// Go gate registry and (b) carry "scripts/lib/ratchet.sh" in its Match globs —
// otherwise an edit to the shared lib would not re-run the gates that depend
// on it, and a bad lib change could land without any dependent gate executing.
// The test is self-extending: each migration slice that adopts the lib is
// covered the moment its script starts sourcing it.
func TestRatchetLibConsumersRouteLibEdits(t *testing.T) {
	root := repoRootFromTest(t)
	const libPath = "scripts/lib/ratchet.sh"

	entries, err := os.ReadDir(filepath.Join(root, "scripts"))
	if err != nil {
		t.Fatalf("read scripts dir: %v", err)
	}

	byBacking := map[string]gates.Check{}
	for _, c := range gates.Default.All() {
		if c.Backing != "" {
			byBacking[c.Backing] = c
		}
	}

	consumers := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "check-") || !strings.HasSuffix(name, ".sh") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, "scripts", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(body), "lib/ratchet.sh") {
			continue
		}
		consumers++
		c, ok := byBacking[name]
		if !ok {
			t.Errorf("%s sources %s but is not a registered Backing in the gate registry — lib edits cannot route to it", name, libPath)
			continue
		}
		if !slices.Contains(c.Match, libPath) {
			t.Errorf("check %q (Backing %s) sources the ratchet lib but its Match globs omit %q — add it so lib edits re-run this gate", c.ID, name, libPath)
		}
	}
	t.Logf("ratchet-lib consumers verified: %d", consumers)
}

// TestDocClaimsTrackedRoutesItsOwnSubject is the docs.claims-tracked routing
// witness. A Match glob the router's own matcher cannot evaluate is worse than
// no glob at all: the gate reads as routed while never being selected. The
// registry shipped `evals/**/*.md`, which matchGlob resolves as "prefix evals/
// AND literal suffix */*.md" — a suffix no path can have — so the gate was
// dead on every doc edit. Assert the real paths the gate reasons about select
// it, including a DELETED claimed target under scripts/ or tests/, which is
// how a doc sentence starts outrunning the tree in the first place.
func TestDocClaimsTrackedRoutesItsOwnSubject(t *testing.T) {
	var check gates.Check
	for _, c := range gates.Default.All() {
		if c.ID == "docs.claims-tracked" {
			check = c
		}
	}
	if check.ID == "" {
		t.Fatal("docs.claims-tracked is not registered")
	}
	if len(check.Match) == 0 {
		t.Fatal("docs.claims-tracked has no Match globs; it would always run")
	}
	selecting := []string{
		"evals/skill-probes/README.md",
		"evals/notes.md",
		"docs/evals/scorecards/2026-09-05/notes.md",
		".gitignore",
		"scripts/check-doc-claims-tracked.sh",
		"tests/scripts/check-doc-claims-tracked.bats",
		// Deleting a claimed target is exactly the change that turns a clean
		// doc sentence into a false one.
		"scripts/probe-skill.sh",
		"tests/fixtures/verdict-contract/cases/valid-pass.json",
	}
	for _, path := range selecting {
		if !gates.PathMatchesAny(check.Match, path) {
			t.Errorf("docs.claims-tracked does not route %q; its Match globs are %v", path, check.Match)
		}
	}
	for _, path := range []string{"cli/internal/gates/routing.go", "README.md", "skills/rpi/SKILL.md"} {
		if gates.PathMatchesAny(check.Match, path) {
			t.Errorf("docs.claims-tracked routes unrelated path %q", path)
		}
	}
}
