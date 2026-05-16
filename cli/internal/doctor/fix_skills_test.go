package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// skillsTestCtx builds a real MutateContext rooted at repo with the given home
// dir, plus the RunArtifact, so a test can drive a fixer end-to-end and then
// Undo it. It returns the context, the run artifact, and a close func.
func skillsTestCtx(t *testing.T, repo, home string) (*MutateContext, *RunArtifact, func()) {
	t.Helper()
	ra, err := NewRunArtifact(repo, "testsha", time.Now())
	if err != nil {
		t.Fatalf("NewRunArtifact: %v", err)
	}
	caps := NewCapabilities("2.0.0")
	locks := NewLockManager(filepath.Join(repo, ".doctor", "locks"))
	af, err := ra.OpenActionsFile()
	if err != nil {
		t.Fatalf("OpenActionsFile: %v", err)
	}
	mctx := NewMutateContext(ra, caps, home, locks, af, false)
	return mctx, ra, func() { _ = af.Close() }
}

// writeSkillsFile is a test helper that creates a file (and parents) with content.
func writeSkillsFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- fm-skills-stale-command-refs ------------------------------------------

// TestSkillsStaleCommandRefsFixer verifies the substitution, backup, the
// actions.jsonl line, and that Undo restores the stale reference verbatim.
func TestSkillsStaleCommandRefsFixer(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	skillMD := filepath.Join(repo, "skills", "sample", "SKILL.md")
	original := "# Sample\n\nRun `ao know inject` to load context.\n" +
		"ao know inject -> ao inject (renamed)\n"
	writeSkillsFile(t, skillMD, original)
	docMD := filepath.Join(repo, "docs", "sample.md")
	writeSkillsFile(t, docMD, "Use `ao work rpi` to start.\n")

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	findings, err := skillsStaleCommandRefsDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 || findings[0].ID != "fm-skills-stale-command-refs" {
		t.Fatalf("expected 1 stale-command-refs finding, got %+v", findings)
	}
	if !findings[0].Remediation.AutoFixable {
		t.Fatal("expected stale-command-refs to be auto-fixable")
	}

	mctx, ra, closer := skillsTestCtx(t, repo, home)
	res, err := skillsStaleCommandRefsFixer{}.Fix(mctx.WithFixer("fm-skills-stale-command-refs"), env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed {
		t.Fatal("Fix not marked Fixed")
	}
	if res.ActionsTaken != 2 {
		t.Fatalf("ActionsTaken = %d, want 2", res.ActionsTaken)
	}

	// Substitution applied; arrow rename-doc line untouched.
	got, _ := os.ReadFile(skillMD)
	want := "# Sample\n\nRun `ao inject` to load context.\n" +
		"ao know inject -> ao inject (renamed)\n"
	if string(got) != want {
		t.Fatalf("SKILL.md after fix = %q, want %q", got, want)
	}
	gotDoc, _ := os.ReadFile(docMD)
	if string(gotDoc) != "Use `ao rpi` to start.\n" {
		t.Fatalf("docs/sample.md after fix = %q", gotDoc)
	}

	// Backup exists, byte-identical to original.
	backup := filepath.Join(ra.BackupsDir(), "skills", "sample", "SKILL.md")
	bgot, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bgot) != original {
		t.Fatalf("backup = %q, want %q", bgot, original)
	}

	// actions.jsonl has exactly two lines, correct fixer id + op.
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("actions.jsonl lines = %d, want 2", len(recs))
	}
	for _, r := range recs {
		if r.Op != "WriteFile" || r.FixerID != "fm-skills-stale-command-refs" || !r.OK {
			t.Fatalf("unexpected action record: %+v", r)
		}
	}

	// Detector no longer fires after fix.
	post, err := skillsStaleCommandRefsDetector{}.Detect(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(post) != 0 {
		t.Fatalf("expected no findings after fix, got %+v", post)
	}

	// Undo restores the stale references verbatim.
	closer()
	ur, err := Undo(repo, ra.RunID, true, false)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if ur.Restored != 2 {
		t.Fatalf("Undo restored = %d, want 2", ur.Restored)
	}
	restored, _ := os.ReadFile(skillMD)
	if string(restored) != original {
		t.Fatalf("after undo SKILL.md = %q, want %q", restored, original)
	}
}

