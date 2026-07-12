package gate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

type gateRunnerStub struct {
	request ports.GateRunRequest
	verdict ports.GateVerdict
	err     error
}

func (runner *gateRunnerStub) Run(_ context.Context, request ports.GateRunRequest) (ports.GateVerdict, error) {
	runner.request = request
	return runner.verdict, runner.err
}

func TestRunServiceReturnsTypedVerdict(t *testing.T) {
	runner := &gateRunnerStub{verdict: ports.GateVerdict{Status: ports.GateStatusPass, Reason: "exit 0"}}
	verdict, err := (RunService{Runner: runner}).Execute(context.Background(), RunRequest{Name: "compile-health"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if runner.request.Name != "compile-health" || verdict.Status != ports.GateStatusPass {
		t.Fatalf("request=%+v verdict=%+v", runner.request, verdict)
	}
}

func TestRunServiceRejectsEmptyName(t *testing.T) {
	runner := &gateRunnerStub{}
	_, err := (RunService{Runner: runner}).Execute(context.Background(), RunRequest{})
	if err == nil || runner.request.Name != "" {
		t.Fatalf("err=%v request=%+v", err, runner.request)
	}
}

func TestRunServiceWrapsRunnerFailure(t *testing.T) {
	runner := &gateRunnerStub{err: errors.New("subprocess died")}
	_, err := (RunService{Runner: runner}).Execute(context.Background(), RunRequest{Name: "broken"})
	if err == nil || !strings.Contains(err.Error(), "gate run: subprocess died") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunServiceForwardsEnvironment(t *testing.T) {
	runner := &gateRunnerStub{}
	_, err := (RunService{Runner: runner}).Execute(context.Background(), RunRequest{Name: "env", Env: map[string]string{"KEY": "value"}})
	if err != nil || runner.request.Env["KEY"] != "value" {
		t.Fatalf("err=%v request=%+v", err, runner.request)
	}
}
