package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

func TestDoctorDaemonRuntimeCheckPassesWithReadyServer(t *testing.T) {
	server := httptest.NewServer(daemonpkg.NewReadOnlyRouter(
		daemonpkg.NewStore(t.TempDir()),
		daemonpkg.ServerOptions{Now: func() time.Time { return time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC) }},
	))
	t.Cleanup(server.Close)

	check := checkDaemonRuntimeURL(server.URL)
	if check.Name != "Daemon Runtime" || check.Status != "pass" {
		t.Fatalf("daemon check = %#v, want pass", check)
	}
	if !strings.Contains(check.Detail, "events=0") || !strings.Contains(check.Detail, "projection=current") {
		t.Fatalf("daemon detail = %q, want event/projection details", check.Detail)
	}
}

func TestDoctorOpenClawConsumerCheckPassesWithReadyServer(t *testing.T) {
	server := httptest.NewServer(daemonpkg.NewReadOnlyRouter(
		daemonpkg.NewStore(t.TempDir()),
		daemonpkg.ServerOptions{Now: func() time.Time { return time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC) }},
	))
	t.Cleanup(server.Close)

	check := checkOpenClawConsumerURL(server.URL)
	if check.Name != "OpenClaw Consumer" || check.Status != "pass" {
		t.Fatalf("OpenClaw check = %#v, want pass", check)
	}
	if !strings.Contains(check.Detail, "snapshot_status=current") || !strings.Contains(check.Detail, "jobs=0") {
		t.Fatalf("OpenClaw detail = %q, want snapshot/count details", check.Detail)
	}
}

func TestDoctorLedgerHealthCheckPassesOnFreshStore(t *testing.T) {
	t.Chdir(t.TempDir())
	check := checkDaemonLedgerHealth(time.Now(), daemonpkg.LedgerHealthDefaultThresholds())
	if check.Name != "Daemon Ledger Health" {
		t.Fatalf("Name = %q", check.Name)
	}
	if check.Status != "pass" {
		t.Fatalf("Status = %q, want pass on missing daemon dir", check.Status)
	}
	if !strings.Contains(check.Detail, "no daemon store") {
		t.Fatalf("Detail = %q, want missing-store message", check.Detail)
	}
}

func TestDoctorLedgerHealthCheckWarnsOnStaleSnapshot(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	store := daemonpkg.NewStore(dir)
	rebuiltAt := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	now := rebuiltAt.Add(72 * time.Hour)
	set := daemonpkg.ProjectionSet{
		SchemaVersion: daemonpkg.ProjectionSchemaVersion,
		RebuiltAt:     rebuiltAt.Format(time.RFC3339Nano),
		LastEventID:   "evt-snap",
		Manifests:     map[daemonpkg.ProjectionName]daemonpkg.ProjectionManifest{},
	}
	if _, err := store.WriteProjectionSnapshot(set); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	check := checkDaemonLedgerHealth(now, daemonpkg.LedgerHealthDefaultThresholds())
	if check.Status != "warn" {
		t.Fatalf("Status = %q, want warn (72h old snapshot vs 24h threshold)", check.Status)
	}
	if !strings.Contains(check.Detail, "snapshot_age=") {
		t.Fatalf("Detail = %q, want snapshot_age in detail", check.Detail)
	}
	if !strings.Contains(check.Detail, "snapshot age") {
		t.Fatalf("Detail = %q, want snapshot-age warn reason", check.Detail)
	}
}

func TestDoctorLedgerHealthCheckSurfacesArchiveCount(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	store := daemonpkg.NewStore(dir)
	if err := os.MkdirAll(store.Dir(), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, ts := range []string{"20260101T000000.000000000Z", "20260102T000000.000000000Z"} {
		path := filepath.Join(store.Dir(), "ledger."+ts+".jsonl")
		if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
			t.Fatalf("plant archive: %v", err)
		}
	}
	check := checkDaemonLedgerHealth(time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC), daemonpkg.LedgerHealthDefaultThresholds())
	if check.Status != "pass" {
		t.Fatalf("Status = %q, want pass with 2 archives under threshold of 20", check.Status)
	}
	if !strings.Contains(check.Detail, "archives=2") {
		t.Fatalf("Detail = %q, want archives=2", check.Detail)
	}
	if !strings.Contains(check.Detail, "oldest_archive=2026-01-01") {
		t.Fatalf("Detail = %q, want oldest_archive timestamp", check.Detail)
	}
}

