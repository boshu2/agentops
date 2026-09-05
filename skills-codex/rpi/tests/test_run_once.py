from __future__ import annotations

import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


MODULE_PATH = Path(__file__).parents[1] / "scripts" / "run_once.py"
SPEC = importlib.util.spec_from_file_location("rpi_run_once", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)

# The REAL Validate implementation, not a stand-in. The composed-contract test
# below drives RPI against this module's actual identity functions; that is the
# only shape that can catch a disagreement between the two skills, which is
# exactly the defect that hid here (both suites green over a broken contract
# because the fake Validate borrowed RPI's own digest function).
VALIDATE_PATH = Path(__file__).parents[2] / "validate" / "scripts" / "validate.py"
VALIDATE_SPEC = importlib.util.spec_from_file_location("ao_validate", VALIDATE_PATH)
assert VALIDATE_SPEC and VALIDATE_SPEC.loader
VALIDATE = importlib.util.module_from_spec(VALIDATE_SPEC)
VALIDATE_SPEC.loader.exec_module(VALIDATE)

# A literal, independently written digest. Fakes must never derive an expected
# identity by calling the code under test — that is how the original defect
# stayed invisible.
INTENT_SNAPSHOT_BYTES = b'{"acceptance": ["works"], "intent_ref": "bead:agentops-test"}'
# The digest of the bytes above, computed by the test rather than by the module
# under test: a fake that borrows the subject's own digest function proves
# nothing, which is how the original identity defect stayed invisible.
INTENT_DIGEST = hashlib.sha256(INTENT_SNAPSHOT_BYTES).hexdigest()

# The SHARED adversarial corpus. workflows/rpi.js reads the same file through
# tests/scripts/agentops-product-boundary.bats, so a case only one side honors
# is a parity break, not a passing suite. (parents[3] is the repository root
# from both skills/rpi/tests/ and its skills-codex/ projection.)
LAW_CASES = json.loads(
    (Path(__file__).parents[3] / "tests" / "fixtures" / "rpi-convergence-law" / "cases.json").read_text(
        encoding="utf-8"
    )
)


def case_leg(verdict, findings, digest, family="fresh"):
    return {
        "status": verdict,
        "findings": [dict(f) for f in findings],
        "subject_digest": digest * 64,
        "evidence_refs": [],
        "validator_family": family,
        "checked": ["acceptance"],
        "not_checked": [],
    }


def case_round(spec):
    """Expand one shared-corpus round into a validate leg, or a list of them."""
    if "legs" in spec:
        families = ("fresh", "cross-family")
        return [
            case_leg(leg["verdict"], leg["findings"], spec["digest"], families[index % 2])
            for index, leg in enumerate(spec["legs"])
        ]
    return case_leg(spec["verdict"], spec["findings"], spec["digest"])


def validation_round(
    status,
    finding_ids,
    *,
    digest="a" * 64,
    evidence=(),
    family="fresh",
    summaries=None,
    classes=None,
    checked=("acceptance",),
    not_checked=(),
):
    """One validate leg's result, as the repair phase consumes it (pure data)."""
    summaries = summaries or {}
    classes = classes or {}
    return {
        "status": status,
        "findings": [
            dict(
                {"id": fid, "summary": summaries.get(fid, f"finding {fid}")},
                **({"class": classes[fid]} if fid in classes else {}),
            )
            for fid in finding_ids
        ],
        "subject_digest": digest,
        "evidence_refs": list(evidence),
        "validator_family": family,
        "checked": list(checked),
        "not_checked": list(not_checked),
    }


def continue_guard(intent):
    return {
        "decision": "CONTINUE",
        "reason": "The frozen outcome still requires implementation proof.",
        "frozen_outcome": str(intent),
        "parked_process_work": [],
        "remaining_proof": ["implementation", "fresh validation"],
        "stop_condition": "Stop after one fresh validation result.",
    }


