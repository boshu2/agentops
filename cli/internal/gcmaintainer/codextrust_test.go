package gcmaintainer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realHooksDoc is the shape a Gas City per-provider overlay drops into a
// session home: several events, one with two matcher groups, one group with
// two hooks. Deriving keys from it exercises every index dimension.
const realHooksDoc = `{
  "hooks": {
    "SessionStart": [
      {"matcher": "startup", "hooks": [{"type": "command", "command": "gc prime"}]},
      {"matcher": "", "hooks": [{"type": "command", "command": "gc prime --again"}]}
    ],
    "PreCompact": [
      {"matcher": "", "hooks": [{"type": "command", "command": "gc handoff"}]}
    ],
    "UserPromptSubmit": [
      {"matcher": "", "hooks": [
        {"type": "command", "command": "gc nudge drain"},
        {"type": "command", "command": "gc mail check"}
      ]}
    ]
  }
}`

// addSessionHome materializes an agent session home carrying a Codex overlay,
// the shape a Gas City pack drops into `<city>/.gc/agents/<name>`.
func (f *fixture) addSessionHome(t *testing.T, name string) string {
	t.Helper()
	home := filepath.Join(f.city, ".gc", "agents", name)
	mustWrite(t, filepath.Join(home, ".codex", "hooks.json"), realHooksDoc)
	return home
}

// addRigWorktree materializes a rig worktree root, which a session recognizes
// by its `.git` entry rather than a Codex overlay.
func (f *fixture) addRigWorktree(t *testing.T, rig, name string) string {
	t.Helper()
	worktree := filepath.Join(f.city, ".gc", "worktrees", rig, name)
	mustWrite(t, filepath.Join(worktree, ".git"), "gitdir: /elsewhere\n")
	return worktree
}

// hooksFor builds the codex hooks/list answer for a session home by deriving
// the real key set from its on-disk hooks.json, so the fixture can never claim
// a key shape production would not produce.
func (f *fixture) hooksFor(t *testing.T, dir string) []codexHook {
	t.Helper()
	keys, err := derivedHookKeys(mustCanonical(t, dir))
	if err != nil {
		t.Fatalf("derive keys for %s: %v", dir, err)
	}
	if len(keys) == 0 {
		t.Fatalf("no hook keys derived for %s", dir)
	}
	hooks := make([]codexHook, 0, len(keys))
	for index, key := range keys {
		hooks = append(hooks, codexHook{
			Key:         key,
			CurrentHash: hashFor(dir, index),
		})
	}
	return hooks
}

// hashFor produces a distinct, stable stand-in digest per hook identity.
func hashFor(dir string, index int) string {
	return "sha256:" + strings.ReplaceAll(filepath.Base(dir), "/", "_") + "-" + string(rune('a'+index%26))
}

func (f *fixture) trustStore(t *testing.T) string {
	t.Helper()
	return mustReadFile(t, f.codexTrust)
}

func (f *fixture) mustReadStore(t *testing.T) *trustStore {
	t.Helper()
	store, err := readCodexTrustStore(f.codexTrust)
	if err != nil {
		t.Fatalf("read trust store: %v", err)
	}
	return store
}

func TestPrepare_PreSeedsCodexWorkspaceAndHookTrustForEverySessionDirectory(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	nested := f.addSessionHome(t, filepath.Join("agentops", "witness"))
	worktree := f.addRigWorktree(t, "smoke", "polecat")
	hooks := append(f.hooksFor(t, home), f.hooksFor(t, nested)...)
	f.setCodexHooks(t, hooks...)

	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	store := f.mustReadStore(t)
	for _, dir := range []string{f.city, f.rig, home, nested, worktree} {
		resolved := mustCanonical(t, dir)
		if !store.workspaceTrusted(resolved) {
			t.Errorf("workspace %s is not actually trusted (level=%q)", resolved, store.workspaceLevel(resolved))
		}
	}
	for _, hook := range hooks {
		if !store.hookTrustSatisfied(hook.Key) {
			t.Errorf("hook %s is not trusted after prepare", hook.Key)
		}
		if got := store.Hooks.State[hook.Key].TrustedHash; got != hook.CurrentHash {
			t.Errorf("hook %s recorded hash %q, want %q", hook.Key, got, hook.CurrentHash)
		}
	}
	// The seeded store must be exactly the state `check` then accepts.
	if _, err := f.run(t, Check, nil); err != nil {
		t.Fatalf("check after prepare: %v", err)
	}
}

