// practices: [agile-manifesto, dora-metrics]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

func TestExecutionPacket_RoundTripWithCriteria(t *testing.T) {
	original := executionPacket{
		SchemaVersion: 1,
		Objective:     "round trip criteria",
		RunID:         "run-1",
		EpicID:        "soc-bcrn",
		Density: &rpi.ExecutionPacketDensity{
			Intent: "round trip criteria",
			Boundary: rpi.ExecutionPacketBoundary{
				BoundedContext: "agentops",
				NonGoals:       []string{"unrelated refactors"},
				WriteScope:     []string{"cli/cmd/ao/rpi_execution_packet.go"},
			},
			Evidence:   []string{"go test ./cmd/ao -run ExecutionPacket"},
			Decision:   "align schema and runtime packet fields",
			Constraint: []string{"do not touch doctor workspace"},
			NextAction: "/crank soc-bcrn",
		},
		Artifacts: &rpi.ExecutionPacketArtifacts{
			ResearchPath:     ".agents/research/topic.md",
			PlanPath:         ".agents/plans/topic.md",
			PreMortemPath:    ".agents/council/pre-mortem-topic.md",
			RankedPacketPath: ".agents/rpi/ranked-packet.json",
		},
		ContractSurfaces: []string{"docs/contracts/repo-execution-profile.md"},
		TrackerMode:      "bd",
		TestLevels: &rpi.ExecutionPacketTestLevels{
			Required:    []string{"L0", "L1"},
			Recommended: []string{"L2"},
			Rationale:   "schema fixture covers loop-density handoff fields",
		},
		RankedPacketPath:   ".agents/rpi/ranked-packet.json",
		DiscoveryTimestamp: "2026-05-16T00:00:00Z",
		EpicCriteria: []rpi.Criterion{
			{
				ID:               "ac-soc-bcrn.1",
				Description:      "executionPacket struct extends with criteria fields",
				CheckType:        "test_pass",
				CheckCommand:     "go test ./cmd/ao/...",
				EvidencePath:     ".agents/rpi/test-output.txt",
				EvidenceRequired: true,
				Weight:           1.0,
				Optional:         false,
			},
		},
		BeadCriteria: map[string][]rpi.Criterion{
			"soc-bcrn.1.1": {
				{
					ID:               "ac-soc-bcrn.1.1.1",
					Description:      "JSON schema for execution packet exists and parses",
					CheckType:        "file_exists",
					EvidencePath:     "schemas/execution-packet.schema.json",
					EvidenceRequired: true,
					Weight:           0.5,
					Optional:         false,
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded executionPacket
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("round-trip mismatch:\noriginal=%#v\ndecoded =%#v", original, decoded)
	}
}

func TestExecutionPacket_BackCompatV1NoCriteria(t *testing.T) {
	v1JSON := `{
		"schema_version": 1,
		"objective": "v1 packet",
		"run_id": "run-v1",
		"epic_id": "ag-100",
		"contract_surfaces": ["docs/contracts/repo-execution-profile.md"],
		"tracker_mode": "bd",
		"done_criteria": ["all tests pass", "coverage above threshold"]
	}`

	var packet executionPacket
	if err := json.Unmarshal([]byte(v1JSON), &packet); err != nil {
		t.Fatalf("v1 unmarshal: %v", err)
	}

	if packet.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", packet.SchemaVersion)
	}
	if packet.Objective != "v1 packet" {
		t.Errorf("Objective = %q, want %q", packet.Objective, "v1 packet")
	}
	wantDone := []string{"all tests pass", "coverage above threshold"}
	if !reflect.DeepEqual(packet.DoneCriteria, wantDone) {
		t.Errorf("DoneCriteria = %v, want %v", packet.DoneCriteria, wantDone)
	}
	if packet.EpicCriteria != nil {
		t.Errorf("EpicCriteria = %v, want nil for v1 packet", packet.EpicCriteria)
	}
	if packet.BeadCriteria != nil {
		t.Errorf("BeadCriteria = %v, want nil for v1 packet", packet.BeadCriteria)
	}
}

func TestExecutionPacket_CustomRubricCriterion(t *testing.T) {
	original := rpi.Criterion{
		ID:               "ac-soc-bcrn.2.1",
		Description:      "council:vibe judges packet readiness",
		CheckType:        "custom_rubric",
		EvidenceRequired: false,
		Weight:           1.0,
		Optional:         false,
		AgentJudge:       "council:vibe",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded rpi.Criterion
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("custom_rubric round-trip mismatch:\noriginal=%#v\ndecoded =%#v", original, decoded)
	}
	if decoded.AgentJudge != "council:vibe" {
		t.Errorf("AgentJudge = %q, want %q", decoded.AgentJudge, "council:vibe")
	}
}

func TestExecutionPacket_OmitEmptyCriteria(t *testing.T) {
	packet := executionPacket{
		SchemaVersion:    1,
		Objective:        "omit empty",
		ContractSurfaces: []string{},
		TrackerMode:      "bd",
	}

	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if bytes.Contains(data, []byte(`"epic_criteria"`)) {
		t.Errorf("epic_criteria key should be omitted when empty; got: %s", data)
	}
	if bytes.Contains(data, []byte(`"bead_criteria"`)) {
		t.Errorf("bead_criteria key should be omitted when empty; got: %s", data)
	}
}

func TestWriteExecutionPacketSeed_PopulatesLoopDensityFields(t *testing.T) {
	tmpDir := t.TempDir()
	state := &phasedState{
		Goal:       "ship loop density",
		EpicID:     "soc-z3qo.1",
		RunID:      "density-run",
		Complexity: ComplexityStandard,
		Opts:       phasedEngineOptions{},
	}

	if err := writeExecutionPacketSeed(tmpDir, state); err != nil {
		t.Fatalf("writeExecutionPacketSeed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".agents", "rpi", "execution-packet.json"))
	if err != nil {
		t.Fatalf("read execution packet: %v", err)
	}
	var packet executionPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatalf("unmarshal execution packet: %v", err)
	}

	if packet.Density == nil {
		t.Fatalf("Density = nil")
	}
	if packet.Density.Intent != "ship loop density" {
		t.Fatalf("Density.Intent = %q", packet.Density.Intent)
	}
	if packet.Density.Boundary.BoundedContext == "" {
		t.Fatalf("Density.Boundary.BoundedContext is empty")
	}
	if packet.Density.NextAction != "/crank soc-z3qo.1" {
		t.Fatalf("Density.NextAction = %q", packet.Density.NextAction)
	}
	if packet.Artifacts == nil || packet.Artifacts.RankedPacketPath != executionPacketRankedPacketPath {
		t.Fatalf("Artifacts = %#v, want ranked packet path", packet.Artifacts)
	}
	if packet.TestLevels == nil || !reflect.DeepEqual(packet.TestLevels.Required, []string{"L0", "L1"}) {
		t.Fatalf("TestLevels = %#v, want required L0/L1", packet.TestLevels)
	}
	if packet.RankedPacketPath != executionPacketRankedPacketPath {
		t.Fatalf("RankedPacketPath = %q", packet.RankedPacketPath)
	}
	if _, err := time.Parse(time.RFC3339, packet.DiscoveryTimestamp); err != nil {
		t.Fatalf("DiscoveryTimestamp = %q is not RFC3339: %v", packet.DiscoveryTimestamp, err)
	}
}

func TestWriteExecutionPacketSeed_IncludesRepoProfileAndPlanEpic(t *testing.T) {
	tmpDir := t.TempDir()
	contractsDir := filepath.Join(tmpDir, "docs", "contracts")
	if err := os.MkdirAll(contractsDir, 0o750); err != nil {
		t.Fatalf("create contracts dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "repo-execution-profile.md"), []byte("# Profile\n"), 0o600); err != nil {
		t.Fatalf("write profile doc: %v", err)
	}
	profileJSON := `{
		"validation_commands": ["scripts/pre-push-gate.sh --fast"],
		"validation_lanes": [
			{
				"name": "pre-push-fast",
				"command": "scripts/pre-push-gate.sh --fast",
				"read_only": true,
				"writes_artifacts": false,
				"isolated_agents_home": true,
				"release_only": false,
				"mutation_escape_hatch": null,
				"cost_class": "standard",
				"auto_select": "default",
				"timeout_seconds": 180
			}
		]
	}`
	if err := os.WriteFile(repoExecutionProfileJSONPath(tmpDir), []byte(profileJSON), 0o600); err != nil {
		t.Fatalf("write profile json: %v", err)
	}

	state := &phasedState{
		Goal:       "profile-backed packet",
		EpicID:     "plan:.agents/plans/profile-plan.md",
		RunID:      "profile-run",
		Complexity: ComplexityFull,
		TestFirst:  true,
		Opts:       phasedEngineOptions{},
	}
	if err := writeExecutionPacketSeed(tmpDir, state); err != nil {
		t.Fatalf("writeExecutionPacketSeed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".agents", "rpi", "execution-packet.json"))
	if err != nil {
		t.Fatalf("read execution packet: %v", err)
	}
	var packet executionPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatalf("unmarshal execution packet: %v", err)
	}

	if packet.PlanPath != ".agents/plans/profile-plan.md" {
		t.Fatalf("PlanPath = %q", packet.PlanPath)
	}
	if !containsProgramContract(packet.ContractSurfaces, "docs/contracts/repo-execution-profile.md") ||
		!containsProgramContract(packet.ContractSurfaces, "docs/contracts/repo-execution-profile.json") {
		t.Fatalf("ContractSurfaces = %#v, want repo execution profile surfaces", packet.ContractSurfaces)
	}
	if !stringSlicesEqual(packet.ValidationCommands, []string{"scripts/pre-push-gate.sh --fast"}) {
		t.Fatalf("ValidationCommands = %#v", packet.ValidationCommands)
	}
	if len(packet.ValidationLanes) != 1 || packet.ValidationLanes[0].Name != "pre-push-fast" {
		t.Fatalf("ValidationLanes = %#v", packet.ValidationLanes)
	}
	if packet.BeadID != "" {
		t.Fatalf("BeadID = %q, want empty for plan-file epic", packet.BeadID)
	}
	if packet.TestLevels == nil || !stringSlicesEqual(packet.TestLevels.Required, []string{"L0", "L1", "L2"}) {
		t.Fatalf("TestLevels = %#v, want full proof floor", packet.TestLevels)
	}
	if packet.Density == nil || !stringSlicesEqual(packet.Density.Evidence, []string{"scripts/pre-push-gate.sh --fast"}) {
		t.Fatalf("Density = %#v, want profile command evidence", packet.Density)
	}
}

func TestExecutionPacketLoopDensityHelpers_CoverFallbackBranches(t *testing.T) {
	tmpDir := t.TempDir()
	packet := &executionPacket{
		Objective:          "objective fallback",
		ContractSurfaces:   []string{"schemas/execution-packet.schema.json"},
		ValidationCommands: []string{"go test ./cmd/ao -run ExecutionPacket"},
	}
	density := executionPacketDensityForState(tmpDir, nil, packet)
	if density.Intent != "objective fallback" {
		t.Fatalf("density intent = %q", density.Intent)
	}
	if !stringSlicesEqual(density.Boundary.WriteScope, packet.ContractSurfaces) {
		t.Fatalf("density write scope = %#v", density.Boundary.WriteScope)
	}
	if !stringSlicesEqual(density.Evidence, packet.ValidationCommands) {
		t.Fatalf("density evidence = %#v", density.Evidence)
	}
	if density.NextAction != "/crank .agents/rpi/execution-packet.json" {
		t.Fatalf("density next action = %q", density.NextAction)
	}
	if got := executionPacketBoundedContext(string(filepath.Separator)); got != "repository" {
		t.Fatalf("executionPacketBoundedContext(/) = %q", got)
	}
	if got := executionPacketTrackingRepoRoot(" "); got == "" {
		t.Fatalf("executionPacketTrackingRepoRoot(empty) returned empty")
	}
	if got := executionPacketBeadID(nil); got != "" {
		t.Fatalf("executionPacketBeadID(nil) = %q", got)
	}
	if got := executionPacketBeadID(&phasedState{EpicID: "plan:.agents/plans/x.md"}); got != "" {
		t.Fatalf("executionPacketBeadID(plan epic) = %q", got)
	}

	fast := executionPacketTestLevelsForState(&phasedState{Complexity: ComplexityFast})
	if !stringSlicesEqual(fast.Required, []string{"L0"}) || !stringSlicesEqual(fast.Recommended, []string{"L1"}) {
		t.Fatalf("fast test levels = %#v", fast)
	}
	testFirst := executionPacketTestLevelsForState(&phasedState{Complexity: ComplexityStandard, TestFirst: true})
	if !stringSlicesEqual(testFirst.Required, []string{"L0", "L1", "L2"}) ||
		!stringSlicesEqual(testFirst.Recommended, []string{"L3"}) {
		t.Fatalf("test-first levels = %#v", testFirst)
	}
}

func TestExecutionPacketStorageHelpers_CoverArchiveAndRestore(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("BEADS_DIR", filepath.Join(tmpDir, "custom-beads"))
	if got := executionPacketBeadsDir(tmpDir); got != filepath.Join(tmpDir, "custom-beads") {
		t.Fatalf("executionPacketBeadsDir env = %q", got)
	}
	t.Setenv("BEADS_DIR", "")
	if err := os.Mkdir(filepath.Join(tmpDir, ".beads"), 0o750); err != nil {
		t.Fatalf("create .beads: %v", err)
	}
	if got := executionPacketBeadsDir(tmpDir); got != filepath.Join(tmpDir, ".beads") {
		t.Fatalf("executionPacketBeadsDir .beads = %q", got)
	}

	data := []byte(`{"schema_version":1,"objective":"archive","contract_surfaces":[],"tracker_mode":"bd"}` + "\n")
	if err := writeExecutionPacketDataToRoot(tmpDir, "run-archive", data); err != nil {
		t.Fatalf("writeExecutionPacketDataToRoot: %v", err)
	}
	archivePath := filepath.Join(tmpDir, ".agents", "rpi", "runs", "run-archive", "execution-packet.json")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive stat: %v", err)
	}

	snap, err := captureExecutionPacketAliasSnapshot(tmpDir)
	if err != nil {
		t.Fatalf("capture snapshot: %v", err)
	}
	latestPath := filepath.Join(tmpDir, ".agents", "rpi", "execution-packet.json")
	if err := os.WriteFile(latestPath, []byte("changed\n"), 0o600); err != nil {
		t.Fatalf("overwrite latest: %v", err)
	}
	if err := snap.restore(); err != nil {
		t.Fatalf("restore existing snapshot: %v", err)
	}
	restored, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("read restored latest: %v", err)
	}
	if !bytes.Equal(restored, data) {
		t.Fatalf("restored data = %q, want %q", restored, data)
	}

	missingRoot := filepath.Join(tmpDir, "missing")
	missingSnap, err := captureExecutionPacketAliasSnapshot(missingRoot)
	if err != nil {
		t.Fatalf("capture missing snapshot: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(missingSnap.path), 0o750); err != nil {
		t.Fatalf("create missing snapshot dir: %v", err)
	}
	if err := os.WriteFile(missingSnap.path, []byte("dry-run\n"), 0o600); err != nil {
		t.Fatalf("write dry-run alias: %v", err)
	}
	if err := missingSnap.restore(); err != nil {
		t.Fatalf("restore missing snapshot: %v", err)
	}
	if _, err := os.Stat(missingSnap.path); !os.IsNotExist(err) {
		t.Fatalf("missing snapshot path stat = %v, want removed", err)
	}
	if err := (*executionPacketAliasSnapshot)(nil).restore(); err != nil {
		t.Fatalf("nil snapshot restore: %v", err)
	}
}

func TestExecutionPacketStorageHelpers_ErrorBranches(t *testing.T) {
	tmpDir := t.TempDir()

	rootFile := filepath.Join(tmpDir, "root-file")
	if err := os.WriteFile(rootFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	if err := writeExecutionPacketData(rootFile, nil, "", []byte("{}\n")); err == nil {
		t.Fatalf("expected writeExecutionPacketData to fail for file root")
	}
	if err := writeExecutionPacketDataToRoot(rootFile, "", []byte("{}\n")); err == nil {
		t.Fatalf("expected writeExecutionPacketDataToRoot to fail for file root")
	}
	if _, err := captureExecutionPacketAliasSnapshot(rootFile); err == nil {
		t.Fatalf("expected capture snapshot to fail for file root")
	}

	contractsDir := filepath.Join(tmpDir, "docs", "contracts")
	if err := os.MkdirAll(contractsDir, 0o750); err != nil {
		t.Fatalf("create contracts dir: %v", err)
	}
	if err := os.Mkdir(repoExecutionProfileJSONPath(tmpDir), 0o750); err != nil {
		t.Fatalf("create profile path as directory: %v", err)
	}
	if _, err := loadRepoExecutionProfile(tmpDir); err == nil {
		t.Fatalf("expected read repo execution profile error")
	}

	runRoot := filepath.Join(tmpDir, "run-root")
	if err := os.MkdirAll(filepath.Join(runRoot, ".agents", "rpi"), 0o750); err != nil {
		t.Fatalf("create rpi dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runRoot, ".agents", "rpi", "runs"), []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("write runs file: %v", err)
	}
	if err := writeExecutionPacketDataToRoot(runRoot, "run-mkdir-fails", []byte("{}\n")); err == nil {
		t.Fatalf("expected run archive mkdir error")
	}

	archiveRoot := filepath.Join(tmpDir, "archive-root")
	archivePath := filepath.Join(archiveRoot, ".agents", "rpi", "runs", "run-write-fails", "execution-packet.json")
	if err := os.MkdirAll(archivePath, 0o750); err != nil {
		t.Fatalf("create archive path as directory: %v", err)
	}
	if err := writeExecutionPacketDataToRoot(archiveRoot, "run-write-fails", []byte("{}\n")); err == nil {
		t.Fatalf("expected run archive write error")
	}

	if err := (&executionPacketAliasSnapshot{path: filepath.Join(rootFile, "alias.json")}).restore(); err == nil {
		t.Fatalf("expected missing snapshot restore remove error")
	}
	if err := (&executionPacketAliasSnapshot{
		path:    filepath.Join(rootFile, "alias.json"),
		data:    []byte("original\n"),
		existed: true,
	}).restore(); err == nil {
		t.Fatalf("expected existing snapshot restore mkdir error")
	}
	writeErrorPath := filepath.Join(tmpDir, "alias-dir", "execution-packet.json")
	if err := os.MkdirAll(writeErrorPath, 0o750); err != nil {
		t.Fatalf("create alias path as directory: %v", err)
	}
	if err := (&executionPacketAliasSnapshot{
		path:    writeErrorPath,
		data:    []byte("original\n"),
		existed: true,
	}).restore(); err == nil {
		t.Fatalf("expected existing snapshot restore write error")
	}
}

func TestLoadRepoExecutionProfile_InvalidJSONReturnsContext(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "docs", "contracts"), 0o750); err != nil {
		t.Fatalf("create contracts dir: %v", err)
	}
	if err := os.WriteFile(repoExecutionProfileJSONPath(tmpDir), []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid profile: %v", err)
	}
	if _, err := loadRepoExecutionProfile(tmpDir); err == nil {
		t.Fatalf("expected invalid profile error")
	}
}

func TestWriteExecutionPacketSeed_ReturnsProfileAndProgramErrors(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "docs", "contracts"), 0o750); err != nil {
		t.Fatalf("create contracts dir: %v", err)
	}
	if err := os.WriteFile(repoExecutionProfileJSONPath(tmpDir), []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid profile: %v", err)
	}
	state := &phasedState{Goal: "bad profile", RunID: "bad-profile", Opts: phasedEngineOptions{}}
	if err := writeExecutionPacketSeed(tmpDir, state); err == nil {
		t.Fatalf("expected writeExecutionPacketSeed profile error")
	}

	programRoot := t.TempDir()
	programState := &phasedState{
		Goal:        "bad program",
		RunID:       "bad-program",
		ProgramPath: "PROGRAM.md",
		Opts:        phasedEngineOptions{},
	}
	if err := os.WriteFile(filepath.Join(programRoot, "PROGRAM.md"), []byte(" \n"), 0o600); err != nil {
		t.Fatalf("write invalid program: %v", err)
	}
	if err := writeExecutionPacketSeed(programRoot, programState); err == nil {
		t.Fatalf("expected writeExecutionPacketSeed program error")
	}
}

func TestExecutionPacketSchema_CoversLoopDensityFixture(t *testing.T) {
	schemaPath := findRepoFileForTest(t, "schemas", "execution-packet.schema.json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema struct {
		AdditionalProperties bool           `json:"additionalProperties"`
		Properties           map[string]any `json:"properties"`
		Defs                 map[string]any `json:"$defs"`
		Required             []string       `json:"required"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema.AdditionalProperties {
		t.Fatalf("schema additionalProperties = true, want false")
	}

	fixture := executionPacket{
		SchemaVersion: 1,
		Objective:     "fixture",
		Density: &rpi.ExecutionPacketDensity{
			Intent: "fixture",
			Boundary: rpi.ExecutionPacketBoundary{
				BoundedContext: "agentops",
				NonGoals:       []string{"doctor workspace"},
				WriteScope:     []string{"schemas/execution-packet.schema.json"},
			},
			Evidence:   []string{"go test ./cmd/ao -run ExecutionPacket"},
			Decision:   "prove schema names every loop-density field",
			Constraint: []string{"additive schema change only"},
			NextAction: "/crank .agents/rpi/execution-packet.json",
		},
		Artifacts: &rpi.ExecutionPacketArtifacts{
			ResearchPath:     ".agents/research/fixture.md",
			PlanPath:         ".agents/plans/fixture.md",
			PreMortemPath:    ".agents/council/pre-mortem-fixture.md",
			RankedPacketPath: executionPacketRankedPacketPath,
		},
		ContractSurfaces: []string{"docs/contracts/repo-execution-profile.md"},
		TrackerMode:      "bd",
		TestLevels: &rpi.ExecutionPacketTestLevels{
			Required:    []string{"L0", "L1"},
			Recommended: []string{"L2"},
			Rationale:   "fixture",
		},
		RankedPacketPath:   executionPacketRankedPacketPath,
		DiscoveryTimestamp: "2026-05-16T00:00:00Z",
	}
	payload, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	var topLevel map[string]any
	if err := json.Unmarshal(payload, &topLevel); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	for key := range topLevel {
		if _, ok := schema.Properties[key]; !ok {
			t.Fatalf("schema missing top-level property %q for fixture JSON: %s", key, payload)
		}
	}
	for _, def := range []string{"Density", "Boundary", "Artifacts", "TestLevels", "TestLevel"} {
		if _, ok := schema.Defs[def]; !ok {
			t.Fatalf("schema missing $defs.%s", def)
		}
	}
}

func findRepoFileForTest(t *testing.T, parts ...string) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(append([]string{dir}, parts...)...)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo file %s", filepath.Join(parts...))
		}
		dir = parent
	}
}