// TestSkillsStaleCommandRefsIdempotent verifies a second fix run is a no-op.
func TestSkillsStaleCommandRefsIdempotent(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	writeSkillsFile(t, filepath.Join(repo, "skills", "s", "SKILL.md"), "Run `ao know inject` now.\n")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()
	f := skillsStaleCommandRefsFixer{}
	findings, _ := skillsStaleCommandRefsDetector{}.Detect(env)
	if _, err := f.Fix(mctx.WithFixer(f.ID()), env, findings); err != nil {
		t.Fatalf("first Fix: %v", err)
	}
	res2, err := f.Fix(mctx.WithFixer(f.ID()), env, nil)
	if err != nil {
		t.Fatalf("second Fix: %v", err)
	}
	if res2.ActionsTaken != 0 {
		t.Fatalf("second-run ActionsTaken = %d, want 0", res2.ActionsTaken)
	}
}

// TestSkillsStaleCommandRefsLongestFirst verifies the longest deprecated
// command is matched before a shorter prefix could capture it.
func TestSkillsStaleCommandRefsLongestFirst(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	skillMD := filepath.Join(repo, "skills", "s", "SKILL.md")
	writeSkillsFile(t, skillMD, "Run `ao know batch-feedback` now.\n")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()
	findings, _ := skillsStaleCommandRefsDetector{}.Detect(env)
	f := skillsStaleCommandRefsFixer{}
	if _, err := f.Fix(mctx.WithFixer("fm-skills-stale-command-refs"), env, findings); err != nil {
		t.Fatalf("Fix: %v", err)
	}
	got, _ := os.ReadFile(skillMD)
	if string(got) != "Run `ao batch-feedback` now.\n" {
		t.Fatalf("longest-first substitution wrong: %q", got)
	}
}

// --- fm-skills-missing ------------------------------------------------------

// TestSkillsMissingDetectAndFix verifies the detector fires when all roots are
// empty and the fixer mirrors the repo skills/ tree into ~/.claude/skills.
func TestSkillsMissingDetectAndFix(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	writeSkillsFile(t, filepath.Join(repo, "skills", "rpi", "SKILL.md"), "---\nname: rpi\n---\nbody\n")
	writeSkillsFile(t, filepath.Join(repo, "skills", "evolve", "SKILL.md"), "---\nname: evolve\n---\nbody\n")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	findings, err := skillsMissingDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 || findings[0].ID != "fm-skills-missing" {
		t.Fatalf("expected fm-skills-missing finding, got %+v", findings)
	}
	if !findings[0].Remediation.AutoFixable {
		t.Fatal("expected auto-fixable when repo skills/ source present")
	}

	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsMissingFixer{}.Fix(mctx.WithFixer("fm-skills-missing"), env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 2 {
		t.Fatalf("Fix result Fixed=%t ActionsTaken=%d, want true/2", res.Fixed, res.ActionsTaken)
	}
	rpi := filepath.Join(home, ".claude", "skills", "rpi", "SKILL.md")
	got, err := os.ReadFile(rpi)
	if err != nil {
		t.Fatalf("installed rpi SKILL.md missing: %v", err)
	}
	if string(got) != "---\nname: rpi\n---\nbody\n" {
		t.Fatalf("installed rpi SKILL.md = %q", got)
	}
	// Codex root must NOT have been created.
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Fatal("fixer must not create ~/.codex")
	}
	// Detector no longer fires.
	post, _ := skillsMissingDetector{}.Detect(env)
	if len(post) != 0 {
		t.Fatalf("expected no finding after fix, got %+v", post)
	}
}

