// practices: [design-by-contract, code-complete]
package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// setInitRemove sets the package-global --remove flag var and restores it on
// cleanup (go.md: shared cobra-global flag vars must be restored via t.Cleanup).
func setInitRemove(t *testing.T, v bool) {
	t.Helper()
	prev := verifyInitRemove
	verifyInitRemove = v
	t.Cleanup(func() { verifyInitRemove = prev })
}

// runInitT invokes `ao verify init` (with the given --remove) at cwd=repo and
// returns the command output.
func runInitT(t *testing.T, repo string, remove bool) string {
	t.Helper()
	t.Chdir(repo)
	setInitRemove(t, remove)
	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&buf)
	c.SetErr(&buf)
	if err := runVerifyInit(c, nil); err != nil {
		t.Fatalf("ao verify init (remove=%v): %v\n%s", remove, err, buf.String())
	}
	return buf.String()
}

// setInitSelfBinary overrides the install-time self-binary seam and restores it
// on cleanup (go.md: shared package-global state must be restored via t.Cleanup).
func setInitSelfBinary(t *testing.T, fn func() (string, error)) {
	t.Helper()
	prev := verifyInitSelfBinary
	verifyInitSelfBinary = fn
	t.Cleanup(func() { verifyInitSelfBinary = prev })
}

// TestVerifyInit_RefusesRepoInternalAoBake locks the install-side half of the
// no-repo-internal-absolute invariant: running the install from an ao INSIDE the
// repo must REFUSE (never bake a repo-local path a later swap could subvert), and
// must not write a hook.
func TestVerifyInit_RefusesRepoInternalAoBake(t *testing.T) {
	repo := gitInitRepoT(t)
	repoAo := filepath.Join(repo, "bin", "ao")
	if err := os.MkdirAll(filepath.Dir(repoAo), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(repoAo, []byte("#!/bin/sh\n"), 0o755); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("write repo ao: %v", err)
	}
	setInitSelfBinary(t, func() (string, error) { return repoAo, nil })

	t.Chdir(repo)
	setInitRemove(t, false)
	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&buf)
	c.SetErr(&buf)
	err := runVerifyInit(c, nil)
	if err == nil {
		t.Fatalf("install from a repo-internal ao must be refused; got success\n%s", buf.String())
	}
	if !strings.Contains(err.Error(), "repo-internal ao path") {
		t.Fatalf("refusal must name the repo-internal-path reason: %v", err)
	}
	if _, statErr := os.Stat(hookPathT(repo)); !os.IsNotExist(statErr) {
		t.Fatalf("no hook must be written when the install is refused")
	}
}

func hookPathT(repo string) string { return filepath.Join(repo, ".git", "hooks", "pre-push") }
func origPathT(repo string) string { return filepath.Join(repo, ".git", "hooks", origHookName) }

func readT(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func sha256hex(b []byte) [32]byte { return sha256.Sum256(b) }

func TestVerifyInit_InstallsExecutableRatchet(t *testing.T) {
	repo := gitInitRepoT(t)
	out := runInitT(t, repo, false)
	if !strings.Contains(out, "installed") {
		t.Fatalf("expected install confirmation:\n%s", out)
	}

	hook := readT(t, hookPathT(repo))
	if !isAgentopsHook(hook) {
		t.Fatalf("installed hook missing the AGENTOPS-VERIFY-RATCHET marker")
	}
	// The placeholder must be substituted with the running binary's absolute path.
	if strings.Contains(string(hook), aoBinPlaceholder) {
		t.Fatalf("hook still carries the unsubstituted %s placeholder", aoBinPlaceholder)
	}
	self := resolveSelfBinaryPath()
	if !strings.Contains(string(hook), self) {
		t.Fatalf("hook should invoke the install-time ao path %q", self)
	}
	// The hook must be executable.
	info, err := os.Stat(hookPathT(repo))
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("hook is not executable: mode=%v", info.Mode())
	}
	// No sidecar when there was no pre-existing hook.
	if _, err := os.Stat(origPathT(repo)); !os.IsNotExist(err) {
		t.Fatalf("no sidecar should be created for a fresh install")
	}
}

