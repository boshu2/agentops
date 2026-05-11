// practices: [sre, distributed-tracing]
package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/spf13/cobra"
)

func TestAoWatch(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	first, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{})
	if err != nil {
		t.Fatalf("submit first job: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-dream", JobID: "job-dream", JobType: daemonpkg.JobTypeDreamRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit second job: %v", err)
	}

	out := runWatchCommandForTest(t, cwd, server.URL, first.LastEventID)
	if strings.Contains(out, "job-rpi") || !strings.Contains(out, "job-dream") {
		t.Fatalf("watch output = %q, want only events after %s", out, first.LastEventID)
	}
	for _, needle := range []string{"job.accepted", "dream.run", "agentopsd"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("watch output missing %q:\n%s", needle, out)
		}
	}
}

func runWatchCommandForTest(t *testing.T, cwd, daemonBaseURL, since string) string {
	t.Helper()
	prevProjectDir := testProjectDir
	prevURL := watchDaemonURL
	prevSince := watchSince
	prevInterval := watchInterval
	prevOnce := watchOnce
	testProjectDir = cwd
	watchDaemonURL = daemonBaseURL
	watchSince = since
	watchInterval = time.Millisecond
	watchOnce = true
	t.Cleanup(func() {
		testProjectDir = prevProjectDir
		watchDaemonURL = prevURL
		watchSince = prevSince
		watchInterval = prevInterval
		watchOnce = prevOnce
	})
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetContext(context.Background())
	if err := runAoWatchCommand(cmd, nil); err != nil {
		t.Fatalf("runAoWatchCommand: %v", err)
	}
	return out.String()
}
