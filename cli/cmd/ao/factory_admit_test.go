// practices: [microservices, team-topologies]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/spf13/cobra"
)

func TestFactoryAdmitSubmitsLocalPilotRPIHandoff(t *testing.T) {
	cwd, server, queue := newDaemonJobsCommandFixture(t)
	workPath := filepath.Join(cwd, "work-order.json")
	writeFactoryAdmitWorkOrder(t, workPath, testFactoryAdmitWorkOrder())
	restore := setFactoryAdmitGlobalsForTest(cwd, server.URL, "secret-token")
	defer restore()
	factoryAdmitWorkOrder = "@" + workPath
	factoryAdmitRunID = "factory-run-test"
	factoryAdmitLocalPilot = true
	factoryAdmitRPIHandoff = true
	factoryAdmitExecutionPacket = ".agents/rpi/execution-packet.json"
	factoryAdmitEpicID = "soc-ff7b.7"
	output = "json"

	var out testOutputBuffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runFactoryAdmitCommand(cmd, nil); err != nil {
		t.Fatalf("factory admit: %v\noutput=%s", err, out.String())
	}

	var response submitDaemonJobResponse
	if err := json.Unmarshal([]byte(out.String()), &response); err != nil {
		t.Fatalf("decode response: %v\n%s", err, out.String())
	}
	if response.JobType != daemonpkg.JobTypeFactoryLocalPilot || response.Status != daemonpkg.JobStatusQueued {
		t.Fatalf("response = %#v, want queued factory.local-pilot", response)
	}
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot.Jobs) != 1 {
		t.Fatalf("jobs = %#v, want one submitted job", snapshot.Jobs)
	}
	spec, err := daemonpkg.FactoryLocalPilotJobSpecFromPayload(snapshot.Jobs[0].Payload)
	if err != nil {
		t.Fatalf("decode submitted payload: %v", err)
	}
	if spec.Mode != daemonpkg.FactoryAdmissionModeRPIHandoff || spec.Handoff.Kind != daemonpkg.FactoryHandoffRPI {
		t.Fatalf("submitted spec = %#v, want rpi handoff", spec)
	}
	if spec.Handoff.ExecutionPacketPath != ".agents/rpi/execution-packet.json" || spec.Handoff.EpicID != "soc-ff7b.7" {
		t.Fatalf("handoff = %#v", spec.Handoff)
	}
}

func TestFactoryAdmitRequiresExecutionPacketForRPIHandoff(t *testing.T) {
	cwd := t.TempDir()
	workPath := filepath.Join(cwd, "work-order.json")
	writeFactoryAdmitWorkOrder(t, workPath, testFactoryAdmitWorkOrder())
	restore := setFactoryAdmitGlobalsForTest(cwd, "http://127.0.0.1:1", "secret-token")
	defer restore()
	factoryAdmitWorkOrder = "@" + workPath
	factoryAdmitRunID = "factory-run-test"
	factoryAdmitRPIHandoff = true

	cmd := &cobra.Command{}
	err := runFactoryAdmitCommand(cmd, nil)
	if err == nil {
		t.Fatal("factory admit accepted rpi handoff without execution packet")
	}
}

type testOutputBuffer struct {
	data []byte
}

func (b *testOutputBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *testOutputBuffer) String() string {
	return string(b.data)
}

func setFactoryAdmitGlobalsForTest(cwd, url, token string) func() {
	prevProjectDir := testProjectDir
	prevURL := daemonURL
	prevToken := daemonToken
	prevTokenFile := daemonTokenFile
	prevOutput := output
	prevWorkOrder := factoryAdmitWorkOrder
	prevRunID := factoryAdmitRunID
	prevLocalPilot := factoryAdmitLocalPilot
	prevRPIHandoff := factoryAdmitRPIHandoff
	prevExecutionPacket := factoryAdmitExecutionPacket
	prevEpicID := factoryAdmitEpicID
	testProjectDir = cwd
	daemonURL = url
	daemonToken = token
	daemonTokenFile = ""
	output = "table"
	factoryAdmitWorkOrder = ""
	factoryAdmitRunID = ""
	factoryAdmitLocalPilot = false
	factoryAdmitRPIHandoff = false
	factoryAdmitExecutionPacket = ""
	factoryAdmitEpicID = ""
	return func() {
		testProjectDir = prevProjectDir
		daemonURL = prevURL
		daemonToken = prevToken
		daemonTokenFile = prevTokenFile
		output = prevOutput
		factoryAdmitWorkOrder = prevWorkOrder
		factoryAdmitRunID = prevRunID
		factoryAdmitLocalPilot = prevLocalPilot
		factoryAdmitRPIHandoff = prevRPIHandoff
		factoryAdmitExecutionPacket = prevExecutionPacket
		factoryAdmitEpicID = prevEpicID
	}
}

func writeFactoryAdmitWorkOrder(t *testing.T, path string, work daemonpkg.FactoryWorkOrder) {
	t.Helper()
	data, err := json.Marshal(work)
	if err != nil {
		t.Fatalf("marshal work order: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write work order: %v", err)
	}
}

func testFactoryAdmitWorkOrder() daemonpkg.FactoryWorkOrder {
	return daemonpkg.FactoryWorkOrder{
		SchemaVersion: daemonpkg.FactoryAdmissionJobSpecSchemaVersion,
		WorkOrderID:   "factory-work-admit-test",
		GeneratedAt:   "2026-05-04T23:30:00Z",
		ExpiresAt:     "2026-05-05T00:30:00Z",
		BaseSHA:       "abcdef123456",
		Target: daemonpkg.FactoryTarget{
			Type:    daemonpkg.FactoryTargetBead,
			ID:      "soc-ff7b.7",
			Summary: "Submit factory admission from CLI",
		},
		AllowedFiles: []string{
			"cli/internal/daemon/factory_admission_executor.go",
		},
		ValidationCommands: []string{
			"cd cli && go test ./internal/daemon -run FactoryAdmission",
		},
		LandingPolicy:         daemonpkg.FactoryLandingPolicyManualPR,
		DigestPolicy:          daemonpkg.FactoryDigestPolicyRequired,
		OpenPRBlockers:        []daemonpkg.FactoryOpenPRBlocker{},
		UnknownEvidencePolicy: daemonpkg.FactoryUnknownEvidenceBlock,
		MainCIBaseline: daemonpkg.FactoryMainCIBaseline{
			Status:     daemonpkg.FactoryCIStatusGreen,
			RunID:      "123",
			CheckedAt:  "2026-05-04T23:29:00Z",
			FailedJobs: []string{},
		},
	}
}
