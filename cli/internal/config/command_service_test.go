package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type fakeCommandGateway struct{}

func (gateway *fakeCommandGateway) Resolve(string, bool) *ResolvedConfig { return &ResolvedConfig{} }
func (gateway *fakeCommandGateway) Files() (ConfigFiles, error)          { return ConfigFiles{}, nil }
func (gateway *fakeCommandGateway) Environment([]string) map[string]string {
	return map[string]string{}
}

// TestCommandServiceShowReportsLegacyReadPaths pins the files-panel half of
// novice edge 8: when the deprecated home fallback is active, ShowResult must
// carry the path actually read so the renderer can label it instead of
// reporting the canonical path as missing.
func TestCommandServiceShowReportsLegacyReadPaths(t *testing.T) {
	clearConfigEnv(t)
	isolateLegacyWarnings(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	legacy := filepath.Join(home, ".agentops", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("output: json\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	result, err := NewCommandService(&fakeCommandGateway{}).Show(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	files := result.ConfigFiles
	if !files.HomeLegacy {
		t.Fatal("HomeLegacy = false, want true when only the legacy home config exists")
	}
	if files.HomeReadPath != legacy {
		t.Errorf("HomeReadPath = %q, want %q", files.HomeReadPath, legacy)
	}
	if files.ProjectLegacy {
		t.Error("ProjectLegacy = true, want false with no project config at all")
	}
}

// TestCommandServiceShowCanonicalReadPath: with the canonical home config in
// place the read path equals it and no legacy label is requested.
func TestCommandServiceShowCanonicalReadPath(t *testing.T) {
	clearConfigEnv(t)
	isolateLegacyWarnings(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	canonical := filepath.Join(home, ".agents", "ao", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(canonical), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(canonical, []byte("output: json\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := NewCommandService(&fakeCommandGateway{}).Show(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.ConfigFiles.HomeLegacy {
		t.Fatal("HomeLegacy = true, want false when the canonical home config exists")
	}
	if result.ConfigFiles.HomeReadPath != canonical {
		t.Errorf("HomeReadPath = %q, want %q", result.ConfigFiles.HomeReadPath, canonical)
	}
}
