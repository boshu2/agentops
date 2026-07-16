package doctor

import "context"

type RuntimeError struct{ Err error }

func (failure *RuntimeError) Error() string { return failure.Err.Error() }
func (failure *RuntimeError) Unwrap() error { return failure.Err }

type ReadRequest struct {
	Only     []string
	Skip     []string
	Quick    bool
	Online   bool
	Severity string
	DryRun   bool
	JSON     bool
	Since    string
}

type ReadRuntime interface {
	Options(context.Context, ReadRequest) (Options, error)
	RepoRoot(context.Context) (string, error)
}

type ReadGateway interface {
	Diagnose(context.Context, Options) (*Report, error)
	Triage(context.Context, Options) (*RobotTriageResult, *Report, error)
	Explain(context.Context, string, string) (*Finding, error)
	Capabilities(context.Context, string) *Capabilities
	Health(context.Context, string, string) (string, *HealthResult, error)
	RobotDocs(context.Context) string
	List(context.Context, string) ([]RunSummary, error)
	Diff(context.Context, Options) (*Report, error)
}

type ReadService struct {
	toolVersion string
	runtime     ReadRuntime
	gateway     ReadGateway
}

func NewReadService(toolVersion string, runtime ReadRuntime, gateway ReadGateway) ReadService {
	return ReadService{toolVersion: toolVersion, runtime: runtime, gateway: gateway}
}

func (service ReadService) Diagnose(ctx context.Context, request ReadRequest) (*Report, error) {
	options, err := service.runtime.Options(ctx, request)
	if err != nil {
		return nil, &RuntimeError{Err: err}
	}
	return service.gateway.Diagnose(ctx, options)
}

func (service ReadService) Triage(ctx context.Context, request ReadRequest) (*RobotTriageResult, *Report, error) {
	options, err := service.runtime.Options(ctx, request)
	if err != nil {
		return nil, nil, &RuntimeError{Err: err}
	}
	return service.gateway.Triage(ctx, options)
}

func (service ReadService) Explain(ctx context.Context, findingID string) (*Finding, error) {
	root, err := service.runtime.RepoRoot(ctx)
	if err != nil {
		return nil, &RuntimeError{Err: err}
	}
	return service.gateway.Explain(ctx, root, findingID)
}

func (service ReadService) Capabilities(ctx context.Context) *Capabilities {
	return service.gateway.Capabilities(ctx, service.toolVersion)
}

func (service ReadService) Health(ctx context.Context) (string, *HealthResult, error) {
	root, err := service.runtime.RepoRoot(ctx)
	if err != nil {
		return "", nil, &RuntimeError{Err: err}
	}
	return service.gateway.Health(ctx, root, service.toolVersion)
}

func (service ReadService) RobotDocs(ctx context.Context) string {
	return service.gateway.RobotDocs(ctx)
}

func (service ReadService) List(ctx context.Context) ([]RunSummary, error) {
	root, err := service.runtime.RepoRoot(ctx)
	if err != nil {
		return nil, &RuntimeError{Err: err}
	}
	return service.gateway.List(ctx, root)
}

func (service ReadService) Diff(ctx context.Context, request ReadRequest) (*Report, error) {
	options, err := service.runtime.Options(ctx, request)
	if err != nil {
		return nil, &RuntimeError{Err: err}
	}
	return service.gateway.Diff(ctx, options)
}