func TestVerifyInit_Idempotent(t *testing.T) {
	repo := gitInitRepoT(t)
	runInitT(t, repo, false)
	first := readT(t, hookPathT(repo))
	out := runInitT(t, repo, false)
	if !strings.Contains(out, "refreshed") {
		t.Fatalf("second init should report a refresh:\n%s", out)
	}
	second := readT(t, hookPathT(repo))
	if sha256hex(first) != sha256hex(second) {
		t.Fatalf("idempotent re-init changed the hook bytes")
	}
	// Marker must appear exactly once (begin marker), not duplicated.
	if n := strings.Count(string(second), ">>> "+hookMarker); n != 1 {
		t.Fatalf("expected exactly one begin marker after re-init, got %d", n)
	}
}

func TestVerifyInit_RemoveStandalone(t *testing.T) {
	repo := gitInitRepoT(t)
	runInitT(t, repo, false)
	out := runInitT(t, repo, true)
	if !strings.Contains(out, "removed") {
		t.Fatalf("expected removal confirmation:\n%s", out)
	}
	if _, err := os.Stat(hookPathT(repo)); !os.IsNotExist(err) {
		t.Fatalf("hook should be gone after --remove")
	}
}

func TestVerifyInit_ChainsAndRestoresByteIdentically(t *testing.T) {
	repo := gitInitRepoT(t)
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	// A pre-existing foreign hook with distinctive bytes + mode.
	orig := []byte("#!/usr/bin/env sh\n# my custom hook\necho custom >&2\nexit 0\n")
	if err := os.WriteFile(hookPathT(repo), orig, 0o755); err != nil {
		t.Fatalf("write foreign hook: %v", err)
	}
	origHash := sha256hex(orig)

	// Install → the foreign hook is set aside and chained.
	out := runInitT(t, repo, false)
	if !strings.Contains(out, "chained") {
		t.Fatalf("expected chaining message:\n%s", out)
	}
	if !isAgentopsHook(readT(t, hookPathT(repo))) {
		t.Fatalf("our hook should now occupy pre-push")
	}
	if sha256hex(readT(t, origPathT(repo))) != origHash {
		t.Fatalf("sidecar must be byte-identical to the original hook")
	}

	// Re-init must NOT re-chain (must not turn our own hook into the original).
	runInitT(t, repo, false)
	if sha256hex(readT(t, origPathT(repo))) != origHash {
		t.Fatalf("re-init corrupted the chained original sidecar")
	}

	// Remove → the original is restored byte-identically and the sidecar is gone.
	runInitT(t, repo, true)
	restored := readT(t, hookPathT(repo))
	if sha256hex(restored) != origHash {
		t.Fatalf("--remove did not restore the original hook byte-identically")
	}
	if _, err := os.Stat(origPathT(repo)); !os.IsNotExist(err) {
		t.Fatalf("sidecar should be gone after --remove")
	}
}

func TestVerifyInit_RemoveLeavesForeignHookUntouched(t *testing.T) {
	repo := gitInitRepoT(t)
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	foreign := []byte("#!/bin/sh\necho not-ours\n")
	if err := os.WriteFile(hookPathT(repo), foreign, 0o755); err != nil {
		t.Fatalf("write foreign hook: %v", err)
	}
	out := runInitT(t, repo, true)
	if !strings.Contains(out, "not managed by AgentOps") {
		t.Fatalf("remove should decline to touch a foreign hook:\n%s", out)
	}
	if sha256hex(readT(t, hookPathT(repo))) != sha256hex(foreign) {
		t.Fatalf("a foreign hook must be left untouched by --remove")
	}
}

func TestResolveHooksDir_HonorsCoreHooksPath(t *testing.T) {
	repo := gitInitRepoT(t)
	custom := filepath.Join(repo, "myhooks")
	runGitT(t, repo, "config", "core.hooksPath", "myhooks")

	got, err := resolveHooksDir(repo)
	if err != nil {
		t.Fatalf("resolveHooksDir: %v", err)
	}
	if got != custom {
		t.Fatalf("resolveHooksDir honoring core.hooksPath = %q, want %q", got, custom)
	}
}

func TestResolveHooksDir_DefaultsToGitDirHooks(t *testing.T) {
	repo := gitInitRepoT(t)
	got, err := resolveHooksDir(repo)
	if err != nil {
		t.Fatalf("resolveHooksDir: %v", err)
	}
	want := filepath.Join(repo, ".git", "hooks")
	if got != want {
		t.Fatalf("default hooks dir = %q, want %q", got, want)
	}
}

