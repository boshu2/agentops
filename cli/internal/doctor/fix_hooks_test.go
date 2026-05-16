package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- test harness -----------------------------------------------------------

// hookTestEnv builds an isolated repo + temp HOME for a hooks doctor test. It
// writes the repo's hooks/hooks.json from the embedded blob so the coverage
// contract resolves cleanly (no fm-hooks-contract-fallback) unless a test
// deliberately removes it.
func hookTestEnv(t *testing.T) (repo, home string, env *DetectEnv) {
	t.Helper()
	repo = t.TempDir()
	home = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "hooks", "hooks.json"), embeddedHooksJSONForTest(t), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o750); err != nil {
		t.Fatal(err)
	}
	env = &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home, Logger: os.Stderr}
	return repo, home, env
}

// embeddedHooksJSONForTest returns the embedded hooks.json blob, failing the
// test if it is empty (the doctor relies on it as a manifest source).
func embeddedHooksJSONForTest(t *testing.T) []byte {
	t.Helper()
	data, _, err := hookFindManifest(&DetectEnv{RepoRoot: t.TempDir(), HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("no embedded hooks manifest available: %v", err)
	}
	return data
}

// hookFixCtx builds a real MutateContext (live RunArtifact, lock manager,
// actions.jsonl) scoped to the given fixer id.
func hookFixCtx(t *testing.T, repo, home, fixerID string) (*MutateContext, *RunArtifact, *os.File) {
	t.Helper()
	ra, err := NewRunArtifact(repo, "sha", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	caps := NewCapabilities("2.0.0-test")
	locks := NewLockManager(filepath.Join(repo, ".doctor", "locks"))
	af, err := ra.OpenActionsFile()
	if err != nil {
		t.Fatal(err)
	}
	mctx := NewMutateContext(ra, caps, home, locks, af, false).WithFixer(fixerID)
	return mctx, ra, af
}

// writeSettings writes raw bytes to ~/.claude/settings.json in the temp HOME.
func writeSettings(t *testing.T, home string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), content, 0o600); err != nil {
		t.Fatal(err)
	}
}

// settingsHooksKeys returns the sorted-by-presence event keys in the hooks
// object of ~/.claude/settings.json.
func settingsHooksKeys(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("settings.json not valid JSON after fix: %v", err)
	}
	hooks, _ := obj["hooks"].(map[string]any)
	return hooks
}

// --- fm-hooks-coverage-zero --------------------------------------------------

// TestHooksCoverageZero_DetectAndFix verifies the coverage-zero detector fires
// on a hookless settings.json and the fixer materializes full coverage through
// Mutate, preserving non-hooks settings, with a backup and an actions line.
func TestHooksCoverageZero_DetectAndFix(t *testing.T) {
	repo, home, env := hookTestEnv(t)
	writeSettings(t, home, []byte(`{"model":"claude-opus-4-7","permissions":{"allow":["Bash"]}}`))

	det := hooksCoverageZeroDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 || findings[0].ID != "fm-hooks-coverage-zero" {
		t.Fatalf("findings = %+v, want one fm-hooks-coverage-zero", findings)
	}
	if !findings[0].Remediation.AutoFixable {
		t.Fatal("coverage-zero must be auto-fixable")
	}

	mctx, ra, af := hookFixCtx(t, repo, home, "fm-hooks-coverage-zero")
	res, err := hooksCoverageZeroFixer{}.Fix(mctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 1 {
		t.Fatalf("FixResult = %+v, want Fixed=true ActionsTaken=1", res)
	}

	// settings.json gained the hooks object with the full contract.
	hooks := settingsHooksKeys(t, home)
	contract := hookResolveContract(env)
	if len(hooks) != len(contract.ActiveEvents) {
		t.Fatalf("installed hook events = %d, want %d", len(hooks), len(contract.ActiveEvents))
	}
	// Non-hooks settings preserved exactly.
	data, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	var obj map[string]any
	_ = json.Unmarshal(data, &obj)
	if obj["model"] != "claude-opus-4-7" {
		t.Fatalf("model = %v, want claude-opus-4-7 (non-hooks settings clobbered)", obj["model"])
	}

	// Detector now clean.
	again, err := det.Detect(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("post-fix Detect = %+v, want empty", again)
	}

	// Backup exists + actions.jsonl has exactly one WriteFile line.
	_ = af.Close()
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Op != "WriteFile" || !recs[0].OK {
		t.Fatalf("actions = %+v, want one OK WriteFile", recs)
	}
	backup := filepath.Join(ra.BackupsDir(), recs[0].Path)
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup missing: %v", err)
	}

	// Undo restores the original hookless settings byte-identically.
	ur, err := Undo(repo, ra.RunID, true, false)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if ur.Restored != 1 {
		t.Fatalf("Undo restored = %d, want 1", ur.Restored)
	}
	restored, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if string(restored) != `{"model":"claude-opus-4-7","permissions":{"allow":["Bash"]}}` {
		t.Fatalf("after undo: %q", restored)
	}
}

