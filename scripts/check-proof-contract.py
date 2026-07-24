#!/usr/bin/env python3
"""Verify the active proof pointer and every byte frozen by its descriptor."""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
import sys
from typing import Any


def load_bootstrap_module(repository: Path):
    path = repository / "scripts" / "bootstrap-proof-transition.py"
    specification = importlib.util.spec_from_file_location(
        "agentops_bootstrap_proof_transition", path
    )
    if specification is None or specification.loader is None:
        raise RuntimeError(f"cannot load bootstrap recorder: {path}")
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


def mode(path: Path) -> str:
    return f"{stat.S_IMODE(path.lstat().st_mode):04o}"


def check(repository: Path, active_path: Path) -> dict[str, Any]:
    module = load_bootstrap_module(repository)
    active = module.load_object(active_path)
    module.validate_active(active)
    descriptor_path = repository / active["contract_ref"]
    descriptor = module.load_object(descriptor_path)
    if descriptor.get("epoch") != active["epoch"]:
        raise module.TransitionError("active pointer and descriptor epochs differ")
    expected_descriptor_digest = hashlib.sha256(descriptor_path.read_bytes()).hexdigest()
    if active["contract_digest"] != expected_descriptor_digest:
        raise module.TransitionError("active descriptor bytes do not match pointer")

    component_roles: list[str] = []
    for index, component in enumerate(descriptor.get("components", [])):
        role, _ = module.validate_component(component, index)
        component_roles.append(role)
        component_path = repository / component["ref"]
        if hashlib.sha256(component_path.read_bytes()).hexdigest() != component["digest"]:
            raise module.TransitionError(f"frozen component digest changed: {role}")
        if mode(component_path) != component["mode"]:
            raise module.TransitionError(f"frozen component mode changed: {role}")
    if len(component_roles) != len(set(component_roles)):
        raise module.TransitionError("frozen descriptor has duplicate component roles")
    missing = sorted(module.REQUIRED_COMPONENT_ROLES - set(component_roles))
    if missing:
        raise module.TransitionError(
            "frozen descriptor lacks required roles: " + ", ".join(missing)
        )

    corpus = descriptor.get("qualification_corpus")
    if not isinstance(corpus, dict):
        raise module.TransitionError("frozen descriptor lacks qualification corpus")
    corpus_path = repository / corpus["ref"]
    if module.tree_digest(corpus_path) != corpus.get("digest"):
        raise module.TransitionError("frozen qualification corpus digest changed")

    recorder = descriptor.get("transition_recorder")
    if not isinstance(recorder, dict):
        raise module.TransitionError("frozen descriptor lacks transition recorder")
    recorder_path = repository / recorder["ref"]
    if hashlib.sha256(recorder_path.read_bytes()).hexdigest() != recorder.get("digest"):
        raise module.TransitionError("frozen transition recorder digest changed")
    if mode(recorder_path) != recorder.get("mode"):
        raise module.TransitionError("frozen transition recorder mode changed")

    if active["epoch"] == 0 and descriptor.get(
        "qualification_subject_manifest_digest"
    ) is not None:
        raise module.TransitionError(
            "bootstrap descriptor must not claim a qualification subject"
        )
    replacements = sorted(
        (
            repository
            / "docs"
            / "contracts"
            / "proof-contracts"
            / "bootstrap-root-replacements"
        ).glob("*.json")
    )
    if active["contract_ref"].startswith(
        "docs/contracts/proof-contracts/epoch-0b/"
    ):
        if len(replacements) != 1:
            raise module.TransitionError(
                "corrected bootstrap root requires exactly one replacement record"
            )
        replacement = module.load_object(replacements[0])
        required = {
            "schema_version",
            "rejected_contract_ref",
            "rejected_contract_digest",
            "rejecting_verdict_ref",
            "rejecting_verdict_digest",
            "replacement_contract_ref",
            "replacement_contract_digest",
            "authority",
            "reason",
            "replaced_at",
        }
        if set(replacement) != required or replacement["schema_version"] != (
            "proof-bootstrap-root-replacement.v1"
        ):
            raise module.TransitionError(
                "bootstrap root replacement record has invalid fields"
            )
        rejected = repository / replacement["rejected_contract_ref"]
        if (
            hashlib.sha256(rejected.read_bytes()).hexdigest()
            != replacement["rejected_contract_digest"]
        ):
            raise module.TransitionError("rejected bootstrap descriptor changed")
        verdict_path = repository / replacement["rejecting_verdict_ref"]
        verdict = module.load_object(verdict_path)
        if (
            verdict.get("artifact_digest")
            != replacement["rejecting_verdict_digest"]
            or verdict.get("verdict") != "FAIL"
        ):
            raise module.TransitionError(
                "bootstrap replacement is not bound to the rejecting FAIL"
            )
        unsigned = {
            key: value
            for key, value in verdict.items()
            if key != "artifact_digest"
        }
        if (
            hashlib.sha256(module.canonical_bytes(unsigned)).hexdigest()
            != verdict["artifact_digest"]
        ):
            raise module.TransitionError("rejecting verdict digest changed")
        if (
            verdict_path.name != f"{verdict['artifact_digest']}.json"
            or verdict_path.read_bytes() != module.canonical_bytes(verdict) + b"\n"
        ):
            raise module.TransitionError(
                "rejecting verdict bytes or filename changed"
            )
        if (
            replacement["replacement_contract_ref"] != active["contract_ref"]
            or replacement["replacement_contract_digest"]
            != active["contract_digest"]
        ):
            raise module.TransitionError(
                "bootstrap replacement does not bind the active pointer"
            )
        if not replacement["authority"] or not replacement["reason"]:
            raise module.TransitionError(
                "bootstrap replacement lacks authority or reason"
            )
    return {
        "result": "PASS",
        "epoch": active["epoch"],
        "contract_digest": active["contract_digest"],
        "component_count": len(component_roles),
        "qualification_corpus_digest": corpus["digest"],
        "bootstrap_replacement_count": len(replacements),
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", default=".")
    parser.add_argument(
        "--active-pointer",
        default="docs/contracts/proof-contracts/active.json",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    repository = Path(args.repository).resolve()
    active_path = Path(args.active_pointer)
    if not active_path.is_absolute():
        active_path = repository / active_path
    try:
        result = check(repository, active_path)
    except (OSError, RuntimeError, ValueError) as error:
        print(f"check-proof-contract: FAIL — {error}", file=sys.stderr)
        return 1
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
