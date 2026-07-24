#!/usr/bin/env python3
"""Pure reference behavior for one exact RPI invocation.

Plan returns resolved intent bytes.  The runtime mints those bytes once and
passes only the immutable ref+digest identity to Implement and Validate.  Each
phase is called at most once and every terminal status is reported immediately.
"""

from __future__ import annotations

from collections.abc import Callable, Mapping
import importlib.util
from pathlib import Path
from typing import Any


KERNEL_PATH = Path(__file__).parents[2] / "validate" / "scripts" / "kernel_v3.py"
SPEC = importlib.util.spec_from_file_location("agentops_kernel_v3", KERNEL_PATH)
assert SPEC and SPEC.loader
kernel = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(kernel)


def invoke_once(
    intent: Any,
    *,
    invocation_id: str,
    intent_dir: Path,
    intent_ref_root: str = ".agents/ao/intents",
    correlation: dict[str, str] | None = None,
    report_dir: Path | None = None,
    plan_phase: Callable[[Any], bytes | None],
    implement_phase: Callable[[Mapping[str, str]], Mapping[str, Any] | None],
    validate_phase: Callable[
        [Mapping[str, str], Mapping[str, Any]],
        Mapping[str, Any],
    ],
) -> dict[str, Any]:
    """Dispatch Plan, Implement, and Validate no more than once each."""
    def finish(report: dict[str, Any]) -> dict[str, Any]:
        if report_dir is not None:
            kernel.store_rpi_report_v2(report, report_dir)
        return report

    resolved_bytes = plan_phase(intent)
    if resolved_bytes is None:
        return finish(kernel.build_rpi_report_v2(
            invocation_id=invocation_id,
            correlation=correlation,
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
            not_checked=["implement", "validate"],
        ))
    if not isinstance(resolved_bytes, bytes):
        raise TypeError("Plan must return exact resolved intent bytes")
    intent_path, intent_digest, _ = kernel.mint_intent_snapshot(
        resolved_bytes,
        intent_dir,
    )
    intent_ref = (
        kernel.normalize_rel(intent_ref_root).rstrip("/")
        + "/"
        + intent_path.name
    )
    intent_identity = {
        "intent_ref": intent_ref,
        "intent_digest": intent_digest,
    }

    subject = implement_phase(intent_identity)
    if subject is None:
        return finish(kernel.build_rpi_report_v2(
            invocation_id=invocation_id,
            correlation=correlation,
            status="NOT_BUILT",
            intent_ref=intent_ref,
            intent_digest=intent_digest,
            proof_identity=None,
            before_manifest_digest=None,
            final_manifest_digest=None,
            effect_receipt_digest=None,
            verdict_ref=None,
            verdict_digest=None,
            checked=["plan"],
            not_checked=["validate"],
        ))
    subject = dict(subject)
    validation = dict(validate_phase(intent_identity, subject))
    status = validation.get("verdict")
    if status not in {"PASS", "FAIL", "NOT_PROVEN"}:
        raise ValueError("Validate must return PASS, FAIL, or NOT_PROVEN")
    if validation.get("intent_digest") != intent_digest:
        raise ValueError("Validate verdict does not match exact intent bytes")
    required = (
        "proof_identity",
        "before_manifest_digest",
        "final_manifest_digest",
        "effect_receipt_digest",
        "verdict_ref",
        "verdict_digest",
    )
    if any(validation.get(field) is None for field in required):
        raise ValueError("Validate must return durable proof and subject identities")
    return finish(kernel.build_rpi_report_v2(
        invocation_id=invocation_id,
        correlation=correlation,
        status=status,
        intent_ref=intent_ref,
        intent_digest=intent_digest,
        proof_identity=validation["proof_identity"],
        before_manifest_digest=validation["before_manifest_digest"],
        final_manifest_digest=validation["final_manifest_digest"],
        effect_receipt_digest=validation["effect_receipt_digest"],
        verdict_ref=validation["verdict_ref"],
        verdict_digest=validation["verdict_digest"],
        checked=list(validation.get("checked") or []),
        not_checked=list(validation.get("not_checked") or []),
    ))