// TestHooksCoverageZero_FileAbsent verifies the detector's file-absent branch.
func TestHooksCoverageZero_FileAbsent(t *testing.T) {
	_, _, env := hookTestEnv(t)
	// No settings.json written at all.
	findings, err := hooksCoverageZeroDetector{}.Detect(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].ID != "fm-hooks-coverage-zero" {
		t.Fatalf("findings = %+v, want one fm-hooks-coverage-zero", findings)
	}
}

// TestHooksCoverageZero_CedesToMalformed verifies coverage-zero stays silent
// when settings.json is unparseable (fm-hooks-settings-malformed owns it).
func TestHooksCoverageZero_CedesToMalformed(t *testing.T) {
	_, home, env := hookTestEnv(t)
	writeSettings(t, home, []byte(`{"model":"x",}`)) // trailing comma
	findings, err := hooksCoverageZeroDetector{}.Detect(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("coverage-zero should cede to settings-malformed, got %+v", findings)
	}
}

// TestHooksCoverageZero_FixerRefusesOnMalformed verifies the fixer refuses
// (error) when settings.json is unparseable JSON.
func TestHooksCoverageZero_FixerRefusesOnMalformed(t *testing.T) {
	repo, home, env := hookTestEnv(t)
	writeSettings(t, home, []byte(`{"hooks":{},}`))
	mctx, _, af := hookFixCtx(t, repo, home, "fm-hooks-coverage-zero")
	defer func() { _ = af.Close() }()
	_, err := hooksCoverageZeroFixer{}.Fix(mctx, env, nil)
	if err == nil {
		t.Fatal("expected refusal on unparseable settings.json")
	}
}

// --- fm-hooks-coverage-partial -----------------------------------------------

