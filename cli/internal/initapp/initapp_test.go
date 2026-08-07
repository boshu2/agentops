package initapp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// A fresh `ao init` creates exactly the requested-proof stores with declared
// consumers (intents, verdicts) and nothing else: no session, index,
// provenance, or handoff store may be scaffolded (ADR-0016 closed set;
// operations-layer alignment).
func TestRun_CreatesOnlyDeclaredEvidenceStores(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer

	if err := Run(RunOptions{Stdout: &out}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mustBeDir := func(rel string) {
		t.Helper()
		info, err := os.Stat(rel)
		if err != nil || !info.IsDir() {
			t.Errorf("init did not create directory %s (err=%v)", rel, err)
		}
	}

	// Status's loop-evidence stores: intents and verdicts content stores.
	mustBeDir(filepath.Join(storage.DefaultBaseDir, "intents", "sha256"))
	mustBeDir(filepath.Join(storage.DefaultBaseDir, "verdicts", "sha256"))

	// The retired undeclared stores must NOT come back: a directory minted
	// "just in case" becomes an undeclared knowledge store.
	for _, retired := range []string{
		filepath.Join(storage.DefaultBaseDir, "sessions"),
		filepath.Join(storage.DefaultBaseDir, "index"),
		filepath.Join(storage.DefaultBaseDir, "provenance"),
		filepath.Join(".agents", "handoff"),
	} {
		if _, err := os.Stat(retired); !os.IsNotExist(err) {
			t.Errorf("init scaffolded retired store %s (stat err=%v)", retired, err)
		}
	}

	if got := strings.Count(out.String(), "created "); got != len(evidenceDirs) {
		t.Errorf("expected %d 'created' lines, got %d:\n%s", len(evidenceDirs), got, out.String())
	}
}

// TestRun_GitignoreBlockIsWrittenOnceAndIsIdempotent is the acceptance for the
// scaffold-without-ignore-guidance defect: `ao init` created .agents/ao/** and
// said nothing about git, so one loop later the tree was full of untracked
// scratch. Init now writes the block, and running init again must not duplicate
// it — the file is appended to, and a repeated init is a normal thing to do.
func TestRun_GitignoreBlockIsWrittenOnceAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var first bytes.Buffer

	if err := Run(RunOptions{Stdout: &first}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	body := string(data)
	if got := strings.Count(body, GitignoreBeginMarker); got != 1 {
		t.Fatalf("begin marker appears %d times after one init, want 1:\n%s", got, body)
	}
	if got := strings.Count(body, GitignoreEndMarker); got != 1 {
		t.Fatalf("end marker appears %d times after one init, want 1:\n%s", got, body)
	}
	for _, want := range []string{".agents/scratch/", ".agents/projections/", "__pycache__/"} {
		if !strings.Contains(body, want) {
			t.Errorf(".gitignore missing scratch pattern %q:\n%s", want, body)
		}
	}
	// Loop evidence stays trackable: that call belongs to the consumer repo.
	for _, tracked := range []string{".agents/ao/intents/", ".agents/ao/verdicts/"} {
		for _, line := range strings.Split(body, "\n") {
			if strings.TrimSpace(line) == tracked {
				t.Errorf(".gitignore ignores loop evidence %q, which is repo policy, not the CLI's:\n%s", tracked, body)
			}
		}
	}

	var second bytes.Buffer
	if err := Run(RunOptions{Stdout: &second}); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore after second init: %v", err)
	}
	if string(after) != body {
		t.Fatalf("second init changed .gitignore:\nbefore:\n%s\nafter:\n%s", body, after)
	}
	if got := strings.Count(string(after), GitignoreBeginMarker); got != 1 {
		t.Fatalf("begin marker appears %d times after two inits, want exactly 1", got)
	}
	if !strings.Contains(second.String(), "already has the AgentOps block") {
		t.Errorf("second init did not report the block as already present: %s", second.String())
	}
}

// TestRun_GitignoreBlockPreservesExistingContent: init appends, never rewrites.
// An existing .gitignore keeps every line it had, and a file that did not end
// in a newline does not get its last rule glued to the block's first comment.
func TestRun_GitignoreBlockPreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules\n*.log"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Run(RunOptions{Stdout: &out}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.HasPrefix(body, "node_modules\n*.log\n") {
		t.Fatalf("existing rules were not preserved verbatim:\n%s", body)
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "*.log") && strings.TrimSpace(line) != "*.log" {
			t.Fatalf("last existing rule was glued to the appended block: %q", line)
		}
	}
	if !strings.Contains(body, GitignoreBeginMarker) {
		t.Fatalf("block not appended:\n%s", body)
	}
}

// TestRun_GitignoreBlockRespectsUserEdits: the marker is the idempotency key,
// not the block body. A user who edits or trims the lines inside has made a
// local decision; a second init must respect it rather than "repair" it.
func TestRun_GitignoreBlockRespectsUserEdits(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	edited := GitignoreBeginMarker + "\n.agents/scratch/\n" + GitignoreEndMarker + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Run(RunOptions{Stdout: &out}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != edited {
		t.Fatalf("init rewrote a user-edited block:\nwant:\n%s\ngot:\n%s", edited, data)
	}
}

func TestRun_DryRunCreatesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer

	if err := Run(RunOptions{DryRun: true, Stdout: &out}); err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}

	if got := strings.Count(out.String(), "would create "); got != len(evidenceDirs) {
		t.Errorf("expected %d 'would create' lines, got %d:\n%s", len(evidenceDirs), got, out.String())
	}
	if _, err := os.Stat(".agents"); !os.IsNotExist(err) {
		t.Errorf("dry-run created .agents (stat err=%v); want nothing on disk", err)
	}
	if _, err := os.Stat(".gitignore"); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote .gitignore (stat err=%v); want nothing on disk", err)
	}
	if !strings.Contains(out.String(), "would append the AgentOps block to .gitignore") {
		t.Errorf("dry-run did not announce the .gitignore block: %s", out.String())
	}
}
