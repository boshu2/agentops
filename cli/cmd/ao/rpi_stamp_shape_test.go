// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/orchestration"
)

func TestParseAMActiveCount(t *testing.T) {
	// Real shape from `am robot agents --active --json`.
	out := []byte(`{"_meta":{"command":"robot agents"},"count":3,"agents":[]}`)
	if got := parseAMActiveCount(out); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}
	if got := parseAMActiveCount([]byte("not json")); got != 0 {
		t.Fatalf("malformed count = %d, want 0 floor", got)
	}
	if got := parseAMActiveCount([]byte(`{"count":-5}`)); got != 0 {
		t.Fatalf("negative count = %d, want 0 floor", got)
	}
}

func TestParseAMReservationWriteSets_GroupsByAgent(t *testing.T) {
	// Two lanes, one shared path (BlueLake + GreenCastle both hold dag.md).
	out := []byte(`{"all_active":[
		{"agent":"BlueLake","path":"skills/discovery/references/dag.md","exclusive":true},
		{"agent":"BlueLake","path":"cli/internal/orchestration/shape.go","exclusive":true},
		{"agent":"GreenCastle","path":"skills/discovery/references/dag.md","exclusive":true},
		{"agent":"","path":"ignored.go"},
		{"agent":"RedIsland","path":""}
	]}`)
	sets := parseAMReservationWriteSets(out)
	if len(sets) != 2 {
		t.Fatalf("lanes = %d, want 2 (empty agent/path skipped): %v", len(sets), sets)
	}
	// Sorted by agent: BlueLake first, GreenCastle second.
	if len(sets[0]) != 2 || sets[1][0] != "skills/discovery/references/dag.md" {
		t.Fatalf("unexpected grouping: %v", sets)
	}
	// The grouped sets must actually drive a contention verdict.
	v := orchestration.ValidateShape(orchestration.ShapeInputs{LiveWriters: 2, WriteSets: sets})
	if v.Shape != orchestration.ShapeAMOnly {
		t.Fatalf("overlapping lanes → shape = %s, want am-only", v.Shape)
	}
}

func TestParseAMReservationWriteSets_Empty(t *testing.T) {
	if got := parseAMReservationWriteSets([]byte(`{"all_active":[]}`)); got != nil {
		t.Fatalf("empty reservations = %v, want nil", got)
	}
	if got := parseAMReservationWriteSets([]byte("garbage")); got != nil {
		t.Fatalf("malformed reservations = %v, want nil", got)
	}
}

