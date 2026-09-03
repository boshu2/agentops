#!/usr/bin/env python3
"""Pure reference behavior for one RPI invocation and its bounded repair phase.

The caller supplies one anti-ceremony guard and the three core phase functions.
This module invokes the guard once before Plan, dispatches Plan and Implement at
most once, and never chooses a retry, a budget, or a next action.

Under ADR-0017 (loop as control flow, not knowledge) the traversal no longer
ends at the first validation result. `run_repair_phase` models the bounded
repair phase as pure data: it consumes validate rounds that already happened and
decides, under the convergence law, whether another repair round is admitted.
It performs no I/O, dispatches nothing, and owns no budget of its own — the
caller declares `repair_rounds`.
"""

from __future__ import annotations

from collections.abc import Callable, Mapping, Sequence
import re
from typing import Any


# The exact-identity property is BYTE-addressed: Validate snapshots the resolved
# intent bytes under `sha256(bytes)` and stores them as `<digest>.intent`
# (validate.py snapshot_intent), then re-derives that same digest from the same
# bytes when it binds runtime facts into the verdict. RPI is a dispatcher, not a
# second digest authority — it carries the digest Plan declares over the bytes it
# snapshotted, and cross-checks Validate's independently re-derived value against
# it.
#
# This module previously computed its own `sha256(canonical-JSON(mapping))` here
# and hard-compared that against Validate's `sha256(raw bytes)`. The two can
# never agree unless the source is byte-identical canonical JSON, so the composed
# contract was broken; both unit suites stayed green only because the RPI test
# mocked Validate with THIS module's digest function. A canonical-JSON digest is
# also the wrong identity in principle: two different source files that parse to
# the same mapping share it, which is precisely the collision exact identity
# exists to forbid.
DIGEST_PATTERN = re.compile(r"^[0-9a-f]{64}$")


def valid_digest(value: Any) -> bool:
    """True for a lowercase hex SHA-256, the only shape an identity may take."""
    return isinstance(value, str) and bool(DIGEST_PATTERN.match(value))


def valid_string_list(value: Any) -> bool:
    """True for the guard contract's JSON-shaped string lists."""
    return isinstance(value, list) and all(
        isinstance(item, str) and bool(item.strip()) for item in value
    )


def guard_result(value: Any) -> dict[str, Any]:
    """Return one valid artifact-free anti-ceremony decision."""
    if not isinstance(value, Mapping):
        raise ValueError("anti-ceremony guard must return a mapping")
    result = dict(value)
    expected = {
        "decision",
        "reason",
        "frozen_outcome",
        "parked_process_work",
        "remaining_proof",
        "stop_condition",
    }
    if set(result) != expected:
        raise ValueError("anti-ceremony guard returned the wrong fields")
    if result["decision"] not in {"CONTINUE", "STOP"}:
        raise ValueError("anti-ceremony decision must be CONTINUE or STOP")
    reason = result["reason"]
    if (
        not isinstance(reason, str)
        or not reason.strip()
        or "\n" in reason
        or reason[-1] not in ".!?"
        or sum(reason.count(mark) for mark in ".!?") != 1
    ):
        raise ValueError("anti-ceremony reason must be exactly one sentence")
    if not isinstance(result["frozen_outcome"], str) or not result["frozen_outcome"].strip():
        raise ValueError("anti-ceremony frozen_outcome must be a nonempty string")
    if not valid_string_list(result["parked_process_work"]):
        raise ValueError("anti-ceremony parked_process_work must be a string list")
    if not valid_string_list(result["remaining_proof"]):
        raise ValueError("anti-ceremony remaining_proof must be a string list")
    if not isinstance(result["stop_condition"], str) or not result["stop_condition"].strip():
        raise ValueError("anti-ceremony stop_condition must be a nonempty string")
    return result


def report(
    status: str,
    *,
    intent_ref: str | None = None,
    acceptance_digest: str | None = None,
    subject_digest: str | None = None,
    verdict_ref: str | None = None,
    verdict_digest: str | None = None,
    checked: list[str] | None = None,
    not_checked: list[str] | None = None,
) -> dict[str, Any]:
    return {
        "schema_version": "rpi-report.v1",
        "status": status,
        "intent_ref": intent_ref,
        "acceptance_digest": acceptance_digest,
        "subject_manifest_digest": subject_digest,
        "verdict_ref": verdict_ref,
        "verdict_digest": verdict_digest,
        "checked": checked or [],
        "not_checked": not_checked or [],
    }