func TestAoDoctorHistograms(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	store := daemonpkg.NewStore(t.TempDir())
	queue := daemonpkg.NewQueue(store, daemonpkg.QueueOptions{
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
	})
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-phase",
		JobID:     "job-phase",
		JobType:   daemonpkg.JobTypeRPIPhase,
		Payload:   map[string]any{"phase_name": "discovery"},
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit phase: %v", err)
	}
	claim, err := queue.ClaimJob("job-phase", "worker", daemonpkg.QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim phase: %v", err)
	}
	now = now.Add(12 * time.Second)
	if _, err := queue.CompleteJob(daemonpkg.CompleteJobInput{
		JobID:      "job-phase",
		RequestID:  "req-phase-complete",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker",
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("complete phase: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-wiki",
		JobID:     "job-wiki",
		JobType:   daemonpkg.JobTypeWikiForge,
		Payload:   map[string]any{"worker_kind": "codex"},
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit wiki: %v", err)
	}
	server := httptest.NewServer(daemonpkg.NewReadOnlyRouter(store, daemonpkg.ServerOptions{Now: func() time.Time { return now }}))
	t.Cleanup(server.Close)

	check := checkDaemonTelemetryURL(server.URL)
	if check.Name != "Daemon Telemetry" || check.Status != "pass" {
		t.Fatalf("telemetry check = %#v, want pass", check)
	}
	for _, want := range []string{"phase_latency=discovery", "p50=12s", "worker_kind_24h=codex=1", "failure_rate=rpi.phase 0/1"} {
		if !strings.Contains(check.Detail, want) {
			t.Fatalf("telemetry detail = %q, want %q", check.Detail, want)
		}
	}
}

func TestDoctorRuntimeChecksWarnWhenUnavailable(t *testing.T) {
	baseURL := "http://127.0.0.1:1"
	for _, check := range []doctorCheck{
		checkDaemonRuntimeURL(baseURL),
		checkOpenClawConsumerURL(baseURL),
	} {
		if check.Status != "warn" || check.Required {
			t.Fatalf("%s = %#v, want non-required warning", check.Name, check)
		}
		if !strings.Contains(check.Detail, "unavailable") {
			t.Fatalf("%s detail = %q, want unavailable detail", check.Name, check.Detail)
		}
	}
}

func TestDoctorGasCityBridgeCheckUsesDiagnostics(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.14.0"})
	mock.on("status --json", gcMockHandler{Stdout: `{"city":"test","controller":{"running":true,"pid":99},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`})

	check := checkGasCityBridgeWith("", mock.execCommand, mock.lookPathFn)
	if check.Name != "GasCity Bridge" || check.Status != "pass" {
		t.Fatalf("GasCity check = %#v, want pass", check)
	}
	for _, want := range []string{"binary=true", "version=0.14.0", "api=true", "ready=true"} {
		if !strings.Contains(check.Detail, want) {
			t.Fatalf("GasCity detail = %q, want %q", check.Detail, want)
		}
	}

	missing := newGCMock()
	missing.binaryAvailable = false
	check = checkGasCityBridgeWith("", missing.execCommand, missing.lookPathFn)
	if check.Status != "warn" || check.Required {
		t.Fatalf("missing GasCity check = %#v, want non-required warning", check)
	}
	if !strings.Contains(check.Detail, "binary=false") || !strings.Contains(check.Detail, "gc binary not found") {
		t.Fatalf("missing GasCity detail = %q, want binary diagnostic", check.Detail)
	}
}

func TestDoctorProductRuntimeFailsClosedWithoutGasCityAPI(t *testing.T) {
	missing := newGCMock()
	missing.binaryAvailable = false
	check := checkGasCityProductRuntimeWith("", missing.execCommand, missing.lookPathFn)
	if check.Name != "GasCity Product Runtime" || check.Status != "fail" || !check.Required {
		t.Fatalf("missing product runtime check = %#v, want required fail", check)
	}

	unready := newGCMock()
	unready.on("version", gcMockHandler{Stdout: "0.14.0"})
	unready.on("status --json", gcMockHandler{Stdout: `{"city":"test","controller":{"running":false,"pid":0},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`})
	check = checkGasCityProductRuntimeWith("", unready.execCommand, unready.lookPathFn)
	if check.Status != "fail" || !strings.Contains(check.Detail, "api=true") || !strings.Contains(check.Detail, "ready=false") {
		t.Fatalf("unready product runtime check = %#v, want API reached but readiness fail", check)
	}

	ready := newGCMock()
	ready.on("version", gcMockHandler{Stdout: "0.14.0"})
	ready.on("status --json", gcMockHandler{Stdout: `{"city":"test","controller":{"running":true,"pid":99},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`})
	check = checkGasCityProductRuntimeWith("", ready.execCommand, ready.lookPathFn)
	if check.Status != "pass" || !check.Required {
		t.Fatalf("ready product runtime check = %#v, want required pass", check)
	}
}

func TestGatherDoctorChecksIncludesProductRuntimeSurfaces(t *testing.T) {
	checks := gatherDoctorChecks()
	names := map[string]bool{}
	for _, check := range checks {
		names[check.Name] = true
	}
	for _, name := range []string{"Daemon Runtime", "GasCity Bridge", "OpenClaw Consumer"} {
		if !names[name] {
			t.Fatalf("gatherDoctorChecks missing %q in %#v", name, names)
		}
	}
}
