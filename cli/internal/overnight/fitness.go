package overnight

import (
	"sort"
	"time"
)

// DefaultRegressionFloor is the fallback per-metric regression floor used
// when a caller does not supply a tighter override in the floors map passed
// to FitnessSnapshot.Delta. Mirrors defaultRegressionFloor in types.go but
// is exported so command-line callers can reference the same constant.
const DefaultRegressionFloor = 0.05

// FitnessSnapshot is a normalized single-iteration fitness capture.
//
// Source-agnostic: any producer (corpus.FitnessVector, ao goals measure,
// retrieval-bench alone) can populate it by filling the Metrics map. The
// overnight loop only compares snapshots; it does not interpret individual
// metric names, so adding or removing metrics between iterations is
// handled explicitly in Delta.
type FitnessSnapshot struct {
	// Metrics is the per-metric score map. Values are expected to live on
	// a comparable scale (typically 0..1) but Delta makes no assumption
	// about range — it only subtracts curr - prev.
	Metrics map[string]float64

	// CapturedAt records when the snapshot was produced. Purely
	// informational; not used by Delta.
	CapturedAt time.Time
}

// MetricRegression describes a single metric that breached its floor
// between two snapshots. Drop is always positive (Previous - Current); a
// negative Drop indicates improvement and never populates this struct.
type MetricRegression struct {
	// Name is the metric key from the Metrics map.
	Name string `json:"name"`
	// Previous is the prior snapshot's value for this metric.
	Previous float64 `json:"previous"`
	// Current is the latest snapshot's value for this metric. A missing
	// key in the current snapshot is treated as 0 for the comparison and
	// yields a full-value drop.
	Current float64 `json:"current"`
	// Drop is Previous - Current; always positive when Regressed.
	Drop float64 `json:"drop"`
	// FloorBreached is the per-metric floor that was exceeded. This is
	// the effective floor after applying any override in the floors map.
	FloorBreached float64 `json:"floor_breached"`
}

// Delta returns the composite delta between this snapshot and prev, the
// list of metrics whose drop exceeded their per-metric floor, and a bool
// flag set iff regressions is non-empty.
//
// The composite is the arithmetic mean of per-metric (curr - prev)
// contributions — positive means "got better", negative means "got
// worse". Each unique key across both snapshots contributes exactly
// once:
//
//   - Metrics present in both: (curr - prev) is added to the composite;
//     if (prev - curr) > floor, the metric is also flagged as a
//     regression.
//   - Metrics present in prev but missing in curr: counted as a full
//     drop of size prev. The contribution to the composite is -prev. If
//     prev > floor the metric is flagged.
//   - Metrics present in curr but missing in prev: counted as a full
//     gain of +curr with no regression.
//
// floors is an optional per-metric override map; metrics without an
// entry fall back to defaultFloor. Callers supplying a zero or negative
// defaultFloor get DefaultRegressionFloor substituted transparently.
//
// A nil prev short-circuits to (0, nil, false): the first iteration has
// nothing to compare against and is therefore never considered
// regressed. Map iteration is done in sorted key order so the returned
// regression slice and composite computation are fully deterministic.
func (f FitnessSnapshot) Delta(prev *FitnessSnapshot, floors map[string]float64, defaultFloor float64) (composite float64, regressions []MetricRegression, regressed bool) {
	if prev == nil {
		return 0, nil, false
	}
	if defaultFloor <= 0 {
		defaultFloor = DefaultRegressionFloor
	}

	// Collect the union of metric names in sorted order for determinism.
	seen := make(map[string]struct{}, len(prev.Metrics)+len(f.Metrics))
	for k := range prev.Metrics {
		seen[k] = struct{}{}
	}
	for k := range f.Metrics {
		seen[k] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return 0, nil, false
	}

	var sum float64
	for _, name := range names {
		prevVal, hasPrev := prev.Metrics[name]
		currVal, hasCurr := f.Metrics[name]

		var delta float64
		switch {
		case hasPrev && hasCurr:
			delta = currVal - prevVal
		case hasPrev && !hasCurr:
			// Metric vanished from curr; treat as full drop.
			delta = -prevVal
		case !hasPrev && hasCurr:
			// New metric; pure gain, never a regression.
			delta = currVal
		}
		sum += delta

		// Only a negative delta can be a regression.
		if delta >= 0 {
			continue
		}
		drop := -delta
		floor := defaultFloor
		if override, ok := floors[name]; ok && override > 0 {
			floor = override
		}
		if drop > floor {
			regressions = append(regressions, MetricRegression{
				Name:          name,
				Previous:      prevVal,
				Current:       currVal,
				Drop:          drop,
				FloorBreached: floor,
			})
		}
	}

	composite = sum / float64(len(names))
	regressed = len(regressions) > 0
	return composite, regressions, regressed
}
