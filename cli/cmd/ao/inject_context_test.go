// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseContextDeclaration_StringForm(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "testskill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	skillMD := `---
name: testskill
context: fork
---

# Test Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to tmpDir so resolveSkillPath finds local skills/
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	decl, err := parseContextDeclaration("testskill")
	if err != nil {
		t.Fatalf("parseContextDeclaration() error = %v", err)
	}
	if decl == nil {
		t.Fatal("parseContextDeclaration() returned nil, want non-nil")
	}
	if decl.Window != "fork" {
		t.Errorf("Window = %q, want %q", decl.Window, "fork")
	}
	if decl.Sections != nil {
		t.Errorf("Sections = %v, want nil", decl.Sections)
	}
	if decl.Intent != nil {
		t.Errorf("Intent = %v, want nil", decl.Intent)
	}
	if decl.IntelScope != "" {
		t.Errorf("IntelScope = %q, want empty", decl.IntelScope)
	}
}

func TestParseContextDeclaration_ObjectForm(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "fullctx")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	skillMD := `---
name: fullctx
context:
  window: isolated
  sections:
    include:
      - INTEL
    exclude:
      - HISTORY
  intent:
    mode: questions
  intel_scope: none
---

# Full Context Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	decl, err := parseContextDeclaration("fullctx")
	if err != nil {
		t.Fatalf("parseContextDeclaration() error = %v", err)
	}
	if decl == nil {
		t.Fatal("parseContextDeclaration() returned nil, want non-nil")
	}
	if decl.Window != "isolated" {
		t.Errorf("Window = %q, want %q", decl.Window, "isolated")
	}
	if decl.Sections == nil {
		t.Fatal("Sections is nil, want non-nil")
	}
	if len(decl.Sections.Include) != 1 || decl.Sections.Include[0] != "INTEL" {
		t.Errorf("Sections.Include = %v, want [INTEL]", decl.Sections.Include)
	}
	if len(decl.Sections.Exclude) != 1 || decl.Sections.Exclude[0] != "HISTORY" {
		t.Errorf("Sections.Exclude = %v, want [HISTORY]", decl.Sections.Exclude)
	}
	if decl.Intent == nil {
		t.Fatal("Intent is nil, want non-nil")
	}
	if decl.Intent.Mode != "questions" {
		t.Errorf("Intent.Mode = %q, want %q", decl.Intent.Mode, "questions")
	}
	if decl.IntelScope != "none" {
		t.Errorf("IntelScope = %q, want %q", decl.IntelScope, "none")
	}
}

func TestParseContextDeclaration_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "noctx")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	skillMD := `---
name: noctx
description: A skill without context
---

# No Context Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	decl, err := parseContextDeclaration("noctx")
	if err != nil {
		t.Fatalf("parseContextDeclaration() error = %v", err)
	}
	if decl != nil {
		t.Errorf("parseContextDeclaration() = %+v, want nil (no context field)", decl)
	}
}

func TestParseContextDeclaration_BackwardCompat(t *testing.T) {
	// Use the real council SKILL.md which has context: fork
	// Skip if running outside the repo (e.g. CI without full checkout)
	origDir, _ := os.Getwd()

	// Try to find the repo root by walking up from cwd
	repoSkill := filepath.Join(origDir, "skills", "council", "SKILL.md")
	// Also check relative to cli/cmd/ao/ (typical test working directory)
	altSkill := filepath.Join(origDir, "..", "..", "..", "skills", "council", "SKILL.md")

	var skillPath string
	if _, err := os.Stat(repoSkill); err == nil {
		skillPath = repoSkill
	} else if _, err := os.Stat(altSkill); err == nil {
		skillPath = altSkill
	}

	if skillPath == "" {
		t.Skip("skills/council/SKILL.md not found — skipping backward compat test")
	}

	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read council SKILL.md: %v", err)
	}

	fm, err := extractFrontmatter(string(data))
	if err != nil {
		t.Fatalf("extractFrontmatter() error = %v", err)
	}
	if fm == "" {
		t.Fatal("extractFrontmatter() returned empty — council SKILL.md should have frontmatter")
	}

	decl, err := parseContextFromFrontmatter([]byte(fm))
	if err != nil {
		t.Fatalf("parseContextFromFrontmatter() error = %v", err)
	}
	if decl == nil {
		t.Fatal("council SKILL.md should have context declaration, got nil")
	}
	// council was upgraded from "context: fork" to structured object with window: isolated
	if decl.Window != "isolated" {
		t.Errorf("council context Window = %q, want %q", decl.Window, "isolated")
	}
}