class RunOnceTests(unittest.TestCase):
    def phases(self, verdict: str = "PASS"):
        calls: list[str] = []

        def plan(intent):
            calls.append("plan")
            return {
                "intent_ref": "bead:agentops-test",
                "intent": intent,
                "acceptance": ["works"],
                "acceptance_digest": INTENT_DIGEST,
                "intent_snapshot_bytes": INTENT_SNAPSHOT_BYTES,
            }

        def implement(_plan):
            calls.append("implement")
            return {"subject_manifest_digest": "a" * 64, "checks": ["focused"]}

        def validate(_plan, _candidate):
            calls.append("validate")
            return {
                "verdict": verdict,
                "acceptance_digest": INTENT_DIGEST,
                "intent_snapshot_bytes": INTENT_SNAPSHOT_BYTES,
                "subject_manifest_digest": "a" * 64,
                "author_context_id": "author-ctx",
                "validator_context_id": "validator-ctx",
                "freshness_attestation": {
                    "source": "runtime",
                    "attester_identity": "runtime:rpi-test",
                },
                "verdict_digest": "b" * 64,
                "verdict_ref": "/tmp/verdict.json",
                "checked": ["acceptance"],
                "not_checked": [],
            }

        return calls, plan, implement, validate

    def test_anti_ceremony_guard_runs_once_before_plan(self):
        calls, plan, implement, validate = self.phases()

        def anti_ceremony(intent):
            calls.append("anti-ceremony")
            return {
                "decision": "CONTINUE",
                "reason": "The frozen outcome still requires implementation proof.",
                "frozen_outcome": intent,
                "parked_process_work": [],
                "remaining_proof": ["implementation", "fresh validation"],
                "stop_condition": "Stop after one fresh validation result.",
            }

        result = MODULE.invoke_once(
            "intent",
            anti_ceremony,
            plan,
            implement,
            validate,
        )

        self.assertEqual(
            calls,
            ["anti-ceremony", "plan", "implement", "validate"],
        )
        self.assertEqual(result["status"], "PASS")

    def test_anti_ceremony_stop_dispatches_no_core_phase(self):
        calls: list[str] = []
        reason = "The proposed traversal would create only process artifacts."

        def anti_ceremony(_intent):
            calls.append("anti-ceremony")
            return {
                "decision": "STOP",
                "reason": reason,
                "frozen_outcome": "Ship the already-proved caller outcome",
                "parked_process_work": ["another plan", "another audit"],
                "remaining_proof": [],
                "stop_condition": "Stop before Plan.",
            }

        result = MODULE.invoke_once(
            "intent",
            anti_ceremony,
            lambda _intent: calls.append("plan"),
            lambda _plan: calls.append("implement"),
            lambda _plan, _candidate: calls.append("validate"),
        )

        self.assertEqual(calls, ["anti-ceremony"])
        self.assertEqual(result["status"], "NOT_PLANNED")
        self.assertEqual(
            result["checked"],
            [f"anti-ceremony guard: STOP — {reason}"],
        )
        self.assertEqual(result["not_checked"], ["plan", "implement", "validate"])

    def test_each_phase_runs_once_and_pass_reports(self):
        calls, plan, implement, validate = self.phases()
        result = MODULE.invoke_once("intent", continue_guard, plan, implement, validate)
        self.assertEqual(calls, ["plan", "implement", "validate"])
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(result["intent_ref"], "bead:agentops-test")
        self.assertEqual(result["acceptance_digest"], INTENT_DIGEST)
        self.assertNotIn("next_action", result)

    def test_fail_from_one_experiment_feeds_the_repair_phase(self):
        """Replaces the old stop-on-FAIL test (ADR-0017).

        One experiment still dispatches Plan and Implement exactly once, and the
        FAIL it produces is no longer terminal by itself: it is the first round
        handed to the bounded repair phase, which owns the stop decision.
        """
        calls, plan, implement, validate = self.phases("FAIL")
        result = MODULE.invoke_once("intent", continue_guard, plan, implement, validate)
        self.assertEqual(calls, ["plan", "implement", "validate"])
        self.assertEqual(result["status"], "FAIL")

        outcome = MODULE.run_repair_phase(
            [
                validation_round("FAIL", ["f1"], digest="a" * 64),
                validation_round("PASS", [], digest="d" * 64),
            ],
            repair_rounds=2,
            intent_ref=result["intent_ref"],
            acceptance_digest=result["acceptance_digest"],
        )
        self.assertEqual(outcome["stop_reason"], "converged")
        self.assertEqual(outcome["report"]["status"], "PASS")
        self.assertEqual(outcome["rounds_used"], 1)

    def test_fresh_validation_does_not_require_persisted_verdict(self):
        calls, plan, implement, validate = self.phases()

        def inline_result(resolved, subject):
            result = validate(resolved, subject)
            result.pop("verdict_digest")
            result.pop("verdict_ref")
            return result

        result = MODULE.invoke_once("intent", continue_guard, plan, implement, inline_result)

        self.assertEqual(calls, ["plan", "implement", "validate"])
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(result["subject_manifest_digest"], "a" * 64)
        self.assertIsNone(result["verdict_ref"])
        self.assertIsNone(result["verdict_digest"])

    def test_fresh_validation_requires_distinct_contexts_and_attestation(self):
        _calls, plan, implement, validate = self.phases()

        for field, value in (
            ("validator_context_id", "author-ctx"),
            ("freshness_attestation", None),
        ):
            with self.subTest(field=field):
                def invalid(resolved, subject, field=field, value=value):
                    result = validate(resolved, subject)
                    result[field] = value
                    return result

                with self.assertRaisesRegex(ValueError, "distinct context identities"):
                    MODULE.invoke_once("intent", continue_guard, plan, implement, invalid)

    def test_missing_plan_stops_before_implement(self):
        calls: list[str] = []
        result = MODULE.invoke_once(
            "intent",
            continue_guard,
            lambda _intent: None,
            lambda _plan: calls.append("implement"),
            lambda _plan, _candidate: calls.append("validate"),
        )
        self.assertEqual(calls, [])
        self.assertEqual(result["status"], "NOT_PLANNED")

    def test_missing_candidate_stops_before_validate(self):
        calls: list[str] = []
        result = MODULE.invoke_once(
            "intent",
            continue_guard,
            lambda _intent: {
                "intent_ref": "caller",
                "acceptance": ["works"],
                "acceptance_digest": INTENT_DIGEST,
                "intent_snapshot_bytes": INTENT_SNAPSHOT_BYTES,
            },
            lambda _plan: None,
            lambda _plan, _candidate: calls.append("validate"),
        )
        self.assertEqual(calls, [])
        self.assertEqual(result["status"], "NOT_BUILT")

    def test_validate_cannot_report_a_different_intent(self):
        calls, plan, implement, validate = self.phases()

        def mismatched(resolved, subject):
            result = validate(resolved, subject)
            result["acceptance_digest"] = "f" * 64
            return result

        with self.assertRaisesRegex(ValueError, "resolved intent digest"):
            MODULE.invoke_once("intent", continue_guard, plan, implement, mismatched)

    def test_plan_without_a_declared_digest_is_a_contract_error(self):
        _calls, _plan, implement, validate = self.phases()

        def undeclared(_intent):
            return {"intent_ref": "caller", "acceptance": ["works"]}

        with self.assertRaisesRegex(ValueError, "acceptance_digest"):
            MODULE.invoke_once("intent", continue_guard, undeclared, implement, validate)

    def test_plan_digest_must_be_a_sha256(self):
        _calls, _plan, implement, validate = self.phases()

        for bogus in ("", "not-a-digest", "C" * 64, "a" * 63, 12345):
            with self.subTest(digest=bogus):
                def undeclared(_intent, value=bogus):
                    return {
                        "intent_ref": "caller",
                        "acceptance": ["works"],
                        "acceptance_digest": value,
                        "intent_snapshot_bytes": INTENT_SNAPSHOT_BYTES,
                    }

                with self.assertRaisesRegex(ValueError, "acceptance_digest"):
                    MODULE.invoke_once("intent", continue_guard, undeclared, implement, validate)