// TestHooksCoveragePartial_DetectAndFix verifies the partial detector fires on
// a 1-of-N install and the fixer fills the remaining contract events while
// keeping the originally-installed SessionStart group byte-identical.
func TestHooksCoveragePartial_DetectAndFix(t *testing.T) {
	repo, home, env := hookTestEnv(t)
	// Only SessionStart wired, with an ao-managed command.
	partial := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"ao inject"}]}]}}`
	writeSettings(t, home, []byte(partial))

	contract := hookResolveContract(env)
	if len(contract.ActiveEvents) < 2 {
		t.Skip("contract has fewer than 2 events; partial coverage not exercisable")
	}

	det := hooksCoveragePartialDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].ID != "fm-hooks-coverage-partial" {
		t.Fatalf("findings = %+v, want one fm-hooks-coverage-partial", findings)
	}

	mctx, ra, af := hookFixCtx(t, repo, home, "fm-hooks-coverage-partial")
	res, err := hooksCoveragePartialFixer{}.Fix(mctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 1 {
		t.Fatalf("FixResult = %+v, want Fixed=true ActionsTaken=1", res)
	}

	hooks := settingsHooksKeys(t, home)
	if len(hooks) != len(contract.ActiveEvents) {
		t.Fatalf("installed events = %d, want %d", len(hooks), len(contract.ActiveEvents))
	}
	// SessionStart still ao-managed.
	if !hookGroupContainsAoForTest(hooks, "SessionStart") {
		t.Fatal("SessionStart lost its ao command after fix")
	}

	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix Detect = %+v, want empty", again)
	}

	_ = af.Close()
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 1 {
		t.Fatalf("actions = %d, want 1", len(recs))
	}
	if _, err := os.Stat(filepath.Join(ra.BackupsDir(), recs[0].Path)); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}

// TestHooksCoveragePartial_IdempotentSecondRun verifies a second fix on an
// already-full install issues zero Mutate calls.
func TestHooksCoveragePartial_IdempotentSecondRun(t *testing.T) {
	repo, home, env := hookTestEnv(t)
	writeSettings(t, home, []byte(`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"ao inject"}]}]}}`))

	mctx1, _, af1 := hookFixCtx(t, repo, home, "fm-hooks-coverage-partial")
	if _, err := (hooksCoveragePartialFixer{}).Fix(mctx1, env, nil); err != nil {
		t.Fatalf("first fix: %v", err)
	}
	_ = af1.Close()

	mctx2, _, af2 := hookFixCtx(t, repo, home, "fm-hooks-coverage-partial")
	defer func() { _ = af2.Close() }()
	res, err := hooksCoveragePartialFixer{}.Fix(mctx2, env, nil)
	if err != nil {
		t.Fatalf("second fix: %v", err)
	}
	if res.ActionsTaken != 0 {
		t.Fatalf("second-run ActionsTaken = %d, want 0 (idempotent)", res.ActionsTaken)
	}
	if !res.Fixed {
		t.Fatal("idempotent no-op should still report Fixed=true")
	}
}

// --- fm-hooks-contract-fallback ----------------------------------------------

// TestHooksContractFallback_DetectAndFix verifies the fallback detector fires
// when no manifest is on disk and the fixer materializes ~/.agentops/hooks.json.
func TestHooksContractFallback_DetectAndFix(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o750); err != nil {
		t.Fatal(err)
	}
	// No repo hooks/hooks.json and no ~/.agentops/hooks.json: the only manifest
	// source is the embedded blob, so the contract resolves via embedded (a
	// fallback) and the detector must fire.
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home, Logger: os.Stderr}

	det := hooksContractFallbackDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatal(err)
	}
	// The embedded blob yields a clean contract, so detection depends on the
	// blob having active events. Verify by checking the resolved contract.
	contract := hookResolveContract(env)
	if contract.FallbackReason == "" {
		// Embedded blob resolved cleanly without fallback: force the explicit
		// fallback path by writing an empty-event manifest in the repo.
		emptyManifest := `{"hooks":{}}`
		if err := os.MkdirAll(filepath.Join(repo, "hooks"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, "hooks", "hooks.json"), []byte(emptyManifest), 0o644); err != nil {
			t.Fatal(err)
		}
		findings, err = det.Detect(env)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(findings) != 1 || findings[0].ID != "fm-hooks-contract-fallback" {
		t.Fatalf("findings = %+v, want one fm-hooks-contract-fallback", findings)
	}

	// Provide a valid repo manifest so the fixer has a source to materialize.
	if err := os.MkdirAll(filepath.Join(repo, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "hooks", "hooks.json"), embeddedHooksJSONForTest(t), 0o644); err != nil {
		t.Fatal(err)
	}

	mctx, ra, af := hookFixCtx(t, repo, home, "fm-hooks-contract-fallback")
	res, err := hooksContractFallbackFixer{}.Fix(mctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 1 {
		t.Fatalf("FixResult = %+v, want Fixed=true ActionsTaken=1", res)
	}

	// ~/.agentops/hooks.json now exists and parses with active events.
	homeManifest := filepath.Join(home, ".agentops", "hooks.json")
	data, err := os.ReadFile(homeManifest)
	if err != nil {
		t.Fatalf("home manifest not materialized: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("materialized manifest is not valid JSON")
	}

	_ = af.Close()
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 1 || recs[0].Op != "WriteFile" {
		t.Fatalf("actions = %+v, want one WriteFile", recs)
	}
}

// TestHooksContractFallback_RefusesEmptyManifest verifies the fixer refuses
// when the only manifest source declares zero active events.
func TestHooksContractFallback_RefusesEmptyManifest(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Repo manifest has zero active events; embedded fallback is bypassed
	// because the repo source is found first.
	if err := os.WriteFile(filepath.Join(repo, "hooks", "hooks.json"), []byte(`{"hooks":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home, Logger: os.Stderr}
	mctx, _, af := hookFixCtx(t, repo, home, "fm-hooks-contract-fallback")
	defer func() { _ = af.Close() }()
	_, err := hooksContractFallbackFixer{}.Fix(mctx, env, nil)
	if err == nil {
		t.Fatal("expected refusal on zero-active-event manifest source")
	}
}

// --- fm-hooks-non-ao-shadow --------------------------------------------------

