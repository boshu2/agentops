package yieldledger

import (
	"sort"
	"time"
)

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
// explicit and report it in the output so a reader knows what R means. When a
// usage surface reported only a combined total (tokens_out=0, tokens_total>0 —
// the codex exec "tokens used" shape), the total is counted instead so measured
// spend never reads as zero (age-ivoq).
const SpendMeasure = "tokens_out (tokens_total fallback)"

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

	// A = count(GATE-ADMITTED accept events) for the run — an accept counts only
	// when its gate_verdict_ref resolves to a CONFIRMED gate-verdict for the same
	// bead+head_sha (mesh edge E-G admission). An accept with no matching CONFIRMED
	// verdict is an UNADMITTED deposit: it does NOT count toward A (that would let
	// the mesh self-excite on unjudged work) and is surfaced as Unadmitted instead.
	A int `json:"a_accepted"`

	// Unadmitted = accept events whose gate_verdict_ref does NOT resolve to a
	// CONFIRMED verdict (E-G leak surfaced). >0 means the mesh saw deposits that
	// bypassed the gate — C/self-excitation must be treated as suspect.
	Unadmitted int `json:"unadmitted_accepts"`

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

	// CatchRate = REFUTED ÷ (REFUTED + CONFIRMED) — the in-situ membrane
	// catch-rate: the fraction of adjudicated claimed-done units the membrane
	// caught as false-done (a REFUTED gate-verdict). CatchRate is nil (not 0)
	// when the denominator is 0 so a 0/0 reads as "no signal", with CatchRateNote
	// explaining why. CatchRateCrossFamily restricts the same ratio to verdicts
	// with cross_family == true (the diversity-gated subset).
	Refuted              int      `json:"refuted"`
	Confirmed            int      `json:"confirmed"`
	CatchRate            *float64 `json:"catch_rate"`
	CatchRateCrossFamily *float64 `json:"catch_rate_cross_family"`
	CatchRateNote        string   `json:"catch_rate_note,omitempty"`

	// Escapes = count(membrane MISSES): a CONFIRMED gate-verdict whose bead a
	// later, higher-attempt verdict REFUTED (DetectEscapes). EscapeRate = escapes
	// ÷ confirmed — the fraction of the membrane's CONFIRMEDs later proven wrong.
	// This is the v2 quality gauge (age-6ty) that catch_rate alone can't give: a
	// lenient/rubber-stamp membrane confirms freely, so its CONFIRMEDs are
	// untrustworthy — a HIGH escape_rate exposes that regardless of catch_rate.
	// EscapeRate is nil (not 0) when there are no CONFIRMED verdicts (0/0 reads as
	// "no signal"), with EscapeRateNote explaining why.
	Escapes        int      `json:"escapes"`
	EscapeRate     *float64 `json:"escape_rate"`
	EscapeRateNote string   `json:"escape_rate_note,omitempty"`

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
	// SpendMeasure: tokens_out is the produced-output measure, the most
	// defensible "raw input consumed to produce work" proxy that is symmetric
	// between accepted and rejected beads. When the surface reported only a
	// combined total (codex exec "tokens used"), tokens_out is 0 and the total
	// is the only truthful number — count it rather than reading measured spend
	// as zero (age-ivoq). An explicit tokens_out always wins (no double count).
	if u.TokensOut == 0 && u.TokensTotal > 0 {
		return u.TokensTotal
	}
	return u.TokensOut
}

type gateAttempt struct {
	headSHA    string
	attempt    int
	ts         time.Time
	eventIndex int
}

type runSets struct {
	accepted         map[string]bool
	attempted        map[string]bool
	acceptingAttempt map[string]int
	gateAttempts     map[string][]gateAttempt
}

