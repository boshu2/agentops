package init

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/initapp"
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
	// The ignore block is the ONLY Git-adjacent effect: no repository is
	// initialized, no hook installed, no index touched.
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("init initialized a Git repository (stat err=%v)", err)
	}
}

// TestInitAppendsGitignoreBlockExactlyOnce is the L2 acceptance for the
// no-ignore-guidance defect, driven through the command entry point: running
// `ao init` twice in a fresh directory leaves the managed block present exactly
// once. Before this, init scaffolded .agents/ao/** and left the user to invent
// the same ignore rules by hand after their first loop.
func TestInitAppendsGitignoreBlockExactlyOnce(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	runInit := func() string {
		t.Helper()
		var output bytes.Buffer
		command := newTestModule(false).Command()
		command.SetOut(&output)
		command.SetArgs(nil)
		if err := command.Execute(); err != nil {
			t.Fatalf("ao init: %v", err)
		}
		return output.String()
	}

	runInit()
	runInit()

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	body := string(data)
	if got := strings.Count(body, initapp.GitignoreBeginMarker); got != 1 {
		t.Fatalf("AgentOps block appears %d times after two inits, want 1:\n%s", got, body)
	}
	if !strings.Contains(body, ".agents/ao/sessions/") {
		t.Fatalf(".gitignore is missing the scratch patterns:\n%s", body)
	}
}

// TestInitHelpDocumentsTheIgnorePolicy: the policy is only usable if the
// command says what it ignores and what it deliberately leaves trackable.
func TestInitHelpDocumentsTheIgnorePolicy(t *testing.T) {
	long := newTestModule(false).Command().Long
	for _, want := range []string{".gitignore", ".agents/ao/sessions/", ".agents/ao/intents/", ".agents/ao/verdicts/"} {
		if !strings.Contains(long, want) {
			t.Errorf("init help does not document %q:\n%s", want, long)
		}
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
