// practices: [pragmatic-programmer, design-patterns]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateDetect_GoMod(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectTemplateFromProjectRoot(tmp)
	if got != "go-cli" {
		t.Errorf("detectTemplateFromProjectRoot = %q, want %q", got, "go-cli")
	}
}

func TestTemplateDetect_GoModInCLI(t *testing.T) {
	tmp := t.TempDir()
	cliDir := filepath.Join(tmp, "cli")
	if err := os.MkdirAll(cliDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliDir, "go.mod"), []byte("module test/cli"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectTemplateFromProjectRoot(tmp)
	if got != "go-cli" {
		t.Errorf("detectTemplateFromProjectRoot = %q, want %q", got, "go-cli")
	}
}

func TestTemplateDetect_PackageJSON(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectTemplateFromProjectRoot(tmp)
	if got != "web-app" {
		t.Errorf("detectTemplateFromProjectRoot = %q, want %q", got, "web-app")
	}
}

func TestTemplateDetect_PyprojectToml(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectTemplateFromProjectRoot(tmp)
	if got != "python-lib" {
		t.Errorf("detectTemplateFromProjectRoot = %q, want %q", got, "python-lib")
	}
}

func TestTemplateDetect_CargoToml(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte("[package]"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectTemplateFromProjectRoot(tmp)
	if got != "rust-cli" {
		t.Errorf("detectTemplateFromProjectRoot = %q, want %q", got, "rust-cli")
	}
}

func TestTemplateDetect_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	got := detectTemplateFromProjectRoot(tmp)
	if got != "generic" {
		t.Errorf("detectTemplateFromProjectRoot = %q, want %q", got, "generic")
	}
}

func TestTemplateDetect_GoModTakesPrecedenceOverPackageJSON(t *testing.T) {
	// go.mod is checked first in the switch, so it wins even if package.json exists
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectTemplateFromProjectRoot(tmp)
	if got != "go-cli" {
		t.Errorf("detectTemplateFromProjectRoot = %q, want %q (go.mod should take precedence)", got, "go-cli")
	}
}
