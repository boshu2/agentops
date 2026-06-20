// `ao beads verify-acceptance` — assert every bead carries the acceptance
// contract that fits its TYPE, read br-natively (never the retired `bd`).
//
// Unlike `ao beads scenarios` (which shells `bd show` and forces Gherkin onto
// every bead), this verifier branches on `issue_type` and applies the right
// contract per type:
//
//	feature  -> Gherkin "## Scenarios" block + a TDD/test signal
//	spike    -> decision-criteria
//	design   -> formal-spec
//	test     -> assertion-inventory
//	cutover  -> migration-checklist
//	task/epic-> a baseline acceptance signal (acceptance_criteria block or an
//	            'Acceptance' heading) — NOT Gherkin, but not a free pass either
//	other    -> UNDEFINED (no contract defined; never a silent skip)
//
// It is advisory by default (prints a per-bead verdict, exits 0) so it can run
// over today's ledger as an adoption ratchet. With --strict, any FAIL or
// UNDEFINED maps to a non-zero exit — the gate posture for new/changed beads.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/scenarios"
)

// execBR is the single entry point for shelling out to br (beads_rust), the
// sanctioned tracker. It is a package var so tests inject a fake without a live
// ledger — the br-native analog of execBD. BEADS_DIR is inherited from the
// process environment (exec.Command keeps os.Environ by default).
var execBR = func(args ...string) ([]byte, error) {
	cmd := exec.Command("br", args...)
	return cmd.Output()
}

// acceptBead is the subset of a br bead the verifier needs.
type acceptBead struct {
	ID          string `json:"id"`
	IssueType   string `json:"issue_type"`
	Description string `json:"description"`
}

// acceptanceVerdict is the per-bead outcome.
type acceptanceVerdict string

const (
	acPass      acceptanceVerdict = "PASS"
	acFail      acceptanceVerdict = "FAIL"
	acUndefined acceptanceVerdict = "UNDEFINED"
)

// brErrorEnvelope matches br's `{"error": {...}}` failure shape. br emits this
// object with exit code 0 for a missing id, so detecting it at PARSE time — not
// via the exit code — is the only guard against coercing a missing bead into a
// zero-value (empty-description) bead that would falsely PASS.
type brErrorEnvelope struct {
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// parseBeadsFromBRJSON parses `br show <id...> --format json` output into beads.
// br emits a JSON array; it emits an `{"error":...}` object (exit 0) when an id
// is not found — that object is rejected, never coerced to a bead.
func parseBeadsFromBRJSON(out []byte) ([]acceptBead, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("br returned no output")
	}
	if trimmed[0] == '{' {
		var env brErrorEnvelope
		if err := json.Unmarshal(trimmed, &env); err == nil && env.Error != nil {
			return nil, fmt.Errorf("br error %s: %s", env.Error.Code, env.Error.Message)
		}
		return nil, fmt.Errorf("br returned an object, not the expected bead array")
	}
	var beads []acceptBead
	if err := json.Unmarshal(trimmed, &beads); err != nil {
		return nil, fmt.Errorf("parse br json array: %w", err)
	}
	if len(beads) == 0 {
		return nil, fmt.Errorf("bead not found")
	}
	return beads, nil
}

// checkAcceptanceContract applies the per-type acceptance contract and returns
// the verdict plus, for FAIL, the named missing elements.
func checkAcceptanceContract(b acceptBead) (acceptanceVerdict, []string) {
	desc := b.Description
	switch strings.ToLower(strings.TrimSpace(b.IssueType)) {
	case "feature":
		var missing []string
		if scenarios.Validate(desc) != nil {
			missing = append(missing, "Gherkin: a well-formed '## Scenarios' Given/When/Then block")
		}
		if !hasTestSignal(desc) {
			missing = append(missing, "test: a TDD signal (acceptance_criteria / check_command / check_type: test_pass)")
		}
		if len(missing) > 0 {
			return acFail, missing
		}
		return acPass, nil
	case "spike":
		return single(desc, []string{"decision", "decisions"}, "decision-criteria: a 'Decision' / 'Decision Criteria' section heading")
	case "design":
		return single(desc, []string{"spec", "specification", "specs"}, "formal-spec: a 'Spec' / 'Specification' section heading")
	case "test":
		return checkListContract(desc, []string{"assertion", "assertions"}, "assertion-inventory: an 'Assertions' section heading with at least one listed assertion")
	case "cutover":
		return checkCheckboxContract(desc, []string{"migration", "migrations"}, "migration-checklist: a 'Migration' section heading with at least one checkbox ('- [ ]')")
	case "task", "epic":
		// The mandate is "rigorous acceptance on EVERY bead" — task/epic do NOT
		// need Gherkin (this verifier exists precisely to not force it on
		// non-features), but they DO need a measurable acceptance signal.
		// skills/plan/SKILL.md and skills/discovery/references/dag.md require
		// acceptance_criteria on every issue body, including the epic and its
		// children. Returning N/A here was a fail-open (cross-family REFUTE):
		// `--strict` would pass a task/epic with no acceptance at all.
		if hasAcceptanceSignal(desc) {
			return acPass, nil
		}
		return acFail, []string{"acceptance: a measurable acceptance_criteria block or 'Acceptance' section heading (every bead requires acceptance per skills/plan/SKILL.md)"}
	default:
		return acUndefined, nil
	}
}

