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
	"regexp"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

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

// placeholderBodies are the stand-in tokens that mark an UNFILLED section or
// criterion — structurally present (a heading, a bullet, a description) but with
// no real content. Content-quality validation (age-xmkn) treats a body whose
// only content is one of these as absent: a "## Assertions" section listing just
// "- TBD" is not a real assertion inventory, and a criterion described as "TODO"
// is not measurable. Matched case-insensitively against the trimmed token.
var placeholderBodies = map[string]bool{
	"tbd": true, "todo": true, "tba": true, "n/a": true, "na": true,
	"none": true, "fixme": true, "wip": true, "xxx": true, "...": true,
	"placeholder": true, "to be determined": true, "to be defined": true,
	"to be added": true, "?": true, "-": true,
}

// placeholderPhrases are multi-word stand-ins (a placeholder token plus filler)
// that also mark an unfilled body — caught in addition to the single tokens.
var placeholderPhrases = map[string]bool{
	"todo later": true, "tbd later": true, "fill in": true, "fill me in": true,
	"to do": true, "to be done": true, "coming soon": true, "tbd here": true,
}

// acLeadingMarker matches a single leading list / ordinal / checkbox marker:
// "-", "*", "+", "1." / "1)", or "[ ]" / "[x]". isPlaceholder strips these
// repeatedly so EVERY marker syntax (unordered, ordered, checkbox, and combos
// like "- [ ] TBD" or "1. TBD") normalizes to the bare body — closing the
// marker-class whack-a-mole as one regex rather than per-marker special cases
// (cross-family REFUTE found "-", then "- [ ]", then "1." one at a time).
var acLeadingMarker = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)]|\[[ xX]\])\s*`)

// isPlaceholder reports whether s, once stripped of leading list/ordinal/checkbox
// markers and trailing punctuation and lowercased, is nothing but a placeholder
// (an empty body, a known token like "TBD"/"...", or a known placeholder phrase).
func isPlaceholder(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	for {
		stripped := strings.TrimSpace(acLeadingMarker.ReplaceAllString(t, ""))
		if stripped == t {
			break
		}
		t = stripped
	}
	// Trim wrapping/leading/trailing punctuation from BOTH ends so the bare token
	// is exposed: "(TBD)", "[TBD]", "\"TBD.\"", "**TBD**", "TBD:" all reduce to
	// "tbd". Internal punctuation is preserved (e.g. "well-formed" stays real).
	// This generalizes the punctuation class the way acLeadingMarker generalizes
	// the marker class (cross-family REFUTE: wrapping parens slipped through).
	t = strings.Trim(t, " \t.,:;!?()[]{}<>\"'*_~`")
	// An empty result is itself a placeholder: a body that is ONLY markers and
	// punctuation ("...", "-", "- [ ]", "1.", "()") has no real content.
	return t == "" || placeholderBodies[t] || placeholderPhrases[t]
}

// acIDPattern is the canonical authored criterion id shape
// (schemas/execution-packet.schema.json #/$defs/Criterion): ac-<scope>.<n>.
// Unlike weight/optional/evidence_required (lifted by /discovery), id is authored
// in every canonical example and is load-bearing for stable per-criterion
// evidence/verdict binding — so it IS validated at authoring time (age-xmkn,
// cross-family REFUTE).
var acIDPattern = regexp.MustCompile(`^ac-[a-z0-9][a-z0-9._-]*\.[0-9]+$`)

// checkAcceptanceContract applies the per-type acceptance contract and returns
// the verdict plus, for FAIL, the named missing elements. Beyond the per-type
// SHAPE check it also runs CONTENT-QUALITY validation (age-xmkn): a section or
// criterion that is structurally present but only a placeholder ("TBD"), and a
// fenced acceptance_criteria block whose parsed criteria violate the canonical
// authored contract, both FAIL — even when the shape check alone would pass.
func checkAcceptanceContract(b acceptBead) (acceptanceVerdict, []string) {
	verdict, missing := checkAcceptanceShape(b)
	// Content-quality only sharpens a would-be PASS into a FAIL; it never rescues
	// an already-failing or UNDEFINED verdict. A present-but-invalid
	// acceptance_criteria block is a content defect on any type.
	if verdict == acPass {
		if problems := validateAcceptanceCriteriaContent(b.Description); len(problems) > 0 {
			return acFail, problems
		}
	}
	return verdict, missing
}