func TestPrepare_CodexTrustIsIdempotentAndPreservesExistingDecisions(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	hooks := f.hooksFor(t, home)

	// A pre-existing store: unrelated operator settings, an already trusted
	// city, and one hook the operator already trusted.
	settled := hooks[0]
	preexisting := "model = \"gpt-5.6\"\n\n" +
		"[projects." + tomlQuote(mustCanonical(t, f.city)) + "]\ntrust_level = \"trusted\"\n\n" +
		"[hooks.state." + tomlQuote(settled.Key) + "]\ntrusted_hash = " + tomlQuote(settled.CurrentHash) + "\n"
	mustWrite(t, f.codexTrust, preexisting)
	// The fake derives status from the store, so it reports the recorded hook
	// as trusted and the rest as new - exactly what codex would report.
	f.setCodexHooks(t, hooks...)

	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	afterFirst := f.trustStore(t)
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("second prepare: %v", err)
	}
	afterSecond := f.trustStore(t)

	if afterFirst != afterSecond {
		t.Fatalf("second prepare mutated the trust store:\n--- first ---\n%s\n--- second ---\n%s", afterFirst, afterSecond)
	}
	if !strings.HasPrefix(afterSecond, preexisting) {
		t.Fatal("prepare did not preserve the pre-existing trust store verbatim")
	}
	if got := strings.Count(afterSecond, "[hooks.state."+tomlQuote(settled.Key)+"]"); got != 1 {
		t.Errorf("hook table appears %d times, want 1", got)
	}
	if got := f.mustReadStore(t).Hooks.State[settled.Key].TrustedHash; got != settled.CurrentHash {
		t.Errorf("prepare rewrote the settled hook hash to %q", got)
	}
	if _, err := readCodexTrustStore(f.codexTrust); err != nil {
		t.Fatalf("store is unparsable after two prepares: %v", err)
	}
}

// R3-F1: a hook recorded `enabled = false` is a disabled hook, not a trusted
// working one. prepare must surface it and check must name it.
func TestPrepare_RefusesADisabledHookInsteadOfTreatingItAsTrusted(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	hooks := f.hooksFor(t, home)
	f.setCodexHooks(t, hooks...)
	mustWrite(t, f.codexTrust,
		"[hooks.state."+tomlQuote(hooks[0].Key)+"]\ntrusted_hash = "+tomlQuote(hooks[0].CurrentHash)+"\nenabled = false\n")

	_, err := f.run(t, Prepare, nil)
	wantErrContaining(t, err, "is recorded as enabled = false")
	wantErrContaining(t, err, hooks[0].Key)
}

func TestCheck_NamesADisabledHook(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	hooks := f.hooksFor(t, home)
	f.setCodexHooks(t, hooks...)
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	store := f.trustStore(t)
	mustWrite(t, f.codexTrust, strings.Replace(store,
		"[hooks.state."+tomlQuote(hooks[0].Key)+"]\ntrusted_hash = "+tomlQuote(hooks[0].CurrentHash)+"\n",
		"[hooks.state."+tomlQuote(hooks[0].Key)+"]\ntrusted_hash = "+tomlQuote(hooks[0].CurrentHash)+"\nenabled = false\n", 1))

	_, err := f.run(t, Check, nil)
	wantErrContaining(t, err, "is recorded as enabled = false")
	wantErrContaining(t, err, hooks[0].Key)
}

// R3-F3: whatever prepare accepts as complete, check must accept by the same
// rule. A `managed` hook is the case where the two could drift apart.
func TestPrepareThenCheck_RoundTripsAManagedHook(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	hooks := f.hooksFor(t, home)
	f.setCodexHooks(t, hooks...)
	f.forceCodexHookStatus(t, "managed")

	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	// prepare must have recorded it, because check cannot ask codex whether a
	// hook is managed.
	store := f.mustReadStore(t)
	for _, hook := range hooks {
		if !store.hookTrustSatisfied(hook.Key) {
			t.Errorf("managed hook %s is not satisfied after prepare (%s)", hook.Key, store.hookTrustDefect(hook.Key))
		}
	}
	if _, err := f.run(t, Check, nil); err != nil {
		t.Fatalf("check must accept what prepare accepted for a managed hook: %v", err)
	}
}

