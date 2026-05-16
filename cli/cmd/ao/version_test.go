// practices: [continuous-delivery, supply-chain-integrity]
package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestVersion_CommandExists(t *testing.T) {
	if versionCmd == nil {
		t.Fatal("versionCmd should not be nil")
	}
	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %q, want %q", versionCmd.Use, "version")
	}
	if versionCmd.GroupID != "core" {
		t.Errorf("versionCmd.GroupID = %q, want %q", versionCmd.GroupID, "core")
	}
}

func TestVersion_VersionVariableHasDefault(t *testing.T) {
	// The version variable should have a default value at build time.
	// In test context it will be "dev".
	if version == "" {
		t.Error("version should not be empty")
	}
}

func TestVersion_RegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "version" {
			found = true
			break
		}
	}
	if !found {
		t.Error("versionCmd should be registered on rootCmd")
	}
}

func TestVersion_RootVersionFlagWired(t *testing.T) {
	// CLI-C2 (soc-nx1o): rootCmd.Version must be set so `ao --version` works,
	// not only the `ao version` subcommand.
	if rootCmd.Version != version {
		t.Errorf("rootCmd.Version = %q, want %q", rootCmd.Version, version)
	}
	out, err := executeCommand("--version")
	if err != nil {
		t.Fatalf("ao --version returned error: %v", err)
	}
	if !strings.Contains(out, version) {
		t.Errorf("ao --version output should contain %q, got: %s", version, out)
	}
}

func TestVersion_ExecuteOutputContainsVersionString(t *testing.T) {
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("ao version returned error: %v", err)
	}
	if !strings.Contains(out, "ao version "+version) {
		t.Errorf("output should contain 'ao version %s', got: %s", version, out)
	}
}

func TestVersion_ExecuteOutputContainsGoVersion(t *testing.T) {
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("ao version returned error: %v", err)
	}
	goVer := runtime.Version()
	if !strings.Contains(out, goVer) {
		t.Errorf("output should contain Go version %q, got: %s", goVer, out)
	}
}

func TestVersion_ExecuteOutputContainsPlatform(t *testing.T) {
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("ao version returned error: %v", err)
	}
	platform := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(out, platform) {
		t.Errorf("output should contain platform %q, got: %s", platform, out)
	}
}

func TestVersion_ExecuteOutputLineCount(t *testing.T) {
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("ao version returned error: %v", err)
	}
	// The version command outputs exactly 3 lines:
	//   ao version <ver>
	//   Go version: <ver>
	//   Platform: <os>/<arch>
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 output lines, got %d: %q", len(lines), out)
	}
}

func TestVersion_DevVersionDefault(t *testing.T) {
	// In test context (no ldflags), version should be "dev".
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("ao version returned error: %v", err)
	}
	if !strings.Contains(out, "ao version dev") {
		t.Errorf("expected default 'ao version dev' in test context, got: %s", out)
	}
}
