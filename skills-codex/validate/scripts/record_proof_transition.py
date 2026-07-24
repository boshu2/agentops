#!/usr/bin/env python3
"""Advance an activated proof contract from epoch N to N+1.

The recorder is a narrow compare-and-swap writer.  It accepts only a PASS
verdict.v3 issued under the currently active proof identity, verifies that every
candidate proof component and the future recorder are exact members of the
judged subject, writes a content-addressed transition, and replaces the active
pointer last.  Bootstrap epoch 0 -> 1 remains owned by the frozen bootstrap
recorder because its qualifying verdict is legacy verdict.v2.
"""

from __future__ import annotations

import argparse
import fcntl
import json
import os
from pathlib import Path
import sys
from typing import Any

import kernel_v3 as kernel


def repository_ref(repository: Path, path: Path) -> str:
    try:
        relative = path.resolve().relative_to(repository.resolve()).as_posix()
    except ValueError as exc:
        raise kernel.ContractError(f"path is outside repository: {path}") from exc
    return kernel.normalize_rel(relative)


def manifest_entries(manifest: dict[str, Any]) -> dict[str, dict[str, Any]]:
    entries = manifest.get("entries")
    if not isinstance(entries, list):
        raise kernel.ContractError("qualification subject has no entries")
    result: dict[str, dict[str, Any]] = {}
    for entry in entries:
        if (
            not isinstance(entry, dict)
            or not isinstance(entry.get("path"), str)
            or entry["path"] in result
        ):
            raise kernel.ContractError(
                "qualification subject entries are invalid or duplicated"
            )
        result[entry["path"]] = entry
    return result


def verify_candidate_membership(
    repository: Path,
    candidate: dict[str, Any],
    candidate_ref: str,
    subject: dict[str, Any],
) -> None:
    entries = manifest_entries(subject)
    bindings = list(candidate["components"]) + [
        {
            "role": "transition-recorder",
            **candidate["transition_recorder"],
        }
    ]
    for binding in bindings:
        reference = binding["ref"]
        path = kernel.resolve_repository_ref(repository, reference)
        observed_digest = kernel.sha256(path.read_bytes())
        observed_mode = kernel._mode(path)
        if (
            observed_digest != binding["digest"]
            or observed_mode != binding["mode"]
        ):
            raise kernel.TerminalValidation(
                f"candidate proof binding changed: {reference}"
            )
        entry = entries.get(reference)
        if (
            entry is None
            or entry.get("kind") not in {"file", "symlink"}
            or entry.get("digest") != observed_digest
            or entry.get("executable") != bool(int(observed_mode, 8) & 0o111)
        ):
            raise kernel.TerminalValidation(
                f"candidate proof component is not exact in judged subject: {reference}"
            )
    descriptor_entry = entries.get(candidate_ref)
    if descriptor_entry is not None:
        descriptor_path = kernel.resolve_repository_ref(repository, candidate_ref)
        if descriptor_entry.get("digest") != kernel.sha256(
            descriptor_path.read_bytes()
        ):
            raise kernel.TerminalValidation(
                "candidate descriptor entry differs from judged subject"
            )


