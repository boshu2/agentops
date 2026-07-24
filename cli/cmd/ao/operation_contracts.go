package main

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

var (
	textOutput       = []clicontract.OutputFormat{clicontract.FormatText}
	structuredOutput = []clicontract.OutputFormat{clicontract.FormatText, clicontract.FormatJSON, clicontract.FormatYAML}
	jsonOutput       = []clicontract.OutputFormat{clicontract.FormatJSON}
	jsonYAMLOutput   = []clicontract.OutputFormat{clicontract.FormatJSON, clicontract.FormatYAML}
)

func pureOperation(outputs []clicontract.OutputFormat) clicontract.OperationContract {
	return clicontract.OperationContract{
		Effects: []clicontract.Effect{clicontract.EffectNone},
		Outputs: outputs, DryRun: clicontract.DryRunNotApplicable, ReadOnly: true,
	}
}

func readOperation(outputs []clicontract.OutputFormat, effects ...clicontract.Effect) clicontract.OperationContract {
	return clicontract.OperationContract{
		Effects: effects, Outputs: outputs,
		DryRun: clicontract.DryRunNotApplicable, ReadOnly: true,
	}
}

func readProcessOperation(outputs []clicontract.OutputFormat, effects ...clicontract.Effect) clicontract.OperationContract {
	return clicontract.OperationContract{
		Effects: effects, Outputs: outputs,
		DryRun: clicontract.DryRunRejects, ReadOnly: true,
	}
}

func rejectingOperation(outputs []clicontract.OutputFormat, effects ...clicontract.Effect) clicontract.OperationContract {
	return clicontract.OperationContract{
		Effects: effects, Outputs: outputs,
		DryRun: clicontract.DryRunRejects,
	}
}

func suppressingOperation(outputs []clicontract.OutputFormat, effects ...clicontract.Effect) clicontract.OperationContract {
	return clicontract.OperationContract{
		Effects: effects, Outputs: outputs,
		DryRun: clicontract.DryRunSuppresses,
	}
}

