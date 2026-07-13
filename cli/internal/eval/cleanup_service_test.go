package eval

import (
	"context"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

type cleanupRuntimeSpy struct {
	manifests   map[string]*evalsubstrate.Manifest
	transitions []string
	deletions   []string
}

func (*cleanupRuntimeSpy) Root() string { return "/evals" }
func (runtime *cleanupRuntimeSpy) ListRunIDs(string) ([]string, error) {
	return []string{"pending", "running", "retracted"}, nil
}
func (runtime *cleanupRuntimeSpy) LoadManifest(_ string, id string) (*evalsubstrate.Manifest, error) {
	return runtime.manifests[id], nil
}
func (runtime *cleanupRuntimeSpy) Transition(_ string, id string, next evalsubstrate.RunStatus, _ string, _ time.Time) error {
	runtime.transitions = append(runtime.transitions, id+":"+string(next))
	runtime.manifests[id].Status = next
	return nil
}
func (runtime *cleanupRuntimeSpy) DeleteRun(_ string, id string) error {
	runtime.deletions = append(runtime.deletions, id)
	return nil
}
func (*cleanupRuntimeSpy) SweepTempFiles(string, int64) ([]string, error) {
	return []string{"one.tmp"}, nil
}
func (*cleanupRuntimeSpy) Now() time.Time { return time.Unix(1_000, 0).UTC() }

func TestCleanupServiceTransitionsDeletesAndPreservesRetracted(t *testing.T) {
	old := time.Unix(1_000, 0).Add(-10 * time.Minute).UnixMilli()
	runtime := &cleanupRuntimeSpy{manifests: map[string]*evalsubstrate.Manifest{
		"pending":   {Status: evalsubstrate.StatusPending, StartedAtUnixMs: old},
		"running":   {Status: evalsubstrate.StatusRunning, StartedAtUnixMs: old},
		"retracted": {Status: evalsubstrate.StatusRetracted, StartedAtUnixMs: old},
	}}
	report, err := (CleanupService{Runtime: runtime}).Execute(context.Background(), CleanupRequest{Delete: true, TmpFiles: true, TmpAgeSeconds: 60})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if report.TransitionsAborted != 1 || report.TransitionsFailed != 1 || report.RunsDeleted != 2 || report.TmpFilesSwept != 1 {
		t.Fatalf("report = %#v", report)
	}
	for _, deleted := range runtime.deletions {
		if deleted == "retracted" {
			t.Fatal("retracted run was deleted")
		}
	}
}
