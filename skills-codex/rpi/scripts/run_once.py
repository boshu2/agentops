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
import hashlib
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
    return isinstance(value, str) and bool(DIGEST_PATTERN.fullmatch(value))


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


def verify_intent_snapshot(
    resolved_intent: Mapping[str, Any], acceptance_digest: str
) -> str:
    """Re-derive the intent snapshot digest and bind it, or refuse.

    Returns the digest it derived. Raises when the snapshot is absent,
    malformed, or hashes to anything other than the acceptance digest Plan
    declared. This module performs no I/O, so the caller supplies the bytes or
    a callable that produces them; what it may never supply is nothing at all.
    """
    payload = resolved_intent.get("intent_snapshot_bytes")
    verifier = resolved_intent.get("verify_snapshot")
    if payload is None and verifier is None:
        raise ValueError(
            "Plan must return the intent snapshot to be verified, as "
            "intent_snapshot_bytes or a verify_snapshot callable; a declared digest "
            "nobody re-derived is the author's word, not an identity"
        )
    if payload is not None and verifier is not None:
        raise ValueError(
            "Plan must return exactly one of intent_snapshot_bytes or verify_snapshot"
        )
    from_verifier = False
    if verifier is not None:
        if not callable(verifier):
            raise ValueError("verify_snapshot must be callable")
        payload = verifier()
        from_verifier = True
    if isinstance(payload, str):
        # Only a callable that re-derived the digest itself may report a digest;
        # a string handed over as intent_snapshot_bytes is a declaration, not bytes.
        if not from_verifier:
            raise ValueError(
                "intent_snapshot_bytes must be bytes; a digest string is the author's "
                "word, not a re-derivation"
            )
        derived = payload
    elif isinstance(payload, (bytes, bytearray)):
        derived = hashlib.sha256(bytes(payload)).hexdigest()
    else:
        raise ValueError(
            "the intent snapshot must be bytes, or a lowercase hex SHA-256 the "
            "verifier re-derived from them"
        )
    if not valid_digest(derived):
        raise ValueError(
            "the re-derived intent snapshot digest must be a lowercase hex SHA-256"
        )
    if derived != acceptance_digest:
        raise ValueError(
            "the intent snapshot does not hash to the acceptance digest Plan declared; "
            "nothing may be built on an intent identity that does not bind"
        )
    return derived


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
    # THE SNAPSHOT BINDING, and it is REQUIRED. A digest is a claim about bytes,
    # and a claim about bytes nobody re-derived is the author's word. Making the
    # re-derivation optional made the whole identity chain optional with it: a
    # Plan that simply omitted the field got everything a verified one got.
    #
    # Plan must therefore hand back the snapshot itself, one of:
    #   `intent_snapshot_bytes`  the exact bytes, hashed HERE
    #   `verify_snapshot`        a zero-argument callable returning those bytes,
    #                            or the lowercase hex SHA-256 it re-derived
    # Equality against the declared acceptance digest is the composed check, not
    # a self-comparison, and it is refused before Implement is dispatched.
    verify_intent_snapshot(resolved_intent, acceptance_digest)

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
# Condition ordering is deliberate. A reopened id is diagnosed before a
# reopened class, and a reopened class before a grown set, so the operator is
# told the specific regression rather than the generic symptom; progress is
# checked last because it is the only condition that can be satisfied by
# evidence instead of by bytes.

REPAIR_ROUNDS_DEFAULT = 2