// checkAcceptanceShape is the per-type SHAPE contract (the original
// checkAcceptanceContract body): the right section headings / structural tokens
// for the bead's issue_type, now with placeholder-only bodies rejected.
func checkAcceptanceShape(b acceptBead) (acceptanceVerdict, []string) {
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
// A structural acceptance_criteria token OR an 'Acceptance' section with REAL
// (non-placeholder) body qualifies; a bare heading over "TBD" does not (age-xmkn).
func hasAcceptanceSignal(desc string) bool {
	return hasStructuralToken(desc) || sectionHasRealContent(desc, []string{"acceptance"})
}

// sectionHasRealContent reports whether a section whose heading matches one of
// the keywords exists AND has at least one body line that is real content —
// non-blank and not a placeholder ("TBD"). This is the content-quality core
// shared by every heading-based contract: a heading alone, or a heading over
// only placeholders, is not a satisfied contract (age-xmkn — the recurring
// cross-family REFUTE was that heading-present-but-empty bodies passed).
func sectionHasRealContent(desc string, keywords []string) bool {
	body, ok := sectionBody(desc, keywords)
	if !ok {
		return false
	}
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || isPlaceholder(t) {
			continue
		}
		return true
	}
	return false
}

// single passes when a markdown heading matches one of the keywords AND that
// section carries real (non-placeholder) body content — not just the heading.
func single(desc string, keywords []string, missingMsg string) (acceptanceVerdict, []string) {
	if sectionHasRealContent(desc, keywords) {
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

// acListItem matches a REAL markdown list item: an unordered (-,*,+) or ordered
// (1., 1)) bullet FOLLOWED BY whitespace and content. DETECTION is stricter than
// acLeadingMarker NORMALIZATION on purpose: normalization strips a leading marker
// even with no following space ("-foo" -> "foo"), but deciding whether a line IS
// a list item requires real markdown syntax — a space after the bullet and some
// content. So "-not a list", "1.real", and a bare "[x] foo" (no bullet) are NOT
// list items (cross-family REFUTE: acLeadingMarker over-matched non-list lines).
var acListItem = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\S`)

// hasListItem reports whether desc contains at least one REAL list item (ordered
// or unordered) whose content is not merely a placeholder. A placeholder bullet
// ("- TBD", "1. TBD") does not count.
func hasListItem(desc string) bool {
	for _, line := range strings.Split(desc, "\n") {
		t := strings.TrimSpace(line)
		if acListItem.MatchString(t) && !isPlaceholder(t) {
			return true
		}
	}
	return false
}

// acCheckboxMarker matches a markdown checkbox with ANY bullet char — "- [ ]",
// "* [x]", "+ [ ]" — so checkbox detection covers the same bullet class
// (acLeadingMarker) that isPlaceholder/hasListItem use, not just a hardcoded "-"
// (cross-family REFUTE: "* [ ] step" was wrongly rejected).
var acCheckboxMarker = regexp.MustCompile(`^\s*[-*+]\s*\[[ xX]\]\s*`)

// hasCheckbox reports whether desc contains at least one REAL checkbox — a
// "<bullet> [ ]"/"<bullet> [x]" item with task text that is not merely a
// placeholder. A marker-only checkbox ("- [ ]" with no step) is empty content
// (age-xmkn), mirroring hasListItem so all content-requiring checks reject
// placeholders consistently across the full bullet class.
func hasCheckbox(desc string) bool {
	for _, line := range strings.Split(desc, "\n") {
		t := strings.TrimSpace(line)
		if acCheckboxMarker.MatchString(t) && !isPlaceholder(t) {
			return true
		}
	}
	return false
}

// acCheckTypes is the canonical check_type enum (schemas/execution-packet.schema.json
// #/$defs/Criterion). A criterion with any other check_type is a contract violation.
var acCheckTypes = map[string]bool{
	"test_pass": true, "command_exit_zero": true, "file_exists": true,
	"grep_match": true, "manual": true, "council_judge": true, "custom_rubric": true,
}

// acRunnableCheckTypes are the check_types that are meaningless without a
// check_command (a shell command/script to run). manual/council_judge/custom_rubric
// are judged, not run, so they do not require one.
var acRunnableCheckTypes = map[string]bool{
	"test_pass": true, "command_exit_zero": true, "file_exists": true, "grep_match": true,
}

// acCriterion is the AUTHORED acceptance criterion shape (plan/SKILL.md). The
// full lifted Criterion (weight/optional/evidence_required) is added by
// /discovery STEP 6, NOT authored in the bead body, so content-quality validates
// only the measurable authored core — not those lifted fields (age-xmkn).
type acCriterion struct {
	ID           string `yaml:"id"`
	Description  string `yaml:"description"`
	CheckType    string `yaml:"check_type"`
	CheckCommand string `yaml:"check_command"`
	AgentJudge   string `yaml:"agent_judge"`
}

type acBlock struct {
	AcceptanceCriteria []acCriterion `yaml:"acceptance_criteria"`
}

// extractAcceptanceCriteriaYAML returns the YAML text of the acceptance_criteria
// block embedded in a bead description (with or without a ```yaml fence), or ""
// if no `acceptance_criteria:` key is present. It captures the key line plus its
// indented item lines, stopping at the next top-level (non-indented, non-blank)
// line — a markdown heading, a closing fence, or following prose/key.
func extractAcceptanceCriteriaYAML(desc string) string {
	lines := strings.Split(desc, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "acceptance_criteria:") {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	out := []string{lines[start]}
	for _, line := range lines[start+1:] {
		if strings.TrimSpace(line) == "" {
			out = append(out, line) // blank lines are valid inside the block
			continue
		}
		// A non-indented, non-blank line ends the block (heading, ``` fence, prose).
		if line[0] != ' ' && line[0] != '\t' {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// validateAcceptanceCriteriaContent parses the embedded acceptance_criteria block
// and validates each criterion against the canonical authored contract (age-xmkn):
// a measurable (non-placeholder) description, a valid check_type, a check_command
// for runnable check_types, and agent_judge when check_type == custom_rubric. It
// returns content-quality problems (empty when the block is absent or valid). A
// present-but-unparseable block is itself a problem (a malformed contract).
func validateAcceptanceCriteriaContent(desc string) []string {
	raw := extractAcceptanceCriteriaYAML(desc)
	if raw == "" {
		return nil // no block present — the SHAPE check owns "block required"
	}
	var block acBlock
	if err := yaml.Unmarshal([]byte(raw), &block); err != nil {
		return []string{"acceptance_criteria block is present but not valid YAML"}
	}
	if len(block.AcceptanceCriteria) == 0 {
		return []string{"acceptance_criteria block has no criteria (an empty or malformed list)"}
	}
	var problems []string
	for i, c := range block.AcceptanceCriteria {
		label := c.ID
		if strings.TrimSpace(label) == "" {
			label = fmt.Sprintf("#%d", i+1)
		}
		if id := strings.TrimSpace(c.ID); id == "" || !acIDPattern.MatchString(id) {
			problems = append(problems, fmt.Sprintf("criterion %s: missing or invalid id (must match ^ac-<scope>.<n>, e.g. ac-foo.1)", label))
		}
		if strings.TrimSpace(c.Description) == "" || isPlaceholder(c.Description) {
			problems = append(problems, fmt.Sprintf("criterion %s: missing or placeholder description (not measurable)", label))
		}
		ct := strings.TrimSpace(c.CheckType)
		switch {
		case ct == "":
			problems = append(problems, fmt.Sprintf("criterion %s: missing check_type", label))
		case !acCheckTypes[ct]:
			problems = append(problems, fmt.Sprintf("criterion %s: invalid check_type %q (not in the canonical enum)", label, ct))
		default:
			if acRunnableCheckTypes[ct] && strings.TrimSpace(c.CheckCommand) == "" {
				problems = append(problems, fmt.Sprintf("criterion %s: check_type %q requires a check_command", label, ct))
			}
			if ct == "custom_rubric" && strings.TrimSpace(c.AgentJudge) == "" {
				problems = append(problems, fmt.Sprintf("criterion %s: check_type custom_rubric requires agent_judge (the council/judge that owns the verdict)", label))
			}
		}
	}
	return problems
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
headings / structural tokens) AND its CONTENT QUALITY (age-xmkn): a section or
criterion that is structurally present but only a placeholder ("TBD") FAILs, and
a fenced acceptance_criteria block is parsed and each criterion validated against
the canonical authored contract (skills/plan/SKILL.md) — a measurable
description, a valid check_type, a check_command for runnable check_types, and
agent_judge when check_type == custom_rubric. The lifted fields (weight /
optional / evidence_required) are added by /discovery, not authored, so they are
not required here.`,
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
