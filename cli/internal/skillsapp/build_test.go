package skillsapp

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildScaffoldAndReceipt(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	if err := os.Mkdir(filepath.Join(root, "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	reportDir := t.TempDir()
	reportDir, _ = filepath.EvalSymlinks(reportDir)
	opts := BuildOptions{Repo: root, Mode: "from-scratch", Slug: "status-adapter", InitOnly: true, Report: filepath.Join(reportDir, "build.json")}
	report, err := Build(opts, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.FilesCreated) != 1 || report.AuthoringState != "scaffold" || report.SemanticsEvaluated || report.StructureCheckPass {
		t.Fatalf("misleading report: %+v", report)
	}
	if _, err = os.Stat(filepath.Join(root, "skills", opts.Slug, "scripts")); !os.IsNotExist(err) {
		t.Fatal("unnecessary helper directory")
	}
	info, err := os.Stat(opts.Report)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("receipt permissions %v %v", info, err)
	}
	if _, err = Build(opts, io.Discard); err == nil {
		t.Fatal("overwrote existing package/report")
	}
	data, _ := os.ReadFile(filepath.Join(root, "skills", opts.Slug, "SKILL.md"))
	if !strings.Contains(string(data), "authoring_state: scaffold") {
		t.Fatal("missing explicit incomplete state")
	}
}

func TestBuildRefusesUnsafeInputsBeforeMutation(t *testing.T) {
	for _, tc := range []struct{ name, slug, mode, source, report string }{
		{"traversal", "../outside", "from-scratch", "", ""},
		{"template traversal", "valid", "from-template", "../outside", ""},
		{"external missing", "valid", "absorb-external", "/nonexistent/source", ""},
		{"workspace report", "valid", "from-scratch", "", "inside"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			root, _ = filepath.EvalSymlinks(root)
			if err := os.Mkdir(filepath.Join(root, "skills"), 0755); err != nil {
				t.Fatal(err)
			}
			report := tc.report
			if report != "" {
				report = filepath.Join(root, "receipt.json")
			}
			if _, err := Build(BuildOptions{Repo: root, Mode: tc.mode, Slug: tc.slug, Source: tc.source, Report: report, InitOnly: true}, io.Discard); err == nil {
				t.Fatal("unsafe request accepted")
			}
			entries, _ := os.ReadDir(filepath.Join(root, "skills"))
			if len(entries) != 0 {
				t.Fatal("mutated before rejection")
			}
		})
	}
}

func TestBuildRejectsGitReportStorageBeforeMutation(t *testing.T) {
	for _, kind := range []string{"ordinary", "linked-worktree", "bare", "git-internals", "linked-administration", "active-objects"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "skills"), 0700); err != nil {
				t.Fatal(err)
			}
			storage := t.TempDir()
			storage, err = filepath.EvalSymlinks(storage)
			if err != nil {
				t.Fatal(err)
			}
			mkdir := func(path string) {
				t.Helper()
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, value string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "ordinary":
				mkdir(filepath.Join(storage, ".git"))
			case "linked-worktree":
				write(filepath.Join(storage, ".git"), "gitdir: /opaque/main/.git/worktrees/example\n")
			case "bare":
				mkdir(filepath.Join(storage, "objects"))
				mkdir(filepath.Join(storage, "refs"))
				write(filepath.Join(storage, "HEAD"), "ref: refs/heads/main\n")
			case "git-internals":
				storage = filepath.Join(storage, ".git")
				mkdir(filepath.Join(storage, "objects"))
				mkdir(filepath.Join(storage, "refs"))
			case "linked-administration":
				write(filepath.Join(storage, "HEAD"), "ref: refs/heads/main\n")
				write(filepath.Join(storage, "commondir"), "../..\n")
			case "active-objects":
				t.Setenv("GIT_OBJECT_DIRECTORY", storage)
			}
			for _, parent := range []string{storage, filepath.Join(storage, "proof")} {
				mkdir(parent)
				report := filepath.Join(parent, "build.json")
				_, err := Build(BuildOptions{Repo: root, Mode: "from-scratch", Slug: "blocked-report", Report: report, InitOnly: true}, io.Discard)
				if err == nil {
					t.Fatalf("accepted report in %s", kind)
				}
				if _, err := os.Lstat(report); !os.IsNotExist(err) {
					t.Fatalf("report created before rejection: %v", err)
				}
				entries, err := os.ReadDir(filepath.Join(root, "skills"))
				if err != nil || len(entries) != 0 {
					t.Fatalf("source mutated before report rejection: %v %v", entries, err)
				}
			}
		})
	}
}
