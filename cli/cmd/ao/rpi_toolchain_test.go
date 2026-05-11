// practices: [agile-manifesto, dora-metrics]
package main

import (
	"testing"
)

func TestRpiToolchain_FlagSetAllFalse(t *testing.T) {
	fs := rpiToolchainFlagSet{}
	if fs.RuntimeMode || fs.RuntimeCommand || fs.AOCommand || fs.BDCommand || fs.TmuxCommand {
		t.Error("default rpiToolchainFlagSet should have all false values")
	}
}

func TestRpiToolchain_FlagSetFields(t *testing.T) {
	fs := rpiToolchainFlagSet{
		RuntimeMode:    true,
		RuntimeCommand: true,
		AOCommand:      true,
		BDCommand:      true,
		TmuxCommand:    true,
	}
	if !fs.RuntimeMode {
		t.Error("RuntimeMode should be true")
	}
	if !fs.RuntimeCommand {
		t.Error("RuntimeCommand should be true")
	}
	if !fs.AOCommand {
		t.Error("AOCommand should be true")
	}
	if !fs.BDCommand {
		t.Error("BDCommand should be true")
	}
	if !fs.TmuxCommand {
		t.Error("TmuxCommand should be true")
	}
}

func TestRpiToolchain_ResolveDefaults(t *testing.T) {
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	// resolveRPIToolchainDefaults should not panic and should return
	// a valid toolchain (may fail config load, which is logged as warning).
	tc, err := resolveRPIToolchainDefaults()
	if err != nil {
		t.Fatalf("resolveRPIToolchainDefaults returned error: %v", err)
	}
	// Should have defaults or config-loaded values.
	if tc.RuntimeMode == "" {
		t.Error("RuntimeMode should not be empty")
	}
}
