// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRPIDiscoveryArtifact_ParseFixtureRoundtrips is the L1 regression test for
// the markdown + frontmatter parser. It exercises a fixture shaped like the
// /council --evidence --commit-ready output documented in
// skills/rpi/references/discovery-artifact-mode.md.
func TestRPIDiscoveryArtifact_ParseFixtureRoundtrips(t *testing.T) {
	content := `---
goal: Wire --discovery-artifact flag
loc_estimate: "150"
---

# Wire --discovery-artifact flag

Add a new rpi flag that lets callers skip Phase 1.

## In Scope

- cli/cmd/ao/rpi_phased.go
- cli/cmd/ao/rpi_discovery_artifact.go
- tests for the new flag

## Out of Scope

- skill file edits
- codex.go territory

## Abort Gates

- more than 5 existing tests break
- cyclomatic complexity regressions

## TDD Matrix

- L1 parse fixture roundtrips
- L2 entry point produces execution packet

## Risks

- YAML parsing drift if the artifact shape changes
`
	art := parseDiscoveryArtifact(content)
	if got, want := art.Goal, "Wire --discovery-artifact flag"; got != want {
		t.Errorf("goal = %q, want %q", got, want)
	}
	if got, want := art.LocEstimate, "150"; got != want {
		t.Errorf("loc_estimate = %q, want %q", got, want)
	}
	wantIn := []string{
		"cli/cmd/ao/rpi_phased.go",
		"cli/cmd/ao/rpi_discovery_artifact.go",
		"tests for the new flag",
	}
	if got := art.InScope; !stringSlicesEqual(got, wantIn) {
		t.Errorf("in_scope = %v, want %v", got, wantIn)
	}
	wantOut := []string{"skill file edits", "codex.go territory"}
	if got := art.OutOfScope; !stringSlicesEqual(got, wantOut) {
		t.Errorf("out_of_scope = %v, want %v", got, wantOut)
	}
	wantAbort := []string{"more than 5 existing tests break", "cyclomatic complexity regressions"}
	if got := art.AbortGates; !stringSlicesEqual(got, wantAbort) {
		t.Errorf("abort_gates = %v, want %v", got, wantAbort)
	}
	wantTDD := []string{"L1 parse fixture roundtrips", "L2 entry point produces execution packet"}
	if got := art.TDDMatrix; !stringSlicesEqual(got, wantTDD) {
		t.Errorf("tdd_matrix = %v, want %v", got, wantTDD)
	}
	wantRisks := []string{"YAML parsing drift if the artifact shape changes"}
	if got := art.Risks; !stringSlicesEqual(got, wantRisks) {
		t.Errorf("risks = %v, want %v", got, wantRisks)
	}
}

// TestRPIDiscoveryArtifact_LoadMissingFile asserts the "artifact not found"
// error path returns a readable message instead of panicking.
func TestRPIDiscoveryArtifact_LoadMissingFile(t *testing.T) {
	tmp := t.TempDir()
	_, err := loadDiscoveryArtifact(filepath.Join(tmp, "nope.md"))
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want substring 'not found'", err.Error())
	}
}

// TestRPIDiscoveryArtifact_LoadEmptyPath asserts the empty-path error is
// surfaced (the flag default is empty, so callers must guard).
func TestRPIDiscoveryArtifact_LoadEmptyPath(t *testing.T) {
	if _, err := loadDiscoveryArtifact("   "); err == nil {
		t.Fatalf("expected error for empty path, got nil")
	}
}

