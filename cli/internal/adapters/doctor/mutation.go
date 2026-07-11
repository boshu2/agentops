package doctor

import (
	"context"

	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
)

// MutationRuntime snapshots process state for doctor mutation requests.
type MutationRuntime struct{ ToolVersion string }

// Options maps an application mutation request to engine options.
func (runtime MutationRuntime) Options(ctx context.Context, request doctorapp.MutationRequest) (doctorapp.Options, error) {
	return ReadRuntime(runtime).Options(ctx, doctorapp.ReadRequest{
		Only: append([]string(nil), request.Only...), Skip: append([]string(nil), request.Skip...),
		Quick: request.Quick, Online: request.Online, Severity: request.Severity,
		DryRun: request.DryRun, JSON: request.JSON,
	})
}

// MutationGateway adapts the application service to the existing fix engine.
type MutationGateway struct{}

// Fix executes the doctor fix engine.
func (MutationGateway) Fix(_ context.Context, options doctorapp.Options) (*doctorapp.Report, error) {
	return doctorapp.Fix(options)
}
