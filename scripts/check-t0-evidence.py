#!/usr/bin/env python3
"""Verify the immutable T0 snapshot and its live proof-transition bindings."""

from __future__ import annotations

import argparse
from collections import Counter
from datetime import datetime
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath
import re
import stat
import sys
from typing import Any


class EvidenceError(ValueError):
    pass


PAUSE_LINEAGE = {
    "origin_main_base_commit": "edc018f6e15a00648736bed84c2ea93265443094",
    "seed_commit": "b6794d76e62c4aca2ba682474090b6de0026a0f3",
    "initial_candidate_commit": "9321872587afcf77ac9cd0b3e7aa758704a564e4",
    "initial_fail_artifact_digest": "bf865e3233c1e19e6346d37403db775e9fb0fa6b252d14af88e4c9aaa081d804",
    "initial_fail_file_sha256": "2eb46f2da1ff1c5671725bd949efd4975961b5db60778245dcce3a0c55b11a8d",
    "initial_failed_subject_file_sha256": "4cbbdfbfed3c5a3af2e5d0d050a8899c77db5c0e98e1c9c6a92489fd713ab7db",
    "initial_failed_report_file_sha256": "1f97efd1754ae72e135d3e23136a654eaa09ac571e94ddce47d934171489d700",
    "initial_failure_evidence_commit": "50e20ed07d9dd612add428cd3b5530df0e82d821",
    "repair_candidate_commit": "0e4bc0d90575593cb4c3a048fc0cb69e96837ee5",
    "repair_fail_artifact_digest": "3c297141dd11978fc3c741733773373a57028b88f24e73b24df5c55fa4e932f7",
    "repair_fail_file_sha256": "a515fd4abc6a15125889c7d05518e9b5f64dd0eb27260c450c950284f1034a54",
    "repair_failed_subject_file_sha256": "8a1201dc2a9b74210c81db273d8e973994a4023a7895b6476dab03e049c2bd7c",
    "repair_failed_report_file_sha256": "f04f821a65979b99dbe72e73cf9b9d79e7094ccc29292e1596dad3bebaec0153",
    "repair_failure_evidence_commit": "45268dbeb900ec9a99a51eb9cf451573371ff175",
    "pause_candidate_commit": "b5b5f561785d8f41d1d18eb7e79d3284bebeff80",
    "pause_fail_artifact_digest": "b27793442f1e1067b9ab6d22dec797b263d4dcb93e080ab8b93ff2c4da6695e2",
    "pause_fail_file_sha256": "27eddbc7b4e25d088ec63c421f7b71de061248d19b333e23771593c17095c4e3",
    "pause_failed_subject_file_sha256": "3bad6ac553eb72694ca760f7e36c316d1a6183e9d38b14d37f7514846a2f3b3c",
    "pause_failed_report_file_sha256": "d32e39fc01b39a3e2747804bbd45c37af2590349d88a84e2add8385be1eb8a35",
    "pause_failure_evidence_commit": "60485a4767bababd1994262043a5e73a03d15b6e",
    "typed_pause_candidate_commit": "3dd99abbc76b079b69c97cda26b9706f1e8bd6e1",
    "typed_pause_fail_artifact_digest": "bc97dc05ced93855a0e2326f5ddd92dc4814db9b114158e8acc5298e63051d5b",
    "typed_pause_fail_file_sha256": "5df88394b4755c3629e43d8f8eff599019cded4ef80e84feadea48dc0d4d967c",
    "typed_pause_failed_subject_file_sha256": "424f339286de1b91dbc452f3a34b742d8bd364dd04d57c6c88e91bf8bb04a57b",
    "typed_pause_failed_report_file_sha256": "851cfc9a9597f65298dafcbfa57713b1577bc0d65e6a6abd9ebaafccb9811972",
    "typed_pause_failure_evidence_commit": "eccb1abf180d5ee79d36798d2e40c1eb67cf2ff6",
    "transition_schema_candidate_commit": "759c78de1aa1bfa93a36b3231060e5a1c83fdbcc",
    "transition_schema_terminal_artifact_digest": "4255446100319be16e31553e51278d94f029d6b4942a4f90a27ae9293c47978e",
    "transition_schema_terminal_file_sha256": "dd1c500ba8b10797e713a87fbe8611709efff4cc1dbbf54abf9c7605821e7e92",
    "transition_schema_terminal_subject_file_sha256": "393446cf840cc407a6426dac25aab5417a162d2f40da635d020e0b28699d4e8f",
    "transition_schema_terminal_report_file_sha256": "33abb5dce6fb815a2680625e9d57019957cb5ea422cf353bb3f79fe2fb4b3c65",
    "transition_schema_terminal_evidence_commit": "1016ae749f1656be692b159f1e13ceb9e868c28b",
    "current_invocation": "docs/evidence/proof-epochs/epoch-0b/t0-ref-safety-repair-intent.md",
    "current_candidate_identity": "enclosing subject manifest and fresh verdict",
}

