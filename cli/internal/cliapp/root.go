package cliapp

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

type selectedModule struct {
	module   Module
	contract clicontract.CommandContract
	command  *cobra.Command
}

func BuildRoot(profile Profile, modules ...Module) (*cobra.Command, error) {
	profileMask, err := profile.contractProfile()
	if err != nil {
		return nil, err
	}

	selected := make([]selectedModule, 0, len(modules))
	ids := make(map[string]struct{}, len(modules))
	for index, module := range modules {
		if nilModule(module) {
			return nil, fmt.Errorf("module %d is nil", index)
		}
		contract := module.Contract()
		if err := clicontract.ValidateContract(contract); err != nil {
			return nil, fmt.Errorf("module %d: %w", index, err)
		}
		if contract.Profiles&profileMask == 0 {
			continue
		}
		if _, duplicate := ids[contract.ID]; duplicate {
			return nil, fmt.Errorf("duplicate command ID %q", contract.ID)
		}
		ids[contract.ID] = struct{}{}
		selected = append(selected, selectedModule{module: module, contract: contract})
	}

	names := make(map[string]string, len(selected))
	for index := range selected {
		candidate := &selected[index]
		candidate.command = candidate.module.Command()
		if candidate.command == nil {
			return nil, fmt.Errorf("module %s returned a nil command", candidate.contract.ID)
		}
		if candidate.command.Parent() != nil {
			return nil, fmt.Errorf("module %s returned an already-parented command", candidate.contract.ID)
		}
		if candidate.command.Name() == "" {
			return nil, fmt.Errorf("module %s returned a command without a name", candidate.contract.ID)
		}
		if err := clicontract.Attach(candidate.command, candidate.contract); err != nil {
			return nil, fmt.Errorf("module %s: %w", candidate.contract.ID, err)
		}
		for _, name := range append([]string{candidate.command.Name()}, candidate.command.Aliases...) {
			if owner, conflict := names[name]; conflict {
				return nil, fmt.Errorf("command name or alias %q conflicts between %s and %s", name, owner, candidate.contract.ID)
			}
			names[name] = candidate.contract.ID
		}
	}

	sort.Slice(selected, func(left, right int) bool {
		leftName := selected[left].command.Name()
		rightName := selected[right].command.Name()
		if leftName == rightName {
			return selected[left].contract.ID < selected[right].contract.ID
		}
		return leftName < rightName
	})

	root := &cobra.Command{
		Use:           "ao",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	for _, candidate := range selected {
		root.AddCommand(candidate.command)
	}
	return root, nil
}

func nilModule(module Module) bool {
	if module == nil {
		return true
	}
	value := reflect.ValueOf(module)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
