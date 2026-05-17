// Package feedbackcompiler scans a verdict ledger for fail→pass transitions
// and drafts a learning entry in docs/learnings/ for each transition found.
//
// Design: a fail→pass transition for a directive is defined as the FIRST
// iteration record whose verdict is "pass" that is immediately preceded by at
// least one consecutive "fail" record (the preceding failure streak). The draft
// captures the last failure record as the "failure mode" and the first pass
// record as the "recovery". Only one draft per directive per transition window
// is produced.
//
// All output files carry status: draft in their frontmatter; a human must
// promote them per the docs/learnings/README.md contract.
//
// Learning files are written to docs/learnings/ (git-tracked) per ADR-0003 and
// ADR-0005 §2.6.
package feedbackcompiler

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/boshu2/agentops/cli/internal/verdictledger"
)

// LearningsRelDir is the path of the learning drafts dir relative to the
// project root (git-tracked per ADR-0003).
const LearningsRelDir = "docs/learnings"

// draftFilename builds the canonical filename for a learning draft:
// YYYY-MM-DD-auto-<directive>-fail-to-pass.md
func draftFilename(directiveID string, passTime time.Time) string {
	slug := strings.TrimPrefix(directiveID, "d-")
	slug = directiveIDSlugRe.ReplaceAllString(slug, "-")
	return fmt.Sprintf("%s-auto-%s-fail-to-pass.md",
		passTime.UTC().Format("2006-01-02"), slug)
}

// directiveIDSlugRe strips characters that are not safe in filenames.
var directiveIDSlugRe = regexp.MustCompile(`[^a-z0-9-]`)

// Transition holds the pair of ledger records that represent one fail→pass
// event for a directive.
type Transition struct {
	DirectiveID string
	// FailRecord is the last consecutive failure before the pass.
	FailRecord verdictledger.Record
	// PassRecord is the first pass that ended the streak.
	PassRecord verdictledger.Record
}

// scanTransitions returns one Transition per fail→pass transition found in
// ledger. Only the FIRST pass that follows at least one consecutive fail is
// recorded; subsequent passes (no preceding fail streak) are ignored.
func scanTransitions(ledger *verdictledger.Ledger) []Transition {
	seen := map[string]bool{}
	for _, r := range ledger.Records {
		seen[r.DirectiveID] = true
	}

	var out []Transition
	for directiveID := range seen {
		iters := ledger.IterationsFor(directiveID)
		out = append(out, directiveTransitions(directiveID, iters)...)
	}
	return out
}

// directiveTransitions scans the ordered iteration list for one directive and
// returns all fail→pass transitions found.
func directiveTransitions(directiveID string, iters []verdictledger.Record) []Transition {
	var out []Transition
	var lastFail *verdictledger.Record

	for i := range iters {
		r := &iters[i]
		switch r.ScenarioVerdict {
		case verdictledger.VerdictFail:
			lastFail = r
		case verdictledger.VerdictPass:
			if lastFail != nil {
				out = append(out, Transition{
					DirectiveID: directiveID,
					FailRecord:  *lastFail,
					PassRecord:  *r,
				})
				lastFail = nil
			}
		default:
			// skip / unknown: do not reset lastFail (streak is not broken by
			// non-fail non-pass verdicts for the purpose of transition detection)
		}
	}
	return out
}

// draftContent renders the learning draft markdown for a transition.
func draftContent(t Transition, now time.Time) (string, error) {
	data := struct {
		Date        string
		DirectiveID string
		FailRunID   string
		FailTime    string
		FailVerdict string
		FailSat     string
		PassRunID   string
		PassTime    string
		PassVerdict string
		PassSat     string
	}{
		Date:        now.UTC().Format("2006-01-02"),
		DirectiveID: t.DirectiveID,
		FailRunID:   t.FailRecord.RunID,
		FailTime:    t.FailRecord.RunTime,
		FailVerdict: t.FailRecord.ScenarioVerdict,
		FailSat:     formatSat(t.FailRecord.ScenarioSatisfaction),
		PassRunID:   t.PassRecord.RunID,
		PassTime:    t.PassRecord.RunTime,
		PassVerdict: t.PassRecord.ScenarioVerdict,
		PassSat:     formatSat(t.PassRecord.ScenarioSatisfaction),
	}
	var buf bytes.Buffer
	if err := draftTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render learning draft: %w", err)
	}
	return buf.String(), nil
}

// formatSat renders a scenario_satisfaction pointer as a percentage string.
func formatSat(sat *float64) string {
	if sat == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", *sat*100)
}

