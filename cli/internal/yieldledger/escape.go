package yieldledger

import "sort"

// Escape detection (age-zqc): the input label for the self-improving membrane.
//
// An escape is a membrane MISS: a gate-verdict that CONFIRMED a bead which a
// later, strictly-higher-attempt gate-verdict for the SAME bead then REFUTED.
// The earlier CONFIRMED is the false-done the membrane let through; the later
// REFUTED is the catch that proves the miss. An escape is exactly the signal
// the membrane needs to get harder to fool: the check that would have caught it
// is derivable from it (escape -> finding -> membrane-check, age-zqc), and the
// escape_rate gauge (age-6ty) distinguishes a real catch-rate from a
// rubber-stamp one.
//
// This is a pure READ over the ledger — nothing here mutates events. It mirrors
// gauge.go's read-time terminal-disposition join: a unit emitted CONFIRMED that
// a later attempt refutes is reclassified as an escape without rewriting the
// append-only CONFIRMED row.

// Escape is one detected membrane miss for a bead within a run: the CONFIRMED
// gate-verdict that was later REFUTED at a higher attempt. Confirmed* describes
// the verdict that let the false-done through; Refuted* describes the verdict
// that eventually caught it (and so names the diversity that worked).
type Escape struct {
	BeadID string `json:"bead_id"`
	RunID  string `json:"run_id"`

	ConfirmedHeadSHA string `json:"confirmed_head_sha"`
	ConfirmedAttempt int    `json:"confirmed_attempt"`
	ConfirmedTS      string `json:"confirmed_ts"`

	RefutedHeadSHA  string   `json:"refuted_head_sha"`
	RefutedAttempt  int      `json:"refuted_attempt"`
	RefutedTS       string   `json:"refuted_ts"`
	RefuterFamilies []string `json:"refuter_families,omitempty"`
}

// verdictRow is a flattened gate-verdict carrying the envelope TS for ordering.
type verdictRow struct {
	disposition string
	headSHA     string
	attempt     int
	ts          string
	families    []string
}

// DetectEscapes returns the escapes in runID, one per escaping bead (v1
// counting): for each bead with at least one CONFIRMED gate-verdict followed by
// a REFUTED gate-verdict at a strictly higher attempt, the earliest such
// CONFIRMED is paired with the earliest REFUTED that follows it. Beads with no
// later refutation of a confirm are not escapes. Output is deterministically
// ordered by bead id.
//
// One-escape-per-bead is the minimal honest unit for the escape->check wire; the
// escape_rate gauge (age-6ty) owns finer multi-escape accounting.
func DetectEscapes(l *Ledger, runID string) []Escape {
	if l == nil {
		return nil
	}

	// Group this run's gate-verdicts by bead, preserving the data needed to order
	// by attempt (and TS/append order as a stable tiebreak).
	byBead := map[string][]verdictRow{}
	beadOrder := []string{}
	for _, ev := range l.Events {
		if ev.RunID != runID || ev.Event != EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		if _, seen := byBead[ev.BeadID]; !seen {
			beadOrder = append(beadOrder, ev.BeadID)
		}
		byBead[ev.BeadID] = append(byBead[ev.BeadID], verdictRow{
			disposition: ev.GateVerdict.Disposition,
			headSHA:     ev.GateVerdict.HeadSHA,
			attempt:     ev.GateVerdict.Attempt,
			ts:          ev.TS,
			families:    ev.GateVerdict.RefuterFamilies,
		})
	}

	sort.Strings(beadOrder)

	var escapes []Escape
	for _, bead := range beadOrder {
		rows := byBead[bead]
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].attempt != rows[j].attempt {
				return rows[i].attempt < rows[j].attempt
			}
			return rows[i].ts < rows[j].ts
		})

		// Earliest CONFIRMED, then the earliest REFUTED at a strictly higher attempt.
		var confirmed *verdictRow
		for i := range rows {
			if rows[i].disposition == DispositionConfirmed {
				confirmed = &rows[i]
				break
			}
		}
		if confirmed == nil {
			continue
		}
		var refuted *verdictRow
		for i := range rows {
			if rows[i].disposition == DispositionRefuted && rows[i].attempt > confirmed.attempt {
				refuted = &rows[i]
				break
			}
		}
		if refuted == nil {
			continue
		}

		escapes = append(escapes, Escape{
			BeadID:           bead,
			RunID:            runID,
			ConfirmedHeadSHA: confirmed.headSHA,
			ConfirmedAttempt: confirmed.attempt,
			ConfirmedTS:      confirmed.ts,
			RefutedHeadSHA:   refuted.headSHA,
			RefutedAttempt:   refuted.attempt,
			RefutedTS:        refuted.ts,
			RefuterFamilies:  refuted.families,
		})
	}
	return escapes
}
