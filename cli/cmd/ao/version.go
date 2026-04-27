package main

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
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

func init() {
	versionCmd.GroupID = "core"
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