def record_transition(
    *,
    repository: Path,
    active_pointer: Path,
    candidate_descriptor: Path,
    qualification_subject_manifest: Path,
    qualification_verdict: Path,
    qualification_corpus: Path,
    transitions_dir: Path,
    validator_identity: str,
    activated_at: str,
) -> dict[str, Any]:
    repository = repository.resolve()
    active_ref = repository_ref(repository, active_pointer)
    candidate_ref = repository_ref(repository, candidate_descriptor)
    subject_ref = repository_ref(repository, qualification_subject_manifest)
    verdict_ref = repository_ref(repository, qualification_verdict)
    corpus_ref = repository_ref(repository, qualification_corpus)
    transitions_ref = repository_ref(repository, transitions_dir)
    if active_ref != kernel.PROOF_ACTIVE_REF:
        raise kernel.ContractError(
            f"active pointer must be {kernel.PROOF_ACTIVE_REF}"
        )
    kernel.require_id(validator_identity, "transition validator identity")
    kernel.require_datetime(activated_at, "transition activated_at")

    lock_path = active_pointer.parent / f".{active_pointer.name}.lock"
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a+b") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        active_bytes = active_pointer.read_bytes()
        prior_identity = kernel.load_active_proof(repository)
        if prior_identity["epoch"] < 1:
            raise kernel.ContractError(
                "general recorder requires active epoch >= 1; use bootstrap recorder"
            )

        candidate_bytes = candidate_descriptor.read_bytes()
        candidate = kernel.load_json_bytes(
            candidate_bytes,
            "candidate proof contract",
        )
        kernel.validate_proof_contract_v1(candidate)
        if candidate["epoch"] != prior_identity["epoch"] + 1:
            raise kernel.ContractError(
                "candidate proof epoch must be active epoch plus one"
            )
        candidate_digest = kernel.sha256(candidate_bytes)

        corpus_digest = kernel.tree_digest(qualification_corpus)
        if candidate["qualification_corpus"] != {
            "algorithm": "sha256-tree-v1",
            "ref": corpus_ref,
            "digest": corpus_digest,
        }:
            raise kernel.TerminalValidation(
                "candidate qualification corpus binding is invalid"
            )

        subject = kernel.load_json(
            qualification_subject_manifest,
            "qualification subject manifest",
        )
        subject_digest = kernel._manifest_digest(subject)
        if candidate["qualification_subject_manifest_digest"] != subject_digest:
            raise kernel.TerminalValidation(
                "candidate qualification subject binding is invalid"
            )
        if subject.get("schema_version") == "subject-manifest.v2":
            kernel.verify_manifest_v2(subject, repository)
        verify_candidate_membership(
            repository,
            candidate,
            candidate_ref,
            subject,
        )

        verdict = kernel.load_json(
            qualification_verdict,
            "qualification verdict",
        )
        if verdict.get("schema_version") != "verdict.v3":
            raise kernel.ContractError(
                "general recorder requires a verdict.v3 qualification"
            )
        kernel.validate_verdict_v3(verdict)
        if verdict["verdict"] != "PASS":
            raise kernel.ContractError("qualification verdict must PASS")
        if qualification_verdict.name != f"{verdict['artifact_digest']}.json":
            raise kernel.ContractError(
                "qualification verdict filename does not bind artifact digest"
            )
        if verdict["final_manifest_digest"] != subject_digest:
            raise kernel.TerminalValidation(
                "qualification verdict judged a different subject"
            )
        if verdict["proof_identity"] != prior_identity:
            raise kernel.TerminalValidation(
                "qualification verdict was not issued under active prior proof"
            )

        if active_pointer.read_bytes() != active_bytes:
            raise kernel.TerminalValidation(
                "active proof pointer changed during transition preparation"
            )
        if subject.get("schema_version") == "subject-manifest.v2":
            kernel.verify_manifest_v2(subject, repository)
        verify_candidate_membership(
            repository,
            candidate,
            candidate_ref,
            subject,
        )

        transition = {
            "schema_version": "proof-contract-transition.v1",
            "prior": prior_identity,
            "candidate": {
                "epoch": candidate["epoch"],
                "contract_ref": candidate_ref,
                "contract_digest": candidate_digest,
                "subject_manifest_ref": subject_ref,
                "subject_manifest_digest": subject_digest,
                "qualification_corpus_ref": corpus_ref,
                "qualification_corpus_digest": corpus_digest,
            },
            "qualification_verdict": {
                "ref": verdict_ref,
                "digest": verdict["artifact_digest"],
            },
            "validator_identity": validator_identity,
            "activated_at": activated_at,
        }
        kernel.validate_proof_transition_v1(transition)
        transition_payload = kernel.canonical_bytes(transition) + b"\n"
        transition_digest = kernel.sha256(transition_payload)
        transition_path = (
            kernel.resolve_repository_ref(repository, transitions_ref)
            / f"{transition_digest}.json"
        )
        if transition_path.exists():
            if transition_path.read_bytes() != transition_payload:
                raise kernel.ContractError("transition digest collision")
        else:
            kernel.atomic_write_bytes(transition_path, transition_payload)

        if candidate_descriptor.read_bytes() != candidate_bytes:
            raise kernel.TerminalValidation(
                "candidate descriptor changed before activation"
            )
        if subject.get("schema_version") == "subject-manifest.v2":
            kernel.verify_manifest_v2(subject, repository)
        verify_candidate_membership(
            repository,
            candidate,
            candidate_ref,
            subject,
        )
        if active_pointer.read_bytes() != active_bytes:
            raise kernel.TerminalValidation(
                "active proof pointer changed before compare-and-swap"
            )
        next_active = {
            "schema_version": "proof-contract-active.v1",
            "epoch": candidate["epoch"],
            "contract_ref": candidate_ref,
            "contract_digest": candidate_digest,
            "activation_transition_ref": repository_ref(
                repository,
                transition_path,
            ),
            "activation_transition_digest": transition_digest,
        }
        kernel.atomic_write_bytes(
            active_pointer,
            kernel.canonical_bytes(next_active) + b"\n",
        )
        return {
            "result": "ACTIVATED",
            "epoch": candidate["epoch"],
            "contract_digest": candidate_digest,
            "transition_ref": next_active["activation_transition_ref"],
            "transition_digest": transition_digest,
        }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--active-pointer", required=True)
    parser.add_argument("--candidate-descriptor", required=True)
    parser.add_argument("--qualification-subject-manifest", required=True)
    parser.add_argument("--qualification-verdict", required=True)
    parser.add_argument("--qualification-corpus", required=True)
    parser.add_argument("--transitions-dir", required=True)
    parser.add_argument("--validator-id", required=True)
    parser.add_argument("--activated-at", required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        result = record_transition(
            repository=Path(args.repository),
            active_pointer=Path(args.active_pointer),
            candidate_descriptor=Path(args.candidate_descriptor),
            qualification_subject_manifest=Path(
                args.qualification_subject_manifest
            ),
            qualification_verdict=Path(args.qualification_verdict),
            qualification_corpus=Path(args.qualification_corpus),
            transitions_dir=Path(args.transitions_dir),
            validator_identity=args.validator_id,
            activated_at=args.activated_at,
        )
        print(json.dumps(result, sort_keys=True))
        return 0
    except (kernel.ContractError, OSError, json.JSONDecodeError) as exc:
        print(f"record-proof-transition: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
