package yieldledger

// Gauge computation for the dynamo yield vector (ag-qzinh): the dial on top of
// the meter ag-grcz3 built. ComputeGauges reads the bead-keyed operational event
// stream and derives the gauges per
// .agents/specs/2026-06-14-yield-vector-and-ledger-event-gap.md section
// "Derived (computed by ag-qzinh, NOT stored)". Nothing here mutates the ledger;
// the L loss classification is a READ-TIME join over the bead's terminal
// disposition, so a row emitted "productive" on a bead later refuted is counted
// as loss without rewriting an append-only event (spec L, codex attempt-3).
//
// Pre-registered shadow-mode actuation hypotheses (the knob each gauge WOULD
// move) live in ActuationHypotheses; they are PRINTED by the CLI, never
// auto-steered — auto-tuning is ag-qpg99, deferred (pre-registration §3).

// SpendMeasure names which usage field the R / A_over_R / L denominators sum.
// The spec lets ag-qzinh pick tokens_out or tokens_in+out; we make the choice
// explicit and report it in the output so a reader knows what R means.
const SpendMeasure = "tokens_out"

// LossCategory is a read-time loss classification of a single usage row.
type LossCategory string

const (
	// LossProductive is spend on an accepted bead, not in a rework/coordination
	// phase — i.e. spend that produced accepted work. Not a loss.
	LossProductive LossCategory = "productive"
	// LossRejected is spend on a bead that never reached terminal accept.
	LossRejected LossCategory = "rejected"
	// LossRework is spend emitted under phase==rework (an attempt>1 redo).
	LossRework LossCategory = "rework"
	// LossCoordination is spend emitted under phase==coordination.
	LossCoordination LossCategory = "coordination"
)

// CPending is the sentinel C value when ag-8p8o has not yet published a corpus
// delta. C is CONSUMED, never recomputed here; an unpublished delta is reported
// as pending rather than fabricated (spec C; ag-8p8o is in_progress).
const CPending = "pending"

// LossBreakdown carries the per-category loss spend that sums into L.
type LossBreakdown struct {
	Rejected     int `json:"rejected"`
	Rework       int `json:"rework"`
	Coordination int `json:"coordination"`
	Productive   int `json:"productive"`
}

// LossSpend returns the spend counted as loss (everything but productive).
func (b LossBreakdown) LossSpend() int { return b.Rejected + b.Rework + b.Coordination }

// Gauges is the computed yield vector for one run.
type Gauges struct {
	RunID string `json:"run_id"`

	// A = count(accept events) for the run.
	A int `json:"a_accepted"`

	// R = total usage spend (SpendMeasure) for the run — the raw-input
	// denominator. Reported so A_over_R and L are interpretable.
	R int `json:"r_raw_input"`

	// Q = difficulty-weighted first-pass yield. The LEAD gauge.
	//   numerator   = Σ(w · [bead whose attempt==1 verdict is CONFIRMED AND has a matching accept])
	//   denominator = Σ(w · [distinct attempted beads, i.e. with ≥1 gate-verdict])
	// w = that bead's gate-verdict.difficulty. QDefined is false when the
	// denominator is 0 (no attempted beads) so a 0/0 reads as "no signal", not 0.
	Q             float64 `json:"q_first_pass_yield"`
	QDefined      bool    `json:"q_defined"`
	QNumerator    float64 `json:"q_numerator"`
	QDenominator  float64 `json:"q_denominator"`
	QCleanBeads   int     `json:"q_clean_first_pass_beads"`
	QAttemptBeads int     `json:"q_attempted_beads"`

	// AOverR = A ÷ R. WATCH ONLY — Goodhartable (atomization / chore-spam); never
	// a tuning target. AOverRDefined guards the R==0 divide.
	AOverR        float64 `json:"a_over_r_watch_only"`
	AOverRDefined bool    `json:"a_over_r_defined"`

	// E = count(gate-verdict disposition ∈ {ESCALATE, HOLD}) ÷ A. EDefined guards
	// the A==0 divide.
	E              float64 `json:"e_escalation_rate"`
	EDefined       bool    `json:"e_defined"`
	EEscalateHolds int     `json:"e_escalate_or_hold_verdicts"`

	// L = Σ(loss spend) ÷ R via a read-time join. LDefined guards the R==0 divide.
	L         float64       `json:"l_loss"`
	LDefined  bool          `json:"l_defined"`
	LSpend    int           `json:"l_loss_spend"`
	LCategory LossBreakdown `json:"l_breakdown"`

	// C = self-excitation corpus delta, CONSUMED from ag-8p8o. CPendingFlag is
	// true when no published delta exists (ag-8p8o in_progress); CValue then
	// carries the CPending sentinel and CNote explains why. Never fabricated.
	CPendingFlag bool    `json:"c_pending"`
	CValue       string  `json:"c_value"`
	CDelta       float64 `json:"c_delta,omitempty"`
	CNote        string  `json:"c_note"`

	// SpendMeasure echoes which usage field R/A_over_R/L summed.
	SpendMeasure string `json:"spend_measure"`
}

