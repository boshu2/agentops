#!/usr/bin/env python3
"""Pure reference behavior for one RPI invocation.

The caller supplies the three phase functions.  This module dispatches each at
most once, translates missing phase output into an RPI report status, and never
chooses a retry or next action.
"""

from __future__ import annotations

from collections.abc import Callable, Mapping
import hashlib
import json
from typing import Any


def digest(value: Any) -> str:
    payload = json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def report(
    status: str,
    *,
    plan_digest: str | None = None,
    subject_digest: str | None = None,
    verdict_ref: str | None = None,
    verdict_digest: str | None = None,
    checked: list[str] | None = None,
    not_checked: list[str] | None = None,
) -> dict[str, Any]:
    return {
        "schema_version": "rpi-report.v1",
        "status": status,
        "plan_packet_digest": plan_digest,
        "subject_manifest_digest": subject_digest,
        "verdict_ref": verdict_ref,
        "verdict_digest": verdict_digest,
        "checked": checked or [],
        "not_checked": not_checked or [],
    }


def invoke_once(
    intent: Any,
    plan_phase: Callable[[Any], Mapping[str, Any] | None],
    implement_phase: Callable[[Mapping[str, Any]], Mapping[str, Any] | None],
    validate_phase: Callable[[Mapping[str, Any], Mapping[str, Any]], Mapping[str, Any]],
) -> dict[str, Any]:
    """Dispatch Plan, Implement, and Validate no more than once each."""
    plan = plan_phase(intent)
    if plan is None:
        return report("NOT_PLANNED", not_checked=["implement", "validate"])
    plan = dict(plan)
    plan_packet_digest = digest(plan)

    candidate = implement_phase(plan)
    if candidate is None:
        return report(
            "NOT_BUILT",
            plan_digest=plan_packet_digest,
            checked=["plan"],
            not_checked=["validate"],
        )
    candidate = dict(candidate)

    validation = dict(validate_phase(plan, candidate))
    status = validation.get("verdict")
    if status not in {"PASS", "FAIL", "NOT_PROVEN"}:
        raise ValueError("Validate must return PASS, FAIL, or NOT_PROVEN")
    subject_digest = validation.get("subject_manifest_digest")
    verdict_digest = validation.get("verdict_digest")
    verdict_ref = validation.get("verdict_ref")
    if not all(isinstance(value, str) and value for value in (subject_digest, verdict_digest, verdict_ref)):
        raise ValueError("Validate must return durable verdict and subject identities")
    return report(
        status,
        plan_digest=plan_packet_digest,
        subject_digest=subject_digest,
        verdict_ref=verdict_ref,
        verdict_digest=verdict_digest,
        checked=list(validation.get("checked") or []),
        not_checked=list(validation.get("not_checked") or []),
    )