def invoke_once(
    intent: Any,
    anti_ceremony_guard: Callable[[Any], Mapping[str, Any]],
    plan_phase: Callable[[Any], Mapping[str, Any] | None],
    implement_phase: Callable[[Mapping[str, Any]], Mapping[str, Any] | None],
    validate_phase: Callable[[Mapping[str, Any], Mapping[str, Any]], Mapping[str, Any]],
) -> dict[str, Any]:
    """Invoke the guard once, then dispatch each core phase at most once."""
    admission = guard_result(anti_ceremony_guard(intent))
    if admission["decision"] == "STOP":
        return report(
            "NOT_PLANNED",
            checked=[f"anti-ceremony guard: STOP — {admission['reason']}"],
            not_checked=["plan", "implement", "validate"],
        )
    resolved_intent = plan_phase(intent)
    if resolved_intent is None:
        return report("NOT_PLANNED", not_checked=["implement", "validate"])
    resolved_intent = dict(resolved_intent)
    intent_ref = resolved_intent.get("intent_ref")
    if not isinstance(intent_ref, str) or not intent_ref:
        intent_ref = "caller"
    acceptance_digest = resolved_intent.get("acceptance_digest")
    if not valid_digest(acceptance_digest):
        raise ValueError(
            "Plan must declare acceptance_digest as the SHA-256 of the exact resolved "
            "intent bytes it snapshotted (validate.py snapshot-intent emits it)"
        )

    subject = implement_phase(resolved_intent)
    if subject is None:
        return report(
            "NOT_BUILT",
            intent_ref=intent_ref,
            acceptance_digest=acceptance_digest,
            checked=["plan"],
            not_checked=["validate"],
        )
    subject = dict(subject)

    validation = dict(validate_phase(resolved_intent, subject))
    status = validation.get("verdict")
    if status not in {"PASS", "FAIL", "NOT_PROVEN"}:
        raise ValueError("Validate must return PASS, FAIL, or NOT_PROVEN")
    # Validate re-derives this from the snapshot bytes independently; equality
    # here is the composed exact-identity check, not a self-comparison.
    if validation.get("acceptance_digest") != acceptance_digest:
        raise ValueError("Validate verdict does not match the resolved intent digest")
    subject_digest = validation.get("subject_manifest_digest")
    if not valid_digest(subject_digest):
        raise ValueError("Validate must return the exact subject manifest digest")
    candidate_digest = subject.get("subject_manifest_digest")
    if candidate_digest is not None and subject_digest != candidate_digest:
        raise ValueError("Validate result does not match the implemented subject digest")
    author_context_id = validation.get("author_context_id")
    validator_context_id = validation.get("validator_context_id")
    freshness = validation.get("freshness_attestation")
    if (
        not isinstance(author_context_id, str)
        or not author_context_id
        or not isinstance(validator_context_id, str)
        or not validator_context_id
        or author_context_id == validator_context_id
        or not isinstance(freshness, Mapping)
        or freshness.get("source") not in {"runtime", "caller"}
        or not isinstance(freshness.get("attester_identity"), str)
        or not freshness.get("attester_identity")
    ):
        raise ValueError("Validate must return distinct context identities and explicit freshness")
    verdict_digest = validation.get("verdict_digest")
    verdict_ref = validation.get("verdict_ref")
    if (verdict_digest is None) != (verdict_ref is None):
        raise ValueError("Validate must return both verdict_ref and verdict_digest when persistence is requested")
    if verdict_ref is not None and (
        not isinstance(verdict_ref, str)
        or not verdict_ref
        or not valid_digest(verdict_digest)
    ):
        raise ValueError("Persisted verdict identity is invalid")
    return report(
        status,
        intent_ref=intent_ref,
        acceptance_digest=acceptance_digest,
        subject_digest=subject_digest,
        verdict_ref=verdict_ref,
        verdict_digest=verdict_digest,
        checked=list(validation.get("checked") or []),
        not_checked=list(validation.get("not_checked") or []),
    )


# ---------------------------------------------------------------------------
# The bounded repair phase (ADR-0017)
# ---------------------------------------------------------------------------
#
# The 2026-07-14 cathedral cut removed the iterate loop together with the
# unproven compounding claim, although ADR-0011 demoted only the latter. What
# comes back is control flow, not knowledge: a repair round is admitted only
# while every condition of the convergence law holds, so the loop cannot grind,
# cannot re-open settled ground, and cannot spin without moving the subject.
#
# Condition ordering is deliberate. A reopened id is diagnosed before a grown
# set, so the operator is told the specific regression rather than the generic
# symptom; progress is checked last because it is the only condition that can
# be satisfied by evidence instead of by bytes.

REPAIR_ROUNDS_DEFAULT = 2

