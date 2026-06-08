package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSealSkillEditCommitsLiveSkillChange(t *testing.T) {
	repo := setupSkillEditRepo(t)
	writeSkillEditFile(t, repo, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: alpha\n---\nold\n")
	gitSkillEdit(t, repo, "add", ".")
	gitSkillEdit(t, repo, "commit", "-m", "initial")

	writeSkillEditFile(t, repo, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: alpha\n---\nnew\n")
	result, err := sealSkillEdit(skillEditSealOptions{
		RepoRoot: repo,
		Skill:    "alpha",
		Actor:    "test-agent",
	})
	if err != nil {
		t.Fatalf("sealSkillEdit error: %v", err)
	}
	if !strings.Contains(result, "sealed skill edit alpha") {
		t.Fatalf("unexpected result: %s", result)
	}
	status := gitSkillEdit(t, repo, "status", "--porcelain")
	if strings.TrimSpace(status) != "" {
		t.Fatalf("expected clean repo after seal, got:\n%s", status)
	}
	log := gitSkillEdit(t, repo, "log", "-1", "--format=%B")
	if !strings.Contains(log, "Skill-Edit: alpha") || !strings.Contains(log, "Skill-Edit-Actor: test-agent") {
		t.Fatalf("missing skill-edit trailers:\n%s", log)
	}
}

func TestSealSkillEditSetsAgentNameForCommitHooks(t *testing.T) {
	t.Setenv("AGENT_NAME", "")
	repo := setupSkillEditRepo(t)
	writeSkillEditFile(t, repo, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: alpha\n---\nold\n")
	gitSkillEdit(t, repo, "add", ".")
	gitSkillEdit(t, repo, "commit", "-m", "initial")
	writeSkillEditFile(t, repo, ".git/hooks/pre-commit", "#!/bin/sh\n[ \"$AGENT_NAME\" = test-agent ] || exit 42\n")
	if err := os.Chmod(filepath.Join(repo, ".git", "hooks", "pre-commit"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeSkillEditFile(t, repo, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: alpha\n---\nnew\n")
	if _, err := sealSkillEdit(skillEditSealOptions{
		RepoRoot: repo,
		Skill:    "alpha",
		Actor:    "test-agent",
	}); err != nil {
		t.Fatalf("sealSkillEdit with guarded commit hook: %v", err)
	}
}

func TestSealSkillEditRejectsCriticalSkillWithoutOverride(t *testing.T) {
	repo := setupSkillEditRepo(t)
	writeSkillEditFile(t, repo, "skills/evolve/SKILL.md", "---\nname: evolve\ndescription: evolve\n---\nold\n")
	writeSkillEditFile(t, repo, "docs/contracts/critical-skills.txt", "evolve\n")
	gitSkillEdit(t, repo, "add", ".")
	gitSkillEdit(t, repo, "commit", "-m", "initial")

	writeSkillEditFile(t, repo, "skills/evolve/SKILL.md", "---\nname: evolve\ndescription: evolve\n---\nnew\n")
	_, err := sealSkillEdit(skillEditSealOptions{
		RepoRoot: repo,
		Skill:    "evolve",
		Actor:    "test-agent",
	})
	if err == nil || !strings.Contains(err.Error(), "critical skill") {
		t.Fatalf("expected critical skill rejection, got %v", err)
	}
	status := gitSkillEdit(t, repo, "status", "--porcelain")
	if !strings.Contains(status, "skills/evolve/SKILL.md") {
		t.Fatalf("expected edit to remain uncommitted, got:\n%s", status)
	}
}

func TestDigestSkillEditsSurfacesRecentSkillChanges(t *testing.T) {
	repo := setupSkillEditRepo(t)
	writeSkillEditFile(t, repo, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: alpha\n---\nold\n")
	writeSkillEditFile(t, repo, "docs/contracts/critical-skills.txt", "alpha\n")
	gitSkillEdit(t, repo, "add", ".")
	gitSkillEdit(t, repo, "commit", "-m", "initial")

	writeSkillEditFile(t, repo, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: alpha\n---\nnew\n")
	_, err := sealSkillEdit(skillEditSealOptions{
		RepoRoot:      repo,
		Skill:         "alpha",
		Actor:         "test-agent",
		AllowCritical: true,
	})
	if err != nil {
		t.Fatalf("sealSkillEdit error: %v", err)
	}

	entries, err := digestSkillEdits(skillEditDigestOptions{RepoRoot: repo, Since: "1 year ago"})
	if err != nil {
		t.Fatalf("digestSkillEdits error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one digest entry")
	}
	if entries[0].Skills[0] != "alpha" {
		t.Fatalf("expected alpha skill in digest, got %+v", entries[0].Skills)
	}
	if !entries[0].Critical {
		t.Fatalf("expected critical marker in digest entry: %+v", entries[0])
	}
}

func TestSkillsEditSealCommandDryRun(t *testing.T) {
	repo := setupSkillEditRepo(t)
	t.Chdir(repo)
	writeSkillEditFile(t, repo, "skills/evolve/SKILL.md", "---\nname: evolve\ndescription: evolve\n---\nold\n")
	writeSkillEditFile(t, repo, "docs/contracts/critical-skills.txt", "evolve\n")
	gitSkillEdit(t, repo, "add", ".")
	gitSkillEdit(t, repo, "commit", "-m", "initial")

	writeSkillEditFile(t, repo, "skills/evolve/SKILL.md", "---\nname: evolve\ndescription: evolve\n---\nnew\n")
	out, err := executeCommand("skills", "edit", "seal", "--skill", "evolve", "--allow-critical", "--dry-run", "--actor", "test-agent")
	if err != nil {
		t.Fatalf("ao skills edit seal dry-run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "DRY-RUN: would commit evolve") {
		t.Fatalf("unexpected dry-run output:\n%s", out)
	}
	status := gitSkillEdit(t, repo, "status", "--porcelain")
	if !strings.Contains(status, "skills/evolve/SKILL.md") {
		t.Fatalf("expected dry-run to leave edit uncommitted, got:\n%s", status)
	}
}

func TestSkillsEditDigestCommandJSON(t *testing.T) {
	repo := setupSkillEditRepo(t)
	t.Chdir(repo)
	writeSkillEditFile(t, repo, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: alpha\n---\nold\n")
	gitSkillEdit(t, repo, "add", ".")
	gitSkillEdit(t, repo, "commit", "-m", "initial")

	writeSkillEditFile(t, repo, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: alpha\n---\nnew\n")
	if _, err := sealSkillEdit(skillEditSealOptions{
		RepoRoot: repo,
		Skill:    "alpha",
		Actor:    "test-agent",
	}); err != nil {
		t.Fatalf("sealSkillEdit error: %v", err)
	}

	out, err := executeCommand("skills", "edit", "digest", "--since", "1 year ago", "--json")
	if err != nil {
		t.Fatalf("ao skills edit digest: %v\n%s", err, out)
	}
	if !strings.Contains(out, "\"skills\"") || !strings.Contains(out, "\"alpha\"") {
		t.Fatalf("expected digest JSON to include alpha skill, got:\n%s", out)
	}
}

func setupSkillEditRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, dir := range []string{"skills/alpha", "skills/evolve", "docs/contracts"} {
		if err := os.MkdirAll(filepath.Join(repo, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitSkillEdit(t, repo, "init")
	gitSkillEdit(t, repo, "config", "user.email", "test@example.com")
	gitSkillEdit(t, repo, "config", "user.name", "Test Agent")
	return repo
}

func writeSkillEditFile(t *testing.T, repo, rel, body string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitSkillEdit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := gitOutput(repo, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}
