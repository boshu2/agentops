package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rpiProgramMarkdown = `# PROGRAM.md

## Objective

Drive bounded autodev experiments through RPI.

## Mutable Scope

- cli/cmd/ao/**
- cli/internal/autodev/**

## Immutable Scope

- hooks/**

## Experiment Unit

One bounded TDD slice that updates tests, implementation, and docs.

## Validation Commands

- ` + "`cd cli && go test ./cmd/ao/... ./internal/autodev/...`" + `
- ` + "`bash scripts/check-worktree-disposition.sh`" + `

## Decision Policy

- Keep changes only when validation commands pass.
- Revert non-improving or out-of-scope work.

## Escalation Rules

Open a bead when required work escapes mutable scope.

## Stop Conditions

- Validation bundle is green.
- No out-of-scope findings remain.
`

func TestInitPhasedState_AttachesProgramPath(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "PROGRAM.md"), []byte(rpiProgramMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}

	state, _, _, err := initPhasedState(tmp, defaultPhasedEngineOptions(), []string{"build the loop"})
	if err != nil {
		t.Fatalf("initPhasedState() error = %v", err)
	}
	if state.ProgramPath != "PROGRAM.md" {
		t.Fatalf("ProgramPath = %q, want PROGRAM.md", state.ProgramPath)
	}
}

func TestInitPhasedState_InvalidProgramFails(t *testing.T) {
	tmp := t.TempDir()
	badProgram := strings.ReplaceAll(rpiProgramMarkdown, "## Mutable Scope\n\n- cli/cmd/ao/**\n- cli/internal/autodev/**\n\n", "## Mutable Scope\n\n")
	if err := os.WriteFile(filepath.Join(tmp, "PROGRAM.md"), []byte(badProgram), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := initPhasedState(tmp, defaultPhasedEngineOptions(), []string{"build the loop"})
	if err == nil {
		t.Fatal("initPhasedState() error = nil, want invalid PROGRAM.md failure")
	}
	if !strings.Contains(err.Error(), "PROGRAM.md") {
		t.Fatalf("error = %v, want mention of PROGRAM.md", err)
	}
}

func TestBuildPromptForPhase_IncludesProgramContract(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "PROGRAM.md"), []byte(rpiProgramMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}

	state := &phasedState{
		Goal:        "build the loop",
		EpicID:      "ag-123",
		ProgramPath: "PROGRAM.md",
		Opts:        defaultPhasedEngineOptions(),
	}

	prompt, err := buildPromptForPhase(tmp, 1, state, nil)
	if err != nil {
		t.Fatalf("buildPromptForPhase() error = %v", err)
	}
	if !strings.Contains(prompt, "AUTODEV PROGRAM CONTRACT") {
		t.Fatalf("prompt missing program contract:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Read PROGRAM.md before any other repo exploration") {
		t.Fatalf("prompt missing read instruction:\n%s", prompt)
	}
}

func TestBuildPromptForPhase_IncludesProgramContractForAllPhases(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "PROGRAM.md"), []byte(rpiProgramMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}

	state := &phasedState{
		Goal:        "build the loop",
		EpicID:      "ag-123",
		ProgramPath: "PROGRAM.md",
		Opts:        defaultPhasedEngineOptions(),
	}

	for _, phaseNum := range []int{1, 2, 3} {
		prompt, err := buildPromptForPhase(tmp, phaseNum, state, nil)
		if err != nil {
			t.Fatalf("buildPromptForPhase(%d) error = %v", phaseNum, err)
		}
		if !strings.Contains(prompt, "AUTODEV PROGRAM CONTRACT") {
			t.Fatalf("phase %d prompt missing program contract:\n%s", phaseNum, prompt)
		}
		if !strings.Contains(prompt, "Read PROGRAM.md before any other repo exploration") {
			t.Fatalf("phase %d prompt missing read instruction:\n%s", phaseNum, prompt)
		}
	}
}