PAUSE_PROOF_REF = "docs/contracts/proof-contracts/epoch-0b/descriptor.json"
PAUSE_PROOF_DIGEST = "b9735c94f7de98c4d31db93081351a6a1a78f8e03897dad61792277bb36a0302"
TRANSITION_SCHEMA_REF = "schemas/proof-contract-transition.v1.schema.json"
TRANSITION_SCHEMA_DIGEST = "854d252c7af4feb83cfce211edcd2eacda3b1741eca08f6f38a5eaa638596ce3"
ACTIVE_SCHEMA_REF = "schemas/proof-contract-active.v1.schema.json"
ACTIVE_SCHEMA_DIGEST = "0110acf9131aa09e4ace6a12fe58a79c6b59c2cffa405601186573d2d3204282"
PROOF_SCHEMA_REF = "schemas/proof-contract.v1.schema.json"
PROOF_SCHEMA_DIGEST = "22ecac3c127779aeef265b63e0c68405f344f67b8e02dcaf2686c3b9a417e87a"
SUBJECT_SCHEMA_REF = "schemas/subject-manifest.v1.schema.json"
SUBJECT_SCHEMA_DIGEST = "bd2816ef184d080041d703f834973e55bf9ee9ef7f35aaf7648ac56baa38afb3"
VERDICT_SCHEMA_REF = "schemas/verdict.v2.schema.json"
VERDICT_SCHEMA_DIGEST = "848af783786a33ee505d3a1e19afdb1b97c6875d68c4fe817a21ca012bdfedab"
T0_SKILL_LEDGER_FILE_SHA256 = (
    "e60f91f6dbd8956bf2a8d049ca8ead5516cd36f035859939163af9176c1f3a85"
)
PAUSE_TOP_LEVEL_KEYS = {
    "schema_version",
    "performed_on",
    "fresh_context",
    "result",
    "landed_or_frozen",
    "progress",
    "lineage",
    "authority",
    "safe_stop_claim",
    "known_gaps",
}
PAUSE_LANDED_OR_FROZEN = [
    "origin/main base edc018f6e",
    "seed plan/audit commit b6794d76e",
    "stable initial T0 candidate commit 932187258",
    f"immutable T0 FAIL {PAUSE_LINEAGE['initial_fail_artifact_digest']}",
    "initial T0 failure-evidence commit 50e20ed07",
    "stable T0 repair candidate commit 0e4bc0d90",
    f"immutable T0 repair FAIL {PAUSE_LINEAGE['repair_fail_artifact_digest']}",
    "T0 repair failure-evidence commit 45268dbeb",
    "stable T0 pause-repair candidate commit b5b5f5617",
    f"immutable T0 pause-repair FAIL {PAUSE_LINEAGE['pause_fail_artifact_digest']}",
    "T0 pause-repair failure-evidence commit 60485a476",
    "stable T0 typed-pause candidate commit 3dd99abbc",
    f"immutable T0 typed-pause FAIL {PAUSE_LINEAGE['typed_pause_fail_artifact_digest']}",
    "T0 typed-pause failure-evidence commit eccb1abf1",
    "stable T0 transition-schema candidate commit 759c78de1",
    f"immutable T0 transition-schema NOT_PROVEN {PAUSE_LINEAGE['transition_schema_terminal_artifact_digest']}",
    "T0 transition-schema terminal-evidence commit 1016ae749",
    "49-skill exact-byte ledger",
    "rejected epoch-0 proof descriptor retained as history",
    "active corrected epoch-0b proof descriptor and strict bootstrap transition recorder",
    "T0 gate liveness and proof-chain ledgers",
    "routing oracle baseline",
    "installed-estate and #988 reconciliation",
]
PAUSE_PROGRESS = {
    "current_invocation_ref": PAUSE_LINEAGE["current_invocation"],
    "current_candidate_identity": PAUSE_LINEAGE["current_candidate_identity"],
    "states": {
        "T0": "VALIDATION_PENDING",
        "T1": "NOT_STARTED",
        "T2": "NOT_STARTED",
        "T3": "NOT_STARTED",
        "T4": "NOT_STARTED",
        "T5": "NOT_STARTED",
        "T6": "NOT_STARTED",
        "T7a": "NOT_STARTED",
        "T7b": "NOT_STARTED",
        "T8": "NOT_STARTED",
        "G0": "NOT_STARTED",
        "G1": "NOT_STARTED",
        "G2": "NOT_STARTED",
    },
    "reconnaissance_complete": ["T1", "T2", "G0"],
}
PAUSE_AUTHORITY = {
    "semantic_skill_source": "skills/<slug>/SKILL.md",
    "generated_projection_owner": "scripts/regen-all.sh and owning generators",
    "proof_contract_pointer": "docs/contracts/proof-contracts/active.json",
    "proof_contract_ref": PAUSE_PROOF_REF,
    "proof_contract_digest": PAUSE_PROOF_DIGEST,
    "migration_plan": "docs/plans/2026-07-24-skill-system-overhaul.md",
    "landing_policy": "caller-authorized feature branch integration to origin/main",
}
PAUSE_SAFE_STOP_CLAIM = (
    "This is a historical T0 snapshot: API1 and catalog v3 are live, epoch 0b "
    "is the bootstrap authority, and skill-contract.v3, catalog v4, verdict.v3, "
    "T1 through T8, and G0 through G2 are not implemented."
)
PAUSE_KNOWN_GAPS = [
    "CASS historical routing search unavailable because checkpoint is incomplete",
    "remote required-CI job wiring not proven",
    "installed skill roots still target the preserved original worktree",
    "T0 reference-safety repair requires a fresh semantic verdict over its exact subject",
    "T1 exact-kernel implementation and epoch-1 activation have not started",
]


def reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        require(key not in result, f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load(path: Path) -> dict[str, Any]:
    value = json.loads(
        path.read_text(encoding="utf-8"),
        object_pairs_hook=reject_duplicate_keys,
    )
    if not isinstance(value, dict):
        raise EvidenceError(f"expected JSON object: {path}")
    return value


def require(condition: bool, message: str) -> None:
    if not condition:
        raise EvidenceError(message)


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def file_mode(path: Path) -> str:
    return f"{stat.S_IMODE(path.lstat().st_mode):04o}"


def canonical_digest(value: Any) -> str:
    payload = json.dumps(
        value,
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    ).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def is_rfc3339_datetime(value: Any) -> bool:
    if not isinstance(value, str) or not re.fullmatch(
        r"\d{4}-\d{2}-\d{2}[Tt]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[Zz]|[+-]\d{2}:\d{2})",
        value,
    ):
        return False
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00").replace("z", "+00:00"))
    except ValueError:
        return False
    return parsed.tzinfo is not None


def check_failed_invocation(
    *,
    verdict_path: Path,
    subject_path: Path,
    report_path: Path,
    artifact_digest: str,
    verdict_file_sha256: str,
    subject_file_sha256: str,
    report_file_sha256: str,
    expected_result: str = "FAIL",
) -> None:
    require(sha256(verdict_path) == verdict_file_sha256, f"failed verdict bytes changed: {verdict_path}")
    require(sha256(subject_path) == subject_file_sha256, f"failed subject bytes changed: {subject_path}")
    require(sha256(report_path) == report_file_sha256, f"failed report bytes changed: {report_path}")
    verdict = load(verdict_path)
    subject = load(subject_path)
    report = load(report_path)
    require(
        verdict.get("verdict") == expected_result,
        f"terminal verdict is not {expected_result}: {verdict_path}",
    )
    require(
        verdict.get("artifact_digest") == artifact_digest,
        f"rejected verdict artifact identity changed: {verdict_path}",
    )
    require(
        verdict.get("subject_manifest_digest") == subject.get("canonical_manifest_digest"),
        f"rejected verdict and subject do not agree: {verdict_path}",
    )
    require(
        report.get("status") == expected_result,
        f"terminal report is not {expected_result}: {report_path}",
    )
    require(
        report.get("verdict_digest") == artifact_digest
        and report.get("subject_manifest_digest") == subject.get("canonical_manifest_digest"),
        f"rejected report lineage changed: {report_path}",
    )


def resolve_repository_ref(repository: Path, reference: Any, label: str) -> Path:
    require(isinstance(reference, str) and bool(reference), f"{label} must be a nonempty reference")
    require("\\" not in reference and "\x00" not in reference, f"{label} is not a normalized POSIX reference")
    lexical = PurePosixPath(reference)
    require(
        not lexical.is_absolute()
        and lexical.as_posix() == reference
        and reference not in {".", ".."}
        and all(part not in {"", ".", ".."} for part in lexical.parts),
        f"{label} is not a normalized repository-relative reference",
    )
    root = repository.resolve()
    lexical_path = root
    for part in lexical.parts:
        lexical_path = lexical_path / part
        require(
            not lexical_path.is_symlink(),
            f"{label} uses a symlinked path component: {reference}",
        )
    path = lexical_path.resolve()
    try:
        path.relative_to(root)
    except ValueError as exc:
        raise EvidenceError(f"{label} escapes the repository") from exc
    return path


def validate_schema_instance(
    repository: Path,
    *,
    schema_ref: str,
    schema_digest: str,
    instance: dict[str, Any],
    label: str,
) -> None:
    try:
        from jsonschema import Draft202012Validator, FormatChecker
        from jsonschema.exceptions import SchemaError
    except ImportError as exc:
        raise EvidenceError("python jsonschema is required for proof validation") from exc
    schema_path = resolve_repository_ref(repository, schema_ref, f"{label} schema")
    require(sha256(schema_path) == schema_digest, f"{label} schema bytes changed")
    schema = load(schema_path)
    try:
        Draft202012Validator.check_schema(schema)
    except SchemaError as exc:
        raise EvidenceError(f"{label} schema is invalid: {exc.message}") from exc
    format_checker = FormatChecker()
    format_checker.checks("date-time")(is_rfc3339_datetime)
    validator = Draft202012Validator(schema, format_checker=format_checker)
    errors = sorted(
        validator.iter_errors(instance),
        key=lambda error: tuple(str(part) for part in error.absolute_path),
    )
    require(not errors, f"{label} violates its schema: {errors[0].message if errors else ''}")


def load_bootstrap_module(repository: Path) -> Any:
    path = resolve_repository_ref(
        repository,
        "scripts/bootstrap-proof-transition.py",
        "bootstrap transition recorder",
    )
    specification = importlib.util.spec_from_file_location(
        "agentops_t0_evidence_bootstrap",
        path,
    )
    require(
        specification is not None and specification.loader is not None,
        "cannot load bootstrap transition recorder",
    )
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


def check_epoch1_transition(
    repository: Path,
    active: dict[str, Any],
    transition_path: Path,
    transition: dict[str, Any],
) -> None:
    validate_schema_instance(
        repository,
        schema_ref=TRANSITION_SCHEMA_REF,
        schema_digest=TRANSITION_SCHEMA_DIGEST,
        instance=transition,
        label="active transition",
    )
    candidate = transition["candidate"]
    require(
        candidate["epoch"] == active["epoch"]
        and candidate["contract_ref"] == active["contract_ref"]
        and candidate["contract_digest"] == active["contract_digest"],
        "active pointer is not bound to the transition candidate",
    )
    require(
        transition["prior"]
        == {
            "epoch": 0,
            "contract_ref": PAUSE_PROOF_REF,
            "contract_digest": PAUSE_PROOF_DIGEST,
            "activation_transition_digest": None,
        },
        "active transition is not descended from the pause authority",
    )

    bootstrap = load_bootstrap_module(repository)
    descriptor_path = resolve_repository_ref(
        repository,
        candidate["contract_ref"],
        "active proof descriptor",
    )
    require(
        sha256(descriptor_path) == candidate["contract_digest"],
        "active proof descriptor bytes do not match the transition",
    )
    descriptor = load(descriptor_path)
    validate_schema_instance(
        repository,
        schema_ref=PROOF_SCHEMA_REF,
        schema_digest=PROOF_SCHEMA_DIGEST,
        instance=descriptor,
        label="active proof descriptor",
    )
    try:
        bootstrap.validate_descriptor(descriptor, expected_epoch=1)
    except ValueError as exc:
        raise EvidenceError(f"active proof descriptor is invalid: {exc}") from exc

    subject_path = resolve_repository_ref(
        repository,
        candidate["subject_manifest_ref"],
        "qualification subject manifest",
    )
    subject = load(subject_path)
    validate_schema_instance(
        repository,
        schema_ref=SUBJECT_SCHEMA_REF,
        schema_digest=SUBJECT_SCHEMA_DIGEST,
        instance=subject,
        label="qualification subject manifest",
    )
    try:
        subject_digest = bootstrap.manifest_digest(subject)
    except ValueError as exc:
        raise EvidenceError(f"qualification subject manifest is invalid: {exc}") from exc
    require(
        subject_digest == candidate["subject_manifest_digest"]
        and descriptor["qualification_subject_manifest_digest"] == subject_digest,
        "qualification subject manifest is not bound to the transition and descriptor",
    )
    subject_entries = {
        entry.get("path"): entry
        for entry in subject.get("entries", [])
        if isinstance(entry, dict) and isinstance(entry.get("path"), str)
    }
    require(
        len(subject_entries) == len(subject.get("entries", [])),
        "qualification subject manifest has duplicate or invalid entries",
    )
    frozen_items = list(descriptor["components"]) + [descriptor["transition_recorder"]]
    for index, item in enumerate(frozen_items):
        reference = item["ref"]
        path = resolve_repository_ref(repository, reference, f"frozen candidate item {index}")
        require(path.is_file() and not path.is_symlink(), f"frozen candidate item is missing: {reference}")
        require(sha256(path) == item["digest"], f"frozen candidate item bytes changed: {reference}")
        require(file_mode(path) == item["mode"], f"frozen candidate item mode changed: {reference}")
        subject_entry = subject_entries.get(reference)
        require(
            isinstance(subject_entry, dict)
            and subject_entry.get("kind") == "file"
            and subject_entry.get("digest") == item["digest"]
            and subject_entry.get("executable")
            == bool(path.lstat().st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)),
            f"frozen candidate item is not bound to the qualification subject: {reference}",
        )

    corpus_path = resolve_repository_ref(
        repository,
        candidate["qualification_corpus_ref"],
        "qualification corpus",
    )
    try:
        corpus_digest = bootstrap.tree_digest(corpus_path)
    except ValueError as exc:
        raise EvidenceError(f"qualification corpus is invalid: {exc}") from exc
    require(
        corpus_digest == candidate["qualification_corpus_digest"]
        and descriptor["qualification_corpus"]["ref"]
        == candidate["qualification_corpus_ref"]
        and descriptor["qualification_corpus"]["digest"] == corpus_digest,
        "qualification corpus is not bound to the transition and descriptor",
    )

    qualification = transition["qualification_verdict"]
    verdict_path = resolve_repository_ref(
        repository,
        qualification["ref"],
        "qualification verdict",
    )
    verdict = load(verdict_path)
    validate_schema_instance(
        repository,
        schema_ref=VERDICT_SCHEMA_REF,
        schema_digest=VERDICT_SCHEMA_DIGEST,
        instance=verdict,
        label="qualification verdict",
    )
    try:
        verdict_digest = bootstrap.verdict_digest(verdict)
    except ValueError as exc:
        raise EvidenceError(f"qualification verdict is invalid: {exc}") from exc
    require(
        verdict_digest == qualification["digest"]
        and verdict_path.name == f"{verdict_digest}.json",
        "qualification verdict bytes or filename do not match the transition",
    )
    require(
        verdict["subject_manifest_digest"] == subject_digest,
        "qualification verdict judged a different subject",
    )
    require(
        transition["validator_identity"] == verdict["validator_context_id"],
        "transition validator identity does not match the qualification verdict",
    )
    activation_time = datetime.fromisoformat(
        transition["activated_at"].replace("Z", "+00:00").replace("z", "+00:00")
    )
    validation_time = datetime.fromisoformat(
        verdict["validated_at"].replace("Z", "+00:00").replace("z", "+00:00")
    )
    require(
        activation_time >= validation_time,
        "transition activation predates the qualification verdict",
    )
    require(
        transition_path.name == f"{active['activation_transition_digest']}.json",
        "active transition filename changed during validation",
    )


