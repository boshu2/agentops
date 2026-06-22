package governor

import (
	"sort"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// SPC.2 — the noise-band (special-cause-only adjustment) and the two-sided fitness
// gate (control-loop-model.md §4). These keep the self-improving membrane from
// TAMPERING: it adjusts ONLY on a repeated escape pattern past a control limit
// (never a one-off), and it ADMITS a new gate ONLY when that gate raises catch-rate
// without raising the false-alarm rate.

// DefaultSpecialCauseLimit (K) is the control limit: a domain must accumulate at
// least this many escapes within the rolling window before its pattern counts as
// special-cause (worth adjusting for). Below it, the escapes are common-cause
// noise and adjusting would be tampering. K=3 keeps a one-off (1) and a simple
// pair (2) as common-cause.
const DefaultSpecialCauseLimit = 3

// Adjust and Hold are the two noise-band decisions.
const (
	Adjust = "adjust"
	Hold   = "hold"
)

// NoiseBandConfig parameterizes the special-cause noise-band. Non-positive fields
// fall back to defaults via Resolve.
type NoiseBandConfig struct {
	WindowSize        int
	SpecialCauseLimit int
}

// DefaultNoiseBandConfig returns the canonical SPC.2 noise-band configuration.
func DefaultNoiseBandConfig() NoiseBandConfig {
	return NoiseBandConfig{WindowSize: DefaultWindowSize, SpecialCauseLimit: DefaultSpecialCauseLimit}
}

// Resolve fills any non-positive field from the defaults.
func (c NoiseBandConfig) Resolve() NoiseBandConfig {
	out := c
	if out.WindowSize <= 0 {
		out.WindowSize = DefaultWindowSize
	}
	if out.SpecialCauseLimit <= 0 {
		out.SpecialCauseLimit = DefaultSpecialCauseLimit
	}
	return out
}

// AdjustVerdict is the deterministic noise-band output: whether the membrane
// should ADJUST (a special-cause pattern exists) or HOLD (only common-cause noise).
type AdjustVerdict struct {
	Decision            string         `json:"decision"` // "adjust" | "hold"
	SpecialCauseDomains []string       `json:"special_cause_domains"`
	DomainEscapeCounts  map[string]int `json:"domain_escape_counts"`
	WindowSize          int            `json:"window_size"`
	SpecialCauseLimit   int            `json:"special_cause_limit"`
	Reason              string         `json:"reason"`
}

// ShouldAdjust reports whether a special-cause escape pattern exists in the rolling
// window: a domain with >= SpecialCauseLimit escapes within the most-recent
// WindowSize gate-verdict events across all runs. One-escape-per-bead (mirrors
// SPC.1 / DetectEscapes); an escape's domain is the confirmed (false-done) verdict's
// domain. If no domain reaches the limit, the signal is common-cause noise -> HOLD
// (adjusting now would be tampering).
func ShouldAdjust(l *yieldledger.Ledger, cfg NoiseBandConfig) AdjustVerdict {
	cfg = cfg.Resolve()
	v := AdjustVerdict{
		Decision:           Hold,
		DomainEscapeCounts: map[string]int{},
		WindowSize:         cfg.WindowSize,
		SpecialCauseLimit:  cfg.SpecialCauseLimit,
	}
	if l == nil {
		v.Reason = "no ledger: no escapes — hold (no signal)"
		return v
	}

	// Single append-ordered walk. Per bead: the lowest CONFIRMED attempt and the
	// domain of that confirm; the FIRST higher-attempt REFUTED is the escape-catch.
	// Each gate-verdict row records whether it is an escape-catch and that escape's
	// domain, so the window can count escapes-by-domain.
	type erow struct {
		escape bool
		domain string
	}
	minConfAttempt := map[string]int{}
	confDomain := map[string]string{}
	escaped := map[string]bool{}
	var rows []erow
	for _, ev := range l.Events {
		if ev.Event != yieldledger.EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		gv := ev.GateVerdict
		r := erow{}
		switch gv.Disposition {
		case yieldledger.DispositionConfirmed:
			if cur, ok := minConfAttempt[ev.BeadID]; !ok || gv.Attempt < cur {
				minConfAttempt[ev.BeadID] = gv.Attempt
				confDomain[ev.BeadID] = gv.Domain
			}
		case yieldledger.DispositionRefuted:
			if cur, ok := minConfAttempt[ev.BeadID]; ok && gv.Attempt > cur && !escaped[ev.BeadID] {
				r.escape = true
				r.domain = confDomain[ev.BeadID]
				escaped[ev.BeadID] = true
			}
		}
		rows = append(rows, r)
	}

	start := 0
	if len(rows) > cfg.WindowSize {
		start = len(rows) - cfg.WindowSize
	}
	for _, r := range rows[start:] {
		if r.escape {
			v.DomainEscapeCounts[r.domain]++
		}
	}

	for domain, n := range v.DomainEscapeCounts {
		if n >= cfg.SpecialCauseLimit {
			v.SpecialCauseDomains = append(v.SpecialCauseDomains, domain)
		}
	}
	sort.Strings(v.SpecialCauseDomains)

	if len(v.SpecialCauseDomains) > 0 {
		v.Decision = Adjust
		v.Reason = "special-cause: at least one domain reached the escape control limit in the window — a derived gate is warranted"
	} else {
		v.Reason = "common-cause only: no domain reached the escape control limit in the window — hold (adjusting would be tampering)"
	}
	return v
}

// FitnessSnapshot is a membrane fitness reading (the two signals the gate balances):
// CatchRate (REFUTED / (REFUTED+CONFIRMED) — higher catches more) and
// FalseAlarmRate (the cry-wolf rate from DetectFalseAlarms — higher is worse).
type FitnessSnapshot struct {
	CatchRate      float64 `json:"catch_rate"`
	FalseAlarmRate float64 `json:"false_alarm_rate"`
}

// FitnessVerdict is the deterministic two-sided admission decision for a candidate
// gate, comparing fitness BEFORE vs AFTER the gate.
type FitnessVerdict struct {
	Admit  bool   `json:"admit"`
	Reason string `json:"reason"`
}

// FitnessAdmits applies the TWO-SIDED FITNESS rule (control-loop-model.md §4): a
// candidate gate is admitted iff it RAISES catch-rate AND does NOT RAISE the
// false-alarm rate. A gate that catches more by crying wolf (false_alarm up) is
// rejected; so is a gate that does not improve catch-rate (no benefit). Strict on
// the benefit side (must improve), non-increasing on the harm side (must not worsen).
func FitnessAdmits(before, after FitnessSnapshot) FitnessVerdict {
	switch {
	case after.CatchRate <= before.CatchRate:
		return FitnessVerdict{Admit: false, Reason: "rejected: gate does not raise catch-rate (no benefit)"}
	case after.FalseAlarmRate > before.FalseAlarmRate:
		return FitnessVerdict{Admit: false, Reason: "rejected: gate raises the false-alarm rate (catches more by crying wolf)"}
	default:
		return FitnessVerdict{Admit: true, Reason: "admitted: gate raises catch-rate without raising the false-alarm rate"}
	}
}
