package eval

import (
	"context"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

type suiteRuntimeSpy struct {
	args    []string
	output  []byte
	files   map[string][]byte
	readErr error
}

func (*suiteRuntimeSpy) Root() string { return "/evals" }
func (runtime *suiteRuntimeSpy) ReadFile(path string) ([]byte, error) {
	if runtime.readErr != nil {
		return nil, runtime.readErr
	}
	return runtime.files[path], nil
}
func (runtime *suiteRuntimeSpy) RunStats(args []string) ([]byte, error) {
	runtime.args = args
	return runtime.output, nil
}

var errMissingSuite = &missingSuiteError{}

type missingSuiteError struct{}

func (*missingSuiteError) Error() string { return "missing suite" }

func TestSuiteServiceVerdictBuildsDeterministicStatsInvocation(t *testing.T) {
	suite, err := yaml.Marshal(evalsubstrate.Suite{ID: "suite-1"})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &suiteRuntimeSpy{output: []byte(`{"verdict":"improved","n_required":3}`), files: map[string][]byte{"/evals/suites/suite-1/suite.yaml": suite}}
	result, err := (SuiteService{Runtime: runtime}).Verdict(context.Background(), SuiteVerdictRequest{SuiteID: "suite-1", Arms: "a,b", InputsPath: "inputs.json", BootstrapSamples: 10000})
	if err != nil {
		t.Fatalf("Verdict: %v", err)
	}
	if result.Values["verdict"] != "improved" || !strings.Contains(strings.Join(runtime.args, " "), "--suite-id suite-1 --arms a,b") {
		t.Fatalf("result=%#v args=%v", result, runtime.args)
	}
}

func TestSuiteServiceVerdictReturnsSuiteLoadError(t *testing.T) {
	runtime := &suiteRuntimeSpy{output: []byte(`{"verdict":"improved"}`), readErr: errMissingSuite}
	if _, err := (SuiteService{Runtime: runtime}).Verdict(context.Background(), SuiteVerdictRequest{SuiteID: "suite-1", Arms: "a,b", InputsPath: "inputs.json"}); err == nil || !strings.Contains(err.Error(), "missing suite") {
		t.Fatalf("Verdict error = %v, want surfaced suite load error", err)
	}
	if len(runtime.args) != 0 {
		t.Fatalf("stats process invoked after suite load failure: %v", runtime.args)
	}
}

func TestSuiteServiceVerdictRejectsHostileIDBeforeRead(t *testing.T) {
	runtime := &suiteRuntimeSpy{output: []byte(`{"verdict":"improved"}`), files: map[string][]byte{}}
	for _, id := range []string{"../escape", `..\escape`, "/absolute", `C:\escape`} {
		if _, err := (SuiteService{Runtime: runtime}).Verdict(context.Background(), SuiteVerdictRequest{SuiteID: id, Arms: "a,b", InputsPath: "inputs.json"}); err == nil {
			t.Fatalf("Verdict(%q) unexpectedly succeeded", id)
		}
	}
}

func TestSuiteServiceVerdictPreservesExplicitYAMLPath(t *testing.T) {
	suite, err := yaml.Marshal(evalsubstrate.Suite{ID: "suite-from-file"})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &suiteRuntimeSpy{
		output: []byte(`{"verdict":"improved"}`),
		files:  map[string][]byte{"fixtures/custom.yaml": suite},
	}
	if _, err := (SuiteService{Runtime: runtime}).Verdict(context.Background(), SuiteVerdictRequest{
		SuiteID: "fixtures/custom.yaml", Arms: "a,b", InputsPath: "inputs.json",
	}); err != nil {
		t.Fatalf("Verdict explicit YAML path: %v", err)
	}
}