// TestSkillsMissingFixerRefusesWithoutSource verifies the fixer refuses when no
// repo skills/ source tree is resolvable.
func TestSkillsMissingFixerRefusesWithoutSource(t *testing.T) {
	repo := t.TempDir() // no skills/ dir
	home := t.TempDir()
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	// Detector still fires, but flagged not auto-fixable.
	findings, err := skillsMissingDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected fm-skills-missing finding, got %+v", findings)
	}
	if findings[0].Remediation.AutoFixable {
		t.Fatal("expected not-auto-fixable when no repo skills/ source")
	}

	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsMissingFixer{}.Fix(mctx.WithFixer("fm-skills-missing"), env, nil)
	if err == nil {
		t.Fatal("expected refusal error when no skills/ source")
	}
	if res.Fixed || res.ActionsTaken != 0 {
		t.Fatalf("refused fix should be Fixed=false ActionsTaken=0, got %+v", res)
	}
}

// TestSkillsMissingNoFindingWhenPopulated verifies a populated install root
// produces no finding.
func TestSkillsMissingNoFindingWhenPopulated(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	writeSkillsFile(t, filepath.Join(home, ".claude", "skills", "rpi", "SKILL.md"), "x\n")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}
	findings, err := skillsMissingDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("populated root produced %d findings", len(findings))
	}
}

// --- fm-skills-hash-drift ---------------------------------------------------

// TestSkillsHashDriftFixer verifies drifted hash fields are rewritten to the
// recomputed values while other JSON keys are preserved.
func TestSkillsHashDriftFixer(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	writeSkillsFile(t, filepath.Join(repo, "skills", "demo", "SKILL.md"), "skill body\n")
	writeSkillsFile(t, filepath.Join(repo, "skills-codex", "demo", "SKILL.md"), "codex body\n")
	zero := "0000000000000000000000000000000000000000000000000000000000000000"
	genPath := filepath.Join(repo, "skills-codex", "demo", ".agentops-generated.json")
	writeSkillsFile(t, genPath, `{"skill":"demo","source_hash":"`+zero+`","generated_hash":"`+zero+`"}`)
	manifestPath := filepath.Join(repo, "skills-codex", ".agentops-manifest.json")
	writeSkillsFile(t, manifestPath, `{"version":1,"codex_override_catalog_hash":"`+zero+`"}`)
	writeSkillsFile(t, filepath.Join(repo, "skills-codex-overrides", "demo", "note.md"), "override\n")

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	findings, err := skillsHashDriftDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 hash-drift finding, got %+v", findings)
	}

	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsHashDriftFixer{}.Fix(mctx.WithFixer("fm-skills-hash-drift"), env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 2 {
		t.Fatalf("Fix Fixed=%t ActionsTaken=%d, want true/2", res.Fixed, res.ActionsTaken)
	}

	// source_hash no longer the all-zero constant; non-hash keys preserved.
	genData, _ := os.ReadFile(genPath)
	if strings.Contains(string(genData), zero) {
		t.Fatalf("generated.json still has zero hash: %s", genData)
	}
	if !strings.Contains(string(genData), `"skill": "demo"`) {
		t.Fatalf("generated.json lost the skill key: %s", genData)
	}
	manData, _ := os.ReadFile(manifestPath)
	if strings.Contains(string(manData), zero) {
		t.Fatalf("manifest still has zero catalog hash: %s", manData)
	}
	if !strings.Contains(string(manData), `"version": 1`) {
		t.Fatalf("manifest lost the version key: %s", manData)
	}

	// Detector no longer fires.
	post, _ := skillsHashDriftDetector{}.Detect(env)
	if len(post) != 0 {
		t.Fatalf("expected no hash-drift finding after fix, got %+v", post)
	}

	// Idempotent second run.
	res2, err := skillsHashDriftFixer{}.Fix(mctx.WithFixer("fm-skills-hash-drift"), env, nil)
	if err != nil {
		t.Fatalf("second Fix: %v", err)
	}
	if res2.ActionsTaken != 0 {
		t.Fatalf("second-run ActionsTaken = %d, want 0", res2.ActionsTaken)
	}
}

