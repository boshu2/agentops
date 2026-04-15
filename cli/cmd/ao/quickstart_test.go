package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestQuickstartSkillCoversGlobalCorpus guards the 2026-04-15 regression:
// a user with GIT=false + AO=true + ~/.agents existing must get a
// global-corpus-aware branch in the quickstart skill, not only "git init".
// This is a doc-level test but lives with the CLI tests so CI catches
// regressions at the same gate.
func TestQuickstartSkillCoversGlobalCorpus(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	cases := []struct {
		path         string
		wantMarkers  []string
		description  string
	}{
		{
			path: filepath.Join(repoRoot, "skills", "quickstart", "SKILL.md"),
			wantMarkers: []string{
				"GLOBAL_AGENTS=true",
				"GIT=false + AO=true + GLOBAL_AGENTS=true",
				"/harvest",
				"/compile",
				"/knowledge-activation",
			},
			description: "Claude skill",
		},
		{
			path: filepath.Join(repoRoot, "skills-codex", "quickstart", "SKILL.md"),
			wantMarkers: []string{
				"GLOBAL_AGENTS=true",
				"GIT=false + AO=true + GLOBAL_AGENTS=true",
				"$harvest",
				"$compile",
				"$knowledge-activation",
			},
			description: "Codex skill",
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.path, err)
			}
			body := string(data)
			for _, marker := range tc.wantMarkers {
				if !strings.Contains(body, marker) {
					t.Errorf("%s missing marker %q (global-corpus branch regressed)", tc.path, marker)
				}
			}
		})
	}
}

func TestQuickstart_CommandExists(t *testing.T) {
	if quickstartCmd == nil {
		t.Fatal("quickstartCmd should not be nil")
	}
	if quickstartCmd.Use != "quick-start" {
		t.Errorf("quickstartCmd.Use = %q, want %q", quickstartCmd.Use, "quick-start")
	}
	if quickstartCmd.GroupID != "start" {
		t.Errorf("quickstartCmd.GroupID = %q, want %q", quickstartCmd.GroupID, "start")
	}
}

func TestQuickstart_HasFlags(t *testing.T) {
	if quickstartCmd.Flags().Lookup("no-beads") == nil {
		t.Error("quick-start should have --no-beads flag")
	}
	if quickstartCmd.Flags().Lookup("minimal") == nil {
		t.Error("quick-start should have --minimal flag")
	}
}

func TestQuickstart_RegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "quick-start" {
			found = true
			break
		}
	}
	if !found {
		t.Error("quickstartCmd should be registered on rootCmd")
	}
}

