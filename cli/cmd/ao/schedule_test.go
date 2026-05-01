package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