func TestWriteExecutionPacketSeed_UsesProgramContract(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "PROGRAM.md"), []byte(rpiProgramMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState().WithGoal("build the loop")
	state.Complexity = ComplexityStandard
	state.ProgramPath = "PROGRAM.md"
	state.RunID = "program-contract-run"

	if err := writeExecutionPacketSeed(tmp, state); err != nil {
		t.Fatalf("writeExecutionPacketSeed() error = %v", err)
	}

	packetPath := filepath.Join(tmp, ".agents", "rpi", "execution-packet.json")
	data, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatalf("read execution packet: %v", err)
	}
	archivedData, err := os.ReadFile(filepath.Join(tmp, ".agents", "rpi", "runs", state.RunID, executionPacketFile))
	if err != nil {
		t.Fatalf("read archived execution packet: %v", err)
	}
	if string(archivedData) != string(data) {
		t.Fatalf("archived execution packet does not match latest alias:\nlatest:\n%s\narchived:\n%s", data, archivedData)
	}

	var packet map[string]any
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatalf("unmarshal execution packet: %v", err)
	}
	if objective, _ := packet["objective"].(string); objective != "build the loop" {
		t.Fatalf("objective = %q", objective)
	}
	contracts, _ := packet["contract_surfaces"].([]any)
	if len(contracts) == 0 {
		t.Fatalf("contract_surfaces = %#v, want non-empty", packet["contract_surfaces"])
	}
	foundProgram := false
	for _, item := range contracts {
		if item == "PROGRAM.md" {
			foundProgram = true
			break
		}
	}
	if !foundProgram {
		t.Fatalf("contract_surfaces = %#v, want PROGRAM.md", packet["contract_surfaces"])
	}
	doneCriteria, _ := packet["done_criteria"].([]any)
	if len(doneCriteria) < 2 {
		t.Fatalf("done_criteria = %#v, want >= 2", packet["done_criteria"])
	}
}

func TestRunPhasedEngine_DryRunUsesResolvedProgramContract(t *testing.T) {
	for _, tc := range dryRunProgramContractCases() {
		t.Run(tc.name, func(t *testing.T) {
			assertDryRunUsesResolvedProgramContract(t, tc)
		})
	}
}

type dryRunProgramContractCase struct {
	name                  string
	setupFiles            func(t *testing.T, dir string)
	wantPath              string
	wantValidationCommand string
}

type dryRunProgramContractPacket struct {
	ContractSurfaces []string `json:"contract_surfaces"`
	AutodevProgram   struct {
		Path               string   `json:"path"`
		ValidationCommands []string `json:"validation_commands"`
	} `json:"autodev_program"`
}

func dryRunProgramContractCases() []dryRunProgramContractCase {
	return []dryRunProgramContractCase{
		{
			name:                  "PROGRAM preferred when both exist",
			setupFiles:            setupProgramPreferredContractFiles,
			wantPath:              "PROGRAM.md",
			wantValidationCommand: "echo program-preferred",
		},
		{
			name:                  "AUTODEV fallback when PROGRAM missing",
			setupFiles:            setupAutodevFallbackContractFiles,
			wantPath:              "AUTODEV.md",
			wantValidationCommand: "echo autodev-fallback",
		},
	}
}

func assertDryRunUsesResolvedProgramContract(t *testing.T, tc dryRunProgramContractCase) {
	t.Helper()

	tmpDir := setupDryRunProgramContractWorkspace(t, tc)
	state := runDryRunPhasedEngine(t, tmpDir)
	assertDryRunProgramPath(t, state, tc.wantPath)

	packetData := readDryRunExecutionPacket(t, tmpDir, state.RunID)
	packet := decodeDryRunProgramContractPacket(t, packetData)
	assertDryRunProgramContractPacket(t, packet, tc)
	assertDryRunOrchestrationLog(t, tmpDir)
}

func setupDryRunProgramContractWorkspace(t *testing.T, tc dryRunProgramContractCase) string {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "docs", "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "docs", "contracts", "repo-execution-profile.md"), []byte("# Repo Execution Profile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tc.setupFiles(t, tmpDir)
	return tmpDir
}

func setupProgramPreferredContractFiles(t *testing.T, dir string) {
	t.Helper()

	writeProgramContractVariant(t, dir, "PROGRAM.md", "echo program-preferred")
	writeProgramContractVariant(t, dir, "AUTODEV.md", "echo autodev-fallback")
}

func setupAutodevFallbackContractFiles(t *testing.T, dir string) {
	t.Helper()

	writeProgramContractVariant(t, dir, "AUTODEV.md", "echo autodev-fallback")
}

func writeProgramContractVariant(t *testing.T, dir, name, validationCommand string) {
	t.Helper()

	text := strings.Replace(rpiProgramMarkdown, "cd cli && go test ./cmd/ao/... ./internal/autodev/...", validationCommand, 1)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runDryRunPhasedEngine(t *testing.T, tmpDir string) phasedState {
	t.Helper()

	prevDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = prevDryRun })

	opts := defaultPhasedEngineOptions()
	opts.NoWorktree = true
	opts.SwarmFirst = false

	if err := runPhasedEngine(context.Background(), tmpDir, "drive bounded autodev experiments", opts); err != nil {
		t.Fatalf("runPhasedEngine() error = %v", err)
	}
	return readDryRunPhasedState(t, tmpDir)
}