func TestQuickstart_CreateProjectClaudeMd_Content(t *testing.T) {
	tmp := t.TempDir()
	err := createProjectClaudeMd(tmp)
	if err != nil {
		t.Fatalf("createProjectClaudeMd: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	content := string(data)

	// Should contain key sections.
	for _, want := range []string{"Quick Start", "Session Protocol", "JIT Loading"} {
		if !strings.Contains(content, want) {
			t.Errorf("CLAUDE.md should contain %q", want)
		}
	}

	// Should use directory name as title.
	dirName := filepath.Base(tmp)
	if !strings.Contains(content, dirName) {
		t.Errorf("CLAUDE.md should contain directory name %q", dirName)
	}
}

func TestQuickstart_CreateTasksFile_ValidJSON(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}

	createTasksFile(tmp)

	data, err := os.ReadFile(filepath.Join(tmp, ".agents", "tasks.json"))
	if err != nil {
		t.Fatalf("read tasks.json: %v", err)
	}
	if !strings.Contains(string(data), "tasks") {
		t.Errorf("tasks.json should contain 'tasks' field, got: %s", string(data))
	}
	if !strings.Contains(string(data), "Beads-optional") {
		t.Errorf("tasks.json should contain note about beads-optional mode")
	}
}

func TestQuickstart_ShowNextSteps_WithBeads(t *testing.T) {
	out, _ := captureStdout(t, func() error { showNextSteps(true); return nil })
	if !strings.Contains(out, "bd") {
		t.Errorf("with beads=true, expected bd reference in output:\n%s", out)
	}
	if !strings.Contains(out, "ao factory start") {
		t.Errorf("with beads=true, expected factory lane in output:\n%s", out)
	}
}

func TestQuickstart_ShowNextSteps_WithoutBeads(t *testing.T) {
	out, _ := captureStdout(t, func() error { showNextSteps(false); return nil })
	if out == "" {
		t.Error("expected non-empty output for next steps")
	}
	if !strings.Contains(out, "ao factory start") {
		t.Errorf("without beads, expected factory lane in output:\n%s", out)
	}
}

func TestQuickstart_CreateStarterPack_CreatesPatterns(t *testing.T) {
	tmp := t.TempDir()
	for _, dir := range []string{".agents/patterns", ".agents/learnings"} {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	err := createStarterPack(tmp)
	if err != nil {
		t.Fatalf("createStarterPack: %v", err)
	}

	expectedFiles := []string{
		".agents/patterns/context-boundaries.md",
		".agents/patterns/pre-mortem-first.md",
		".agents/learnings/session-hygiene.md",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(tmp, f)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			t.Errorf("expected %s to exist", f)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("expected %s to have content", f)
		}
	}
}

func TestQuickstart_CreateStarterPack_PatternContent(t *testing.T) {
	tmp := t.TempDir()
	for _, dir := range []string{".agents/patterns", ".agents/learnings"} {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	err := createStarterPack(tmp)
	if err != nil {
		t.Fatalf("createStarterPack: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".agents/patterns/context-boundaries.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Fresh Context Per Phase") {
		t.Error("context-boundaries.md should contain 'Fresh Context Per Phase'")
	}
	if !strings.Contains(content, "40% Rule") {
		t.Error("context-boundaries.md should contain '40% Rule'")
	}
}

// --- runQuickstart tests ---

func TestQuickstart_runQuickstart_minimal(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	oldMinimal := minimal
	minimal = true
	defer func() { minimal = oldMinimal }()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runQuickstart(cmd, nil)
		if err != nil {
			t.Fatalf("runQuickstart minimal: %v", err)
		}
	})

	if !strings.Contains(got, "Minimal setup complete") {
		t.Fatalf("expected minimal completion message, got: %s", got)
	}

	// Verify directories were created
	dirs := []string{
		".agents/research",
		".agents/synthesis",
		".agents/specs",
		".agents/learnings",
		".agents/patterns",
		".agents/retro",
		".agents/handoff",
	}
	for _, dir := range dirs {
		path := filepath.Join(tmp, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("expected directory %s to exist", dir)
		}
	}
}

func TestQuickstart_runQuickstart_fullNoBeads(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	oldMinimal := minimal
	minimal = false
	defer func() { minimal = oldMinimal }()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runQuickstart(cmd, nil)
		if err != nil {
			t.Fatalf("runQuickstart full no-beads: %v", err)
		}
	})

	if !strings.Contains(got, "SETUP COMPLETE") {
		t.Fatalf("expected setup complete message, got: %s", got)
	}

	// Verify starter pack files
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "patterns", "context-boundaries.md")); os.IsNotExist(err) {
		t.Fatal("expected context-boundaries.md to be created")
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "patterns", "pre-mortem-first.md")); os.IsNotExist(err) {
		t.Fatal("expected pre-mortem-first.md to be created")
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "learnings", "session-hygiene.md")); os.IsNotExist(err) {
		t.Fatal("expected session-hygiene.md to be created")
	}
}

func TestQuickstart_runQuickstart_createsClaudeMd(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	oldMinimal := minimal
	minimal = false
	defer func() { minimal = oldMinimal }()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	cmd := &cobra.Command{}
	captureJSONStdout(t, func() {
		err := runQuickstart(cmd, nil)
		if err != nil {
			t.Fatalf("runQuickstart: %v", err)
		}
	})

	claudeMdPath := filepath.Join(tmp, "CLAUDE.md")
	if _, err := os.Stat(claudeMdPath); os.IsNotExist(err) {
		t.Fatal("expected CLAUDE.md to be created")
	}

	content, err := os.ReadFile(claudeMdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Quick Start") {
		t.Fatal("expected CLAUDE.md to contain Quick Start section")
	}
}

func TestQuickstart_runQuickstart_existingClaudeMd(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	// Pre-create CLAUDE.md
	claudeMdPath := filepath.Join(tmp, "CLAUDE.md")
	if err := os.WriteFile(claudeMdPath, []byte("# Existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldMinimal := minimal
	minimal = false
	defer func() { minimal = oldMinimal }()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runQuickstart(cmd, nil)
		if err != nil {
			t.Fatalf("runQuickstart: %v", err)
		}
	})

	if !strings.Contains(got, "CLAUDE.md already exists") {
		t.Fatalf("expected 'already exists' message, got: %s", got)
	}

	// Verify original content preserved
	content, err := os.ReadFile(claudeMdPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# Existing\n" {
		t.Fatal("CLAUDE.md should not have been overwritten")
	}
}

