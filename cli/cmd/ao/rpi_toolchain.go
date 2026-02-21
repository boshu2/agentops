package main

import (
	cliConfig "github.com/boshu2/agentops/cli/internal/config"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

type rpiToolchainFlagSet struct {
	RuntimeMode    bool
	RuntimeCommand bool
	AOCommand      bool
	BDCommand      bool
	TmuxCommand    bool
}

func resolveRPIToolchain(flagValues cliRPI.Toolchain, flagSet rpiToolchainFlagSet) (cliRPI.Toolchain, error) {
	cfgValues := cliRPI.Toolchain{}
	cfg, err := cliConfig.Load(nil)
	if err != nil {
		VerbosePrintf("Warning: could not load config for RPI toolchain: %v\n", err)
	} else {
		cfgValues = cliRPI.Toolchain{
			RuntimeMode:    cfg.RPI.RuntimeMode,
			RuntimeCommand: cfg.RPI.RuntimeCommand,
			AOCommand:      cfg.RPI.AOCommand,
			BDCommand:      cfg.RPI.BDCommand,
			TmuxCommand:    cfg.RPI.TmuxCommand,
		}
	}

	return cliRPI.ResolveToolchain(cliRPI.ResolveToolchainOptions{
		Config:     cfgValues,
		FlagValues: flagValues,
		FlagSet: cliRPI.ToolchainFlagSet{
			RuntimeMode:    flagSet.RuntimeMode,
			RuntimeCommand: flagSet.RuntimeCommand,
			AOCommand:      flagSet.AOCommand,
			BDCommand:      flagSet.BDCommand,
			TmuxCommand:    flagSet.TmuxCommand,
		},
	})
}

func resolveRPIToolchainDefaults() (cliRPI.Toolchain, error) {
	return resolveRPIToolchain(cliRPI.Toolchain{}, rpiToolchainFlagSet{})
}
