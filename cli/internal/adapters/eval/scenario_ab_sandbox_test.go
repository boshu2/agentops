package eval

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCorpusRoot_WalksUpFromSubdir: the corpus root is found by walking UP to the
// nearest .agents/.ao ancestor, so isolation is correct no matter which subdir the
// command ran from (refuter: os.Getwd() under cli/ left <repo>/.agents readable).
func TestCorpusRoot_WalksUpFromSubdir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "cli", "cmd", "ao")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := corpusRoot(sub); got != root {
		t.Errorf("corpusRoot(%q) = %q, want %q (must walk up to the .agents root)", sub, got, root)
	}
}

// TestCorpusDenyPaths_RepoAndGlobal: the deny set covers the repo corpus (both
// .agents and .ao under the resolved root) AND the global ~/.agents.
func TestCorpusDenyPaths_RepoAndGlobal(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(corpusDenyPaths(root), "|")
	for _, want := range []string{filepath.Join(root, ".agents"), filepath.Join(root, ".ao")} {
		if !strings.Contains(joined, want) {
			t.Errorf("deny set must include repo %q; got %s", want, joined)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if !strings.Contains(joined, filepath.Join(home, ".agents")) {
			t.Errorf("deny set must include the global ~/.agents; got %s", joined)
		}
	}
}

// TestConfigCorpusDenyPaths_CustomGlobalRoots: a global corpus configured OUTSIDE
// ~/.agents (refuter r2) is denied, including the derived global findings dir that
// `ao lookup` computes as Dir(globalLearnings)/findings.
func TestConfigCorpusDenyPaths_CustomGlobalRoots(t *testing.T) {
	got := configCorpusDenyPaths("/data/corpus/learnings", "/opt/patterns")
	joined := strings.Join(got, "|")
	for _, want := range []string{"/data/corpus/learnings", filepath.Join("/data/corpus", "findings"), "/opt/patterns"} {
		if !strings.Contains(joined, want) {
			t.Errorf("must deny configured global corpus root %q; got %v", want, got)
		}
	}
	if len(configCorpusDenyPaths("", "")) != 0 {
		t.Error("empty config global dirs must add no deny paths")
	}
}

// TestEnvCorpusDenyPaths_NonDefaultOverride: a NON-DEFAULT AO_AGENTS_DIR (or AO_HOME)
// redirects where `ao` reads the corpus OUTSIDE <root>/.agents, so the resolved
// override must enter the deny set; with no override (or the default), nothing is
// added (it equals the already-denied <root>/.agents). age-58r.
func TestEnvCorpusDenyPaths_NonDefaultOverride(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(t.TempDir(), "elsewhere-corpus")

	// AO_AGENTS_DIR override → denied verbatim (the path `ao` actually reads).
	t.Setenv("AO_HOME", "")
	t.Setenv("AO_AGENTS_DIR", custom)
	got := envCorpusDenyPaths(root)
	if len(got) != 1 || got[0] != custom {
		t.Errorf("AO_AGENTS_DIR override must be denied; got %v want [%q]", got, custom)
	}

	// AO_HOME override (no AO_AGENTS_DIR) → also denied.
	t.Setenv("AO_AGENTS_DIR", "")
	t.Setenv("AO_HOME", custom)
	if got := envCorpusDenyPaths(root); len(got) != 1 || got[0] != custom {
		t.Errorf("AO_HOME override must be denied; got %v want [%q]", got, custom)
	}

	// No override → resolves to the default <root>/.agents (already denied) → nothing added.
	t.Setenv("AO_AGENTS_DIR", "")
	t.Setenv("AO_HOME", "")
	if got := envCorpusDenyPaths(root); len(got) != 0 {
		t.Errorf("no env override must add no deny path; got %v", got)
	}
}

// TestEnvCorpusDenyPaths_FlowsIntoCorpusDenyPaths: end-to-end, a non-default
// AO_AGENTS_DIR appears in the full deny set corpusDenyPaths builds (not just the
// helper). The control arm cannot read the override corpus. age-58r.
func TestEnvCorpusDenyPaths_FlowsIntoCorpusDenyPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(t.TempDir(), "elsewhere-corpus")
	t.Setenv("AO_HOME", "")
	t.Setenv("AO_AGENTS_DIR", custom)
	joined := strings.Join(corpusDenyPaths(root), "|")
	if !strings.Contains(joined, custom) {
		t.Errorf("full deny set must include the AO_AGENTS_DIR override %q; got %s", custom, joined)
	}
}

