package clicontract

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestOperationContractRoundTripAndDryRunControl(t *testing.T) {
	command := &cobra.Command{Use: "write", RunE: func(*cobra.Command, []string) error { return nil }}
	contract := OperationContract{
		Effects:  []Effect{EffectFilesystemRead, EffectFilesystemWrite},
		Outputs:  []OutputFormat{FormatText, FormatJSON},
		DryRun:   DryRunSuppresses,
		ReadOnly: false,
	}
	if err := AttachOperation(command, contract); err != nil {
		t.Fatalf("AttachOperation() error = %v", err)
	}
	got, ok := OperationFor(command)
	if !ok {
		t.Fatal("OperationFor() did not recover attached contract")
	}
	if !RequiresDryRunControl(got) || !SupportsOutput(got, FormatJSON) {
		t.Fatalf("operation contract lost semantics: %+v", got)
	}
	if err := AttachOperation(command, contract); !errors.Is(err, ErrOperationContractAttached) {
		t.Fatalf("second AttachOperation() error = %v, want ErrOperationContractAttached", err)
	}
}

func TestOperationContractRejectsDishonestPolicies(t *testing.T) {
	tests := map[string]OperationContract{
		"mutation called not applicable": {
			Effects: []Effect{EffectFilesystemWrite}, Outputs: []OutputFormat{FormatText},
			DryRun: DryRunNotApplicable,
		},
		"read called suppressing": {
			Effects: []Effect{EffectFilesystemRead}, Outputs: []OutputFormat{FormatText},
			DryRun: DryRunSuppresses, ReadOnly: true,
		},
		"read-only mutation": {
			Effects: []Effect{EffectFilesystemWrite}, Outputs: []OutputFormat{FormatText},
			DryRun: DryRunRejects, ReadOnly: true,
		},
		"unknown effect": {
			Effects: []Effect{"filesystem.maybe"}, Outputs: []OutputFormat{FormatText},
			DryRun: DryRunNotApplicable,
		},
		"unknown output": {
			Effects: []Effect{EffectFilesystemRead}, Outputs: []OutputFormat{"protobuf"},
			DryRun: DryRunNotApplicable, ReadOnly: true,
		},
	}
	for name, contract := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateOperationContract(contract); err == nil {
				t.Fatal("ValidateOperationContract() error = nil")
			}
		})
	}
}

func TestSkillCompatibleMutationVocabularyRequiresDryRunControl(t *testing.T) {
	for _, effect := range []Effect{
		EffectFilesystemWrite,
		EffectProcessStart,
		EffectNetworkWrite,
		EffectEnvironmentWrite,
		EffectCredentialSwitch,
		EffectExternalMutate,
		EffectRuntimeSession,
		EffectHostConfigure,
	} {
		t.Run(string(effect), func(t *testing.T) {
			contract := OperationContract{
				Effects: []Effect{effect}, Outputs: []OutputFormat{FormatText},
				DryRun: DryRunRejects,
			}
			if err := ValidateOperationContract(contract); err != nil {
				t.Fatalf("shared effect token rejected: %v", err)
			}
			if !RequiresDryRunControl(contract) {
				t.Fatal("mutation/process effect does not require dry-run control")
			}
		})
	}
}
