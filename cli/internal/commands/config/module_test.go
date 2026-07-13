package config

import (
	"bytes"
	"context"
	"strings"
	"testing"

	configapp "github.com/boshu2/agentops/cli/internal/config"
)

type fakeUseCases struct {
	showResult   configapp.ShowResult
	modelsResult configapp.ModelsResult
	writeRequest configapp.ModelsWriteRequest
	writeResult  configapp.ModelsWriteResult
}

func (useCases *fakeUseCases) Show(context.Context, string, bool) (configapp.ShowResult, error) {
	return useCases.showResult, nil
}

func (useCases *fakeUseCases) Models(context.Context) (configapp.ModelsResult, error) {
	return useCases.modelsResult, nil
}

func (useCases *fakeUseCases) WriteModels(_ context.Context, request configapp.ModelsWriteRequest) (configapp.ModelsWriteResult, error) {
	useCases.writeRequest = request
	return useCases.writeResult, nil
}

func TestModuleShowJSONUsesCommandWriter(t *testing.T) {
	useCases := &fakeUseCases{showResult: configapp.ShowResult{Resolved: &configapp.ResolvedConfig{}}}
	command := NewModule(useCases, func() string { return "json" }, func() bool { return false }).Command()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"--show"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout.String(), "{\n") || !strings.Contains(stdout.String(), `"output"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestModuleModelsWriteParsesDelegatesAndRenders(t *testing.T) {
	useCases := &fakeUseCases{writeResult: configapp.ModelsWriteResult{Updated: true, DefaultTier: "quality"}}
	command := NewModule(useCases, func() string { return "table" }, func() bool { return false }).Command()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"models", "--set-tier", "quality", "--set-skill", "council=budget"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if useCases.writeRequest.DefaultTier != "quality" || useCases.writeRequest.Skill != "council=budget" {
		t.Fatalf("request = %+v", useCases.writeRequest)
	}
	if got := stdout.String(); got != "Set default model tier to \"quality\"\nSet skill \"council\" tier to \"budget\"\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestModuleContractDeclaresConfigEffects(t *testing.T) {
	contract := (Module{}).Contract()
	if contract.ExitClasses[0] == "" || contract.ExitClasses[1] == "" {
		t.Fatalf("exit classes = %+v", contract.ExitClasses)
	}
	if contract.Effects == 0 {
		t.Fatal("config effects are undeclared")
	}
}
