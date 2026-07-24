#!/usr/bin/env python3
"""Atomically activate proof-contract epoch 1 from the frozen bootstrap epoch.

This recorder is deliberately narrow and standalone.  It does not import the
candidate validator, proof reader, or RPI implementation it is qualifying.
It can perform only the epoch-0 -> epoch-1 transition.
"""

from __future__ import annotations

import argparse
from datetime import datetime
import fcntl
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import stat
import tempfile
from typing import Any


HEX64 = set("0123456789abcdef")
ACTIVE_KEYS = {
    "schema_version",
    "epoch",
    "contract_ref",
    "contract_digest",
    "activation_transition_ref",
    "activation_transition_digest",
}
DESCRIPTOR_KEYS = {
    "schema_version",
    "epoch",
    "components",
    "qualification_corpus",
    "qualification_subject_manifest_digest",
    "transition_recorder",
    "known_gaps",
}
REQUIRED_COMPONENT_ROLES = {
    "validator-contract",
    "validator-implementation",
    "verdict-schema",
    "rpi-report-schema",
    "subject-manifest-schema",
}


class TransitionError(ValueError):
    pass


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value,
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    ).encode("utf-8")


def digest_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def valid_digest(value: Any) -> bool:
    return (
        isinstance(value, str)
        and len(value) == 64
        and all(character in HEX64 for character in value)
    )


def reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise TransitionError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_object(path: Path) -> dict[str, Any]:
    try:
        decoder = json.JSONDecoder(object_pairs_hook=reject_duplicate_keys)
        text = path.read_text(encoding="utf-8")
        value, end = decoder.raw_decode(text)
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        raise TransitionError(f"cannot read JSON object {path}: {error}") from error
    if text[end:].strip():
        raise TransitionError(f"trailing JSON data in {path}")
    if not isinstance(value, dict):
        raise TransitionError(f"expected JSON object: {path}")
    return value


def normalize_ref(root: Path, path: Path) -> str:
    try:
        relative = path.resolve().relative_to(root.resolve())
    except ValueError as error:
        raise TransitionError(f"artifact is outside repository root: {path}") from error
    value = PurePosixPath(relative.as_posix())
    if value.is_absolute() or ".." in value.parts or value.as_posix() in {"", "."}:
        raise TransitionError(f"invalid repository-relative artifact ref: {path}")
    return value.as_posix()


def tree_digest(root: Path) -> str:
    if not root.is_dir():
        raise TransitionError(f"qualification corpus is not a directory: {root}")
    entries: list[dict[str, Any]] = []
    for path in sorted(root.rglob("*")):
        if path.is_dir():
            continue
        relative = PurePosixPath(path.relative_to(root).as_posix())
        if relative.is_absolute() or ".." in relative.parts:
            raise TransitionError(f"qualification path escapes corpus: {path}")
        info = path.lstat()
        executable = bool(
            info.st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
        )
        if path.is_symlink():
            kind = "symlink"
            content = os.readlink(path).encode("utf-8")
        elif path.is_file():
            kind = "file"
            content = path.read_bytes()
        else:
            raise TransitionError(f"unsupported qualification artifact: {path}")
        entries.append(
            {
                "path": relative.as_posix(),
                "kind": kind,
                "executable": executable,
                "sha256": digest_bytes(content),
            }
        )
    if not entries:
        raise TransitionError("qualification corpus must contain at least one file")
    return digest_bytes(canonical_bytes(entries))


def validate_component(value: Any, index: int) -> tuple[str, str]:
    if not isinstance(value, dict) or set(value) != {"role", "ref", "digest", "mode"}:
        raise TransitionError(f"components[{index}] has invalid fields")
    role = value["role"]
    reference = value["ref"]
    digest = value["digest"]
    mode = value["mode"]
    if not isinstance(role, str) or not role:
        raise TransitionError(f"components[{index}].role must be nonempty")
    if not isinstance(reference, str) or not reference:
        raise TransitionError(f"components[{index}].ref must be nonempty")
    if not valid_digest(digest):
        raise TransitionError(f"components[{index}].digest must be SHA-256")
    if not isinstance(mode, str) or len(mode) != 4 or any(
        character not in "01234567" for character in mode
    ):
        raise TransitionError(f"components[{index}].mode must be four octal digits")
    return role, reference


