package gates

import (
	"context"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestGoCLIArchitectureRoutingSelectsRelevantInducedViolation(t *testing.T) {
	check := GoCLIArchitectureCheck()
	registry := NewRegistry()
	if err := registry.Add(check); err != nil {
		t.Fatal(err)
	}
	orchestrator := testOrch(t, registry, fakeFiles{files: []string{"cli/internal/commands/demo/module.go"}}, map[ports.GateName]ports.GateVerdict{
		"check-go-cli-architecture.sh": {Status: ports.GateStatusFail, Reason: "effect.process"},
	})
	report, err := orchestrator.Run(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Results) != 1 || report.Results[0].Check.ID != check.ID {
		t.Fatalf("induced command violation did not select architecture gate: %+v", report.Results)
	}
	if report.ExitCode() != 1 {
		t.Fatalf("blocking induced violation exit = %d, want 1", report.ExitCode())
	}
}

func TestGoCLIArchitectureRoutingSkipsUnrelatedDocs(t *testing.T) {
	check := GoCLIArchitectureCheck()
	if check.affected([]string{"docs/README.md"}) {
		t.Fatal("unrelated docs edit selected Go CLI architecture gate")
	}
	if !check.affected([]string{"scripts/check-go-cli-architecture.sh"}) {
		t.Fatal("architecture gate does not self-route")
	}
}
