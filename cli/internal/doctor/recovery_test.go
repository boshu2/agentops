package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func recordedBackup(t *testing.T, ra *RunArtifact, index int) string {
	t.Helper()
	data, err := os.ReadFile(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	var action struct {
		Path       string `json:"path"`
		BackupPath string `json:"backup_path"`
	}
	if err := json.Unmarshal([]byte(strings.Split(strings.TrimSpace(string(data)), "\n")[index]), &action); err != nil {
		t.Fatal(err)
	}
	if action.BackupPath == "" {
		return filepath.Join(ra.BackupsDir(), action.Path)
	}
	return filepath.Join(ra.RunDir, action.BackupPath)
}

func recordedBackupForPath(t *testing.T, ra *RunArtifact, path string) string {
	t.Helper()
	records, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	for i, rec := range records {
		if filepath.ToSlash(rec.Path) == path {
			return recordedBackup(t, ra, i)
		}
	}
	t.Fatalf("no action for %s", path)
	return ""
}

func requireFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s content=%q err=%v, want %q", path, got, err, want)
	}
}

func TestRecoveryTwoSkillFixersRestoreOriginal(t *testing.T) {
	repo, home := t.TempDir(), t.TempDir()
	path := filepath.Join(repo, "skills", "sample", "SKILL.md")
	original := "---\nname: sample\ndescription: sample\ntier: core\n---\nUse `ao know forge`.\n"
	writeSkillsFile(t, path, original)
	writeSkillsFile(t, filepath.Join(repo, "skills", "sample", "references", "detail.md"), "detail\n")
	report, err := Fix(Options{RepoRoot: repo, CWD: repo, HomeDir: home, Only: []string{"fm-skills-stale-command-refs", "fm-skills-integrity-hygiene"}})
	if err != nil || report.ActionsTaken != 2 {
		t.Fatalf("Fix=%+v err=%v, want two mutations", report, err)
	}
	result, err := Undo(repo, filepath.Base(report.RunDir), true, false)
	if err != nil || result.Restored != 2 {
		t.Errorf("Undo=%+v err=%v, want two restorations", result, err)
	}
	requireFileContent(t, path, original)
}

func TestRecoveryHomeBackupsStayInTheirRun(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "dev", "repo")
	path := filepath.Join(home, ".codex", ".agentops-codex-install.json")
	writeSkillsFile(t, path, "original")
	var runs []*RunArtifact
	for i, content := range []string{"first", "second"} {
		ra, err := NewRunArtifact(repo, "testsha", time.Unix(1_700_000_000+int64(i), 0))
		if err != nil {
			t.Fatal(err)
		}
		af, err := ra.OpenActionsFile()
		if err != nil {
			t.Fatal(err)
		}
		ctx := NewMutateContext(ra, NewCapabilities("test"), home, NewLockManager(filepath.Join(repo, ".doctor", "locks")), af, false)
		_, err = Mutate(ctx, path, WriteFile{Content: []byte(content), Mode: 0o600})
		if closeErr := af.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err != nil {
			t.Fatal(err)
		}
		runs = append(runs, ra)
		backup := recordedBackup(t, ra, 0)
		rel, err := filepath.Rel(ra.RunDir, backup)
		if err != nil || !filepath.IsLocal(rel) {
			t.Errorf("backup %s escapes run %s", backup, ra.RunDir)
		}
	}
	if recordedBackup(t, runs[0], 0) == recordedBackup(t, runs[1], 0) {
		t.Error("separate runs share a backup")
	}
	for i, want := range []string{"first", "original"} {
		result, err := Undo(repo, runs[1-i].RunID, true, false)
		if err != nil || result.Restored != 1 {
			t.Errorf("Undo=%+v err=%v", result, err)
		}
		requireFileContent(t, path, want)
	}
}

func TestRecoveryRenamePreservesRecreatedPath(t *testing.T) {
	for _, kind := range []string{"file", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			repo, home := t.TempDir(), t.TempDir()
			ctx, ra, closeRun := skillsTestCtx(t, repo, home)
			defer closeRun()
			path := filepath.Join(repo, ".agents", "ao", "record")
			dest := filepath.Join(ra.RunDir, "quarantine", "record")
			if kind == "directory" {
				writeSkillsFile(t, filepath.Join(path, "original"), "original")
				if err := workspaceDirRename(ctx, path, dest, nil); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeSkillsFile(t, path, "original")
				if _, err := Mutate(ctx, path, Rename{To: dest}); err != nil {
					t.Fatal(err)
				}
				if kind == "symlink" {
					if err := os.Symlink("absent", path); err != nil {
						t.Fatal(err)
					}
				} else {
					writeSkillsFile(t, path, "new user content")
				}
			}
			result, err := Undo(repo, ra.RunID, true, false)
			if err == nil || result.ExitCode != ExitFixFailed || result.Restored != 0 {
				t.Errorf("Undo=%+v err=%v, want conflict", result, err)
			}
			switch kind {
			case "directory":
				requireFileContent(t, filepath.Join(dest, "original"), "original")
				entries, readErr := os.ReadDir(path)
				if readErr != nil || len(entries) != 0 {
					t.Fatalf("new empty directory changed: entries=%v err=%v", entries, readErr)
				}
			case "symlink":
				link, readErr := os.Readlink(path)
				if readErr != nil || link != "absent" {
					t.Fatalf("new symlink changed: target=%s err=%v", link, readErr)
				}
				requireFileContent(t, dest, "original")
			default:
				requireFileContent(t, path, "new user content")
				requireFileContent(t, dest, "original")
			}
		})
	}
}

