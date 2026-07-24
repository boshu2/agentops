#!/usr/bin/env python3
"""Verify the active proof pointer and every byte frozen by its descriptor."""

from __future__ import annotations

import argparse
from datetime import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
import sys
from typing import Any

EPOCH_ONE_REQUIRED_COMPONENT_ROLES = {
    "validator-contract",
    "validator-implementation",
    "validator-cli",
    "verdict-schema",
    "rpi-report-schema",
    "subject-manifest-schema",
    "scope-index-schema",
    "effect-receipt-schema",
    "check-receipt-schema",
}


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


def load_kernel_module(path: Path):
    specification = importlib.util.spec_from_file_location(
        "agentops_kernel_v3_proof_check", path
    )
    if specification is None or specification.loader is None:
        raise RuntimeError(f"cannot load epoch-1 proof reader: {path}")
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


def repository_ref(module: Any, repository: Path, raw: Any, label: str) -> Path:
    if (
        not isinstance(raw, str)
        or not raw
        or raw == "."
        or "\\" in raw
        or raw.startswith("/")
        or raw.startswith("//")
        or raw.endswith("/")
        or (len(raw) >= 2 and raw[0].isalpha() and raw[1] == ":")
        or any(part in {"", ".", ".."} for part in raw.split("/"))
        or any(ord(character) < 32 or ord(character) == 127 for character in raw)
    ):
        raise module.TransitionError(f"{label} is not a canonical repository ref")
    target = repository / raw
    cursor = repository
    for part in raw.split("/"):
        cursor = cursor / part
        if cursor.is_symlink():
            raise module.TransitionError(f"{label} traverses a symlink")
    try:
        target.resolve(strict=False).relative_to(repository.resolve())
    except ValueError as error:
        raise module.TransitionError(f"{label} escapes the repository") from error
    return target


def exact_artifact(module: Any, path: Path, label: str) -> dict[str, Any]:
    try:
        information = path.lstat()
        executable = bool(
            information.st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
        )
        if path.is_symlink():
            kind = "symlink"
            payload = os.readlink(path).encode("utf-8")
        elif path.is_file():
            kind = "file"
            payload = path.read_bytes()
        else:
            raise module.TransitionError(f"{label} is not a file or symlink")
    except OSError as error:
        raise module.TransitionError(f"{label} is unavailable: {error}") from error
    return {
        "kind": kind,
        "digest": hashlib.sha256(payload).hexdigest(),
        "executable": executable,
        "mode": mode(path),
    }