#: Terminal reasons `run_repair_phase` may report. `converged` is the only
#: success; the rest are law stops the caller owns the response to.
STOP_REASONS = (
    "converged",
    "diversity_unsatisfied",
    "repair_budget_exhausted",
    "reopened_finding",
    "finding_set_grew",
    "no_subject_or_evidence_change",
    "not_converged",
)

_STATUS_RANK = {"PASS": 0, "NOT_PROVEN": 1, "FAIL": 2}


def _leg_status(leg: Mapping[str, Any]) -> str:
    """Read a validate leg's semantic verdict under either spelling."""
    status = leg.get("status", leg.get("verdict"))
    if status not in _STATUS_RANK:
        raise ValueError("each validate result must report PASS, FAIL, or NOT_PROVEN")
    return str(status)


def normalize_round(value: Any) -> dict[str, Any]:
    """Fold one validation round's legs into the facts the law reasons over.

    A round is one or more validate results (the fresh validator, plus the
    cross-family validator when the diff touches a risky surface). Open findings
    are the UNION of the legs' stable `findings[].id`; the round's status is the
    worst leg's; the digest is the subject every leg judged.
    """
    legs: list[Mapping[str, Any]]
    if isinstance(value, Mapping):
        legs = [value]
    elif isinstance(value, Sequence) and not isinstance(value, (str, bytes)):
        legs = list(value)
    else:
        raise ValueError("a validation round must be a validate result or a list of them")
    if not legs:
        raise ValueError("a validation round must contain at least one validate result")

    open_findings: dict[str, dict[str, Any]] = {}
    families: list[str] = []
    evidence_refs: list[str] = []
    checked: list[str] = []
    not_checked: list[str] = []
    digest: Any = None
    status = "PASS"
    for leg in legs:
        if not isinstance(leg, Mapping):
            raise ValueError("each validate result must be a mapping")
        leg_status = _leg_status(leg)
        if _STATUS_RANK[leg_status] > _STATUS_RANK[status]:
            status = leg_status
        raw_findings = leg.get("findings")
        if raw_findings is None:
            raw_findings = []
        if not isinstance(raw_findings, (list, tuple)):
            raise ValueError("findings must be a list")
        leg_ids: set[str] = set()
        for finding in raw_findings:
            if not isinstance(finding, Mapping):
                raise ValueError("each finding must be a mapping")
            finding_id = finding.get("id")
            if not isinstance(finding_id, str) or not finding_id.strip():
                raise ValueError("each finding must carry a stable nonempty id")
            if finding_id in leg_ids:
                raise ValueError(f"finding id {finding_id!r} appears twice in one validate leg")
            leg_ids.add(finding_id)
            # Last leg wins on wording; the id is the identity, so a reworded
            # summary is the same finding and never counts as a new one.
            open_findings[finding_id] = dict(finding)
        if leg_status == "PASS" and leg_ids:
            raise ValueError("a PASS leg cannot carry open findings")
        if leg_status == "FAIL" and not leg_ids:
            raise ValueError("a FAIL leg must name at least one finding")
        family = leg.get("validator_family")
        if isinstance(family, str) and family and family not in families:
            families.append(family)
        for ref in leg.get("evidence_refs") or []:
            if ref not in evidence_refs:
                evidence_refs.append(ref)
        leg_digest = leg.get("subject_digest", leg.get("subject_manifest_digest"))
        if not valid_digest(leg_digest):
            raise ValueError("each validate leg must carry a valid subject digest")
        if digest is not None and leg_digest != digest:
            raise ValueError("validate legs disagree about the subject digest")
        digest = leg_digest
        checked.extend(str(item) for item in leg.get("checked") or [])
        not_checked.extend(str(item) for item in leg.get("not_checked") or [])

    return {
        "status": status,
        "open_findings": list(open_findings.values()),
        "open_ids": set(open_findings),
        "subject_digest": digest,
        "evidence_refs": evidence_refs,
        "families": families,
        "checked": checked,
        "not_checked": not_checked,
    }


def law_violation(
    previous: Mapping[str, Any],
    current: Mapping[str, Any],
    closed_ids: set[str],
) -> str | None:
    """Return the violated convergence-law condition, or None when all hold.

    Condition 1 (the caller's `repair_rounds`) is a precondition on admission
    and is checked by `run_repair_phase` before a round is consumed; conditions
    2, 3, and 4 are properties of the round that was produced.
    """
    reopened = current["open_ids"] & closed_ids
    if reopened:
        return "reopened_finding"
    if len(current["open_ids"]) > len(previous["open_ids"]):
        return "finding_set_grew"
    if current["subject_digest"] != previous["subject_digest"]:
        return None
    new_evidence = [
        ref for ref in current["evidence_refs"] if ref not in set(previous["evidence_refs"])
    ]
    # The evidence branch is NOT_PROVEN-only by construction, on both sides: a
    # FAIL says the subject is wrong, and no amount of new evidence over
    # unchanged bytes repairs a wrong subject. The new evidence must also have
    # RESOLVED something: at least one previously open finding id is closed.
    resolved = previous["open_ids"] - current["open_ids"]
    if (
        previous["status"] == "NOT_PROVEN"
        and current["status"] != "FAIL"
        and new_evidence
        and resolved
    ):
        return None
    return "no_subject_or_evidence_change"