// TestLocalCorpusDenyPaths_AbsoluteOutsideRoot: a config-local LearningsDir/PatternsDir
// pointed at an ABSOLUTE path outside <root>/.agents is denied; a relative dir (the
// default ".agents/learnings", resolving under the already-denied .agents) and an
// absolute dir nested inside <root>/.agents are NOT re-added. age-58r.
func TestLocalCorpusDenyPaths_AbsoluteOutsideRoot(t *testing.T) {
	root := t.TempDir()

	// Absolute, outside <root>/.agents → denied.
	got := localCorpusDenyPaths(root, "/custom/learnings", "/opt/patterns")
	joined := strings.Join(got, "|")
	for _, want := range []string{"/custom/learnings", "/opt/patterns"} {
		if !strings.Contains(joined, want) {
			t.Errorf("must deny absolute local corpus root %q; got %v", want, got)
		}
	}

	// Relative dirs resolve under <root>/.agents (already denied) → not re-added.
	if out := localCorpusDenyPaths(root, ".agents/learnings", ".agents/patterns"); len(out) != 0 {
		t.Errorf("relative local dirs must add no deny path (covered by .agents); got %v", out)
	}

	// Absolute but nested inside <root>/.agents → already covered, not re-added.
	inside := filepath.Join(root, ".agents", "learnings")
	if out := localCorpusDenyPaths(root, inside, ""); len(out) != 0 {
		t.Errorf("absolute dir inside <root>/.agents must not be re-added; got %v", out)
	}

	// Empty → nothing.
	if out := localCorpusDenyPaths(root, "", ""); len(out) != 0 {
		t.Errorf("empty local dirs must add no deny path; got %v", out)
	}
}

// TestSandboxExecDenyProfile_DeniesAllPaths: every deny path appears in a
// (deny file-read* (subpath ...)) clause, and non-corpus reads remain allowed.
func TestSandboxExecDenyProfile_DeniesAllPaths(t *testing.T) {
	p := sandboxExecDenyProfile([]string{"/r/.agents", "/r/.ao", "/home/u/.agents"})
	if !strings.Contains(p, "(allow default)") || !strings.Contains(p, "deny file-read*") {
		t.Errorf("profile must allow-default then deny corpus reads; got %s", p)
	}
	for _, d := range []string{"/r/.agents", "/r/.ao", "/home/u/.agents"} {
		if !strings.Contains(p, `(subpath "`+d+`")`) {
			t.Errorf("profile must deny subpath %q; got %s", d, p)
		}
	}
}

// TestSandboxedCodexArgs_WrapsCodex: codex runs under sandbox-exec with the profile.
func TestSandboxedCodexArgs_WrapsCodex(t *testing.T) {
	got := sandboxedCodexArgs("PROFILE", []string{"exec", "--foo", "the prompt"})
	want := []string{"-p", "PROFILE", "codex", "exec", "--foo", "the prompt"}
	if len(got) != len(want) {
		t.Fatalf("argv = %v (len %d), want len %d", got, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestSandboxedCodexCmd_FailClosed: a sandboxed command on macOS; an ERROR (never a
// bare codex) when isolation is unavailable — empty deny set, or a non-darwin GOOS.
func TestSandboxedCodexCmd_FailClosed(t *testing.T) {
	// empty deny set → fail closed on every platform
	if cmd, err := sandboxedCodexCmd(context.Background(), nil, []string{"exec"}); err == nil || cmd != nil {
		t.Errorf("empty deny set must fail closed; got cmd=%v err=%v", cmd, err)
	}

	cmd, err := sandboxedCodexCmd(context.Background(), []string{"/r/.agents", "/r/.ao"}, []string{"exec", "--output-last-message", "/tmp/x"})
	switch runtime.GOOS {
	case "darwin":
		if err != nil || cmd == nil {
			t.Fatalf("darwin: expected a sandboxed command; got cmd=%v err=%v", cmd, err)
		}
		if !strings.Contains(cmd.Path, "sandbox-exec") {
			t.Errorf("darwin: must wrap sandbox-exec; got path %q", cmd.Path)
		}
		joined := strings.Join(cmd.Args, " ")
		if !strings.Contains(joined, "deny file-read*") || !strings.Contains(joined, "codex") {
			t.Errorf("darwin: args must carry the deny profile + codex; got %q", joined)
		}
	default:
		if err == nil || cmd != nil {
			t.Errorf("non-darwin must fail closed (no unisolated arm); got cmd=%v err=%v", cmd, err)
		}
	}
}

// TestSandboxExec_Integration_DeniesCorpusRead (age-wp1): on darwin, sandbox-exec
// with the scenario-ab deny profile must block reads under .agents while allowing
// reads outside the denied corpus paths. String-only profile tests are insufficient.
func TestSandboxExec_Integration_DeniesCorpusRead(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only: macOS Seatbelt sandbox-exec integration")
	}
	if _, err := os.Stat(macOSSandboxExec); err != nil {
		t.Skipf("sandbox-exec unavailable: %v", err)
	}

	root := t.TempDir()
	agents := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(agents, "sentinel.txt")
	if err := os.WriteFile(secret, []byte("CORPUS-SECRET-MUST-NOT-READ"), 0o644); err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(root, "allowed.txt")
	if err := os.WriteFile(allowed, []byte("allowed-ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := sandboxExecDenyProfile(corpusDenyPaths(root))

	denyCmd := exec.Command(macOSSandboxExec, "-p", profile, "/bin/cat", secret)
	if err := denyCmd.Run(); err == nil {
		t.Fatal("expected sandbox-exec to deny read of corpus file under .agents")
	}

	allowCmd := exec.Command(macOSSandboxExec, "-p", profile, "/bin/cat", allowed)
	out, err := allowCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected read outside deny set to succeed: %v output=%q", err, out)
	}
	if !strings.Contains(string(out), "allowed-ok") {
		t.Errorf("allowed read output = %q, want allowed-ok", out)
	}
}