// TestHooksNonAoShadow_DetectAndGatedFix verifies the shadow detector fires on
// a foreign SessionStart, the fixer is advertised non-auto-fixable, refuses
// without the confirm flag, and additively merges (preserving the foreign
// command) when confirmed.
func TestHooksNonAoShadow_DetectAndGatedFix(t *testing.T) {
	repo, home, env := hookTestEnv(t)
	foreign := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"/usr/local/bin/some-other-tool init"}]}]}}`
	writeSettings(t, home, []byte(foreign))

	det := hooksNonAoShadowDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].ID != "fm-hooks-non-ao-shadow" {
		t.Fatalf("findings = %+v, want one fm-hooks-non-ao-shadow", findings)
	}
	if findings[0].Remediation.AutoFixable {
		t.Fatal("non-ao-shadow must NOT advertise auto_fixable (gated)")
	}

	fixer := hooksNonAoShadowFixer{}
	if fixer.AutoFixable() {
		t.Fatal("hooksNonAoShadowFixer.AutoFixable() must be false (gated)")
	}

	// Without the confirm flag the fixer refuses and writes nothing.
	os.Unsetenv(hookEnvConfirmForeignMerge)
	mctx, _, af := hookFixCtx(t, repo, home, "fm-hooks-non-ao-shadow")
	_, err = fixer.Fix(mctx, env, findings)
	if err == nil {
		t.Fatal("expected refusal without --confirm-foreign-merge")
	}
	_ = af.Close()
	after, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if string(after) != foreign {
		t.Fatalf("settings.json mutated by a refused fix: %q", after)
	}

	// With the confirm flag the additive merge proceeds.
	t.Setenv(hookEnvConfirmForeignMerge, "1")
	mctx2, ra2, af2 := hookFixCtx(t, repo, home, "fm-hooks-non-ao-shadow")
	res, err := fixer.Fix(mctx2, env, findings)
	if err != nil {
		t.Fatalf("confirmed Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 1 {
		t.Fatalf("FixResult = %+v, want Fixed=true ActionsTaken=1", res)
	}

	// The foreign command still appears in SessionStart.
	hooks := settingsHooksKeys(t, home)
	cmds := hookCollectGroupCommands(hooks, "SessionStart")
	if !hookContainsString(cmds, "/usr/local/bin/some-other-tool init") {
		t.Fatalf("foreign SessionStart command dropped; commands = %v", cmds)
	}
	// And SessionStart is now ao-managed.
	if !hookGroupContainsAoForTest(hooks, "SessionStart") {
		t.Fatal("SessionStart not ao-managed after confirmed merge")
	}

	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix Detect = %+v, want empty", again)
	}

	_ = af2.Close()
	recs, _ := readActions(ra2.ActionsPath())
	if len(recs) != 1 {
		t.Fatalf("actions = %d, want 1", len(recs))
	}
}

// --- fm-hooks-settings-malformed ---------------------------------------------

// TestHooksSettingsMalformed_ClassifiesAndRefuses verifies the detector
// precisely classifies each corruption kind and the fixer always refuses
// (detect-only).
func TestHooksSettingsMalformed_ClassifiesAndRefuses(t *testing.T) {
	cases := []struct {
		name    string
		content string
		kind    string
	}{
		{"trailing-comma", `{"model":"claude-opus-4-7","hooks":{},}`, "syntax-error"},
		{"merge-conflict", "<<<<<<< HEAD\n{\"hooks\":{}}\n=======\n{\"hooks\":{}}\n>>>>>>> branch\n", "git-merge-conflict-markers"},
		{"wrong-hooks-type", `{"hooks":[]}`, "wrong-hooks-type"},
		{"truncated-write", `{"model":"claude-opus-4-7","hooks":{"SessionSt`, "truncated-write"},
		{"utf8-bom", "\xEF\xBB\xBF{\"model\":\"x\"} trailing-garbage", "utf8-bom-prefix"},
	}
	det := hooksSettingsMalformedDetector{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, home, env := hookTestEnv(t)
			writeSettings(t, home, []byte(tc.content))

			findings, err := det.Detect(env)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 || findings[0].ID != "fm-hooks-settings-malformed" {
				t.Fatalf("findings = %+v, want one fm-hooks-settings-malformed", findings)
			}
			if findings[0].Remediation.AutoFixable {
				t.Fatal("settings-malformed must NOT advertise auto_fixable (detect-only)")
			}
			if got := findings[0].Evidence.Query; got != "corruption="+tc.kind {
				t.Fatalf("corruption classification = %q, want corruption=%s", got, tc.kind)
			}

			// Fixer always refuses.
			mctx, _, af := hookFixCtx(t, repo, home, "fm-hooks-settings-malformed")
			res, ferr := hooksSettingsMalformedFixer{}.Fix(mctx, env, findings)
			_ = af.Close()
			if ferr == nil {
				t.Fatal("settings-malformed fixer must refuse")
			}
			if res.Fixed {
				t.Fatal("settings-malformed fixer must not report Fixed=true")
			}
			// settings.json untouched by the refused fix.
			after, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
			if string(after) != tc.content {
				t.Fatalf("settings.json mutated by a detect-only fixer: %q", after)
			}
		})
	}
}

