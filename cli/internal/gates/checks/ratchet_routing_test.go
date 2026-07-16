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
