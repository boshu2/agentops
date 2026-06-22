package yieldledger

import "sort"

// False-alarm detection (SPC.2): the OTHER side of the membrane's two-sided
// fitness. An escape is a membrane MISS (a CONFIRMED a harder pass later REFUTED);
// a FALSE ALARM is a membrane CRY-WOLF — a REFUTED that a later pass overturned to
// CONFIRMED *on the SAME code*.
//
// The asymmetry is load-bearing and is why this is NOT a mirror of DetectEscapes:
// a REFUTED-then-CONFIRMED at a DIFFERENT head_sha is normal REWORK (the membrane
// correctly refused, the author fixed it, the fix confirmed) — the membrane
// WORKING, not crying wolf. Only a reversal on the SAME head_sha (no code change
// between the refute and the confirm) is evidence the refute was wrong. So the
// false-alarm signal requires head_sha equality; it is deliberately CONSERVATIVE
// (it misses cosmetic-change reversals), which is the safe direction for the
// two-sided fitness gate: under-counting false alarms never spuriously rejects a
// good gate.

// FalseAlarm is one detected membrane cry-wolf for a bead within a run: a REFUTED
// gate-verdict that a later, strictly-higher-attempt CONFIRMED overturned at the
// SAME head_sha.
type FalseAlarm struct {
	BeadID  string `json:"bead_id"`
	RunID   string `json:"run_id"`
	HeadSHA string `json:"head_sha"`

	RefutedAttempt   int    `json:"refuted_attempt"`
	RefutedTS        string `json:"refuted_ts"`
	ConfirmedAttempt int    `json:"confirmed_attempt"`
	ConfirmedTS      string `json:"confirmed_ts"`

	// Domain is the bounded-context the cry-wolf happened in (from the refuted
	// verdict) — the dimension the slow loop queries false alarms by.
	Domain string `json:"domain,omitempty"`
}

// DetectFalseAlarms returns the false alarms in runID, one per crying-wolf bead:
// for each bead with a REFUTED gate-verdict that a later, strictly-higher-attempt
// CONFIRMED for the SAME head_sha overturns, the earliest such REFUTED is paired
// with the earliest overturning CONFIRMED. Output is deterministically ordered by
// bead id. A reversal on a DIFFERENT head_sha is rework, not a false alarm, and is
// not returned.
func DetectFalseAlarms(l *Ledger, runID string) []FalseAlarm {
	if l == nil {
		return nil
	}

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
			domain:      ev.GateVerdict.Domain,
		})
	}
	sort.Strings(beadOrder)

	var alarms []FalseAlarm
	for _, bead := range beadOrder {
		rows := byBead[bead]
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].attempt != rows[j].attempt {
				return rows[i].attempt < rows[j].attempt
			}
			return rows[i].ts < rows[j].ts
		})

		// Earliest REFUTED, then the earliest CONFIRMED at a strictly higher attempt
		// with the SAME head_sha (a reversal on unchanged code).
		var refuted *verdictRow
		for i := range rows {
			if rows[i].disposition == DispositionRefuted {
				refuted = &rows[i]
				break
			}
		}
		if refuted == nil {
			continue
		}
		var confirmed *verdictRow
		for i := range rows {
			if rows[i].disposition == DispositionConfirmed &&
				rows[i].attempt > refuted.attempt &&
				rows[i].headSHA == refuted.headSHA {
				confirmed = &rows[i]
				break
			}
		}
		if confirmed == nil {
			continue
		}

		alarms = append(alarms, FalseAlarm{
			BeadID:           bead,
			RunID:            runID,
			HeadSHA:          refuted.headSHA,
			RefutedAttempt:   refuted.attempt,
			RefutedTS:        refuted.ts,
			ConfirmedAttempt: confirmed.attempt,
			ConfirmedTS:      confirmed.ts,
			Domain:           refuted.domain,
		})
	}
	return alarms
}
