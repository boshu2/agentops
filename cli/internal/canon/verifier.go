// practices: [mechanical-verify-before-judgment, hexagonal-architecture]

package canon

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Claim is a learning submitted for independent verification.
type Claim struct {
	EntryID string
	Path    string
	Content string
}

// CouncilVerifier produces an independent verdict on a claim. Implementations
// MUST judge from a different vantage than the author — a cross-vendor model is
// the canonical case (council's cross-vendor DISAGREE rule). That independence
// is the whole point: a verdict the author could have produced proves nothing.
type CouncilVerifier interface {
	// Judge returns the verdict, the judge's identity (the vendor/model that
	// ruled), and the evidence it produced — recorded as the verification
	// receipt so the ruling is auditable, not just asserted.
	Judge(Claim) (verdict Verdict, judge Identity, evidence string, err error)
}

// CommandVerifier runs an external judge command — the cross-vendor lane. The
// command receives the judge prompt on stdin and must print a line matching
// `VERDICT: confirmed` or `VERDICT: refuted`; its full combined output becomes
// the receipt. Configurable (codex in production, a fixture in tests) so the
// vendor is swappable without touching canon and the path is testable offline.
//
// Honest scope: this wires the cross-vendor *judgment* lane (council P9). How
// deeply the judge gathers its own evidence is bounded by what Command does —
// a codex judge has shell access and can read the cited files; a thin judge
// only sees the claim text. The receipt records which it was.
type CommandVerifier struct {
	// Command is a shell command (run via `sh -c`) that receives the claim
	// prompt on stdin. Empty Command makes Judge fail loudly rather than
	// silently fabricating a verdict.
	Command string
	// JudgeID attributes the verdict to the cross-vendor judge (e.g. the model
	// name). It must denote a vantage distinct from any learning author.
	JudgeID Identity
}

var verdictPattern = regexp.MustCompile(`(?i)VERDICT:\s*(confirmed|refuted)`)

// Judge runs the configured command over the claim and parses its verdict.
func (cv CommandVerifier) Judge(c Claim) (Verdict, Identity, string, error) {
	if strings.TrimSpace(cv.Command) == "" {
		return "", cv.JudgeID, "", fmt.Errorf("no council verifier command configured (set AGENTOPS_CANON_VERIFIER_CMD, e.g. \"codex exec\")")
	}
	cmd := exec.Command("sh", "-c", cv.Command)
	cmd.Stdin = strings.NewReader(BuildJudgePrompt(c))
	out, runErr := cmd.CombinedOutput()
	evidence := strings.TrimSpace(string(out))
	if runErr != nil {
		return "", cv.JudgeID, evidence, fmt.Errorf("council verifier command failed: %w", runErr)
	}
	verdict, parseErr := ParseVerdict(evidence)
	if parseErr != nil {
		return "", cv.JudgeID, evidence, parseErr
	}
	return verdict, cv.JudgeID, evidence, nil
}

// BuildJudgePrompt instructs an independent judge to rule on a learning. It
// tells the judge to gather its own evidence rather than trust the claim's
// self-assertion — the anti-self-certification rule applied to the judge.
func BuildJudgePrompt(c Claim) string {
	var b strings.Builder
	b.WriteString("You are an INDEPENDENT verifier judging whether a team learning holds.\n")
	b.WriteString("Do NOT trust the learning's own assertion. Gather your own evidence — read the\n")
	b.WriteString("cited files, run the cited commands where you can — then rule.\n\n")
	if c.Path != "" {
		b.WriteString("Learning file: " + c.Path + "\n")
	}
	b.WriteString("Learning id: " + c.EntryID + "\n\n")
	b.WriteString("--- learning content ---\n")
	b.WriteString(c.Content)
	b.WriteString("\n--- end ---\n\n")
	b.WriteString("Cite the file:line or command output you actually examined, then end with EXACTLY one line:\n")
	b.WriteString("VERDICT: confirmed   (if your own evidence supports the learning)\n")
	b.WriteString("VERDICT: refuted     (if it does not, or you could not corroborate it)\n")
	return b.String()
}

// ParseVerdict extracts the VERDICT line from judge output. Absence of a clear
// verdict is an error — never default to confirmed.
func ParseVerdict(output string) (Verdict, error) {
	m := verdictPattern.FindStringSubmatch(output)
	if m == nil {
		return "", fmt.Errorf("no VERDICT line in judge output")
	}
	switch strings.ToLower(m[1]) {
	case "confirmed":
		return VerdictConfirmed, nil
	case "refuted":
		return VerdictRefuted, nil
	default:
		return "", fmt.Errorf("unrecognized verdict %q", m[1])
	}
}
