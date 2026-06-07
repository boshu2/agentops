package checks

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
)

// deferredBacking are bash-gate backing scripts intentionally NOT yet in the Go
// registry, with the reason. The coverage test allows exactly these; anything
// else missing is a parity gap that must be closed before flipping the default
// to the Go gate (ag-3n71 PB2). This is the no-strangler net.
var deferredBacking = map[string]string{
	"check-agents-hash-snapshot.sh": "stateful capture/diff pair — needs a native Go port",
}

func repoRootFromTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "pre-push-gate.sh")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("repo root with scripts/pre-push-gate.sh not found (out-of-tree build)")
		}
		dir = parent
	}
}

// TestRegistryCoversBashGateBackingScripts is the flip-gate (PB2 precondition):
// every check-*.sh / validate-*.sh the bash gate invokes AND that exists on disk
// must be in the Go registry's Backing set, except the documented deferrals.
func TestRegistryCoversBashGateBackingScripts(t *testing.T) {
	root := repoRootFromTest(t)
	gate, err := os.ReadFile(filepath.Join(root, "scripts", "pre-push-gate.sh"))
	if err != nil {
		t.Fatalf("read pre-push-gate.sh: %v", err)
	}
	re := regexp.MustCompile(`scripts/((?:check|validate)-[a-z0-9-]+\.sh)`)
	bashScripts := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(gate), -1) {
		name := m[1]
		// only count scripts that actually exist on disk
		if _, err := os.Stat(filepath.Join(root, "scripts", name)); err == nil {
			bashScripts[name] = true
		}
	}

	registered := map[string]bool{}
	for _, c := range gates.Default.All() {
		if c.Backing != "" {
			registered[c.Backing] = true
		}
	}

	var missing []string
	for s := range bashScripts {
		if registered[s] {
			continue
		}
		if _, ok := deferredBacking[s]; ok {
			continue
		}
		missing = append(missing, s)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("Go registry is missing %d bash-gate backing scripts (parity gap — close before PB2 flip, or add to deferredBacking with a reason):\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}

	// Guard the allowlist: a deferral that no longer exists on disk is stale.
	for s := range deferredBacking {
		if _, err := os.Stat(filepath.Join(root, "scripts", s)); err != nil {
			t.Errorf("deferredBacking lists %q but it no longer exists — remove the stale deferral", s)
		}
	}
}
