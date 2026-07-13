package capabilities

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	capabilitiesapp "github.com/boshu2/agentops/cli/internal/capabilities"
	"github.com/boshu2/agentops/cli/internal/clicontract"
)

type recordingBuilder struct {
	calls int
}

func (builder *recordingBuilder) Build() capabilitiesapp.Document {
	builder.calls++
	return capabilitiesapp.Document{
		SchemaVersion: capabilitiesapp.ContractVersion,
		Tool:          "ao",
		Platform:      capabilitiesapp.Platform{OS: "test-os", Arch: "test-arch"},
	}
}

func TestCommandDelegatesAndRendersJSON(t *testing.T) {
	builder := &recordingBuilder{}
	module := NewModule(builder, func() string { return "json" })
	command := module.Command()
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if builder.calls != 1 {
		t.Fatalf("Build calls = %d, want 1", builder.calls)
	}
	var document capabilitiesapp.Document
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("JSON output: %v\n%s", err, output.String())
	}
	if document.Platform.OS != "test-os" {
		t.Fatalf("document = %+v", document)
	}
}

func TestCommandDelegatesAndRendersYAML(t *testing.T) {
	builder := &recordingBuilder{}
	command := NewModule(builder, func() string { return "yaml" }).Command()
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if builder.calls != 1 || !strings.Contains(output.String(), "schemaversion: \"1.1\"") {
		t.Fatalf("YAML output = %q, calls=%d", output.String(), builder.calls)
	}
}

func TestModuleOwnsFreshCommandsAndExplicitContract(t *testing.T) {
	module := NewModule(&recordingBuilder{}, nil)
	if module.Command() == module.Command() {
		t.Fatal("Command returned shared Cobra state")
	}
	contract := module.Contract()
	if err := clicontract.ValidateContract(contract); err != nil {
		t.Fatal(err)
	}
	wantProfiles := clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined
	if contract.Profiles != wantProfiles || contract.Effects != clicontract.EffectPure || contract.Output != clicontract.OutputStructured {
		t.Fatalf("contract = %+v", contract)
	}
}
