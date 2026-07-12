package eval

import (
	"strings"
	"testing"
)

func TestParseJudgeJSONAndCodexTokens(t *testing.T) {
	verdict, err := parseJudgeJSON(`prefix {"vectors":[{"dimension":"task-success","pass":true,"score":0.9}],"aggregate_score":0.9} suffix`)
	if err != nil || verdict.AggregateScore != .9 {
		t.Fatalf("verdict=%#v err=%v", verdict, err)
	}
	if tokens := parseCodexTokens("tokens used: 1,234"); tokens != 1234 {
		t.Fatalf("tokens=%d", tokens)
	}
}

func TestScenarioRunnerSourceUsesCodexExecAndNeverClaudePrintMode(t *testing.T) {
	args := codexExecArgs("message.txt", "schema.json", "prompt")
	joined := strings.Join(args, " ")
	if args[0] != "exec" || !strings.Contains(joined, "--output-last-message message.txt") || strings.Contains(joined, "claude") {
		t.Fatalf("codex args=%q", joined)
	}
}
