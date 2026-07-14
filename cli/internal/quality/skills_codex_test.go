package quality

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// setHome points os.UserHomeDir() at dir on every platform for the duration of
// the test. On POSIX Go resolves the home directory from $HOME, but on Windows
// it reads %USERPROFILE% and ignores HOME entirely — so a HOME-only setenv
// silently misses on the Windows runners and the production checks scan the real
// (fixture-free) home. Setting both keeps these tests platform-independent.
func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	}
}

func writeSkill(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeNativeManifest(t *testing.T, home string, names ...string) (string, string) {
	t.Helper()
	root := CodexNativePluginRootPath(home)
	skills := filepath.Join(root, "skills-codex")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	items := make([]string, 0, len(names))
	for _, name := range names {
		writeSkill(t, skills, name)
		items = append(items, fmt.Sprintf(`{"name":%q}`, name))
	}
	manifest := filepath.Join(skills, ".agentops-manifest.json")
	if err := os.WriteFile(manifest, []byte(`{"skills":[`+strings.Join(items, ",")+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := SHA256File(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return root, hash
}

func writeInstallMeta(t *testing.T, home, root, version, hash string, count int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"install_mode":"native-plugin","plugin_root":%q,"version":%q,"manifest_hash":%q,"skill_count":%d}`,
		root, version, hash, count)
	if err := os.WriteFile(CodexInstallMetaPath(home), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSkillsReportsInstallGuidanceWhenEmpty(t *testing.T) {
	setHome(t, t.TempDir())
	check := CheckSkills()
	// The empty-home hint is platform-specific: POSIX points at install.sh,
	// Windows at install-codex.ps1 (pluginInstallHint switches on GOOS). Assert
	// against the production hint itself so the test is platform-independent —
	// a literal "install.sh" substring silently fails on the Windows runners.
	if check.Status != "warn" || !strings.Contains(check.Detail, pluginInstallHint()) {
		t.Fatalf("check = %+v", check)
	}
}

func TestCheckSkillsValidatesNativeManifest(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	root, hash := writeNativeManifest(t, home, "research")
	writeInstallMeta(t, home, root, "v1", hash, 1)
	check := CheckSkills()
	if check.Status != "pass" || !strings.Contains(check.Detail, "native manifest OK") {
		t.Fatalf("valid native install = %+v", check)
	}

	writeInstallMeta(t, home, root, "v1", "deadbeef", 1)
	check = CheckSkills()
	if check.Status != "warn" || !strings.Contains(check.Detail, "manifest hash does not match") {
		t.Fatalf("drifted native install = %+v", check)
	}
}

func TestCheckSkillsCountsCompatibilityPointerPackages(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	root := CodexNativePluginRootPath(home)
	skills := filepath.Join(root, "skills-codex")
	writeSkill(t, skills, "research")

	pointerDir := filepath.Join(skills, "pre-mortem")
	if err := os.MkdirAll(pointerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pointerDir, "SKILL.md"), []byte("---\nname: pre-mortem\nimplementation: false\nredirect_to: premortem\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.Join(skills, ".agentops-manifest.json")
	if err := os.WriteFile(manifest, []byte(`{"package_count":2,"skills":[{"name":"research"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := SHA256File(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeInstallMeta(t, home, root, "v1", hash, 2)

	check := CheckSkills()
	if check.Status != "pass" || !strings.Contains(check.Detail, "native manifest OK") {
		t.Fatalf("native install with compatibility pointer = %+v", check)
	}
}

func TestCheckSkillsWarnsForOverlappingInstallLayouts(t *testing.T) {
	for _, test := range []struct {
		name, duplicateRoot, detail string
	}{
		{name: "raw Codex", duplicateRoot: filepath.Join(".codex", "skills"), detail: "duplicate raw Codex install"},
		{name: "user skills", duplicateRoot: filepath.Join(".agents", "skills"), detail: "duplicate raw skill install"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			setHome(t, home)
			root, hash := writeNativeManifest(t, home, "research")
			writeInstallMeta(t, home, root, "v1", hash, 1)
			writeSkill(t, filepath.Join(home, test.duplicateRoot), "research")
			check := CheckSkills()
			if check.Status != "warn" || !strings.Contains(check.Detail, test.detail) || !strings.Contains(check.Detail, "research") {
				t.Fatalf("check = %+v", check)
			}
		})
	}
}

func setupCodexSyncRepo(t *testing.T) (string, string, string) {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		if output, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	manifest := filepath.Join(repo, "skills-codex", ".agentops-manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"skills":[{"name":"research"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "fixture"}} {
		if output, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	versionBytes, err := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := SHA256File(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return repo, strings.TrimSpace(string(versionBytes)), hash
}

func TestCheckCodexSyncDetectsMatchManifestDriftAndStaleVersion(t *testing.T) {
	for _, test := range []struct {
		name, version, hash, status, detail string
	}{
		{name: "match", status: "pass", detail: "matches repo"},
		{name: "manifest drift", hash: "deadbeef", status: "warn", detail: "manifest differs from repo"},
		{name: "stale version", version: "oldsha", hash: "deadbeef", status: "warn", detail: "refresh-codex-local.sh"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, current, currentHash := setupCodexSyncRepo(t)
			t.Chdir(repo)
			home := t.TempDir()
			setHome(t, home)
			version, hash := test.version, test.hash
			if version == "" {
				version = current
			}
			if hash == "" {
				hash = currentHash
			}
			writeInstallMeta(t, home, CodexNativePluginRootPath(home), version, hash, 1)
			check := CheckCodexSync()
			if check.Status != test.status || !strings.Contains(check.Detail, test.detail) {
				t.Fatalf("check = %+v", check)
			}
		})
	}
}

func TestCheckSkillIntegrityAbsentCleanAndFindings(t *testing.T) {
	for _, test := range []struct {
		name, script, status, detail string
	}{
		{name: "absent", status: "warn", detail: "not installed"},
		{name: "clean", script: "#!/bin/sh\nexit 0\n", status: "pass", detail: "passed"},
		{name: "findings", script: "#!/bin/sh\necho '[DEAD_REF] skill: broken'\nexit 1\n", status: "warn", detail: "1 skill hygiene finding"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			setHome(t, t.TempDir())
			if test.script != "" {
				path := filepath.Join(root, "skills", "heal-skill", "scripts", "heal.sh")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(test.script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			check := CheckSkillIntegrity()
			if check.Status != test.status || check.Required || !strings.Contains(check.Detail, test.detail) {
				t.Fatalf("check = %+v", check)
			}
		})
	}
}