// commandOperationContracts is the exhaustive runtime inventory for every
// runnable command in the shipped Cobra tree. Commands that are only structural
// parents are intentionally absent. A test fails whenever a runnable command is
// added without first declaring its effects, outputs, and --dry-run behavior.
var commandOperationContracts = map[string]clicontract.OperationContract{
	"ao capabilities": pureOperation(jsonYAMLOutput),
	"ao config": readOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectEnvironmentRead),
	"ao demo": pureOperation(textOutput),

	"ao doctor": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart, clicontract.EffectNetworkRead,
		clicontract.EffectEnvironmentRead, clicontract.EffectClockRead),
	"ao doctor capabilities": pureOperation(jsonOutput),
	"ao doctor diff": readProcessOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectProcessStart,
		clicontract.EffectEnvironmentRead, clicontract.EffectClockRead),
	"ao doctor explain": readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao doctor fix": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart, clicontract.EffectNetworkRead,
		clicontract.EffectEnvironmentRead, clicontract.EffectClockRead),
	"ao doctor gc": suppressingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectClockRead),
	"ao doctor health":     readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao doctor ls":         readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao doctor robot-docs": pureOperation(textOutput),
	"ao doctor undo": suppressingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite),

	"ao eval baseline": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectClockRead),
	"ao eval baseline-audit": readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao eval cleanup": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectEnvironmentRead, clicontract.EffectClockRead),
	"ao eval compare": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite),
	"ao eval coverage":         readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao eval outcomes compile": readOperation(jsonOutput, clicontract.EffectFilesystemRead),
	"ao eval outcomes ingest": rejectingOperation(jsonYAMLOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectClockRead),
	"ao eval run": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart, clicontract.EffectEnvironmentRead,
		clicontract.EffectClockRead),
	"ao eval scenario add": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectClockRead),
	"ao eval scenario evaluate": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart, clicontract.EffectClockRead),
	"ao eval scenario init": rejectingOperation(textOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite),
	"ao eval scenario list":     readOperation(jsonOutput, clicontract.EffectFilesystemRead),
	"ao eval scenario validate": readOperation(textOutput, clicontract.EffectFilesystemRead),
	"ao eval scenario-ab": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart, clicontract.EffectEnvironmentRead,
		clicontract.EffectClockRead),
	"ao eval scenario-moat": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectClockRead),
	"ao eval scorecard": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite),
	"ao eval suite n-required": readProcessOperation(structuredOutput,
		clicontract.EffectProcessStart, clicontract.EffectEnvironmentRead),
	"ao eval suite verdict": readProcessOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectProcessStart,
		clicontract.EffectEnvironmentRead),
	"ao eval task add": rejectingOperation(textOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectEnvironmentRead),
	"ao eval task list": readOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectEnvironmentRead),
	"ao eval task run": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart, clicontract.EffectEnvironmentRead,
		clicontract.EffectClockRead),
	"ao eval task show": readOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectEnvironmentRead),

	"ao flywheel compare": readOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectClockRead),
	"ao flywheel status": readOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectClockRead),
	"ao gate check": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart, clicontract.EffectEnvironmentRead,
		clicontract.EffectClockRead),

	"ao goals drift": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectProcessStart,
		clicontract.EffectClockRead),
	"ao goals export": rejectingOperation(jsonYAMLOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectProcessStart,
		clicontract.EffectClockRead),
	"ao goals history": readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao goals measure": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart, clicontract.EffectClockRead),
	"ao goals meta": rejectingOperation([]clicontract.OutputFormat{clicontract.FormatText, clicontract.FormatJSON},
		clicontract.EffectFilesystemRead, clicontract.EffectProcessStart,
		clicontract.EffectClockRead),
	"ao goals render": rejectingOperation(textOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite),
	"ao goals scenarios": readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao goals validate":  readOperation(structuredOutput, clicontract.EffectFilesystemRead),

	"ao init": suppressingOperation(textOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite),
	"ao provenance add": rejectingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectClockRead),
	"ao provenance export": readOperation(
		[]clicontract.OutputFormat{clicontract.FormatJSONL, clicontract.FormatJSON, clicontract.FormatYAML},
		clicontract.EffectFilesystemRead),
	"ao provenance list": readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao provenance mine-session": rejectingOperation(
		[]clicontract.OutputFormat{clicontract.FormatJSONL, clicontract.FormatJSON},
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite),
	"ao provenance position": readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao provenance show":     readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao provenance trace": readOperation(
		structuredOutput,
		clicontract.EffectFilesystemRead),
	"ao provenance verify": readOperation(structuredOutput, clicontract.EffectFilesystemRead),

	"ao quick-start":       pureOperation(textOutput),
	"ao redact":            pureOperation(textOutput),
	"ao robot-docs":        pureOperation(textOutput),
	"ao session bootstrap": readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao session handoff": suppressingOperation(
		[]clicontract.OutputFormat{clicontract.FormatText, clicontract.FormatJSON},
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart, clicontract.EffectClockRead),
	"ao session rehydrate": readOperation(structuredOutput, clicontract.EffectFilesystemRead),

	"ao skills check": readOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectClockRead),
	"ao skills consumers": readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao skills find":      readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao skills graph": readOperation(
		[]clicontract.OutputFormat{clicontract.FormatText, clicontract.FormatJSON},
		clicontract.EffectFilesystemRead),
	"ao skills link": suppressingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectEnvironmentRead),
	"ao skills list":      readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao skills producers": readOperation(structuredOutput, clicontract.EffectFilesystemRead),
	"ao skills resolve": readOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectClockRead),
	"ao skills unlink": suppressingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectEnvironmentRead),
	"ao status": readOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectClockRead),
	"ao version": pureOperation(structuredOutput),
	"ao workflows link": suppressingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart),
	"ao workflows unlink": suppressingOperation(structuredOutput,
		clicontract.EffectFilesystemRead, clicontract.EffectFilesystemWrite,
		clicontract.EffectProcessStart),
}

