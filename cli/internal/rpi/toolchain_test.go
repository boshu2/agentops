package rpi

import (
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(key, "AGENTOPS_RPI_") {
			_ = os.Unsetenv(key)
		}
	}
	os.Exit(m.Run())
}

func TestResolveToolchain_Defaults(t *testing.T) {
	tc, err := ResolveToolchain(ResolveToolchainOptions{})
	if err != nil {
		t.Fatalf("ResolveToolchain() error = %v", err)
	}

	if tc.RuntimeMode != DefaultRuntimeMode {
		t.Fatalf("RuntimeMode = %q, want %q", tc.RuntimeMode, DefaultRuntimeMode)
	}
	if tc.RuntimeCommand != DefaultRuntimeCommand {
		t.Fatalf("RuntimeCommand = %q, want %q", tc.RuntimeCommand, DefaultRuntimeCommand)
	}
	if tc.AOCommand != DefaultAOCommand {
		t.Fatalf("AOCommand = %q, want %q", tc.AOCommand, DefaultAOCommand)
	}
	if tc.BDCommand != DefaultBDCommand {
		t.Fatalf("BDCommand = %q, want %q", tc.BDCommand, DefaultBDCommand)
	}
	if tc.TmuxCommand != DefaultTmuxCommand {
		t.Fatalf("TmuxCommand = %q, want %q", tc.TmuxCommand, DefaultTmuxCommand)
	}
}

