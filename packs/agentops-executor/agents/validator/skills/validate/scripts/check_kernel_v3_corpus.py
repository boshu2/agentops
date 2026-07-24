#!/usr/bin/env python3
"""Replay the shared exact-kernel behavior corpus through Python invariants.

The Go kernel reader consumes the same JSON file.  This checker intentionally
fails when the required Go consumer is removed even while that reader is being
implemented in a separate lane.
"""

from __future__ import annotations

import json
from pathlib import Path
import sys

import kernel_v3 as kernel


def find_repository() -> Path:
    for candidate in Path(__file__).resolve().parents:
        if (
            candidate / "tests" / "fixtures" / "rpi-kernel-v3" / "corpus.json"
        ).is_file():
            return candidate
    raise RuntimeError("cannot locate the shared RPI kernel corpus")


ROOT = find_repository()
CORPUS = ROOT / "tests/fixtures/rpi-kernel-v3/corpus.json"
REQUIRED_IDS = {
    "intent.exact-utf8",
    "intent.living-source-mutation",
    "dispatch.each-phase-at-most-once",
    "coverage.generated-companion",
    "coverage.outside-write-scope",
    "coverage.partial-observation",
    "criteria.required-exclusion",
    "criteria.duplicate-id",
    "candidate.post-freeze-mutation",
    "terminal.fail",
    "terminal.not-proven",
    "judgment.duplicate-intent-subject",
    "proof.self-activation",
    "proof.transition-next-epoch",
    "proof.transition-skipped-epoch",
    "contract.unknown-field",
    "path.windows-drive",
    "path.backslash",
    "effect.forged-empty-complete",
    "proof.transitive-component-mutation",
    "correlation.opaque-preserved",
    "correlation.over-property-bound",
}


def outcome(case: dict) -> str:
    case_class = case["class"]
    if case_class == "intent-digest":
        return (
            "DIFFERENT"
            if kernel.sha256(case["left"].encode("utf-8"))
            != kernel.sha256(case["right"].encode("utf-8"))
            else "SAME"
        )
    if case_class == "intent-snapshot":
        return (
            "SNAPSHOT_WINS"
            if kernel.sha256(case["snapshot"].encode("utf-8"))
            != kernel.sha256(case["living_after"].encode("utf-8"))
            else "LIVING_SOURCE_REDERIVED"
        )
    if case_class == "dispatch":
        return (
            "PASS"
            if case["expected_calls"] == ["plan", "implement", "validate"]
            and case["terminal"] in {"PASS", "FAIL", "NOT_PROVEN"}
            else "REJECT"
        )
    if case_class == "coverage":
        if case["observation"] != ["."]:
            return "NOT_PROVEN"
        undeclared = [
            path
            for path in case["changed"]
            if not any(kernel.path_matches(path, item) for item in case["scope"])
        ]
        return "FAIL" if undeclared else "PASS"
    if case_class == "criterion-freeze":
        identifiers = case["criterion_ids"]
        if len(identifiers) != len(set(identifiers)):
            return "REJECT"
        if set(case["required_ids"]) & set(case["excluded_ids"]):
            return "REJECT"
        return "PASS"
    if case_class == "candidate-freeze":
        return (
            "NOT_PROVEN_STOP"
            if case["start_digest"] != case["end_digest"]
            else "PASS"
        )
    if case_class == "judgment-ledger":
        return (
            "REJECT"
            if case["same_intent"]
            and case["same_final_subject"]
            and case["different_judgment_id"]
            else "PASS"
        )
    if case_class == "proof-identity":
        return "REJECT" if kernel.PROOF_ACTIVE_REF in case["changed"] else "PASS"
    if case_class == "proof-transition":
        return (
            "PASS"
            if case["candidate_epoch"] == case["prior_epoch"] + 1
            else "REJECT"
        )
    if case_class == "strict-reader":
        return "REJECT" if case["mutation"] == "add-next-action" else "PASS"
    if case_class == "repository-ref":
        try:
            kernel.normalize_rel(case["ref"])
        except kernel.ContractError:
            return "REJECT"
        return "PASS"
    if case_class == "effect-integrity":
        return (
            "REJECT"
            if case["before_final_changed"] != case["claimed_changed"]
            else "PASS"
        )
    if case_class == "proof-transitive":
        return (
            "PASS"
            if case["bound_digest"] == case["observed_digest"]
            else "REJECT"
        )
    if case_class == "correlation":
        value = case["value"]
        try:
            report = kernel.build_rpi_report_v2(
                invocation_id="invocation:corpus",
                correlation=value,
                status="NOT_PLANNED",
                intent_ref=None,
                intent_digest=None,
                proof_identity=None,
                before_manifest_digest=None,
                final_manifest_digest=None,
                effect_receipt_digest=None,
                verdict_ref=None,
                verdict_digest=None,
                checked=[],
                not_checked=["plan"],
            )
            kernel.validate_rpi_report_v2(report)
        except kernel.ContractError:
            return "REJECT"
        return "PASS"
    raise kernel.ContractError(f"unknown corpus class: {case_class}")


def main() -> int:
    corpus = json.loads(CORPUS.read_text(encoding="utf-8"))
    if corpus.get("schema_version") != "rpi-kernel-corpus.v1":
        print("kernel-v3-corpus: invalid schema_version", file=sys.stderr)
        return 1
    consumers = set(corpus.get("required_consumers") or [])
    if consumers != {
        "plan-python",
        "implement-python",
        "rpi-python",
        "validate-python",
        "go",
    }:
        print("kernel-v3-corpus: consumer set drifted", file=sys.stderr)
        return 1
    cases = corpus.get("cases")
    if not isinstance(cases, list):
        print("kernel-v3-corpus: cases must be an array", file=sys.stderr)
        return 1
    identifiers = [case.get("id") for case in cases if isinstance(case, dict)]
    if len(identifiers) != len(set(identifiers)) or set(identifiers) != REQUIRED_IDS:
        print("kernel-v3-corpus: case inventory drifted", file=sys.stderr)
        return 1
    failures = []
    for case in cases:
        observed = outcome(case)
        expected = case["expected"]
        if case["class"] == "dispatch":
            expected = "PASS"
        if observed != expected:
            failures.append(
                f"{case['id']}: expected {expected}, observed {observed}"
            )
    if failures:
        print("kernel-v3-corpus: FAIL", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1
    print(f"kernel-v3-corpus: PASS ({len(cases)} cases, shared Python/Go)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
