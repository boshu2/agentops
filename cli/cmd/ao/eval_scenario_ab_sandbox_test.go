package main

import (
	"context"
	"os"
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
	for _, want := range []string{"/data/corpus/learnings", filepath.Join("/data/corpus", SectionFindings), "/opt/patterns"} {
		if !strings.Contains(joined, want) {
			t.Errorf("must deny configured global corpus root %q; got %v", want, got)
		}
	}
	if len(configCorpusDenyPaths("", "")) != 0 {
		t.Error("empty config global dirs must add no deny paths")
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
