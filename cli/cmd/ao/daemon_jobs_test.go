// practices: [microservices, sre]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/spf13/cobra"
)

func TestDaemonJobsList(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit rpi job: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-dream", JobID: "job-dream", JobType: daemonpkg.JobTypeDreamRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit dream job: %v", err)
	}
	out := runDaemonJobsCommandForTest(t, cwd, server.URL, "", "table", func(cmd *cobra.Command) error {
		return runAgentOpsDaemonJobsListCommand(cmd, nil)
	})
	for _, needle := range []string{"job-rpi", "rpi.run", "queued", "job-dream", "dream.run"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("list output missing %q:\n%s", needle, out)
		}
	}
}

func TestDaemonJobsShow(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	out := runDaemonJobsCommandForTest(t, cwd, server.URL, "", "table", func(cmd *cobra.Command) error {
		return runAgentOpsDaemonJobsShowCommand(cmd, []string{"job-rpi"})
	})
	if !strings.Contains(out, "job-rpi") || !strings.Contains(out, "queued") {
		t.Fatalf("show output = %q, want queued job-rpi", out)
	}
}

func TestDaemonJobsWaitCompleted(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	claim, err := queue.ClaimJob("job-rpi", "worker", daemonpkg.QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}
	if _, err := queue.CompleteJob(daemonpkg.CompleteJobInput{
		JobID:      "job-rpi",
		RequestID:  "req-rpi-complete",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker",
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("complete job: %v", err)
	}
	daemonJobWaitTimeout = time.Second
	out := runDaemonJobsCommandForTest(t, cwd, server.URL, "", "table", func(cmd *cobra.Command) error {
		return runAgentOpsDaemonJobsWaitCommand(cmd, []string{"job-rpi"})
	})
	if !strings.Contains(out, "completed") {
		t.Fatalf("wait output = %q, want completed", out)
	}
}

func TestDaemonJobsWaitTimeout(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	daemonJobWaitTimeout = time.Millisecond
	err := runDaemonJobsCommandForTestErr(t, cwd, server.URL, "", "table", func(cmd *cobra.Command) error {
		return runAgentOpsDaemonJobsWaitCommand(cmd, []string{"job-rpi"})
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("wait error = %v, want timeout", err)
	}
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.Jobs[0].Status != daemonpkg.JobStatusQueued {
		t.Fatalf("wait timeout mutated job to %q", snapshot.Jobs[0].Status)
	}
}

func TestDaemonJobsCancelWritesTerminalEvent(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	daemonJobCancelReason = "operator stop"
	out := runDaemonJobsCommandForTest(t, cwd, server.URL, "secret-token", "table", func(cmd *cobra.Command) error {
		return runAgentOpsDaemonJobsCancelCommand(cmd, []string{"job-rpi"})
	})
	if !strings.Contains(out, "cancelled") {
		t.Fatalf("cancel output = %q, want cancelled", out)
	}
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.Jobs[0].Status != daemonpkg.JobStatusCancelled {
		t.Fatalf("cancel status = %q, want cancelled", snapshot.Jobs[0].Status)
	}
}

func TestDaemonJobsCancelUsesAgentOpsDTokenEnv(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	t.Setenv("AGENTOPSD_TOKEN", "secret-token")
	daemonJobCancelReason = "operator stop"
	out := runDaemonJobsCommandForTest(t, cwd, server.URL, "", "table", func(cmd *cobra.Command) error {
		return runAgentOpsDaemonJobsCancelCommand(cmd, []string{"job-rpi"})
	})
	if !strings.Contains(out, "cancelled") {
		t.Fatalf("cancel output = %q, want cancelled", out)
	}
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.Jobs[0].Status != daemonpkg.JobStatusCancelled {
		t.Fatalf("cancel status = %q, want cancelled", snapshot.Jobs[0].Status)
	}
}

func TestDaemonJobsJSONTextParity(t *testing.T) {
	for _, format := range []string{"table", "json"} {
		t.Run(format, func(t *testing.T) {
			cwd, server, queue := newDaemonJobsCommandFixture(t)
			if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{}); err != nil {
				t.Fatalf("submit job: %v", err)
			}
			daemonJobCancelReason = "operator stop"
			out := runDaemonJobsCommandForTest(t, cwd, server.URL, "secret-token", format, func(cmd *cobra.Command) error {
				return runAgentOpsDaemonJobsCancelCommand(cmd, []string{"job-rpi"})
			})
			if format == "json" {
				var response daemonpkg.CancelJobResponse
				if err := json.Unmarshal([]byte(out), &response); err != nil {
					t.Fatalf("decode json cancel output: %v\n%s", err, out)
				}
			}
			snapshot, err := queue.Snapshot()
			if err != nil {
				t.Fatalf("snapshot: %v", err)
			}
			if snapshot.Jobs[0].Status != daemonpkg.JobStatusCancelled {
				t.Fatalf("%s cancel status = %q, want cancelled", format, snapshot.Jobs[0].Status)
			}
		})
	}
}

func TestDaemonEventsTailAfter(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	first, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{})
	if err != nil {
		t.Fatalf("submit first job: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-dream", JobID: "job-dream", JobType: daemonpkg.JobTypeDreamRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit second job: %v", err)
	}
	daemonEventsAfter = first.LastEventID
	out := runDaemonJobsCommandForTest(t, cwd, server.URL, "", "table", func(cmd *cobra.Command) error {
		return runAgentOpsDaemonEventsTailCommand(cmd, nil)
	})
	if strings.Contains(out, "job-rpi") || !strings.Contains(out, "job-dream") {
		t.Fatalf("events tail output = %q, want only events after %s", out, first.LastEventID)
	}
}

func newDaemonJobsCommandFixture(t *testing.T) (string, *httptest.Server, *daemonpkg.Queue) {
	t.Helper()
	cwd := t.TempDir()
	store := daemonpkg.NewStore(cwd)
	queue := daemonpkg.NewQueue(store, daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	router := daemonpkg.NewDaemonRouter(store, daemonpkg.ServerOptions{
		MutationPolicy: daemonpkg.DefaultMutationPolicy("secret-token", []string{
			"/jobs",
			"/v1/jobs",
			"/jobs/cancel",
			"/v1/jobs/cancel",
			"/openclaw/v1/triggers/jobs",
		}),
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return cwd, server, queue
}

func runDaemonJobsCommandForTest(t *testing.T, cwd, url, token, format string, run func(*cobra.Command) error) string {
	t.Helper()
	out, err := runDaemonJobsCommandForTestWithOutput(t, cwd, url, token, format, run)
	if err != nil {
		t.Fatalf("command error: %v\noutput=%s", err, out)
	}
	return out
}

func runDaemonJobsCommandForTestErr(t *testing.T, cwd, url, token, format string, run func(*cobra.Command) error) error {
	t.Helper()
	_, err := runDaemonJobsCommandForTestWithOutput(t, cwd, url, token, format, run)
	return err
}

func runDaemonJobsCommandForTestWithOutput(t *testing.T, cwd, url, token, format string, run func(*cobra.Command) error) (string, error) {
	t.Helper()
	prevProjectDir := testProjectDir
	prevURL := daemonURL
	prevToken := daemonToken
	prevTokenFile := daemonTokenFile
	prevOutput := output
	prevCancelReason := daemonJobCancelReason
	prevWaitTimeout := daemonJobWaitTimeout
	prevEventsAfter := daemonEventsAfter
	testProjectDir = cwd
	daemonURL = url
	daemonToken = token
	daemonTokenFile = ""
	output = format
	t.Cleanup(func() {
		testProjectDir = prevProjectDir
		daemonURL = prevURL
		daemonToken = prevToken
		daemonTokenFile = prevTokenFile
		output = prevOutput
		daemonJobCancelReason = prevCancelReason
		daemonJobWaitTimeout = prevWaitTimeout
		daemonEventsAfter = prevEventsAfter
	})
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetContext(context.Background())
	err := run(cmd)
	return out.String(), err
}

func TestDaemonJobsCancelRequiresToken(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: daemonpkg.JobTypeRPIRun}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	err := runDaemonJobsCommandForTestErr(t, cwd, server.URL, "", "table", func(cmd *cobra.Command) error {
		return runAgentOpsDaemonJobsCancelCommand(cmd, []string{"job-rpi"})
	})
	if err == nil {
		t.Fatal("cancel without token succeeded")
	}
	snapshot, snapErr := queue.Snapshot()
	if snapErr != nil {
		t.Fatalf("snapshot: %v", snapErr)
	}
	if snapshot.Jobs[0].Status != daemonpkg.JobStatusQueued {
		t.Fatalf("unauthorized cancel mutated job to %q", snapshot.Jobs[0].Status)
	}
}

func TestDaemonJobTerminalStatusHelper(t *testing.T) {
	for _, status := range []daemonpkg.JobStatus{daemonpkg.JobStatusCompleted, daemonpkg.JobStatusFailed, daemonpkg.JobStatusCancelled} {
		if !daemonJobIsTerminal(status) {
			t.Fatalf("%s should be terminal", status)
		}
	}
	if daemonJobIsTerminal(daemonpkg.JobStatusQueued) || daemonJobIsTerminal(daemonpkg.JobStatusRunning) {
		t.Fatal("non-terminal status reported terminal")
	}
}

func TestDaemonJobsSubmitPostsJob(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	prevType, prevPayload := daemonJobSubmitType, daemonJobSubmitPayload
	daemonJobSubmitType = string(daemonpkg.JobTypeOpenClawSnapshot)
	daemonJobSubmitPayload = `{"smoke":true}`
	t.Cleanup(func() {
		daemonJobSubmitType = prevType
		daemonJobSubmitPayload = prevPayload
	})

	out := runDaemonJobsCommandForTest(t, cwd, server.URL, "secret-token", "json", func(cmd *cobra.Command) error {
		return runAgentOpsDaemonJobsSubmitCommand(cmd, nil)
	})

	var response submitDaemonJobResponse
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		t.Fatalf("decode submit output: %v\n%s", err, out)
	}
	if !response.Accepted {
		t.Fatalf("submit not accepted: %+v", response)
	}
	if response.JobType != daemonpkg.JobTypeOpenClawSnapshot {
		t.Fatalf("submit job_type = %q, want %q", response.JobType, daemonpkg.JobTypeOpenClawSnapshot)
	}
	if response.JobID == "" {
		t.Fatal("submit returned empty job_id")
	}
	if response.Status != daemonpkg.JobStatusQueued {
		t.Fatalf("submit status = %q, want queued", response.Status)
	}

	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot.Jobs) != 1 || snapshot.Jobs[0].JobID != response.JobID {
		t.Fatalf("queue does not contain submitted job; jobs=%+v", snapshot.Jobs)
	}
}

func TestValidateSubmitJobTypeAcceptsKnown(t *testing.T) {
	for _, jt := range knownDaemonJobTypes {
		if err := validateSubmitJobType(string(jt)); err != nil {
			t.Errorf("validateSubmitJobType(%q) returned error: %v", jt, err)
		}
	}
	for _, jt := range []daemonpkg.JobType{
		daemonpkg.JobTypeEvalSuite,
		daemonpkg.JobTypeEvalSkillDelta,
		daemonpkg.JobTypePlansProjection,
		daemonpkg.JobTypeLLMWikiLoop,
		daemonpkg.JobTypeSkillInvoke,
	} {
		if err := validateSubmitJobType(string(jt)); err != nil {
			t.Errorf("new job type %q rejected: %v", jt, err)
		}
	}
}

func TestValidateSubmitJobTypeRejectsUnknown(t *testing.T) {
	err := validateSubmitJobType("not-a-real-type")
	if err == nil {
		t.Fatal("validateSubmitJobType accepted unknown type")
	}
	var unk unknownJobTypeError
	if !errors.As(err, &unk) {
		t.Fatalf("validateSubmitJobType error type = %T, want unknownJobTypeError", err)
	}
	if unk.got != "not-a-real-type" {
		t.Errorf("unknownJobTypeError.got = %q, want %q", unk.got, "not-a-real-type")
	}
	for _, want := range []string{"rpi.run", "wiki.forge", "openclaw.snapshot"} {
		if !strings.Contains(unk.Error(), want) {
			t.Errorf("error %q missing known-type %q", unk.Error(), want)
		}
	}
}

func TestReadSubmitPayloadInline(t *testing.T) {
	got, err := readSubmitPayload(`{"k":1}`)
	if err != nil {
		t.Fatalf("inline payload: %v", err)
	}
	if got["k"] != float64(1) {
		t.Errorf("payload k = %v, want 1", got["k"])
	}
}

func TestReadSubmitPayloadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(path, []byte(`{"file":true}`), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	got, err := readSubmitPayload("@" + path)
	if err != nil {
		t.Fatalf("file payload: %v", err)
	}
	if got["file"] != true {
		t.Errorf("payload file = %v, want true", got["file"])
	}
}

func TestReadSubmitPayloadRejectsEmpty(t *testing.T) {
	if _, err := readSubmitPayload(""); err == nil {
		t.Fatal("empty payload accepted")
	}
	if _, err := readSubmitPayload("   "); err == nil {
		t.Fatal("whitespace payload accepted")
	}
}

func TestReadSubmitPayloadRejectsNonJSON(t *testing.T) {
	if _, err := readSubmitPayload("not json"); err == nil {
		t.Fatal("non-JSON payload accepted")
	}
}
