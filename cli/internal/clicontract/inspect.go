package clicontract

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func Inspect(root *cobra.Command, commandExits map[string]map[string]string) []Command {
	commands := make([]Command, 0)
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, child := range parent.Commands() {
			if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			path := child.CommandPath()
			record := Command{
				ID:         stableID(path),
				Path:       path,
				Use:        child.Use,
				Short:      child.Short,
				Aliases:    append([]string(nil), child.Aliases...),
				Deprecated: child.Deprecated,
				Args:       argsPolicy(child),
				Output:     "none",
				Effects:    effectsPolicy(child),
				ExitCodes:  exitsFor(path, commandExits),
			}
			if contract, ok := ContractFor(child); ok {
				// ContractFor has already validated the attached metadata, so this
				// projection cannot fail without an internal contract bug.
				record, _ = ProjectContract(record, contract)
			}
			if path == "ao capabilities" {
				record.Output = "structured"
				record.Effects = "pure"
			}
			child.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
				record.Flags = append(record.Flags, flagRecord(flag, "local", child))
			})
			child.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
				record.Flags = append(record.Flags, flagRecord(flag, "inherited", child))
			})
			sort.Slice(record.Flags, func(i, j int) bool {
				if record.Flags[i].Name == record.Flags[j].Name {
					return record.Flags[i].Origin < record.Flags[j].Origin
				}
				return record.Flags[i].Name < record.Flags[j].Name
			})
			commands = append(commands, record)
			walk(child)
		}
	}
	walk(root)
	sort.Slice(commands, func(i, j int) bool { return commands[i].Path < commands[j].Path })
	return commands
}

func stableID(path string) string {
	return strings.ReplaceAll(strings.TrimSpace(path), " ", ".")
}

func argsPolicy(command *cobra.Command) string {
	if command.Run == nil && command.RunE == nil {
		return "subcommands-only"
	}
	if command.Args == nil {
		return "arbitrary"
	}
	return "range"
}

func effectsPolicy(command *cobra.Command) string {
	if command.Run == nil && command.RunE == nil {
		return "pure"
	}
	return "mixed"
}

func exitsFor(path string, commandExits map[string]map[string]string) map[string]string {
	key := strings.TrimPrefix(path, "ao ")
	if exits, ok := commandExits[key]; ok {
		copyOf := make(map[string]string, len(exits))
		for code, meaning := range exits {
			copyOf[code] = meaning
		}
		return copyOf
	}
	return map[string]string{"0": "success", "1": "error"}
}

func flagRecord(flag *pflag.Flag, origin string, command *cobra.Command) Flag {
	required := false
	if annotation := flag.Annotations[cobra.BashCompOneRequiredFlag]; len(annotation) != 0 {
		required = annotation[0] == "true"
	}
	return Flag{Name: flag.Name, Shorthand: flag.Shorthand, Origin: origin, Required: required, Description: flag.Usage}
}
