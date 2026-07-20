package init

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func newTestModule(dryRun bool) Module {
	return NewModule(clicontract.HostOptions{DryRun: func() bool { return dryRun }})
}

func TestModule_Contract(t *testing.T) {
	contract := newTestModule(false).Contract()
	if contract.ID != "ao.init" {
		t.Fatalf("contract ID = %q, want ao.init", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects&clicontract.EffectFilesystem == 0 {
		t.Fatalf("effects = %v, want filesystem", contract.Effects)
	}
}

func TestModule_CommandAttributes(t *testing.T) {
	command := newTestModule(false).Command()
	if command.Use != "init" {
		t.Fatalf("Use = %q, want init", command.Use)
	}
	if command.GroupID != "start" {
		t.Fatalf("GroupID = %q, want start", command.GroupID)
	}
}

func TestInitCreatesEvidenceStorageWithoutGitMutation(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var output bytes.Buffer
	command := newTestModule(false).Command()
	command.SetOut(&output)
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		filepath.Join(".agents", "ao", "verdicts", "sha256"),
		filepath.Join(".agents", "ao", "provenance"),
		filepath.Join(".agents", "handoff"),
	} {
		if info, err := os.Stat(filepath.Join(dir, relative)); err != nil || !info.IsDir() {
			t.Fatalf("missing %s: %v", relative, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Fatal("init must not edit Git ignore state")
	}
}

func TestInitDryRunCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var output bytes.Buffer
	command := newTestModule(true).Command()
	command.SetOut(&output)
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "would create") {
		t.Fatalf("dry-run output missing 'would create': %s", output.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents")); !os.IsNotExist(err) {
		t.Fatal("dry-run must not create directories")
	}
}