// headingMatches reports whether any markdown heading line ('#'-prefixed) has a
// whole word EXACTLY equal to one of the keywords (case-insensitive). Anchoring
// to headings AND requiring an exact word (not a prefix or a whole-body
// substring) is what closes the loose-match fail-opens cross-family review
// flagged: "No acceptance criteria yet" has no heading, and "## Specific TBD"
// has the word "specific" — not "spec" — so neither passes. Provide both
// singular and plural keyword forms at the call site.
func headingMatches(desc string, keywords ...string) bool {
	for _, line := range strings.Split(desc, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "#") {
			continue
		}
		heading := strings.ToLower(strings.TrimLeft(t, "# "))
		for _, tok := range strings.FieldsFunc(heading, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			for _, kw := range keywords {
				if tok == kw {
					return true
				}
			}
		}
	}
	return false
}

// hasStructuralToken reports whether a description carries a distinctive yaml
// acceptance KEY — matched with its trailing colon so that prose mentioning the
// word (e.g. "No acceptance_criteria yet.") does NOT qualify; only an actual
// `key:` does.
func hasStructuralToken(desc string) bool {
	l := strings.ToLower(desc)
	for _, tok := range []string{"acceptance_criteria:", "check_command:", "check_type:"} {
		if strings.Contains(l, tok) {
			return true
		}
	}
	return false
}

// hasTestSignal reports whether a description carries a TDD/test signal — the
// "+TDD" half of the feature contract. Structural tokens only (no loose prose).
func hasTestSignal(desc string) bool {
	return hasStructuralToken(desc)
}

// hasAcceptanceSignal reports whether a description carries any measurable
// acceptance signal — the baseline contract for non-feature types that still
// fall under the "acceptance on every bead" mandate without needing Gherkin.
// A structural token OR an 'Acceptance' heading qualifies; bare prose does not.
func hasAcceptanceSignal(desc string) bool {
	return hasStructuralToken(desc) || headingMatches(desc, "acceptance")
}

// single passes when a markdown heading matches one of the keywords.
func single(desc string, keywords []string, missingMsg string) (acceptanceVerdict, []string) {
	if headingMatches(desc, keywords...) {
		return acPass, nil
	}
	return acFail, []string{missingMsg}
}

// sectionBody returns the body lines under the FIRST heading whose words
// exactly match one of the keywords, up to the next heading (or end of text),
// and whether such a section was found. Scoping the list/checkbox check to the
// matched section — rather than the whole body — closes a fail-open where a
// bullet under an unrelated heading (e.g. "## Notes") wrongly satisfied a
// "## Assertions" contract (cross-family REFUTE).
func sectionBody(desc string, keywords []string) (string, bool) {
	var body []string
	inSection := false
	for _, line := range strings.Split(desc, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			if inSection {
				break // the next heading ends our section
			}
			heading := strings.ToLower(strings.TrimLeft(t, "# "))
			for _, tok := range strings.FieldsFunc(heading, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
				for _, kw := range keywords {
					if tok == kw {
						inSection = true
					}
				}
			}
			continue
		}
		if inSection {
			body = append(body, line)
		}
	}
	return strings.Join(body, "\n"), inSection
}

// checkListContract passes when a matching section heading exists AND at least
// one bullet list item appears WITHIN that section.
func checkListContract(desc string, keywords []string, missingMsg string) (acceptanceVerdict, []string) {
	if body, ok := sectionBody(desc, keywords); ok && hasListItem(body) {
		return acPass, nil
	}
	return acFail, []string{missingMsg}
}