// R3-F2: no failure path may leave the operator's config corrupted.
func TestAppendCodexConfig_LeavesTheOriginalIntactWhenTheMergeWouldNotParse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model = \"gpt-5.6\"\n\n[projects.\"/a\"]\ntrust_level = \"trusted\"\n"
	mustWrite(t, path, original)

	// A duplicate table is exactly the corruption an append could introduce.
	err := appendCodexConfig(path, "\n[projects.\"/a\"]\ntrust_level = \"trusted\"\n")
	wantErrContaining(t, err, "would not be parsable")

	if got := mustReadFile(t, path); got != original {
		t.Fatalf("the operator's config was modified by a rejected append:\n%s", got)
	}
	if _, err := readCodexTrustStore(path); err != nil {
		t.Fatalf("original config no longer parses: %v", err)
	}
	// No debris: neither the staged temp file nor a stale backup survives.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "config.toml" {
			t.Errorf("rejected append left %s behind", entry.Name())
		}
	}
}

func TestAppendCodexConfig_PreservesModeAndLeavesNoBackupOnSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	mustWrite(t, path, "[projects.\"/a\"]\ntrust_level = \"trusted\"\n")
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if err := appendCodexConfig(path, "\n[projects.\"/b\"]\ntrust_level = \"trusted\"\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640 preserved", info.Mode().Perm())
	}
	if _, err := os.Stat(path + codexTrustBackupSuffix); !os.IsNotExist(err) {
		t.Errorf("backup survived a successful append: %v", err)
	}
	store, err := readCodexTrustStore(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !store.workspaceTrusted("/a") || !store.workspaceTrusted("/b") {
		t.Error("append lost an entry")
	}
}

// F1: an entry that exists but does not confer trust must not be treated as
// trusted by either command.
func TestPrepare_RefusesAnExplicitlyUntrustedWorkspaceInsteadOfReportingSuccess(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	mustWrite(t, f.codexTrust,
		"[projects."+tomlQuote(mustCanonical(t, home))+"]\ntrust_level = \"untrusted\"\n")

	_, err := f.run(t, Prepare, nil)
	wantErrContaining(t, err, `is recorded as trust_level "untrusted"`)
	wantErrContaining(t, err, mustCanonical(t, home))
	if strings.Contains(f.trustStore(t), `"trusted"`) {
		t.Fatal("prepare flipped an explicit untrusted decision")
	}
}

func TestCheck_RejectsAnExplicitlyUntrustedWorkspace(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	f.setCodexHooks(t, f.hooksFor(t, home)...)
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	// Flip the recorded level: the table is still present, but trust is gone.
	store := f.trustStore(t)
	target := "[projects." + tomlQuote(mustCanonical(t, home)) + "]\ntrust_level = \"trusted\""
	if !strings.Contains(store, target) {
		t.Fatalf("prepare did not write the expected project table:\n%s", store)
	}
	mustWrite(t, f.codexTrust, strings.Replace(store, target,
		"[projects."+tomlQuote(mustCanonical(t, home))+"]\ntrust_level = \"untrusted\"", 1))

	_, err := f.run(t, Check, nil)
	wantErrContaining(t, err, `is recorded as trust_level "untrusted"`)
	wantErrContaining(t, err, mustCanonical(t, home))
}

func TestCheck_RejectsAHookEntryWithNoUsableHash(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	hooks := f.hooksFor(t, home)
	f.setCodexHooks(t, hooks...)
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	// A present table whose hash is empty must not read as trusted.
	store := f.trustStore(t)
	mustWrite(t, f.codexTrust, strings.Replace(store,
		"[hooks.state."+tomlQuote(hooks[0].Key)+"]\ntrusted_hash = "+tomlQuote(hooks[0].CurrentHash),
		"[hooks.state."+tomlQuote(hooks[0].Key)+"]\ntrusted_hash = \"\"", 1))

	_, err := f.run(t, Check, nil)
	wantErrContaining(t, err, "no usable trusted_hash")
	wantErrContaining(t, err, hooks[0].Key)
}

