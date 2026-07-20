package doctor

import (
	"fmt"
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
	codexRefMD := filepath.Join(repo, "skills-codex", "sample", "references", "flow.md")
	writeSkillsFile(t, codexRefMD, "Use `ao handoff` to pass context.\n")

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
	if string(gotDoc) != "Use `ao work rpi` to start.\n" {
		t.Fatalf("docs/sample.md after fix = %q", gotDoc)
	}
	gotCodexRef, _ := os.ReadFile(codexRefMD)
	if string(gotCodexRef) != "Use `ao session handoff` to pass context.\n" {
		t.Fatalf("skills-codex reference after fix = %q", gotCodexRef)
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

	// actions.jsonl has one line for each changed file, correct fixer id + op.
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

// TestSkillsStaleCommandRefsRefusesAmbiguous is the migration-owner discipline
// proof (skills/standards/references/migration-owner.md, rule 3): when a file
// holds BOTH a deprecated command AND its replacement, the fixer must REFUSE to
// rewrite it — skip and surface, never guess which reference the author meant to
// keep. The prior behavior rewrote the old form blindly, corrupting a deliberate
// reference (or double-applying the rename).
func TestSkillsStaleCommandRefsRefusesAmbiguous(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	// Both `ao state` (deprecated) and `ao session state` (its replacement)
	// appear on ordinary (non-rename-doc) lines. The author references both on
	// purpose — the migration owner cannot tell stale usage from a deliberate
	// mention, so it must refuse.
	ambMD := filepath.Join(repo, "skills", "amb", "SKILL.md")
	original := "# Amb\n\nThe `ao state` command was renamed last release.\n" +
		"Use `ao session state` from now on.\n"
	writeSkillsFile(t, ambMD, original)
	// A control file with ONLY the deprecated form must still be fixed in the
	// same pass — the refusal is scoped to the ambiguous unit, not the run.
	cleanMD := filepath.Join(repo, "skills", "clean", "SKILL.md")
	writeSkillsFile(t, cleanMD, "Run `ao know inject` to load context.\n")

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}
	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()

	findings, _ := skillsStaleCommandRefsDetector{}.Detect(env)
	res, err := skillsStaleCommandRefsFixer{}.Fix(mctx.WithFixer("fm-skills-stale-command-refs"), env, findings)
	if err != nil {
		t.Fatalf("Fix returned error (should refuse cleanly, not error): %v", err)
	}

	// The ambiguous file is left byte-for-byte unchanged (refused, not guessed).
	got, _ := os.ReadFile(ambMD)
	if string(got) != original {
		t.Fatalf("ambiguous file was rewritten (guessed) instead of refused:\n got=%q\nwant=%q", got, original)
	}
	// The refusal is surfaced, not swallowed.
	if len(res.Skipped) == 0 {
		t.Fatal("ambiguous file must be surfaced in FixResult.Skipped, got none")
	}
	joined := strings.Join(res.Skipped, "\n")
	if !strings.Contains(joined, "amb") {
		t.Fatalf("Skipped surface does not name the ambiguous file: %q", res.Skipped)
	}
	// An unresolved ambiguous refusal is NOT a completed fix.
	if res.Fixed {
		t.Fatal("Fixed must be false while an ambiguous unit remains unresolved")
	}
	// The clean control file WAS fixed in the same pass.
	gotClean, _ := os.ReadFile(cleanMD)
	if string(gotClean) != "Run `ao inject` to load context.\n" {
		t.Fatalf("clean file not fixed in the same pass: %q", gotClean)
	}
	if res.ActionsTaken != 1 {
		t.Fatalf("ActionsTaken = %d, want 1 (only the clean file)", res.ActionsTaken)
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

// TestSkillsMissingSymlinkResolution verifies the detector counts a skill as
// present when the install-dir entry is a symlink that resolves to a directory
// containing SKILL.md (the canonical `ao skills link` layout), while a dangling
// symlink still counts as absent.
func TestSkillsMissingSymlinkResolution(t *testing.T) {
	cases := []struct {
		name         string
		populate     func(t *testing.T, home string)
		wantFindings int
	}{
		{
			name: "real dir with SKILL.md counts as present",
			populate: func(t *testing.T, home string) {
				writeSkillsFile(t, filepath.Join(home, ".agents", "skills", "rpi", "SKILL.md"), "x\n")
			},
			wantFindings: 0,
		},
		{
			name: "symlink resolving to dir with SKILL.md counts as present",
			populate: func(t *testing.T, home string) {
				src := filepath.Join(home, "checkout", "skills", "rpi")
				writeSkillsFile(t, filepath.Join(src, "SKILL.md"), "x\n")
				install := filepath.Join(home, ".agents", "skills")
				if err := os.MkdirAll(install, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(src, filepath.Join(install, "rpi")); err != nil {
					t.Fatal(err)
				}
			},
			wantFindings: 0,
		},
		{
			name: "dangling symlink still counts as absent",
			populate: func(t *testing.T, home string) {
				install := filepath.Join(home, ".agents", "skills")
				if err := os.MkdirAll(install, 0o755); err != nil {
					t.Fatal(err)
				}
				gone := filepath.Join(home, "checkout", "skills", "gone")
				if err := os.Symlink(gone, filepath.Join(install, "rpi")); err != nil {
					t.Fatal(err)
				}
			},
			wantFindings: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			home := t.TempDir()
			tc.populate(t, home)
			env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}
			findings, err := skillsMissingDetector{}.Detect(env)
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if len(findings) != tc.wantFindings {
				t.Fatalf("findings = %d, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			if tc.wantFindings == 1 && findings[0].ID != "fm-skills-missing" {
				t.Fatalf("finding ID = %q, want fm-skills-missing", findings[0].ID)
			}
		})
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
	hygiene, _, _ := scanSkillHygiene(repo)
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
// TestSkillsHygiene_PlaceholderLinkNotDeadRef pins the DEAD_REF placeholder
// exclusion: a template link like [text](references/<topic>.md) in skill docs
// (showing the link FORMAT, e.g. skill-builder's "move section bodies to
// references/<topic>.md") is NOT a real link and must not be flagged DEAD_REF.
// '<'/'>' are never valid in real filenames. Found via the diagnostic-layer audit.
func TestSkillsHygiene_PlaceholderLinkNotDeadRef(t *testing.T) {
	repo := t.TempDir()
	// Full frontmatter (no MISSING_*), no references/ dir (no UNLINKED), a template
	// placeholder link — the only thing that could (wrongly) fire is DEAD_REF.
	skillMD := filepath.Join(repo, "skills", "doc", "SKILL.md")
	body := "---\nname: doc\ndescription: docs\ntier: reference\n---\n\n# Doc\n\n" +
		"Move section bodies to `references/<topic>.md`; reference inline as [text](references/<topic>.md).\n"
	writeSkillsFile(t, skillMD, body)

	hygiene, _, err := scanSkillHygiene(repo)
	if err != nil {
		t.Fatalf("scanSkillHygiene: %v", err)
	}
	for _, h := range hygiene {
		if h.Kind == "DEAD_REF" {
			t.Fatalf("template placeholder link must not be flagged DEAD_REF, got %+v", h)
		}
	}
}

// TestHasAnglePlaceholder pins the precision the cross-family pawl required: only
// a real '<...>' SEGMENT (placeholder) is skipped — a lone bracket or a normal
// path is NOT, so a genuine dead reference is still reported (no fail-open).
func TestHasAnglePlaceholder(t *testing.T) {
	cases := map[string]bool{
		"references/<topic>.md":  true,  // documented placeholder (stem is entirely <...>)
		"references/<a>/<b>.md":  true,  // last segment stem "<b>" is a placeholder
		"references/real.md":     false, // normal path → still checked for DEAD_REF
		"references/foo<bar>.md": false, // EMBEDDED <...> → malformed, still flagged (no fail-open)
		"references/a<b.md":      false, // lone '<' → not a placeholder
		"references/a>b.md":      false, // lone '>' → not a placeholder
		"references/<x>y.md":     false, // <...> not the whole stem → not a placeholder
		"":                       false,
	}
	for in, want := range cases {
		if got := hasAnglePlaceholder(in); got != want {
			t.Errorf("hasAnglePlaceholder(%q) = %v, want %v", in, got, want)
		}
	}
}

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

	// Stale Codex install: wrong manifest_hash + version.
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

func TestSkillsStaleCodexSyncFixerNoDriftNoMutate(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	manifestPath := filepath.Join(repo, "skills-codex", ".agentops-manifest.json")
	writeSkillsFile(t, manifestPath, `{"skills":[{"name":"demo"}]}`)
	manifestBytes, _ := os.ReadFile(manifestPath)
	writeSkillsFile(t, codexInstallMetaPath(home),
		`{"install_mode":"native-plugin","manifest_hash":"`+hashHex(manifestBytes)+`","version":"newsha1"}`)

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home, TargetSHA: "newsha1"}
	mctx, ra, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsStaleCodexSyncFixer{}.Fix(mctx.WithFixer("fm-skills-stale-codex-sync"), env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 0 {
		t.Fatalf("no-drift fix: Fixed=%t ActionsTaken=%d, want true/0", res.Fixed, res.ActionsTaken)
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 0 {
		t.Fatalf("no-drift fix wrote %d action(s), want 0", len(recs))
	}
}

func TestSkillsStaleCodexSyncSourcesRejectMissingInputs(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	_, err := skillsStaleCodexSyncFixer{}.codexSyncSources(mctx, env)
	if err == nil || !strings.Contains(err.Error(), "no skills-codex/ source") {
		t.Fatalf("missing skills-codex error = %v, want source refusal", err)
	}

	writeSkillsFile(t, filepath.Join(repo, "skills-codex", ".agentops-manifest.json"), `{}`)
	_, err = skillsStaleCodexSyncFixer{}.codexSyncSources(mctx, env)
	if err == nil || !strings.Contains(err.Error(), "no Codex install present") {
		t.Fatalf("missing Codex install error = %v, want install refusal", err)
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

func TestSkillsDuplicateInstallDetectsVersionedNativeCache(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	versionedRoot := filepath.Join(home, ".codex", "plugins", "cache",
		"agentops-marketplace", "agentops", "3.2.0", "skills-codex")
	legacyRoot := filepath.Join(home, ".agents", "skills")
	writeSkillsFile(t, filepath.Join(versionedRoot, "rpi", "SKILL.md"), "native\n")
	writeSkillsFile(t, filepath.Join(legacyRoot, "rpi", "SKILL.md"), "stale raw\n")
	// Reproduce the live failure: metadata still names the old local cache.
	writeSkillsFile(t, filepath.Join(home, ".codex", ".agentops-codex-install.json"),
		`{"plugin_root":"`+filepath.Join(home, ".codex", "plugins", "cache",
			"agentops-marketplace", "agentops", "local")+`"}`)

	findings, err := skillsDuplicateInstallDetector{}.Detect(&DetectEnv{
		RepoRoot: repo, CWD: repo, HomeDir: home,
	})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0].Evidence.Query, "rpi") {
		t.Fatalf("versioned native/raw overlap was not detected: %+v", findings)
	}
	if !strings.Contains(findings[0].Evidence.File, "3.2.0") {
		t.Fatalf("detector did not select the live versioned plugin as primary: %+v", findings[0].Evidence)
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

// TestSkillsRegistration verifies all five detectors and five fixers are
// registered and that fm-skills-duplicate-install is the only non-auto-fixable.
func TestSkillsRegistration(t *testing.T) {
	want := []string{
		"fm-skills-duplicate-install",
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

// TestSkillsHashDriftDetectorIsRemoved pins age-aau9: the codex hash-drift
// detector+fixer was REMOVED because its Go re-implementation diverged from the
// canonical scripts/regen-codex-hashes.sh hash (false-positiving on all skills,
// and its --fix would corrupt the canonical hashes and red regen-check). Codex
// hash validation is owned by `make regen-check`. This guard fails if anyone
// re-registers a divergent reimplementation.
func TestSkillsHashDriftDetectorIsRemoved(t *testing.T) {
	const id = "fm-skills-hash-drift"
	if FixerByID(id) != nil {
		t.Fatal("fm-skills-hash-drift fixer must NOT be registered (age-aau9: owned by make regen-check)")
	}
	for _, d := range Detectors() {
		if d.ID() == id {
			t.Fatal("fm-skills-hash-drift detector must NOT be registered (age-aau9: owned by make regen-check)")
		}
	}
}

// ---------------------------------------------------------------------------
// Symlinked-root guards (age-knowledge-symlink-root-inbpg)
// ---------------------------------------------------------------------------

// TestSkillsSymlinkedSkillsRoot: a repo whose skills/ root is a symlink to an
// external tree must silence the stale-command-refs and integrity-hygiene
// detectors, make both repo-writing fixers refuse with refused_unsafe, record
// zero actions, and leave the external tree byte-untouched.
func TestSkillsSymlinkedSkillsRoot(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	external := t.TempDir()
	// Baits: a stale command ref (rewrite bait) and an unlinked references
	// file (hygiene append bait) in the external tree.
	skillMDContent := "---\nname: sample\ndescription: d\ntier: core\n---\n\nRun `ao know inject` now.\n"
	writeSkillsFile(t, filepath.Join(external, "sample", "SKILL.md"), skillMDContent)
	refContent := "unlinked reference\n"
	writeSkillsFile(t, filepath.Join(external, "sample", "references", "extra.md"), refContent)
	if err := os.Symlink(external, filepath.Join(repo, "skills")); err != nil {
		t.Fatalf("symlink skills: %v", err)
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	for name, d := range map[string]Detector{
		"stale-command-refs": skillsStaleCommandRefsDetector{},
		"integrity-hygiene":  skillsIntegrityHygieneDetector{},
	} {
		findings, err := d.Detect(env)
		if err != nil {
			t.Errorf("%s Detect on symlinked skills root: %v", name, err)
		}
		if len(findings) != 0 {
			t.Errorf("%s Detect on symlinked skills root = %d findings, want 0", name, len(findings))
		}
	}

	mctx, ra, closer := skillsTestCtx(t, repo, home)
	defer closer()
	for name, f := range map[string]Fixer{
		"stale-command-refs": skillsStaleCommandRefsFixer{},
		"integrity-hygiene":  skillsIntegrityHygieneFixer{},
	} {
		res, err := f.Fix(mctx.WithFixer(f.ID()), env, nil)
		if err == nil || !strings.Contains(err.Error(), "refused_unsafe") {
			t.Errorf("%s Fix on symlinked skills root: err=%v, want refused_unsafe", name, err)
			continue
		}
		if res.Fixed {
			t.Errorf("%s Fix reported Fixed despite refusal", name)
		}
		if res.ActionsTaken != 0 {
			t.Errorf("%s Fix took %d actions through a symlinked skills root", name, res.ActionsTaken)
		}
	}
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Fatalf("action records = %d, want 0", len(recs))
	}
	got, err := os.ReadFile(filepath.Join(external, "sample", "SKILL.md"))
	if err != nil || string(got) != skillMDContent {
		t.Errorf("external SKILL.md = %q err=%v, want untouched", got, err)
	}
	gotRef, err := os.ReadFile(filepath.Join(external, "sample", "references", "extra.md"))
	if err != nil || string(gotRef) != refContent {
		t.Errorf("external reference = %q err=%v, want untouched", gotRef, err)
	}
}

// TestSkillsSymlinkedDocsRoot: the guard covers every stale-ref scan root, not
// just skills/ — a symlinked docs/ silences the detector and refuses the fixer
// even when skills/ is a real directory with its own bait.
func TestSkillsSymlinkedDocsRoot(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	external := t.TempDir()
	writeSkillsFile(t, filepath.Join(repo, "skills", "sample", "SKILL.md"), "Run `ao know inject` now.\n")
	docContent := "Run `ao know inject` now.\n"
	writeSkillsFile(t, filepath.Join(external, "sample.md"), docContent)
	if err := os.Symlink(external, filepath.Join(repo, "docs")); err != nil {
		t.Fatalf("symlink docs: %v", err)
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	findings, err := skillsStaleCommandRefsDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("stale-refs Detect with symlinked docs root = %d findings, want 0", len(findings))
	}
	mctx, ra, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsStaleCommandRefsFixer{}.Fix(mctx.WithFixer("fm-skills-stale-command-refs"), env, nil)
	if err == nil || !strings.Contains(err.Error(), "refused_unsafe") {
		t.Fatalf("stale-refs Fix with symlinked docs root: err=%v, want refused_unsafe", err)
	}
	if res.ActionsTaken != 0 {
		t.Fatalf("stale-refs Fix took %d actions", res.ActionsTaken)
	}
	recs, rerr := readActions(ra.ActionsPath())
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(recs) != 0 {
		t.Fatalf("action records = %d, want 0", len(recs))
	}
	gotDoc, err := os.ReadFile(filepath.Join(external, "sample.md"))
	if err != nil || string(gotDoc) != docContent {
		t.Fatalf("external doc = %q err=%v, want untouched", gotDoc, err)
	}
	// The repo-side bait is real: the skill file still holds its stale ref.
	gotSkill, err := os.ReadFile(filepath.Join(repo, "skills", "sample", "SKILL.md"))
	if err != nil || string(gotSkill) != "Run `ao know inject` now.\n" {
		t.Fatalf("repo skill file = %q err=%v, want untouched", gotSkill, err)
	}
}

// TestSkillsStaleRefsSkipsSymlinkedFile: with real roots, a scanned FILE that
// is itself a symlink to an external target is invisible to the scan — the
// detector reports only the real file, the fixer rewrites only the real file,
// and the symlink target stays byte-untouched.
func TestSkillsStaleRefsSkipsSymlinkedFile(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	external := t.TempDir()
	realDoc := filepath.Join(repo, "docs", "real.md")
	writeSkillsFile(t, realDoc, "Run `ao know inject` to load context.\n")
	victim := filepath.Join(external, "victim.md")
	victimContent := "Run `ao know inject` now.\n"
	if err := os.WriteFile(victim, []byte(victimContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(repo, "docs", "linked.md")); err != nil {
		t.Fatalf("symlink doc file: %v", err)
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	findings, err := skillsStaleCommandRefsDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1 (real file only)", len(findings))
	}
	if q := findings[0].Evidence.Query; strings.Contains(q, "linked.md") {
		t.Fatalf("evidence lists the symlinked file: %q", q)
	}
	mctx, _, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsStaleCommandRefsFixer{}.Fix(mctx.WithFixer("fm-skills-stale-command-refs"), env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 1 {
		t.Fatalf("Fix = fixed=%t actions=%d, want fixed=true actions=1", res.Fixed, res.ActionsTaken)
	}
	gotReal, _ := os.ReadFile(realDoc)
	if string(gotReal) != "Run `ao inject` to load context.\n" {
		t.Fatalf("real doc after fix = %q", gotReal)
	}
	gotVictim, err := os.ReadFile(victim)
	if err != nil || string(gotVictim) != victimContent {
		t.Fatalf("symlink target = %q err=%v, want untouched", gotVictim, err)
	}
}

// TestSkillsHygieneSymlinkedTargetRefusesWholeRun covers the partial-fix
// hazard: with a real skills/ root, skill "askill" is real (with an unlinked
// reference) and skill "zskill"'s SKILL.md is a SYMLINK to an external file.
// The detector must exclude zskill from its findings entirely
// (detector-silence), and the fixer must refuse the WHOLE run with zero
// actions — never fix askill first and then hit the symlink (action already
// committed). The final control replaces the symlink with a real file and
// proves both skills then fix, so the refusal is attributable to the symlink.
func TestSkillsHygieneSymlinkedTargetRefusesWholeRun(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	external := t.TempDir()
	frontmatter := "---\nname: %s\ndescription: d\ntier: core\n---\n\n# Body\n"
	askillMD := fmt.Sprintf(frontmatter, "askill")
	writeSkillsFile(t, filepath.Join(repo, "skills", "askill", "SKILL.md"), askillMD)
	writeSkillsFile(t, filepath.Join(repo, "skills", "askill", "references", "extra.md"), "unlinked a\n")
	// zskill: real dir + real unlinked reference, but SKILL.md is a symlink.
	writeSkillsFile(t, filepath.Join(repo, "skills", "zskill", "references", "zref.md"), "unlinked z\n")
	zskillContent := fmt.Sprintf(frontmatter, "zskill")
	zVictim := filepath.Join(external, "zskill-SKILL.md")
	if err := os.WriteFile(zVictim, []byte(zskillContent), 0o644); err != nil {
		t.Fatal(err)
	}
	zskillMD := filepath.Join(repo, "skills", "zskill", "SKILL.md")
	if err := os.Symlink(zVictim, zskillMD); err != nil {
		t.Fatalf("symlink zskill SKILL.md: %v", err)
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	// Detector-silence: findings cover askill only, never the symlinked zskill.
	findings, err := skillsIntegrityHygieneDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if q := findings[0].Evidence.Query; !strings.Contains(q, "askill:UNLINKED") || strings.Contains(q, "zskill") {
		t.Fatalf("evidence = %q, want askill:UNLINKED only, zskill excluded", q)
	}

	mctx, ra, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsIntegrityHygieneFixer{}.Fix(mctx.WithFixer("fm-skills-integrity-hygiene"), env, nil)
	if err == nil || !strings.Contains(err.Error(), "refused_unsafe") {
		t.Fatalf("Fix with symlinked zskill: err=%v, want whole-run refused_unsafe", err)
	}
	if !strings.Contains(err.Error(), "zskill") {
		t.Errorf("refusal error %v does not name the symlinked skill", err)
	}
	if res.Fixed {
		t.Error("Fix reported Fixed despite refusal")
	}
	if res.ActionsTaken != 0 {
		t.Errorf("Fix took %d actions, want 0 (whole-run refusal, no partial fix)", res.ActionsTaken)
	}
	recs, rerr := readActions(ra.ActionsPath())
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(recs) != 0 {
		t.Fatalf("action records = %d, want 0", len(recs))
	}
	// askill untouched (no partial fix) and the external target byte-untouched.
	gotA, err := os.ReadFile(filepath.Join(repo, "skills", "askill", "SKILL.md"))
	if err != nil || string(gotA) != askillMD {
		t.Fatalf("askill SKILL.md = %q err=%v, want byte-untouched", gotA, err)
	}
	gotZ, err := os.ReadFile(zVictim)
	if err != nil || string(gotZ) != zskillContent {
		t.Fatalf("external zskill target = %q err=%v, want byte-untouched", gotZ, err)
	}

	// Fidelity control: replace the symlink with a REAL file — the same run now
	// fixes BOTH skills, proving the refusal above came from the symlink gate.
	if err := os.Remove(zskillMD); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zskillMD, []byte(zskillContent), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = skillsIntegrityHygieneFixer{}.Fix(mctx.WithFixer("fm-skills-integrity-hygiene"), env, nil)
	if err != nil {
		t.Fatalf("control Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 2 {
		t.Fatalf("control Fix = fixed=%t actions=%d, want fixed=true actions=2 (bait is dead)", res.Fixed, res.ActionsTaken)
	}
}

// TestSkillsHygieneSymlinkedReferencesDirExcluded: a skill whose references/
// dir is a symlink to an external directory must be excluded from hygiene
// findings entirely (the detector never lists the external dir's contents),
// and the fixer must refuse the whole run rather than act in a tree holding a
// symlinked skill surface.
func TestSkillsHygieneSymlinkedReferencesDirExcluded(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	external := t.TempDir()
	bskillMD := "---\nname: bskill\ndescription: d\ntier: core\n---\n\n# Body\n"
	writeSkillsFile(t, filepath.Join(repo, "skills", "bskill", "SKILL.md"), bskillMD)
	// External references dir with a would-be UNLINKED bait.
	extRefs := filepath.Join(external, "refs")
	writeSkillsFile(t, filepath.Join(extRefs, "extra.md"), "external unlinked ref\n")
	if err := os.Symlink(extRefs, filepath.Join(repo, "skills", "bskill", "references")); err != nil {
		t.Fatalf("symlink references dir: %v", err)
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home}

	findings, err := skillsIntegrityHygieneDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0 (skill with symlinked references dir excluded entirely): %+v", len(findings), findings)
	}

	mctx, ra, closer := skillsTestCtx(t, repo, home)
	defer closer()
	res, err := skillsIntegrityHygieneFixer{}.Fix(mctx.WithFixer("fm-skills-integrity-hygiene"), env, nil)
	if err == nil || !strings.Contains(err.Error(), "refused_unsafe") {
		t.Fatalf("Fix with symlinked references dir: err=%v, want refused_unsafe", err)
	}
	if res.Fixed || res.ActionsTaken != 0 {
		t.Fatalf("Fix = fixed=%t actions=%d, want fixed=false actions=0", res.Fixed, res.ActionsTaken)
	}
	recs, rerr := readActions(ra.ActionsPath())
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(recs) != 0 {
		t.Fatalf("action records = %d, want 0", len(recs))
	}
	gotRef, err := os.ReadFile(filepath.Join(extRefs, "extra.md"))
	if err != nil || string(gotRef) != "external unlinked ref\n" {
		t.Fatalf("external reference = %q err=%v, want untouched", gotRef, err)
	}
	if got, rerr := os.ReadFile(filepath.Join(repo, "skills", "bskill", "SKILL.md")); rerr != nil || string(got) != bskillMD {
		t.Fatalf("bskill SKILL.md = %q err=%v, want byte-untouched", got, rerr)
	}
}
