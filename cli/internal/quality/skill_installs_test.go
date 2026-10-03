package quality

import (
	"fmt"
	"os"
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

// writeNativeSkills populates the live Codex plugin cache's skills/ tree, the
// tree `.codex-plugin/plugin.json` ships, and returns the plugin root.
func writeNativeSkills(t *testing.T, home string, names ...string) string {
	t.Helper()
	root := CodexNativePluginRootPath(home)
	skills := filepath.Join(root, "skills")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		writeSkill(t, skills, name)
	}
	return root
}

func writeInstallMeta(t *testing.T, home, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"install_mode":"native-plugin","plugin_root":%q}`, root)
	if err := os.WriteFile(CodexInstallMetaPath(home), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSkillsReportsInstallGuidanceWhenEmpty(t *testing.T) {
	setHome(t, t.TempDir())
	check := CheckSkills()
	// The empty-home hint is platform-specific (pluginInstallHint switches on
	// GOOS). Assert against the production hint itself so the test is
	// platform-independent.
	if check.Status != "warn" || !strings.Contains(check.Detail, pluginInstallHint()) {
		t.Fatalf("check = %+v", check)
	}
}

func TestCheckSkillsReportsNativePluginSkills(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	root := writeNativeSkills(t, home, "research")
	want := "1 skills found in ~/.codex/plugins/cache/agentops-marketplace/agentops/local/skills"
	for _, withMeta := range []bool{false, true} {
		if withMeta {
			writeInstallMeta(t, home, root)
		}
		check := CheckSkills()
		if check.Status != "pass" || check.Detail != want {
			t.Fatalf("native plugin install (install metadata present: %t) = %+v, want pass %q", withMeta, check, want)
		}
	}
}

func TestCheckSkillsCountsCompatibilityPointerPackages(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	root := writeNativeSkills(t, home, "research")

	pointerDir := filepath.Join(root, "skills", "premortem")
	if err := os.MkdirAll(pointerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pointerDir, "SKILL.md"), []byte("---\nname: premortem\nimplementation: false\nredirect_to: premortem\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	check := CheckSkills()
	if check.Status != "pass" || !strings.HasPrefix(check.Detail, "2 skills found in ") {
		t.Fatalf("native install with compatibility pointer = %+v, want 2 packages counted", check)
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
			writeNativeSkills(t, home, "research")
			writeSkill(t, filepath.Join(home, test.duplicateRoot), "research")
			check := CheckSkills()
			if check.Status != "warn" || !strings.Contains(check.Detail, test.detail) || !strings.Contains(check.Detail, "research") {
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
				path := filepath.Join(root, "skills", "skill-builder", "scripts", "heal.sh")
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
