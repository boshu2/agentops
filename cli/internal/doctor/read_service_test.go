package doctor

import (
	"context"
	"errors"
	"testing"
)

type fakeReadRuntime struct {
	options Options
	request *ReadRequest
}

func (runtime fakeReadRuntime) Options(_ context.Context, request ReadRequest) (Options, error) {
	if runtime.request != nil {
		*runtime.request = request
	}
	return runtime.options, nil
}
func (runtime fakeReadRuntime) RepoRoot(context.Context) (string, error) {
	return runtime.options.RepoRoot, nil
}

type fakeReadGateway struct{ diagnosed, diffed bool }

func (gateway *fakeReadGateway) Diagnose(context.Context, Options) (*Report, error) {
	gateway.diagnosed = true
	return &Report{}, nil
}
func (gateway *fakeReadGateway) Triage(context.Context, Options) (*RobotTriageResult, *Report, error) {
	return &RobotTriageResult{}, &Report{}, nil
}
func (gateway *fakeReadGateway) Explain(context.Context, string, string) (*Finding, error) {
	return &Finding{}, nil
}
func (gateway *fakeReadGateway) Capabilities(context.Context, string) *Capabilities {
	return &Capabilities{}
}
func (gateway *fakeReadGateway) Health(context.Context, string, string) (string, *HealthResult, error) {
	return "ok", &HealthResult{}, nil
}
func (gateway *fakeReadGateway) RobotDocs(context.Context) string                   { return "docs" }
func (gateway *fakeReadGateway) List(context.Context, string) ([]RunSummary, error) { return nil, nil }
func (gateway *fakeReadGateway) Diff(context.Context, Options) (*Report, error) {
	gateway.diffed = true
	return &Report{}, nil
}

func TestReadServiceDelegatesDiagnoseAndDiffThroughPorts(t *testing.T) {
	gateway := &fakeReadGateway{}
	var captured ReadRequest
	service := NewReadService("3.0.0", fakeReadRuntime{options: Options{RepoRoot: "/repo"}, request: &captured}, gateway)
	if _, err := service.Diagnose(context.Background(), ReadRequest{Since: "old"}); err != nil {
		t.Fatal(err)
	}
	if captured.Since != "old" {
		t.Fatalf("request = %+v", captured)
	}
	if _, err := service.Diff(context.Background(), ReadRequest{}); err != nil {
		t.Fatal(err)
	}
	if !gateway.diagnosed || !gateway.diffed {
		t.Fatalf("gateway calls diagnose=%t diff=%t", gateway.diagnosed, gateway.diffed)
	}
}

type failingReadRuntime struct{}

func (failingReadRuntime) Options(context.Context, ReadRequest) (Options, error) {
	return Options{}, errors.New("cwd")
}
func (failingReadRuntime) RepoRoot(context.Context) (string, error) { return "", errors.New("cwd") }

func TestReadServiceMarksRuntimeFailures(t *testing.T) {
	service := NewReadService("3.0.0", failingReadRuntime{}, &fakeReadGateway{})
	_, err := service.Explain(context.Background(), "finding")
	var runtimeFailure *RuntimeError
	if !errors.As(err, &runtimeFailure) {
		t.Fatalf("error = %#v, want RuntimeError", err)
	}
}
