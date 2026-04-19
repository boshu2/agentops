package quality

import (
	"os"
	"path/filepath"
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