def check_epoch_one(
    repository: Path,
    active_path: Path,
    active: dict[str, Any],
    module: Any,
) -> dict[str, Any]:
    if set(active) != module.ACTIVE_KEYS:
        raise module.TransitionError("active proof pointer has invalid fields")
    if active["schema_version"] != "proof-contract-active.v1" or active["epoch"] != 1:
        raise module.TransitionError("epoch-1 active pointer identity is invalid")
    for field in ("contract_digest", "activation_transition_digest"):
        if not module.valid_digest(active[field]):
            raise module.TransitionError(f"active {field} must be SHA-256")
    descriptor_path = repository_ref(
        module, repository, active["contract_ref"], "active contract_ref"
    )
    transition_path = repository_ref(
        module,
        repository,
        active["activation_transition_ref"],
        "active activation_transition_ref",
    )
    descriptor_bytes = descriptor_path.read_bytes()
    if hashlib.sha256(descriptor_bytes).hexdigest() != active["contract_digest"]:
        raise module.TransitionError("active descriptor bytes do not match pointer")
    transition_bytes = transition_path.read_bytes()
    if hashlib.sha256(transition_bytes).hexdigest() != active[
        "activation_transition_digest"
    ]:
        raise module.TransitionError("active transition bytes do not match pointer")
    if transition_path.name != f"{active['activation_transition_digest']}.json":
        raise module.TransitionError("active transition filename is not content-addressed")

    descriptor = module.load_object(descriptor_path)
    module.validate_descriptor(descriptor, expected_epoch=1)
    if type(descriptor.get("epoch")) is not int:
        raise module.TransitionError("epoch-1 descriptor epoch must be an integer")
    roles: dict[str, dict[str, Any]] = {}
    subject_bindings: list[tuple[str, dict[str, Any], dict[str, Any]]] = []
    for index, component in enumerate(descriptor["components"]):
        role, reference = module.validate_component(component, index)
        if role in roles:
            raise module.TransitionError("epoch-1 descriptor has duplicate roles")
        path = repository_ref(
            module, repository, reference, f"component ref for {role}"
        )
        observed = exact_artifact(module, path, f"component {role}")
        if (
            observed["digest"] != component["digest"]
            or observed["mode"] != component["mode"]
        ):
            raise module.TransitionError(
                f"frozen component bytes or mode changed: {role}"
            )
        roles[role] = component
        subject_bindings.append((role, component, observed))
    missing = EPOCH_ONE_REQUIRED_COMPONENT_ROLES - set(roles)
    if missing:
        raise module.TransitionError(
            "epoch-1 descriptor lacks required roles: " + ", ".join(sorted(missing))
        )

    corpus = descriptor["qualification_corpus"]
    corpus_path = repository_ref(
        module, repository, corpus["ref"], "qualification corpus ref"
    )
    if module.tree_digest(corpus_path) != corpus["digest"]:
        raise module.TransitionError("frozen qualification corpus digest changed")
    recorder = descriptor["transition_recorder"]
    recorder_path = repository_ref(
        module, repository, recorder["ref"], "transition recorder ref"
    )
    recorder_observed = exact_artifact(
        module, recorder_path, "transition recorder"
    )
    if (
        recorder_observed["digest"] != recorder["digest"]
        or recorder_observed["mode"] != recorder["mode"]
    ):
        raise module.TransitionError("frozen transition recorder bytes or mode changed")
    subject_bindings.append(("transition-recorder", recorder, recorder_observed))

    transition = module.load_object(transition_path)
    if transition_bytes != module.canonical_bytes(transition) + b"\n":
        raise module.TransitionError("active transition bytes are not canonical")
    required_transition = {
        "schema_version",
        "prior",
        "candidate",
        "qualification_verdict",
        "validator_identity",
        "activated_at",
    }
    if (
        set(transition) != required_transition
        or transition["schema_version"] != "proof-contract-transition.v1"
    ):
        raise module.TransitionError("active transition has invalid fields")
    prior = transition["prior"]
    candidate = transition["candidate"]
    verdict_binding = transition["qualification_verdict"]
    if not isinstance(prior, dict) or set(prior) != {
        "epoch",
        "contract_ref",
        "contract_digest",
        "activation_transition_digest",
    }:
        raise module.TransitionError("active transition prior has invalid fields")
    if not isinstance(candidate, dict) or set(candidate) != {
        "epoch",
        "contract_ref",
        "contract_digest",
        "subject_manifest_ref",
        "subject_manifest_digest",
        "qualification_corpus_ref",
        "qualification_corpus_digest",
    }:
        raise module.TransitionError("active transition candidate has invalid fields")
    if not isinstance(verdict_binding, dict) or set(verdict_binding) != {
        "ref",
        "digest",
    }:
        raise module.TransitionError(
            "active transition qualification verdict has invalid fields"
        )
    if (
        type(candidate["epoch"]) is not int
        or candidate["epoch"] != 1
        or candidate["contract_ref"] != active["contract_ref"]
        or candidate["contract_digest"] != active["contract_digest"]
        or candidate["subject_manifest_digest"]
        != descriptor["qualification_subject_manifest_digest"]
        or candidate["qualification_corpus_ref"] != corpus["ref"]
        or candidate["qualification_corpus_digest"] != corpus["digest"]
    ):
        raise module.TransitionError(
            "active proof transition does not bind the active descriptor"
        )
    if (
        not isinstance(transition["validator_identity"], str)
        or not transition["validator_identity"]
    ):
        raise module.TransitionError("active transition validator identity is invalid")
    try:
        activated = datetime.fromisoformat(
            transition["activated_at"].replace("Z", "+00:00")
        )
    except (AttributeError, ValueError) as error:
        raise module.TransitionError(
            "active transition activated_at is invalid"
        ) from error
    if activated.tzinfo is None:
        raise module.TransitionError("active transition activated_at lacks timezone")

    prior_path = repository_ref(
        module, repository, prior["contract_ref"], "prior contract_ref"
    )
    if (
        type(prior["epoch"]) is not int
        or prior["epoch"] != 0
        or prior["activation_transition_digest"] is not None
        or not module.valid_digest(prior["contract_digest"])
        or hashlib.sha256(prior_path.read_bytes()).hexdigest()
        != prior["contract_digest"]
    ):
        raise module.TransitionError("active transition prior binding is invalid")
    prior_descriptor = module.load_object(prior_path)
    if (
        set(prior_descriptor) != module.DESCRIPTOR_KEYS
        or prior_descriptor.get("schema_version") != "proof-contract.v1"
        or type(prior_descriptor.get("epoch")) is not int
        or prior_descriptor.get("epoch") != 0
        or prior_descriptor.get("qualification_subject_manifest_digest") is not None
    ):
        raise module.TransitionError(
            "active transition prior descriptor is invalid"
        )
    prior_roles: set[str] = set()
    for index, component in enumerate(prior_descriptor.get("components", [])):
        role, reference = module.validate_component(component, index)
        repository_ref(
            module, repository, reference, f"prior component ref for {role}"
        )
        if role in prior_roles:
            raise module.TransitionError(
                "active transition prior descriptor duplicates roles"
            )
        prior_roles.add(role)
    if module.REQUIRED_COMPONENT_ROLES - prior_roles:
        raise module.TransitionError(
            "active transition prior descriptor lacks required roles"
        )
    prior_corpus = prior_descriptor.get("qualification_corpus")
    prior_recorder = prior_descriptor.get("transition_recorder")
    if (
        not isinstance(prior_corpus, dict)
        or set(prior_corpus) != {"algorithm", "ref", "digest"}
        or prior_corpus.get("algorithm") != "sha256-tree-v1"
        or not module.valid_digest(prior_corpus.get("digest"))
        or not isinstance(prior_recorder, dict)
        or set(prior_recorder) != {"ref", "digest", "mode"}
    ):
        raise module.TransitionError(
            "active transition prior proof surfaces are invalid"
        )
    repository_ref(
        module, repository, prior_corpus["ref"], "prior qualification corpus ref"
    )
    repository_ref(
        module, repository, prior_recorder["ref"], "prior transition recorder ref"
    )

    subject_path = repository_ref(
        module,
        repository,
        candidate["subject_manifest_ref"],
        "qualification subject ref",
    )
    subject = module.load_object(subject_path)
    subject_digest = module.manifest_digest(subject)
    if subject_digest != candidate["subject_manifest_digest"]:
        raise module.TransitionError("qualification subject binding changed")
    entries = subject.get("entries")
    if not isinstance(entries, list):
        raise module.TransitionError("qualification subject entries are invalid")
    by_path = {
        entry.get("path"): entry
        for entry in entries
        if isinstance(entry, dict) and isinstance(entry.get("path"), str)
    }
    if len(by_path) != len(entries):
        raise module.TransitionError(
            "qualification subject entries are invalid or duplicated"
        )
    for role, binding, observed in subject_bindings:
        entry = by_path.get(binding["ref"])
        if (
            entry is None
            or entry.get("kind") != observed["kind"]
            or entry.get("digest") != observed["digest"]
            or entry.get("executable") is not observed["executable"]
        ):
            raise module.TransitionError(
                f"qualification subject does not bind component: {role}"
            )

    verdict_path = repository_ref(
        module, repository, verdict_binding["ref"], "qualification verdict ref"
    )
    verdict = module.load_object(verdict_path)
    verdict_digest = module.verdict_digest(verdict)
    if (
        verdict_digest != verdict_binding["digest"]
        or verdict_path.name != f"{verdict_digest}.json"
        or verdict_path.read_bytes() != module.canonical_bytes(verdict) + b"\n"
        or verdict.get("subject_manifest_digest") != subject_digest
    ):
        raise module.TransitionError("qualification verdict binding is invalid")

    validator_binding = roles["validator-implementation"]
    validator_path = repository_ref(
        module,
        repository,
        validator_binding["ref"],
        "validator implementation ref",
    )
    kernel = load_kernel_module(validator_path)
    identity = kernel.load_active_proof(repository)
    expected_identity = {
        "epoch": 1,
        "contract_ref": active["contract_ref"],
        "contract_digest": active["contract_digest"],
        "activation_transition_digest": active["activation_transition_digest"],
    }
    if identity != expected_identity:
        raise module.TransitionError(
            "descriptor-bound validator disagrees with standalone proof check"
        )
    return {
        "result": "PASS",
        "epoch": 1,
        "contract_digest": active["contract_digest"],
        "activation_transition_digest": active["activation_transition_digest"],
        "component_count": len(roles),
        "qualification_corpus_digest": corpus["digest"],
        "bootstrap_replacement_count": len(
            list(
                (
                    repository
                    / "docs/contracts/proof-contracts/bootstrap-root-replacements"
                ).glob("*.json")
            )
        ),
    }


def mode(path: Path) -> str:
    return f"{stat.S_IMODE(path.lstat().st_mode):04o}"


def check(repository: Path, active_path: Path) -> dict[str, Any]:
    module = load_bootstrap_module(repository)
    active = module.load_object(active_path)
    epoch = active.get("epoch")
    if type(epoch) is not int or epoch < 0:
        raise module.TransitionError(
            "active proof pointer epoch must be a nonnegative integer"
        )
    if epoch == 1:
        expected_active = (
            repository / "docs/contracts/proof-contracts/active.json"
        ).resolve()
        if active_path.resolve() != expected_active:
            raise module.TransitionError(
                "epoch-1 checker requires the canonical active pointer"
            )
        return check_epoch_one(repository, active_path, active, module)
    if epoch != 0:
        raise module.TransitionError(
            f"unsupported active proof epoch: {epoch}; checker supports 0 and 1"
        )
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