// writePacket writes a minimal hand-compiled packet like the live /discovery
// path produces (no orchestration_decision yet), plus a sentinel field that
// must survive the stamp.
func writeLivePacket(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "execution-packet.json")
	body := map[string]any{
		"schema_version":    1,
		"objective":         "wire the live discovery shape",
		"epic_id":           "age-gud",
		"contract_surfaces": []string{"skills/discovery/references/dag.md"},
		"tracker_mode":      "br",
	}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStampShapeOnPacket_RoundTrip_PreservesFields(t *testing.T) {
	dir := t.TempDir()
	path := writeLivePacket(t, dir)

	verdict := orchestration.ShapeVerdict{
		Shape:           orchestration.ShapeATMOnly,
		PredicatesFired: []string{"durability:unattended"},
		Rationale:       "unattended fired",
	}
	if err := stampShapeOnPacket(path, verdict, "2026-06-16T00:00:00Z"); err != nil {
		t.Fatalf("stamp: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	// Sentinel fields preserved.
	if got["objective"] != "wire the live discovery shape" || got["epic_id"] != "age-gud" {
		t.Fatalf("stamp dropped existing fields: %v", got)
	}
	dec, ok := got["orchestration_decision"].(map[string]any)
	if !ok {
		t.Fatalf("orchestration_decision missing or wrong type: %v", got["orchestration_decision"])
	}
	if dec["chosen_shape"] != "atm-only" {
		t.Fatalf("chosen_shape = %v, want atm-only", dec["chosen_shape"])
	}
	fired, ok := dec["predicates_fired"].([]any)
	if !ok || len(fired) != 1 || fired[0] != "durability:unattended" {
		t.Fatalf("predicates_fired = %v, want [durability:unattended]", dec["predicates_fired"])
	}
	if dec["ts"] != "2026-06-16T00:00:00Z" {
		t.Fatalf("ts = %v", dec["ts"])
	}
}

func TestStampShapeOnPacket_PredicatesFiredAlwaysPresent(t *testing.T) {
	dir := t.TempDir()
	path := writeLivePacket(t, dir)
	// Single-agent verdict: no predicates fired, but the field must still be
	// present as an empty array (honest record).
	verdict := orchestration.ShapeVerdict{Shape: orchestration.ShapeSingleAgent, Rationale: "default"}
	if err := stampShapeOnPacket(path, verdict, "2026-06-16T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	dec := got["orchestration_decision"].(map[string]any)
	fired, ok := dec["predicates_fired"].([]any)
	if !ok || len(fired) != 0 {
		t.Fatalf("predicates_fired = %v, want present empty array", dec["predicates_fired"])
	}
}

// writeRunAlias writes an alias packet under <root>/.agents/rpi/ carrying run_id,
// plus the matching per-run archive snapshot under
// <root>/.agents/rpi/runs/<run-id>/ (the real on-disk shape /discovery STEP 6
// produces). Returns the alias and archive paths.
func writeRunAlias(t *testing.T, root, runID string) (string, string) {
	t.Helper()
	stateDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(filepath.Join(stateDir, "runs", runID), 0o750); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"schema_version":    1,
		"objective":         "wire the live discovery shape",
		"run_id":            runID,
		"epic_id":           "age-gud",
		"contract_surfaces": []string{"skills/discovery/references/dag.md"},
		"tracker_mode":      "br",
	}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	aliasPath := filepath.Join(stateDir, executionPacketFile)
	archivePath := filepath.Join(stateDir, "runs", runID, executionPacketFile)
	for _, p := range []string{aliasPath, archivePath} {
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return aliasPath, archivePath
}

func stampedShape(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("re-parse %s: %v", path, err)
	}
	dec, ok := got["orchestration_decision"].(map[string]any)
	if !ok {
		return ""
	}
	shape, _ := dec["chosen_shape"].(string)
	return shape
}

func TestRunArchivePacketPath(t *testing.T) {
	root := "/repo"
	alias := filepath.Join(root, ".agents", "rpi", executionPacketFile)
	got := runArchivePacketPath(alias, "run-123")
	want := filepath.Join(root, ".agents", "rpi", "runs", "run-123", executionPacketFile)
	if got != want {
		t.Fatalf("archive path = %q, want %q", got, want)
	}
	if got := runArchivePacketPath(alias, ""); got != "" {
		t.Fatalf("empty run id should yield empty path, got %q", got)
	}
	if got := runArchivePacketPath(alias, "  "); got != "" {
		t.Fatalf("whitespace run id should yield empty path, got %q", got)
	}
}

func TestResolveStampRunID_EnvOverridesPacket(t *testing.T) {
	dir := t.TempDir()
	alias, _ := writeRunAlias(t, dir, "packet-run")
	t.Setenv("RPI_RUN_ID", "env-run")
	if got := resolveStampRunID(alias); got != "env-run" {
		t.Fatalf("run id = %q, want env-run (env wins over packet)", got)
	}
}

func TestResolveStampRunID_FallsBackToPacket(t *testing.T) {
	dir := t.TempDir()
	alias, _ := writeRunAlias(t, dir, "packet-run")
	t.Setenv("RPI_RUN_ID", "")
	if got := resolveStampRunID(alias); got != "packet-run" {
		t.Fatalf("run id = %q, want packet-run (packet fallback)", got)
	}
	// Missing packet → empty, no panic.
	if got := resolveStampRunID(filepath.Join(dir, "nope.json")); got != "" {
		t.Fatalf("missing packet run id = %q, want empty", got)
	}
}

func TestStampShapeEverywhere_StampsAliasAndArchive(t *testing.T) {
	dir := t.TempDir()
	alias, archive := writeRunAlias(t, dir, "run-abc")
	t.Setenv("RPI_RUN_ID", "") // force packet-based resolution
	verdict := orchestration.ShapeVerdict{Shape: orchestration.ShapeATMOnly, Rationale: "unattended"}

	stamped, err := stampShapeEverywhere(alias, verdict, "2026-06-16T00:00:00Z")
	if err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if len(stamped) != 2 {
		t.Fatalf("stamped %d paths, want 2 (alias + run archive): %v", len(stamped), stamped)
	}
	if got := stampedShape(t, alias); got != "atm-only" {
		t.Fatalf("alias chosen_shape = %q, want atm-only", got)
	}
	if got := stampedShape(t, archive); got != "atm-only" {
		t.Fatalf("run archive chosen_shape = %q, want atm-only — the durable snapshot must carry the decision", got)
	}
}

func TestStampShapeEverywhere_NoArchiveStampsAliasOnly(t *testing.T) {
	// Alias exists with run_id, but no run-archive snapshot on disk → stamp the
	// alias only, no error (stamp-shape mirrors existing archives, never creates).
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(stateDir, executionPacketFile)
	body, _ := json.MarshalIndent(map[string]any{"schema_version": 1, "run_id": "ghost-run", "objective": "x"}, "", "  ")
	if err := os.WriteFile(alias, body, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RPI_RUN_ID", "")
	stamped, err := stampShapeEverywhere(alias, orchestration.ShapeVerdict{Shape: orchestration.ShapeSingleAgent}, "2026-06-16T00:00:00Z")
	if err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if len(stamped) != 1 {
		t.Fatalf("stamped %d paths, want 1 (alias only — no archive exists): %v", len(stamped), stamped)
	}
	if got := stampedShape(t, alias); got != "single-agent" {
		t.Fatalf("alias chosen_shape = %q, want single-agent", got)
	}
}

// fakeAM returns canned am stdout per (robot, <subcommand>, ...).
func fakeAM(active, reservations string) amRunner {
	return func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[1] == "agents" {
			return []byte(active), nil
		}
		if len(args) >= 2 && args[1] == "reservations" {
			return []byte(reservations), nil
		}
		return []byte("{}"), nil
	}
}

func TestGatherShapeInputs_LiveContentionOverridesProposal(t *testing.T) {
	// The live path's worst case: model proposes single-agent ("disjoint lanes")
	// but ground truth shows 2 writers on the same path → override to am-only.
	active := `{"count":2}`
	reservations := `{"all_active":[
		{"agent":"A","path":"skills/discovery/references/dag.md"},
		{"agent":"B","path":"skills/discovery/references/dag.md"}
	]}`
	in := gatherShapeInputs("/repo", "single-agent", false, fakeAM(active, reservations))
	v := orchestration.ValidateShape(in)
	if v.Shape != orchestration.ShapeAMOnly {
		t.Fatalf("shape = %s, want am-only (contention override)", v.Shape)
	}
	if !v.Overridden {
		t.Fatalf("expected proposal overridden")
	}
	foundContention := false
	for _, p := range v.PredicatesFired {
		if p == "contention:live-writers-overlap" {
			foundContention = true
		}
	}
	if !foundContention {
		t.Fatalf("predicates_fired missing contention: %v", v.PredicatesFired)
	}
}

func TestGatherShapeInputs_NilRunnerIsSingleAgentFloor(t *testing.T) {
	in := gatherShapeInputs("/repo", "", false, nil)
	if in.LiveWriters != 0 || in.WriteSets != nil {
		t.Fatalf("nil runner should yield empty inputs: %+v", in)
	}
	if v := orchestration.ValidateShape(in); v.Shape != orchestration.ShapeSingleAgent {
		t.Fatalf("nil-runner shape = %s, want single-agent", v.Shape)
	}
}

func TestGatherShapeInputs_UnattendedDrivesATM(t *testing.T) {
	in := gatherShapeInputs("/repo", "", true, fakeAM(`{"count":0}`, `{"all_active":[]}`))
	if v := orchestration.ValidateShape(in); v.Shape != orchestration.ShapeATMOnly {
		t.Fatalf("unattended shape = %s, want atm-only", v.Shape)
	}
}
