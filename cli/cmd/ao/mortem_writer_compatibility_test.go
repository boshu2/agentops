package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

type legacyV2WriterFixture struct {
	SchemaVersion int               `json:"schema_version"`
	PacketFields  map[string]string `json:"packet_fields"`
	RuntimePaths  []string          `json:"runtime_paths"`
	RatchetSteps  []string          `json:"ratchet_steps"`
}

func loadLegacyV2WriterFixture(t *testing.T) legacyV2WriterFixture {
	t.Helper()
	root := os.Getenv("MORTEM_COMPAT_FIXTURES_DIR")
	if root == "" {
		root = filepath.Join("..", "..", "..", "tests", "fixtures", "mortem-compatibility")
	}
	data, err := os.ReadFile(filepath.Join(root, "writer-legacy-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture legacyV2WriterFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse writer fixture: %v", err)
	}
	return fixture
}

func TestProductionFindingCompiler_PremortemAliasesWriteOneLegacyArtifactThroughS7(t *testing.T) {
	fixture := loadLegacyV2WriterFixture(t)
	if len(fixture.RuntimePaths) != 1 {
		t.Fatalf("writer fixture runtime paths = %v, want the one executable compiler sink", fixture.RuntimePaths)
	}
	out, err := newProductionFindingCompiler().Compile(context.Background(), ports.FindingArtifact{
		ID: "finding-production-mortem",
		Frontmatter: map[string]string{
			"compiler_targets": "premortem,pre-mortem,pre_mortem",
		},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("mortem aliases emitted %d artifacts, want exactly one", len(out))
	}
	wantPath := strings.Replace(fixture.RuntimePaths[0], "current.md", "finding-production-mortem.md", 1)
	if out[0].Path != wantPath {
		t.Fatalf("writer path = %q, want fixture-derived legacy path %q through S7", out[0].Path, wantPath)
	}
}

func TestMortemCompatibilityFixture_LegacyV2WriterMatchesProductionRatchet(t *testing.T) {
	fixture := loadLegacyV2WriterFixture(t)
	steps := ratchet.AllSteps()
	for _, want := range fixture.RatchetSteps {
		found := false
		for _, step := range steps {
			found = found || string(step) == want
		}
		if !found {
			t.Errorf("production ratchet steps %v omit fixture step %q", steps, want)
		}
	}
}

func TestStigmergicScorecard_EmitsCanonicalMortemJSONAndHumanLabels(t *testing.T) {
	scorecard := stigmergicScorecard{PreMortemChecks: 2}
	data, err := json.Marshal(scorecard)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"premortem_checks":2`)) {
		t.Fatalf("scorecard JSON = %s, want premortem_checks", data)
	}
	if bytes.Contains(data, []byte("pre_mortem_checks")) {
		t.Fatalf("scorecard JSON = %s, contains legacy emitted key", data)
	}

	var out bytes.Buffer
	cmd := contextCmd
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })
	printPacketHuman(cmd, StigmergicPacket{Scorecard: scorecard})
	if !strings.Contains(out.String(), "Premortem checks:") {
		t.Fatalf("human packet = %q, want canonical Premortem checks label", out.String())
	}
	if strings.Contains(out.String(), "Pre-mortem checks:") {
		t.Fatalf("human packet = %q, contains legacy label", out.String())
	}
}

func TestStatusFlywheel_EmitsCanonicalPremortemChecksJSONKey(t *testing.T) {
	data, err := json.Marshal(flywheelBrief{PreMortemChecks: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"premortem_checks":3`)) {
		t.Fatalf("status JSON = %s, want premortem_checks", data)
	}
	if bytes.Contains(data, []byte("pre_mortem_checks")) {
		t.Fatalf("status JSON = %s, contains legacy emitted key", data)
	}
}

func TestMetricsFlywheelHuman_EmitsCanonicalPremortemChecksLabel(t *testing.T) {
	metrics := &types.FlywheelMetrics{
		Timestamp:   time.Now(),
		PeriodStart: time.Now().Add(-24 * time.Hour),
		PeriodEnd:   time.Now(),
		TierCounts:  map[string]int{},
		StigmergicScorecard: &types.StigmergicScorecard{
			PromotedFindings: 1,
			PlanningRules:    1,
			PreMortemChecks:  2,
		},
	}
	var out bytes.Buffer
	printFlywheelStatus(&out, metrics)
	if !strings.Contains(out.String(), "Premortem checks: 2") {
		t.Fatalf("flywheel human output = %q, want canonical Premortem checks label", out.String())
	}
	if strings.Contains(out.String(), "pre-mortem checks") || strings.Contains(out.String(), "Pre-mortem checks") {
		t.Fatalf("flywheel human output = %q, contains legacy mortem label", out.String())
	}
}