class RepairPhaseTests(unittest.TestCase):
    """The bounded repair phase and its convergence law (ADR-0017).

    RPI is no longer single-pass: a `FAIL` or `NOT_PROVEN` with findings may be
    repaired and re-validated while all four law conditions hold. The loop is
    modelled here as pure data — a sequence of already-produced validate rounds
    — so the stop semantics are executable without Git, `ao`, or a tracker.
    """

    def repair(self, rounds, **kwargs):
        kwargs.setdefault("intent_ref", "bead:agentops-test")
        kwargs.setdefault("acceptance_digest", INTENT_DIGEST)
        return MODULE.run_repair_phase(rounds, **kwargs)

    def test_repair_rounds_zero_with_findings_is_budget_exhausted(self):
        outcome = self.repair([validation_round("FAIL", ["f1"])], repair_rounds=0)
        self.assertEqual(outcome["stop_reason"], "repair_budget_exhausted")
        self.assertEqual(outcome["rounds_used"], 0)
        self.assertEqual(outcome["report"]["status"], "FAIL")

    def test_a_pass_over_unchanged_bytes_after_a_fail_is_a_flip_not_a_proof(self):
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1"], digest="a" * 64),
                validation_round("PASS", [], digest="a" * 64),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "no_subject_or_evidence_change")
        self.assertEqual(outcome["report"]["status"], "NOT_PROVEN")

    def test_new_evidence_must_resolve_a_prior_finding_to_admit_an_unchanged_digest(self):
        outcome = self.repair(
            [
                validation_round("NOT_PROVEN", ["gap"], digest="a" * 64),
                validation_round(
                    "NOT_PROVEN", ["gap"], digest="a" * 64,
                    evidence=({"ref": "receipt-2", "subject_digest": "a" * 64, "resolves": ["gap"]},),
                ),
            ]
        )
        # "resolves" claims gap, but gap is still open: nothing was resolved.
        self.assertEqual(outcome["stop_reason"], "no_subject_or_evidence_change")

    def test_a_bare_new_evidence_label_does_not_admit_an_unchanged_digest(self):
        outcome = self.repair(
            [
                validation_round("NOT_PROVEN", ["gap", "other"], digest="a" * 64),
                validation_round("NOT_PROVEN", ["other"], digest="a" * 64, evidence=("receipt-2",)),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "no_subject_or_evidence_change")

    def test_evidence_bound_to_another_digest_does_not_admit(self):
        outcome = self.repair(
            [
                validation_round("NOT_PROVEN", ["gap", "other"], digest="a" * 64),
                validation_round(
                    "NOT_PROVEN", ["other"], digest="a" * 64,
                    evidence=({"ref": "receipt-2", "subject_digest": "b" * 64, "resolves": ["gap"]},),
                ),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "no_subject_or_evidence_change")

    def test_missing_findings_or_evidence_keys_are_rejected(self):
        for key in ("findings", "evidence_refs"):
            with self.subTest(missing=key):
                bad = validation_round("PASS", [])
                del bad[key]
                with self.assertRaisesRegex(ValueError, key):
                    self.repair([bad])
        bad = validation_round("PASS", [])
        bad["checked"] = "acceptance"
        with self.assertRaisesRegex(ValueError, "checked must be a list"):
            self.repair([bad])

    def test_scalar_evidence_refs_are_rejected(self):
        bad = validation_round("FAIL", ["f1"])
        bad["evidence_refs"] = "receipt-1"
        with self.assertRaisesRegex(ValueError, "evidence_refs must be a list"):
            self.repair([bad])

    def test_new_evidence_does_not_admit_a_current_fail_over_unchanged_bytes(self):
        outcome = self.repair(
            [
                validation_round("NOT_PROVEN", ["gap", "bug"], digest="a" * 64),
                validation_round("FAIL", ["bug"], digest="a" * 64, evidence=("receipt-2",)),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "no_subject_or_evidence_change")
        self.assertEqual(outcome["report"]["status"], "FAIL")

    def test_malformed_rounds_are_rejected_not_swallowed(self):
        cases = {
            "pass with findings": validation_round("PASS", ["f1"]),
            "fail without findings": validation_round("FAIL", []),
            "missing digest": validation_round("FAIL", ["f1"], digest=None),
        }
        for name, bad in cases.items():
            with self.subTest(case=name):
                with self.assertRaises(ValueError):
                    self.repair([bad])
        duplicate = validation_round("FAIL", ["f1"])
        duplicate["findings"].append({"id": "f1", "summary": "again"})
        with self.assertRaisesRegex(ValueError, "twice"):
            self.repair([duplicate])

    def test_rounds_past_the_bound_are_never_normalized(self):
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1"], digest="a" * 64),
                validation_round("FAIL", ["f1"], digest="b" * 64),
                {"status": "garbage-that-would-raise"},
            ],
            repair_rounds=1,
        )
        self.assertEqual(outcome["stop_reason"], "repair_budget_exhausted")

    def test_a_first_round_pass_converges_without_spending_a_repair_round(self):
        outcome = self.repair([validation_round("PASS", [])])
        self.assertEqual(outcome["stop_reason"], "converged")
        self.assertEqual(outcome["report"]["status"], "PASS")
        self.assertEqual(outcome["rounds_used"], 0)
        self.assertEqual(outcome["open_findings"], [])
        self.assertEqual(
            outcome["report"]["checked"][0], "repair round 0: 0 open findings"
        )

    def test_repair_stops_at_the_declared_repair_rounds_budget(self):
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1"], digest="a" * 64),
                validation_round("FAIL", ["f1"], digest="b" * 64),
                validation_round("FAIL", ["f1"], digest="c" * 64),
            ],
            repair_rounds=1,
        )
        self.assertEqual(outcome["stop_reason"], "repair_budget_exhausted")
        self.assertEqual(outcome["rounds_used"], 1)
        self.assertEqual(outcome["report"]["status"], "FAIL")
        self.assertEqual(
            outcome["report"]["checked"][:2],
            ["repair round 0: 1 open findings", "repair round 1: 1 open findings"],
        )

    def test_repair_stops_when_the_open_finding_set_grows(self):
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1"], digest="a" * 64),
                validation_round("FAIL", ["f1", "f2"], digest="b" * 64),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "finding_set_grew")
        self.assertEqual(outcome["report"]["status"], "FAIL")
        self.assertEqual(outcome["rounds_used"], 1)
        self.assertEqual(
            sorted(f["id"] for f in outcome["open_findings"]), ["f1", "f2"]
        )

    def test_repair_stops_when_a_closed_finding_reopens(self):
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1", "f2"], digest="a" * 64),
                validation_round("FAIL", ["f1"], digest="b" * 64),
                validation_round("FAIL", ["f2"], digest="c" * 64),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "reopened_finding")
        self.assertEqual(outcome["rounds_used"], 2)
        self.assertEqual(outcome["report"]["status"], "FAIL")

    def test_a_new_id_reusing_a_closed_class_stops_the_loop(self):
        """The CLASS is the law's second key (the 2026-09-03 run).

        Three rounds running, the open set never grew and no id reopened, yet
        every round named a fresh id for the same recorded-but-unenforced
        property. Counting ids alone cannot see that; the class can.
        """
        outcome = self.repair(
            [
                validation_round(
                    "FAIL", ["f1"], digest="a" * 64, classes={"f1": "seal.pinning"}
                ),
                validation_round("FAIL", ["f2"], digest="b" * 64),
                validation_round(
                    "FAIL", ["f3"], digest="c" * 64, classes={"f3": "seal.pinning"}
                ),
            ],
            repair_rounds=3,
        )
        self.assertEqual(outcome["stop_reason"], "class_reopened")
        self.assertEqual(outcome["rounds_used"], 2)
        self.assertEqual(outcome["report"]["status"], "NOT_PROVEN")
        self.assertEqual([f["id"] for f in outcome["open_findings"]], ["f3"])

    def test_findings_without_a_class_never_trigger_the_class_law(self):
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1"], digest="a" * 64),
                validation_round("FAIL", ["f2"], digest="b" * 64),
                validation_round("FAIL", ["f3"], digest="c" * 64),
            ],
            repair_rounds=3,
        )
        self.assertEqual(outcome["stop_reason"], "not_converged")
        self.assertEqual(outcome["rounds_used"], 2)
        self.assertEqual(outcome["report"]["status"], "FAIL")

    def test_a_different_class_is_not_a_class_reopen(self):
        outcome = self.repair(
            [
                validation_round(
                    "FAIL", ["f1"], digest="a" * 64, classes={"f1": "seal.pinning"}
                ),
                validation_round("FAIL", ["f2"], digest="b" * 64),
                validation_round(
                    "FAIL", ["f3"], digest="c" * 64, classes={"f3": "scope.coverage"}
                ),
            ],
            repair_rounds=3,
        )
        self.assertEqual(outcome["stop_reason"], "not_converged")
        self.assertEqual(outcome["rounds_used"], 2)

    def test_a_surviving_id_keeps_its_class_without_reopening_it(self):
        """A class is closed only when every id carrying it closed."""
        outcome = self.repair(
            [
                validation_round(
                    "FAIL",
                    ["f1", "f2"],
                    digest="a" * 64,
                    classes={"f1": "seal.pinning", "f2": "seal.pinning"},
                ),
                validation_round(
                    "FAIL", ["f2"], digest="b" * 64, classes={"f2": "seal.pinning"}
                ),
                validation_round(
                    "FAIL", ["f2"], digest="c" * 64, classes={"f2": "seal.pinning"}
                ),
            ],
            repair_rounds=3,
        )
        self.assertEqual(outcome["stop_reason"], "not_converged")
        self.assertEqual(outcome["rounds_used"], 2)

    def test_a_reopened_id_is_diagnosed_before_its_class(self):
        outcome = self.repair(
            [
                validation_round(
                    "FAIL",
                    ["f1", "f2"],
                    digest="a" * 64,
                    classes={"f1": "seal.pinning", "f2": "other"},
                ),
                validation_round(
                    "FAIL", ["f2"], digest="b" * 64, classes={"f2": "other"}
                ),
                validation_round(
                    "FAIL", ["f1"], digest="c" * 64, classes={"f1": "seal.pinning"}
                ),
            ],
            repair_rounds=3,
        )
        self.assertEqual(outcome["stop_reason"], "reopened_finding")

    def test_a_class_reopen_is_diagnosed_before_a_grown_set(self):
        outcome = self.repair(
            [
                validation_round(
                    "FAIL", ["f1"], digest="a" * 64, classes={"f1": "seal.pinning"}
                ),
                validation_round("FAIL", ["f2"], digest="b" * 64),
                validation_round(
                    "FAIL",
                    ["f3", "f4"],
                    digest="c" * 64,
                    classes={"f3": "seal.pinning"},
                ),
            ],
            repair_rounds=3,
        )
        self.assertEqual(outcome["stop_reason"], "class_reopened")

    def test_a_malformed_class_is_rejected_not_swallowed(self):
        for bogus in ("", "   ", 7, []):
            with self.subTest(value=bogus):
                bad = validation_round("FAIL", ["f1"])
                bad["findings"][0]["class"] = bogus
                with self.assertRaisesRegex(ValueError, "class"):
                    self.repair([bad])

    def test_the_open_findings_carry_their_class_through(self):
        outcome = self.repair(
            [validation_round("FAIL", ["f1"], classes={"f1": "seal.pinning"})],
            repair_rounds=0,
        )
        self.assertEqual(outcome["open_findings"][0]["class"], "seal.pinning")

    def test_repair_stops_when_the_digest_is_unchanged_and_no_new_evidence(self):
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1"], digest="a" * 64, evidence=["r1"]),
                validation_round("FAIL", ["f1"], digest="a" * 64, evidence=["r1"]),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "no_subject_or_evidence_change")
        self.assertEqual(outcome["rounds_used"], 1)

    def test_not_proven_is_resolved_by_new_evidence_with_an_unchanged_digest(self):
        outcome = self.repair(
            [
                validation_round(
                    "NOT_PROVEN", ["gap1"], digest="a" * 64, evidence=["r1"]
                ),
                validation_round(
                    "PASS", [], digest="a" * 64,
                    evidence=["r1", {"ref": "r2", "subject_digest": "a" * 64, "resolves": ["gap1"]}],
                ),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "converged")
        self.assertEqual(outcome["report"]["status"], "PASS")
        self.assertEqual(outcome["rounds_used"], 1)
        self.assertEqual(outcome["report"]["subject_manifest_digest"], "a" * 64)

    def test_new_evidence_does_not_rescue_a_fail_round(self):
        """Condition 4's evidence branch is NOT_PROVEN-only.

        A FAIL means the subject is wrong, so only a moved subject digest is
        progress; extra evidence over the same bytes is not.
        """
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1"], digest="a" * 64, evidence=["r1"]),
                validation_round("FAIL", ["f1"], digest="a" * 64, evidence=["r1", "r2"]),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "no_subject_or_evidence_change")

    def test_a_reworded_summary_with_the_same_id_is_the_same_finding(self):
        outcome = self.repair(
            [
                validation_round(
                    "FAIL", ["f1"], digest="a" * 64, summaries={"f1": "gate fails"}
                ),
                validation_round(
                    "FAIL",
                    ["f1"],
                    digest="b" * 64,
                    summaries={"f1": "the deterministic gate still rejects the tree"},
                ),
            ]
        )
        self.assertEqual(outcome["rounds_used"], 1)
        self.assertEqual(outcome["stop_reason"], "not_converged")
        self.assertEqual([f["id"] for f in outcome["open_findings"]], ["f1"])
        self.assertEqual(
            outcome["open_findings"][0]["summary"],
            "the deterministic gate still rejects the tree",
        )

    def test_open_findings_are_the_union_of_fresh_and_cross_family_ids(self):
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1", "f2", "f3"], digest="a" * 64),
                [
                    validation_round("FAIL", ["f1"], digest="b" * 64, family="fresh"),
                    validation_round("FAIL", ["f2"], digest="b" * 64, family="codex"),
                ],
            ]
        )
        self.assertEqual(outcome["rounds_used"], 1)
        self.assertEqual(sorted(f["id"] for f in outcome["open_findings"]), ["f1", "f2"])
        self.assertEqual(
            outcome["report"]["checked"][1], "repair round 1: 2 open findings"
        )

    def test_a_generated_only_change_that_moves_the_digest_counts_as_a_round(self):
        outcome = self.repair(
            [
                validation_round("FAIL", ["f1"], digest="a" * 64),
                validation_round("PASS", [], digest="b" * 64),
            ]
        )
        self.assertEqual(outcome["stop_reason"], "converged")
        self.assertEqual(outcome["rounds_used"], 1)
        self.assertEqual(
            outcome["report"]["checked"][:2],
            ["repair round 0: 1 open findings", "repair round 1: 0 open findings"],
        )

    def test_open_findings_never_land_in_not_checked(self):
        outcome = self.repair(
            [
                validation_round(
                    "FAIL",
                    ["f1"],
                    digest="a" * 64,
                    not_checked=["edge case acceptance"],
                )
            ],
            repair_rounds=0,
        )
        self.assertEqual(outcome["report"]["not_checked"], ["edge case acceptance"])
        self.assertEqual([f["id"] for f in outcome["open_findings"]], ["f1"])
        self.assertNotIn("finding f1", outcome["report"]["not_checked"])

    def test_a_risky_surface_needs_a_second_validator_family_to_converge(self):
        single = self.repair(
            [validation_round("PASS", [], family="fresh")], risky_surface=True
        )
        self.assertEqual(single["stop_reason"], "diversity_unsatisfied")
        self.assertEqual(single["report"]["status"], "NOT_PROVEN")

        crossed = self.repair(
            [
                [
                    validation_round("PASS", [], family="fresh"),
                    validation_round("PASS", [], family="codex"),
                ]
            ],
            risky_surface=True,
        )
        self.assertEqual(crossed["stop_reason"], "converged")
        self.assertEqual(crossed["report"]["status"], "PASS")

    def test_the_report_keeps_the_nine_key_rpi_report_shape(self):
        outcome = self.repair([validation_round("PASS", [])])
        self.assertEqual(
            sorted(outcome["report"]),
            sorted(
                [
                    "schema_version",
                    "status",
                    "intent_ref",
                    "acceptance_digest",
                    "subject_manifest_digest",
                    "verdict_ref",
                    "verdict_digest",
                    "checked",
                    "not_checked",
                ]
            ),
        )
        self.assertEqual(outcome["report"]["schema_version"], "rpi-report.v1")

    def test_the_repair_phase_needs_at_least_one_validation_round(self):
        with self.assertRaisesRegex(ValueError, "at least one validation round"):
            self.repair([])


