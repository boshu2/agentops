package doctor

import (
	"context"

	"github.com/boshu2/agentops/cli/internal/quality"
)

type LegacyChecks interface {
	Checks(context.Context) []quality.Check
}

type LegacyService struct {
	version string
	checks  LegacyChecks
}

func NewLegacyService(version string, checks LegacyChecks) LegacyService {
	return LegacyService{version: version, checks: checks}
}

func (service LegacyService) Checks(ctx context.Context) []quality.Check {
	checks := []quality.Check{{Name: "ao CLI", Status: "pass", Detail: quality.FormatVersion(service.version), Required: true}}
	return append(checks, service.checks.Checks(ctx)...)
}
