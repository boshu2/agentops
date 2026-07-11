package clicontract

import (
	"errors"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func validCommandContract() CommandContract {
	return CommandContract{
		ID:       "ao.lookup",
		Profiles: ProfileDefault | ProfileFlywheel | ProfileCombined,
		Args: ArgsPolicy{
			Name:     "maximum-1",
			Validate: cobra.MaximumNArgs(1),
		},
		Output:  OutputStructured,
		Effects: EffectFilesystem | EffectProcess,
		ExitClasses: map[int]ExitClass{
			0: ExitSuccess,
			1: ExitFailure,
			2: ExitUsage,
		},
	}
}

func TestCommandContractStableIDSurvivesPathAliasMutation(t *testing.T) {
	command := &cobra.Command{Use: "lookup [query]", Aliases: []string{"find"}}
	contract := validCommandContract()
	if err := Attach(command, contract); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}

	contract.ExitClasses[1] = ExitConflict
	command.Use = "search [query]"
	command.Aliases = []string{"seek"}
	got, ok := ContractFor(command)
	if !ok {
		t.Fatal("ContractFor() did not find attached contract")
	}
	if got.ID != contract.ID {
		t.Fatalf("stable ID = %q, want %q", got.ID, contract.ID)
	}
	if got.ExitClasses[1] != ExitFailure {
		t.Fatal("attached contract retained the caller's mutable exit map")
	}
	if err := got.Args.Validate(command, []string{"one", "two"}); err == nil {
		t.Fatal("attached Args policy stopped enforcing maximum-1")
	}
}

func TestCommandContractRequiresExactPolicies(t *testing.T) {
	tests := map[string]func(*CommandContract){
		"empty ID":          func(c *CommandContract) { c.ID = "" },
		"unstable ID":       func(c *CommandContract) { c.ID = "AO Lookup" },
		"missing profile":   func(c *CommandContract) { c.Profiles = 0 },
		"unknown profile":   func(c *CommandContract) { c.Profiles = ProfileSet(1 << 7) },
		"missing Args name": func(c *CommandContract) { c.Args.Name = "" },
		"nil Args":          func(c *CommandContract) { c.Args.Validate = nil },
		"missing output":    func(c *CommandContract) { c.Output = OutputUnspecified },
		"unknown output":    func(c *CommandContract) { c.Output = OutputPolicy("sometimes") },
		"missing effects":   func(c *CommandContract) { c.Effects = 0 },
		"unknown effects":   func(c *CommandContract) { c.Effects = EffectSet(1 << 15) },
		"pure plus effect":  func(c *CommandContract) { c.Effects = EffectPure | EffectNetwork },
		"nil exits":         func(c *CommandContract) { c.ExitClasses = nil },
		"missing success":   func(c *CommandContract) { delete(c.ExitClasses, 0) },
		"unknown exit":      func(c *CommandContract) { c.ExitClasses[1] = ExitClass("shrug") },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			contract := validCommandContract()
			mutate(&contract)
			if err := ValidateContract(contract); err == nil {
				t.Fatal("ValidateContract() error = nil")
			}
		})
	}
	if err := ValidateContract(validCommandContract()); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
}

func TestAttachRejectsInvalidOrDuplicateContract(t *testing.T) {
	if err := Attach(nil, validCommandContract()); err == nil {
		t.Fatal("Attach(nil) error = nil")
	}

	invalid := &cobra.Command{Use: "lookup"}
	contract := validCommandContract()
	contract.Output = OutputUnspecified
	if err := Attach(invalid, contract); err == nil {
		t.Fatal("Attach() accepted invalid contract")
	}

	conflictingAliases := &cobra.Command{Use: "lookup", Aliases: []string{"find", "find"}}
	if err := Attach(conflictingAliases, validCommandContract()); err == nil {
		t.Fatal("Attach() accepted duplicate aliases")
	}

	command := &cobra.Command{Use: "lookup"}
	if err := Attach(command, validCommandContract()); err != nil {
		t.Fatalf("first Attach() error = %v", err)
	}
	if err := Attach(command, validCommandContract()); !errors.Is(err, ErrContractAttached) {
		t.Fatalf("second Attach() error = %v, want ErrContractAttached", err)
	}
}

func TestOldCapabilitiesConsumerAcceptsExplicitProjection(t *testing.T) {
	contract := validCommandContract()
	legacy := Command{
		ID:        "ao.path.derived",
		Path:      "ao lookup",
		Use:       "lookup [query]",
		Short:     "Search the corpus",
		Args:      "range",
		Output:    "none",
		Effects:   "mixed",
		ExitCodes: map[string]string{"0": "success", "1": "error"},
	}

	got, err := ProjectContract(legacy, contract)
	if err != nil {
		t.Fatalf("ProjectContract() error = %v", err)
	}
	if got.ID != contract.ID || got.Args != "maximum-1" || got.Output != "structured" || got.Effects != "filesystem,process" {
		t.Fatalf("explicit projection did not replace inferred policy: %+v", got)
	}
	if got.Path != legacy.Path || got.Use != legacy.Use || got.Short != legacy.Short {
		t.Fatalf("explicit projection changed presentation: %+v", got)
	}
	wantExits := map[string]string{"0": "success", "1": "failure", "2": "usage"}
	if !reflect.DeepEqual(got.ExitCodes, wantExits) {
		t.Fatalf("ExitCodes = %#v, want %#v", got.ExitCodes, wantExits)
	}

	got.ExitCodes["1"] = "mutated"
	if contract.ExitClasses[1] != ExitFailure {
		t.Fatal("projection leaked a mutable exit map back into the contract")
	}
}