func TestRPIDiscoveryArtifact_PreloadUsesArtifactGoalFallback(t *testing.T) {
	tmp := t.TempDir()
	artPath := filepath.Join(tmp, "art.md")
	if err := os.WriteFile(artPath, []byte("# goal from artifact\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	art, args, err := preloadDiscoveryArtifact(artPath, nil)
	if err != nil {
		t.Fatalf("preloadDiscoveryArtifact: %v", err)
	}
	if art == nil {
		t.Fatalf("preloadDiscoveryArtifact returned nil artifact")
	}
	if !stringSlicesEqual(args, []string{"goal from artifact"}) {
		t.Fatalf("args = %v, want artifact goal fallback", args)
	}

	_, args, err = preloadDiscoveryArtifact(artPath, []string{"explicit goal"})
	if err != nil {
		t.Fatalf("preloadDiscoveryArtifact with explicit goal: %v", err)
	}
	if !stringSlicesEqual(args, []string{"explicit goal"}) {
		t.Fatalf("args = %v, want explicit goal preserved", args)
	}
}

func TestRPIDiscoveryArtifact_ApplyIgnoresDiscoveryStart(t *testing.T) {
	tmp := t.TempDir()
	art := &discoveryArtifact{Goal: "goal from artifact", SourcePath: filepath.Join(tmp, "art.md")}

	if err := applyDiscoveryArtifactToPacket(tmp, art, 1, art.Goal); err != nil {
		t.Fatalf("applyDiscoveryArtifactToPacket: %v", err)
	}
	packetPath := filepath.Join(tmp, ".agents", "rpi", "execution-packet.json")
	if _, err := os.Stat(packetPath); !os.IsNotExist(err) {
		t.Fatalf("execution packet stat err = %v, want not exist", err)
	}
}

// TestRPIDiscoveryArtifact_WritesExecutionPacket is the L2 behavioral test.
// It produces a fixture artifact on disk, calls the write helper at the
// same integration point the phased runner uses, and asserts the resulting
// JSON has all the expected canonical + extension fields.
func TestRPIDiscoveryArtifact_WritesExecutionPacket(t *testing.T) {
	tmp := t.TempDir()
	artPath := filepath.Join(tmp, "council-report.md")
	fixture := `# Skip Phase 1 with a council report

## In Scope

- cli/cmd/ao/rpi_phased.go

## Abort Gates

- any existing TestRPI test fails

## TDD Matrix

- new flag appears in ` + "`--help`" + `
- execution packet contains goal, scope, abort_gates
`
	if err := os.WriteFile(artPath, []byte(fixture), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	art, err := loadDiscoveryArtifact(artPath)
	if err != nil {
		t.Fatalf("loadDiscoveryArtifact: %v", err)
	}

	// Use tmp as cwd so the packet lands in tmp/.agents/rpi/.
	packetPath, err := writeExecutionPacketFromArtifact(tmp, art, "")
	if err != nil {
		t.Fatalf("writeExecutionPacketFromArtifact: %v", err)
	}
	// Assert the canonical location is produced.
	if got, want := packetPath, filepath.Join(tmp, ".agents", "rpi", "execution-packet.json"); got != want {
		t.Errorf("packetPath = %q, want %q", got, want)
	}

	buf, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	var packet map[string]any
	if err := json.Unmarshal(buf, &packet); err != nil {
		t.Fatalf("unmarshal packet: %v\nraw: %s", err, buf)
	}

	if got, want := packet["objective"], "Skip Phase 1 with a council report"; got != want {
		t.Errorf("objective = %v, want %v", got, want)
	}
	if got, want := packet["phase"], "implementation"; got != want {
		t.Errorf("phase = %v, want %v", got, want)
	}
	if got, want := packet["source"], "discovery-artifact"; got != want {
		t.Errorf("source = %v, want %v", got, want)
	}
	if got, want := packet["tracker_mode"], "discovery-artifact"; got != want {
		t.Errorf("tracker_mode = %v, want %v", got, want)
	}

	// discovery_artifacts must list the absolute artifact path.
	absArt, _ := filepath.Abs(artPath)
	if da, ok := packet["discovery_artifacts"].([]any); !ok || len(da) != 1 || da[0] != absArt {
		t.Errorf("discovery_artifacts = %v, want [%q]", packet["discovery_artifacts"], absArt)
	}

	// done_criteria must include both the tdd matrix and the abort gates.
	dc, ok := packet["done_criteria"].([]any)
	if !ok {
		t.Fatalf("done_criteria is not a slice: %v", packet["done_criteria"])
	}
	dcStrs := anySliceToStrings(dc)
	wantDC := []string{
		"new flag appears in `--help`",
		"execution packet contains goal, scope, abort_gates",
		"any existing TestRPI test fails",
	}
	if !stringSlicesEqual(dcStrs, wantDC) {
		t.Errorf("done_criteria = %v, want %v", dcStrs, wantDC)
	}

	// scope block must be present with the single in_scope entry.
	scope, ok := packet["scope"].(map[string]any)
	if !ok {
		t.Fatalf("scope missing or wrong type: %v", packet["scope"])
	}
	inScope := anySliceToStrings(scope["in_scope"].([]any))
	if !stringSlicesEqual(inScope, []string{"cli/cmd/ao/rpi_phased.go"}) {
		t.Errorf("scope.in_scope = %v, want [cli/cmd/ao/rpi_phased.go]", inScope)
	}

	// abort_gates top-level mirror.
	abort := anySliceToStrings(packet["abort_gates"].([]any))
	if !stringSlicesEqual(abort, []string{"any existing TestRPI test fails"}) {
		t.Errorf("abort_gates = %v, want [any existing TestRPI test fails]", abort)
	}
}

// TestRPIDiscoveryArtifact_GoalOverrideWins asserts that an explicit
// goalOverride (e.g. a goal argument on the command line) takes precedence
// over the goal extracted from the artifact.
func TestRPIDiscoveryArtifact_GoalOverrideWins(t *testing.T) {
	tmp := t.TempDir()
	artPath := filepath.Join(tmp, "art.md")
	if err := os.WriteFile(artPath, []byte("# goal from artifact\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	art, err := loadDiscoveryArtifact(artPath)
	if err != nil {
		t.Fatalf("loadDiscoveryArtifact: %v", err)
	}

	packetPath, err := writeExecutionPacketFromArtifact(tmp, art, "goal from CLI")
	if err != nil {
		t.Fatalf("writeExecutionPacketFromArtifact: %v", err)
	}
	buf, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	var packet map[string]any
	if err := json.Unmarshal(buf, &packet); err != nil {
		t.Fatalf("unmarshal packet: %v", err)
	}
	if got, want := packet["objective"], "goal from CLI"; got != want {
		t.Errorf("objective = %v, want %v", got, want)
	}
}

// TestRPIDiscoveryArtifact_ParseEmptyContent asserts the minimal degraded
// behavior documented in the spec: empty content produces an empty artifact
// (no goal, no scope) without panicking. Callers are expected to downgrade.
func TestRPIDiscoveryArtifact_ParseEmptyContent(t *testing.T) {
	art := parseDiscoveryArtifact("")
	if art == nil {
		t.Fatalf("parseDiscoveryArtifact returned nil")
	}
	if art.Goal != "" {
		t.Errorf("goal = %q, want empty", art.Goal)
	}
	if len(art.InScope) != 0 {
		t.Errorf("in_scope = %v, want empty", art.InScope)
	}
}

// stringSlicesEqual compares two string slices element-by-element.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// anySliceToStrings coerces a []any (the shape produced by json.Unmarshal into
// map[string]any) into []string, skipping non-string entries.
func anySliceToStrings(in []any) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
