//go:build integration
// +build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/orchestration"
)

func TestOrchestrateToolsExecuteJSON(t *testing.T) {
	orchestrateIntegrationChdir(t)
	out, err := executeCommand("orchestrate", "tools", "--json")
	if err != nil {
		t.Fatalf("orchestrate tools --json: %v", err)
	}
	if !strings.Contains(out, `"command": "tools"`) {
		t.Fatalf("orchestrate tools output missing command marker: %s", out)
	}
}

func TestOrchestratePreflightExecuteJSON(t *testing.T) {
	root := orchestrateIntegrationChdir(t)
	out, err := executeCommand("orchestrate", "preflight", "--profile", "dual-pane", "--json")
	if err != nil {
		t.Fatalf("orchestrate preflight --json: %v out=%s", err, out)
	}
	if !strings.Contains(out, `"command": "preflight"`) {
		t.Fatalf("orchestrate preflight output missing command marker: %s", out)
	}

	ledgerPath := filepath.Join(root, "docs", "provenance", "ledger.jsonl")
	ledger, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read temp preflight ledger: %v", err)
	}
	if !strings.Contains(string(ledger), orchestration.LedgerEventPreflight) {
		t.Fatalf("temp preflight ledger missing %s: %s", orchestration.LedgerEventPreflight, ledger)
	}
}

func orchestrateIntegrationChdir(t *testing.T) string {
	t.Helper()

	sourceRoot := orchestrateTestRepoRoot(t)
	root := t.TempDir()
	for _, rel := range []string{orchestration.ToolsContractRelPath, orchestration.ProfilesContractRelPath} {
		src := filepath.Join(sourceRoot, rel)
		dst := filepath.Join(root, rel)
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir stub bin: %v", err)
	}
	writeOrchestrateStub(t, binDir, "atm", `#!/usr/bin/env sh
if [ "${1:-}" = "list" ]; then exit 0; fi
echo "atm version 1.2.0"
`)
	writeOrchestrateStub(t, binDir, "am", `#!/usr/bin/env sh
if [ "${1:-}" = "robot" ] && [ "${2:-}" = "health" ]; then echo '{"ok":true}'; exit 0; fi
echo "am version 1.2.0"
`)
	writeOrchestrateStub(t, binDir, "ntm", `#!/usr/bin/env sh
if [ "${1:-}" = "--robot-capabilities" ]; then echo '{"capabilities":["tmux","git","persistent-host","agent-CLIs"]}'; exit 0; fi
echo "ntm version 1.2.0"
`)
	writeOrchestrateStub(t, binDir, "claude", "#!/usr/bin/env sh\necho 'claude version 1.2.0'\n")
	writeOrchestrateStub(t, binDir, "codex", "#!/usr/bin/env sh\necho 'codex version 1.2.0'\n")
	writeOrchestrateStub(t, binDir, "agy", "#!/usr/bin/env sh\necho 'agy version 1.2.0'\n")
	writeOrchestrateStub(t, binDir, "tmux", `#!/usr/bin/env sh
if [ "${1:-}" = "list-sessions" ]; then exit 0; fi
echo "tmux version 1.2.0"
`)

	tmpDir := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMPDIR", tmpDir)
	t.Chdir(root)
	return root
}

func writeOrchestrateStub(t *testing.T, binDir, name, body string) {
	t.Helper()
	path := filepath.Join(binDir, name)
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatalf("write stub %s: %v", name, err)
	}
}