def check_pause_state(repository: Path, evidence_root: Path, pause: dict[str, Any]) -> None:
    require(set(pause) == PAUSE_TOP_LEVEL_KEYS, "pause drill fields are not closed-world")
    require(
        pause.get("schema_version") == "agentops.t0-pause-drill.v1",
        "pause drill schema version changed",
    )
    require(pause.get("performed_on") == "2026-07-24", "pause drill date changed")
    require(pause.get("fresh_context") == "native Codex subagent", "pause drill context changed")
    require(pause.get("result") == "PASS", "pause drill is not PASS")
    require(
        pause.get("landed_or_frozen") == PAUSE_LANDED_OR_FROZEN,
        "pause drill landed/frozen state is missing or contradictory",
    )
    require(pause.get("progress") == PAUSE_PROGRESS, "pause drill typed progress is missing or contradictory")
    require(pause.get("lineage") == PAUSE_LINEAGE, "pause drill lineage is missing or inaccurate")
    require(pause.get("authority") == PAUSE_AUTHORITY, "pause drill authority is missing or inaccurate")
    require(
        pause.get("safe_stop_claim") == PAUSE_SAFE_STOP_CLAIM,
        "pause drill safe-stop claim is missing or contradictory",
    )
    require(pause.get("known_gaps") == PAUSE_KNOWN_GAPS, "pause drill known gaps are missing or contradictory")

    pause_descriptor = resolve_repository_ref(repository, PAUSE_PROOF_REF, "pause proof descriptor")
    require(sha256(pause_descriptor) == PAUSE_PROOF_DIGEST, "pause drill proof descriptor bytes changed")
    active_path = resolve_repository_ref(
        repository,
        PAUSE_AUTHORITY["proof_contract_pointer"],
        "active proof pointer",
    )
    active = load(active_path)
    validate_schema_instance(
        repository,
        schema_ref=ACTIVE_SCHEMA_REF,
        schema_digest=ACTIVE_SCHEMA_DIGEST,
        instance=active,
        label="active proof pointer",
    )
    require(
        set(active)
        == {
            "schema_version",
            "epoch",
            "contract_ref",
            "contract_digest",
            "activation_transition_ref",
            "activation_transition_digest",
        }
        and active.get("schema_version") == "proof-contract-active.v1",
        "live proof pointer has invalid fields",
    )
    if active.get("epoch") == 0:
        require(
            active.get("contract_ref") == PAUSE_PROOF_REF
            and active.get("contract_digest") == PAUSE_PROOF_DIGEST
            and active.get("activation_transition_ref") is None
            and active.get("activation_transition_digest") is None,
            "live epoch-0 proof pointer does not match the pause authority",
        )
    else:
        require(active.get("epoch") == 1, "pause checker is not qualified beyond proof epoch 1")
        transition_path = resolve_repository_ref(
            repository,
            active.get("activation_transition_ref"),
            "active transition",
        )
        transition_digest = active.get("activation_transition_digest")
        require(
            isinstance(transition_digest, str)
            and sha256(transition_path) == transition_digest
            and transition_path.name == f"{transition_digest}.json",
            "active transition bytes or filename do not match the pointer",
        )
        transition = load(transition_path)
        check_epoch1_transition(repository, active, transition_path, transition)

    require(
        resolve_repository_ref(
            repository,
            PAUSE_LINEAGE["current_invocation"],
            "pause current invocation",
        ).is_file(),
        "pause drill current invocation is missing",
    )
    epoch0b = repository / "docs/evidence/proof-epochs/epoch-0b"
    check_failed_invocation(
        verdict_path=evidence_root
        / "verdicts"
        / f"{PAUSE_LINEAGE['initial_fail_artifact_digest']}.json",
        subject_path=evidence_root / "t0-failed-subject-manifest.json",
        report_path=evidence_root / "t0-failed-report.json",
        artifact_digest=PAUSE_LINEAGE["initial_fail_artifact_digest"],
        verdict_file_sha256=PAUSE_LINEAGE["initial_fail_file_sha256"],
        subject_file_sha256=PAUSE_LINEAGE["initial_failed_subject_file_sha256"],
        report_file_sha256=PAUSE_LINEAGE["initial_failed_report_file_sha256"],
    )
    check_failed_invocation(
        verdict_path=epoch0b
        / "verdicts"
        / f"{PAUSE_LINEAGE['repair_fail_artifact_digest']}.json",
        subject_path=epoch0b / "t0r-failed-subject-manifest.json",
        report_path=epoch0b / "t0r-failed-report.json",
        artifact_digest=PAUSE_LINEAGE["repair_fail_artifact_digest"],
        verdict_file_sha256=PAUSE_LINEAGE["repair_fail_file_sha256"],
        subject_file_sha256=PAUSE_LINEAGE["repair_failed_subject_file_sha256"],
        report_file_sha256=PAUSE_LINEAGE["repair_failed_report_file_sha256"],
    )
    check_failed_invocation(
        verdict_path=epoch0b
        / "verdicts"
        / f"{PAUSE_LINEAGE['pause_fail_artifact_digest']}.json",
        subject_path=epoch0b / "t0p-failed-subject-manifest.json",
        report_path=epoch0b / "t0p-failed-report.json",
        artifact_digest=PAUSE_LINEAGE["pause_fail_artifact_digest"],
        verdict_file_sha256=PAUSE_LINEAGE["pause_fail_file_sha256"],
        subject_file_sha256=PAUSE_LINEAGE["pause_failed_subject_file_sha256"],
        report_file_sha256=PAUSE_LINEAGE["pause_failed_report_file_sha256"],
    )
    check_failed_invocation(
        verdict_path=epoch0b
        / "verdicts"
        / f"{PAUSE_LINEAGE['typed_pause_fail_artifact_digest']}.json",
        subject_path=epoch0b / "t0pp-failed-subject-manifest.json",
        report_path=epoch0b / "t0pp-failed-report.json",
        artifact_digest=PAUSE_LINEAGE["typed_pause_fail_artifact_digest"],
        verdict_file_sha256=PAUSE_LINEAGE["typed_pause_fail_file_sha256"],
        subject_file_sha256=PAUSE_LINEAGE["typed_pause_failed_subject_file_sha256"],
        report_file_sha256=PAUSE_LINEAGE["typed_pause_failed_report_file_sha256"],
    )
    check_failed_invocation(
        verdict_path=epoch0b
        / "verdicts"
        / f"{PAUSE_LINEAGE['transition_schema_terminal_artifact_digest']}.json",
        subject_path=epoch0b / "t0ps-not-proven-subject-manifest.json",
        report_path=epoch0b / "t0ps-not-proven-report.json",
        artifact_digest=PAUSE_LINEAGE["transition_schema_terminal_artifact_digest"],
        verdict_file_sha256=PAUSE_LINEAGE["transition_schema_terminal_file_sha256"],
        subject_file_sha256=PAUSE_LINEAGE[
            "transition_schema_terminal_subject_file_sha256"
        ],
        report_file_sha256=PAUSE_LINEAGE[
            "transition_schema_terminal_report_file_sha256"
        ],
        expected_result="NOT_PROVEN",
    )