// --- quickstartBeadsStep tests ---

func TestQuickstart_quickstartBeadsStep_noBeads(t *testing.T) {
	tmp := t.TempDir()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	// Pre-create the .agents dir for tasks.json creation
	if err := os.MkdirAll(filepath.Join(tmp, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}

	got := captureJSONStdout(t, func() {
		quickstartBeadsStep(tmp)
	})

	if !strings.Contains(got, "Skipping beads") {
		t.Fatalf("expected skipping beads message, got: %s", got)
	}

	// Verify tasks.json was created
	tasksPath := filepath.Join(tmp, ".agents", "tasks.json")
	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		t.Fatal("expected tasks.json to be created")
	}
}

// --- quickstartClaudeMdStep tests ---

func TestQuickstart_quickstartClaudeMdStep_creates(t *testing.T) {
	tmp := t.TempDir()

	got := captureJSONStdout(t, func() {
		quickstartClaudeMdStep(tmp)
	})

	if !strings.Contains(got, "Created CLAUDE.md") {
		t.Fatalf("expected creation message, got: %s", got)
	}

	claudeMdPath := filepath.Join(tmp, "CLAUDE.md")
	if _, err := os.Stat(claudeMdPath); os.IsNotExist(err) {
		t.Fatal("expected CLAUDE.md to be created")
	}
}

func TestQuickstart_quickstartClaudeMdStep_alreadyExists(t *testing.T) {
	tmp := t.TempDir()
	claudeMdPath := filepath.Join(tmp, "CLAUDE.md")
	if err := os.WriteFile(claudeMdPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	got := captureJSONStdout(t, func() {
		quickstartClaudeMdStep(tmp)
	})

	if !strings.Contains(got, "already exists") {
		t.Fatalf("expected 'already exists' message, got: %s", got)
	}
}

// --- createStarterPack tests ---

func TestQuickstart_createStarterPack(t *testing.T) {
	tmp := t.TempDir()

	// Create needed directories
	dirs := []string{".agents/patterns", ".agents/learnings"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	captureJSONStdout(t, func() {
		err := createStarterPack(tmp)
		if err != nil {
			t.Fatalf("createStarterPack: %v", err)
		}
	})

	// Verify files exist
	expected := []string{
		".agents/patterns/context-boundaries.md",
		".agents/patterns/pre-mortem-first.md",
		".agents/learnings/session-hygiene.md",
	}
	for _, name := range expected {
		path := filepath.Join(tmp, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("expected %s to be created", name)
		}
	}
}

// --- createTasksFile tests ---

func TestQuickstart_createTasksFile(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}

	captureJSONStdout(t, func() {
		createTasksFile(tmp)
	})

	tasksPath := filepath.Join(tmp, ".agents", "tasks.json")
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "tasks") {
		t.Fatalf("expected tasks field in file, got: %s", string(content))
	}
}

// --- createProjectClaudeMd tests ---

func TestQuickstart_createProjectClaudeMd(t *testing.T) {
	tmp := t.TempDir()

	err := createProjectClaudeMd(tmp)
	if err != nil {
		t.Fatalf("createProjectClaudeMd: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}

	// Should contain the directory name as the title
	dirName := filepath.Base(tmp)
	if !strings.Contains(string(content), dirName) {
		t.Fatalf("expected CLAUDE.md to contain dir name %q, got: %s", dirName, string(content))
	}
}

// --- showNextSteps tests ---

func TestQuickstart_showNextSteps_withBeads(t *testing.T) {
	got := captureJSONStdout(t, func() {
		showNextSteps(true)
	})

	if !strings.Contains(got, "Create your first issue") {
		t.Fatalf("expected beads next steps, got: %s", got)
	}
}

func TestQuickstart_showNextSteps_withoutBeads(t *testing.T) {
	got := captureJSONStdout(t, func() {
		showNextSteps(false)
	})

	if !strings.Contains(got, "Start Claude in your project") {
		t.Fatalf("expected no-beads next steps, got: %s", got)
	}
}