func TestResolveToolchain_ConfigOverrides(t *testing.T) {
	tc, err := ResolveToolchain(ResolveToolchainOptions{
		Config: Toolchain{
			RuntimeMode:    "stream",
			RuntimeCommand: "codex",
			AOCommand:      "aox",
			BDCommand:      "bdx",
			TmuxCommand:    "tmuxx",
		},
		EnvLookup: func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("ResolveToolchain() error = %v", err)
	}

	if tc.RuntimeMode != "stream" {
		t.Fatalf("RuntimeMode = %q, want stream", tc.RuntimeMode)
	}
	if tc.RuntimeCommand != "codex" {
		t.Fatalf("RuntimeCommand = %q, want codex", tc.RuntimeCommand)
	}
	if tc.AOCommand != "aox" {
		t.Fatalf("AOCommand = %q, want aox", tc.AOCommand)
	}
	if tc.BDCommand != "bdx" {
		t.Fatalf("BDCommand = %q, want bdx", tc.BDCommand)
	}
	if tc.TmuxCommand != "tmuxx" {
		t.Fatalf("TmuxCommand = %q, want tmuxx", tc.TmuxCommand)
	}
}

func TestResolveToolchain_EnvOverridesConfig(t *testing.T) {
	env := map[string]string{
		"AGENTOPS_RPI_RUNTIME":         "direct",
		"AGENTOPS_RPI_RUNTIME_MODE":    "stream",
		"AGENTOPS_RPI_RUNTIME_COMMAND": "runtime-env",
		"AGENTOPS_RPI_AO_COMMAND":      "ao-env",
		"AGENTOPS_RPI_BD_COMMAND":      "bd-env",
		"AGENTOPS_RPI_TMUX_COMMAND":    "tmux-env",
	}
	tc, err := ResolveToolchain(ResolveToolchainOptions{
		Config: Toolchain{
			RuntimeMode:    "auto",
			RuntimeCommand: "runtime-config",
			AOCommand:      "ao-config",
			BDCommand:      "bd-config",
			TmuxCommand:    "tmux-config",
		},
		EnvLookup: func(k string) string { return env[k] },
	})
	if err != nil {
		t.Fatalf("ResolveToolchain() error = %v", err)
	}

	// AGENTOPS_RPI_RUNTIME_MODE should win over AGENTOPS_RPI_RUNTIME.
	if tc.RuntimeMode != "stream" {
		t.Fatalf("RuntimeMode = %q, want stream", tc.RuntimeMode)
	}
	if tc.RuntimeCommand != "runtime-env" {
		t.Fatalf("RuntimeCommand = %q, want runtime-env", tc.RuntimeCommand)
	}
	if tc.AOCommand != "ao-env" {
		t.Fatalf("AOCommand = %q, want ao-env", tc.AOCommand)
	}
	if tc.BDCommand != "bd-env" {
		t.Fatalf("BDCommand = %q, want bd-env", tc.BDCommand)
	}
	if tc.TmuxCommand != "tmux-env" {
		t.Fatalf("TmuxCommand = %q, want tmux-env", tc.TmuxCommand)
	}
}

func TestResolveToolchain_FlagsOverrideEnv(t *testing.T) {
	env := map[string]string{
		"AGENTOPS_RPI_RUNTIME_MODE": "stream",
		"AGENTOPS_RPI_BD_COMMAND":   "bd-env",
	}
	tc, err := ResolveToolchain(ResolveToolchainOptions{
		EnvLookup: func(k string) string { return env[k] },
		FlagValues: Toolchain{
			RuntimeMode: "direct",
			BDCommand:   "bd-flag",
		},
		FlagSet: ToolchainFlagSet{
			RuntimeMode: true,
			BDCommand:   true,
		},
	})
	if err != nil {
		t.Fatalf("ResolveToolchain() error = %v", err)
	}

	if tc.RuntimeMode != "direct" {
		t.Fatalf("RuntimeMode = %q, want direct", tc.RuntimeMode)
	}
	if tc.BDCommand != "bd-flag" {
		t.Fatalf("BDCommand = %q, want bd-flag", tc.BDCommand)
	}
}

func TestResolveToolchain_InvalidRuntimeMode(t *testing.T) {
	_, err := ResolveToolchain(ResolveToolchainOptions{
		FlagValues: Toolchain{RuntimeMode: "bad-mode"},
		FlagSet:    ToolchainFlagSet{RuntimeMode: true},
		EnvLookup:  func(string) string { return "" },
	})
	if err == nil {
		t.Fatal("expected invalid runtime mode error")
	}
}

func TestNormalizeRuntimeMode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", DefaultRuntimeMode},
		{"  ", DefaultRuntimeMode},
		{"auto", "auto"},
		{"  AUTO  ", "auto"},
		{"Direct", "direct"},
		{"STREAM", "stream"},
		{"tmux", "tmux"},
		{"GC", "gc"},
	}
	for _, tc := range cases {
		got := NormalizeRuntimeMode(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeRuntimeMode(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestValidateRuntimeMode(t *testing.T) {
	validModes := []string{"auto", "direct", "stream", "tmux", "gc", " Auto ", "DIRECT", "TMUX", "GC"}
	for _, m := range validModes {
		if err := ValidateRuntimeMode(m); err != nil {
			t.Errorf("ValidateRuntimeMode(%q) unexpected error: %v", m, err)
		}
	}

	invalidModes := []string{"invalid", "hybrid", "custom"}
	for _, m := range invalidModes {
		if err := ValidateRuntimeMode(m); err == nil {
			t.Errorf("ValidateRuntimeMode(%q) expected error", m)
		}
	}
}

func TestResolveToolchain_AllFlagOverrides(t *testing.T) {
	tc, err := ResolveToolchain(ResolveToolchainOptions{
		Config: Toolchain{
			RuntimeMode:    "stream",
			RuntimeCommand: "config-cmd",
			AOCommand:      "config-ao",
			BDCommand:      "config-bd",
			TmuxCommand:    "config-tmux",
		},
		FlagValues: Toolchain{
			RuntimeMode:    "direct",
			RuntimeCommand: "flag-cmd",
			AOCommand:      "flag-ao",
			BDCommand:      "flag-bd",
			TmuxCommand:    "flag-tmux",
		},
		FlagSet: ToolchainFlagSet{
			RuntimeMode:    true,
			RuntimeCommand: true,
			AOCommand:      true,
			BDCommand:      true,
			TmuxCommand:    true,
		},
		EnvLookup: func(k string) string {
			// Even with env set, flags should win
			return "env-value"
		},
	})
	if err != nil {
		t.Fatalf("ResolveToolchain() error = %v", err)
	}
	if tc.RuntimeMode != "direct" {
		t.Errorf("RuntimeMode = %q, want direct", tc.RuntimeMode)
	}
	if tc.RuntimeCommand != "flag-cmd" {
		t.Errorf("RuntimeCommand = %q, want flag-cmd", tc.RuntimeCommand)
	}
	if tc.AOCommand != "flag-ao" {
		t.Errorf("AOCommand = %q, want flag-ao", tc.AOCommand)
	}
	if tc.BDCommand != "flag-bd" {
		t.Errorf("BDCommand = %q, want flag-bd", tc.BDCommand)
	}
	if tc.TmuxCommand != "flag-tmux" {
		t.Errorf("TmuxCommand = %q, want flag-tmux", tc.TmuxCommand)
	}
}

func TestResolveToolchain_EmptyCommandNormalization(t *testing.T) {
	// Empty command values should fall back to defaults
	tc, err := ResolveToolchain(ResolveToolchainOptions{
		Config: Toolchain{
			RuntimeCommand: "  ",
			AOCommand:      "",
		},
		EnvLookup: func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("ResolveToolchain() error = %v", err)
	}
	if tc.RuntimeCommand != DefaultRuntimeCommand {
		t.Errorf("RuntimeCommand = %q, want %q (default)", tc.RuntimeCommand, DefaultRuntimeCommand)
	}
	if tc.AOCommand != DefaultAOCommand {
		t.Errorf("AOCommand = %q, want %q (default)", tc.AOCommand, DefaultAOCommand)
	}
}

func TestNormalizeCommand(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		fallback string
		want     string
	}{
		{"empty returns fallback", "", "default-cmd", "default-cmd"},
		{"whitespace returns fallback", "   ", "default-cmd", "default-cmd"},
		{"non-empty returns trimmed value", "  my-cmd  ", "default-cmd", "my-cmd"},
		{"normal value", "ao", "default-cmd", "ao"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCommand(tc.value, tc.fallback)
			if got != tc.want {
				t.Errorf("normalizeCommand(%q, %q) = %q, want %q", tc.value, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestResolveToolchain_WhitespaceOnlyFlagValues(t *testing.T) {
	// Whitespace-only flag values should be normalized to defaults
	tc, err := ResolveToolchain(ResolveToolchainOptions{
		FlagValues: Toolchain{
			RuntimeCommand: "   ",
			AOCommand:      "   ",
			BDCommand:      "   ",
			TmuxCommand:    "   ",
		},
		FlagSet: ToolchainFlagSet{
			RuntimeCommand: true,
			AOCommand:      true,
			BDCommand:      true,
			TmuxCommand:    true,
		},
		EnvLookup: func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("ResolveToolchain() error = %v", err)
	}
	if tc.RuntimeCommand != DefaultRuntimeCommand {
		t.Errorf("RuntimeCommand = %q, want %q", tc.RuntimeCommand, DefaultRuntimeCommand)
	}
	if tc.AOCommand != DefaultAOCommand {
		t.Errorf("AOCommand = %q, want %q", tc.AOCommand, DefaultAOCommand)
	}
	if tc.BDCommand != DefaultBDCommand {
		t.Errorf("BDCommand = %q, want %q", tc.BDCommand, DefaultBDCommand)
	}
	if tc.TmuxCommand != DefaultTmuxCommand {
		t.Errorf("TmuxCommand = %q, want %q", tc.TmuxCommand, DefaultTmuxCommand)
	}
}

func TestResolveToolchain_NilEnvLookupUsesOsGetenv(t *testing.T) {
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "stream")

	// nil EnvLookup should use os.Getenv.
	tc, err := ResolveToolchain(ResolveToolchainOptions{})
	if err != nil {
		t.Fatalf("ResolveToolchain() with nil EnvLookup: %v", err)
	}
	if tc.RuntimeMode != "stream" {
		t.Errorf("RuntimeMode = %q, want stream", tc.RuntimeMode)
	}
}
