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

// TestVersion_ReleaseFallbackHasNoPrereleaseMarker guards the release-cut
// invariant that the checked-in `version` fallback (what `go install` builds
// self-report, since they carry no ldflags-injected tag) is a clean release
// string, never a pre-release like "3.3.0-rc". Release-readiness audit
// (docs/audits/release-readiness-3.3-2026-07-20.md, minor 1) flagged a stale
// "-rc" fallback shipping against 3.3.0 manifests. ldflags-injected
// git-describe strings (e.g. "v3.2.0-901-g<sha>-dirty") are exempt: they are
// build-injected, not the checked-in release fallback.
func TestVersion_ReleaseFallbackHasNoPrereleaseMarker(t *testing.T) {
	// git-describe strings carry a "-g<hash>" commit segment or a "-dirty"
	// suffix; the checked-in release fallback does not.
	if strings.Contains(version, "-g") || strings.HasSuffix(version, "-dirty") {
		t.Skipf("version %q is an ldflags-injected build-describe string, not the checked-in fallback", version)
	}
	for _, marker := range []string{"-rc", "-alpha", "-beta", "-pre", "-dev", "-snapshot"} {
		if strings.Contains(version, marker) {
			t.Errorf("release fallback version %q carries pre-release marker %q; a tagged release fallback must be a clean MAJOR.MINOR.PATCH", version, marker)
		}
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