// spendOf returns the SpendMeasure value of one usage body. Centralized so the R
// denominator and the L numerator always agree on the measure.
func spendOf(u *UsageBody) int {
	if u == nil {
		return 0
	}
	// SpendMeasure == tokens_out: tokens_out is the produced-output measure, the
	// most defensible "raw input consumed to produce work" proxy that is symmetric
	// between accepted and rejected beads.
	return u.TokensOut
}

// runUsage returns the usage events for one run keyed by bead, plus the set of
// beads that have ≥1 accept and the set with ≥1 gate-verdict in this run.
func computeRunSets(l *Ledger, runID string) (
	accepted map[string]bool, // bead -> has a terminal accept this run
	attempted map[string]bool, // bead -> has ≥1 gate-verdict this run
) {
	accepted = map[string]bool{}
	attempted = map[string]bool{}
	for _, ev := range l.Events {
		if ev.RunID != runID {
			continue
		}
		switch ev.Event {
		case EventAccept:
			accepted[ev.BeadID] = true
		case EventGateVerdict:
			attempted[ev.BeadID] = true
		}
	}
	return accepted, attempted
}

// difficultyOf returns the bead's difficulty weight for this run, taken from its
// attempt==1 gate-verdict (the intrinsic scope weight set once per bead). Falls
// back to the first gate-verdict's difficulty if no attempt==1 row exists, and 0
// if the bead has no gate-verdict.
func difficultyOf(l *Ledger, runID, beadID string) float64 {
	var first *GateVerdictBody
	for _, ev := range l.Events {
		if ev.RunID != runID || ev.BeadID != beadID || ev.Event != EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		if first == nil {
			first = ev.GateVerdict
		}
		if ev.GateVerdict.Attempt == 1 {
			return ev.GateVerdict.Difficulty
		}
	}
	if first != nil {
		return first.Difficulty
	}
	return 0
}

// cleanFirstPass reports whether the bead is a verifiable clean first pass: its
// attempt==1 gate-verdict is CONFIRMED AND an accept event for this bead is
// authorized by THAT attempt-1 verdict specifically — accept.gate_verdict_ref
// .head_sha must equal the attempt-1 gate-verdict's head_sha. A bead refuted on
// attempt 1 and only later confirmed (accept references a later attempt's
// head_sha) is NOT clean first pass: its accept is not bound to the attempt-1
// verdict, so it does not inflate Q. head_sha commit-binding makes this real.
func cleanFirstPass(l *Ledger, runID, beadID string) bool {
	// Find the attempt==1 gate-verdict's head_sha and confirm disposition.
	var attempt1SHA string
	var attempt1Confirmed bool
	for _, ev := range l.Events {
		if ev.RunID != runID || ev.BeadID != beadID || ev.Event != EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		if ev.GateVerdict.Attempt == 1 {
			attempt1SHA = ev.GateVerdict.HeadSHA
			attempt1Confirmed = ev.GateVerdict.Disposition == DispositionConfirmed
			break
		}
	}
	if !attempt1Confirmed || attempt1SHA == "" {
		return false
	}
	// Require an accept for this bead whose gate_verdict_ref.head_sha matches the
	// attempt-1 verdict's head_sha (same bead). An accept authorized by a later
	// attempt does not count.
	for _, ev := range l.Events {
		if ev.RunID != runID || ev.BeadID != beadID || ev.Event != EventAccept || ev.Accept == nil {
			continue
		}
		ref := ev.Accept.GateVerdictRef
		if ref.BeadID == beadID && ref.HeadSHA == attempt1SHA {
			return true
		}
	}
	return false
}

// classifyUsage performs the read-time L join for one usage row: it joins the
// row's bead to that bead's terminal disposition this run. A never-accepted bead
// is a rejected loss; phase==rework is a rework loss; phase==coordination is a
// coordination loss; spend on an accepted bead otherwise is productive.
func classifyUsage(ev Event, accepted map[string]bool) LossCategory {
	switch ev.Usage.Phase {
	case PhaseRework:
		return LossRework
	case PhaseCoordination:
		return LossCoordination
	}
	if !accepted[ev.BeadID] {
		return LossRejected
	}
	return LossProductive
}

