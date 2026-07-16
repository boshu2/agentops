// Package capabilities adapts the Cobra command tree and Go runtime to the
// capabilities application's dependency-neutral ports.
package capabilities

import (
	"runtime"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	capabilitiesapp "github.com/boshu2/agentops/cli/internal/capabilities"
	"github.com/boshu2/agentops/cli/internal/clicontract"
)

type CobraSurface struct {
	root *cobra.Command
}

func NewCobraSurface(root *cobra.Command) CobraSurface {
	return CobraSurface{root: root}
}

func (surface CobraSurface) Snapshot(commandExitCodes map[string]map[string]string) capabilitiesapp.Snapshot {
	groups := map[string]*capabilitiesapp.CommandGroup{}
	var order []string
	for _, group := range surface.root.Groups() {
		groups[group.ID] = &capabilitiesapp.CommandGroup{ID: group.ID, Title: group.Title}
		order = append(order, group.ID)
	}
	const ungrouped = ""
	groups[ungrouped] = &capabilitiesapp.CommandGroup{ID: ungrouped, Title: "Additional Commands:"}
	order = append(order, ungrouped)

	for _, command := range surface.root.Commands() {
		if command.Hidden || command.Name() == "help" || command.Name() == "completion" {
			continue
		}
		group, ok := groups[command.GroupID]
		if !ok {
			group = groups[ungrouped]
		}
		group.Commands = append(group.Commands, capabilitiesapp.Command{Name: command.Name(), Short: command.Short})
	}

	var commandGroups []capabilitiesapp.CommandGroup
	for _, id := range order {
		group := groups[id]
		if len(group.Commands) == 0 {
			continue
		}
		sort.Slice(group.Commands, func(i, j int) bool { return group.Commands[i].Name < group.Commands[j].Name })
		commandGroups = append(commandGroups, *group)
	}

	var globalFlags []capabilitiesapp.Flag
	surface.root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		globalFlags = append(globalFlags, capabilitiesapp.Flag{
			Name: flag.Name, Shorthand: flag.Shorthand, Description: flag.Usage,
		})
	})
	sort.Slice(globalFlags, func(i, j int) bool { return globalFlags[i].Name < globalFlags[j].Name })

	return capabilitiesapp.Snapshot{
		GlobalFlags: globalFlags, CommandGroups: commandGroups,
		Commands: clicontract.Inspect(surface.root, commandExitCodes),
	}
}

type RuntimePlatform struct{}

func (RuntimePlatform) Platform() capabilitiesapp.Platform {
	return capabilitiesapp.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
}