func TestApplyContextFilter_ExcludeHistory(t *testing.T) {
	knowledge := &injectedKnowledge{
		Sessions: []session{
			{Date: "2026-03-01", Summary: "test session"},
		},
		Learnings: []learning{
			{ID: "l1", Title: "test learning"},
		},
		Timestamp: time.Now(),
	}

	decl := &ContextDeclaration{
		Window: "fork",
		Sections: &SectionFilter{
			Exclude: []string{"HISTORY"},
		},
	}

	result := applyContextFilter(knowledge, decl)
	if result.Sessions != nil {
		t.Errorf("Sessions = %v, want nil after excluding HISTORY", result.Sessions)
	}
	// Learnings should be preserved
	if len(result.Learnings) != 1 {
		t.Errorf("Learnings count = %d, want 1 (should not be affected by HISTORY exclude)", len(result.Learnings))
	}
}

func TestApplyContextFilter_ExcludeIntel(t *testing.T) {
	knowledge := &injectedKnowledge{
		Learnings: []learning{
			{ID: "l1", Title: "test learning"},
		},
		Patterns: []pattern{
			{Name: "p1", Description: "test pattern"},
		},
		Sessions: []session{
			{Date: "2026-03-01", Summary: "test session"},
		},
		Timestamp: time.Now(),
	}

	decl := &ContextDeclaration{
		Window: "inherit",
		Sections: &SectionFilter{
			Exclude: []string{"INTEL"},
		},
	}

	result := applyContextFilter(knowledge, decl)
	if result.Learnings != nil {
		t.Errorf("Learnings = %v, want nil after excluding INTEL", result.Learnings)
	}
	if result.Patterns != nil {
		t.Errorf("Patterns = %v, want nil after excluding INTEL", result.Patterns)
	}
	// Sessions should be preserved
	if len(result.Sessions) != 1 {
		t.Errorf("Sessions count = %d, want 1 (should not be affected by INTEL exclude)", len(result.Sessions))
	}
}

func TestApplyContextFilter_ExcludeMultiple(t *testing.T) {
	pred := &predecessorContext{
		WorkingOn: "test task",
	}
	knowledge := &injectedKnowledge{
		Sessions: []session{
			{Date: "2026-03-01", Summary: "test session"},
		},
		BeadID:      "ag-test",
		Predecessor: pred,
		Learnings: []learning{
			{ID: "l1", Title: "test learning"},
		},
		Timestamp: time.Now(),
	}

	decl := &ContextDeclaration{
		Window: "isolated",
		Sections: &SectionFilter{
			Exclude: []string{"HISTORY", "TASK"},
		},
	}

	result := applyContextFilter(knowledge, decl)
	if result.Sessions != nil {
		t.Errorf("Sessions = %v, want nil after excluding HISTORY", result.Sessions)
	}
	if result.BeadID != "" {
		t.Errorf("BeadID = %q, want empty after excluding TASK", result.BeadID)
	}
	if result.Predecessor != nil {
		t.Errorf("Predecessor = %v, want nil after excluding TASK", result.Predecessor)
	}
	// Learnings should be preserved
	if len(result.Learnings) != 1 {
		t.Errorf("Learnings count = %d, want 1 (should not be affected by HISTORY+TASK exclude)", len(result.Learnings))
	}
}

func TestApplyContextFilter_NoFilter(t *testing.T) {
	pred := &predecessorContext{
		WorkingOn: "test task",
	}
	knowledge := &injectedKnowledge{
		Sessions: []session{
			{Date: "2026-03-01", Summary: "test session"},
		},
		Learnings: []learning{
			{ID: "l1", Title: "test learning"},
		},
		Patterns: []pattern{
			{Name: "p1", Description: "test pattern"},
		},
		BeadID:      "ag-test",
		Predecessor: pred,
		Timestamp:   time.Now(),
	}

	// No excludes — nil declaration
	result := applyContextFilter(knowledge, nil)
	if len(result.Sessions) != 1 {
		t.Errorf("Sessions count = %d, want 1", len(result.Sessions))
	}
	if len(result.Learnings) != 1 {
		t.Errorf("Learnings count = %d, want 1", len(result.Learnings))
	}
	if len(result.Patterns) != 1 {
		t.Errorf("Patterns count = %d, want 1", len(result.Patterns))
	}
	if result.BeadID != "ag-test" {
		t.Errorf("BeadID = %q, want %q", result.BeadID, "ag-test")
	}
	if result.Predecessor == nil {
		t.Error("Predecessor is nil, want non-nil")
	}

	// Also test with empty declaration (no sections filter)
	result2 := applyContextFilter(knowledge, &ContextDeclaration{Window: "inherit"})
	if len(result2.Sessions) != 1 {
		t.Errorf("Sessions count = %d, want 1 (empty decl)", len(result2.Sessions))
	}
	if len(result2.Learnings) != 1 {
		t.Errorf("Learnings count = %d, want 1 (empty decl)", len(result2.Learnings))
	}
}

