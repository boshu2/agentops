package rpi

import (
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultRuntimeMode controls backend selection when no overrides are provided.
	DefaultRuntimeMode = "auto"
	// DefaultRuntimeCommand is the default runtime process command.
	DefaultRuntimeCommand = "claude"
	// DefaultAOCommand is the default ao CLI command.
	DefaultAOCommand = "ao"
	// DefaultBDCommand is the default bd CLI command.
	DefaultBDCommand = "bd"
	// DefaultTmuxCommand is the default tmux command.
	DefaultTmuxCommand = "tmux"
)

// Toolchain contains the effective command configuration used by RPI.
type Toolchain struct {
	RuntimeMode    string
	RuntimeCommand string
	AOCommand      string
	BDCommand      string
	TmuxCommand    string
}

// ToolchainFlagSet tracks which fields were explicitly set by command-line flags.
type ToolchainFlagSet struct {
	RuntimeMode    bool
	RuntimeCommand bool
	AOCommand      bool
	BDCommand      bool
	TmuxCommand    bool
}

// ResolveToolchainOptions controls deterministic toolchain resolution.
type ResolveToolchainOptions struct {
	// Config contains values loaded from config files.
	Config Toolchain
	// FlagValues contains command-line values.
	FlagValues Toolchain
	// FlagSet indicates which FlagValues were explicitly set by the user.
	FlagSet ToolchainFlagSet
	// EnvLookup returns environment variable values; defaults to os.Getenv.
	EnvLookup func(string) string
}

// NormalizeRuntimeMode canonicalizes runtime mode values.
func NormalizeRuntimeMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return DefaultRuntimeMode
	}
	return normalized
}

// ValidateRuntimeMode validates the runtime mode domain.
func ValidateRuntimeMode(mode string) error {
	switch NormalizeRuntimeMode(mode) {
	case "auto", "direct", "stream", "tmux", "gc":
		return nil
	default:
		return fmt.Errorf("invalid runtime %q (valid: auto|direct|stream|tmux|gc)", mode)
	}
}

// applyEnvField sets dest from the named env var (via lookup) when non-empty.
func applyEnvField(dest *string, lookup func(string) string, envKey string) {
	if v := strings.TrimSpace(lookup(envKey)); v != "" {
		*dest = v
	}
}

// applyFlagField sets dest when the flag was explicitly set.
func applyFlagField(dest *string, flagSet bool, flagValue string) {
	if flagSet {
		*dest = flagValue
	}
}

// ResolveToolchain resolves command configuration with precedence:
// flags > env > config > defaults.
func ResolveToolchain(opts ResolveToolchainOptions) (Toolchain, error) {
	lookup := opts.EnvLookup
	if lookup == nil {
		lookup = os.Getenv
	}

	tc := Toolchain{
		RuntimeMode:    DefaultRuntimeMode,
		RuntimeCommand: DefaultRuntimeCommand,
		AOCommand:      DefaultAOCommand,
		BDCommand:      DefaultBDCommand,
		TmuxCommand:    DefaultTmuxCommand,
	}

	// Layer 1: config overrides defaults
	applyConfigField(&tc.RuntimeMode, opts.Config.RuntimeMode)
	applyConfigField(&tc.RuntimeCommand, opts.Config.RuntimeCommand)
	applyConfigField(&tc.AOCommand, opts.Config.AOCommand)
	applyConfigField(&tc.BDCommand, opts.Config.BDCommand)
	applyConfigField(&tc.TmuxCommand, opts.Config.TmuxCommand)

	// Layer 2: env overrides config
	// AGENTOPS_RPI_RUNTIME is the legacy env var name; AGENTOPS_RPI_RUNTIME_MODE
	// takes precedence because it is applied second (last-write-wins).
	applyEnvField(&tc.RuntimeMode, lookup, "AGENTOPS_RPI_RUNTIME")
	applyEnvField(&tc.RuntimeMode, lookup, "AGENTOPS_RPI_RUNTIME_MODE")
	applyEnvField(&tc.RuntimeCommand, lookup, "AGENTOPS_RPI_RUNTIME_COMMAND")
	applyEnvField(&tc.AOCommand, lookup, "AGENTOPS_RPI_AO_COMMAND")
	applyEnvField(&tc.BDCommand, lookup, "AGENTOPS_RPI_BD_COMMAND")
	applyEnvField(&tc.TmuxCommand, lookup, "AGENTOPS_RPI_TMUX_COMMAND")

	// Layer 3: flags override env
	applyFlagField(&tc.RuntimeMode, opts.FlagSet.RuntimeMode, opts.FlagValues.RuntimeMode)
	applyFlagField(&tc.RuntimeCommand, opts.FlagSet.RuntimeCommand, opts.FlagValues.RuntimeCommand)
	applyFlagField(&tc.AOCommand, opts.FlagSet.AOCommand, opts.FlagValues.AOCommand)
	applyFlagField(&tc.BDCommand, opts.FlagSet.BDCommand, opts.FlagValues.BDCommand)
	applyFlagField(&tc.TmuxCommand, opts.FlagSet.TmuxCommand, opts.FlagValues.TmuxCommand)

	tc.RuntimeMode = NormalizeRuntimeMode(tc.RuntimeMode)
	if err := ValidateRuntimeMode(tc.RuntimeMode); err != nil {
		return Toolchain{}, err
	}

	tc.RuntimeCommand = normalizeCommand(tc.RuntimeCommand, DefaultRuntimeCommand)
	tc.AOCommand = normalizeCommand(tc.AOCommand, DefaultAOCommand)
	tc.BDCommand = normalizeCommand(tc.BDCommand, DefaultBDCommand)
	tc.TmuxCommand = normalizeCommand(tc.TmuxCommand, DefaultTmuxCommand)

	return tc, nil
}

func applyConfigField(dest *string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		*dest = trimmed
	}
}

func normalizeCommand(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
