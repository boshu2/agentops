#!/usr/bin/env python3
"""Pure reference behavior for one exact RPI invocation.

Plan returns the single-mint immutable ref+digest+byte-length identity. RPI
verifies the already-minted snapshot and passes that same packet to Implement
and Validate; it never mints a second snapshot. Each phase is called at most
once and every terminal status is reported immediately.
"""

from __future__ import annotations

from collections.abc import Callable, Mapping
import importlib.util
from pathlib import Path
from types import MappingProxyType
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
    plan_phase: Callable[[Any], Mapping[str, Any] | None],
    implement_phase: Callable[[Mapping[str, Any]], Mapping[str, Any] | None],
    validate_phase: Callable[
        [Mapping[str, Any], Mapping[str, Any]],
        Mapping[str, Any],
    ],
) -> dict[str, Any]:
    """Dispatch Plan, Implement, and Validate no more than once each."""
    def finish(report: dict[str, Any]) -> dict[str, Any]:
        if report_dir is not None:
            kernel.store_rpi_report_v2(report, report_dir)
        return report

    planned_identity = plan_phase(intent)
    if planned_identity is None:
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
    if not isinstance(planned_identity, Mapping) or set(planned_identity) != {
        "intent_ref",
        "intent_digest",
        "byte_length",
    }:
        raise TypeError("Plan must return one exact intent identity packet")
    intent_ref = kernel.normalize_rel(planned_identity["intent_ref"])
    intent_digest = planned_identity["intent_digest"]
    if not kernel.valid_digest(intent_digest):
        raise kernel.ContractError("Plan intent_digest is invalid")
    byte_length = planned_identity["byte_length"]
    if type(byte_length) is not int or byte_length < 0:
        raise kernel.ContractError("Plan byte_length must be a nonnegative integer")
    expected_ref = (
        kernel.normalize_rel(intent_ref_root).rstrip("/")
        + "/"
        + f"{intent_digest}.intent"
    )
    if intent_ref != expected_ref:
        raise kernel.ContractError(
            f"Plan intent_ref does not bind the minted digest: expected {expected_ref}"
        )
    snapshot_bytes = kernel.consume_intent_snapshot(
        intent_dir / f"{intent_digest}.intent",
        intent_digest,
    )
    if len(snapshot_bytes) != byte_length:
        raise kernel.ContractError(
            "Plan byte_length does not match the exact intent snapshot"
        )
    intent_identity = MappingProxyType(
        {
            "intent_ref": intent_ref,
            "intent_digest": intent_digest,
            "byte_length": byte_length,
        }
    )

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
    # The identity is byte-addressed. Plan minted the exact snapshot, RPI
    # independently consumed and rehashed it above, and Validate must bind its
    # verdict to the same digest. RPI never invents a canonical-value digest.
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
