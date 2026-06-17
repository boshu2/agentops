package main

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/extract"
)

// TestJudgeSchema_CodexStrictValid cross-pins the hand-written judgeOutputSchema
// (the scenario-ab judge's codex --output-schema) to the SAME single source of
// truth as the compiled extraction schema: extract.ValidateCodexStrictSchema.
// Both codex-schema paths in the CLI must satisfy one verified contract so they
// cannot drift — if either path stops listing every property in required (the
// age-nzx 400), this test goes red. The judge const is the WORKING reference
// that proves the contract is real; pinning it here makes that explicit.
func TestJudgeSchema_CodexStrictValid(t *testing.T) {
	if err := extract.ValidateCodexStrictSchema([]byte(judgeOutputSchema)); err != nil {
		t.Fatalf("judgeOutputSchema is not codex-strict-valid (would 400 live): %v", err)
	}
}
