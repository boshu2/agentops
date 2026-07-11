package cliapp

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

type testModule struct {
	contract clicontract.CommandContract
	name     string
	aliases  []string
	calls    int
}

func (module *testModule) Contract() clicontract.CommandContract {
	return module.contract
}

func (module *testModule) Command() *cobra.Command {
	module.calls++
	return &cobra.Command{Use: module.name, Aliases: append([]string(nil), module.aliases...)}
}

func testContract(id string, profiles clicontract.ProfileSet) clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       id,
		Profiles: profiles,
		Args: clicontract.ArgsPolicy{
			Name:     "no-args",
			Validate: cobra.NoArgs,
		},
		Output:  clicontract.OutputText,
		Effects: clicontract.EffectPure,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

func TestBuildRootSelectsDeclaredProfileBeforeAssembly(t *testing.T) {
	defaultModule := &testModule{
		contract: testContract("ao.status", clicontract.ProfileDefault|clicontract.ProfileCombined),
		name:     "status",
	}
	flywheelModule := &testModule{
		contract: testContract("ao.defrag", clicontract.ProfileFlywheel|clicontract.ProfileCombined),
		name:     "defrag",
	}

	root, err := BuildRoot(ProfileDefault, flywheelModule, defaultModule)
	if err != nil {
		t.Fatalf("BuildRoot() error = %v", err)
	}
	if flywheelModule.calls != 0 {
		t.Fatalf("excluded module factory called %d times", flywheelModule.calls)
	}
	if defaultModule.calls != 1 {
		t.Fatalf("selected module factory called %d times", defaultModule.calls)
	}
	commands := root.Commands()
	if len(commands) != 1 || commands[0].Name() != "status" {
		t.Fatalf("root commands = %v, want [status]", commandNames(commands))
	}
	attached, ok := clicontract.ContractFor(commands[0])
	if !ok || attached.ID != "ao.status" {
		t.Fatalf("selected command contract = %+v, %v", attached, ok)
	}
}

func TestBuildRootReturnsFreshIndependentTrees(t *testing.T) {
	module := &testModule{
		contract: testContract("ao.status", clicontract.ProfileDefault),
		name:     "status",
	}
	first, err := BuildRoot(ProfileDefault, module)
	if err != nil {
		t.Fatalf("first BuildRoot() error = %v", err)
	}
	second, err := BuildRoot(ProfileDefault, module)
	if err != nil {
		t.Fatalf("second BuildRoot() error = %v", err)
	}
	if first == second || first.Commands()[0] == second.Commands()[0] {
		t.Fatal("BuildRoot() reused a root or command pointer")
	}
	first.Commands()[0].Use = "mutated"
	if second.Commands()[0].Use != "status" {
		t.Fatal("mutating one tree changed another")
	}
}

func TestBuildRootRejectsInvalidOrConflictingModulesBeforeRegistration(t *testing.T) {
	t.Run("invalid profile", func(t *testing.T) {
		module := &testModule{contract: testContract("ao.status", clicontract.ProfileDefault), name: "status"}
		if _, err := BuildRoot(Profile("bogus"), module); err == nil {
			t.Fatal("BuildRoot() accepted invalid profile")
		}
		if module.calls != 0 {
			t.Fatal("invalid profile invoked a module factory")
		}
	})

	t.Run("duplicate ID", func(t *testing.T) {
		first := &testModule{contract: testContract("ao.same", clicontract.ProfileDefault), name: "one"}
		second := &testModule{contract: testContract("ao.same", clicontract.ProfileDefault), name: "two"}
		if _, err := BuildRoot(ProfileDefault, first, second); err == nil || !strings.Contains(err.Error(), "duplicate command ID") {
			t.Fatalf("BuildRoot() error = %v, want duplicate command ID", err)
		}
		if first.calls != 0 || second.calls != 0 {
			t.Fatal("duplicate IDs invoked command factories")
		}
	})

	for name, pair := range map[string][2]*testModule{
		"duplicate path": {
			{contract: testContract("ao.one", clicontract.ProfileDefault), name: "same"},
			{contract: testContract("ao.two", clicontract.ProfileDefault), name: "same"},
		},
		"alias conflicts with path": {
			{contract: testContract("ao.one", clicontract.ProfileDefault), name: "one", aliases: []string{"two"}},
			{contract: testContract("ao.two", clicontract.ProfileDefault), name: "two"},
		},
		"duplicate alias": {
			{contract: testContract("ao.one", clicontract.ProfileDefault), name: "one", aliases: []string{"shared"}},
			{contract: testContract("ao.two", clicontract.ProfileDefault), name: "two", aliases: []string{"shared"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildRoot(ProfileDefault, pair[0], pair[1]); err == nil {
				t.Fatal("BuildRoot() accepted conflicting modules")
			}
		})
	}
}

func TestBuildRootIsDeterministicAcrossModuleOrder(t *testing.T) {
	alpha := &testModule{contract: testContract("ao.alpha", clicontract.ProfileDefault), name: "alpha"}
	zulu := &testModule{contract: testContract("ao.zulu", clicontract.ProfileDefault), name: "zulu"}
	root, err := BuildRoot(ProfileDefault, zulu, alpha)
	if err != nil {
		t.Fatalf("BuildRoot() error = %v", err)
	}
	got := commandNames(root.Commands())
	if strings.Join(got, ",") != "alpha,zulu" {
		t.Fatalf("command order = %v, want [alpha zulu]", got)
	}
}

func commandNames(commands []*cobra.Command) []string {
	names := make([]string, len(commands))
	for index, command := range commands {
		names[index] = command.Name()
	}
	return names
}
