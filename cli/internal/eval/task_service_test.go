package eval

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"gopkg.in/yaml.v3"
)

type taskRuntimeSpy struct {
	files  map[string][]byte
	writes map[string][]byte
}

func (*taskRuntimeSpy) Root() string { return "/evals" }
func (runtime *taskRuntimeSpy) ReadFile(path string) ([]byte, error) {
	data, ok := runtime.files[path]
	if !ok {
		return nil, errors.New("missing file")
	}
	return data, nil
}
func (runtime *taskRuntimeSpy) WriteAtomic(path string, data []byte) error {
	runtime.writes[path] = data
	return nil
}

func TestTaskServiceRejectsHostileTaskIDsBeforeWrite(t *testing.T) {
	for _, id := range []string{"../escape", `..\escape`, "/absolute", `C:\escape`, "Task-1", " task-1"} {
		t.Run(id, func(t *testing.T) {
			source, err := yaml.Marshal(evalsubstrate.Task{ID: id, SchemaVersion: 1, Stats: evalsubstrate.TaskStat{MinNSamples: 3}})
			if err != nil {
				t.Fatal(err)
			}
			runtime := &taskRuntimeSpy{
				files:  map[string][]byte{"task.yaml": source},
				writes: map[string][]byte{},
			}
			if _, err := (TaskService{Runtime: runtime}).Add(context.Background(), TaskAddRequest{SourcePath: "task.yaml"}); err == nil {
				t.Fatalf("Add with id %q unexpectedly succeeded", id)
			}
			if len(runtime.writes) != 0 {
				t.Fatalf("Add with id %q wrote files: %v", id, runtime.writes)
			}
		})
	}
}

func TestTaskServiceShowRejectsHostileIDBeforeRead(t *testing.T) {
	runtime := &taskRuntimeSpy{files: map[string][]byte{}, writes: map[string][]byte{}}
	for _, id := range []string{"../escape", `..\escape`, "/absolute", `C:\escape`} {
		if _, err := (TaskService{Runtime: runtime}).Show(context.Background(), id); err == nil {
			t.Fatalf("Show(%q) unexpectedly succeeded", id)
		}
	}
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
