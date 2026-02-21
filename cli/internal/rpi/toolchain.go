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
	case "auto", "direct", "stream":
		return nil
	default:
		return fmt.Errorf("invalid runtime %q (valid: auto|direct|stream)", mode)
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

	applyConfigField(&tc.RuntimeMode, opts.Config.RuntimeMode)
	applyConfigField(&tc.RuntimeCommand, opts.Config.RuntimeCommand)
	applyConfigField(&tc.AOCommand, opts.Config.AOCommand)
	applyConfigField(&tc.BDCommand, opts.Config.BDCommand)
	applyConfigField(&tc.TmuxCommand, opts.Config.TmuxCommand)

	if envRuntime := strings.TrimSpace(lookup("AGENTOPS_RPI_RUNTIME")); envRuntime != "" {
		tc.RuntimeMode = envRuntime
	}
	if envRuntimeMode := strings.TrimSpace(lookup("AGENTOPS_RPI_RUNTIME_MODE")); envRuntimeMode != "" {
		tc.RuntimeMode = envRuntimeMode
	}
	if envRuntimeCommand := strings.TrimSpace(lookup("AGENTOPS_RPI_RUNTIME_COMMAND")); envRuntimeCommand != "" {
		tc.RuntimeCommand = envRuntimeCommand
	}
	if envAOCommand := strings.TrimSpace(lookup("AGENTOPS_RPI_AO_COMMAND")); envAOCommand != "" {
		tc.AOCommand = envAOCommand
	}
	if envBDCommand := strings.TrimSpace(lookup("AGENTOPS_RPI_BD_COMMAND")); envBDCommand != "" {
		tc.BDCommand = envBDCommand
	}
	if envTmuxCommand := strings.TrimSpace(lookup("AGENTOPS_RPI_TMUX_COMMAND")); envTmuxCommand != "" {
		tc.TmuxCommand = envTmuxCommand
	}

	if opts.FlagSet.RuntimeMode {
		tc.RuntimeMode = opts.FlagValues.RuntimeMode
	}
	if opts.FlagSet.RuntimeCommand {
		tc.RuntimeCommand = opts.FlagValues.RuntimeCommand
	}
	if opts.FlagSet.AOCommand {
		tc.AOCommand = opts.FlagValues.AOCommand
	}
	if opts.FlagSet.BDCommand {
		tc.BDCommand = opts.FlagValues.BDCommand
	}
	if opts.FlagSet.TmuxCommand {
		tc.TmuxCommand = opts.FlagValues.TmuxCommand
	}

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