func readDryRunPhasedState(t *testing.T, tmpDir string) phasedState {
	t.Helper()

	statePath := filepath.Join(tmpDir, ".agents", "rpi", phasedStateFile)
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read phased state: %v", err)
	}
	var state phasedState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatalf("unmarshal phased state: %v", err)
	}
	return state
}

func assertDryRunProgramPath(t *testing.T, state phasedState, wantPath string) {
	t.Helper()

	if state.ProgramPath != wantPath {
		t.Fatalf("ProgramPath = %q, want %q", state.ProgramPath, wantPath)
	}
}

func readDryRunExecutionPacket(t *testing.T, tmpDir, runID string) []byte {
	t.Helper()

	packetPath := filepath.Join(tmpDir, ".agents", "rpi", "execution-packet.json")
	packetData, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatalf("read execution packet: %v", err)
	}
	archivedPacketData, err := os.ReadFile(filepath.Join(tmpDir, ".agents", "rpi", "runs", runID, executionPacketFile))
	if err != nil {
		t.Fatalf("read archived execution packet: %v", err)
	}
	if string(archivedPacketData) != string(packetData) {
		t.Fatalf("archived execution packet does not match latest alias:\nlatest:\n%s\narchived:\n%s", packetData, archivedPacketData)
	}
	return packetData
}

func decodeDryRunProgramContractPacket(t *testing.T, packetData []byte) dryRunProgramContractPacket {
	t.Helper()

	var packet dryRunProgramContractPacket
	if err := json.Unmarshal(packetData, &packet); err != nil {
		t.Fatalf("unmarshal execution packet: %v", err)
	}
	return packet
}

func assertDryRunProgramContractPacket(
	t *testing.T,
	packet dryRunProgramContractPacket,
	tc dryRunProgramContractCase,
) {
	t.Helper()

	if packet.AutodevProgram.Path != tc.wantPath {
		t.Fatalf("packet autodev_program.path = %q, want %q", packet.AutodevProgram.Path, tc.wantPath)
	}
	if len(packet.AutodevProgram.ValidationCommands) == 0 {
		t.Fatalf("packet validation_commands empty: %+v", packet.AutodevProgram)
	}
	if packet.AutodevProgram.ValidationCommands[0] != tc.wantValidationCommand {
		t.Fatalf("packet validation command = %q, want %q", packet.AutodevProgram.ValidationCommands[0], tc.wantValidationCommand)
	}
	if !containsProgramContract(packet.ContractSurfaces, "docs/contracts/repo-execution-profile.md") {
		t.Fatalf("contract_surfaces = %#v, want repo execution profile", packet.ContractSurfaces)
	}
	if !containsProgramContract(packet.ContractSurfaces, tc.wantPath) {
		t.Fatalf("contract_surfaces = %#v, want %s", packet.ContractSurfaces, tc.wantPath)
	}
}

func assertDryRunOrchestrationLog(t *testing.T, tmpDir string) {
	t.Helper()

	logPath := filepath.Join(tmpDir, ".agents", "rpi", "phased-orchestration.log")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read orchestration log: %v", err)
	}
	logText := string(logData)
	for _, phaseName := range []string{"discovery", "implementation", "validation"} {
		if !strings.Contains(logText, phaseName) {
			t.Fatalf("expected orchestration log to mention %s, got: %s", phaseName, logText)
		}
	}
}

func containsProgramContract(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestExecutionPacket_MixedProvenanceFields(t *testing.T) {
	tmpDir := t.TempDir()
	state := &phasedState{
		Goal:  "test mixed",
		RunID: "test-run",
		Opts:  phasedEngineOptions{Mixed: true},
	}

	if err := writeExecutionPacketSeed(tmpDir, state); err != nil {
		t.Fatalf("writeExecutionPacketSeed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".agents", "rpi", "execution-packet.json"))
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}

	var packet executionPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !packet.MixedModeRequested {
		t.Error("MixedModeRequested should be true when opts.Mixed is set")
	}
	// MixedModeEffective is not set at seed time — downstream phases populate it.
	if packet.MixedModeEffective {
		t.Error("MixedModeEffective should be false at seed time")
	}
}

func TestExecutionPacket_MixedNotSetByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	state := &phasedState{
		Goal:  "test no mixed",
		RunID: "test-run-2",
		Opts:  phasedEngineOptions{},
	}

	if err := writeExecutionPacketSeed(tmpDir, state); err != nil {
		t.Fatalf("writeExecutionPacketSeed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".agents", "rpi", "execution-packet.json"))
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}

	var packet executionPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if packet.MixedModeRequested {
		t.Error("MixedModeRequested should be false when opts.Mixed is not set")
	}
}