func applyOperationContracts(root *cobra.Command) error {
	if root == nil {
		return errors.New("cannot apply operation contracts to nil root")
	}
	seen := make(map[string]bool, len(commandOperationContracts))
	var missing []string
	var walk func(*cobra.Command) error
	walk = func(parent *cobra.Command) error {
		for _, command := range parent.Commands() {
			if command.Hidden || command.Name() == "help" || command.Name() == "completion" {
				continue
			}
			isGroupGuard := command.Annotations[groupGuardAnnotation] == "true"
			if command.Runnable() && !isGroupGuard {
				path := command.CommandPath()
				contract, ok := commandOperationContracts[path]
				if !ok {
					missing = append(missing, path)
				} else {
					seen[path] = true
					if attached, exists := clicontract.OperationFor(command); exists {
						if !reflect.DeepEqual(attached, normalizedOperation(contract)) {
							return fmt.Errorf("operation contract drift for %s", path)
						}
					} else if err := clicontract.AttachOperation(command, contract); err != nil {
						return fmt.Errorf("attach operation contract to %s: %w", path, err)
					}
				}
			}
			if err := walk(command); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return err
	}
	for path := range commandOperationContracts {
		if !seen[path] {
			return fmt.Errorf("operation contract names absent or non-runnable command %s", path)
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		return fmt.Errorf("runnable commands missing operation contracts: %s", strings.Join(missing, ", "))
	}
	return nil
}

func normalizedOperation(contract clicontract.OperationContract) clicontract.OperationContract {
	command := &cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) {}}
	if err := clicontract.AttachOperation(command, contract); err != nil {
		panic(err)
	}
	normalized, _ := clicontract.OperationFor(command)
	return normalized
}

func enforceOperationContract(command *cobra.Command) error {
	if command == command.Root() || command.Name() == "help" || command.Name() == "completion" {
		return nil
	}
	if command.Annotations[groupGuardAnnotation] == "true" {
		return nil
	}
	contract, ok := clicontract.OperationFor(command)
	if !ok {
		return operationRefusal(command.CommandPath() + " has no executable effect/output contract")
	}
	if dryRunRequested(command) {
		switch contract.DryRun {
		case clicontract.DryRunSuppresses:
		case clicontract.DryRunRejects:
			return operationRefusal(command.CommandPath() + " does not support --dry-run; no action was executed")
		default:
			if clicontract.RequiresDryRunControl(contract) {
				return operationRefusal(command.CommandPath() + " has an unsafe --dry-run contract; no action was executed")
			}
		}
	}
	if format, requested := requestedOutputFormat(command); requested && !clicontract.SupportsOutput(contract, format) {
		return operationRefusal(fmt.Sprintf("%s does not support %s output", command.CommandPath(), format))
	}
	return nil
}

func operationRefusal(message string) error {
	return &clicontract.ExitError{Code: 1, Message: message, Label: "ao"}
}

func dryRunRequested(command *cobra.Command) bool {
	if booleanFlagRequested(command, "dry-run") {
		return true
	}
	flag := command.Flag("dry-run")
	return flag != nil && flag.Value.String() == "true"
}

func requestedOutputFormat(command *cobra.Command) (clicontract.OutputFormat, bool) {
	if booleanFlagRequested(command, "json") {
		return clicontract.FormatJSON, true
	}
	value, changed := changedFlagValue(command, "output")
	if !changed {
		return "", false
	}
	switch value {
	case "json":
		return clicontract.FormatJSON, true
	case "yaml":
		return clicontract.FormatYAML, true
	default:
		return clicontract.FormatText, true
	}
}

func booleanFlagRequested(command *cobra.Command, name string) bool {
	for _, value := range changedFlagValues(command, name) {
		if value == "true" {
			return true
		}
	}
	return false
}

func changedFlagValue(command *cobra.Command, name string) (string, bool) {
	values := changedFlagValues(command, name)
	if len(values) == 0 {
		return "", false
	}
	return values[len(values)-1], true
}

// changedFlagValues walks the actual command ancestry instead of asking
// command.Flag, because a local flag with the same name shadows a changed root
// persistent flag. Prefix and suffix placement must describe the same request.
func changedFlagValues(command *cobra.Command, name string) []string {
	var values []string
	seen := make(map[*pflag.Flag]bool)
	for node := command; node != nil; node = node.Parent() {
		for _, flags := range []*pflag.FlagSet{node.LocalNonPersistentFlags(), node.PersistentFlags()} {
			flag := flags.Lookup(name)
			if flag == nil || seen[flag] || !flag.Changed {
				continue
			}
			seen[flag] = true
			values = append(values, flag.Value.String())
		}
	}
	return values
}
