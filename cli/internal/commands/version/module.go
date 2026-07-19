// Package version owns Cobra presentation for the `ao version` command. The
// module builds its command with host-provided seams (the build-time version
// string and the global output mode) and reads only build/runtime metadata, so
// it performs no filesystem, process, or clock effect. Unlike the other W3
// families, version carries a real CommandContract that the composition
// attaches to the command tree, preserving its capabilities surface.
package version

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

// versionInfo is the machine-readable version document.
type versionInfo struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	Platform  string `json:"platform"`
}

// HostOptions carries the ambient CLI seams the version command reads. The
// build-time version string and the global -o/--output mode are injected here
// so the module reads no package-global build metadata directly.
type HostOptions struct {
	Version    func() string
	OutputMode func() string
}

// Module owns Cobra presentation for the version command.
type Module struct {
	host HostOptions
}

// NewModule constructs the version command module from its host seams.
func NewModule(host HostOptions) Module {
	return Module{host: host}
}

// Contract declares version's real behavior: it accepts (and ignores) arbitrary
// positional args exactly as Cobra does today, emits text (JSON under -o json),
// is a pure read of build/runtime metadata, and exits 0 on success or 1 on an
// output-encoding failure. The composition attaches this contract to the
// command tree, preserving version's declared capabilities surface exactly.
func (Module) Contract() clicontract.CommandContract {
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

// Command builds the `ao version` command.
func (m Module) Command() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Show version information",
		Long:    `Display the version, build information, and runtime details.`,
		GroupID: "core",
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := currentVersionInfo(m.host.Version())
			if m.host.OutputMode() == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ao version %s\n", info.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "  Go version: %s\n", info.GoVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "  Platform: %s\n", info.Platform)
			return nil
		},
	}
}

func currentVersionInfo(version string) versionInfo {
	return versionInfo{
		Version:   version,
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}
