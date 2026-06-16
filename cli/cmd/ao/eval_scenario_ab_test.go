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

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/scenario"
)

type fakeCmdRunner struct {
	with, without aoeval.ArmOutcome
}

func (f fakeCmdRunner) RunArm(_ context.Context, _ scenario.Scenario, withGold bool) (aoeval.ArmOutcome, error) {
	if withGold {
		return f.with, nil
	}
	return f.without, nil
}

type fakeCmdJudge struct {
	with, without aoeval.JudgeVerdict
}

func (f fakeCmdJudge) Judge(_ context.Context, _ scenario.Scenario, arm aoeval.ScenarioArm, _ aoeval.ArmOutcome) (aoeval.JudgeVerdict, error) {
	if arm == aoeval.ArmWithGold {
		return f.with, nil
	}
	return f.without, nil
}

func writeCmdFixture(t *testing.T, threshold float64) string {
	t.Helper()
	dir := t.TempDir()
	res, err := scenario.Create(scenario.CreateOptions{
		Goal:      "design a reward mechanism that gates on a verified outcome",
		Threshold: threshold,
		Status:    "active",
		Source:    "human",
		Dir:       dir,
		Now:       func() time.Time { return time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	return res.Path
}

// withFakes overrides the injectable factory seam for the duration of a test.
func withFakes(t *testing.T, runner aoeval.ScenarioRunner, judge aoeval.ScenarioJudge) {
	t.Helper()
	origR, origJ := scenarioABRunnerFactory, scenarioABJudgeFactory
	scenarioABRunnerFactory = func() aoeval.ScenarioRunner { return runner }
	scenarioABJudgeFactory = func() aoeval.ScenarioJudge { return judge }
	t.Cleanup(func() {
		scenarioABRunnerFactory = origR
		scenarioABJudgeFactory = origJ
		evalScenarioABScenario = ""
		evalScenarioABOutput = ""
		evalScenarioABBudget = 0
	})
}

func runScenarioABCmd(t *testing.T, scenarioPath, outPath string, budget int) (string, error) {
	t.Helper()
	cmd := evalScenarioABCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetContext(context.Background())
	evalScenarioABScenario = scenarioPath
	evalScenarioABOutput = outPath
	evalScenarioABBudget = budget
	err := cmd.RunE(cmd, nil)
	return buf.String(), err
}

func TestEvalScenarioABCmd_GatePassWritesScorecard(t *testing.T) {
	withFakes(t,
		fakeCmdRunner{
			with:    aoeval.ArmOutcome{Output: "treatment", TokenCost: 100},
			without: aoeval.ArmOutcome{Output: "control", TokenCost: 100},
		},
		fakeCmdJudge{
			with:    aoeval.JudgeVerdict{AggregateScore: 0.9},
			without: aoeval.JudgeVerdict{AggregateScore: 0.4},
		},
	)
	out := filepath.Join(t.TempDir(), "card.json")
	stdout, err := runScenarioABCmd(t, writeCmdFixture(t, 0.8), out, 200000)
	if err != nil {
		t.Fatalf("expected pass, got error: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "gate=PASS") {
		t.Errorf("stdout missing gate=PASS: %q", stdout)
	}
	if !strings.Contains(stdout, "delta=0.5000") {
		t.Errorf("stdout missing expected delta: %q", stdout)
	}
	// Scorecard must be persisted and decode back into the typed struct.
	data, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("read scorecard: %v", readErr)
	}
	var card aoeval.ScenarioDeltaScorecard
	if jsonErr := json.Unmarshal(data, &card); jsonErr != nil {
		t.Fatalf("decode scorecard: %v", jsonErr)
	}
	if !card.Gate.Pass || card.AggregateDelta != 0.5 {
		t.Errorf("scorecard = pass:%v delta:%.4f, want pass:true delta:0.5", card.Gate.Pass, card.AggregateDelta)
	}
}

func TestEvalScenarioABCmd_GateFailReturnsError(t *testing.T) {
	withFakes(t,
		fakeCmdRunner{
			with:    aoeval.ArmOutcome{TokenCost: 100},
			without: aoeval.ArmOutcome{TokenCost: 100},
		},
		// Equal scores -> delta 0 -> fail-loud.
		fakeCmdJudge{
			with:    aoeval.JudgeVerdict{AggregateScore: 0.5},
			without: aoeval.JudgeVerdict{AggregateScore: 0.5},
		},
	)
	stdout, err := runScenarioABCmd(t, writeCmdFixture(t, 0.5), "", 200000)
	if err == nil {
		t.Fatalf("expected gate-fail error, got nil\n%s", stdout)
	}
	if !strings.Contains(stdout, "gate=FAIL") {
		t.Errorf("stdout missing gate=FAIL: %q", stdout)
	}
}

func TestEvalScenarioABCmd_MissingScenarioFlag(t *testing.T) {
	withFakes(t, fakeCmdRunner{}, fakeCmdJudge{})
	if _, err := runScenarioABCmd(t, "", "", 0); err == nil {
		t.Error("expected error when --scenario is empty")
	}
}

func TestParseJudgeJSON(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantAgg float64
		wantErr bool
	}{
		{"bare json", `{"vectors":[],"aggregate_score":0.75}`, 0.75, false},
		{"json wrapped in prose", "Here is my verdict:\n{\"aggregate_score\":0.9}\nDone.", 0.9, false},
		{"no json", "no object here", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := parseJudgeJSON(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.AggregateScore != tc.wantAgg {
				t.Errorf("aggregate = %.4f, want %.4f", v.AggregateScore, tc.wantAgg)
			}
		})
	}
}

func TestParseCodexTokens(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"... \ntokens used\n147,396\n", 147396},
		{"tokens used: 1024", 1024},
		{"no token line here, just 12 chars-ish", len("no token line here, just 12 chars-ish") / 4},
	}
	for _, tc := range cases {
		if got := parseCodexTokens(tc.in); got != tc.want {
			t.Errorf("parseCodexTokens(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
