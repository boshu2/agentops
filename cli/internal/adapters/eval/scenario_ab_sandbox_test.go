package eval

import (
	"context"
	"errors"
	"fmt"
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
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	deny, err := corpusDenyPaths(root)
	if err != nil {
		t.Fatalf("corpusDenyPaths: %v", err)
	}
	joined := strings.Join(deny, "|")
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
	t.Setenv("HOME", t.TempDir()) // isolate from the real ~/.agents symlink farm (deterministic closure)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(t.TempDir(), "elsewhere-corpus")
	t.Setenv("AO_HOME", "")
	t.Setenv("AO_AGENTS_DIR", custom)
	deny, err := corpusDenyPaths(root)
	if err != nil {
		t.Fatalf("corpusDenyPaths: %v", err)
	}
	joined := strings.Join(deny, "|")
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

// TestCorpusDenyPaths_NestedSymlinkTargetDenied (age-6j9ee.3, construction, no GOOS
// skip): a directory symlink INSIDE a corpus root (.agents/learnings -> /external)
// canonicalizes outside the (subpath .agents) deny, so its resolved target must be
// added to the deny set. A top-level resolveSymlink on .agents never reaches it.
func TestCorpusDenyPaths_NestedSymlinkTargetDenied(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	agents := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "secret.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(agents, "learnings")); err != nil {
		t.Fatal(err)
	}
	// Seatbelt matches the canonical path, so assert against the resolved target.
	wantTarget, err := filepath.EvalSymlinks(external)
	if err != nil {
		t.Fatal(err)
	}
	deny, err := corpusDenyPaths(root)
	if err != nil {
		t.Fatalf("corpusDenyPaths: %v", err)
	}
	joined := strings.Join(deny, "|")
	if !strings.Contains(joined, wantTarget) {
		t.Fatalf("nested symlink target %q must be in the deny set; got %s", wantTarget, joined)
	}
}

// TestCorpusDenyPaths_NestedFileSymlinkTargetDenied (age-6j9ee.3, construction, no
// GOOS skip): a FILE symlink INSIDE a corpus root (.agents/note.md -> /external/secret.txt)
// canonicalizes outside the (subpath .agents) deny, so its resolved file target must be
// added to the deny set. The nested walk previously only appended DIRECTORY targets, so
// a file symlink's target was silently dropped and stayed readable by the control arm.
func TestCorpusDenyPaths_NestedFileSymlinkTargetDenied(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	agents := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	secretFile := filepath.Join(external, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secretFile, filepath.Join(agents, "note.md")); err != nil {
		t.Fatal(err)
	}
	// Seatbelt matches the canonical path, so assert against the resolved file target.
	wantTarget, err := filepath.EvalSymlinks(secretFile)
	if err != nil {
		t.Fatal(err)
	}
	deny, err := corpusDenyPaths(root)
	if err != nil {
		t.Fatalf("corpusDenyPaths: %v", err)
	}
	joined := strings.Join(deny, "|")
	if !strings.Contains(joined, wantTarget) {
		t.Fatalf("nested FILE symlink target %q must be in the deny set; got %s", wantTarget, joined)
	}
}

// TestSymlinkClosureDenyTargets_SkipsDanglingSymlink (age-6j9ee.3): a dangling nested
// symlink (target does not exist) resolves to nothing via EvalSymlinks and must be
// skipped without error, contributing no deny path.
func TestSymlinkClosureDenyTargets_SkipsDanglingSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "does-not-exist"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	got, err := symlinkClosureDenyTargets([]string{root}, maxSymlinkClosureDirs, maxSymlinkDenyEntries, maxSymlinkWalkDepth)
	if err != nil {
		t.Fatalf("symlinkClosureDenyTargets: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("dangling symlink must contribute no deny target; got %v", got)
	}
}