def check(repository: Path, evidence_root: Path) -> dict[str, Any]:
    skill_ledger_path = evidence_root / "t0-skill-ledger.json"
    require(
        sha256(skill_ledger_path) == T0_SKILL_LEDGER_FILE_SHA256,
        "T0 skill ledger bytes changed",
    )
    skills = load(skill_ledger_path)
    rows = skills.get("skills")
    require(isinstance(rows, list), "skill ledger rows must be an array")
    require(skills.get("skill_count") == 49 == len(rows), "skill ledger must contain exactly 49 rows")
    paths = [row.get("path") for row in rows if isinstance(row, dict)]
    require(len(paths) == len(set(paths)), "skill ledger paths must be unique")
    for row in rows:
        require(
            isinstance(row, dict)
            and isinstance(row.get("path"), str)
            and isinstance(row.get("sha256"), str),
            "skill ledger row has invalid fields",
        )
        require(
            len(row["sha256"]) == 64
            and all(character in "0123456789abcdef" for character in row["sha256"]),
            f"skill ledger digest is invalid: {row['path']}",
        )

    routing = load(evidence_root / "routing-baseline.json")
    scenarios = routing.get("scenarios")
    require(isinstance(scenarios, list) and len(scenarios) >= 30, "routing corpus is too small")
    identifiers = [case.get("id") for case in scenarios if isinstance(case, dict)]
    require(len(identifiers) == len(set(identifiers)), "routing scenario IDs must be unique")
    allowed_decisions = set(routing.get("decision_values", []))
    allowed_assessments = set(routing.get("assessment_values", []))
    for case in scenarios:
        require(isinstance(case, dict), "routing scenario must be an object")
        require(case.get("expected", {}).get("decision") in allowed_decisions, f"invalid routing decision: {case.get('id')}")
        require(case.get("assessment") in allowed_assessments, f"invalid routing assessment: {case.get('id')}")
    observed = Counter(case["assessment"] for case in scenarios)
    require(
        dict(sorted(observed.items()))
        == dict(sorted(routing.get("observation", {}).get("summary", {}).items())),
        "routing assessment summary does not match scenarios",
    )

    liveness = load(evidence_root / "t0-check-liveness.json")
    checks = liveness.get("checks")
    require(isinstance(checks, list) and checks, "check-liveness ledger is empty")
    for item in checks:
        if item.get("load_bearing"):
            require(item.get("status") == "GREEN", f"load-bearing check is not GREEN: {item.get('id')}")
            require(bool(item.get("negative_witness")), f"load-bearing check lacks a negative witness: {item.get('id')}")
            witness_path = str(item["negative_witness"]).split()[0]
            require(
                (repository / witness_path).exists(),
                f"load-bearing negative witness is missing: {item.get('id')}: {witness_path}",
            )

    chain = load(evidence_root / "t0-proof-chain.json")
    classifications = set(chain.get("classification", []))
    require(
        classifications == {"USED_SOUND", "USED_UNSOUND", "PROVEN_UNUSED", "UNKNOWN"},
        "proof-chain classification set is invalid",
    )
    for edge in chain.get("edges", []):
        require(edge.get("status") in classifications, "proof-chain edge has invalid status")

    reconciliation = load(evidence_root / "t0-worktree-reconciliation.json")
    commit = reconciliation.get("commit_16d764b5a", {})
    counts = commit.get("counts", {})
    require(
        counts.get("PRESENT_IDENTICAL") == len(commit.get("PRESENT_IDENTICAL", [])),
        "#988 identical count does not match path list",
    )
    require(
        counts.get("PRESENT_DIVERGED") == len(commit.get("PRESENT_DIVERGED", [])),
        "#988 divergent count does not match path list",
    )

    pause = load(evidence_root / "t0-pause-drill.json")
    check_pause_state(repository, evidence_root, pause)
    estate = load(evidence_root / "t0-installed-estate.json")
    require(len(estate.get("source_links", [])) == 4, "installed-estate link-root count changed")

    return {
        "result": "PASS",
        "skill_count": len(rows),
        "routing_scenario_count": len(scenarios),
        "load_bearing_check_count": sum(
            1 for item in checks if item.get("load_bearing")
        ),
        "proof_edge_count": len(chain.get("edges", [])),
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", default=".")
    parser.add_argument(
        "--evidence-root",
        default="docs/evidence/proof-epochs/epoch-0",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    repository = Path(args.repository).resolve()
    evidence_root = Path(args.evidence_root)
    if not evidence_root.is_absolute():
        evidence_root = repository / evidence_root
    try:
        result = check(repository, evidence_root)
    except (EvidenceError, OSError, json.JSONDecodeError) as error:
        print(f"check-t0-evidence: FAIL — {error}", file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
