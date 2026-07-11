package clicontract

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const contractAnnotation = "agentops.dev/clicontract.v1"

var (
	stablePolicyName = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)

	ErrContractAttached = errors.New("command contract already attached")
)

// ProfileSet is an immutable membership mask. Profile selection remains owned
// by cliapp; clicontract only declares where a command is available.
type ProfileSet uint8

const (
	ProfileDefault ProfileSet = 1 << iota
	ProfileFlywheel
	ProfileLegacy
	ProfileCombined

	allProfiles = ProfileDefault | ProfileFlywheel | ProfileLegacy | ProfileCombined
)

// ArgsPolicy gives a positional-argument validator a stable machine name.
type ArgsPolicy struct {
	Name     string
	Validate cobra.PositionalArgs
}

type OutputPolicy string

const (
	OutputUnspecified OutputPolicy = ""
	OutputNone        OutputPolicy = "none"
	OutputText        OutputPolicy = "text"
	OutputStructured  OutputPolicy = "structured"
	OutputStreaming   OutputPolicy = "streaming"
)

type EffectSet uint16

const (
	EffectPure EffectSet = 1 << iota
	EffectFilesystem
	EffectProcess
	EffectNetwork
	EffectTracker
	EffectEnvironment
	EffectClock

	allEffects = EffectPure | EffectFilesystem | EffectProcess | EffectNetwork |
		EffectTracker | EffectEnvironment | EffectClock
)

type ExitClass string

const (
	ExitSuccess     ExitClass = "success"
	ExitFailure     ExitClass = "failure"
	ExitUsage       ExitClass = "usage"
	ExitNotFound    ExitClass = "not-found"
	ExitConflict    ExitClass = "conflict"
	ExitUnavailable ExitClass = "unavailable"
	ExitPartial     ExitClass = "partial"
)

// CommandContract is the dependency-neutral, explicit behavior declaration
// shared by command modules, root assembly, and capability projection.
type CommandContract struct {
	ID          string
	Profiles    ProfileSet
	Args        ArgsPolicy
	Output      OutputPolicy
	Effects     EffectSet
	ExitClasses map[int]ExitClass
}

type attachedContract struct {
	ID          string            `json:"id"`
	Profiles    ProfileSet        `json:"profiles"`
	Args        string            `json:"args"`
	Output      OutputPolicy      `json:"output"`
	Effects     EffectSet         `json:"effects"`
	ExitClasses map[int]ExitClass `json:"exit_classes"`
}

func ValidateContract(contract CommandContract) error {
	if !stablePolicyName.MatchString(contract.ID) {
		return fmt.Errorf("invalid command contract ID %q", contract.ID)
	}
	if contract.Profiles == 0 || contract.Profiles&^allProfiles != 0 {
		return fmt.Errorf("command contract %s has invalid profiles %#x", contract.ID, contract.Profiles)
	}
	if !stablePolicyName.MatchString(contract.Args.Name) || contract.Args.Validate == nil {
		return fmt.Errorf("command contract %s has incomplete Args policy", contract.ID)
	}
	switch contract.Output {
	case OutputNone, OutputText, OutputStructured, OutputStreaming:
	default:
		return fmt.Errorf("command contract %s has invalid output policy %q", contract.ID, contract.Output)
	}
	if contract.Effects == 0 || contract.Effects&^allEffects != 0 {
		return fmt.Errorf("command contract %s has invalid effects %#x", contract.ID, contract.Effects)
	}
	if contract.Effects&EffectPure != 0 && contract.Effects != EffectPure {
		return fmt.Errorf("command contract %s mixes pure with stateful effects", contract.ID)
	}
	if len(contract.ExitClasses) == 0 {
		return fmt.Errorf("command contract %s has no exit classes", contract.ID)
	}
	if contract.ExitClasses[0] != ExitSuccess {
		return fmt.Errorf("command contract %s must declare exit 0 as success", contract.ID)
	}
	for code, class := range contract.ExitClasses {
		if code < 0 || code > 255 {
			return fmt.Errorf("command contract %s has invalid exit code %d", contract.ID, code)
		}
		if !validExitClass(class) {
			return fmt.Errorf("command contract %s has invalid exit class %q", contract.ID, class)
		}
		if code != 0 && class == ExitSuccess {
			return fmt.Errorf("command contract %s maps nonzero exit %d to success", contract.ID, code)
		}
	}
	return nil
}