def validate_descriptor(value: dict[str, Any], *, expected_epoch: int) -> None:
    if set(value) != DESCRIPTOR_KEYS:
        raise TransitionError("candidate proof descriptor has invalid fields")
    if value["schema_version"] != "proof-contract.v1":
        raise TransitionError("candidate descriptor must be proof-contract.v1")
    if value["epoch"] != expected_epoch:
        raise TransitionError(
            f"candidate epoch must be {expected_epoch}, got {value['epoch']!r}"
        )
    components = value["components"]
    if not isinstance(components, list) or not components:
        raise TransitionError("candidate descriptor components must be nonempty")
    roles: list[str] = []
    refs: list[str] = []
    for index, component in enumerate(components):
        role, reference = validate_component(component, index)
        roles.append(role)
        refs.append(reference)
    if len(roles) != len(set(roles)):
        raise TransitionError("candidate descriptor has duplicate component roles")
    if len(refs) != len(set(refs)):
        raise TransitionError("candidate descriptor has duplicate component refs")
    missing = sorted(REQUIRED_COMPONENT_ROLES - set(roles))
    if missing:
        raise TransitionError(
            "candidate descriptor lacks required component roles: "
            + ", ".join(missing)
        )
    corpus = value["qualification_corpus"]
    if not isinstance(corpus, dict) or set(corpus) != {"algorithm", "ref", "digest"}:
        raise TransitionError("candidate qualification_corpus has invalid fields")
    if corpus["algorithm"] != "sha256-tree-v1":
        raise TransitionError("candidate qualification corpus algorithm is unsupported")
    if not isinstance(corpus["ref"], str) or not corpus["ref"]:
        raise TransitionError("candidate qualification corpus ref must be nonempty")
    if not valid_digest(corpus["digest"]):
        raise TransitionError("candidate qualification corpus digest must be SHA-256")
    if not valid_digest(value["qualification_subject_manifest_digest"]):
        raise TransitionError(
            "candidate qualification_subject_manifest_digest must be SHA-256"
        )
    recorder = value["transition_recorder"]
    if not isinstance(recorder, dict) or set(recorder) != {"ref", "digest", "mode"}:
        raise TransitionError("candidate transition_recorder has invalid fields")
    if not isinstance(recorder["ref"], str) or not recorder["ref"]:
        raise TransitionError("candidate transition recorder ref must be nonempty")
    if not valid_digest(recorder["digest"]):
        raise TransitionError("candidate transition recorder digest must be SHA-256")
    if not isinstance(recorder["mode"], str) or len(recorder["mode"]) != 4 or any(
        character not in "01234567" for character in recorder["mode"]
    ):
        raise TransitionError("candidate transition recorder mode must be four octal digits")
    if not isinstance(value["known_gaps"], list) or any(
        not isinstance(item, str) or not item for item in value["known_gaps"]
    ):
        raise TransitionError("candidate known_gaps must contain nonempty strings")


def validate_active(value: dict[str, Any]) -> None:
    if set(value) != ACTIVE_KEYS:
        raise TransitionError("active proof pointer has invalid fields")
    if value["schema_version"] != "proof-contract-active.v1":
        raise TransitionError("active proof pointer has invalid schema_version")
    if value["epoch"] != 0:
        raise TransitionError("bootstrap recorder only accepts active epoch 0")
    if not isinstance(value["contract_ref"], str) or not value["contract_ref"]:
        raise TransitionError("active contract_ref must be nonempty")
    if not valid_digest(value["contract_digest"]):
        raise TransitionError("active contract_digest must be SHA-256")
    if value["activation_transition_ref"] is not None:
        raise TransitionError("epoch 0 must not have an activation transition ref")
    if value["activation_transition_digest"] is not None:
        raise TransitionError("epoch 0 must not have an activation transition digest")


def manifest_digest(value: dict[str, Any]) -> str:
    claimed = value.get("canonical_manifest_digest")
    if not valid_digest(claimed):
        raise TransitionError("qualification subject manifest digest is invalid")
    identity = {
        key: item
        for key, item in value.items()
        if key not in {"canonical_manifest_digest", "git_metadata"}
    }
    if digest_bytes(canonical_bytes(identity)) != claimed:
        raise TransitionError("qualification subject manifest digest does not match")
    return claimed


def verdict_digest(value: dict[str, Any]) -> str:
    if value.get("schema_version") != "verdict.v2":
        raise TransitionError("bootstrap qualification verdict must be verdict.v2")
    claimed = value.get("artifact_digest")
    if not valid_digest(claimed):
        raise TransitionError("qualification verdict artifact_digest is invalid")
    unsigned = {key: item for key, item in value.items() if key != "artifact_digest"}
    if digest_bytes(canonical_bytes(unsigned)) != claimed:
        raise TransitionError("qualification verdict artifact_digest does not match")
    if value.get("verdict") != "PASS":
        raise TransitionError("qualification verdict must PASS")
    author = value.get("author_context_id")
    validator = value.get("validator_context_id")
    if (
        not isinstance(author, str)
        or not author
        or not isinstance(validator, str)
        or not validator
        or author == validator
    ):
        raise TransitionError("qualification verdict requires distinct identities")
    freshness = value.get("freshness_attestation")
    if (
        not isinstance(freshness, dict)
        or freshness.get("source") not in {"runtime", "caller"}
        or not isinstance(freshness.get("attester_identity"), str)
        or not freshness["attester_identity"]
    ):
        raise TransitionError("qualification verdict lacks freshness attestation")
    return claimed


def parse_time(value: str) -> str:
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as error:
        raise TransitionError("activated-at must be an RFC3339 date-time") from error
    if parsed.tzinfo is None:
        raise TransitionError("activated-at must include a timezone")
    return value


