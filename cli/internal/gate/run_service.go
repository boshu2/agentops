package gate

import (
	"context"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/ports"
)

type RunRequest struct {
	Name string
	Env  map[string]string
}

type RunService struct {
	Runner ports.GateRunnerPort
}

func (service RunService) Execute(ctx context.Context, request RunRequest) (ports.GateVerdict, error) {
	if request.Name == "" {
		return ports.GateVerdict{}, fmt.Errorf("gate run: name required")
	}
	if service.Runner == nil {
		return ports.GateVerdict{}, fmt.Errorf("gate run: runner required")
	}
	verdict, err := service.Runner.Run(ctx, ports.GateRunRequest{Name: ports.GateName(request.Name), Env: request.Env})
	if err != nil {
		return ports.GateVerdict{}, fmt.Errorf("gate run: %w", err)
	}
	return verdict, nil
}