func TestPrepare_RefusesToSilentlyReTrustAModifiedHook(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	hooks := f.hooksFor(t, home)
	f.setCodexHooks(t, hooks...)
	mustWrite(t, f.codexTrust,
		"[hooks.state."+tomlQuote(hooks[0].Key)+"]\ntrusted_hash = \"sha256:stale\"\n")

	_, err := f.run(t, Prepare, nil)
	wantErrContaining(t, err, "changed since it was trusted")
	wantErrContaining(t, err, hooks[0].Key)
	if strings.Contains(f.trustStore(t), hooks[0].CurrentHash) {
		t.Fatal("prepare rewrote a modified hook's recorded hash")
	}
}

func TestPrepare_RefusesAHookCodexStillReportsAsUntrusted(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	hooks := f.hooksFor(t, home)
	f.setCodexHooks(t, hooks...)
	// The entry exists but codex still does not consider it trusted, so
	// skipping it would report success while the wedge remains.
	f.forceCodexHookStatus(t, "untrusted")
	mustWrite(t, f.codexTrust,
		"[hooks.state."+tomlQuote(hooks[0].Key)+"]\ntrusted_hash = \"sha256:wrong\"\n")

	_, err := f.run(t, Prepare, nil)
	wantErrContaining(t, err, "recorded trust entry that codex still reports as")
	wantErrContaining(t, err, hooks[0].Key)
}

// F2: an unexpected or empty hooks/list result must fail closed.
func TestPrepare_FailsClosedWhenCodexReportsNoHooksForAHookedDirectory(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	f.setCodexHooks(t) // codex answers with an empty hook list

	_, err := f.run(t, Prepare, nil)
	wantErrContaining(t, err, "reported no hooks for")
	wantErrContaining(t, err, mustCanonical(t, home))
}

func TestDecodeHooksList_RejectsUnexpectedShapes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"empty payload", ``, "no result payload"},
		{"missing data", `{"items":[]}`, "missing its data array"},
		{"null data", `{"data":null}`, "missing its data array"},
		{"entry missing hooks", `{"data":[{"cwd":"/a"}]}`, "missing cwd or hooks"},
		{"entry missing cwd", `{"data":[{"hooks":[]}]}`, "missing cwd or hooks"},
		{"hook without key", `{"data":[{"cwd":"/a","hooks":[{"key":"","currentHash":"sha256:x","trustStatus":"untrusted"}]}]}`, "no key or hash"},
		{"hook without hash", `{"data":[{"cwd":"/a","hooks":[{"key":"/a:stop:0:0","currentHash":"","trustStatus":"untrusted"}]}]}`, "no key or hash"},
		{"unknown status", `{"data":[{"cwd":"/a","hooks":[{"key":"/a:stop:0:0","currentHash":"sha256:x","trustStatus":"weird"}]}]}`, "unknown trustStatus"},
		{"not an object", `["data"]`, "unrecognized shape"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeHooksList([]byte(tc.raw))
			wantErrContaining(t, err, tc.want)
		})
	}
}