func Attach(command *cobra.Command, contract CommandContract) error {
	if command == nil {
		return errors.New("cannot attach command contract to nil command")
	}
	if err := ValidateContract(contract); err != nil {
		return err
	}
	if err := validateAliases(command); err != nil {
		return fmt.Errorf("command contract %s: %w", contract.ID, err)
	}
	if command.Annotations != nil {
		if _, exists := command.Annotations[contractAnnotation]; exists {
			return ErrContractAttached
		}
	} else {
		command.Annotations = make(map[string]string)
	}

	encoded, err := json.Marshal(attachedContract{
		ID:          contract.ID,
		Profiles:    contract.Profiles,
		Args:        contract.Args.Name,
		Output:      contract.Output,
		Effects:     contract.Effects,
		ExitClasses: cloneExitClasses(contract.ExitClasses),
	})
	if err != nil {
		return fmt.Errorf("encode command contract %s: %w", contract.ID, err)
	}
	validator := contract.Args.Validate
	command.Args = func(cmd *cobra.Command, args []string) error {
		return validator(cmd, args)
	}
	command.Annotations[contractAnnotation] = string(encoded)
	return nil
}

func ContractFor(command *cobra.Command) (CommandContract, bool) {
	if command == nil || command.Annotations == nil || command.Args == nil {
		return CommandContract{}, false
	}
	raw, ok := command.Annotations[contractAnnotation]
	if !ok {
		return CommandContract{}, false
	}
	var attached attachedContract
	if err := json.Unmarshal([]byte(raw), &attached); err != nil {
		return CommandContract{}, false
	}
	contract := CommandContract{
		ID:          attached.ID,
		Profiles:    attached.Profiles,
		Args:        ArgsPolicy{Name: attached.Args, Validate: command.Args},
		Output:      attached.Output,
		Effects:     attached.Effects,
		ExitClasses: cloneExitClasses(attached.ExitClasses),
	}
	if ValidateContract(contract) != nil {
		return CommandContract{}, false
	}
	return contract, true
}

// ProjectContract overlays explicit behavior metadata onto the existing
// capabilities record while preserving Cobra presentation and flag fields.
func ProjectContract(record Command, contract CommandContract) (Command, error) {
	if err := ValidateContract(contract); err != nil {
		return Command{}, err
	}
	record.ID = contract.ID
	record.Args = contract.Args.Name
	record.Output = string(contract.Output)
	record.Effects = contract.Effects.String()
	record.ExitCodes = make(map[string]string, len(contract.ExitClasses))
	for code, class := range contract.ExitClasses {
		record.ExitCodes[strconv.Itoa(code)] = string(class)
	}
	return record, nil
}

func (effects EffectSet) String() string {
	labels := make([]string, 0, 7)
	for _, item := range []struct {
		mask EffectSet
		name string
	}{
		{EffectPure, "pure"},
		{EffectFilesystem, "filesystem"},
		{EffectProcess, "process"},
		{EffectNetwork, "network"},
		{EffectTracker, "tracker"},
		{EffectEnvironment, "environment"},
		{EffectClock, "clock"},
	} {
		if effects&item.mask != 0 {
			labels = append(labels, item.name)
		}
	}
	return strings.Join(labels, ",")
}

func validExitClass(class ExitClass) bool {
	switch class {
	case ExitSuccess, ExitFailure, ExitUsage, ExitNotFound, ExitConflict, ExitUnavailable, ExitPartial:
		return true
	default:
		return false
	}
}

func cloneExitClasses(classes map[int]ExitClass) map[int]ExitClass {
	cloned := make(map[int]ExitClass, len(classes))
	for code, class := range classes {
		cloned[code] = class
	}
	return cloned
}

func validateAliases(command *cobra.Command) error {
	seen := map[string]struct{}{command.Name(): {}}
	aliases := append([]string(nil), command.Aliases...)
	sort.Strings(aliases)
	for _, alias := range aliases {
		if alias == "" || strings.TrimSpace(alias) != alias || strings.ContainsAny(alias, " \t\r\n") {
			return fmt.Errorf("invalid alias %q", alias)
		}
		if _, exists := seen[alias]; exists {
			return fmt.Errorf("conflicting alias %q", alias)
		}
		seen[alias] = struct{}{}
	}
	return nil
}
