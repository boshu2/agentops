// practices: [continuous-delivery, supply-chain-integrity]
package main

import (
	"strings"
	"testing"
)

// version.go was carved into internal/commands/version; the command-behavior
// tests moved with it. These tests remain in package main because they exercise
// root-level wiring the module does not own: the build-time `version` string
// var (set via ldflags) and rootCmd.Version powering `ao --version`.

func TestVersion_VersionVariableHasDefault(t *testing.T) {
	// The version variable should have a default value at build time.
	// In test context it will be the source fallback.
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
		t.Error("version command should be registered on rootCmd")
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