// TestSkillsHashDriftNoFindingWhenAligned verifies no finding when recorded
// hashes already equal the recomputed values.
func TestSkillsHashDriftNoFindingWhenAligned(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	writeSkillsFile(t, filepath.Join(repo, "skills", "demo", "SKILL.md"), "skill body\n")
	writeSkillsFile(t, filepath.Join(repo, "skills-codex", "demo", "SKILL.md"), "codex body\n")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	srcH := hashDirRecursive(filepath.Join(repo, "skills", "demo"))
	genDir := filepath.Join(repo, "skills-codex", "demo")
	genH := hashDirRecursive(genDir, ".agentops-generated.json")
	writeSkillsFile(t, filepath.Join(genDir, ".agentops-generated.json"),
		`{"source_hash":"`+srcH+`","generated_hash":"`+genH+`"}`)
	catalogH := hashDirRecursive(filepath.Join(repo, "skills-codex-overrides"))
	writeSkillsFile(t, filepath.Join(repo, "skills-codex", ".agentops-manifest.json"),
		`{"codex_override_catalog_hash":"`+catalogH+`"}`)

	findings, err := skillsHashDriftDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("aligned hashes produced %d findings", len(findings))
	}
}

// --- fm-skills-integrity-hygiene -------------------------------------------

// TestSkillsIntegrityHygieneFixer verifies the partial fixer appends a link for
// an unlinked reference and leaves report-only findings in place.
func TestSkillsIntegrityHygieneFixer(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	// Corrupt skill: full frontmatter EXCEPT tier; an unlinked reference file.
	skillMD := filepath.Join(repo, "skills", "sample", "SKILL.md")
	body := "---\nname: sample\ndescription: a sample skill\n---\n\n# Sample\n\nBody text.\n"
	writeSkillsFile(t, skillMD, body)
	writeSkillsFile(t, filepath.Join(repo, "skills", "sample", "references", "extra.md"), "extra ref\n")

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	findings, err := skillsIntegrityHygieneDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 hygiene finding, got %+v", findings)
	}

	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsIntegrityHygieneFixer{}.Fix(mctx.WithFixer("fm-skills-integrity-hygiene"), env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 1 {
		t.Fatalf("Fix Fixed=%t ActionsTaken=%d, want true/1", res.Fixed, res.ActionsTaken)
	}

	got, _ := os.ReadFile(skillMD)
	if !strings.Contains(string(got), "](references/extra.md)") {
		t.Fatalf("SKILL.md missing appended link: %s", got)
	}
	if !strings.HasPrefix(string(got), body) {
		t.Fatalf("pre-existing SKILL.md content changed: %s", got)
	}
	// reference file untouched.
	rf, _ := os.ReadFile(filepath.Join(repo, "skills", "sample", "references", "extra.md"))
	if string(rf) != "extra ref\n" {
		t.Fatalf("reference file modified: %q", rf)
	}

	// Report-only MISSING_TIER finding still reported by the detector.
	post, _ := skillsIntegrityHygieneDetector{}.Detect(env)
	if len(post) != 1 {
		t.Fatalf("expected MISSING_TIER report-only finding to remain, got %+v", post)
	}
	hygiene, _ := scanSkillHygiene(repo)
	hasTier := false
	for _, h := range hygiene {
		if h.Kind == "MISSING_TIER" {
			hasTier = true
		}
		if h.Kind == "UNLINKED" {
			t.Fatalf("UNLINKED finding should be gone after fix")
		}
	}
	if !hasTier {
		t.Fatal("expected MISSING_TIER report-only finding to remain")
	}
}