// computeRunSets returns the run-level bead sets plus the gate attempt metadata
// needed for Q and L's accepted-bead rework attribution.
func computeRunSets(l *Ledger, runID string) runSets {
	sets := runSets{
		accepted:         map[string]bool{}, // bead -> has a terminal accept this run
		attempted:        map[string]bool{}, // bead -> has ≥1 gate-verdict this run
		acceptingAttempt: map[string]int{},  // bead -> attempt number authorized by accept
		gateAttempts:     map[string][]gateAttempt{},
	}
	for idx, ev := range l.Events {
		if ev.RunID != runID {
			continue
		}
		switch ev.Event {
		case EventAccept:
			// E-G admission: a bead is "accepted" for Q AND L only if the accept is
			// gate-admitted. An unadmitted accept must NOT mark the bead accepted —
			// else classifyUsage would count its spend productive instead of loss
			// (the L-side of the same self-excite-on-unjudged-work hole; codex gate).
			if acceptAdmitted(l, runID, ev) {
				sets.accepted[ev.BeadID] = true
			}
		case EventGateVerdict:
			if ev.GateVerdict == nil {
				continue
			}
			sets.attempted[ev.BeadID] = true
			sets.gateAttempts[ev.BeadID] = append(sets.gateAttempts[ev.BeadID], gateAttempt{
				headSHA:    ev.GateVerdict.HeadSHA,
				attempt:    ev.GateVerdict.Attempt,
				ts:         eventTime(ev),
				eventIndex: idx,
			})
		}
	}
	for bead := range sets.gateAttempts {
		sort.SliceStable(sets.gateAttempts[bead], func(i, j int) bool {
			left := sets.gateAttempts[bead][i]
			right := sets.gateAttempts[bead][j]
			if left.ts.Equal(right.ts) {
				return left.eventIndex < right.eventIndex
			}
			return left.ts.Before(right.ts)
		})
	}
	for _, ev := range l.Events {
		if ev.RunID != runID || ev.Event != EventAccept || ev.Accept == nil {
			continue
		}
		// Only an admitted accept authorizes an accepting-attempt (E-G consistency).
		if !acceptAdmitted(l, runID, ev) {
			continue
		}
		if attempt, ok := acceptingAttemptFor(ev, sets.gateAttempts[ev.BeadID]); ok {
			if current, exists := sets.acceptingAttempt[ev.BeadID]; !exists || attempt < current {
				sets.acceptingAttempt[ev.BeadID] = attempt
			}
		}
	}
	return sets
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

// acceptAdmitted reports whether an accept event is GATE-ADMITTED (mesh edge E-G):
// there exists a gate-verdict for the SAME bead whose head_sha equals the accept's
// gate_verdict_ref.head_sha AND whose disposition is CONFIRMED. An accept that
// references a missing, REFUTED, ESCALATE/HOLD, or mismatched verdict is NOT
// admitted — it is an unadmitted deposit that must not count as accepted work.
func acceptAdmitted(l *Ledger, runID string, accept Event) bool {
	if accept.Accept == nil {
		return false
	}
	ref := accept.Accept.GateVerdictRef
	// The ref must be internally consistent: it must claim THIS accept's bead. An
	// accept on bead-X carrying a ref.bead_id of bead-Y is malformed and must not
	// be admitted via bead-X's verdicts (codex gate: cross-bead-ref false-admit).
	// cleanFirstPass binds ref.BeadID the same way.
	if ref.HeadSHA == "" || ref.BeadID != accept.BeadID {
		return false
	}
	for _, ev := range l.Events {
		if ev.RunID != runID || ev.BeadID != accept.BeadID || ev.Event != EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		if ev.GateVerdict.HeadSHA == ref.HeadSHA && ev.GateVerdict.Disposition == DispositionConfirmed {
			return true
		}
	}
	return false
}

func eventTime(ev Event) time.Time {
	ts, _ := time.Parse(time.RFC3339, ev.TS)
	return ts
}

func acceptingAttemptFor(ev Event, gates []gateAttempt) (int, bool) {
	ref := ev.Accept.GateVerdictRef
	if ref.BeadID != ev.BeadID {
		return 0, false
	}
	for _, gate := range gates {
		if gate.headSHA == ref.HeadSHA {
			return gate.attempt, true
		}
	}
	return 0, false
}

func usageAttempt(ev Event, eventIndex int, gates []gateAttempt) (int, bool) {
	usageTS := eventTime(ev)
	for _, gate := range gates {
		if gate.ts.After(usageTS) || (gate.ts.Equal(usageTS) && gate.eventIndex > eventIndex) {
			return gate.attempt, true
		}
	}
	return 0, false
}

func (b *LossBreakdown) add(other LossBreakdown) {
	b.Rejected += other.Rejected
	b.Rework += other.Rework
	b.Coordination += other.Coordination
	b.Productive += other.Productive
}

// classifyUsage performs the read-time L join for one usage row. A
// never-accepted bead is rejected loss; explicit rework/coordination phases stay
// loss; an accepted bead's spend before the accepting attempt-N is rework loss.
// Ambiguous or post-confirm accepted-bead spend stays productive: L should
// under-attribute rework rather than over-charge productive work as loss.
func classifyUsage(ev Event, eventIndex int, sets runSets) LossBreakdown {
	spend := spendOf(ev.Usage)
	switch ev.Usage.Phase {
	case PhaseRework:
		return LossBreakdown{Rework: spend}
	case PhaseCoordination:
		return LossBreakdown{Coordination: spend}
	}
	if !sets.accepted[ev.BeadID] {
		return LossBreakdown{Rejected: spend}
	}
	acceptingAttempt, hasAcceptingAttempt := sets.acceptingAttempt[ev.BeadID]
	if !hasAcceptingAttempt || acceptingAttempt <= 1 {
		return LossBreakdown{Productive: spend}
	}
	if attempt, ok := usageAttempt(ev, eventIndex, sets.gateAttempts[ev.BeadID]); ok {
		if attempt < acceptingAttempt {
			return LossBreakdown{Rework: spend}
		}
		return LossBreakdown{Productive: spend}
	}
	return LossBreakdown{Productive: spend}
}

// accumulateAREvents walks runID's events once, accumulating A (gate-admitted
// accepts), Unadmitted, R (usage spend), EscalateHolds, and the terminal
// REFUTED/CONFIRMED adjudication counts into g. It returns the cross-family
// REFUTED and CONFIRMED counts (kept out of Gauges because they are only used to
// derive the cross-family catch rate). Behavior-identical to the inline A/R loop
// it was extracted from.
func accumulateAREvents(l *Ledger, runID string, g *Gauges) (refutedXF, confirmedXF int) {
	for _, ev := range l.Events {
		if ev.RunID != runID {
			continue
		}
		switch ev.Event {
		case EventAccept:
			// E-G admission: an accept counts toward A only if it is gate-admitted
			// (its gate_verdict_ref resolves to a CONFIRMED verdict for this
			// bead+head_sha). Otherwise it is an unadmitted deposit — surfaced, not
			// counted, so the mesh can't self-excite on unjudged work.
			if acceptAdmitted(l, runID, ev) {
				g.A++
			} else {
				g.Unadmitted++
			}
		case EventUsage:
			g.R += spendOf(ev.Usage)
		case EventGateVerdict:
			if ev.GateVerdict == nil {
				continue
			}
			if ev.GateVerdict.Disposition == DispositionEscalate ||
				ev.GateVerdict.Disposition == DispositionHold {
				g.EEscalateHolds++
			}
			// Catch-rate adjudication: REFUTED = a false-done the membrane caught;
			// CONFIRMED = an adjudicated true-done. ESCALATE/HOLD are not terminal
			// adjudications and do NOT enter the denominator.
			switch ev.GateVerdict.Disposition {
			case DispositionRefuted:
				g.Refuted++
				if ev.GateVerdict.CrossFamily {
					refutedXF++
				}
			case DispositionConfirmed:
				g.Confirmed++
				if ev.GateVerdict.CrossFamily {
					confirmedXF++
				}
			}
		}
	}
	return refutedXF, confirmedXF
}

// ComputeGauges derives the yield vector for runID from the ledger. C is
// consumed via cIn: pass a published corpus delta (cDelta, cKnown=true) or leave
// it unknown to get the pending sentinel. Nothing here recomputes C.
func ComputeGauges(l *Ledger, runID string, cDelta float64, cKnown bool) Gauges {
	g := Gauges{RunID: runID, SpendMeasure: SpendMeasure}

	sets := computeRunSets(l, runID)

	// A, R, escalation/holds, and terminal-adjudication counts, plus the
	// cross-family-restricted catch-rate accumulators.
	refutedXF, confirmedXF := accumulateAREvents(l, runID, &g)

	// CatchRate = REFUTED / (REFUTED + CONFIRMED) — nil when no adjudicated
	// gate-verdicts (0/0 reads as no signal, never a divide).
	if denom := g.Refuted + g.Confirmed; denom > 0 {
		cr := float64(g.Refuted) / float64(denom)
		g.CatchRate = &cr
	} else {
		g.CatchRateNote = "no confirmed+refuted gate-verdicts"
	}
	if denomXF := refutedXF + confirmedXF; denomXF > 0 {
		crXF := float64(refutedXF) / float64(denomXF)
		g.CatchRateCrossFamily = &crXF
	}

	// EscapeRate = escapes ÷ confirmed (age-6ty). escapes is the count of CONFIRMED
	// verdicts a later attempt refuted — each escape pins a distinct confirmed
	// verdict, so escapes <= confirmed and the ratio is in [0,1]. nil when there
	// are no confirmed verdicts (0/0 = no signal), never a fabricated 0.
	g.Escapes = len(DetectEscapes(l, runID))
	if g.Confirmed > 0 {
		er := float64(g.Escapes) / float64(g.Confirmed)
		g.EscapeRate = &er
	} else {
		g.EscapeRateNote = "no confirmed gate-verdicts to escape"
	}

	// Q — difficulty-weighted first-pass yield over distinct attempted beads.
	for bead := range sets.attempted {
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
	for idx, ev := range l.Events {
		if ev.RunID != runID || ev.Event != EventUsage || ev.Usage == nil {
			continue
		}
		g.LCategory.add(classifyUsage(ev, idx, sets))
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
