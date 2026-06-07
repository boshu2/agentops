// practices: [wiki-knowledge-surface, design-by-contract]
package agentsinspect

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/adapters/agentsurface"
)

func TestRun_TextOutput(t *testing.T) {
	tmp := t.TempDir()
	contract := filepath.Join(tmp, "contract.md")
	contractContent := `# Title
<!-- BEGIN agents-write-surfaces-allowlist -->
ao
learnings
<!-- END agents-write-surfaces-allowlist -->
`
	if err := os.WriteFile(contract, []byte(contractContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := Run(Options{
		Contract: contract,
		Stdout:   &buf,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Contract: " + contract,
		"Catalogued surfaces: 2",
		".agents/ao/",
		".agents/learnings/",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestRun_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	contract := filepath.Join(tmp, "contract.md")
	contractContent := `<!-- BEGIN agents-write-surfaces-allowlist -->
ao
patterns
<!-- END agents-write-surfaces-allowlist -->
`
	if err := os.WriteFile(contract, []byte(contractContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := Run(Options{
		Contract: contract,
		JSON:     true,
		Stdout:   &buf,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got agentsurface.Inventory
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\nGot: %s", err, buf.String())
	}
	if got.Contract != contract {
		t.Errorf("Contract = %q, want %q", got.Contract, contract)
	}
	wantList := []string{"ao", "patterns"}
	if !reflect.DeepEqual(got.Allowlist, wantList) {
		t.Errorf("Allowlist = %v, want %v", got.Allowlist, wantList)
	}
}

func TestRun_MissingContract(t *testing.T) {
	err := Run(Options{
		Contract: filepath.Join(t.TempDir(), "missing.md"),
	})
	if err == nil {
		t.Fatal("expected error for missing contract")
	}
	if !strings.Contains(err.Error(), "reading contract") {
		t.Errorf("error = %v, want one mentioning 'reading contract'", err)
	}
}