// TestSkillsIntegrityHygieneReportOnlyNoMutate verifies that when the only
// hygiene violations are report-only, the fixer takes no action and does not
// refuse (a clean run with nothing safely fixable).
func TestSkillsIntegrityHygieneReportOnlyNoMutate(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	// Skill missing the tier frontmatter key, no unlinked references.
	skillMD := filepath.Join(repo, "skills", "sample", "SKILL.md")
	writeSkillsFile(t, skillMD, "---\nname: sample\ndescription: d\n---\n\nBody.\n")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	mctx, ra, closer := skillsTestCtx(t, repo, home)
	defer closer()
	findings, _ := skillsIntegrityHygieneDetector{}.Detect(env)
	res, err := skillsIntegrityHygieneFixer{}.Fix(mctx.WithFixer("fm-skills-integrity-hygiene"), env, findings)
	if err != nil {
		t.Fatalf("report-only run should not refuse: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 0 {
		t.Fatalf("report-only run: Fixed=%t ActionsTaken=%d, want true/0", res.Fixed, res.ActionsTaken)
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 0 {
		t.Fatalf("report-only run wrote %d action(s), want 0", len(recs))
	}
}

// --- fm-skills-stale-codex-sync --------------------------------------------

// TestSkillsStaleCodexSyncFixer verifies the fixer mirrors drift surfaces into
// the Codex cache and stamps the install metadata.
func TestSkillsStaleCodexSyncFixer(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	writeSkillsFile(t, filepath.Join(repo, "skills-codex", "demo", "SKILL.md"), "codex skill\n")
	manifestPath := filepath.Join(repo, "skills-codex", ".agentops-manifest.json")
	writeSkillsFile(t, manifestPath, `{"skills":[{"name":"demo"}]}`)
	writeSkillsFile(t, filepath.Join(repo, "hooks", "dangerous-git-guard.sh"), "#!/bin/sh\necho guard\n")
	writeSkillsFile(t, filepath.Join(repo, "lib", "hook-helpers.sh"), "helper\n")

	// Stale Codex install: wrong manifest_hash, missing hook in cache.
	metaPath := filepath.Join(home, ".codex", ".agentops-codex-install.json")
	writeSkillsFile(t, metaPath, `{"install_mode":"native-plugin","manifest_hash":"stale","version":"old"}`)

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home, TargetSHA: "newsha1"}

	findings, err := skillsStaleCodexSyncDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 codex-sync finding, got %+v", findings)
	}

	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsStaleCodexSyncFixer{}.Fix(mctx.WithFixer("fm-skills-stale-codex-sync"), env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed {
		t.Fatal("Fix not marked Fixed")
	}

	codexRoot := codexNativeRoot(home)
	hookGot, err := os.ReadFile(filepath.Join(codexRoot, "hooks", "dangerous-git-guard.sh"))
	if err != nil {
		t.Fatalf("cached hook missing: %v", err)
	}
	if string(hookGot) != "#!/bin/sh\necho guard\n" {
		t.Fatalf("cached hook content = %q", hookGot)
	}
	skillGot, _ := os.ReadFile(filepath.Join(codexRoot, "skills-codex", "demo", "SKILL.md"))
	if string(skillGot) != "codex skill\n" {
		t.Fatalf("cached skill content = %q", skillGot)
	}
	// Install metadata stamped with repo manifest hash + TargetSHA.
	metaGot, _ := os.ReadFile(metaPath)
	manifestBytes, _ := os.ReadFile(manifestPath)
	if !strings.Contains(string(metaGot), hashHex(manifestBytes)) {
		t.Fatalf("install metadata not stamped with manifest hash: %s", metaGot)
	}
	if !strings.Contains(string(metaGot), "newsha1") {
		t.Fatalf("install metadata version not stamped: %s", metaGot)
	}
	if !strings.Contains(string(metaGot), `"install_mode": "native-plugin"`) {
		t.Fatalf("install metadata lost install_mode key: %s", metaGot)
	}

	// Detector no longer fires.
	post, _ := skillsStaleCodexSyncDetector{}.Detect(env)
	if len(post) != 0 {
		t.Fatalf("expected no codex-sync finding after fix, got %+v", post)
	}
}

// TestSkillsStaleCodexSyncNoInstall verifies the detector is silent when there
// is no Codex install (that is fm-skills-missing's domain, not this FM).
func TestSkillsStaleCodexSyncNoInstall(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	writeSkillsFile(t, filepath.Join(repo, "skills-codex", ".agentops-manifest.json"), `{"x":1}`)
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}
	findings, err := skillsStaleCodexSyncDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("no Codex install should yield no findings, got %+v", findings)
	}
}

