"""Static reference for the post-observation GC33 handoff contract.

This module is deliberately pure: it makes no Beads, Dolt, Gas City, process,
or filesystem calls.  ``beads-capability.py`` remains the exact executable
that produced the successful live-attempt-2 receipt (SHA-256
``b5d6d4490492de047554c984008289282aab82cd3e89bf67d5ae8ea71bcbc48e``).
That receipt is store-observation provenance, not evidence that this later
static tightening was executed against the real store.
"""

from __future__ import annotations

from typing import Any, Mapping


def committed_handoff_matches(
    prepared: Mapping[str, Any],
    published: Mapping[str, Any],
    committed: Mapping[str, Any],
    *,
    prepared_digest: str,
    published_digest: str,
) -> bool:
    """Authorize only one committed marker bound to every delivery identity.

    The committed marker binds its prepared source digest and the immutable
    published payload digest.  The explicit comparisons make a substituted
    semantic bead, handoff, certificate, successor, external reference, epoch,
    mode, state, or deadline a fail-closed refusal before any effect.
    """

    expected = {
        "handoff_id": prepared["handoff_id"],
        "prepared_digest": prepared_digest,
        "semantic_bead_id": prepared["semantic_bead_id"],
        "semantic_terminal_verdict": "PASS",
        "semantic_terminal_ref": prepared["semantic_terminal_ref"],
        "admission_certificate_digest": prepared["admission_certificate_digest"],
        "delivery_bead_id": prepared["expected_delivery_bead_id"],
        "expected_external_ref": prepared["expected_external_ref"],
        "epoch": prepared["epoch"],
        "delivery_payload_ref": "delivery.published.json",
        "delivery_payload_digest": published_digest,
        "mode": published["mode"],
        "state": published["state"],
        "deadline": published["deadline"],
    }
    published_identity = {
        "kind": "delivery",
        "handoff_id": prepared["handoff_id"],
        "semantic_bead_id": prepared["semantic_bead_id"],
        "semantic_terminal_ref": prepared["semantic_terminal_ref"],
        "admission_certificate_digest": prepared["admission_certificate_digest"],
        "delivery_bead_id": prepared["expected_delivery_bead_id"],
        "external_ref": prepared["expected_external_ref"],
        "epoch": prepared["epoch"],
        "mode": prepared["mode"],
        "state": prepared["state"],
        "publication": "published",
        "deadline": prepared["deadline"],
    }
    return all(committed.get(key) == value for key, value in expected.items()) and all(
        published.get(key) == value for key, value in published_identity.items()
    )
