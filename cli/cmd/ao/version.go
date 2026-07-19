// practices: [continuous-delivery, supply-chain-integrity]
package main

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

type versionInfo struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	Platform  string `json:"platform"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the version, build information, and runtime details.`,
	RunE:  runVersion,
}

// versionContract declares version's real behavior: it accepts (and ignores)
// arbitrary positional args exactly as Cobra does today, emits text (JSON under
// -o json), is a pure read of build/runtime metadata, and exits 0 on success or
// 1 on an output-encoding failure.
func versionContract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.version",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectPure,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

func init() {
	versionCmd.GroupID = "core"
	if err := clicontract.Attach(versionCmd, versionContract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) error {
	info := currentVersionInfo()
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ao version %s\n", info.Version)
	fmt.Fprintf(cmd.OutOrStdout(), "  Go version: %s\n", info.GoVersion)
	fmt.Fprintf(cmd.OutOrStdout(), "  Platform: %s\n", info.Platform)
	return nil
}

func currentVersionInfo() versionInfo {
	return versionInfo{
		Version:   version,
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}
