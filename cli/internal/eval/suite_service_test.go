package eval

import (
	"context"
	"strings"
	"testing"
)

type suiteRuntimeSpy struct {
	args   []string
	output []byte
}

func (*suiteRuntimeSpy) Root() string                    { return "/evals" }
func (*suiteRuntimeSpy) ReadFile(string) ([]byte, error) { return nil, errMissingSuite }
func (runtime *suiteRuntimeSpy) RunStats(args []string) ([]byte, error) {
	runtime.args = args
	return runtime.output, nil
}

var errMissingSuite = &missingSuiteError{}

type missingSuiteError struct{}

func (*missingSuiteError) Error() string { return "missing suite" }

func TestSuiteServiceVerdictBuildsDeterministicStatsInvocation(t *testing.T) {
	runtime := &suiteRuntimeSpy{output: []byte(`{"verdict":"improved","n_required":3}`)}
	result, err := (SuiteService{Runtime: runtime}).Verdict(context.Background(), SuiteVerdictRequest{SuiteID: "suite-1", Arms: "a,b", InputsPath: "inputs.json", BootstrapSamples: 10000})
	if err != nil {
		t.Fatalf("Verdict: %v", err)
	}
	if result.Values["verdict"] != "improved" || !strings.Contains(strings.Join(runtime.args, " "), "--suite-id suite-1 --arms a,b") {
		t.Fatalf("result=%#v args=%v", result, runtime.args)
	}
}