// TestCorpusDenyPaths_ChainedSymlinkClosure (age-6j9ee.3, the round-3 refute, GREEN):
// the EXACT chained-escape shape the validator refuted twice as a class —
//
//	root/.agents/learnings -> ext1        (dir target; the FIRST hop)
//	ext1/pivot.md          -> ext2/secret.txt (the SECOND hop, INSIDE the resolved target)
//
// The single-pass walk added only ext1 and never descended INTO it, so ext2's canonical
// FILE target escaped the deny set and leaked (26 bytes, exit 0). The fixpoint closure
// descends into the resolved external DIRECTORY ext1, resolves ext1/pivot.md, and denies
// ext2's canonical target too. BOTH hops must land in the deny set.
func TestCorpusDenyPaths_ChainedSymlinkClosure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	agents := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	ext1 := t.TempDir()
	ext2 := t.TempDir()
	secret := filepath.Join(ext2, "secret.txt")
	if err := os.WriteFile(secret, []byte("CHAINED-CORPUS-SECRET-26B"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(ext1, "pivot.md")); err != nil {
		t.Fatal(err) // ext1/pivot.md -> ext2/secret.txt (second hop)
	}
	if err := os.Symlink(ext1, filepath.Join(agents, "learnings")); err != nil {
		t.Fatal(err) // .agents/learnings -> ext1 (first hop)
	}
	wantExt1, err := filepath.EvalSymlinks(ext1)
	if err != nil {
		t.Fatal(err)
	}
	wantExt2, err := filepath.EvalSymlinks(secret)
	if err != nil {
		t.Fatal(err)
	}
	deny, err := corpusDenyPaths(root)
	if err != nil {
		t.Fatalf("corpusDenyPaths: %v", err)
	}
	joined := strings.Join(deny, "|")
	if !strings.Contains(joined, wantExt1) {
		t.Errorf("first-hop dir target %q must be denied; got %s", wantExt1, joined)
	}
	if !strings.Contains(joined, wantExt2) {
		t.Fatalf("CHAINED second-hop file target %q must be in the closure (round-3 leak); got %s", wantExt2, joined)
	}
}

// TestCorpusDenyPaths_DeeperChainClosure (age-6j9ee.3): a 3-hop chain must be fully
// closed — .agents/l1 -> extA; extA/l2 -> extB; extB/l3 -> extC/secret.txt. The
// closure descends hop by hop; the final file target must be denied.
func TestCorpusDenyPaths_DeeperChainClosure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	agents := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	extA, extB, extC := t.TempDir(), t.TempDir(), t.TempDir()
	secret := filepath.Join(extC, "secret.txt")
	if err := os.WriteFile(secret, []byte("DEEP-CHAIN-SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(extB, "l3")); err != nil {
		t.Fatal(err) // extB/l3 -> extC/secret.txt
	}
	if err := os.Symlink(extB, filepath.Join(extA, "l2")); err != nil {
		t.Fatal(err) // extA/l2 -> extB
	}
	if err := os.Symlink(extA, filepath.Join(agents, "l1")); err != nil {
		t.Fatal(err) // .agents/l1 -> extA
	}
	wantFinal, err := filepath.EvalSymlinks(secret)
	if err != nil {
		t.Fatal(err)
	}
	deny, err := corpusDenyPaths(root)
	if err != nil {
		t.Fatalf("corpusDenyPaths: %v", err)
	}
	if joined := strings.Join(deny, "|"); !strings.Contains(joined, wantFinal) {
		t.Fatalf("3-hop chain final target %q must be in the closure; got %s", wantFinal, joined)
	}
}

// TestSymlinkClosureDenyTargets_CycleTerminates (age-6j9ee.3): a symlink cycle
// a/link -> b, b/link -> a must terminate (no infinite loop, no error) and deny both
// canonical directory targets. Termination is proven by the test completing; the
// seen-set dedup before pushing to the worklist is the guard.
func TestSymlinkClosureDenyTargets_CycleTerminates(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if err := os.Symlink(b, filepath.Join(a, "link")); err != nil {
		t.Fatal(err) // a/link -> b
	}
	if err := os.Symlink(a, filepath.Join(b, "link")); err != nil {
		t.Fatal(err) // b/link -> a (cycle)
	}
	got, err := symlinkClosureDenyTargets([]string{a}, maxSymlinkClosureDirs, maxSymlinkDenyEntries, maxSymlinkWalkDepth)
	if err != nil {
		t.Fatalf("cycle must not error: %v", err)
	}
	canonA, _ := filepath.EvalSymlinks(a)
	canonB, _ := filepath.EvalSymlinks(b)
	joined := strings.Join(got, "|")
	for _, want := range []string{canonA, canonB} {
		if !strings.Contains(joined, want) {
			t.Errorf("cycle closure must deny %q; got %v", want, got)
		}
	}
}