// checkCheckboxContract passes when a matching section heading exists AND at
// least one markdown checkbox appears WITHIN that section.
func checkCheckboxContract(desc string, keywords []string, missingMsg string) (acceptanceVerdict, []string) {
	if body, ok := sectionBody(desc, keywords); ok && hasCheckbox(body) {
		return acPass, nil
	}
	return acFail, []string{missingMsg}
}

func hasListItem(desc string) bool {
	for _, line := range strings.Split(desc, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			return true
		}
	}
	return false
}

func hasCheckbox(desc string) bool {
	l := strings.ToLower(desc)
	return strings.Contains(l, "- [ ]") || strings.Contains(l, "- [x]")
}

func newBeadsVerifyAcceptanceCmd() *cobra.Command {
	var strict, asJSON bool
	cmd := &cobra.Command{
		Use:   "verify-acceptance <bead-id>...",
		Short: "Assert each bead carries the acceptance contract for its type (br-native)",
		Long: `Read beads via br (never the retired bd) and check each bead carries the
acceptance contract that fits its issue_type:

  feature -> Gherkin '## Scenarios' + a TDD/test signal
  spike   -> decision-criteria
  design  -> formal-spec
  test    -> assertion-inventory
  cutover -> migration-checklist
  task/epic -> N/A (structural, no per-item acceptance)
  other   -> UNDEFINED (no contract defined; reported, never silently skipped)

Advisory by default (prints verdicts, exits 0). With --strict, any FAIL or
UNDEFINED maps to a non-zero exit — the gate posture for new/changed beads.

Scope: this verifies the contract SHAPE for each type (the right section
headings / structural tokens). It does NOT yet validate that the acceptance
CONTENT is non-placeholder or measurable (e.g. a section that just says "TBD",
or a fenced acceptance_criteria block with measurable fields per
skills/plan/SKILL.md). Content-quality validation is tracked as a follow-up.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBeadsVerifyAcceptance(cmd, args, strict, asJSON)
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero on any FAIL or UNDEFINED verdict")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit verdicts as JSON")
	return cmd
}

type acceptanceResult struct {
	BeadID    string            `json:"bead_id"`
	IssueType string            `json:"issue_type"`
	Verdict   acceptanceVerdict `json:"verdict"`
	Missing   []string          `json:"missing,omitempty"`
}

func runBeadsVerifyAcceptance(cmd *cobra.Command, ids []string, strict, asJSON bool) error {
	out, err := execBR(append([]string{"show", "--format", "json"}, ids...)...)
	if err != nil {
		return fmt.Errorf("br show %v: %w", ids, err)
	}
	beads, err := parseBeadsFromBRJSON(out)
	if err != nil {
		return fmt.Errorf("read beads: %w", err)
	}

	// Defense-in-depth (cross-family REFUTE): never silently verify fewer beads
	// than were requested. br today fails the whole request when any id is
	// missing, but a future partial-response (a non-empty array omitting a
	// requested id) must NOT slip through as an exit-0 "all good" — assert every
	// requested id came back.
	got := make(map[string]bool, len(beads))
	for _, b := range beads {
		got[b.ID] = true
	}
	var absent []string
	for _, id := range ids {
		if !got[id] {
			absent = append(absent, id)
		}
	}
	if len(absent) > 0 {
		return fmt.Errorf("br did not return requested bead(s): %s", strings.Join(absent, ", "))
	}

	results := make([]acceptanceResult, 0, len(beads))
	nonPass := false
	for _, b := range beads {
		v, missing := checkAcceptanceContract(b)
		results = append(results, acceptanceResult{BeadID: b.ID, IssueType: b.IssueType, Verdict: v, Missing: missing})
		if v == acFail || v == acUndefined {
			nonPass = true
		}
	}

	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			return err
		}
	} else {
		for _, r := range results {
			fmt.Fprintf(cmd.OutOrStdout(), "%s [%s] %s\n", r.Verdict, r.IssueType, r.BeadID)
			for _, m := range r.Missing {
				fmt.Fprintf(cmd.OutOrStdout(), "    missing: %s\n", m)
			}
		}
	}

	if strict && nonPass {
		return &beadsExitError{code: 1}
	}
	return nil
}

// asBeadsExit is a thin errors.As wrapper used by tests.
func asBeadsExit(err error, target **beadsExitError) bool {
	return errors.As(err, target)
}

func init() {
	beadsCmd.AddCommand(newBeadsVerifyAcceptanceCmd())
}