def atomic_write(path: Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=path.parent
    )
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        parent_descriptor = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(parent_descriptor)
        finally:
            os.close(parent_descriptor)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def activate(args: argparse.Namespace) -> dict[str, Any]:
    repository = Path(args.repository).resolve()
    active_path = Path(args.active_pointer).resolve()
    candidate_path = Path(args.candidate_descriptor).resolve()
    manifest_path = Path(args.candidate_manifest).resolve()
    corpus_path = Path(args.qualification_corpus).resolve()
    verdict_path = Path(args.qualification_verdict).resolve()
    transitions_dir = Path(args.transitions_dir).resolve()

    for path in (active_path, candidate_path, manifest_path, corpus_path, verdict_path):
        normalize_ref(repository, path)
    normalize_ref(repository, transitions_dir)
    if not valid_digest(args.expected_prior_digest):
        raise TransitionError("expected-prior-digest must be SHA-256")
    if not isinstance(args.validator_id, str) or not args.validator_id.strip():
        raise TransitionError("validator-id must be nonempty")

    lock_path = active_path.with_suffix(active_path.suffix + ".lock")
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a+b") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        active = load_object(active_path)
        validate_active(active)
        if active["contract_digest"] != args.expected_prior_digest:
            raise TransitionError("active proof contract changed; CAS refused")
        prior_contract_path = repository / active["contract_ref"]
        if digest_bytes(prior_contract_path.read_bytes()) != active["contract_digest"]:
            raise TransitionError("active proof descriptor bytes do not match pointer")

        candidate = load_object(candidate_path)
        validate_descriptor(candidate, expected_epoch=1)
        candidate_digest = digest_bytes(candidate_path.read_bytes())
        if candidate_digest == active["contract_digest"]:
            raise TransitionError("candidate proof contract cannot activate itself")

        corpus_digest = tree_digest(corpus_path)
        if candidate["qualification_corpus"]["ref"] != normalize_ref(
            repository, corpus_path
        ):
            raise TransitionError("candidate qualification corpus ref does not match")
        if candidate["qualification_corpus"]["digest"] != corpus_digest:
            raise TransitionError("candidate qualification corpus digest does not match")

        subject_manifest = load_object(manifest_path)
        subject_digest = manifest_digest(subject_manifest)
        if candidate["qualification_subject_manifest_digest"] != subject_digest:
            raise TransitionError("candidate qualification subject digest does not match")

        verdict = load_object(verdict_path)
        qualification_digest = verdict_digest(verdict)
        if verdict.get("subject_manifest_digest") != subject_digest:
            raise TransitionError("qualification verdict judged a different subject")
        if verdict_path.name != f"{qualification_digest}.json":
            raise TransitionError("qualification verdict filename does not bind its digest")

        transition = {
            "schema_version": "proof-contract-transition.v1",
            "prior": {
                "epoch": 0,
                "contract_ref": active["contract_ref"],
                "contract_digest": active["contract_digest"],
                "activation_transition_digest": None,
            },
            "candidate": {
                "epoch": 1,
                "contract_ref": normalize_ref(repository, candidate_path),
                "contract_digest": candidate_digest,
                "subject_manifest_ref": normalize_ref(repository, manifest_path),
                "subject_manifest_digest": subject_digest,
                "qualification_corpus_ref": normalize_ref(repository, corpus_path),
                "qualification_corpus_digest": corpus_digest,
            },
            "qualification_verdict": {
                "ref": normalize_ref(repository, verdict_path),
                "digest": qualification_digest,
            },
            "validator_identity": args.validator_id,
            "activated_at": parse_time(args.activated_at),
        }
        transition_payload = canonical_bytes(transition) + b"\n"
        transition_digest = digest_bytes(transition_payload)
        transition_path = transitions_dir / f"{transition_digest}.json"
        if transition_path.exists():
            if transition_path.read_bytes() != transition_payload:
                raise TransitionError("transition digest collision")
        else:
            atomic_write(transition_path, transition_payload)

        next_active = {
            "schema_version": "proof-contract-active.v1",
            "epoch": 1,
            "contract_ref": normalize_ref(repository, candidate_path),
            "contract_digest": candidate_digest,
            "activation_transition_ref": normalize_ref(repository, transition_path),
            "activation_transition_digest": transition_digest,
        }
        atomic_write(active_path, canonical_bytes(next_active) + b"\n")
        return {
            "result": "ACTIVATED",
            "epoch": 1,
            "contract_digest": candidate_digest,
            "transition_digest": transition_digest,
            "transition_ref": normalize_ref(repository, transition_path),
        }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--active-pointer", required=True)
    parser.add_argument("--candidate-descriptor", required=True)
    parser.add_argument("--candidate-manifest", required=True)
    parser.add_argument("--qualification-corpus", required=True)
    parser.add_argument("--qualification-verdict", required=True)
    parser.add_argument("--transitions-dir", required=True)
    parser.add_argument("--expected-prior-digest", required=True)
    parser.add_argument("--validator-id", required=True)
    parser.add_argument("--activated-at", required=True)
    return parser.parse_args()


def main() -> int:
    try:
        result = activate(parse_args())
    except (OSError, TransitionError) as error:
        print(f"bootstrap-proof-transition: {error}", file=os.sys.stderr)
        return 2
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