class ComposedIdentityContractTests(unittest.TestCase):
    """RPI against the REAL Validate identity functions.

    This is the test the defect needed. RPI used to digest a canonical-JSON
    re-serialization of the parsed intent mapping while Validate digested the raw
    intent bytes, and RPI hard-compared the two. Nothing caught it because RPI's
    own suite mocked Validate with RPI's digest function, so the mock agreed with
    the code under test by construction. Here the digest crosses the skill
    boundary in both directions with no shared helper.
    """

    # Deliberately NOT canonical JSON: real intent sources have indentation,
    # trailing newlines, and key order. A canonical-JSON digest of the parsed
    # mapping differs from sha256(these bytes), so this payload discriminates
    # between the two implementations instead of accidentally agreeing.
    INTENT_BYTES = b'{\n  "acceptance": ["works"],\n  "intent_ref": "bead:agentops-test"\n}\n'

    def test_rpi_carries_the_digest_validate_derives_from_the_snapshot_bytes(self):
        with tempfile.TemporaryDirectory() as tmp:
            intent_dir = Path(tmp) / "intents"

            def plan(_intent):
                # Plan resolves the intent and snapshots the EXACT bytes through
                # Validate's own store, which is what defines the identity.
                path, _existed = VALIDATE.snapshot_intent(self.INTENT_BYTES, intent_dir)
                return {
                    "intent_ref": str(path),
                    "acceptance": ["works"],
                    "acceptance_digest": hashlib.sha256(self.INTENT_BYTES).hexdigest(),
                    # RPI re-derives the digest from the snapshot ON DISK before
                    # it dispatches Implement, so the binding is earned here too.
                    "verify_snapshot": lambda: Path(path).read_bytes(),
                }

            def implement(_plan):
                return {"subject_manifest_digest": "a" * 64}

            def validate(resolved, _candidate):
                # Validate re-reads the snapshot from disk and re-derives the
                # digest through its own runtime-fact binder — no value is passed
                # through from Plan, so agreement is earned, not assumed.
                replayed = Path(resolved["intent_ref"]).read_bytes()
                bound = VALIDATE.bind_runtime_facts(
                    {"verdict": "PASS"},
                    replayed,
                    None,
                    None,
                    None,
                    None,
                    None,
                    None,
                )
                return {
                    "verdict": "PASS",
                    "acceptance_digest": bound["acceptance_digest"],
                    "subject_manifest_digest": "a" * 64,
                    "author_context_id": "author-ctx",
                    "validator_context_id": "validator-ctx",
                    "freshness_attestation": {
                        "source": "runtime",
                        "attester_identity": "runtime:composed-test",
                    },
                    "verdict_digest": "b" * 64,
                    "verdict_ref": str(Path(tmp) / "verdict.json"),
                    "checked": ["acceptance"],
                    "not_checked": [],
                }

            result = MODULE.invoke_once(
                self.INTENT_BYTES,
                continue_guard,
                plan,
                implement,
                validate,
            )

        self.assertEqual(result["status"], "PASS")
        self.assertEqual(
            result["acceptance_digest"],
            hashlib.sha256(self.INTENT_BYTES).hexdigest(),
        )

    def test_the_canonical_json_digest_is_not_the_intent_identity(self):
        """Pins the two digests apart so the defect cannot silently return.

        If someone reintroduces a canonical-JSON digest of the parsed mapping as
        the acceptance identity, this fails: the byte digest and the value digest
        are different numbers for the same intent.
        """
        import json

        byte_digest = hashlib.sha256(self.INTENT_BYTES).hexdigest()
        value_digest = VALIDATE.digest_value(json.loads(self.INTENT_BYTES))
        self.assertNotEqual(byte_digest, value_digest)

        # And the collision the byte digest forbids: two distinct sources that
        # parse to the same mapping must NOT share an acceptance identity.
        reordered = b'{"intent_ref": "bead:agentops-test", "acceptance": ["works"]}'
        self.assertEqual(json.loads(reordered), json.loads(self.INTENT_BYTES))
        self.assertNotEqual(byte_digest, hashlib.sha256(reordered).hexdigest())
        self.assertEqual(value_digest, VALIDATE.digest_value(json.loads(reordered)))



