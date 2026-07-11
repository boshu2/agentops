package capabilities

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

type recordingSurface struct {
	exits map[string]map[string]string
}

func (surface *recordingSurface) Snapshot(exits map[string]map[string]string) Snapshot {
	surface.exits = exits
	return Snapshot{
		GlobalFlags:   []Flag{{Name: "json", Description: "structured output"}},
		CommandGroups: []CommandGroup{{ID: "core", Title: "Core", Commands: []Command{{Name: "capabilities", Short: "contract"}}}},
		Commands:      []clicontract.Command{{ID: "ao.capabilities", Path: "ao capabilities"}},
	}
}

type fixedPlatform struct{}

func (fixedPlatform) Platform() Platform { return Platform{OS: "test-os", Arch: "test-arch"} }

func TestServiceBuildsCompleteContractFromPorts(t *testing.T) {
	surface := &recordingSurface{}
	document := NewService("v-test", surface, fixedPlatform{}).Build()
	if document.SchemaVersion != ContractVersion || document.ContractVersion != ContractVersion {
		t.Fatalf("contract versions = %q/%q, want %q", document.SchemaVersion, document.ContractVersion, ContractVersion)
	}
	if document.Tool != "ao" || document.ToolVersion != "v-test" {
		t.Fatalf("tool identity = %q/%q", document.Tool, document.ToolVersion)
	}
	if document.Platform.OS != "test-os" || document.Platform.Arch != "test-arch" {
		t.Fatalf("platform = %+v", document.Platform)
	}
	if len(document.GlobalFlags) != 1 || len(document.CommandGroups) != 1 || len(document.Commands) != 1 {
		t.Fatalf("surface projection incomplete: %+v", document)
	}
	if surface.exits["plan-pawl decide"]["5"] == "" || document.CommandExitCodes["pawl review"]["3"] == "" {
		t.Fatal("typed command exit codes did not cross the surface port")
	}
	if len(document.OutputFormats) != 3 || document.ExitCodes["0"] != "success" {
		t.Fatal("stable output and exit dictionaries are incomplete")
	}
}