// --- fm-skills-duplicate-install -------------------------------------------

// TestSkillsDuplicateInstallDetectOnly verifies the detector reports the full
// overlap set and the fixer refuses without writing.
func TestSkillsDuplicateInstallDetectOnly(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	// Two populated roots with overlapping skills: native plugin cache + legacy.
	nativeRoot := filepath.Join(codexNativeRoot(home), "skills-codex")
	legacyRoot := filepath.Join(home, ".agents", "skills")
	for _, name := range []string{"rpi", "evolve"} {
		writeSkillsFile(t, filepath.Join(nativeRoot, name, "SKILL.md"), "x\n")
		writeSkillsFile(t, filepath.Join(legacyRoot, name, "SKILL.md"), "x\n")
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	findings, err := skillsDuplicateInstallDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 || findings[0].ID != "fm-skills-duplicate-install" {
		t.Fatalf("expected 1 duplicate-install finding, got %+v", findings)
	}
	if findings[0].Remediation.AutoFixable {
		t.Fatal("duplicate-install must be detect-only (not auto-fixable)")
	}
	// Full overlap set reported — both skills named, not truncated.
	q := findings[0].Evidence.Query
	if !strings.Contains(q, "rpi") || !strings.Contains(q, "evolve") {
		t.Fatalf("overlap evidence missing a skill name: %q", q)
	}

	// Fixer refuses, writes nothing.
	f := skillsDuplicateInstallFixer{}
	if f.AutoFixable() {
		t.Fatal("fixer AutoFixable() must be false")
	}
	mctx, ra, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := f.Fix(mctx.WithFixer(f.ID()), env, findings)
	if err == nil {
		t.Fatal("expected refusal error from duplicate-install fixer")
	}
	if res.Fixed || res.ActionsTaken != 0 {
		t.Fatalf("refused fixer should be Fixed=false ActionsTaken=0, got %+v", res)
	}
	// No actions recorded.
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 0 {
		t.Fatalf("duplicate-install fixer wrote %d action(s), want 0", len(recs))
	}
}

// TestSkillsDuplicateInstallNoOverlap verifies the detector is silent when
// install roots do not overlap.
func TestSkillsDuplicateInstallNoOverlap(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	writeSkillsFile(t, filepath.Join(home, ".claude", "skills", "rpi", "SKILL.md"), "x\n")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}
	findings, err := skillsDuplicateInstallDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("non-overlapping installs produced %d findings", len(findings))
	}
}

// --- registration -----------------------------------------------------------

// TestSkillsRegistration verifies all six detectors and six fixers are
// registered and that fm-skills-duplicate-install is the only non-auto-fixable.
func TestSkillsRegistration(t *testing.T) {
	want := []string{
		"fm-skills-duplicate-install",
		"fm-skills-hash-drift",
		"fm-skills-integrity-hygiene",
		"fm-skills-missing",
		"fm-skills-stale-codex-sync",
		"fm-skills-stale-command-refs",
	}
	for _, id := range want {
		if FixerByID(id) == nil {
			t.Fatalf("fixer %s not registered", id)
		}
		found := false
		for _, d := range Detectors() {
			if d.ID() == id {
				found = true
				if d.Subsystem() != skillsSubsystem {
					t.Fatalf("detector %s subsystem = %q, want %q", id, d.Subsystem(), skillsSubsystem)
				}
			}
		}
		if !found {
			t.Fatalf("detector %s not registered", id)
		}
	}
	autoFixable := map[string]bool{}
	for _, f := range Fixers() {
		for _, id := range want {
			if f.ID() == id {
				autoFixable[id] = f.AutoFixable()
			}
		}
	}
	if autoFixable["fm-skills-duplicate-install"] {
		t.Fatal("fm-skills-duplicate-install must not be auto-fixable")
	}
	for _, id := range want {
		if id == "fm-skills-duplicate-install" {
			continue
		}
		if !autoFixable[id] {
			t.Fatalf("%s should be auto-fixable", id)
		}
	}
}