class IntentSnapshotBindingTests(unittest.TestCase):
    """A digest nobody re-derived is the author's word, not an identity.

    The re-derivation is REQUIRED. When it was optional, a Plan that simply
    omitted the field got everything a verified one got, which made the whole
    identity chain optional with it.
    """

    def guard(self, _intent):
        return {
            "decision": "CONTINUE",
            "reason": "The work is real.",
            "frozen_outcome": "one behavior",
            "parked_process_work": [],
            "remaining_proof": [],
            "stop_condition": "converged",
        }

    def run_plan(self, plan_output):
        """Dispatch with a recording Implement; returns the stages that ran."""
        calls = []

        def implement(_resolved):
            calls.append("implement")
            return None

        MODULE.invoke_once(
            "intent", self.guard, lambda _i: plan_output, implement, lambda *_: None
        )
        return calls

    def refuse(self, plan_output, pattern):
        calls = []

        def implement(_resolved):
            calls.append("implement")
            return None

        with self.assertRaisesRegex(ValueError, pattern):
            MODULE.invoke_once(
                "intent", self.guard, lambda _i: plan_output, implement, lambda *_: None
            )
        self.assertEqual(calls, [], "Implement ran on an unbound intent identity")

    def base_plan(self, **overrides):
        plan = {"intent_ref": "bead:agentops-test", "acceptance_digest": INTENT_DIGEST}
        plan.update(overrides)
        return plan

    def test_a_plan_that_returns_no_snapshot_is_refused_before_implement(self):
        self.refuse(self.base_plan(), "must return the intent snapshot")

    def test_snapshot_bytes_that_hash_to_the_declared_digest_dispatch(self):
        self.assertEqual(
            self.run_plan(self.base_plan(intent_snapshot_bytes=INTENT_SNAPSHOT_BYTES)),
            ["implement"],
        )

    def test_snapshot_bytes_that_hash_to_anything_else_are_refused(self):
        self.refuse(
            self.base_plan(intent_snapshot_bytes=INTENT_SNAPSHOT_BYTES + b" tampered"),
            "does not hash to the acceptance digest",
        )

    def test_a_verifier_returning_the_bytes_is_hashed_here(self):
        self.assertEqual(
            self.run_plan(self.base_plan(verify_snapshot=lambda: INTENT_SNAPSHOT_BYTES)),
            ["implement"],
        )

    def test_a_verifier_returning_its_own_rederived_digest_binds(self):
        self.assertEqual(
            self.run_plan(self.base_plan(verify_snapshot=lambda: INTENT_DIGEST)),
            ["implement"],
        )

    def test_a_verifier_returning_a_disagreeing_digest_is_refused(self):
        self.refuse(
            self.base_plan(verify_snapshot=lambda: "d" * 64),
            "does not hash to the acceptance digest",
        )

    def test_a_malformed_rederived_digest_is_refused(self):
        for bogus in ("", "aaaa", "A" * 64):
            with self.subTest(value=bogus):
                self.refuse(
                    self.base_plan(verify_snapshot=lambda v=bogus: v),
                    "lowercase hex SHA-256",
                )

    def test_a_snapshot_of_the_wrong_type_is_refused(self):
        for bogus in (7, [], {}, None):
            with self.subTest(value=bogus):
                self.refuse(
                    self.base_plan(verify_snapshot=lambda v=bogus: v),
                    "must be bytes, or a lowercase hex SHA-256",
                )

    def test_a_non_callable_verifier_is_refused(self):
        self.refuse(self.base_plan(verify_snapshot="not callable"), "must be callable")

    def test_declaring_both_forms_is_refused(self):
        self.refuse(
            self.base_plan(
                intent_snapshot_bytes=INTENT_SNAPSHOT_BYTES,
                verify_snapshot=lambda: INTENT_SNAPSHOT_BYTES,
            ),
            "exactly one of",
        )

    def test_the_digest_is_never_borrowed_from_the_module_under_test(self):
        """The fixture digest is the test's own, over the fixture's own bytes."""
        self.assertEqual(
            INTENT_DIGEST, hashlib.sha256(INTENT_SNAPSHOT_BYTES).hexdigest()
        )
        self.assertNotEqual(INTENT_DIGEST, "c" * 64)


