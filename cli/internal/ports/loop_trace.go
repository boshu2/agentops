// practices: [ddd-bounded-context, code-complete]
package ports

// CycleTrace is the XP/BDD/TDD evidence trace for one evolve cycle. It
// captures the Mt. Olympus continuous-evolution kernel shape so a
// reviewer can reconstruct a cycle without reading the transcript:
// goal hypothesis -> selected gap -> Gherkin scenario (or an explicit
// exemption) -> first failing proof -> red evidence -> green evidence
// -> refactor note -> validation evidence -> ratchet action -> goal
// reshape decision.
//
// Trivial one-shot cycles set ExemptionReason and leave the evidence
// fields empty — see TraceCompleteness for the completeness rule. All
// fields are omitempty so a partially-recorded or exempt trace stays
// compact on disk. soc-y5vh.9 (epic soc-y5vh, BC3 Loop).
type CycleTrace struct {
	GoalHypothesis     string   `json:"goal_hypothesis,omitempty"`
	SelectedGap        string   `json:"selected_gap,omitempty"`
	Gherkin            string   `json:"gherkin,omitempty"`
	ExemptionReason    string   `json:"exemption_reason,omitempty"`
	FirstFailingProof  string   `json:"first_failing_proof,omitempty"`
	RedEvidence        string   `json:"red_evidence,omitempty"`
	GreenEvidence      string   `json:"green_evidence,omitempty"`
	RefactorNote       string   `json:"refactor_note,omitempty"`
	ValidationEvidence string   `json:"validation_evidence,omitempty"`
	RatchetAction      string   `json:"ratchet_action,omitempty"`
	GoalReshape        string   `json:"goal_reshape,omitempty"`
	BeadID             string   `json:"bead_id,omitempty"`
	AcceptanceExamples []string `json:"acceptance_examples,omitempty"`
	ValidationCommands []string `json:"validation_commands,omitempty"`
	CloseoutVerdict    string   `json:"closeout_verdict,omitempty"`
}

// traceField pairs a kernel field's on-disk name with an accessor, so
// TraceCompleteness reports missing fields in kernel order.
type traceField struct {
	name string
	get  func(*CycleTrace) string
}

// requiredTraceFields are the ten evidence fields a non-exempt cycle
// must record, in kernel order. RefactorNote is required too — a
// no-op refactor is recorded as the literal "none", not omitted.
var requiredTraceFields = []traceField{
	{"goal_hypothesis", func(t *CycleTrace) string { return t.GoalHypothesis }},
	{"selected_gap", func(t *CycleTrace) string { return t.SelectedGap }},
	{"gherkin", func(t *CycleTrace) string { return t.Gherkin }},
	{"first_failing_proof", func(t *CycleTrace) string { return t.FirstFailingProof }},
	{"red_evidence", func(t *CycleTrace) string { return t.RedEvidence }},
	{"green_evidence", func(t *CycleTrace) string { return t.GreenEvidence }},
	{"refactor_note", func(t *CycleTrace) string { return t.RefactorNote }},
	{"validation_evidence", func(t *CycleTrace) string { return t.ValidationEvidence }},
	{"ratchet_action", func(t *CycleTrace) string { return t.RatchetAction }},
	{"goal_reshape", func(t *CycleTrace) string { return t.GoalReshape }},
}

// TraceCompleteness reports whether a CycleTrace satisfies the
// XP/BDD/TDD evidence discipline. A trace with a non-empty
// ExemptionReason is exempt (trivial one-shot cycles avoid false
// ceremony) and reports no missing fields. Otherwise every required
// evidence field must be non-empty; missing field names are returned
// in kernel order. A nil trace is not exempt and misses every field.
//
// This is a pure advisory helper. Per the soc-y5vh.9 non-goals it is
// NOT wired into any blocking gate — callers decide what to do with
// the result (e.g. surface it in a report, never to fail a build).
func TraceCompleteness(t *CycleTrace) (exempt bool, missing []string) {
	if t == nil {
		all := make([]string, len(requiredTraceFields))
		for i, f := range requiredTraceFields {
			all[i] = f.name
		}
		return false, all
	}
	if t.ExemptionReason != "" {
		return true, nil
	}
	for _, f := range requiredTraceFields {
		if f.get(t) == "" {
			missing = append(missing, f.name)
		}
	}
	return false, missing
}