// TestSymlinkClosureDenyTargets_DirOverflowFailsClosed (age-6j9ee.3): a symlink into an
// external tree with more directories than the dir cap cannot be enumerated to a complete
// closure, so the closure returns errSymlinkClosureOverflow and a nil set — never a partial
// one. Tiny cap parameter keeps the fixture tiny (the same code path a symlink to `/` hits).
func TestSymlinkClosureDenyTargets_DirOverflowFailsClosed(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	for i := 0; i < 5; i++ { // 5 external subdirs > maxDirs=2
		if err := os.MkdirAll(filepath.Join(external, fmt.Sprintf("d%d", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(external, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	got, err := symlinkClosureDenyTargets([]string{root}, 2, maxSymlinkDenyEntries, maxSymlinkWalkDepth)
	if err == nil {
		t.Fatalf("oversized external tree must fail closed; got %d entries, nil error", len(got))
	}
	if !errors.Is(err, errSymlinkClosureOverflow) {
		t.Fatalf("want errSymlinkClosureOverflow; got %v", err)
	}
	if got != nil {
		t.Errorf("fail-closed must return a nil set (never a partial one); got %v", got)
	}
}

// TestSymlinkClosureDenyTargets_EntryOverflowFailsClosed (age-6j9ee.3): more distinct
// canonical targets than the entry cap also fails closed (the second bound). A pathological
// fan-out of many symlink targets cannot silently truncate the deny set.
func TestSymlinkClosureDenyTargets_EntryOverflowFailsClosed(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 4; i++ { // 4 external file targets > maxEntries=1
		f := filepath.Join(t.TempDir(), "s.txt")
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(f, filepath.Join(root, fmt.Sprintf("l%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	got, err := symlinkClosureDenyTargets([]string{root}, maxSymlinkClosureDirs, 1, maxSymlinkWalkDepth)
	if err == nil {
		t.Fatalf("exceeding the entry cap must fail closed; got %d entries, nil error", len(got))
	}
	if !errors.Is(err, errSymlinkClosureOverflow) {
		t.Fatalf("want errSymlinkClosureOverflow; got %v", err)
	}
}

// TestWorkspaceCommand_OverflowPropagatesRefusal (age-6j9ee.3, guard refusal path, real
// const): when the closure overflows the PRODUCTION dir cap, a real caller (workspaceCommand,
// the agentic control arm) must return the error and NEVER a runnable command — the arm
// refuses to run unisolated, exactly like the no-Seatbelt case. Exercises the real const
// end-to-end via a genuine oversized external tree reached through a corpus symlink. No GOOS
// gate: the refusal happens before any sandbox exec.
func TestWorkspaceCommand_OverflowPropagatesRefusal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	agents := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	bigExternal := t.TempDir()
	for i := 0; i < maxSymlinkClosureDirs+16; i++ {
		if err := os.Mkdir(filepath.Join(bigExternal, fmt.Sprintf("d%04d", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(bigExternal, filepath.Join(agents, "learnings")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	cmd, err := workspaceCommand(context.Background(), t.TempDir(), "cat something", false)
	if err == nil {
		t.Fatalf("control arm must refuse (fail closed) when the deny closure overflows; got cmd=%v", cmd)
	}
	if cmd != nil {
		t.Errorf("no command may be returned on refusal; got %v", cmd)
	}
	if !errors.Is(err, errSymlinkClosureOverflow) {
		t.Fatalf("refusal must carry the overflow cause; got %v", err)
	}
}

// TestWorkspaceCommandRunner_Integration_ControlArmDeniesNestedFileSymlinkRead
// (age-6j9ee.3, darwin): replicates the cross-family validator's probe EXACTLY — a
// control-arm command reading THROUGH a nested corpus FILE symlink
// (.agents/note.md -> <external secret file>) must be DENIED. Before the fix the read
// returned exit 0 and leaked the secret; after, the read canonicalizes to the external
// file target the nested-symlink walk now adds to the deny set.
func TestWorkspaceCommandRunner_Integration_ControlArmDeniesNestedFileSymlinkRead(t *testing.T) {
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
	external := t.TempDir()
	secretFile := filepath.Join(external, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("NESTED-FILE-CORPUS-SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(agents, "note.md")
	if err := os.Symlink(secretFile, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	workDir := t.TempDir()

	// The real leak: the corpus reader RESOLVES the symlink and reads the canonical
	// target path (a plain read of .agents/note.md is denied because the symlink NODE
	// sits under the denied .agents subpath, masking the escape; reading the resolved
	// target is what leaked with exit 0 — the validator's observed vector). Reading the
	// canonical target must be denied because the nested-symlink walk added it.
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	stdout, exitCode, err := defaultWorkspaceCommandRunner(context.Background(), workDir, "cat "+resolved, false)
	if err != nil {
		t.Fatalf("runner returned a Go error (want a denied read = nonzero exit): %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("control arm read canonical target of nested corpus FILE symlink (isolation void): exit=0 stdout=%q", stdout)
	}
	if strings.Contains(stdout, "NESTED-FILE-CORPUS-SECRET") {
		t.Fatalf("control arm leaked corpus bytes via nested FILE symlink target: stdout=%q", stdout)
	}
}

// TestWorkspaceCommandRunner_Integration_ControlArmDeniesNestedSymlinkRead
// (age-6j9ee.3, darwin): a control-arm command reading THROUGH a nested corpus symlink
// (.agents/learnings -> /external) must be DENIED — the read canonicalizes to the
// external target, which the nested-symlink walk added to the deny set.
func TestWorkspaceCommandRunner_Integration_ControlArmDeniesNestedSymlinkRead(t *testing.T) {
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
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "sentinel.txt"), []byte("NESTED-CORPUS-SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(agents, "learnings")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	workDir := t.TempDir()

	stdout, exitCode, err := defaultWorkspaceCommandRunner(context.Background(), workDir, "cat "+filepath.Join(link, "sentinel.txt"), false)
	if err != nil {
		t.Fatalf("runner returned a Go error (want a denied read = nonzero exit): %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("control arm read through nested corpus symlink (isolation void): exit=0 stdout=%q", stdout)
	}
	if strings.Contains(stdout, "NESTED-CORPUS-SECRET") {
		t.Fatalf("control arm leaked corpus bytes via nested symlink: stdout=%q", stdout)
	}
}

// TestWorkspaceCommandRunner_Integration_ControlArmDeniesChainedSymlinkRead
// (age-6j9ee.3, darwin, the round-3 vector RUN): the CHAINED escape end to end —
// .agents/learnings -> ext1; ext1/pivot.md -> ext2/secret.txt. Reading the FINAL resolved
// target (ext2's canonical secret file) leaked 26 bytes with exit 0 before the fix. The
// fixpoint closure denies ext2's canonical target, so the read is now DENIED (nonzero
// exit, zero corpus bytes).
func TestWorkspaceCommandRunner_Integration_ControlArmDeniesChainedSymlinkRead(t *testing.T) {
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
	ext1 := t.TempDir()
	ext2 := t.TempDir()
	secret := filepath.Join(ext2, "secret.txt")
	if err := os.WriteFile(secret, []byte("CHAINED-FINAL-CORPUS-SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(ext1, "pivot.md")); err != nil {
		t.Fatal(err) // ext1/pivot.md -> ext2/secret.txt
	}
	if err := os.Symlink(ext1, filepath.Join(agents, "learnings")); err != nil {
		t.Fatal(err) // .agents/learnings -> ext1
	}
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	workDir := t.TempDir()

	// The observed vector: the corpus reader canonicalizes the whole chain and reads the
	// final resolved target (ext2/secret.txt). That canonical path must be denied.
	resolved, err := filepath.EvalSymlinks(secret)
	if err != nil {
		t.Fatal(err)
	}
	stdout, exitCode, err := defaultWorkspaceCommandRunner(context.Background(), workDir, "cat "+resolved, false)
	if err != nil {
		t.Fatalf("runner returned a Go error (want a denied read = nonzero exit): %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("control arm read final resolved target of a CHAINED corpus symlink (isolation void): exit=0 stdout=%q", stdout)
	}
	if strings.Contains(stdout, "CHAINED-FINAL-CORPUS-SECRET") {
		t.Fatalf("control arm leaked corpus bytes via chained symlink: stdout=%q", stdout)
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

	deny, err := corpusDenyPaths(root)
	if err != nil {
		t.Fatalf("corpusDenyPaths: %v", err)
	}
	profile := sandboxExecDenyProfile(deny)

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
