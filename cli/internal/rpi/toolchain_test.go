package rpi

import "testing"

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
