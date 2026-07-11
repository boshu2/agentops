package doctor

import (
	"context"
	"errors"
	"testing"
)

type fakeMutationRuntime struct {
	options Options
	request *MutationRequest
	err     error
}

func (runtime fakeMutationRuntime) Options(_ context.Context, request MutationRequest) (Options, error) {
	if runtime.request != nil {
		*runtime.request = request
	}
	return runtime.options, runtime.err
}

type fakeMutationGateway struct {
	options Options
	report  *Report
}

func (gateway *fakeMutationGateway) Fix(_ context.Context, options Options) (*Report, error) {
	gateway.options = options
	return gateway.report, nil
}

func TestMutationServiceDelegatesFixThroughPorts(t *testing.T) {
	var captured MutationRequest
	want := &Report{ExitCode: ExitFixPartial}
	gateway := &fakeMutationGateway{report: want}
	service := NewMutationService(
		fakeMutationRuntime{options: Options{RepoRoot: "/repo", DryRun: true}, request: &captured},
		gateway,
	)

	request := MutationRequest{Only: []string{"one"}, DryRun: true, JSON: true}
	got, err := service.Fix(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || gateway.options.RepoRoot != "/repo" || !gateway.options.DryRun {
		t.Fatalf("report=%+v options=%+v", got, gateway.options)
	}
	if len(captured.Only) != 1 || captured.Only[0] != "one" || !captured.DryRun || !captured.JSON {
		t.Fatalf("request=%+v", captured)
	}
}

func TestMutationServiceMarksRuntimeFailures(t *testing.T) {
	service := NewMutationService(
		fakeMutationRuntime{err: errors.New("cwd")},
		&fakeMutationGateway{},
	)
	_, err := service.Fix(context.Background(), MutationRequest{})
	var runtimeFailure *RuntimeError
	if !errors.As(err, &runtimeFailure) {
		t.Fatalf("error=%#v, want RuntimeError", err)
	}
}
