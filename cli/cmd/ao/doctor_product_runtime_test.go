package main

import (
	"net/http/httptest"
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
