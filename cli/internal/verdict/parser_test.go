package verdict

import (
	"strings"
	"testing"
)

func TestParserFailsClosedOnOversizedLine(t *testing.T) {
	body := "author: worker\njudge: judge-a\njudge_program: codex\njudge_model_family: gpt\ncontext_id: ctx-a\nVERDICT: PASS\nCOMMANDS RUN:\n  go test ./...\n" + strings.Repeat("x", LineCap+1)
	if HasCommandsRun(body) {
		t.Fatal("oversized artifact retained a trusted command block")
	}
	if pass, fail := TokenCounts(body); pass != 0 || fail != 0 {
		t.Fatalf("oversized token counts = %d/%d, want 0/0", pass, fail)
	}
}

func TestIdentityRejectsSelfJudgeContext(t *testing.T) {
	body := "author: worker\njudge: judge-a\njudge_program: codex\njudge_model_family: gpt\ncontext_id: ctx-a\nauthor_context_id: ctx-a\n"
	_, gaps := Identity(body)
	if len(gaps) != 1 || gaps[0] != "judge.context_id equals author context" {
		t.Fatalf("gaps = %v", gaps)
	}
}
