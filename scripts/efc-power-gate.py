#!/usr/bin/env python3
"""efc-power-gate.py — the read-only power-gate readiness check for the pre-registered
EFC-transfer experiment (age-k2w; protocol frozen in
.agents/research/2026-06-16-efc-transfer-preregistration.md).

The pre-registration (§55) forbids running the ΔAUC analysis until a power gate is met:
    >= 50 completed runs  AND  >= 15 in the minority (failure) class.
Crucially (§40-47) the failure label MUST be DECOUPLED from `disposition` (the gate
verdict is disqualified as circular). The decoupled failure signal available in the
ledger is an ESCAPE: a bead the membrane CONFIRMED that a later, strictly-higher-attempt
verdict REFUTED — a downstream-overturned outcome, independent of the in-run gate.

This script READS the yield ledger and reports the gate state; it computes NO ΔAUC and
manufactures NO verdict (per §58: below the gate, record INCONCLUSIVE and wait). It is
the honest navigator deliverable: it tells you exactly whether the experiment can run
and, if not, which threshold is binding.

Usage:
    scripts/efc-power-gate.py [--ledger PATH] [--json]
Exit: 0 = RUNNABLE (gate met), 2 = INCONCLUSIVE (gate not met), 1 = error.
"""
import argparse
import json
import sys
from collections import defaultdict

MIN_RUNS = 50      # frozen §55
MIN_MINORITY = 15  # frozen §55 — in the DECOUPLED failure class


def load_gate_verdicts(path):
    """Yield (run_id, bead_id, attempt, disposition) for each gate-verdict event.

    Tolerant to the on-disk shape: the disposition + attempt live under the event's
    `body` (the GateVerdictBody), matching the production writer.
    """
    rows = []
    with open(path) as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                e = json.loads(line)
            except ValueError:
                continue
            if e.get("event") != "gate-verdict":
                continue
            body = e.get("body") or e.get("gate_verdict") or {}
            rows.append((
                e.get("run_id", ""),
                e.get("bead_id", ""),
                body.get("attempt"),
                body.get("disposition"),
            ))
    return rows


def count_escapes(rows):
    """Distinct beads with an ESCAPE: a CONFIRMED then a strictly-higher-attempt REFUTED
    (mirrors yieldledger.DetectEscapes — the decoupled, non-circular failure label)."""
    by_bead = defaultdict(list)
    for _run, bead, attempt, disp in rows:
        if attempt is None or not bead:
            continue
        by_bead[bead].append((attempt, disp))
    escaped = 0
    for rows_b in by_bead.values():
        confirmed_attempts = [a for a, d in rows_b if d == "CONFIRMED"]
        if not confirmed_attempts:
            continue
        lowest_confirmed = min(confirmed_attempts)
        if any(d == "REFUTED" and a > lowest_confirmed for a, d in rows_b):
            escaped += 1
    return escaped


def evaluate(path):
    rows = load_gate_verdicts(path)
    runs = {r for r, _b, _a, _d in rows if r}
    n_runs = len(runs)
    minority = count_escapes(rows)
    runnable = n_runs >= MIN_RUNS and minority >= MIN_MINORITY
    binding = []
    if n_runs < MIN_RUNS:
        binding.append(f"runs {n_runs}/{MIN_RUNS}")
    if minority < MIN_MINORITY:
        binding.append(f"decoupled-failures(escapes) {minority}/{MIN_MINORITY}")
    return {
        "decision": "RUNNABLE" if runnable else "INCONCLUSIVE",
        "runs": n_runs,
        "min_runs": MIN_RUNS,
        "decoupled_minority_escapes": minority,
        "min_minority": MIN_MINORITY,
        "binding_constraints": binding,
        "gate_verdict_events": len(rows),
    }


def main():
    ap = argparse.ArgumentParser(description="EFC-transfer power-gate readiness (age-k2w, read-only)")
    ap.add_argument("--ledger", default=".agents/yield/yield-ledger.jsonl")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()
    try:
        result = evaluate(args.ledger)
    except FileNotFoundError:
        print(f"efc-power-gate: ledger not found: {args.ledger}", file=sys.stderr)
        return 1
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(f"EFC-transfer power gate (age-k2w): {result['decision']}")
        print(f"  runs                {result['runs']} (need >= {MIN_RUNS})")
        print(f"  decoupled failures  {result['decoupled_minority_escapes']} escapes (need >= {MIN_MINORITY})")
        if result["binding_constraints"]:
            print(f"  binding             {', '.join(result['binding_constraints'])}")
        if result["decision"] == "INCONCLUSIVE":
            print("  -> record INCONCLUSIVE and wait (prereg §58); the ΔAUC analysis MUST NOT run.")
        else:
            print("  -> power gate MET; the ΔAUC analysis (prereg §49-53) may now be built/run.")
    return 0 if result["decision"] == "RUNNABLE" else 2


if __name__ == "__main__":
    sys.exit(main())