// ComputeGauges derives the yield vector for runID from the ledger. C is
// consumed via cIn: pass a published corpus delta (cDelta, cKnown=true) or leave
// it unknown to get the pending sentinel. Nothing here recomputes C.
func ComputeGauges(l *Ledger, runID string, cDelta float64, cKnown bool) Gauges {
	g := Gauges{RunID: runID, SpendMeasure: SpendMeasure}

	accepted, attempted := computeRunSets(l, runID)

	// A and R.
	for _, ev := range l.Events {
		if ev.RunID != runID {
			continue
		}
		switch ev.Event {
		case EventAccept:
			g.A++
		case EventUsage:
			g.R += spendOf(ev.Usage)
		case EventGateVerdict:
			if ev.GateVerdict != nil &&
				(ev.GateVerdict.Disposition == DispositionEscalate ||
					ev.GateVerdict.Disposition == DispositionHold) {
				g.EEscalateHolds++
			}
		}
	}

	// Q — difficulty-weighted first-pass yield over distinct attempted beads.
	for bead := range attempted {
		w := difficultyOf(l, runID, bead)
		g.QDenominator += w
		g.QAttemptBeads++
		// cleanFirstPass already requires an accept bound to the attempt-1 verdict
		// (head_sha match), so the bead-level accepted set is not consulted here:
		// an accept authorized by a LATER attempt must not inflate Q.
		if cleanFirstPass(l, runID, bead) {
			g.QNumerator += w
			g.QCleanBeads++
		}
	}
	if g.QDenominator > 0 {
		g.Q = g.QNumerator / g.QDenominator
		g.QDefined = true
	}

	// A/R — WATCH ONLY.
	if g.R > 0 {
		g.AOverR = float64(g.A) / float64(g.R)
		g.AOverRDefined = true
	}

	// E — escalation rate over accepts.
	if g.A > 0 {
		g.E = float64(g.EEscalateHolds) / float64(g.A)
		g.EDefined = true
	}

	// L — read-time loss join.
	for _, ev := range l.Events {
		if ev.RunID != runID || ev.Event != EventUsage || ev.Usage == nil {
			continue
		}
		spend := spendOf(ev.Usage)
		switch classifyUsage(ev, accepted) {
		case LossRejected:
			g.LCategory.Rejected += spend
		case LossRework:
			g.LCategory.Rework += spend
		case LossCoordination:
			g.LCategory.Coordination += spend
		case LossProductive:
			g.LCategory.Productive += spend
		}
	}
	g.LSpend = g.LCategory.LossSpend()
	if g.R > 0 {
		g.L = float64(g.LSpend) / float64(g.R)
		g.LDefined = true
	}

	// C — consumed, never recomputed.
	if cKnown {
		g.CPendingFlag = false
		g.CDelta = cDelta
		g.CValue = "published"
		g.CNote = "corpus delta consumed from ag-8p8o's published artifact"
	} else {
		g.CPendingFlag = true
		g.CValue = CPending
		g.CNote = "ag-8p8o (corpus delta) has no published artifact yet — pending, not fabricated"
	}

	return g
}

// ActuationHypothesis is one pre-registered shadow-mode decision rule: the knob a
// gauge result WOULD move. These are PRINTED, never executed (ag-qpg99 deferred).
// Verbatim from 2026-06-14-dynamo-yield-PRE-REGISTRATION.md §3.
type ActuationHypothesis struct {
	Trigger string `json:"trigger"`
	Knob    string `json:"knob"`
	Mode    string `json:"mode"`
}

// ActuationHypotheses returns the pre-registered shadow-mode actuation
// hypotheses table (pre-registration §3), shipped verbatim as the decision rules.
// They are reported as "shadow, not auto-steered".
func ActuationHypotheses() []ActuationHypothesis {
	return []ActuationHypothesis{
		{
			Trigger: "Q (first-pass yield) drops below run baseline",
			Knob:    "tighten review diversity (>=2 families) / lower retry limit",
			Mode:    "shadow",
		},
		{
			Trigger: "C (corpus delta, ag-8p8o) <= 0 across a window",
			Knob:    "the field isn't self-exciting -> stop, inspect context budget",
			Mode:    "shadow",
		},
		{
			Trigger: "Rework count spikes on one bead class",
			Knob:    "add a pre-flight gate / pawl for that class",
			Mode:    "shadow",
		},
		{
			Trigger: "E (escalations/accept) rises",
			Knob:    "autonomy regressing -> inspect gate false-refutes",
			Mode:    "shadow",
		},
		{
			Trigger: "A/R moves",
			Knob:    "WATCH ONLY — never actuate (atomization/chore-spam Goodhart)",
			Mode:    "watch",
		},
	}
}