// TestHooksSettingsMalformed_SilentOnValidJSON verifies the detector stays
// silent for valid JSON with an object-typed (or absent) hooks key.
func TestHooksSettingsMalformed_SilentOnValidJSON(t *testing.T) {
	_, home, env := hookTestEnv(t)
	writeSettings(t, home, []byte(`{"model":"x","hooks":{"SessionStart":[]}}`))
	findings, err := hooksSettingsMalformedDetector{}.Detect(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("valid JSON wrongly flagged malformed: %+v", findings)
	}
}

// --- registration ------------------------------------------------------------

// TestHooksDetectorsAndFixersRegistered verifies all 5 hooks detectors and 5
// fixers are registered and that the auto-fixable / gated split is correct.
func TestHooksDetectorsAndFixersRegistered(t *testing.T) {
	wantDetectors := map[string]bool{
		"fm-hooks-coverage-zero":      false,
		"fm-hooks-coverage-partial":   false,
		"fm-hooks-contract-fallback":  false,
		"fm-hooks-non-ao-shadow":      false,
		"fm-hooks-settings-malformed": false,
	}
	for _, d := range Detectors() {
		if d.Subsystem() != "hooks" {
			continue
		}
		if _, ok := wantDetectors[d.ID()]; ok {
			wantDetectors[d.ID()] = true
		}
	}
	for id, found := range wantDetectors {
		if !found {
			t.Fatalf("hooks detector %s not registered", id)
		}
	}

	autoFixable := map[string]bool{
		"fm-hooks-coverage-zero":     true,
		"fm-hooks-coverage-partial":  true,
		"fm-hooks-contract-fallback": true,
	}
	gatedOrDetectOnly := map[string]bool{
		"fm-hooks-non-ao-shadow":      true,
		"fm-hooks-settings-malformed": true,
	}
	for id := range autoFixable {
		fx := FixerByID(id)
		if fx == nil {
			t.Fatalf("hooks fixer %s not registered", id)
		}
		if !fx.AutoFixable() {
			t.Fatalf("fixer %s must be auto-fixable", id)
		}
	}
	for id := range gatedOrDetectOnly {
		fx := FixerByID(id)
		if fx == nil {
			t.Fatalf("hooks fixer %s not registered", id)
		}
		if fx.AutoFixable() {
			t.Fatalf("fixer %s must be gated / detect-only (AutoFixable=false)", id)
		}
	}
}

// --- small test helpers ------------------------------------------------------

// hookGroupContainsAoForTest reports whether an event group contains an ao
// command. It wraps the bridge helper used by the detectors so tests assert
// the same predicate.
func hookGroupContainsAoForTest(hooksMap map[string]any, event string) bool {
	for _, cmd := range hookCollectGroupCommands(hooksMap, event) {
		if cmd == "ao inject" {
			return true
		}
	}
	// Fall back to the structural predicate for materialized ao commands.
	groups, ok := hooksMap[event].([]any)
	if !ok {
		return false
	}
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		hooks, _ := group["hooks"].([]any)
		for _, h := range hooks {
			hook, _ := h.(map[string]any)
			cmd, _ := hook["command"].(string)
			if hookContainsSubstr(cmd, "ao ") || hookContainsSubstr(cmd, "/.agentops/hooks/") {
				return true
			}
		}
	}
	return false
}

// hookContainsString reports whether s appears in xs.
func hookContainsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// hookContainsSubstr reports whether sub is a substring of s.
func hookContainsSubstr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

// indexOf returns the index of sub in s, or -1.
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
