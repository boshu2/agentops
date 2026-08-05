"""Command-line entry point so Go callers (and shell scripts) can drive the
§6.5 verdict pipeline end-to-end.

Usage:

    python -m _stats.cli verdict \
        --suite-id <id> \
        --arms a,b \
        --inputs <bootstrap-inputs.json> \
        --decision-rule '{"kind":"ci_excludes_zero","confidence":0.95}' \
        --n-required 100 \
        --mde 0.03 \
        [--B 10000]

    python -m _stats.cli n-required \
        --baseline-rate 0.5 \
        --mde 0.03 \
        --alpha 0.05 \
        [--power 0.80] [--paired true]

The verdict subcommand reads the bootstrap inputs file (canonical JSON list
of {sample_id, seed, score_arm_a, score_arm_b}), derives the bootstrap_seed
per §6.5, runs the paired cluster-bootstrap, computes the verdict, and
prints a JSON object containing all fields the manifest needs.

Exit code: 0 on success, 1 on argument error, 2 on internal error.
"""
from __future__ import annotations

import argparse
import json
import sys
from typing import Any, Dict

from .bootstrap import paired_cluster_bootstrap
from .inputs import (
    BootstrapInput,
    bootstrap_inputs_hash,
    load_bootstrap_inputs,
    paired_sample_ids_hash,
)
from .power import PowerInputs, power_derived_n_required
from .seed import derive_bootstrap_seed
from .verdict import compute_verdict


def cmd_verdict(args) -> int:
    inputs = load_bootstrap_inputs(args.inputs)
    if not inputs:
        print(json.dumps({"error": "no inputs"}), file=sys.stderr)
        return 2
    decision_rule = json.loads(args.decision_rule)
    arm_ids = [a.strip() for a in args.arms.split(",") if a.strip()]
    if len(arm_ids) < 2:
        print(json.dumps({"error": "need >=2 arms"}), file=sys.stderr)
        return 2

    psh = paired_sample_ids_hash(inputs)
    bih = bootstrap_inputs_hash(inputs)
    seed = derive_bootstrap_seed(
        suite_id=args.suite_id,
        arm_ids=arm_ids,
        paired_sample_ids_hash=psh,
        decision_rule=decision_rule,
    )

    confidence = float(decision_rule.get("confidence", 0.95))
    result = paired_cluster_bootstrap(
        inputs,
        bootstrap_seed=seed,
        confidence=confidence,
        B=int(args.B),
    )
    verdict = compute_verdict(
        result,
        rule_kind=decision_rule.get("kind", "ci_excludes_zero"),
        min_delta=decision_rule.get("min_delta"),
        n_required=int(args.n_required),
        mde=(float(args.mde) if args.mde is not None else None),
    )

    out: Dict[str, Any] = {
        "verdict": verdict.to_manifest_field(),
        "delta_point": verdict.delta_point,
        "ci_low": verdict.ci_low,
        "ci_high": verdict.ci_high,
        "n_clusters": verdict.n,
        "n_required": verdict.n_required,
        "rule_kind": verdict.rule_kind,
        "rule_passed": verdict.rule_passed,
        "degenerate": result.degenerate,
        "bootstrap_seed": seed,
        "B": result.B,
        "confidence": confidence,
        "paired_sample_ids_hash": psh,
        "bootstrap_inputs_hash": bih,
        "notes": verdict.notes,
    }
    print(json.dumps(out, sort_keys=True, separators=(",", ":")))
    return 0


def cmd_n_required(args) -> int:
    p = PowerInputs(
        baseline_rate=float(args.baseline_rate),
        minimum_detectable_effect=float(args.mde),
        alpha=float(args.alpha),
        power=float(args.power),
        paired=(str(args.paired).lower() != "false"),
        variance=(float(args.variance) if args.variance is not None else None),
    )
    n = power_derived_n_required(p)
    print(json.dumps({"n_required": n}, sort_keys=True))
    return 0


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(prog="_stats")
    sub = parser.add_subparsers(dest="cmd", required=True)

    v = sub.add_parser("verdict", help="run paired cluster-bootstrap + verdict")
    v.add_argument("--suite-id", required=True)
    v.add_argument("--arms", required=True, help="comma-separated arm ids (e.g. ms:a,ms:b)")
    v.add_argument("--inputs", required=True, help="path to canonical bootstrap-inputs JSON")
    v.add_argument("--decision-rule", required=True, help="JSON object")
    v.add_argument("--n-required", required=True)
    v.add_argument("--mde", default=None)
    v.add_argument("--B", default="10000")
    v.set_defaults(func=cmd_verdict)

    p = sub.add_parser("n-required", help="compute power-derived n_required")
    p.add_argument("--baseline-rate", required=True)
    p.add_argument("--mde", required=True)
    p.add_argument("--alpha", required=True)
    p.add_argument("--power", default="0.80")
    p.add_argument("--paired", default="true")
    p.add_argument("--variance", default=None)
    p.set_defaults(func=cmd_n_required)

    args = parser.parse_args(argv)
    try:
        return args.func(args)
    except Exception as e:  # noqa: BLE001
        print(json.dumps({"error": type(e).__name__, "message": str(e)}), file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
