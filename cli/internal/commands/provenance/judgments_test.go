package provenance

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/evidence"
)

func TestVerifyJudgmentsCommandMissingLegAndProviderDenial(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	write := func(path string, b []byte) {
		t.Helper()
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "toy"), []byte("2 + 2 = 4\n"))
	manifest, err := evidence.BuildManifest(root, []string{"toy"}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	mb, err := evidence.Canonical(manifest)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(out, "manifest.json"), mb)
	write(filepath.Join(out, "intent"), []byte("Check arithmetic.\n"))
	write(filepath.Join(out, "profiles.json"), []byte(`{"profiles":[{"id":"other","runtime":"claude","model":"claude-test","family":"anthropic","effort":""}]}`))
	args := []string{"verify-judgments", "--root", root, "--manifest", filepath.Join(out, "manifest.json"), "--intent", filepath.Join(out, "intent"), "--evidence-root", out, "--author-context-id", "author", "--required-profiles", filepath.Join(out, "profiles.json"), "--allowed-provider", "anthropic"}
	execute := func(args []string) (string, error) {
		cmd := NewModule(clicontract.HostOptions{}).Command()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return buf.String(), err
	}
	got, err := execute(args)
	if err == nil || !strings.Contains(got, `"missing_required_leg"`) || !strings.Contains(got, `"satisfied": false`) {
		t.Fatalf("missing leg output %s, %v", got, err)
	}
	denied := append([]string(nil), args...)
	denied[len(denied)-1] = "openai"
	got, err = execute(append(denied, "--verdict", filepath.Join(out, "missing-private-verdict")))
	if err == nil || !strings.Contains(err.Error(), "provider_denied") {
		t.Fatalf("provider denial %s, %v", got, err)
	}
	got, err = execute(append(args, "--helper-version", "unsupported"))
	if err == nil || !strings.Contains(err.Error(), "incompatible evidence helper") {
		t.Fatalf("version preflight %s, %v", got, err)
	}
}

func TestVerifyJudgmentsCommandContract(t *testing.T) {
	cmd := NewModule(clicontract.HostOptions{}).verifyJudgmentsCommand()
	contract, ok := clicontract.ContractFor(cmd)
	if !ok || contract.ID != "ao.provenance.verify-judgments" || contract.Effects.String() != "filesystem,environment" {
		t.Fatalf("wrong declared effects: %+v", contract)
	}
}
