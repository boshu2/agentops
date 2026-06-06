package canon

import (
	"strings"
	"testing"
)

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    Verdict
		wantErr bool
	}{
		{"confirmed", "checked cli/x.go:42, holds.\nVERDICT: confirmed", VerdictConfirmed, false},
		{"refuted", "could not reproduce.\nVERDICT: refuted", VerdictRefuted, false},
		{"case insensitive", "verdict: CONFIRMED", VerdictConfirmed, false},
		{"no verdict line", "the learning looks fine to me", "", true},
		{"never defaults to confirmed on absence", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVerdict(tt.output)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseVerdict = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommandVerifier_EmptyCommandFailsLoud(t *testing.T) {
	cv := CommandVerifier{Command: "", JudgeID: Identity{Name: "codex"}}
	_, _, _, err := cv.Judge(Claim{EntryID: "e"})
	if err == nil {
		t.Fatal("empty command must error, not fabricate a verdict")
	}
}

// TestCommandVerifier_Confirmed exercises the real exec+parse path with a
// fixture judge command standing in for the cross-vendor model.
func TestCommandVerifier_Confirmed(t *testing.T) {
	cv := CommandVerifier{
		Command: `printf 'read cli/internal/canon/promote.go:1; holds.\nVERDICT: confirmed\n'`,
		JudgeID: Identity{Name: "codex:gpt-5.3", Email: "codex@vendor"},
	}
	verdict, judge, evidence, err := cv.Judge(Claim{EntryID: "e", Path: "l.md", Content: "TIL x"})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if verdict != VerdictConfirmed {
		t.Errorf("verdict = %q, want confirmed", verdict)
	}
	if !judge.SameAs(Identity{Email: "codex@vendor"}) {
		t.Errorf("judge = %+v, want the codex vendor identity", judge)
	}
	if !strings.Contains(evidence, "promote.go") {
		t.Errorf("evidence should capture the judge transcript, got %q", evidence)
	}
}

func TestCommandVerifier_Refuted(t *testing.T) {
	cv := CommandVerifier{Command: `echo "no such file; VERDICT: refuted"`, JudgeID: Identity{Name: "codex"}}
	verdict, _, _, err := cv.Judge(Claim{EntryID: "e"})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if verdict != VerdictRefuted {
		t.Errorf("verdict = %q, want refuted", verdict)
	}
}

func TestCommandVerifier_NoVerdictIsError(t *testing.T) {
	cv := CommandVerifier{Command: `echo "I am not sure, looks plausible"`, JudgeID: Identity{Name: "codex"}}
	_, _, _, err := cv.Judge(Claim{EntryID: "e"})
	if err == nil {
		t.Fatal("judge output without a VERDICT line must error, not pass")
	}
}

// TestCouncilVerificationIsIndependent proves a council verdict counts toward
// promotion even when the operator running it authored the learning — because
// the verdict is attributed to the cross-vendor judge, not the operator.
func TestCouncilVerificationIsIndependent(t *testing.T) {
	dir := t.TempDir()
	entry := writeLearning(t, dir, "l.md", "Alice", "alice@example.com")
	vl := NewVerificationLedger(joinTmp(dir, "v.jsonl"))

	judge := Identity{Name: "codex:gpt-5.3", Email: "codex@vendor"}
	// Alice authored it, but the verdict is the judge's, recorded under the
	// judge identity → not a self-verification.
	v, err := vl.Record("e", entry, "council", "judge transcript", VerdictConfirmed, judge, clock)
	if err != nil {
		t.Fatal(err)
	}
	if v.Self {
		t.Error("council verdict attributed to the cross-vendor judge must not be self")
	}
	confs, err := vl.IndependentConfirmations("e", false)
	if err != nil {
		t.Fatal(err)
	}
	if confs != 1 {
		t.Errorf("IndependentConfirmations = %d, want 1", confs)
	}
}
