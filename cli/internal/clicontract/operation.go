package clicontract

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

const operationAnnotation = "agentops.dev/operation-contract.v1"

var ErrOperationContractAttached = errors.New("operation contract already attached")

// Effect is the command-side spelling of the effect vocabulary shared with
// generated skill contracts. Read and mutation effects are deliberately
// distinct: a caller must be able to determine whether --dry-run is relevant
// without guessing from a broad "filesystem" label.
type Effect string

const (
	EffectNone             Effect = "none"
	EffectFilesystemRead   Effect = "filesystem.read"
	EffectFilesystemWrite  Effect = "filesystem.write"
	EffectProcessStart     Effect = "process.start"
	EffectNetworkRead      Effect = "network.read"
	EffectNetworkWrite     Effect = "network.write"
	EffectEnvironmentRead  Effect = "environment.read"
	EffectEnvironmentWrite Effect = "environment.write"
	EffectClockRead        Effect = "clock.read"
	EffectCredentialSwitch Effect = "credential.switch"
	EffectExternalMutate   Effect = "external.mutate"
	EffectRuntimeSession   Effect = "runtime.session"
	EffectHostConfigure    Effect = "host.configure"
)

// OutputFormat is the wire format a command can emit on stdout.
type OutputFormat string

const (
	FormatText  OutputFormat = "text"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
	FormatJSONL OutputFormat = "jsonl"
)

// DryRunPolicy describes what a command does when the inherited --dry-run flag
// is set. "not-applicable" is valid only when the command declares neither a
// mutation nor a process-start effect.
type DryRunPolicy string

const (
	DryRunNotApplicable DryRunPolicy = "not-applicable"
	DryRunSuppresses    DryRunPolicy = "suppresses-effects"
	DryRunRejects       DryRunPolicy = "rejects"
)

// OperationContract is the runtime-facing effect and output half of a command
// contract. It is intentionally independent from positional-argument and exit
// policy so it can be projected into the future skill compiler without making
// that compiler depend on Cobra validators.
type OperationContract struct {
	Effects  []Effect       `json:"effects"`
	Outputs  []OutputFormat `json:"outputs"`
	DryRun   DryRunPolicy   `json:"dry_run"`
	ReadOnly bool           `json:"read_only"`
}

func ValidateOperationContract(contract OperationContract) error {
	if len(contract.Effects) == 0 {
		return errors.New("operation contract has no effects")
	}
	if len(contract.Outputs) == 0 {
		return errors.New("operation contract has no outputs")
	}
	if !uniqueEffects(contract.Effects) {
		return errors.New("operation contract has duplicate or unknown effects")
	}
	if len(contract.Effects) > 1 {
		for _, effect := range contract.Effects {
			if effect == EffectNone {
				return errors.New("operation contract mixes none with observed effects")
			}
		}
	}
	if !uniqueOutputs(contract.Outputs) {
		return errors.New("operation contract has duplicate or unknown outputs")
	}
	controlled := RequiresDryRunControl(contract)
	switch contract.DryRun {
	case DryRunNotApplicable:
		if controlled {
			return errors.New("effectful operation cannot declare dry-run not-applicable")
		}
	case DryRunSuppresses, DryRunRejects:
		if !controlled {
			return errors.New("read-only operation cannot declare an effectful dry-run policy")
		}
	default:
		return fmt.Errorf("operation contract has invalid dry-run policy %q", contract.DryRun)
	}
	if contract.ReadOnly && hasMutation(contract.Effects) {
		return errors.New("read-only operation declares a mutation effect")
	}
	return nil
}

// AttachOperation adds validated operation metadata to a runnable command.
func AttachOperation(command *cobra.Command, contract OperationContract) error {
	if command == nil {
		return errors.New("cannot attach operation contract to nil command")
	}
	if command.Run == nil && command.RunE == nil {
		return errors.New("cannot attach operation contract to non-runnable command")
	}
	if err := ValidateOperationContract(contract); err != nil {
		return err
	}
	if command.Annotations == nil {
		command.Annotations = make(map[string]string)
	} else if _, exists := command.Annotations[operationAnnotation]; exists {
		return ErrOperationContractAttached
	}
	encoded, err := json.Marshal(normalizeOperationContract(contract))
	if err != nil {
		return fmt.Errorf("encode operation contract: %w", err)
	}
	command.Annotations[operationAnnotation] = string(encoded)
	return nil
}

func OperationFor(command *cobra.Command) (OperationContract, bool) {
	if command == nil || command.Annotations == nil {
		return OperationContract{}, false
	}
	raw, ok := command.Annotations[operationAnnotation]
	if !ok {
		return OperationContract{}, false
	}
	var contract OperationContract
	if err := json.Unmarshal([]byte(raw), &contract); err != nil {
		return OperationContract{}, false
	}
	if err := ValidateOperationContract(contract); err != nil {
		return OperationContract{}, false
	}
	return contract, true
}

func RequiresDryRunControl(contract OperationContract) bool {
	for _, effect := range contract.Effects {
		switch effect {
		case EffectFilesystemWrite, EffectProcessStart, EffectNetworkWrite,
			EffectEnvironmentWrite, EffectCredentialSwitch, EffectExternalMutate,
			EffectRuntimeSession, EffectHostConfigure:
			return true
		}
	}
	return false
}

func SupportsOutput(contract OperationContract, format OutputFormat) bool {
	for _, supported := range contract.Outputs {
		if supported == format {
			return true
		}
	}
	return false
}

func normalizeOperationContract(contract OperationContract) OperationContract {
	normalized := contract
	normalized.Effects = append([]Effect(nil), contract.Effects...)
	normalized.Outputs = append([]OutputFormat(nil), contract.Outputs...)
	sort.Slice(normalized.Effects, func(i, j int) bool { return normalized.Effects[i] < normalized.Effects[j] })
	sort.Slice(normalized.Outputs, func(i, j int) bool { return normalized.Outputs[i] < normalized.Outputs[j] })
	return normalized
}

func hasMutation(effects []Effect) bool {
	for _, effect := range effects {
		switch effect {
		case EffectFilesystemWrite, EffectNetworkWrite, EffectEnvironmentWrite,
			EffectCredentialSwitch, EffectExternalMutate, EffectRuntimeSession,
			EffectHostConfigure:
			return true
		}
	}
	return false
}

func uniqueEffects(effects []Effect) bool {
	seen := make(map[Effect]struct{}, len(effects))
	for _, effect := range effects {
		switch effect {
		case EffectNone, EffectFilesystemRead, EffectFilesystemWrite, EffectProcessStart,
			EffectNetworkRead, EffectNetworkWrite, EffectEnvironmentRead,
			EffectEnvironmentWrite, EffectClockRead, EffectCredentialSwitch,
			EffectExternalMutate, EffectRuntimeSession, EffectHostConfigure:
		default:
			return false
		}
		if _, exists := seen[effect]; exists {
			return false
		}
		seen[effect] = struct{}{}
	}
	return true
}

func uniqueOutputs(outputs []OutputFormat) bool {
	seen := make(map[OutputFormat]struct{}, len(outputs))
	for _, output := range outputs {
		switch output {
		case FormatText, FormatJSON, FormatYAML, FormatJSONL:
		default:
			return false
		}
		if _, exists := seen[output]; exists {
			return false
		}
		seen[output] = struct{}{}
	}
	return true
}
