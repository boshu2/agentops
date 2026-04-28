package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitCreatesDirs(t *testing.T) {
	tmp := t.TempDir()
	// Create a fake .git so it's treated as a git repo
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	dryRun = false
	initStealth = false
	initHooks = false

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// Verify all agentsDirs exist
	for _, dir := range agentsDirs {
		target := filepath.Join(tmp, dir)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			t.Errorf("expected dir %s to exist", dir)
		}
	}

	// Verify .agents/ao storage subdirs (created by storage.Init)
	for _, sub := range []string{"sessions", "index", "provenance"} {
		target := filepath.Join(tmp, ".agents/ao", sub)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			t.Errorf("expected dir .agents/ao/%s to exist", sub)
		}
	}

	// Verify total dir count matches expectation (agentsDirs + 3 storage subdirs)
	// agentsDirs includes .agents/ao, storage.Init adds sessions/index/provenance under it
	expectedDirs := len(agentsDirs) + 3 // +3 for ao/{sessions,index,provenance}
	actualDirs := 0
	_ = filepath.Walk(filepath.Join(tmp, ".agents"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && path != filepath.Join(tmp, ".agents") {
			actualDirs++
		}
		return nil
	})
	if actualDirs < expectedDirs {
		t.Errorf("expected at least %d dirs under .agents/, got %d", expectedDirs, actualDirs)
	}
}

func TestRunInitGitignoreAppend(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if !strings.Contains(string(data), ".agents/") {
		t.Error("expected .gitignore to contain .agents/")
	}
	if !strings.Contains(string(data), "node_modules/") {
		t.Error("expected .gitignore to still contain node_modules/")
	}
}

func TestRunInitGitignoreCreate(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if err != nil {
		t.Fatalf("expected .gitignore to be created: %v", err)
	}
	if !strings.Contains(string(data), ".agents/") {
		t.Error("expected .gitignore to contain .agents/")
	}
}

func TestRunInitIdempotent(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false

	// Run twice
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("second runInit: %v", err)
	}

	// .agents/ should appear only once in .gitignore
	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	count := strings.Count(string(data), ".agents/\n")
	if count != 1 {
		t.Errorf("expected .agents/ once in .gitignore, got %d", count)
	}
}

func TestRunInitNonGitRepo(t *testing.T) {
	tmp := t.TempDir()
	// No .git directory

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// Dirs should still be created
	for _, dir := range agentsDirs {
		if _, err := os.Stat(filepath.Join(tmp, dir)); os.IsNotExist(err) {
			t.Errorf("expected dir %s to exist even without git", dir)
		}
	}

	// .gitignore should NOT be created
	if _, err := os.Stat(filepath.Join(tmp, ".gitignore")); err == nil {
		t.Error("expected .gitignore NOT to be created in non-git repo")
	}
}

func TestRunInitStealth(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git", "info"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = true
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// .git/info/exclude should contain .agents/
	data, err := os.ReadFile(filepath.Join(tmp, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("expected .git/info/exclude to exist: %v", err)
	}
	if !strings.Contains(string(data), ".agents/") {
		t.Error("expected .git/info/exclude to contain .agents/")
	}

	// .gitignore should NOT be modified
	if _, err := os.Stat(filepath.Join(tmp, ".gitignore")); err == nil {
		t.Error("expected .gitignore NOT to be created in stealth mode")
	}
}

func TestRunInitDryRun(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	dryRun = true
	defer func() { dryRun = false }()

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit dry-run: %v", err)
	}

	// No directories should be created (except what TempDir gives us)
	for _, dir := range agentsDirs {
		if _, err := os.Stat(filepath.Join(tmp, dir)); err == nil {
			t.Errorf("expected dir %s NOT to exist in dry-run", dir)
		}
	}
}

func TestNestedGitignoreContent(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".agents", ".gitignore"))
	if err != nil {
		t.Fatalf("expected .agents/.gitignore: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "*") {
		t.Error("expected .agents/.gitignore to contain *")
	}
	if !strings.Contains(content, "!.gitignore") {
		t.Error("expected .agents/.gitignore to contain !.gitignore")
	}
	if !strings.Contains(content, "!README.md") {
		t.Error("expected .agents/.gitignore to contain !README.md")
	}
}

func TestRunInitGitignoreNoTrailingNewline(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	// Write file without trailing newline
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("node_modules/"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	content := string(data)
	// Should have newline between existing content and new entry
	if strings.Contains(content, "node_modules/.agents/") {
		t.Error("expected newline between existing content and .agents/ entry")
	}
	if !strings.Contains(content, ".agents/") {
		t.Error("expected .gitignore to contain .agents/")
	}
}

func TestFileContainsLine(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(path, []byte("foo\n.agents/\nbar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if !fileContainsLine(path, ".agents/") {
		t.Error("expected to find .agents/")
	}
	if fileContainsLine(path, "baz") {
		t.Error("expected NOT to find baz")
	}
	if fileContainsLine(filepath.Join(tmp, "nonexistent"), "x") {
		t.Error("expected false for nonexistent file")
	}
}

func TestIsGitRepository(t *testing.T) {
	tmp := t.TempDir()
	if isGitRepository(tmp) {
		t.Error("expected false for non-git dir")
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if !isGitRepository(tmp) {
		t.Error("expected true for git dir")
	}
}

func TestWarnTrackedFilesNoError(t *testing.T) {
	// warnTrackedFiles should handle non-git directories gracefully
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	tmp := t.TempDir()
	out, _ := captureStdout(t, func() error {
		warnTrackedFiles(tmp)
		return nil
	})
	// In a non-git directory, should produce no warnings about tracked files
	if strings.Contains(out, "tracked") {
		t.Errorf("non-git dir should not warn about tracked files, got: %s", out)
	}
}
