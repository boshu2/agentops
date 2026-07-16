package eval

import (
	"context"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

type taskRuntimeSpy struct {
	files  map[string][]byte
	writes map[string][]byte
}

func (*taskRuntimeSpy) Root() string                                 { return "/evals" }
func (runtime *taskRuntimeSpy) ReadFile(path string) ([]byte, error) { return runtime.files[path], nil }
func (runtime *taskRuntimeSpy) WriteAtomic(path string, data []byte) error {
	runtime.writes[path] = data
	return nil
}
func (*taskRuntimeSpy) ListDirectories(string) ([]string, error) { return nil, nil }
func (*taskRuntimeSpy) UserHome() (string, error)                { return "/home/test", nil }
func (*taskRuntimeSpy) SnapshotHarness(string, string, string) (*evalsubstrate.Harness, *evalsubstrate.HarnessLock, error) {
	return nil, nil, nil
}
func (*taskRuntimeSpy) LoadModelSpec(string, string) (*evalsubstrate.ModelSpec, error) {
	return nil, nil
}
func (*taskRuntimeSpy) GenerateRunID(string) string { return "run-1" }
func (*taskRuntimeSpy) OpenRun(string, string, evalsubstrate.Manifest) (TaskRunWriter, error) {
	return nil, nil
}
func (*taskRuntimeSpy) Now() time.Time { return time.Unix(1_000, 0).UTC() }

func TestTaskServiceAddCanonicalizesAndWritesOwnedDestination(t *testing.T) {
	runtime := &taskRuntimeSpy{files: map[string][]byte{"task.yaml": []byte("id: task-1\nschema_version: 1\nstats:\n  min_n_samples: 3\n")}, writes: map[string][]byte{}}
	result, err := (TaskService{Runtime: runtime}).Add(context.Background(), TaskAddRequest{SourcePath: "task.yaml"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if result.Task.ID != "task-1" || result.Destination != "/evals/tasks/task-1/task.yaml" {
		t.Fatalf("result = %#v", result)
	}
	if len(runtime.writes[result.Destination]) == 0 {
		t.Fatal("canonical task was not written")
	}
}