func TestDecodeHooksList_AcceptsTheRealShape(t *testing.T) {
	raw := `{"data":[{"cwd":"/a","hooks":[
      {"key":"/a/.codex/hooks.json:stop:0:0","currentHash":"sha256:x","trustStatus":"untrusted","enabled":true,"eventName":"stop"}
    ],"warnings":[],"errors":[]}]}`
	hooks, err := decodeHooksList([]byte(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("got %d hooks, want 1", len(hooks))
	}
	if hooks[0].Key != "/a/.codex/hooks.json:stop:0:0" || hooks[0].CurrentHash != "sha256:x" {
		t.Fatalf("decoded %+v", hooks[0])
	}
}

// F4: trust must never be granted to a hook outside the target directories.
func TestPrepare_IgnoresHookIdentitiesOutsideTheSessionDirectories(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	foreign := codexHook{
		Key:         "/Users/someone/.codex/hooks.json:stop:0:0",
		CurrentHash: "sha256:foreign",
		TrustStatus: "untrusted",
	}
	f.setCodexHooks(t, append(f.hooksFor(t, home), foreign)...)

	opts := f.options()
	var stdout, stderr strings.Builder
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	if err := Prepare(opts); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if f.mustReadStore(t).hookRecorded(foreign.Key) {
		t.Fatal("prepare granted trust to a hook outside the Gas City session directories")
	}
	if !strings.Contains(stderr.String(), "ignoring codex hook outside") {
		t.Errorf("stderr %q does not report the ignored foreign hook", stderr.String())
	}
}

// F5: check must be a pure local read - no codex subprocess at all.
func TestCheck_RunsNoCodexSubprocess(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	f.setCodexHooks(t, f.hooksFor(t, home)...)
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	codexLog := filepath.Join(filepath.Dir(f.codexHome), "codex.log")
	if err := os.Remove(codexLog); err != nil {
		t.Fatalf("reset codex log: %v", err)
	}
	before := mustReadFile(t, f.codexTrust)

	if _, err := f.run(t, Check, nil); err != nil {
		t.Fatalf("check: %v", err)
	}
	if _, err := os.Stat(codexLog); !os.IsNotExist(err) {
		t.Fatalf("check invoked the codex binary; log exists: %v", err)
	}
	if got := mustReadFile(t, f.codexTrust); got != before {
		t.Fatal("check wrote to the codex trust store; it must be read-only")
	}
}

func TestCheck_FailsWhenCodexWorkspaceTrustIsMissing(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	f.setCodexHooks(t, f.hooksFor(t, home)...)
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	store := f.trustStore(t)
	drop := "[projects." + tomlQuote(mustCanonical(t, home)) + "]\ntrust_level = \"trusted\"\n"
	if !strings.Contains(store, drop) {
		t.Fatalf("prepare did not write the expected project table:\n%s", store)
	}
	mustWrite(t, f.codexTrust, strings.Replace(store, drop, "", 1))

	_, err := f.run(t, Check, nil)
	wantErrContaining(t, err, "codex workspace trust is not pre-seeded")
	wantErrContaining(t, err, mustCanonical(t, home))
	wantErrContaining(t, err, "blocks on the interactive trust dialog")
}

func TestCheck_FailsWhenCodexHookTrustIsMissing(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	hooks := f.hooksFor(t, home)
	f.setCodexHooks(t, hooks...)
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	store := f.trustStore(t)
	hookTable := "[hooks.state." + tomlQuote(hooks[0].Key) + "]\ntrusted_hash = " + tomlQuote(hooks[0].CurrentHash) + "\n"
	if !strings.Contains(store, hookTable) {
		t.Fatalf("prepare did not write the expected hook table:\n%s", store)
	}
	mustWrite(t, f.codexTrust, strings.ReplaceAll(store, hookTable, ""))

	_, err := f.run(t, Check, nil)
	wantErrContaining(t, err, "has no recorded trust entry")
	wantErrContaining(t, err, hooks[0].Key)
}

// R3-F4 / F6: the residual timing gap must be reported by identity. A stale
// home left behind by a removed agent makes the COUNTS agree while a real
// agent is still missing, so counting would report a false all-clear.
func TestPrepare_NamesConfiguredAgentsWithNoSessionHomeDespiteAStaleHome(t *testing.T) {
	f := newFixture(t)
	f.setAgents(t,
		map[string]any{"name": "mayor", "qualified_name": "mayor", "suspended": false},
		map[string]any{"name": "implementer", "qualified_name": "implementer", "suspended": false},
		map[string]any{"name": "retired", "qualified_name": "retired", "suspended": true},
	)
	mayor := f.addSessionHome(t, "mayor")
	// A home for an agent the city no longer configures: counts now match
	// (2 configured, 2 homes) while `implementer` still has none.
	stale := f.addSessionHome(t, "decommissioned")
	hooks := append(f.hooksFor(t, mayor), f.hooksFor(t, stale)...)
	f.setCodexHooks(t, hooks...)

	opts := f.options()
	var stdout, stderr strings.Builder
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	if err := Prepare(opts); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if !strings.Contains(stderr.String(), "implementer") {
		t.Fatalf("stderr %q does not name the agent with no session home", stderr.String())
	}
	if strings.Contains(stderr.String(), "mayor") {
		t.Errorf("stderr %q names an agent that does have a home", stderr.String())
	}
	if strings.Contains(stderr.String(), "retired") {
		t.Errorf("stderr %q names a suspended agent", stderr.String())
	}
	if !strings.Contains(stderr.String(), "re-run `ao gc prepare`") {
		t.Errorf("stderr %q does not tell the operator what to do", stderr.String())
	}
}

func TestPrepare_MatchesNestedSessionHomesByQualifiedName(t *testing.T) {
	f := newFixture(t)
	f.setAgents(t, map[string]any{"name": "witness", "qualified_name": "agentops/witness", "suspended": false})
	home := f.addSessionHome(t, filepath.Join("agentops", "witness"))
	f.setCodexHooks(t, f.hooksFor(t, home)...)

	opts := f.options()
	var stdout, stderr strings.Builder
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	if err := Prepare(opts); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if strings.Contains(stderr.String(), "no session home yet") {
		t.Fatalf("stderr %q warns even though the nested home exists", stderr.String())
	}
}

func TestPrepare_DoesNotWarnWhenEverySessionHomeExists(t *testing.T) {
	f := newFixture(t)
	f.setAgents(t, map[string]any{"name": "mayor", "qualified_name": "mayor", "suspended": false})
	home := f.addSessionHome(t, "mayor")
	f.setCodexHooks(t, f.hooksFor(t, home)...)

	opts := f.options()
	var stdout, stderr strings.Builder
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	if err := Prepare(opts); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if strings.Contains(stderr.String(), "no session home yet") {
		t.Fatalf("stderr %q warns even though every home exists", stderr.String())
	}
}

func TestPrepare_SeedsWorkspaceTrustWithoutACodexBinary(t *testing.T) {
	f := newFixture(t)

	opts := f.options()
	opts.CodexBin = ""
	var stdout, stderr strings.Builder
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	// A PATH holding the fixture binaries and the system utilities the fake gc
	// script needs, but deliberately no codex.
	t.Setenv("PATH", filepath.Dir(f.gcBin)+":/usr/bin:/bin")
	if err := Prepare(opts); err != nil {
		t.Fatalf("prepare without codex: %v", err)
	}
	if !strings.Contains(stderr.String(), "no codex binary on PATH") {
		t.Fatalf("stderr %q lacks the missing-codex warning", stderr.String())
	}
	if !f.mustReadStore(t).workspaceTrusted(mustCanonical(t, f.city)) {
		t.Error("workspace trust was not seeded without codex")
	}
}

func TestPrepare_RefusesAnUnusableExplicitCodexBinary(t *testing.T) {
	f := newFixture(t)
	_, err := f.run(t, Prepare, func(opts *Options) {
		opts.CodexBin = filepath.Join(f.city, "no-such-codex")
	})
	wantErrContaining(t, err, "codex binary is not executable")
	if _, statErr := os.Lstat(f.codexTrust); !os.IsNotExist(statErr) {
		t.Fatalf("prepare wrote the trust store before refusing the bad binary: %v", statErr)
	}
}

func TestCodexTrustTargets_DiscoversSessionDirsWithoutDescendingIntoThem(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	worktree := f.addRigWorktree(t, "smoke", "polecat")
	// A directory nested inside a discovered session dir must not be reported
	// separately: the walk stops at the session boundary.
	mustWrite(t, filepath.Join(worktree, "vendor", "dep", ".git"), "gitdir: /elsewhere\n")
	// A plain directory with no session marker is not a target.
	mustMkdir(t, filepath.Join(f.city, ".gc", "agents", "scratch"))

	o, err := resolve(f.options(), true)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	targets := o.codexTrustTargets()

	want := []string{
		mustCanonical(t, f.city),
		mustCanonical(t, f.rig),
		mustCanonical(t, home),
		mustCanonical(t, worktree),
	}
	if len(targets) != len(want) {
		t.Fatalf("targets = %v, want exactly %v", targets, want)
	}
	for _, dir := range want {
		found := false
		for _, target := range targets {
			if target == dir {
				found = true
			}
		}
		if !found {
			t.Errorf("target %s is missing from %v", dir, targets)
		}
	}
}

// derivedHookKeys is what `check` verifies against, so its index math must
// match the shape Codex reports. These expectations were captured from
// codex-cli 0.145.0 driven against this exact document.
func TestDerivedHookKeys_MatchesCodexIndexing(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".codex", "hooks.json"), realHooksDoc)
	keys, err := derivedHookKeys(dir)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	path := filepath.Join(dir, ".codex", "hooks.json")
	want := []string{
		path + ":pre_compact:0:0",
		path + ":session_start:0:0",
		path + ":session_start:1:0",
		path + ":user_prompt_submit:0:0",
		path + ":user_prompt_submit:0:1",
	}
	if len(keys) != len(want) {
		t.Fatalf("keys = %q, want %q", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %q, want %q", keys, want)
		}
	}
}

