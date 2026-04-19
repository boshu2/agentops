package quality

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFileForDeprecatedCommands_FlagsStaleUse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "hook.sh")
	content := "#!/usr/bin/env bash\nao settings notebook update\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	refs := ScanFileForDeprecatedCommands(path)
	if len(refs) != 1 {
		t.Fatalf("want 1 stale ref, got %d: %+v", len(refs), refs)
	}
	if refs[0].OldCommand != "ao settings notebook" {
		t.Errorf("OldCommand=%q, want %q", refs[0].OldCommand, "ao settings notebook")
	}
	if refs[0].NewCommand != "ao notebook" {
		t.Errorf("NewCommand=%q, want %q", refs[0].NewCommand, "ao notebook")
	}
}

func TestScanFileForDeprecatedCommands_SkipsRenameArrow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	content := "## v2.0\n- Renamed: `ao settings notebook update` → `ao notebook update`\n- Renamed: `ao flywheel status` -> `ao metrics flywheel status`\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	refs := ScanFileForDeprecatedCommands(path)
	if len(refs) != 0 {
		t.Errorf("rename-doc lines should be exempt; got %d refs: %+v", len(refs), refs)
	}
}

func TestCheckStaleReferences_AggregatesMatches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.sh")
	b := filepath.Join(dir, "b.sh")
	if err := os.WriteFile(a, []byte("ao know forge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("ao work rpi status\nao know inject\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	check := CheckStaleReferences([]string{filepath.Join(dir, "*.sh")})
	if check.Status != "warn" {
		t.Fatalf("status=%q, want warn", check.Status)
	}
	// 2 files, 3 stale references total — detail should surface both counts.
	if !containsAll(check.Detail, "3 stale", "2 file") {
		t.Errorf("detail %q missing expected counts", check.Detail)
	}
}

func TestCheckStaleReferences_EmptyWhenClean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.sh"), []byte("ao forge transcript\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	check := CheckStaleReferences([]string{filepath.Join(dir, "*.sh")})
	if check.Status != "pass" {
		t.Errorf("status=%q, want pass", check.Status)
	}
}

func TestCountUniqueFiles(t *testing.T) {
	t.Parallel()
	refs := []StaleReference{
		{File: "a.sh", OldCommand: "ao know forge"},
		{File: "a.sh", OldCommand: "ao know inject"},
		{File: "b.sh", OldCommand: "ao work rpi"},
	}
	if got := CountUniqueFiles(refs); got != 2 {
		t.Errorf("CountUniqueFiles = %d, want 2", got)
	}
	if got := CountUniqueFiles(nil); got != 0 {
		t.Errorf("CountUniqueFiles(nil) = %d, want 0", got)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func TestIsRenameDocLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line string
		want bool
	}{
		{"`ao settings notebook` → `ao notebook`", true},
		{"`ao foo` -> `ao bar`", true},
		{"ao settings notebook update", false},
		{"->", false},
		{"plain text with no arrow", false},
	}
	for _, c := range cases {
		if got := isRenameDocLine(c.line); got != c.want {
			t.Errorf("isRenameDocLine(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}