def run_repair_phase(
    validations: Sequence[Any],
    *,
    repair_rounds: int = REPAIR_ROUNDS_DEFAULT,
    risky_surface: bool = False,
    intent_ref: str | None = None,
    acceptance_digest: str | None = None,
    verdict_ref: str | None = None,
    verdict_digest: str | None = None,
) -> dict[str, Any]:
    """Walk already-produced validation rounds under the convergence law.

    `validations[0]` is the traversal's first fresh validation; every later
    element is a repair round the orchestrator produced after fixing findings.

    Returns a mapping with:

    - ``report``: the exact nine-key `rpi-report.v1` object. `checked` opens
      with one `repair round N: k open findings` line per round; open findings
      never enter `not_checked`, which keeps its meaning (unverified in-scope
      acceptance).
    - ``open_findings``: the findings still open at the stop, deduplicated by id.
    - ``rounds_used``: repair rounds actually spent (the first validation is
      round 0 and spends none).
    - ``stop_reason``: one of :data:`STOP_REASONS`.
    """
    if not validations:
        raise ValueError("the repair phase needs at least one validation round")
    if not isinstance(repair_rounds, int) or isinstance(repair_rounds, bool) or repair_rounds < 0:
        raise ValueError("repair_rounds must be a non-negative integer")

    checked: list[str] = []
    closed_ids: set[str] = set()
    rounds_used = 0
    current = normalize_round(validations[0])
    previous = current
    stop_reason = "not_converged"
    law_stopped = False

    for index, raw_candidate in enumerate(validations):
        if index > 0:
            # Condition 1: the caller's bound, checked before the round is even
            # normalized, so a round past the bound is never consumed.
            if rounds_used >= repair_rounds:
                stop_reason = "repair_budget_exhausted"
                break
            candidate = normalize_round(raw_candidate)
            rounds_used += 1
            current = candidate
            checked.append(
                f"repair round {rounds_used}: {len(current['open_ids'])} open findings"
            )
            violation = law_violation(previous, current, closed_ids)
            if violation is not None:
                stop_reason = violation
                law_stopped = True
                break
            closed_ids |= previous["open_ids"] - current["open_ids"]
        else:
            checked.append(f"repair round 0: {len(current['open_ids'])} open findings")

        converged, reason = _converged(current, risky_surface)
        previous = current
        if converged:
            stop_reason = "converged"
            break
        if reason is not None:
            stop_reason = reason
            break
    else:
        stop_reason = "not_converged"

    if stop_reason == "not_converged" and rounds_used >= repair_rounds and current["open_ids"]:
        # Findings remain and the caller's bound is spent: name it as such.
        stop_reason = "repair_budget_exhausted"

    status = current["status"]
    if stop_reason == "diversity_unsatisfied" or (law_stopped and status == "PASS"):
        # A PASS produced by a law-violating round cannot certify anything: a
        # PASS over unchanged bytes after a FAIL is a flip, not a proof. A FAIL
        # that also broke the law stays a FAIL; the subject is still wrong.
        status = "NOT_PROVEN"

    return {
        "report": report(
            status,
            intent_ref=intent_ref,
            acceptance_digest=acceptance_digest,
            subject_digest=current["subject_digest"],
            verdict_ref=verdict_ref,
            verdict_digest=verdict_digest,
            checked=checked + current["checked"],
            not_checked=list(current["not_checked"]),
        ),
        "open_findings": list(current["open_findings"]),
        "rounds_used": rounds_used,
        "stop_reason": stop_reason,
    }


def _converged(current: Mapping[str, Any], risky_surface: bool) -> tuple[bool, str | None]:
    """Converged ⇔ fresh PASS, plus a cross-family PASS on a risky surface."""
    if current["status"] != "PASS":
        return False, None
    if risky_surface and len(current["families"]) < 2:
        # No authorized second family judged the risky surface, so same-family
        # agreement is not independence: NOT_PROVEN, never a convergence.
        return False, "diversity_unsatisfied"
    return True, None