// A core.hooksPath value git resolves itself (here a ~-expansion) must install
// where GIT actually runs the hook — not where a naive absolute-vs-repo-relative
// split lands it. "~/aohooks" expands to $HOME/aohooks; the old code treated it
// as repo-relative and installed into <repo>/~/aohooks, where git never looks —
// `ao verify init` reported success while the ratchet silently never ran.
func TestVerifyInit_HonorsGitResolvedHooksPath(t *testing.T) {
	repo := gitInitRepoT(t)
	home := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved // macOS /var -> /private/var, so git's abs path compares
	}
	t.Setenv("HOME", home)
	runGitT(t, repo, "config", "core.hooksPath", "~/aohooks")

	// Ground truth: where git ITSELF says the hook lives.
	want := strings.TrimSpace(runGitT(t, repo, "rev-parse", "--path-format=absolute", "--git-path", "hooks/pre-push"))
	if !strings.HasPrefix(want, home) || strings.HasPrefix(want, repo) {
		t.Fatalf("precondition: git should resolve ~/aohooks under HOME=%s (not repo), got %s", home, want)
	}

	out := runInitT(t, repo, false)
	if !strings.Contains(out, "installed") {
		t.Fatalf("expected install confirmation:\n%s", out)
	}
	// resolveHooksDir must agree with git's own resolution.
	gotDir, err := resolveHooksDir(repo)
	if err != nil {
		t.Fatalf("resolveHooksDir: %v", err)
	}
	if gotDir != filepath.Dir(want) {
		t.Fatalf("resolveHooksDir = %q, want git-resolved %q", gotDir, filepath.Dir(want))
	}
	// The hook is installed at git's resolved path, marker present.
	if !isAgentopsHook(readT(t, want)) {
		t.Fatalf("hook not installed (with marker) at git's resolved path %s", want)
	}
	// And NOT at the naive repo-joined wrong path.
	wrong := filepath.Join(repo, "~", "aohooks", "pre-push")
	if _, statErr := os.Stat(wrong); statErr == nil {
		t.Fatalf("hook wrongly installed at the repo-joined path %s (git never looks there)", wrong)
	}
}

// A core.hooksPath ending in a SPACE is preserved verbatim by git; resolveHooksDir
// must strip ONLY git's trailing newline, never the space (the old strings.TrimSpace
// dropped it). Otherwise init writes the hook at "/x/pre-push" while git runs
// "/x /pre-push" — init reports success and the ratchet silently never fires
// (age-rk3r.6 cross-family refuter).
func TestVerifyInit_HonorsGitResolvedHooksPathTrailingSpace(t *testing.T) {
	repo := gitInitRepoT(t)
	base := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved // macOS /var -> /private/var, so git's abs path compares
	}
	hooksVal := filepath.Join(base, "aohooks") + " " // legitimate trailing space
	runGitT(t, repo, "config", "core.hooksPath", hooksVal)

	// Ground truth: git's own resolved hook path — strip ONLY the trailing newline,
	// never the space under test (TrimSpace here would defeat the test).
	raw := runGitT(t, repo, "rev-parse", "--path-format=absolute", "--git-path", "hooks/pre-push")
	want := strings.TrimSuffix(strings.TrimSuffix(raw, "\n"), "\r")
	if !strings.HasSuffix(filepath.Dir(want), " ") {
		t.Skipf("this git does not preserve a trailing-space core.hooksPath (git resolved %q) — nothing to exercise", want)
	}

	if out := runInitT(t, repo, false); !strings.Contains(out, "installed") {
		t.Fatalf("expected install confirmation:\n%s", out)
	}
	// resolveHooksDir must agree with git's own (space-preserving) resolution.
	gotDir, err := resolveHooksDir(repo)
	if err != nil {
		t.Fatalf("resolveHooksDir: %v", err)
	}
	if gotDir != filepath.Dir(want) {
		t.Fatalf("resolveHooksDir = %q, want git-resolved %q (the trailing space must be preserved)", gotDir, filepath.Dir(want))
	}
	// The hook is installed at git's resolved path (with the space), marker present.
	if !isAgentopsHook(readT(t, want)) {
		t.Fatalf("hook not installed (with marker) at git's resolved space-preserving path %q", want)
	}
}