func TestRecoveryCorruptBackupDoesNotOverwriteLiveFile(t *testing.T) {
	for _, strict := range []bool{true, false} {
		t.Run(map[bool]string{true: "strict", false: "best-effort"}[strict], func(t *testing.T) {
			repo, home := t.TempDir(), t.TempDir()
			ctx, ra, closeRun := skillsTestCtx(t, repo, home)
			defer closeRun()
			path := filepath.Join(repo, ".agents", "ao", "record")
			writeSkillsFile(t, path, "original")
			if _, err := Mutate(ctx, path, WriteFile{Content: []byte("fixed")}); err != nil {
				t.Fatal(err)
			}
			writeSkillsFile(t, recordedBackup(t, ra, 0), "corrupt")
			result, err := Undo(repo, ra.RunID, strict, false)
			if err == nil || result.Restored != 0 {
				t.Errorf("Undo=%+v err=%v, want corrupt backup rejected", result, err)
			}
			requireFileContent(t, path, "fixed")
		})
	}
}

func TestRecoveryLegacyBackups(t *testing.T) {
	for _, variant := range []string{"valid", "overwritten", "escaped", "symlink"} {
		t.Run(variant, func(t *testing.T) {
			repo, home := t.TempDir(), t.TempDir()
			ctx, ra, closeRun := skillsTestCtx(t, repo, home)
			path := filepath.Join(repo, ".agents", "ao", "record")
			writeSkillsFile(t, path, "original")
			if _, err := Mutate(ctx, path, WriteFile{Content: []byte("intermediate")}); err != nil {
				t.Fatal(err)
			}
			if _, err := Mutate(ctx, path, WriteFile{Content: []byte("final")}); err != nil {
				t.Fatal(err)
			}
			closeRun()
			records, err := readActions(ra.ActionsPath())
			if err != nil {
				t.Fatal(err)
			}
			// Derive the legacy on-disk shape from actual production actions.
			// A single legacy action remains usable. Repeated legacy writes
			// shared the last backup; the original hash then no longer matches.
			if variant != "overwritten" {
				records = records[:1]
			}
			legacy := filepath.Join(ra.BackupsDir(), records[0].Path)
			writeSkillsFile(t, legacy, "original")
			if variant == "overwritten" {
				writeSkillsFile(t, legacy, "intermediate")
			}
			if variant == "escaped" {
				records[0].Path = "../../outside"
			}
			if variant == "symlink" {
				if err := os.Remove(legacy); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(recordedBackup(t, ra, 0), legacy); err != nil {
					t.Fatal(err)
				}
			}
			var journal []byte
			for _, rec := range records {
				rec.BackupPath = ""
				line, err := json.Marshal(rec)
				if err != nil {
					t.Fatal(err)
				}
				journal = append(journal, append(line, '\n')...)
			}
			if err := os.WriteFile(ra.ActionsPath(), journal, 0o600); err != nil {
				t.Fatal(err)
			}
			result, err := Undo(repo, ra.RunID, true, false)
			if variant == "valid" {
				if err != nil || result.Restored != 1 {
					t.Fatalf("legacy Undo=%+v err=%v", result, err)
				}
				requireFileContent(t, path, "original")
			} else {
				if err == nil || result.Restored != 0 {
					t.Fatalf("unsafe legacy Undo=%+v err=%v", result, err)
				}
				requireFileContent(t, path, "final")
			}
		})
	}
}

func TestRecoveryPreservesSnapshotMetadata(t *testing.T) {
	repo, home := t.TempDir(), t.TempDir()
	ctx, ra, closeRun := skillsTestCtx(t, repo, home)
	defer closeRun()
	path := filepath.Join(repo, ".agents", "ao", "record")
	writeSkillsFile(t, path, "original")
	stamp := time.Unix(1_700_000_000, 0)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"intermediate", "final"} {
		if _, err := Mutate(ctx, path, WriteFile{Content: []byte(content), Mode: 0o644}); err != nil {
			t.Fatal(err)
		}
	}
	if recordedBackup(t, ra, 0) == recordedBackup(t, ra, 1) {
		t.Fatal("two actions share a backup")
	}
	requireFileContent(t, recordedBackup(t, ra, 0), "original")
	requireFileContent(t, recordedBackup(t, ra, 1), "intermediate")
	if _, err := Undo(repo, ra.RunID, true, false); err != nil {
		t.Fatal(err)
	}
	requireFileContent(t, path, "original")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 || !info.ModTime().Equal(stamp) {
		t.Fatalf("restored mode=%o mtime=%s", info.Mode().Perm(), info.ModTime())
	}
}

func TestRecoveryBackupDirectoryCannotRedirectMutation(t *testing.T) {
	repo, home, outside := t.TempDir(), t.TempDir(), t.TempDir()
	ctx, ra, closeRun := skillsTestCtx(t, repo, home)
	defer closeRun()
	path := filepath.Join(repo, ".agents", "ao", "record")
	writeSkillsFile(t, path, "original")
	if err := os.Remove(ra.BackupsDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, ra.BackupsDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := Mutate(ctx, path, WriteFile{Content: []byte("fixed")}); err == nil {
		t.Fatal("mutation accepted a redirected backup directory")
	}
	requireFileContent(t, path, "original")
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside entries=%v err=%v, want no escaped backup", entries, err)
	}
	records, err := readActions(ra.ActionsPath())
	if err != nil || len(records) != 0 {
		t.Fatalf("actions=%v err=%v, want no mutation", records, err)
	}
}
