// practices: [sre, continuous-delivery]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/spf13/cobra"
)

// schedule_test.go (soc-8inr.10) — L1 integration tests against a real
// in-process daemon HTTP server (httptest + the daemon router). The CLI talks
// to it through the same activation-file/url plumbing it uses in production.
//
// All tests stand up the router built by daemonpkg.NewDaemonRouter so we
// exercise the live POST/GET/DELETE /v1/schedules wiring (and /v1/jobs for
// run). No HTTP behavior is faked — only the underlying store is in-memory.

const scheduleTestToken = "schedule-secret"

func newScheduleCommandFixture(t *testing.T) (string, *httptest.Server) {
	t.Helper()
	cwd := t.TempDir()
	store := daemonpkg.NewStore(cwd)
	router := daemonpkg.NewDaemonRouter(store, daemonpkg.ServerOptions{
		MutationPolicy: daemonpkg.DefaultMutationPolicy(scheduleTestToken, []string{
			"/jobs",
			"/v1/jobs",
			"/jobs/cancel",
			"/v1/jobs/cancel",
			"/openclaw/v1/triggers/jobs",
			"/v1/schedules",
			"/v1/schedules/*",
		}),
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return cwd, server
}

func runScheduleCommandForTest(t *testing.T, cwd, url, token string, jsonOut bool, run func(*cobra.Command) error) (string, error) {
	t.Helper()
	prevProjectDir := testProjectDir
	prevURL := daemonURL
	prevToken := daemonToken
	prevTokenFile := daemonTokenFile
	prevFile := scheduleFile
	prevJSON := scheduleListJSON
	testProjectDir = cwd
	daemonURL = url
	daemonToken = token
	daemonTokenFile = ""
	scheduleListJSON = jsonOut
	t.Cleanup(func() {
		testProjectDir = prevProjectDir
		daemonURL = prevURL
		daemonToken = prevToken
		daemonTokenFile = prevTokenFile
		scheduleFile = prevFile
		scheduleListJSON = prevJSON
	})
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetContext(context.Background())
	err := run(cmd)
	return out.String(), err
}

// writeScheduleYAML writes a minimal valid schedule YAML file with one entry
// using the given name.
func writeScheduleYAML(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, "schedule.yaml")
	body := `schedules:
  - name: ` + name + `
    cron: "0 3 * * *"
    job_type: "rpi.run"
    payload:
      epic_id: "soc-test"
    backpressure:
      skip_if_running: true
      max_queue_depth: 5
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return path
}

func TestSchedule_AddListRemoveCycle(t *testing.T) {
	cwd, server := newScheduleCommandFixture(t)
	yamlPath := writeScheduleYAML(t, cwd, "nightly-rpi")

	// add
	prevFile := scheduleFile
	scheduleFile = yamlPath
	t.Cleanup(func() { scheduleFile = prevFile })
	out, err := runScheduleCommandForTest(t, cwd, server.URL, scheduleTestToken, false, func(cmd *cobra.Command) error {
		return runScheduleAddCommand(cmd, nil)
	})
	if err != nil {
		t.Fatalf("add failed: %v\noutput=%s", err, out)
	}
	if !strings.Contains(out, "added: nightly-rpi") {
		t.Fatalf("add output missing entry: %s", out)
	}

	// list (table)
	out, err = runScheduleCommandForTest(t, cwd, server.URL, "", false, func(cmd *cobra.Command) error {
		return runScheduleListCommand(cmd, nil)
	})
	if err != nil {
		t.Fatalf("list failed: %v\noutput=%s", err, out)
	}
	if !strings.Contains(out, "nightly-rpi") {
		t.Fatalf("list missing schedule: %s", out)
	}
	if !strings.Contains(out, "rpi.run") {
		t.Fatalf("list missing job_type column: %s", out)
	}

	// remove
	out, err = runScheduleCommandForTest(t, cwd, server.URL, scheduleTestToken, false, func(cmd *cobra.Command) error {
		return runScheduleRemoveCommand(cmd, []string{"nightly-rpi"})
	})
	if err != nil {
		t.Fatalf("remove failed: %v\noutput=%s", err, out)
	}
	if !strings.Contains(out, "removed: nightly-rpi") {
		t.Fatalf("remove output missing entry: %s", out)
	}
	if !strings.Contains(out, "deleted=true") {
		t.Fatalf("remove output missing deleted=true: %s", out)
	}

	// list again — should be empty
	out, err = runScheduleCommandForTest(t, cwd, server.URL, "", false, func(cmd *cobra.Command) error {
		return runScheduleListCommand(cmd, nil)
	})
	if err != nil {
		t.Fatalf("list-after-remove failed: %v\noutput=%s", err, out)
	}
	if !strings.Contains(out, "no schedules") {
		t.Fatalf("expected 'no schedules' after remove, got: %s", out)
	}
}

func TestSchedule_RunFiresOneShot(t *testing.T) {
	cwd, server := newScheduleCommandFixture(t)
	yamlPath := writeScheduleYAML(t, cwd, "fire-me")

	// Pre-populate by calling add.
	prevFile := scheduleFile
	scheduleFile = yamlPath
	t.Cleanup(func() { scheduleFile = prevFile })
	if _, err := runScheduleCommandForTest(t, cwd, server.URL, scheduleTestToken, false, func(cmd *cobra.Command) error {
		return runScheduleAddCommand(cmd, nil)
	}); err != nil {
		t.Fatalf("setup add failed: %v", err)
	}

	// run <name>
	out, err := runScheduleCommandForTest(t, cwd, server.URL, scheduleTestToken, false, func(cmd *cobra.Command) error {
		return runScheduleRunCommand(cmd, []string{"fire-me"})
	})
	if err != nil {
		t.Fatalf("run failed: %v\noutput=%s", err, out)
	}
	if !strings.Contains(out, "fired: fire-me") {
		t.Fatalf("run output missing 'fired: fire-me': %s", out)
	}
	if !strings.Contains(out, "job_id=") {
		t.Fatalf("run output missing job_id: %s", out)
	}
}

func TestSchedule_AuthRequiredOnAdd(t *testing.T) {
	cwd, server := newScheduleCommandFixture(t)
	yamlPath := writeScheduleYAML(t, cwd, "no-token-test")

	prevFile := scheduleFile
	scheduleFile = yamlPath
	t.Cleanup(func() { scheduleFile = prevFile })

	// Run with empty token — daemon should reject mutation with non-2xx.
	out, err := runScheduleCommandForTest(t, cwd, server.URL, "", false, func(cmd *cobra.Command) error {
		return runScheduleAddCommand(cmd, nil)
	})
	if err == nil {
		t.Fatalf("add without token unexpectedly succeeded: %s", out)
	}
	// Error message should reference HTTP failure or daemon rejection.
	if !strings.Contains(err.Error(), "add schedule") {
		t.Fatalf("error missing context: %v", err)
	}
}

func TestScheduleMutationsUseAgentOpsDTokenEnv(t *testing.T) {
	cwd, server := newScheduleCommandFixture(t)
	yamlPath := writeScheduleYAML(t, cwd, "env-token-test")
	t.Setenv("AGENTOPSD_TOKEN", scheduleTestToken)

	prevFile := scheduleFile
	scheduleFile = yamlPath
	t.Cleanup(func() { scheduleFile = prevFile })

	out, err := runScheduleCommandForTest(t, cwd, server.URL, "", false, func(cmd *cobra.Command) error {
		return runScheduleAddCommand(cmd, nil)
	})
	if err != nil {
		t.Fatalf("add with env token failed: %v\noutput=%s", err, out)
	}

	out, err = runScheduleCommandForTest(t, cwd, server.URL, "", false, func(cmd *cobra.Command) error {
		return runScheduleRunCommand(cmd, []string{"env-token-test"})
	})
	if err != nil {
		t.Fatalf("run with env token failed: %v\noutput=%s", err, out)
	}

	out, err = runScheduleCommandForTest(t, cwd, server.URL, "", false, func(cmd *cobra.Command) error {
		return runScheduleRemoveCommand(cmd, []string{"env-token-test"})
	})
	if err != nil {
		t.Fatalf("remove with env token failed: %v\noutput=%s", err, out)
	}
}

// boundaryClock is a minimal daemonpkg.Clock fake used by the recurrence
// wiring test below. It pins Now() to a fixed cron-boundary time so the
// supervisor's first tick fires immediately, and returns a never-firing
// channel from After() so the supervisor blocks until ctx is cancelled.
// The real daemonpkg.FakeClock lives in clock_test.go and is package-private
// to daemon tests, hence this local minimum.
type boundaryClock struct{ at time.Time }

func (c boundaryClock) Now() time.Time                         { return c.at }
func (c boundaryClock) After(d time.Duration) <-chan time.Time { return make(chan time.Time) }

type panicOnceClock struct {
	mu       sync.Mutex
	at       time.Time
	panicked bool
}

func (c *panicOnceClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.panicked {
		c.panicked = true
		panic("clock boom")
	}
	return c.at
}

func (c *panicOnceClock) After(d time.Duration) <-chan time.Time {
	return time.After(time.Millisecond)
}

// TestServeAgentOpsDaemon_WiresRecurrenceSupervisor pins the production
// wiring of the cron supervisor into ao daemon run. Regression for soc-63n0
// where RecurrenceSupervisor was fully implemented but never instantiated
// in the run path — the daemon emitted schedule.created on load and then
// never fired anything. The test calls serveAgentOpsDaemon (the actual
// daemon-run entry point) and asserts a schedule.fired event lands in the
// ledger. If the call to startAgentOpsDaemonRecurrence is ever removed
// from serveAgentOpsDaemonWithWorkers, this test fails.
func TestServeAgentOpsDaemon_WiresRecurrenceSupervisor(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".agents"), 0o700); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	yaml := `schedules:
  - name: fast-fire
    cron: "* * * * *"
    job_type: "rpi.run"
    payload:
      epic_id: "soc-test"
    backpressure:
      skip_if_running: true
      max_queue_depth: 5
`
	schedulePath := filepath.Join(cwd, ".agents", "schedule.yaml")
	if err := os.WriteFile(schedulePath, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write schedule.yaml: %v", err)
	}

	// Anchor the clock on a cron-boundary minute so the first tick fires
	// "fast-fire" immediately. The fake's After() returns a never-firing
	// channel; the supervisor blocks on it after the first tick, then exits
	// when ctx is cancelled.
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- serveAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
			Addr:            "127.0.0.1:0", // ephemeral; avoid port collisions
			Workers:         0,
			ScheduleFile:    schedulePath,
			RecurrenceClock: boundaryClock{at: start},
		}, &bytes.Buffer{})
	}()

	// Poll the ledger for schedule.fired with the matching name. Allow
	// generous wall-clock time because serveAgentOpsDaemon spins up a
	// listener + ledger + HTTP router before the recurrence goroutine
	// gets to its first tick.
	store := daemonpkg.NewStore(cwd)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		replay, err := store.ReplayLedgerReadOnly()
		if err == nil {
			for _, e := range replay.Events {
				if e.EventType == daemonpkg.EventScheduleFired && e.Payload["name"] == "fast-fire" {
					cancel()
					<-serveErr
					return
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	cancel()
	<-serveErr
	replay, _ := store.ReplayLedgerReadOnly()
	var seen []string
	for _, e := range replay.Events {
		seen = append(seen, string(e.EventType))
	}
	t.Fatalf("schedule.fired for 'fast-fire' never appeared via serveAgentOpsDaemon (soc-63n0: cron supervisor not wired into daemon-run path). Events seen: %v", seen)
}

func TestStartAgentOpsDaemonRecurrence_RecoversAndContinuesAfterPanic(t *testing.T) {
	cwd := t.TempDir()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := daemonpkg.NewStore(cwd)
	if err := store.SaveSchedule(daemonpkg.RecurringJobTemplate{
		Name:    "recover-fire",
		Cron:    "* * * * *",
		JobType: daemonpkg.JobTypeLLMWikiLoop,
	}); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startAgentOpsDaemonRecurrence(ctx, cwd, agentopsDaemonRunOptions{
		RecurrenceClock:        &panicOnceClock{at: now},
		RecurrencePollInterval: time.Millisecond,
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ledgerHasScheduleFired(t, store, "recover-fire") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	replay, _ := store.ReplayLedgerReadOnly()
	t.Fatalf("recurrence goroutine did not continue after panic; events=%v", replay.Events)
}

func ledgerHasScheduleFired(t *testing.T, store *daemonpkg.Store, name string) bool {
	t.Helper()
	replay, err := store.ReplayLedgerReadOnly()
	if err != nil {
		return false
	}
	for _, event := range replay.Events {
		if event.EventType == daemonpkg.EventScheduleFired && event.Payload["name"] == name {
			return true
		}
	}
	return false
}

func TestSchedule_ListJSONOutput(t *testing.T) {
	cwd, server := newScheduleCommandFixture(t)
	yamlPath := writeScheduleYAML(t, cwd, "json-test")

	prevFile := scheduleFile
	scheduleFile = yamlPath
	t.Cleanup(func() { scheduleFile = prevFile })
	if _, err := runScheduleCommandForTest(t, cwd, server.URL, scheduleTestToken, false, func(cmd *cobra.Command) error {
		return runScheduleAddCommand(cmd, nil)
	}); err != nil {
		t.Fatalf("setup add failed: %v", err)
	}

	out, err := runScheduleCommandForTest(t, cwd, server.URL, "", true, func(cmd *cobra.Command) error {
		return runScheduleListCommand(cmd, nil)
	})
	if err != nil {
		t.Fatalf("list --json failed: %v\noutput=%s", err, out)
	}
	var resp daemonpkg.ListSchedulesResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput=%s", err, out)
	}
	if len(resp.Schedules) != 1 || resp.Schedules[0].Name != "json-test" {
		t.Fatalf("unexpected schedules in JSON: %+v", resp.Schedules)
	}
}