class SharedConvergenceCorpusTests(unittest.TestCase):
    """Every case in the shared corpus, run through the Python law."""

    def test_the_corpus_is_nonempty_and_versioned(self):
        self.assertEqual(LAW_CASES["schema"], "rpi-convergence-law-cases.v1")
        self.assertGreaterEqual(len(LAW_CASES["cases"]), 7)

    def test_every_shared_case_matches_the_reference_behavior(self):
        for case in LAW_CASES["cases"]:
            with self.subTest(case=case["name"]):
                rounds = [case_round(r) for r in case["rounds"]]
                expect = case["expect"]
                if expect["kind"] == "invalid":
                    with self.assertRaises(ValueError) as caught:
                        MODULE.run_repair_phase(
                            rounds,
                            repair_rounds=case["repair_rounds"],
                            intent_ref="bead:agentops-test",
                            acceptance_digest=INTENT_DIGEST,
                        )
                    self.assertIn(expect["message"], str(caught.exception))
                    continue
                outcome = MODULE.run_repair_phase(
                    rounds,
                    repair_rounds=case["repair_rounds"],
                    intent_ref="bead:agentops-test",
                    acceptance_digest=INTENT_DIGEST,
                )
                self.assertEqual(outcome["stop_reason"], expect["stop_reason"])
                self.assertEqual(outcome["stop_reasons"], expect["stop_reasons"])
                self.assertEqual(outcome["report"]["status"], expect["status"])
                self.assertEqual(outcome["rounds_used"], expect["rounds_used"])

    def test_both_dispositions_are_named_when_a_round_carries_both(self):
        """Precedence orders the diagnosis; it never deletes the other one."""
        outcome = self.subject_of("simultaneous-id-and-class-reopen")
        self.assertEqual(outcome["stop_reason"], "reopened_finding")
        self.assertIn("class_reopened", outcome["stop_reasons"])
        self.assertTrue(
            any(
                "class_reopened" in line and "reopened_finding" in line
                for line in outcome["report"]["checked"]
            ),
            outcome["report"]["checked"],
        )

    def subject_of(self, name):
        case = next(c for c in LAW_CASES["cases"] if c["name"] == name)
        return MODULE.run_repair_phase(
            [case_round(r) for r in case["rounds"]],
            repair_rounds=case["repair_rounds"],
            intent_ref="bead:agentops-test",
            acceptance_digest=INTENT_DIGEST,
        )


if __name__ == "__main__":
    unittest.main()


class SnapshotShapeTests(unittest.TestCase):
    def test_digest_string_as_snapshot_bytes_is_refused(self):
        with self.assertRaises(ValueError):
            MODULE.verify_intent_snapshot(
                {"intent_snapshot_bytes": "a" * 64}, "a" * 64
            )

    def test_digest_pattern_rejects_trailing_newline(self):
        self.assertFalse(MODULE.valid_digest("a" * 64 + "\n"))
        self.assertTrue(MODULE.valid_digest("a" * 64))