func TestDerivedHookKeys_MissingFileIsNotAnErrorAndMalformedIs(t *testing.T) {
	keys, err := derivedHookKeys(t.TempDir())
	if err != nil {
		t.Fatalf("missing hooks file: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("keys = %q, want none", keys)
	}

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".codex", "hooks.json"), "{not json")
	if _, err := derivedHookKeys(dir); err == nil {
		t.Fatal("a malformed hooks file must be an error, not an empty key set")
	}
}

func TestSnakeCase(t *testing.T) {
	tests := map[string]string{
		"SessionStart":     "session_start",
		"UserPromptSubmit": "user_prompt_submit",
		"Stop":             "stop",
		"PreToolUse":       "pre_tool_use",
		"PostCompact":      "post_compact",
	}
	for input, want := range tests {
		if got := snakeCase(input); got != want {
			t.Errorf("snakeCase(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHookKeySourcePath(t *testing.T) {
	tests := []struct {
		key   string
		want  string
		wantK bool
	}{
		{"/a/.codex/hooks.json:stop:0:0", "/a/.codex/hooks.json", true},
		{"/a/b c/.codex/hooks.json:user_prompt_submit:1:2", "/a/b c/.codex/hooks.json", true},
		{"nocolons", "", false},
		{"only:two", "", false},
		{":0:0:0", "", false},
	}
	for _, tc := range tests {
		got, ok := hookKeySourcePath(tc.key)
		if ok != tc.wantK || got != tc.want {
			t.Errorf("hookKeySourcePath(%q) = (%q,%v), want (%q,%v)", tc.key, got, ok, tc.want, tc.wantK)
		}
	}
}

// F3: detection must use a real TOML parse, not a header scan, so valid
// spellings a scanner would miss cannot cause a duplicate-table append.
func TestReadCodexTrustStore_UnderstandsValidTOMLSpellings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	mustWrite(t, path, strings.Join([]string{
		`[projects."/a"] # trailing comment after the header`,
		`trust_level = "trusted"`,
		`[projects.'/b/literal']`,
		`trust_level = "untrusted"`,
		`[ projects . "/c/spaced" ]`,
		`trust_level = "trusted"`,
		`[hooks.state."/a/.codex/hooks.json:stop:0:0"]  `,
		`trusted_hash = "sha256:aaa"`,
		`enabled = false`,
		``,
	}, "\n"))

	store, err := readCodexTrustStore(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !store.workspaceTrusted("/a") {
		t.Error("/a (trailing comment after header) not understood")
	}
	if store.workspaceTrusted("/b/literal") || store.workspaceLevel("/b/literal") != "untrusted" {
		t.Errorf("/b/literal (single-quoted key) level = %q, want untrusted", store.workspaceLevel("/b/literal"))
	}
	if !store.workspaceTrusted("/c/spaced") {
		t.Error("/c/spaced (spaced dotted key) not understood")
	}
	key := "/a/.codex/hooks.json:stop:0:0"
	entry, ok := store.Hooks.State[key]
	if !ok || entry.TrustedHash != "sha256:aaa" {
		t.Errorf("hook entry not understood: %+v", entry)
	}
	if entry.Enabled == nil || *entry.Enabled {
		t.Error("enabled = false not decoded")
	}
	// Decoded, but a disabled hook is not a satisfied one.
	if store.hookTrustSatisfied(key) {
		t.Error("a hook recorded enabled = false must not count as satisfied")
	}
	if got := store.hookTrustDefect(key); !strings.Contains(got, "enabled = false") {
		t.Errorf("defect = %q, want it to name enabled = false", got)
	}
}

func TestReadCodexTrustStore_MissingStoreIsEmptyAndBrokenStoreIsAnError(t *testing.T) {
	dir := t.TempDir()
	store, err := readCodexTrustStore(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("missing store: %v", err)
	}
	if len(store.Projects) != 0 || len(store.Hooks.State) != 0 {
		t.Fatalf("missing store reported %d projects and %d hooks, want 0 and 0", len(store.Projects), len(store.Hooks.State))
	}

	broken := filepath.Join(dir, "broken.toml")
	mustWrite(t, broken, "[projects.\"/a\"]\ntrust_level = \"trusted\"\n[projects.\"/a\"]\ntrust_level = \"trusted\"\n")
	if _, err := readCodexTrustStore(broken); err == nil {
		t.Fatal("a duplicate table must be an error, not an empty store")
	}
}

func TestAppendCodexConfig_KeepsTheStoreParsableWithoutATrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	mustWrite(t, path, `[projects."/a"]`+"\ntrust_level = \"trusted\"")

	block := "\n[projects." + tomlQuote("/b") + "]\ntrust_level = \"trusted\"\n"
	if err := appendCodexConfig(path, block); err != nil {
		t.Fatalf("append: %v", err)
	}
	store, err := readCodexTrustStore(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	for _, want := range []string{"/a", "/b"} {
		if !store.workspaceTrusted(want) {
			t.Errorf("project %s is not trusted after the append; store:\n%s", want, mustReadFile(t, path))
		}
	}
}

func TestTomlQuote_EscapesPathologicalPaths(t *testing.T) {
	tests := map[string]string{
		`/a/b`:  `"/a/b"`,
		`/a"b`:  `"/a\"b"`,
		`/a\b`:  `"/a\\b"`,
		"/a\tb": `"/a\tb"`,
		"/a\nb": `"/a\nb"`,
		`/a'b`:  `"/a'b"`,
		`/a#b`:  `"/a#b"`,
	}
	for input, want := range tests {
		if got := tomlQuote(input); got != want {
			t.Errorf("tomlQuote(%q) = %s, want %s", input, got, want)
		}
	}
	// A quoted pathological path must survive a real parse round trip.
	path := filepath.Join(t.TempDir(), "config.toml")
	weird := `/a"b\c#d`
	if err := appendCodexConfig(path, "\n[projects."+tomlQuote(weird)+"]\ntrust_level = \"trusted\"\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	store, err := readCodexTrustStore(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !store.workspaceTrusted(weird) {
		t.Fatalf("pathological path did not round trip; store:\n%s", mustReadFile(t, path))
	}
}

// Fixture fidelity: the scripted codex answer must be exactly what the
// production decoder accepts, so a passing test cannot rest on a shape codex
// would never emit.
func TestFixtureHooksAnswerMatchesProductionDecoder(t *testing.T) {
	f := newFixture(t)
	home := f.addSessionHome(t, "implementer")
	hooks := f.hooksFor(t, home)
	f.setCodexHooks(t, hooks...)

	// Drive the real client against the fake server: the answer must survive
	// the production JSON-RPC read and the production shape validation.
	decoded, err := codexHooksList(f.codexBin, f.codexHome, []string{mustCanonical(t, home)})
	if err != nil {
		t.Fatalf("production client rejects the fixture answer: %v", err)
	}
	if len(decoded) != len(hooks) {
		t.Fatalf("decoded %d hooks, want %d", len(decoded), len(hooks))
	}
	for index, hook := range decoded {
		if hook.Key != hooks[index].Key || hook.CurrentHash != hooks[index].CurrentHash {
			t.Fatalf("decoded[%d] = %+v, want key %q hash %q", index, hook, hooks[index].Key, hooks[index].CurrentHash)
		}
		if hook.TrustStatus != "untrusted" {
			t.Errorf("decoded[%d] status = %q, want untrusted before any seeding", index, hook.TrustStatus)
		}
	}

	// After seeding, the same fake must report them trusted - the property
	// idempotency depends on.
	if _, err := f.run(t, Prepare, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	reseeded, err := codexHooksList(f.codexBin, f.codexHome, []string{mustCanonical(t, home)})
	if err != nil {
		t.Fatalf("second list: %v", err)
	}
	for _, hook := range reseeded {
		if hook.TrustStatus != "trusted" {
			t.Errorf("hook %s status = %q after prepare, want trusted", hook.Key, hook.TrustStatus)
		}
	}
}