func TestResolveSkillPath_LocalSkill(t *testing.T) {
	tmpDir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var) so paths match Getwd() output
	tmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(tmpDir, "skills", "testskill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	path, err := resolveSkillPath("testskill")
	if err != nil {
		t.Fatalf("resolveSkillPath() error = %v", err)
	}
	if path != skillFile {
		t.Errorf("resolveSkillPath() = %q, want %q", path, skillFile)
	}
}

func TestResolveSkillPath_InstalledSkill(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create empty local skills dir (no local match)
	if err := os.MkdirAll(filepath.Join(tmpDir, "skills"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create installed skill at fake HOME
	fakeHome := filepath.Join(tmpDir, "fakehome")
	installedDir := filepath.Join(fakeHome, ".claude", "skills", "myskill")
	if err := os.MkdirAll(installedDir, 0755); err != nil {
		t.Fatal(err)
	}
	installedFile := filepath.Join(installedDir, "SKILL.md")
	if err := os.WriteFile(installedFile, []byte("# installed"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", fakeHome)

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	path, err := resolveSkillPath("myskill")
	if err != nil {
		t.Fatalf("resolveSkillPath() error = %v", err)
	}
	if path != installedFile {
		t.Errorf("resolveSkillPath() = %q, want %q", path, installedFile)
	}
}

func TestResolveSkillPath_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, err := resolveSkillPath("nonexistent-skill-xyz")
	if err == nil {
		t.Fatal("resolveSkillPath() error = nil, want error for nonexistent skill")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "not found")
	}
}

func TestApplyContextFilter_IncludeINTELOnly(t *testing.T) {
	pred := &predecessorContext{WorkingOn: "test task"}
	knowledge := &injectedKnowledge{
		Sessions:    []session{{Date: "2026-03-01", Summary: "test"}},
		Learnings:   []learning{{ID: "l1", Title: "test learning"}},
		Patterns:    []pattern{{Name: "p1", Description: "test pattern"}},
		BeadID:      "ag-test",
		Predecessor: pred,
		Timestamp:   time.Now(),
	}

	decl := &ContextDeclaration{
		Window:   "fork",
		Sections: &SectionFilter{Include: []string{"INTEL"}},
	}

	result := applyContextFilter(knowledge, decl)
	// INTEL preserved
	if len(result.Learnings) != 1 {
		t.Errorf("Learnings count = %d, want 1 (INTEL should be preserved)", len(result.Learnings))
	}
	if len(result.Patterns) != 1 {
		t.Errorf("Patterns count = %d, want 1 (INTEL should be preserved)", len(result.Patterns))
	}
	// HISTORY zeroed
	if result.Sessions != nil {
		t.Errorf("Sessions = %v, want nil (HISTORY not in include list)", result.Sessions)
	}
	// TASK zeroed
	if result.BeadID != "" {
		t.Errorf("BeadID = %q, want empty (TASK not in include list)", result.BeadID)
	}
	if result.Predecessor != nil {
		t.Errorf("Predecessor = %v, want nil (TASK not in include list)", result.Predecessor)
	}
}

func TestApplyContextFilter_IncludeHISTORY(t *testing.T) {
	knowledge := &injectedKnowledge{
		Sessions:  []session{{Date: "2026-03-01", Summary: "test"}},
		Learnings: []learning{{ID: "l1", Title: "test learning"}},
		Patterns:  []pattern{{Name: "p1", Description: "test pattern"}},
		BeadID:    "ag-test",
		Timestamp: time.Now(),
	}

	decl := &ContextDeclaration{
		Window:   "inherit",
		Sections: &SectionFilter{Include: []string{"HISTORY"}},
	}

	result := applyContextFilter(knowledge, decl)
	// HISTORY preserved
	if len(result.Sessions) != 1 {
		t.Errorf("Sessions count = %d, want 1 (HISTORY should be preserved)", len(result.Sessions))
	}
	// INTEL zeroed
	if result.Learnings != nil {
		t.Errorf("Learnings = %v, want nil (INTEL not in include list)", result.Learnings)
	}
	if result.Patterns != nil {
		t.Errorf("Patterns = %v, want nil (INTEL not in include list)", result.Patterns)
	}
	// TASK zeroed
	if result.BeadID != "" {
		t.Errorf("BeadID = %q, want empty (TASK not in include list)", result.BeadID)
	}
}

func TestApplyContextFilter_IncludePrecedenceOverExclude(t *testing.T) {
	knowledge := &injectedKnowledge{
		Sessions:  []session{{Date: "2026-03-01", Summary: "test"}},
		Learnings: []learning{{ID: "l1", Title: "test learning"}},
		Patterns:  []pattern{{Name: "p1", Description: "test pattern"}},
		BeadID:    "ag-test",
		Timestamp: time.Now(),
	}

	// Both include and exclude specified — include should win
	decl := &ContextDeclaration{
		Window: "fork",
		Sections: &SectionFilter{
			Include: []string{"INTEL"},
			Exclude: []string{"INTEL"}, // contradicts include — include wins
		},
	}

	result := applyContextFilter(knowledge, decl)
	// Include takes precedence: INTEL is allowed, so it stays
	if len(result.Learnings) != 1 {
		t.Errorf("Learnings count = %d, want 1 (include takes precedence over exclude)", len(result.Learnings))
	}
	if len(result.Patterns) != 1 {
		t.Errorf("Patterns count = %d, want 1 (include takes precedence over exclude)", len(result.Patterns))
	}
	// HISTORY and TASK zeroed (not in include list)
	if result.Sessions != nil {
		t.Errorf("Sessions = %v, want nil (HISTORY not in include list)", result.Sessions)
	}
	if result.BeadID != "" {
		t.Errorf("BeadID = %q, want empty (TASK not in include list)", result.BeadID)
	}
}

func TestValidateIntelScope_Valid(t *testing.T) {
	for _, scope := range []string{"none", "topic", "full"} {
		if err := validateIntelScope(scope); err != nil {
			t.Errorf("validateIntelScope(%q) = %v, want nil", scope, err)
		}
	}
}

func TestValidateIntelScope_Invalid(t *testing.T) {
	if err := validateIntelScope("typo"); err == nil {
		t.Error("validateIntelScope(\"typo\") = nil, want error")
	}
}

func TestParseContextFromFrontmatter_InvalidIntelScope(t *testing.T) {
	fm := []byte("context:\n  window: fork\n  intel_scope: invalid")
	_, err := parseContextFromFrontmatter(fm)
	if err == nil {
		t.Error("parseContextFromFrontmatter with invalid intel_scope should error")
	}
}

func TestValidateSectionNames_Valid(t *testing.T) {
	sf := &SectionFilter{
		Include: []string{"HISTORY", "INTEL", "TASK"},
	}
	if err := validateSectionNames(sf); err != nil {
		t.Errorf("validateSectionNames() = %v, want nil for valid names", err)
	}
}

func TestValidateSectionNames_InvalidInclude(t *testing.T) {
	sf := &SectionFilter{
		Include: []string{"BOGUS"},
	}
	err := validateSectionNames(sf)
	if err == nil {
		t.Error("validateSectionNames() = nil, want error for invalid include section")
	}
	if err != nil && !contains(err.Error(), "BOGUS") {
		t.Errorf("error = %q, want to mention %q", err.Error(), "BOGUS")
	}
}

func TestValidateSectionNames_InvalidExclude(t *testing.T) {
	sf := &SectionFilter{
		Exclude: []string{"INVALID"},
	}
	err := validateSectionNames(sf)
	if err == nil {
		t.Error("validateSectionNames() = nil, want error for invalid exclude section")
	}
}

func TestValidateSectionNames_Nil(t *testing.T) {
	if err := validateSectionNames(nil); err != nil {
		t.Errorf("validateSectionNames(nil) = %v, want nil", err)
	}
}

func TestParseContextFromFrontmatter_InvalidSectionName(t *testing.T) {
	fm := []byte("context:\n  window: fork\n  sections:\n    include:\n      - BOGUS")
	_, err := parseContextFromFrontmatter(fm)
	if err == nil {
		t.Error("parseContextFromFrontmatter with invalid section name should error")
	}
}

func TestSectionConstants_MatchExpected(t *testing.T) {
	// Verify the exported section constants have the expected values
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"SectionLearnings", SectionLearnings, "learnings"},
		{"SectionPatterns", SectionPatterns, "patterns"},
		{"SectionResearch", SectionResearch, "research"},
		{"SectionSessions", SectionSessions, "sessions"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