// draftTmplSrc is the learning file template source; frontmatter follows
// ADR-0005 §2.6 and docs/learnings/README.md "File shape".
// Note: backtick delimiters cannot appear inside a raw string literal, so this
// string is built via concatenation of raw-string segments and interpreted
// string snippets that contribute the literal backtick characters.
var draftTmplSrc = "" +
	"---\n" +
	"title: \"Auto-draft: directive {{.DirectiveID}} recovered from failure (fail->pass)\"\n" +
	"date: {{.Date}}\n" +
	"status: draft\n" +
	"directive_id: {{.DirectiveID}}\n" +
	"tags: [auto-draft, fail-to-pass, feedback-compiler]\n" +
	"source: verdict-ledger fail->pass transition detected by feedbackcompiler\n" +
	"---\n" +
	"\n" +
	"# Auto-draft: directive {{.DirectiveID}} recovered from failure\n" +
	"\n" +
	"This learning was drafted automatically by the feedback compiler (F5.4)." +
	" A human must review and promote it. Do not treat this as authoritative until promoted.\n" +
	"\n" +
	"## What happened\n" +
	"\n" +
	"Directive **{{.DirectiveID}}** transitioned from a failing verdict to a passing verdict.\n" +
	"\n" +
	"- **Last failure:** run {{.FailRunID}} at {{.FailTime}} -- verdict {{.FailVerdict}} (satisfaction {{.FailSat}})\n" +
	"- **First recovery:** run {{.PassRunID}} at {{.PassTime}} -- verdict {{.PassVerdict}} (satisfaction {{.PassSat}})\n" +
	"\n" +
	"## Failure mode\n" +
	"\n" +
	"The scenario(s) for directive {{.DirectiveID}} were failing as of run {{.FailRunID}}." +
	" The satisfaction score was {{.FailSat}}, indicating the scenario acceptance criteria were not met.\n" +
	"\n" +
	"## The fix (inferred)\n" +
	"\n" +
	"Work completed between run {{.FailRunID}} and run {{.PassRunID}} caused the" +
	" directive's scenarios to pass. Review commits, beads, and artifacts in that" +
	" window to identify the specific change.\n" +
	"\n" +
	"## The rule\n" +
	"\n" +
	"> When a directive recovers from a failure streak, capture what changed." +
	" Look at the diff between the last-fail run and the first-pass run --" +
	" that delta is the corrective action.\n" +
	"\n" +
	"## See also\n" +
	"\n" +
	"- Directive {{.DirectiveID}} in GOALS.md\n" +
	"- Verdict ledger: .agents/goals/verdict-ledger.json\n" +
	"- ADR-0005 -- Trace link convention\n" +
	"- ADR-0006 -- Re-steer policy and mutation safety\n"

var draftTmpl = template.Must(template.New("learning").Parse(draftTmplSrc))

// CompileResult holds the outcome of one compiler run.
type CompileResult struct {
	// Drafts is the list of files written (absolute paths).
	Drafts []string
	// Skipped is the number of transitions skipped because a draft for them
	// already existed.
	Skipped int
}

// Compiler scans the verdict ledger for fail→pass transitions and writes draft
// learning files. Now is injectable for tests; if nil, time.Now is used.
type Compiler struct {
	Now func() time.Time
}

// nowUTC returns the compiler's clock in UTC.
func (c *Compiler) nowUTC() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}

// Compile scans the verdict ledger at projectRoot and writes draft learning
// files to the learnings directory. It is idempotent: if a draft for a
// transition already exists it is skipped (not overwritten).
func (c *Compiler) Compile(projectRoot string) (CompileResult, error) {
	ledger, err := verdictledger.Load(projectRoot)
	if err != nil {
		return CompileResult{}, fmt.Errorf("feedbackcompiler: load ledger: %w", err)
	}
	return c.CompileFromLedger(projectRoot, ledger)
}

// CompileFromLedger accepts an already-loaded ledger. Extracted for testability.
func (c *Compiler) CompileFromLedger(projectRoot string, ledger *verdictledger.Ledger) (CompileResult, error) {
	transitions := scanTransitions(ledger)
	if len(transitions) == 0 {
		return CompileResult{}, nil
	}

	dir := filepath.Join(projectRoot, filepath.FromSlash(LearningsRelDir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return CompileResult{}, fmt.Errorf("feedbackcompiler: create learnings dir: %w", err)
	}

	now := c.nowUTC()
	var result CompileResult
	for _, t := range transitions {
		passTime, err := time.Parse(time.RFC3339, t.PassRecord.RunTime)
		if err != nil {
			return result, fmt.Errorf("feedbackcompiler: parse pass run_timestamp: %w", err)
		}

		filename := draftFilename(t.DirectiveID, passTime)
		dest := filepath.Join(dir, filename)

		if _, err := os.Stat(dest); err == nil {
			result.Skipped++
			continue
		}

		content, err := draftContent(t, now)
		if err != nil {
			return result, err
		}

		if err := writeDraft(dest, content); err != nil {
			return result, err
		}
		result.Drafts = append(result.Drafts, dest)
	}
	return result, nil
}

// writeDraft writes content to dest atomically.
func writeDraft(dest, content string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("feedbackcompiler: create dir %s: %w", filepath.Dir(dest), err)
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("feedbackcompiler: write draft tmp: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("feedbackcompiler: rename draft: %w", err)
	}
	return nil
}
