package doctor

import "context"

// MutationRequest carries caller-owned options for a doctor fix invocation.
type MutationRequest struct {
	Only     []string
	Skip     []string
	Quick    bool
	Online   bool
	Severity string
	DryRun   bool
	JSON     bool
}

// MutationRuntime snapshots process state needed to execute a mutation request.
type MutationRuntime interface {
	Options(context.Context, MutationRequest) (Options, error)
}

// MutationGateway executes the doctor mutation engine.
type MutationGateway interface {
	Fix(context.Context, Options) (*Report, error)
}

// MutationService owns doctor fix application orchestration.
type MutationService struct {
	runtime MutationRuntime
	gateway MutationGateway
}

// NewMutationService constructs the doctor mutation application service.
func NewMutationService(runtime MutationRuntime, gateway MutationGateway) MutationService {
	return MutationService{runtime: runtime, gateway: gateway}
}

// Fix resolves runtime state and executes a doctor fix request.
func (service MutationService) Fix(ctx context.Context, request MutationRequest) (*Report, error) {
	options, err := service.runtime.Options(ctx, request)
	if err != nil {
		return nil, &RuntimeError{Err: err}
	}
	return service.gateway.Fix(ctx, options)
}
