package doctor

import (
	"context"
	"testing"

	"github.com/boshu2/agentops/cli/internal/quality"
)

type fakeLegacyChecks struct{ checks []quality.Check }

func (fake fakeLegacyChecks) Checks(context.Context) []quality.Check {
	return append([]quality.Check(nil), fake.checks...)
}

func TestLegacyServicePrependsCLIVersionAndPreservesAdapterOrder(t *testing.T) {
	service := NewLegacyService("3.0.0", fakeLegacyChecks{checks: []quality.Check{
		{Name: "CLI Dependencies"}, {Name: "Knowledge Base"}, {Name: "Reviewer: codex"},
	}})
	checks := service.Checks(context.Background())
	if len(checks) != 4 || checks[0].Name != "ao CLI" || checks[0].Detail != "v3.0.0" {
		t.Fatalf("checks = %+v", checks)
	}
	if checks[1].Name != "CLI Dependencies" || checks[3].Name != "Reviewer: codex" {
		t.Fatalf("order changed: %+v", checks)
	}
}
