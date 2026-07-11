// Package reviewerhealth owns the shared reviewer catalog and reachability
// policy used by doctor and quick-start.
package reviewerhealth

import (
	"context"
	"time"

	"github.com/boshu2/agentops/cli/internal/quality"
)

type Reviewer struct {
	Name           string
	InstallCommand string
}

type ProbeResult struct {
	Status string
	Detail string
	Live   bool
}

type Probe interface {
	Check(context.Context, Reviewer, time.Duration) ProbeResult
}

type Service struct {
	catalog []Reviewer
	probe   Probe
}

func DefaultCatalog() []Reviewer {
	return []Reviewer{
		{Name: "codex", InstallCommand: "npm install -g @openai/codex && codex login"},
		{Name: "agy", InstallCommand: "install the AGY CLI, then verify with 'agy models'"},
	}
}

func NewService(catalog []Reviewer, probe Probe) Service {
	return Service{catalog: append([]Reviewer(nil), catalog...), probe: probe}
}

func (service Service) Check(ctx context.Context, timeout time.Duration) ([]quality.Check, []string) {
	checks := make([]quality.Check, 0, len(service.catalog))
	var live []string
	for _, reviewer := range service.catalog {
		result := service.probe.Check(ctx, reviewer, timeout)
		checks = append(checks, quality.Check{
			Name: "Reviewer: " + reviewer.Name, Status: result.Status, Detail: result.Detail, Required: false,
		})
		if result.Live {
			live = append(live, reviewer.Name)
		}
	}
	return checks, live
}

func (service Service) Catalog() []Reviewer {
	return append([]Reviewer(nil), service.catalog...)
}