#: Terminal reasons `run_repair_phase` may report. `converged` is the only
#: success; the rest are law stops the caller owns the response to.
STOP_REASONS = (
    "converged",
    "diversity_unsatisfied",
    "repair_budget_exhausted",
    "reopened_finding",
    "class_reopened",
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
        if "findings" not in leg:
            raise ValueError("each validate leg must carry a findings list (empty on PASS)")
        raw_findings = leg["findings"]
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
            # The CLASS is optional and stable: it names the KIND of defect
            # ("seal.pinning"), so a repair phase that keeps minting fresh ids
            # for the same kind is visible to the law. Absent means the class
            # conditions simply do not apply to this finding.
            if "class" in finding:
                finding_class = finding["class"]
                if not isinstance(finding_class, str) or not finding_class.strip():
                    raise ValueError("a finding class must be a nonempty string when given")
            # Last leg wins on WORDING; the id is the identity, so a reworded
            # summary is the same finding and never counts as a new one. The
            # CLASS is not wording. Two legs naming one id with different kinds
            # is an identity collision, and folding it last-write-wins hands the
            # law a key that means two things, so the class conditions then
            # reason over an identity that does not exist. Compatible identities
            # union: a leg that names no kind never erases one another leg named.
            previous = open_findings.get(finding_id)
            merged = dict(finding)
            if previous is not None:
                before = previous.get("class")
                after = merged.get("class")
                before = before.strip() if isinstance(before, str) else None
                after = after.strip() if isinstance(after, str) else None
                if before is not None and after is not None and before != after:
                    raise ValueError(
                        f"validate legs named finding {finding_id!r} with different classes "
                        f"({before!r} and {after!r}); one id cannot carry two kinds"
                    )
                if after is None and before is not None:
                    merged["class"] = before
            open_findings[finding_id] = merged
        if leg_status == "PASS" and leg_ids:
            raise ValueError("a PASS leg cannot carry open findings")
        if leg_status == "FAIL" and not leg_ids:
            raise ValueError("a FAIL leg must name at least one finding")
        family = leg.get("validator_family")
        if isinstance(family, str) and family and family not in families:
            families.append(family)
        if "evidence_refs" not in leg:
            raise ValueError("each validate leg must carry an evidence_refs list (empty if none)")
        raw_evidence = leg["evidence_refs"]
        if not isinstance(raw_evidence, (list, tuple)):
            raise ValueError("evidence_refs must be a list")
        for ref in raw_evidence:
            # Evidence is either a bare label (unbound; it can never admit an
            # unchanged digest) or a binding {ref, subject_digest, resolves}.
            if isinstance(ref, str):
                entry: dict[str, Any] = {"ref": ref}
            elif isinstance(ref, Mapping):
                if not isinstance(ref.get("ref"), str) or not ref["ref"].strip():
                    raise ValueError("each evidence binding must carry a nonempty ref")
                entry = dict(ref)
                resolves = entry.get("resolves")
                if resolves is not None and not valid_string_list(resolves):
                    raise ValueError("evidence.resolves must be a list of finding ids")
                if "subject_digest" in entry and not valid_digest(entry["subject_digest"]):
                    raise ValueError("evidence.subject_digest must be a valid digest")
            else:
                raise ValueError("each evidence ref must be a string or a binding mapping")
            if entry["ref"] not in {e["ref"] for e in evidence_refs}:
                evidence_refs.append(entry)
        leg_digest = leg.get("subject_digest", leg.get("subject_manifest_digest"))
        if not valid_digest(leg_digest):
            raise ValueError("each validate leg must carry a valid subject digest")
        if digest is not None and leg_digest != digest:
            raise ValueError("validate legs disagree about the subject digest")
        digest = leg_digest
        for key, sink in (("checked", checked), ("not_checked", not_checked)):
            items = leg.get(key, [])
            if not valid_string_list(items):
                raise ValueError(f"{key} must be a list of strings")
            sink.extend(items)

    open_classes = {
        finding_id: finding["class"].strip()
        for finding_id, finding in open_findings.items()
        if isinstance(finding.get("class"), str)
    }
    return {
        "status": status,
        "open_findings": list(open_findings.values()),
        "open_ids": set(open_findings),
        "open_classes": open_classes,
        "subject_digest": digest,
        "evidence_refs": evidence_refs,
        "families": families,
        "checked": checked,
        "not_checked": not_checked,
    }


def assert_stable_classes(
    previous: Mapping[str, Any], current: Mapping[str, Any]
) -> None:
    """Refuse a round that mutates the class of an id that CARRIED THROUGH.

    A class is a stable property of a finding, not a field a round may revise.
    Mutating it on a surviving id defeats the class law from the inside: in
    `f1[X] -> f1[Y] -> f2[X]` the id never resolves, so X looks retired with
    nothing having closed it, and the same kind reappears on a new id
    unremarked. Adding or removing a class on a surviving id is the same
    mutation wearing a different sign.

    This is an INVALID round, not a law violation: the law reasons over stable
    keys, and a round that moves the keys gives it nothing to reason with.
    """
    for finding_id in sorted(previous["open_ids"] & current["open_ids"]):
        before = previous["open_classes"].get(finding_id)
        after = current["open_classes"].get(finding_id)
        if before != after:
            raise ValueError(
                f"finding {finding_id!r} changed class from {before!r} to {after!r} "
                "while still open; a finding class is stable across rounds"
            )


def reopened_classes(
    previous: Mapping[str, Any],
    current: Mapping[str, Any],
    closed_classes: Mapping[str, str] | None = None,
) -> list[str]:
    """Return the classes this round reopened, sorted, empty when none.

    Condition 3b. Ids alone cannot see a repair phase that keeps renaming the
    same KIND of defect; that is the failure the 2026-09-03 run produced three
    rounds running while the id count stayed flat.

    SURVIVORS are `previous ∩ current` — the ids that actually carried through.
    A newly minted id is NOT a survivor, however familiar its class: treating it
    as one is precisely the hole that let a continuous rename f1[X] -> f2[X] ->
    f3[X] run forever, because each round's fresh id kept its own class looking
    "still open" and so never retired.

    A class reopens when a NEW id in this round carries either
    - a class an EARLIER round closed (`closed_classes`), or
    - a class carried by an id THIS round resolved, with no surviving prior id
      still carrying it.
    """
    closed_classes = closed_classes or {}
    survivors = previous["open_ids"] & current["open_ids"]
    resolved = previous["open_ids"] - current["open_ids"]
    appeared = current["open_ids"] - previous["open_ids"]
    surviving = {
        previous["open_classes"][finding_id]
        for finding_id in survivors
        if finding_id in previous["open_classes"]
    } | {
        current["open_classes"][finding_id]
        for finding_id in survivors
        if finding_id in current["open_classes"]
    }
    retired_here = {
        previous["open_classes"][finding_id]
        for finding_id in resolved
        if finding_id in previous["open_classes"]
    } - surviving
    reopened = {
        current["open_classes"][finding_id]
        for finding_id in appeared
        if finding_id in current["open_classes"]
        and (
            current["open_classes"][finding_id] in closed_classes
            or current["open_classes"][finding_id] in retired_here
        )
    }
    return sorted(reopened)


def law_violations(
    previous: Mapping[str, Any],
    current: Mapping[str, Any],
    closed_ids: set[str],
    closed_classes: Mapping[str, str] | None = None,
) -> list[str]:
    """Return every convergence-law condition this round violates, in order.

    Condition 1 (the caller's `repair_rounds`) is a precondition on admission
    and is checked by `run_repair_phase` before a round is consumed; conditions
    2, 3, 3b, and 4 are properties of the round that was produced.

    Ordering is the DIAGNOSIS order, not a filter: a reopened id is named before
    a reopened class, and a reopened class before a grown set, so the operator
    reads the most specific regression first. The class disposition is computed
    INDEPENDENTLY of the id precedence — a round that reopens both an id and its
    class reports both, because "we already told you about the id" is how a
    renaming pattern stays invisible.
    """
    violations = []
    if current["open_ids"] & closed_ids:
        violations.append("reopened_finding")
    if reopened_classes(previous, current, closed_classes):
        violations.append("class_reopened")
    if violations:
        return violations
    if len(current["open_ids"]) > len(previous["open_ids"]):
        return ["finding_set_grew"]
    if current["subject_digest"] != previous["subject_digest"]:
        return []
    previous_refs = {e["ref"] for e in previous["evidence_refs"]}
    resolved = previous["open_ids"] - current["open_ids"]
    # The evidence branch is NOT_PROVEN-only by construction, on both sides: a
    # FAIL says the subject is wrong, and no amount of new evidence over
    # unchanged bytes repairs a wrong subject. The new evidence must be BOUND:
    # it names this exact subject digest and at least one finding id that this
    # round actually closed. A bare new label admits nothing.
    binding_evidence = [
        e
        for e in current["evidence_refs"]
        if e["ref"] not in previous_refs
        and e.get("subject_digest") == current["subject_digest"]
        and resolved & set(e.get("resolves") or [])
    ]
    if (
        previous["status"] == "NOT_PROVEN"
        and current["status"] != "FAIL"
        and binding_evidence
    ):
        return []
    return ["no_subject_or_evidence_change"]


def law_violation(
    previous: Mapping[str, Any],
    current: Mapping[str, Any],
    closed_ids: set[str],
    closed_classes: Mapping[str, str] | None = None,
) -> str | None:
    """The single most specific violated condition, or None when all hold."""
    violations = law_violations(previous, current, closed_ids, closed_classes)
    return violations[0] if violations else None


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
    # class -> the finding id whose closure retired it. A class is retired only
    # once no open finding still carries it, so a surviving sibling never makes
    # its own class a violation.
    closed_classes: dict[str, str] = {}
    rounds_used = 0
    current = normalize_round(validations[0])
    previous = current
    stop_reason = "not_converged"
    stop_reasons: list[str] = []
    law_stopped = False

    for index, raw_candidate in enumerate(validations):
        if index > 0:
            # Condition 1: the caller's bound, checked before the round is even
            # normalized, so a round past the bound is never consumed.
            if rounds_used >= repair_rounds:
                stop_reason = "repair_budget_exhausted"
                break
            candidate = normalize_round(raw_candidate)
            assert_stable_classes(previous, candidate)
            rounds_used += 1
            current = candidate
            checked.append(
                f"repair round {rounds_used}: {len(current['open_ids'])} open findings"
            )
            violations = law_violations(previous, current, closed_ids, closed_classes)
            if violations:
                # Precedence orders the diagnosis; it never deletes the other
                # dispositions, so both are on the record and in the report.
                stop_reason = violations[0]
                stop_reasons = list(violations)
                checked.append(
                    f"repair round {rounds_used}: convergence law: "
                    + ", ".join(violations)
                )
                law_stopped = True
                break
            resolved_ids = previous["open_ids"] - current["open_ids"]
            closed_ids |= resolved_ids
            still_open = set(current["open_classes"].values())
            # Comprehension, not a nested loop: the only iteration this phase
            # performs is over the supplied rounds. Reverse order lets the
            # lowest id win the class, so the record is deterministic.
            retired = {
                previous["open_classes"][resolved_id]: resolved_id
                for resolved_id in sorted(resolved_ids, reverse=True)
                if previous["open_classes"].get(resolved_id) is not None
                and previous["open_classes"][resolved_id] not in still_open
            }
            closed_classes.update(
                {k: v for k, v in retired.items() if k not in closed_classes}
            )
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
    if not stop_reasons:
        stop_reasons = [stop_reason]

    status = current["status"]
    # A class reopen means each round named a different id for the same kind of
    # defect, so no round's ruling binds to a converging subject: the honest
    # outcome is NOT_PROVEN, never the churning round's own status. It holds
    # whether or not a reopened id outranked it in the diagnosis order.
    if "class_reopened" in stop_reasons:
        status = "NOT_PROVEN"
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
        "stop_reasons": stop_reasons,
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
