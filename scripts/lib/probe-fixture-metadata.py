#!/usr/bin/env python3
"""Create and verify hash-bound skill-probe fixture-set metadata."""

from __future__ import annotations

import argparse
import base64
import binascii
import ctypes
import errno
import hashlib
import json
import os
import platform
import re
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
from pathlib import Path
from pathlib import PurePosixPath
from typing import Any, NoReturn


LEGACY_BOUND_SCHEMA = "agentops-skill-probe-fixture-set.v1"
LEGACY_CANONICAL_SCHEMA = "agentops-skill-probe-fixture-set.v2"
SCHEMA = "agentops-skill-probe-fixture-set.v3"
SCORECARD_SCHEMA = "agentops-skill-probe.v3"
MANIFEST_NAME = "fixture-set.json"
CAPTURE_CONTRACT_NAME = "capture-contract.json"
LEGACY_CAPTURE_CONTRACT_SCHEMA = "agentops-skill-probe-capture.v2"
CAPTURE_CONTRACT_SCHEMA = "agentops-skill-probe-capture.v3"
# The harness writes this into the capture stage before `snapshot`; it records
# the filesystem seal the dispatch ran under (2026-08-28 contamination: control
# reps read SKILL.md off the checkout). Absent means the run was not sealed.
SEAL_NAME = "seal.json"
# The proxy's raw decision log, published beside the transcripts. Binding only
# its digest in each rep's transcript left nothing to check the digest against
# once the run directory was removed.
NETWORK_LOG_NAME = "network.log"
SEAL_RECORD_SCHEMA = "agentops-skill-probe-seal.v1"
SEAL_MODES = {"seatbelt", "none"}
LEGACY_SEAL_MODE = "legacy-unsealed"
# The 2026-09-03 first shape of the v3 seal block. Sets captured against it stay
# REPLAYABLE, but they can never be tier coverage: the block does not record the
# mechanism, the wrap, the link denies, the rep env or the config sanitization,
# so nothing in it separates a real seatbelt from a hand-written claim.
LEGACY_SEAL_KEYS = {
    "mode",
    "denied_read_roots",
    "writable_roots",
    "profile_sha256",
    "original_home",
    "repository_root",
}
# The 2026-09-03 second shape. It bound the roots and the mechanism but left
# the profile text unreconstructable from the block, so a record could assert
# roots the profile never carried, and it said nothing about the network: the
# outer profile was `(allow default)` and codex's own sandbox is bypassed inside
# it, so a rep could fetch the canonical SKILL.md over HTTPS.
PASS2_SEAL_KEYS = LEGACY_SEAL_KEYS | {
    "platform",
    "mechanism",
    "sandbox_exec",
    "wrap",
    "denied_read_data_roots",
    "denied_link_roots",
    "allowed_read_paths",
    "rep_env",
    "run_root",
    "workspace_root",
    "dispatch_root",
    "git_common_root",
    "real_tmpdir",
    "config_sanitized",
    "auth_copied",
}
# The 2026-09-03 third shape. It bound the network and could rebuild its own
# profile, but most of what it recorded was still only RECORDED: the host list,
# the writable roots, the device list, the env allowlist and the launcher chain
# were whatever the harness wrote, so a record could name any of them and stay
# coverage-eligible.
PASS3_SEAL_KEYS = PASS2_SEAL_KEYS | {
    "network",
    "dev_write_paths",
    "cache_root",
    "real_codex_home",
    "launcher_chain",
    "launcher_sha256",
    "config_sha256",
    "config_text",
    "env_allowlist",
}
SEAL_KEYS = PASS3_SEAL_KEYS | {"timeout_bin"}
SEAL_SHAPES = (LEGACY_SEAL_KEYS, PASS2_SEAL_KEYS, PASS3_SEAL_KEYS, SEAL_KEYS)
SEAL_REP_ENV_KEYS = {"HOME", "CODEX_HOME", "TMPDIR"}
SEAL_NETWORK_KEYS = {"mode", "hosts", "ports", "proxy", "unix_sockets"}
SEAL_NETWORK_MODE = "proxy-allowlist"
# A capture that overrode the allowlist records this instead, so an operator can
# still run a custom probe and the row can never be counted as coverage.
SEAL_NETWORK_MODE_CUSTOM = "proxy-custom"
# The ONLY destinations a counted capture may permit. Recording a host list is
# not the same as constraining it: pinning the set here is what stops a capture
# from allowlisting the forge and calling itself sealed.
PERMITTED_EGRESS_HOSTS = frozenset(
    {
        "chatgpt.com",
        "ab.chatgpt.com",
        ".oaiusercontent.com",
        "api.openai.com",
        "auth.openai.com",
    }
)
PERMITTED_EGRESS_PORTS = [443]
# The devices a sealed rep may write, pinned so the write allow cannot grow.
PERMITTED_DEV_WRITE_PATHS = ["/dev/null", "/dev/zero", "/dev/dtracehelper", "/dev/tty"]
# Every variable a rep may be launched with, beyond the PROBE_* test seams.
PERMITTED_ENV_ALLOWLIST = frozenset(
    {
        "PATH", "HOME", "CODEX_HOME", "TMPDIR", "LANG", "TERM",
        "PWD", "OLDPWD", "SHLVL", "_",
        "HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY", "NO_PROXY",
        "REVIEWER", "REVIEWER_MARKER",
        "CODEX_EXEC_PROMPT_FILE", "CODEX_EXEC_DIR", "CODEX_EXEC_SANDBOX",
        "CODEX_EXEC_SKIP_GIT_CHECK", "CODEX_EXEC_TIMEOUT", "CODEX_EXEC_MODEL",
        "CODEX_EXEC_OUT_FILE", "CODEX_EXEC_STDERR_FILE",
        "CODEX_EXEC_EXPECT_OUTPUT", "CODEX_EXEC_BIN",
        "SKILL_PROBES_DIR", "SKILL_PROBE_SKILLS_DIR",
    }
)
SEAL_ENV_SEAM_PREFIX = "PROBE_"
PROXY_RELATIVE_PATH = "scripts/lib/probe-connect-proxy.py"
SEAL_PLATFORM = "Darwin"
SEAL_MECHANISM = "sandbox-exec"
SEAL_SANDBOX_EXEC_PATH = "/usr/bin/sandbox-exec"
PROXY_AUTHORITY_RE = re.compile(r"^127\.0\.0\.1:([1-9][0-9]{0,4})$")
# Roots a counted row must prove were unreadable, relative to the ORIGINAL home.
SEALED_SKILL_ROOTS = (".agents", ".claude/skills", ".gemini/skills", ".codex/skills")
TRANSCRIPT_RE = re.compile(r"^(control|treatment)-([1-9][0-9]*)\.txt$")
SHA256_RE = re.compile(r"^sha256:[0-9a-f]{64}$")
SAFE_ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")
MAX_REPS = 20
MAX_INPUT_BYTES = 1024 * 1024
MAX_TRANSCRIPT_BYTES = 16 * 1024 * 1024
RESPONSE_EXTRACTION = "codex-jsonl-final-agent-message.v1"
TRANSCRIPT_FORMAT = "codex-exec-jsonl.v1"
PROBE_INPUT_EVENT = "agentops.probe-input.v1"
DISCRIMINATOR_TIMEOUT_SECONDS = 2
LEGACY_EVALUATION_INPUT_NAMES = (
    "probe.json",
    "question.md",
    "treatment-prelude.md",
    "discriminator.sh",
)
BASE_CAPTURE_INPUT_NAMES = ("probe.json", "question.md", "discriminator.sh")
TREATMENT_SOURCES = {"canonical-skill", "injected-prelude"}
CURRENT_VERDICTS = {"BEHAVIORAL", "INERT", "REGRESSIVE"}


class MetadataError(ValueError):
    """The fixture set is incomplete, unsafe, or fails integrity checks."""


def fail(message: str) -> NoReturn:
    print(f"probe-fixture-metadata: error: {message}", file=sys.stderr)
    raise SystemExit(2)


def no_duplicate_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise MetadataError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def read_regular_bytes(path: Path, label: str, *, maximum: int | None = None) -> bytes:
    """Read one stable regular-file identity without following a final symlink."""
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    try:
        fd = os.open(path, flags)
    except OSError as exc:
        raise MetadataError(f"cannot open {label} {path}: {exc}") from exc
    try:
        before = os.fstat(fd)
        if not stat.S_ISREG(before.st_mode):
            raise MetadataError(f"{label} must be a regular non-symlink file: {path}")
        if maximum is not None and before.st_size > maximum:
            raise MetadataError(
                f"{label} exceeds the {maximum}-byte safety limit: {path}"
            )
        chunks: list[bytes] = []
        total = 0
        while chunk := os.read(fd, 1024 * 1024):
            total += len(chunk)
            if maximum is not None and total > maximum:
                raise MetadataError(
                    f"{label} exceeds the {maximum}-byte safety limit: {path}"
                )
            chunks.append(chunk)
        after = os.fstat(fd)
        try:
            path_after = os.stat(path, follow_symlinks=False)
        except OSError as exc:
            raise MetadataError(
                f"{label} identity changed while reading: {path}"
            ) from exc
        if (
            not os.path.samestat(before, after)
            or not os.path.samestat(before, path_after)
            or before.st_size != after.st_size
            or before.st_mtime_ns != after.st_mtime_ns
        ):
            raise MetadataError(f"{label} changed while reading: {path}")
        return b"".join(chunks)
    finally:
        os.close(fd)


def parse_json_bytes(data: bytes, label: str) -> dict[str, Any]:
    try:
        value = json.loads(
            data.decode("utf-8", errors="strict"),
            object_pairs_hook=no_duplicate_object,
        )
    except (UnicodeError, json.JSONDecodeError, MetadataError) as exc:
        raise MetadataError(f"cannot read {label}: {exc}") from exc
    if not isinstance(value, dict):
        raise MetadataError(f"{label} must contain a JSON object")
    return value


def load_json(path: Path) -> dict[str, Any]:
    return parse_json_bytes(
        read_regular_bytes(path, path.name, maximum=MAX_INPUT_BYTES), path.name
    )


def digest_bytes(data: bytes) -> str:
    return "sha256:" + hashlib.sha256(data).hexdigest()


def digest_file(path: Path) -> str:
    return digest_bytes(read_regular_bytes(path, "file"))


def canonical_bytes(value: dict[str, Any]) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def require_text(value: Any, field: str, *, nullable: bool = False) -> str | None:
    if nullable and value is None:
        return None
    if not isinstance(value, str) or not value or any(ord(ch) < 32 for ch in value):
        raise MetadataError(
            f"{field} must be a non-empty string without control characters"
        )
    return value


def require_message_text(value: Any, field: str) -> str:
    if (
        not isinstance(value, str)
        or not value.strip()
        or any(ord(ch) < 32 and ch not in "\t\r\n" for ch in value)
    ):
        raise MetadataError(
            f"{field} must be non-blank text without unsupported control characters"
        )
    try:
        value.encode("utf-8", errors="strict")
    except UnicodeError as exc:
        raise MetadataError(f"{field} must be valid UTF-8 text") from exc
    return value


def require_reps(value: Any) -> int:
    if (
        isinstance(value, bool)
        or not isinstance(value, int)
        or value < 1
        or value > MAX_REPS
    ):
        raise MetadataError(f"reps must be an integer between 1 and {MAX_REPS}")
    return value


def require_nonnegative_int(value: Any, field: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        raise MetadataError(f"{field} must be a non-negative integer")
    return value


def require_rate(value: Any, field: str) -> float | None:
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise MetadataError(f"{field} must be a number or null")
    numeric = float(value)
    if numeric < 0.0 or numeric > 1.0:
        raise MetadataError(f"{field} must be between 0 and 1")
    return numeric


def require_exact_object(value: Any, keys: set[str], field: str) -> dict[str, Any]:
    if not isinstance(value, dict) or set(value) != keys:
        rendered = ", ".join(sorted(keys))
        raise MetadataError(f"{field} must contain exactly: {rendered}")
    return value


def expected_transcripts(reps: int) -> list[str]:
    return [
        f"{arm}-{rep}.txt"
        for rep in range(1, reps + 1)
        for arm in ("control", "treatment")
    ]


def validate_fixture_dir(path: Path) -> None:
    if path.is_symlink():
        raise MetadataError(f"fixture directory must not be a symlink: {path}")
    if not path.is_dir():
        raise MetadataError(f"fixture directory not found: {path}")


def validate_transcript(path: Path) -> None:
    if path.is_symlink() or not path.is_file():
        raise MetadataError(
            f"transcript must be a regular non-symlink file: {path.name}"
        )


def validate_regular_file(path: Path, label: str) -> None:
    if path.is_symlink() or not path.is_file():
        raise MetadataError(f"{label} must be a regular non-symlink file: {path}")


def canonical_skill_path(skills_dir: Path, skill: str) -> Path:
    """Resolve the one canonical skill source without traversing symlinks."""
    if not SAFE_ID_RE.fullmatch(skill):
        raise MetadataError(f"unsafe canonical skill name: {skill!r}")
    if skills_dir.is_symlink() or not skills_dir.is_dir():
        raise MetadataError(
            f"canonical skills directory must be a non-symlink directory: {skills_dir}"
        )
    skill_dir = skills_dir / skill
    if skill_dir.is_symlink() or not skill_dir.is_dir():
        raise MetadataError(
            f"canonical skill directory must be a non-symlink directory: {skill_dir}"
        )
    path = skill_dir / "SKILL.md"
    validate_regular_file(path, "canonical skill")
    return path


def frontmatter_lines(data: bytes, label: str) -> list[str]:
    try:
        lines = data.decode("utf-8", errors="strict").splitlines()
    except UnicodeError as exc:
        raise MetadataError(f"cannot read {label} frontmatter: {exc}") from exc
    if not lines or lines[0].strip() != "---":
        raise MetadataError(f"{label} must start with YAML frontmatter")
    try:
        end = next(index for index, line in enumerate(lines[1:], 1) if line == "---")
    except StopIteration as exc:
        raise MetadataError(f"{label} has no closing YAML frontmatter marker") from exc
    return lines[1:end]


def declared_skill_name_bytes(data: bytes) -> str:
    """Read the top-level name from the YAML frontmatter without a YAML dependency."""
    lines = frontmatter_lines(data, "canonical skill")
    names = []
    for line in lines:
        match = re.fullmatch(r"name:[ \t]*(.*?)[ \t]*", line)
        if match:
            value = match.group(1)
            if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
                value = value[1:-1]
            names.append(value)
    if len(names) != 1 or not names[0]:
        raise MetadataError(
            "canonical skill frontmatter must declare exactly one non-empty top-level name"
        )
    name = require_text(names[0], "canonical skill frontmatter name")
    assert isinstance(name, str)
    return name


def declared_skill_name(path: Path) -> str:
    return declared_skill_name_bytes(
        read_regular_bytes(path, "canonical skill", maximum=MAX_INPUT_BYTES)
    )


def yaml_scalar(value: str, field: str) -> str:
    scalar = value.strip()
    if len(scalar) >= 2 and scalar[0] == scalar[-1] and scalar[0] in {'"', "'"}:
        scalar = scalar[1:-1]
    if not scalar or any(ord(ch) < 32 for ch in scalar):
        raise MetadataError(f"{field} must be one non-empty scalar")
    return scalar


def coverage_frontmatter(path: Path) -> tuple[str | None, bool]:
    lines = frontmatter_lines(
        read_regular_bytes(path, "skill", maximum=MAX_INPUT_BYTES), "skill"
    )
    tiers: list[str] = []
    implementations: list[str] = []
    metadata_blocks = 0
    in_metadata = False
    for line in lines:
        if line and line[0] not in " \t":
            in_metadata = False
            match = re.fullmatch(r"([A-Za-z0-9_-]+):[ \t]*(.*?)[ \t]*", line)
            if not match:
                continue
            key, value = match.groups()
            if key == "metadata":
                metadata_blocks += 1
                if value:
                    raise MetadataError("skill metadata must be a YAML mapping")
                in_metadata = True
            elif key == "implementation":
                implementations.append(yaml_scalar(value, "implementation"))
            elif key == "tier":
                tiers.append(yaml_scalar(value, "tier"))
            continue
        if in_metadata:
            match = re.fullmatch(r"  tier:[ \t]*(.*?)[ \t]*", line)
            if match:
                tiers.append(yaml_scalar(match.group(1), "metadata.tier"))
    if metadata_blocks > 1:
        raise MetadataError(f"duplicate metadata mapping in {path}")
    if len(tiers) > 1:
        raise MetadataError(f"duplicate tier declaration in {path}")
    if len(implementations) > 1:
        raise MetadataError(f"duplicate implementation declaration in {path}")
    implementation = implementations[0].lower() if implementations else "true"
    if implementation not in {"true", "false"}:
        raise MetadataError(f"implementation must be true or false in {path}")
    return (tiers[0] if tiers else None, implementation == "false")


def tier_skills(skills_dir: Path) -> list[str]:
    if skills_dir.is_symlink() or not skills_dir.is_dir():
        raise MetadataError("skills root must be a non-symlink directory")
    result: list[str] = []
    for skill_dir in sorted(skills_dir.iterdir(), key=lambda path: path.name):
        if skill_dir.is_symlink() or not skill_dir.is_dir():
            continue
        skill_path = skill_dir / "SKILL.md"
        if not skill_path.exists():
            continue
        validate_regular_file(skill_path, "skill")
        tier, redirect = coverage_frontmatter(skill_path)
        if not redirect and tier in {"product", "judgment"}:
            result.append(skill_dir.name)
    return result


def build_canonical_skill_record(
    probe_dir: Path, skills_dir: Path, expected_probe: str
) -> dict[str, str]:
    probe_meta = load_json(probe_dir / "probe.json")
    if probe_meta.get("id") != expected_probe:
        raise MetadataError("probe.json id does not match the requested probe")
    skill = require_text(probe_meta.get("skill"), "probe.json skill")
    assert isinstance(skill, str)
    path = canonical_skill_path(skills_dir, skill)
    declared = declared_skill_name(path)
    if declared != skill:
        raise MetadataError(
            f"canonical skill identity mismatch: probe.json names {skill!r}, "
            f"SKILL.md declares {declared!r}"
        )
    return {
        "name": skill,
        "path": f"skills/{skill}/SKILL.md",
        "sha256": digest_file(path),
    }


def build_embedded_canonical_skill_record(
    probe_dir: Path, skills_dir: Path, expected_probe: str
) -> dict[str, str]:
    record = build_canonical_skill_record(probe_dir, skills_dir, expected_probe)
    path = canonical_skill_path(skills_dir, record["name"])
    embedded = embedded_record(path, record["path"], "canonical skill")
    return {"name": record["name"], **embedded}


def validate_probe_metadata(
    probe_meta: dict[str, Any], expected_probe: str, *, require_treatment: bool
) -> dict[str, Any]:
    if probe_meta.get("id") != expected_probe:
        raise MetadataError("probe.json id does not match the requested probe")
    skill = require_text(probe_meta.get("skill"), "probe.json skill")
    assert isinstance(skill, str)
    if not SAFE_ID_RE.fullmatch(skill):
        raise MetadataError(f"unsafe probe.json skill: {skill!r}")
    reps = require_reps(probe_meta.get("reps"))
    if probe_meta.get("discriminator") != "discriminator.sh":
        raise MetadataError(
            "probe.json discriminator must be exactly 'discriminator.sh'"
        )
    source = probe_meta.get("treatment_source")
    if require_treatment:
        source = require_text(source, "probe.json treatment_source")
        if source not in TREATMENT_SOURCES:
            choices = ", ".join(sorted(TREATMENT_SOURCES))
            raise MetadataError(
                f"probe.json treatment_source must be one of: {choices}"
            )
    return {"skill": skill, "reps": reps, "treatment_source": source}


def declared_treatment_source(probe_dir: Path, expected_probe: str) -> str:
    probe_meta = load_json(probe_dir / "probe.json")
    contract = validate_probe_metadata(
        probe_meta, expected_probe, require_treatment=True
    )
    source = contract["treatment_source"]
    assert isinstance(source, str)
    return source


def validate_canonical_skill_record(
    value: Any, probe_dir: Path, skills_dir: Path, expected_probe: str
) -> dict[str, str]:
    record = require_exact_object(value, {"name", "path", "sha256"}, "canonical_skill")
    expected = build_canonical_skill_record(probe_dir, skills_dir, expected_probe)
    for field in ("name", "path"):
        if record[field] != expected[field]:
            raise MetadataError(
                f"canonical_skill {field} mismatch: expected {expected[field]!r}, "
                f"got {record[field]!r}"
            )
    digest = record["sha256"]
    if not isinstance(digest, str) or not SHA256_RE.fullmatch(digest):
        raise MetadataError("canonical_skill sha256 must be a sha256 digest")
    if digest != expected["sha256"]:
        raise MetadataError(
            "canonical skill digest mismatch for "
            f"{record['path']}: expected {digest}, got {expected['sha256']}"
        )
    return expected


def evaluation_input_records(probe_dir: Path) -> list[dict[str, str]]:
    if probe_dir.is_symlink() or not probe_dir.is_dir():
        raise MetadataError(
            f"probe directory must be a non-symlink directory: {probe_dir}"
        )
    records = []
    for name in LEGACY_EVALUATION_INPUT_NAMES:
        path = probe_dir / name
        validate_regular_file(path, "evaluation input")
        records.append({"path": name, "sha256": digest_file(path)})
    return records


def embedded_record(path: Path, logical_path: str, label: str) -> dict[str, str]:
    data = read_regular_bytes(path, label, maximum=MAX_INPUT_BYTES)
    return embedded_bytes_record(data, logical_path)


def embedded_bytes_record(data: bytes, logical_path: str) -> dict[str, str]:
    return {
        "path": logical_path,
        "sha256": digest_bytes(data),
        "content_base64": base64.b64encode(data).decode("ascii"),
    }


def capture_input_names(treatment_source: str) -> tuple[str, ...]:
    if treatment_source == "injected-prelude":
        return LEGACY_EVALUATION_INPUT_NAMES
    if treatment_source == "canonical-skill":
        return BASE_CAPTURE_INPUT_NAMES
    raise MetadataError(f"unsupported treatment source: {treatment_source!r}")


def embedded_input_records(
    probe_dir: Path, treatment_source: str
) -> list[dict[str, str]]:
    return [
        embedded_record(probe_dir / name, name, "capture input")
        for name in capture_input_names(treatment_source)
    ]


def decode_embedded_record(
    value: Any,
    expected_path: str,
    label: str,
    *,
    maximum: int = MAX_INPUT_BYTES,
) -> tuple[dict[str, str], bytes]:
    record = require_exact_object(value, {"path", "sha256", "content_base64"}, label)
    if record["path"] != expected_path:
        raise MetadataError(
            f"{label} path mismatch: expected {expected_path!r}, got {record['path']!r}"
        )
    digest = record["sha256"]
    if not isinstance(digest, str) or not SHA256_RE.fullmatch(digest):
        raise MetadataError(f"{label} sha256 must be a sha256 digest")
    encoded = record["content_base64"]
    if not isinstance(encoded, str):
        raise MetadataError(f"{label} content_base64 must be a string")
    try:
        data = base64.b64decode(encoded, validate=True)
    except (ValueError, binascii.Error) as exc:
        raise MetadataError(f"{label} content_base64 is invalid") from exc
    if len(data) > maximum:
        raise MetadataError(f"{label} exceeds the {maximum}-byte safety limit")
    if digest_bytes(data) != digest:
        raise MetadataError(f"{label} embedded bytes do not match sha256")
    return record, data


def validate_embedded_canonical_skill(
    value: Any, probe_meta: dict[str, Any], expected_probe: str
) -> tuple[dict[str, str], bytes]:
    contract = validate_probe_metadata(
        probe_meta, expected_probe, require_treatment=True
    )
    skill = contract["skill"]
    record = require_exact_object(
        value,
        {"name", "path", "sha256", "content_base64"},
        "canonical_skill",
    )
    if record["name"] != skill:
        raise MetadataError("canonical_skill name disagrees with captured probe.json")
    expected_path = f"skills/{skill}/SKILL.md"
    decoded_record, data = decode_embedded_record(
        {key: record[key] for key in ("path", "sha256", "content_base64")},
        expected_path,
        "canonical_skill",
    )
    if declared_skill_name_bytes(data) != skill:
        raise MetadataError(
            "embedded canonical skill identity disagrees with captured probe.json"
        )
    return {"name": skill, **decoded_record}, data


def decode_capture_inputs(
    records: Any, expected_probe: str, treatment_source: str
) -> tuple[dict[str, bytes], dict[str, Any]]:
    names = capture_input_names(treatment_source)
    if not isinstance(records, list) or len(records) != len(names):
        raise MetadataError("capture_inputs must contain the exact input inventory")
    decoded: dict[str, bytes] = {}
    for expected, value in zip(names, records, strict=True):
        _, data = decode_embedded_record(
            value, expected, f"capture_inputs.{expected}"
        )
        decoded[expected] = data
    probe_meta = parse_json_bytes(decoded["probe.json"], "captured probe.json")
    contract = validate_probe_metadata(
        probe_meta, expected_probe, require_treatment=True
    )
    if contract["treatment_source"] != treatment_source:
        raise MetadataError(
            "capture input treatment source disagrees with the capture contract"
        )
    return decoded, contract


def prompt_bytes(
    inputs: dict[str, bytes], canonical_skill: bytes, treatment_source: str
) -> dict[str, bytes]:
    question = inputs["question.md"]
    treatment = (
        canonical_skill
        if treatment_source == "canonical-skill"
        else inputs["treatment-prelude.md"]
    )
    return {
        "control": question,
        "treatment": treatment + b"\n\n---\n\n" + question,
    }


def prompt_records(
    inputs: dict[str, bytes], canonical_skill: bytes, treatment_source: str
) -> list[dict[str, str]]:
    prompts = prompt_bytes(inputs, canonical_skill, treatment_source)
    return [
        embedded_bytes_record(prompts[arm], f"{arm}.prompt")
        for arm in ("control", "treatment")
    ]


def decode_prompts(value: Any) -> dict[str, bytes]:
    if not isinstance(value, list) or len(value) != 2:
        raise MetadataError("prompts must contain exact control and treatment records")
    decoded: dict[str, bytes] = {}
    for arm, record in zip(("control", "treatment"), value, strict=True):
        _, data = decode_embedded_record(
            record,
            f"{arm}.prompt",
            f"prompts.{arm}",
            maximum=MAX_INPUT_BYTES * 2 + 16,
        )
        decoded[arm] = data
    return decoded


def resolve_executable(command: str) -> Path:
    candidate = shutil.which(command) if os.sep not in command else command
    if not candidate:
        raise MetadataError(f"producer executable not found: {command}")
    try:
        resolved = Path(candidate).resolve(strict=True)
    except OSError as exc:
        raise MetadataError(f"producer executable cannot be resolved: {command}") from exc
    validate_regular_file(resolved, "producer executable")
    if not os.access(resolved, os.X_OK):
        raise MetadataError(f"producer executable is not executable: {resolved}")
    return resolved


def producer_coverage_eligible(
    override: bool, model: str | None, effort: str | None, seal_mode: str
) -> bool:
    """A row counts only for a native, fully specified, seatbelt-sealed producer."""
    return (
        not override
        and model is not None
        and effort is not None
        and seal_mode == "seatbelt"
    )


def producer_runtime_identity(
    requested_model: str | None,
    requested_effort: str | None,
    override_command: str | None,
    seal_mode: str,
) -> dict[str, Any]:
    model = require_text(requested_model, "requested model", nullable=True)
    effort = require_text(requested_effort, "requested effort", nullable=True)
    command = override_command or "codex"
    resolved = resolve_executable(command)
    try:
        completed = subprocess.run(
            [str(resolved), "--version"],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
            timeout=5,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise MetadataError(f"could not identify producer runtime: {exc}") from exc
    if completed.returncode != 0:
        detail = completed.stderr.decode("utf-8", errors="replace").strip()
        raise MetadataError(
            f"producer runtime --version failed with {completed.returncode}: {detail}"
        )
    if completed.stderr:
        detail = completed.stderr.decode("utf-8", errors="replace").strip()
        raise MetadataError(f"producer runtime --version wrote stderr: {detail}")
    try:
        version = completed.stdout.decode("utf-8", errors="strict").strip()
    except UnicodeError as exc:
        raise MetadataError("producer runtime version is not UTF-8") from exc
    require_text(version, "producer runtime version")
    override = override_command is not None
    return {
        "adapter": "codex",
        "model": model,
        "effort": effort,
        "identity": {
            "source": "test-override" if override else "native-codex-path",
            "override": override,
            "version": version,
            "executable_sha256": digest_file(resolved),
            "coverage_eligible": producer_coverage_eligible(
                override, model, effort, seal_mode
            ),
        },
    }


def validate_producer_request(
    value: Any, *, seal_mode: str | None = None
) -> dict[str, Any]:
    """Validate a producer request; with a seal mode, bind eligibility exactly.

    Without a seal mode (manifest or scorecard copies, whose contract is checked
    separately) only the one-way rule holds: a producer that fails the native
    model/effort floor can never claim eligibility.
    """
    request = require_exact_object(
        value, {"adapter", "model", "effort", "identity"}, "producer_request"
    )
    if request["adapter"] != "codex":
        raise MetadataError("producer_request adapter must be codex")
    model = require_text(request["model"], "producer_request model", nullable=True)
    effort = require_text(request["effort"], "producer_request effort", nullable=True)
    identity = require_exact_object(
        request["identity"],
        {"source", "override", "version", "executable_sha256", "coverage_eligible"},
        "producer_request identity",
    )
    if identity["source"] not in {"native-codex-path", "test-override"}:
        raise MetadataError("producer identity source is unsupported")
    if not isinstance(identity["override"], bool):
        raise MetadataError("producer identity override must be boolean")
    if identity["override"] != (identity["source"] == "test-override"):
        raise MetadataError("producer identity source/override mismatch")
    require_text(identity["version"], "producer identity version")
    if not isinstance(identity["executable_sha256"], str) or not SHA256_RE.fullmatch(
        identity["executable_sha256"]
    ):
        raise MetadataError("producer executable_sha256 must be a sha256 digest")
    if not isinstance(identity["coverage_eligible"], bool):
        raise MetadataError("producer coverage_eligible must be boolean")
    floor = not identity["override"] and model is not None and effort is not None
    if seal_mode is None:
        consistent = not identity["coverage_eligible"] or floor
    else:
        consistent = identity["coverage_eligible"] is producer_coverage_eligible(
            identity["override"], model, effort, seal_mode
        )
    if not consistent:
        raise MetadataError("producer coverage_eligible is inconsistent")
    return request


def require_absolute_path(value: Any, field: str) -> str:
    text = require_text(value, field)
    assert text is not None
    if not text.startswith("/") or os.path.normpath(text) != text:
        raise MetadataError(f"{field} must be a normalized absolute path: {text!r}")
    return text


def require_path_list(value: Any, field: str) -> list[str]:
    if not isinstance(value, list):
        raise MetadataError(f"{field} must be an array of absolute paths")
    paths = [require_absolute_path(item, f"{field}[{index}]") for index, item in enumerate(value)]
    if len(set(paths)) != len(paths):
        raise MetadataError(f"{field} contains duplicate paths")
    return paths


def normalized_path_list(value: Any, field: str) -> list[str]:
    """Normalize harness-supplied roots (trailing slashes) before binding them."""
    if not isinstance(value, list):
        raise MetadataError(f"{field} must be an array of absolute paths")
    normalized = []
    for index, item in enumerate(value):
        text = require_text(item, f"{field}[{index}]")
        assert text is not None
        if not text.startswith("/"):
            raise MetadataError(f"{field}[{index}] must be an absolute path: {text!r}")
        normalized.append(os.path.normpath(text))
    return require_path_list(normalized, field)


def original_home() -> str:
    """The HOME the un-sealed agent would resolve its skill roots under.

    The harness must snapshot before it swaps HOME for a scratch directory;
    the seal check resolves the four skill roots under this recorded value.
    """
    home = os.environ.get("HOME") or os.path.expanduser("~")
    return require_absolute_path(os.path.normpath(home), "original home")


def capture_repository_root(skills_dir: Path) -> str:
    try:
        resolved = skills_dir.resolve(strict=True)
    except OSError as exc:
        raise MetadataError(f"canonical skills directory not found: {exc}") from exc
    return require_absolute_path(str(resolved.parent), "capture repository root")


def toml_scalar(value: Any) -> str | None:
    """One TOML right-hand side, or None when the value is not a scalar."""
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        return repr(value)
    if isinstance(value, str):
        return json.dumps(value, ensure_ascii=False)
    if isinstance(value, list) and all(
        isinstance(item, (bool, int, float, str)) for item in value
    ):
        rendered = [toml_scalar(item) for item in value]
        if any(item is None for item in rendered):
            return None
        return "[" + ", ".join(item for item in rendered if item is not None) + "]"
    return None


# The rep's config is GENERATED from this allowlist, never copied or filtered
# from the operator's. Copying it, even table-stripped, carried whatever the
# operator had set: `web_search` live, a `notify` hook naming an operator
# program, and a key set that changes under the harness. A generated file has
# one text, one digest, and nothing else can be in it.
PROBE_CONFIG_KEYS = ("model_reasoning_effort", "web_search")
PROBE_CONFIG_HEADER = (
    "# Generated for one sealed skill-probe capture. Not the operator's config.\n"
    "# Only the keys this harness needs are present: the model comes from the\n"
    "# --model flag, and web search is off so the rep has no second egress path.\n"
)


def render_probe_config(effort: str | None) -> str:
    """The exact config text every rep of one capture runs with."""
    lines = [PROBE_CONFIG_HEADER.rstrip("\n")]
    if effort:
        require_text(effort, "probe config effort")
        if not re.fullmatch(r"[a-z]+", effort):
            raise MetadataError(f"probe config effort is unsafe: {effort!r}")
        lines.append(f'model_reasoning_effort = "{effort}"')
    lines.append('web_search = "disabled"')
    return "\n".join(lines) + "\n"


def probe_config_drift(path: Path, expected_text: str, workspace: str) -> list[str]:
    """What a rep changed in its config beyond the one permitted growth.

    codex writes a `[projects."<cwd>"]` trust table into its config on first run
    in a directory. That growth is expected and harmless; anything else means
    the rep edited the file it was measured under.
    """
    import tomllib

    try:
        actual = path.read_text(encoding="utf-8")
    except OSError as exc:
        return [f"config unreadable after the rep: {exc}"]
    findings: list[str] = []
    # ALWAYS parse, even when the bytes are unchanged: a byte-equality shortcut
    # means the structural rules below never run on the ordinary path, so a
    # mistake in them would only ever show up on a rep that had already drifted.
    try:
        parsed = tomllib.loads(actual)
        baseline = tomllib.loads(expected_text)
    except (UnicodeError, tomllib.TOMLDecodeError) as exc:
        return [f"config is no longer readable TOML: {exc}"]
    for key in sorted(set(baseline) | set(parsed)):
        if key == "projects":
            continue
        if baseline.get(key) != parsed.get(key):
            findings.append(f"key {key!r} changed")
    projects = parsed.get("projects")
    if projects is not None:
        if not isinstance(projects, dict):
            findings.append("[projects] is not a table")
        else:
            for name in sorted(projects):
                if name != workspace:
                    findings.append(f"[projects] gained an entry outside the workspace: {name}")
                    continue
                entry = projects[name]
                if not isinstance(entry, dict):
                    findings.append("[projects] workspace entry is not a table")
                    continue
                extra = sorted(set(entry) - {"trust_level"})
                if extra:
                    findings.append(
                        "[projects] workspace entry carries more than trust_level: "
                        + ", ".join(extra)
                    )
    return findings


def seal_quote(value: str, field: str) -> str:
    """A seatbelt path literal. A quote or a backslash could break the profile."""
    text = require_text(value, field)
    assert isinstance(text, str)
    if any(character in text for character in ('"', "\\", "\n")):
        raise MetadataError(f"{field} cannot be quoted in a seatbelt profile: {text!r}")
    return text


def seal_network_port(network: dict[str, Any]) -> str:
    match = PROXY_AUTHORITY_RE.fullmatch(str(network.get("proxy", "")))
    if match is None:
        raise MetadataError(
            "seal network proxy must be 127.0.0.1:<port>, got: "
            f"{network.get('proxy')!r}"
        )
    return match.group(1)


def render_seal_profile(seal: dict[str, Any]) -> str:
    """Rebuild the seatbelt profile text from the BOUND block.

    This is the only writer of a probe seal profile: the harness renders through
    it, and coverage requires digest(render_seal_profile(block)) to equal the
    block's profile_sha256. Before this, the block's roots were assertions
    ALONGSIDE an opaque profile, so a record could claim writable_roots ["/"] or
    an allowed_read_paths entry pointing at the main checkout and still be
    coverage-eligible: nothing tied the claim to the bytes the kernel enforced.
    """
    if seal.get("mode") != "seatbelt":
        raise MetadataError("only a seatbelt seal has a profile to render")
    if set(seal) != SEAL_KEYS:
        raise MetadataError("only the hardened seal block can be rendered")
    network = seal["network"]
    home = seal["rep_env"]["HOME"]
    workspace = seal["workspace_root"]
    temp = seal["rep_env"]["TMPDIR"]
    lines = ["(version 1)", "(allow default)", "(deny network*)"]
    lines.append(
        f'(allow network-outbound (remote ip "localhost:{seal_network_port(network)}"))'
    )
    for socket_path in network["unix_sockets"]:
        lines.append(
            f'(allow network-outbound (literal "{seal_quote(socket_path, "unix socket")}"))'
        )
    lines.append("(deny file-write*)")
    lines.append("(allow file-write*")
    for root in seal["writable_roots"]:
        lines.append(f'  (subpath "{seal_quote(root, "writable root")}")')
    for device in seal["dev_write_paths"]:
        lines.append(f'  (literal "{seal_quote(device, "device")}")')
    lines.append(")")
    lines.append("(deny file-read*")
    for root in seal["denied_read_roots"]:
        lines.append(f'  (subpath "{seal_quote(root, "denied read root")}")')
    lines.append(")")
    lines.append("(allow file-read-metadata")
    lines.append(f'  (path-ancestors "{seal_quote(workspace, "workspace root")}")')
    for allowed in seal["allowed_read_paths"]:
        lines.append(f'  (path-ancestors "{seal_quote(allowed, "allowed read path")}")')
    lines.append(")")
    lines.append("(allow file-read*")
    for root in (home, workspace, temp):
        lines.append(f'  (subpath "{seal_quote(root, "rep root")}")')
    for allowed in seal["allowed_read_paths"]:
        lines.append(f'  (literal "{seal_quote(allowed, "allowed read path")}")')
    lines.append(")")
    lines.append("(allow file-read-metadata")
    for root in seal["denied_read_data_roots"]:
        lines.append(f'  (subpath "{seal_quote(root, "denied read-data root")}")')
    lines.append(")")
    lines.append("(deny file-write*")
    for root in seal["denied_read_data_roots"]:
        lines.append(f'  (subpath "{seal_quote(root, "denied read-data root")}")')
    lines.append(")")
    for operation in ("file-link", "file-clone"):
        lines.append(f"(deny {operation}")
        for root in seal["denied_link_roots"]:
            lines.append(f'  (subpath "{seal_quote(root, "denied link root")}")')
        lines.append(")")
    return "\n".join(lines)


def legacy_seal() -> dict[str, Any]:
    return {
        "mode": LEGACY_SEAL_MODE,
        "denied_read_roots": [],
        "writable_roots": [],
        "profile_sha256": None,
        "original_home": None,
        "repository_root": None,
    }


def seal_projection(seal: dict[str, Any]) -> dict[str, Any]:
    """The seal fields derived from the seal record itself (not the checkout)."""
    return {
        key: seal[key]
        for key in sorted(set(seal) - {"repository_root"})
    }


def optional_normalized_path(value: Any, field: str) -> str | None:
    if value is None:
        return None
    return require_absolute_path(
        os.path.normpath(require_text(value, field)), field
    )


def seal_block_from_record(
    record: Any, label: str, repo_root: str
) -> dict[str, Any]:
    """Reduce a harness seal record (agentops-skill-probe-seal.v1) to the bound block."""
    if not isinstance(record, dict):
        raise MetadataError(f"{label} must be a JSON object")
    required = {
        "schema",
        "seal_mode",
        "coverage_eligible",
        "profile",
        "profile_sha256",
        "denied_read_roots",
        "writable_roots",
        "real_home",
        "platform",
        "mechanism",
        "sandbox_exec",
        "wrap",
        "denied_read_data_roots",
        "denied_link_roots",
        "allowed_read_paths",
        "rep_env",
        "run_root",
        "workspace_root",
        "dispatch_root",
        "git_common_root",
        "real_tmpdir",
        "config_sanitized",
        "auth_copied",
        "network",
        "dev_write_paths",
        "cache_root",
        "real_codex_home",
        "launcher_chain",
        "launcher_sha256",
        "config_sha256",
        "config_text",
        "env_allowlist",
        "timeout_bin",
        "profile_file",
        "real_codex_home",
    }
    missing = required - set(record)
    if missing:
        raise MetadataError(f"{label} is missing: " + ", ".join(sorted(missing)))
    # A record may carry nothing the reducer does not know about: an unknown
    # field is either a shape this verifier cannot check or a place to smuggle
    # one, and either way it must not pass silently.
    unknown = set(record) - required
    if unknown:
        raise MetadataError(f"{label} has unknown fields: " + ", ".join(sorted(unknown)))
    if record["schema"] != SEAL_RECORD_SCHEMA:
        raise MetadataError(f"{label} schema is unsupported: {record['schema']!r}")
    mode = require_text(record["seal_mode"], f"{label} seal_mode")
    if mode not in SEAL_MODES:
        raise MetadataError(f"{label} seal_mode is unsupported: {mode!r}")
    if record["coverage_eligible"] is not (mode == "seatbelt"):
        raise MetadataError(f"{label} coverage_eligible disagrees with its seal_mode")
    profile = record["profile"]
    if mode == "seatbelt":
        profile_text = require_message_text(profile, f"{label} profile")
        profile_sha256: str | None = digest_bytes(profile_text.encode("utf-8"))
        if record["profile_sha256"] != profile_sha256:
            raise MetadataError(f"{label} profile_sha256 does not match its profile text")
    else:
        if profile is not None or record["profile_sha256"] is not None:
            raise MetadataError(f"{label} profile must be null when the seal_mode is none")
        profile_sha256 = None
    rep_env_value = record["rep_env"]
    rep_env = require_exact_object(rep_env_value, SEAL_REP_ENV_KEYS, f"{label} rep_env")
    config_sanitized = record["config_sanitized"]
    if config_sanitized is not None:
        if not isinstance(config_sanitized, list) or not all(
            isinstance(item, str) for item in config_sanitized
        ):
            raise MetadataError(f"{label} config_sanitized must be a list of key names")
        config_sanitized = list(config_sanitized)
    wrap = record["wrap"]
    if not isinstance(wrap, list) or not all(isinstance(item, str) for item in wrap):
        raise MetadataError(f"{label} wrap must be an array of argv strings")
    if not isinstance(record["auth_copied"], bool):
        raise MetadataError(f"{label} auth_copied must be a boolean")
    network = require_exact_object(
        record["network"], SEAL_NETWORK_KEYS, f"{label} network"
    )
    if not isinstance(network["hosts"], list) or not all(
        isinstance(item, str) and item for item in network["hosts"]
    ):
        raise MetadataError(f"{label} network hosts must be a list of host names")
    if not isinstance(network["ports"], list) or not all(
        isinstance(item, int) and not isinstance(item, bool) and 0 < item < 65536
        for item in network["ports"]
    ):
        raise MetadataError(f"{label} network ports must be a list of port numbers")
    if not isinstance(network["unix_sockets"], list) or not all(
        isinstance(item, str) and item.startswith("/") for item in network["unix_sockets"]
    ):
        raise MetadataError(f"{label} network unix_sockets must be absolute paths")
    launcher_chain = normalized_path_list(
        record["launcher_chain"], f"{label} launcher_chain"
    )
    for field in ("config_sha256", "launcher_sha256"):
        value = record[field]
        if value is not None and not (
            isinstance(value, str) and SHA256_RE.fullmatch(value)
        ):
            raise MetadataError(f"{label} {field} must be a sha256 digest or null")
    if not isinstance(record["env_allowlist"], list) or not all(
        isinstance(item, str) and item for item in record["env_allowlist"]
    ):
        raise MetadataError(f"{label} env_allowlist must be a list of variable names")
    return {
        "mode": mode,
        "platform": require_text(record["platform"], f"{label} platform"),
        "mechanism": require_text(
            record["mechanism"], f"{label} mechanism", nullable=True
        ),
        "sandbox_exec": require_text(
            record["sandbox_exec"], f"{label} sandbox_exec", nullable=True
        ),
        "wrap": list(wrap),
        "denied_read_roots": normalized_path_list(
            record["denied_read_roots"], f"{label} denied_read_roots"
        ),
        "denied_read_data_roots": normalized_path_list(
            record["denied_read_data_roots"], f"{label} denied_read_data_roots"
        ),
        "denied_link_roots": normalized_path_list(
            record["denied_link_roots"], f"{label} denied_link_roots"
        ),
        "writable_roots": normalized_path_list(
            record["writable_roots"], f"{label} writable_roots"
        ),
        "allowed_read_paths": normalized_path_list(
            record["allowed_read_paths"], f"{label} allowed_read_paths"
        ),
        "rep_env": {
            key: require_absolute_path(
                os.path.normpath(require_text(rep_env[key], f"{label} rep_env {key}")),
                f"{label} rep_env {key}",
            )
            for key in sorted(SEAL_REP_ENV_KEYS)
        },
        "run_root": optional_normalized_path(record["run_root"], f"{label} run_root"),
        "workspace_root": optional_normalized_path(
            record["workspace_root"], f"{label} workspace_root"
        ),
        "dispatch_root": optional_normalized_path(
            record["dispatch_root"], f"{label} dispatch_root"
        ),
        "git_common_root": optional_normalized_path(
            record["git_common_root"], f"{label} git_common_root"
        ),
        "real_tmpdir": optional_normalized_path(
            record["real_tmpdir"], f"{label} real_tmpdir"
        ),
        "config_sanitized": config_sanitized,
        "auth_copied": record["auth_copied"],
        "network": {
            "mode": require_text(network["mode"], f"{label} network mode"),
            "hosts": sorted(network["hosts"]),
            "proxy": require_text(
                network["proxy"], f"{label} network proxy", nullable=True
            ),
            "unix_sockets": sorted(network["unix_sockets"]),
            "ports": sorted(network["ports"]),
        },
        "dev_write_paths": normalized_path_list(
            record["dev_write_paths"], f"{label} dev_write_paths"
        ),
        "cache_root": optional_normalized_path(
            record["cache_root"], f"{label} cache_root"
        ),
        "real_codex_home": require_absolute_path(
            os.path.normpath(
                require_text(record["real_codex_home"], f"{label} real_codex_home")
            ),
            f"{label} real_codex_home",
        ),
        "launcher_chain": launcher_chain,
        "launcher_sha256": record["launcher_sha256"],
        "timeout_bin": optional_normalized_path(
            record["timeout_bin"], f"{label} timeout_bin"
        ),
        "config_sha256": record["config_sha256"],
        "config_text": (
            None
            if record["config_text"] is None
            else require_message_text(record["config_text"], f"{label} config_text")
        ),
        "env_allowlist": sorted(record["env_allowlist"]),
        "profile_sha256": profile_sha256,
        "original_home": require_absolute_path(
            os.path.normpath(require_text(record["real_home"], f"{label} real_home")),
            f"{label} real_home",
        ),
        "repository_root": repo_root,
    }


SEAL_PAYLOAD_KEYS = {
    "seal_mode",
    "sandbox_exec",
    "platform",
    "denied_read_roots",
    "denied_read_data_roots",
    "denied_link_roots",
    "writable_roots",
    "dev_write_paths",
    "allowed_read_paths",
    "launcher_chain",
    "launcher_sha256",
    "timeout_bin",
    "rep_env",
    "env_allowlist",
    "real_home",
    "real_codex_home",
    "real_tmpdir",
    "cache_root",
    "git_common_root",
    "run_root",
    "workspace_root",
    "dispatch_root",
    "network",
    "config_sanitized",
    "config_sha256",
    "config_text",
    "auth_copied",
    "profile_file",
}


def write_seal_record(payload_path: Path, output_path: Path) -> dict[str, Any]:
    """Render the profile from the harness payload and write seal.json.

    The harness assembles the facts; this renders the ONE profile text from them
    and stamps its digest and the wrap, so the profile the kernel enforces and
    the block the contract binds cannot drift apart. The record is written
    exclusively (x mode): a capture never overwrites a seal.
    """
    payload = parse_json_bytes(
        read_regular_bytes(payload_path, "seal payload", maximum=MAX_INPUT_BYTES),
        "seal payload",
    )
    fields = require_exact_object(payload, SEAL_PAYLOAD_KEYS, "seal payload")
    sealed = fields["seal_mode"] == "seatbelt"
    record: dict[str, Any] = {
        "schema": SEAL_RECORD_SCHEMA,
        "seal_mode": fields["seal_mode"],
        "coverage_eligible": sealed,
        "mechanism": SEAL_MECHANISM if sealed else None,
        "sandbox_exec": fields["sandbox_exec"] or None,
        "platform": fields["platform"],
        "profile_file": fields["profile_file"] or None,
        "denied_read_roots": list(fields["denied_read_roots"]),
        "denied_read_data_roots": list(fields["denied_read_data_roots"]),
        "denied_link_roots": list(fields["denied_link_roots"]),
        "writable_roots": list(fields["writable_roots"]),
        "dev_write_paths": list(fields["dev_write_paths"]),
        "allowed_read_paths": list(fields["allowed_read_paths"]),
        "launcher_chain": list(fields["launcher_chain"]),
        "launcher_sha256": fields["launcher_sha256"] or None,
        "timeout_bin": fields["timeout_bin"] or None,
        "rep_env": dict(fields["rep_env"]),
        "env_allowlist": sorted(fields["env_allowlist"]),
        "real_home": fields["real_home"],
        "real_codex_home": fields["real_codex_home"],
        "real_tmpdir": fields["real_tmpdir"] or None,
        "cache_root": fields["cache_root"] or None,
        "git_common_root": fields["git_common_root"] or None,
        "run_root": fields["run_root"] or None,
        "workspace_root": fields["workspace_root"] or None,
        "dispatch_root": fields["dispatch_root"] or None,
        "network": dict(fields["network"]),
        "config_sanitized": fields["config_sanitized"],
        "config_sha256": fields["config_sha256"] or None,
        "config_text": fields["config_text"] or None,
        "auth_copied": bool(fields["auth_copied"]),
    }
    if sealed:
        # Round-trip through the bound block so the rendered profile is exactly
        # what a verifier will rebuild from the contract, not a parallel string.
        placeholder = "(version 1)"
        block = seal_block_from_record(
            dict(
                record,
                profile=placeholder,
                profile_sha256=digest_bytes(placeholder.encode("utf-8")),
                wrap=[],
            ),
            "seal payload",
            fields["run_root"] or "/",
        )
        profile = render_seal_profile(block)
        record["profile"] = profile
        record["profile_sha256"] = digest_bytes(profile.encode("utf-8"))
        record["wrap"] = [fields["sandbox_exec"], "-p", record["profile_sha256"]]
    else:
        record["profile"] = None
        record["profile_sha256"] = None
        record["wrap"] = []
    encoded = json.dumps(record, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    with open(output_path, "x", encoding="utf-8") as handle:
        handle.write(encoded)
    return record


def proxy_egress_lines(raw: bytes, rep: str) -> tuple[list[dict[str, Any]], bytes, int]:
    """The proxy log lines belonging to ONE rep, and their exact bytes.

    The digest is over the rep's OWN lines, not the whole file: the proxy keeps
    running while the next rep starts and a connection can be logged after the
    rep that opened it has been summarized, so a whole-file digest is only ever
    valid for the instant it was taken and never again.
    """
    entries: list[dict[str, Any]] = []
    kept: list[bytes] = []
    unparseable = 0
    for line in raw.split(b"\n"):
        stripped = line.strip()
        if not stripped:
            continue
        try:
            entry = json.loads(stripped)
        except ValueError:
            unparseable += 1
            continue
        if not isinstance(entry, dict) or entry.get("rep") != rep:
            continue
        entries.append(entry)
        kept.append(stripped)
    return entries, b"\n".join(kept) + (b"\n" if kept else b""), unparseable


def proxy_egress_summary(log_path: Path, rep: str) -> dict[str, Any]:
    """One rep's CONNECT decisions plus the digest of that rep's log lines."""
    raw = read_regular_bytes(log_path, "proxy log", maximum=MAX_TRANSCRIPT_BYTES)
    entries, own_bytes, unparseable = proxy_egress_lines(raw, rep)
    allowed = 0
    refused = unparseable
    detail: list[str] = ["unparseable proxy log line"] * unparseable
    for entry in entries:
        decision = entry.get("decision")
        # `attempt` is the pre-decision trace of the same connection, not an
        # outcome; counting it would make every allowed CONNECT look refused.
        if decision == "attempt":
            continue
        if decision == "allowed":
            allowed += 1
        else:
            refused += 1
            detail.append(f"{decision}: {entry.get('host')}:{entry.get('port')}")
    return {
        "allowed": allowed,
        "refused": refused,
        "log_sha256": digest_bytes(own_bytes),
        "detail": detail,
    }


def read_seal_file(
    fixture_dir: Path, skills_dir: Path, seal_path: Path | None = None
) -> dict[str, Any]:
    """Build the contract seal block from the stage's seal.json (absent = none).

    An explicit seal_path (the harness's workspace copy) takes precedence over
    the stage sidecar.
    """
    path = seal_path if seal_path is not None else fixture_dir / SEAL_NAME
    if path.is_symlink():
        raise MetadataError(f"{SEAL_NAME} must be a regular non-symlink file")
    repo_root = capture_repository_root(skills_dir)
    if not path.exists():
        if seal_path is not None:
            raise MetadataError(f"seal record not found: {seal_path}")
        home = original_home()
        return {
            "mode": "none",
            "platform": platform.system(),
            "mechanism": None,
            "sandbox_exec": None,
            "wrap": [],
            "denied_read_roots": [],
            "denied_read_data_roots": [],
            "denied_link_roots": [],
            "writable_roots": [],
            "allowed_read_paths": [],
            "rep_env": {"HOME": home, "CODEX_HOME": os.path.join(home, ".codex"),
                        "TMPDIR": os.path.normpath(tempfile.gettempdir())},
            "run_root": None,
            "workspace_root": None,
            "dispatch_root": None,
            "git_common_root": None,
            "real_tmpdir": None,
            "config_sanitized": None,
            "auth_copied": False,
            "network": {
                "mode": "open", "hosts": [], "ports": [], "proxy": None,
                "unix_sockets": [],
            },
            "timeout_bin": None,
            "dev_write_paths": [],
            "cache_root": None,
            "real_codex_home": os.path.join(home, ".codex"),
            "launcher_chain": [],
            "launcher_sha256": None,
            "config_sha256": None,
            "config_text": None,
            "env_allowlist": [],
            "profile_sha256": None,
            "original_home": home,
            "repository_root": repo_root,
        }
    record = parse_json_bytes(
        read_regular_bytes(path, SEAL_NAME, maximum=MAX_INPUT_BYTES), SEAL_NAME
    )
    return seal_block_from_record(record, SEAL_NAME, repo_root)


def validate_seal(value: Any) -> dict[str, Any]:
    """Accept the hardened seal block or the 2026-09-03 first shape.

    Both shapes replay. Only the hardened shape can be tier coverage; the
    narrower one is refused by verify_seal_for_coverage, not here, so an older
    capture stays readable instead of failing to load at all.
    """
    if not isinstance(value, dict):
        raise MetadataError("capture contract seal must be a JSON object")
    keys = set(value)
    shape = next((candidate for candidate in SEAL_SHAPES if keys == candidate), SEAL_KEYS)
    seal = require_exact_object(value, shape, "capture contract seal")
    mode = require_text(seal["mode"], "seal mode")
    if mode not in SEAL_MODES | {LEGACY_SEAL_MODE}:
        raise MetadataError(f"seal mode is unsupported: {mode!r}")
    if mode == LEGACY_SEAL_MODE:
        if seal != legacy_seal():
            raise MetadataError("legacy-unsealed seal must carry no roots or profile")
        return seal
    require_path_list(seal["denied_read_roots"], "seal denied_read_roots")
    require_path_list(seal["writable_roots"], "seal writable_roots")
    require_absolute_path(seal["original_home"], "seal original_home")
    require_absolute_path(seal["repository_root"], "seal repository_root")
    if keys != LEGACY_SEAL_KEYS:
        require_path_list(seal["denied_read_data_roots"], "seal denied_read_data_roots")
        require_path_list(seal["denied_link_roots"], "seal denied_link_roots")
        require_path_list(seal["allowed_read_paths"], "seal allowed_read_paths")
        require_exact_object(seal["rep_env"], SEAL_REP_ENV_KEYS, "seal rep_env")
    if keys == SEAL_KEYS:
        require_exact_object(seal["network"], SEAL_NETWORK_KEYS, "seal network")
        require_path_list(seal["dev_write_paths"], "seal dev_write_paths")
        require_path_list(seal["launcher_chain"], "seal launcher_chain")
        require_absolute_path(seal["real_codex_home"], "seal real_codex_home")
    profile_sha256 = seal["profile_sha256"]
    if mode == "seatbelt":
        if not isinstance(profile_sha256, str) or not SHA256_RE.fullmatch(profile_sha256):
            raise MetadataError("seatbelt seal profile_sha256 must be a sha256 digest")
    elif profile_sha256 is not None:
        raise MetadataError("seal profile_sha256 must be null when the seal mode is none")
    return seal


def path_forms(path: str) -> set[str]:
    """A path and, when it exists here, its realpath; the kernel seals the latter."""
    forms = {path}
    try:
        if os.path.lexists(path):
            forms.add(os.path.realpath(path))
    except OSError:
        pass
    return forms


def seal_covers(denied_roots: list[str], required: str) -> bool:
    candidates = path_forms(required)
    for root in denied_roots:
        for denied in path_forms(root):
            prefix = denied.rstrip("/") + "/"
            if any(item == denied or item.startswith(prefix) for item in candidates):
                return True
    return False


def required_sealed_roots(seal: dict[str, Any]) -> list[str]:
    """Roots a counted row must prove were unreadable.

    The original home subsumes the four skill roots, but they stay listed so a
    seal that moved HOME still has to name them. The git common directory's
    parent is the MAIN checkout a linked worktree shares: the same canonical
    SKILL.md bytes under a path the checkout deny does not reach. The real
    TMPDIR is where every other probe run's debris lives.
    """
    home = seal["original_home"]
    roots = [seal["repository_root"], home]
    roots += [os.path.join(home, relative) for relative in SEALED_SKILL_ROOTS]
    # real_codex_home can be configured OUTSIDE the home (its sessions and
    # rollouts carry canonical text), and cache_root is the operator-writable
    # Darwin cache directory beside the temp root. Both were recorded and
    # neither was required, so a seal could name them and deny neither.
    for key in ("git_common_root", "real_tmpdir", "real_codex_home", "cache_root"):
        value = seal.get(key)
        if value:
            roots.append(value)
    return roots


def unsealed_roots(seal: dict[str, Any]) -> list[str]:
    """Required roots the recorded seal did not deny reads under."""
    return [
        root
        for root in required_sealed_roots(seal)
        if not seal_covers(seal["denied_read_roots"], root)
    ]


def seal_coverage_failure(seal: dict[str, Any]) -> str | None:
    """The reason this seal cannot be tier coverage, or None when it can.

    Every bound field is checked, not just the mode: a hand-written record
    claiming `seal_mode: seatbelt` on Linux with no mechanism and a two-line
    profile passed the mode-and-roots check that shipped on 2026-09-03.
    """
    mode = seal["mode"]
    if mode == LEGACY_SEAL_MODE:
        return (
            "tier coverage requires a seatbelt-sealed capture; this capture "
            "contract predates the filesystem seal (legacy-unsealed)"
        )
    if mode != "seatbelt":
        return (
            f"tier coverage requires a seatbelt-sealed capture; seal mode {mode!r} "
            "cannot prove the checkout and skill roots were unreadable"
        )
    if set(seal) == LEGACY_SEAL_KEYS:
        return (
            "tier coverage requires the hardened seal block (2026-09-03 second "
            "pass): this contract binds only the mode, roots and profile digest, "
            "so it cannot prove the mechanism, the wrap, the link denies, the "
            "rep environment or the config sanitization"
        )
    if set(seal) == PASS2_SEAL_KEYS:
        return (
            "tier coverage requires the hardened seal block (2026-09-03 third "
            "pass): this contract cannot reconstruct its own profile and records "
            "no network mode, so it proves nothing about what the rep fetched"
        )
    if set(seal) == PASS3_SEAL_KEYS:
        return (
            "tier coverage requires the hardened seal block (2026-09-03 fourth "
            "pass): this contract records no timeout binary, so the wrapper that "
            "ran inside the sandbox is unaccounted for"
        )
    if seal["platform"] != SEAL_PLATFORM:
        return (
            f"tier coverage requires a {SEAL_PLATFORM} seatbelt capture; the seal "
            f"records platform {seal['platform']!r}"
        )
    if seal["mechanism"] != SEAL_MECHANISM:
        return (
            f"tier coverage requires mechanism {SEAL_MECHANISM!r}; the seal records "
            f"{seal['mechanism']!r}"
        )
    sandbox_exec = seal["sandbox_exec"]
    if not isinstance(sandbox_exec, str) or not sandbox_exec.startswith("/"):
        return "tier coverage requires the resolved sandbox-exec path in the seal"
    # The wrap must name the RECORDED binary, not whatever `sandbox-exec`
    # resolved to on PATH at dispatch time (a stub earlier on PATH would have
    # run instead, and the record would still have said /usr/bin/sandbox-exec).
    expected_wrap = [seal["sandbox_exec"], "-p", seal["profile_sha256"]]
    if seal["wrap"] != expected_wrap:
        return (
            "tier coverage requires the dispatch wrap to be the bound profile: "
            f"expected {expected_wrap}, seal records {seal['wrap']}"
        )
    if seal["config_sanitized"] is None:
        return (
            "tier coverage requires the rep's producer config to be generated; "
            "the operator's own config starts their MCP servers inside the rep"
        )
    if seal["config_sha256"] is None or not seal["config_text"]:
        return (
            "tier coverage requires the generated config's exact text and digest "
            "in the seal, so the file every rep ran under is inspectable"
        )
    if digest_bytes(seal["config_text"].encode("utf-8")) != seal["config_sha256"]:
        return "tier coverage requires config_sha256 to match config_text"
    if seal["sandbox_exec"] != SEAL_SANDBOX_EXEC_PATH:
        return (
            "tier coverage requires the system seatbelt binary "
            f"{SEAL_SANDBOX_EXEC_PATH}; the seal records {seal['sandbox_exec']!r}"
        )
    network = seal["network"]
    if network["mode"] != SEAL_NETWORK_MODE:
        return (
            f"tier coverage requires network mode {SEAL_NETWORK_MODE!r}; the seal "
            f"records {network['mode']!r}, so the rep could reach any host and "
            "fetch the canonical SKILL.md over the network"
        )
    if not network["hosts"]:
        return "tier coverage requires a non-empty network host allowlist"
    # Recording a host list is not constraining one. Pinning the permitted set
    # here is what stops a capture from allowlisting the forge, recording that
    # it did, and still counting.
    extra_hosts = sorted(set(network["hosts"]) - PERMITTED_EGRESS_HOSTS)
    if extra_hosts:
        return (
            "tier coverage permits egress only to the pinned host set; this seal "
            "also allows: " + ", ".join(extra_hosts)
        )
    if network["ports"] != PERMITTED_EGRESS_PORTS:
        return (
            f"tier coverage permits egress only on ports {PERMITTED_EGRESS_PORTS}; "
            f"the seal records {network['ports']}"
        )
    if network["unix_sockets"]:
        return (
            "tier coverage permits no unix-socket egress; the seal allows: "
            + ", ".join(network["unix_sockets"])
        )
    try:
        seal_network_port(network)
    except MetadataError as exc:
        return f"tier coverage requires a local proxy authority: {exc}"
    if not seal["timeout_bin"]:
        return (
            "tier coverage requires the resolved timeout binary in the seal; the "
            "wrapper runs inside the sandbox and a PATH lookup is not the record"
        )
    for field in ("real_tmpdir", "git_common_root"):
        if not seal[field]:
            return (
                f"tier coverage requires {field} in the seal; a null value leaves "
                "the required-root check with nothing to compare"
            )
    for field, root in (
        ("repository_root", seal["repository_root"]),
        ("git_common_root", seal["git_common_root"]),
        ("original_home", seal["original_home"]),
    ):
        if seal_covers(seal["writable_roots"], root):
            return (
                "tier coverage refuses a seal whose writable roots reach the "
                f"{field}: {root}"
            )
    if not seal["auth_copied"]:
        return (
            "tier coverage requires auth.json to be COPIED into the scratch "
            "CODEX_HOME; a symlink cannot resolve under the read-denied home"
        )
    missing = unsealed_roots(seal)
    if missing:
        return (
            "tier coverage requires the seal to deny reads under the checkout, the "
            "original home, the shared git checkout and the real temp root; "
            "denied_read_roots omit: " + ", ".join(missing)
        )
    dispatch = seal["dispatch_root"]
    if not dispatch:
        return "tier coverage requires the harness dispatch directory in the seal"
    if not seal_covers(seal["denied_read_data_roots"], dispatch):
        return (
            "tier coverage requires the dispatch directory to be read-data denied: "
            f"{dispatch}"
        )
    if not seal_covers(seal["denied_link_roots"], dispatch):
        return (
            "tier coverage requires link and clone to be denied on the dispatch "
            f"directory: {dispatch}"
        )
    for root in (seal["repository_root"], seal["original_home"]):
        if not seal_covers(seal["denied_link_roots"], root):
            return (
                "tier coverage requires link and clone to be denied wherever reads "
                f"are; denied_link_roots omit: {root}"
            )
    workspace = seal["workspace_root"]
    if not workspace or not seal_covers(seal["writable_roots"], workspace):
        return (
            "tier coverage requires the rep workspace to sit under the seal's "
            f"writable roots: {workspace!r}"
        )
    for key in ("HOME", "CODEX_HOME", "TMPDIR"):
        value = seal["rep_env"][key]
        if not seal_covers(seal["writable_roots"], value):
            return (
                f"tier coverage requires the rep's {key} to be a writable scratch "
                f"path inside the seal: {value}"
            )
        if seal_covers([seal["original_home"]], value):
            return (
                f"tier coverage requires the rep's {key} to sit OUTSIDE the "
                f"operator's home: {value}"
            )
    # The launcher exception is the one hole in the read denies, so it is tied to
    # the producer the contract bound: exactly the resolved launcher chain, and
    # only the links of it that a denied root would otherwise cover. Without
    # this, a HOME path holding a SKILL.md could be listed as an allowed
    # "launcher" and read straight through the seal.
    expected_allowed = [
        path
        for path in seal["launcher_chain"]
        if seal_covers(seal["denied_read_roots"], path)
    ]
    if seal["allowed_read_paths"] != expected_allowed:
        return (
            "tier coverage requires allowed_read_paths to be exactly the bound "
            f"launcher chain under a denied root; expected {expected_allowed}, "
            f"seal records {seal['allowed_read_paths']}"
        )
    forbidden = [
        ("repository_root", seal["repository_root"]),
        ("git_common_root", seal["git_common_root"]),
        ("real_codex_home", seal["real_codex_home"]),
    ] + [
        ("skill root", os.path.join(seal["original_home"], relative))
        for relative in SEALED_SKILL_ROOTS
    ]
    for allowed in seal["allowed_read_paths"]:
        for label, root in forbidden:
            if root and seal_covers([root], allowed):
                return (
                    "tier coverage refuses a seal that re-allows a read inside the "
                    f"{label}: {allowed}"
                )
    # Every writable root must sit inside the run directory. "Not the checkout,
    # not the home" left the rest of the filesystem available to a record that
    # simply named it.
    run_root = seal["run_root"]
    if not run_root:
        return "tier coverage requires the run root in the seal"
    for root in seal["writable_roots"]:
        if not seal_covers([run_root], root):
            return (
                "tier coverage requires every writable root under the run "
                f"directory; the seal also allows writes to: {root}"
            )
    if seal["dev_write_paths"] != PERMITTED_DEV_WRITE_PATHS:
        return (
            f"tier coverage permits exactly the devices {PERMITTED_DEV_WRITE_PATHS}; "
            f"the seal records {seal['dev_write_paths']}"
        )
    extra_env = sorted(
        name
        for name in seal["env_allowlist"]
        if name not in PERMITTED_ENV_ALLOWLIST
        and not name.startswith(SEAL_ENV_SEAM_PREFIX)
    )
    if extra_env:
        return (
            "tier coverage permits only the pinned rep environment plus "
            f"{SEAL_ENV_SEAM_PREFIX}* seams; this seal also passes: "
            + ", ".join(extra_env)
        )
    # The launcher chain is the read exception, so it has to be a real chain:
    # every link an existing file, and the last one the binary whose digest the
    # producer identity also carries.
    chain = seal["launcher_chain"]
    if not chain:
        return "tier coverage requires the producer launcher chain in the seal"
    for link in chain:
        if not os.path.isfile(link):
            return (
                "tier coverage requires every launcher_chain entry to be an "
                f"existing file; missing: {link}"
            )
    try:
        final_digest = digest_file(Path(chain[-1]))
    except (OSError, MetadataError) as exc:
        return f"tier coverage could not digest the bound launcher: {exc}"
    if final_digest != seal["launcher_sha256"]:
        return (
            "tier coverage requires launcher_sha256 to be the digest of the last "
            f"launcher_chain entry; recorded {seal['launcher_sha256']}, "
            f"{chain[-1]} is {final_digest}"
        )
    # The last check is the one that makes every check above load-bearing: the
    # block must reproduce the exact profile bytes the kernel enforced.
    try:
        rendered = render_seal_profile(seal)
    except MetadataError as exc:
        return f"tier coverage could not rebuild the profile from the seal block: {exc}"
    if digest_bytes(rendered.encode("utf-8")) != seal["profile_sha256"]:
        return (
            "tier coverage requires the seal block to rebuild its own profile: "
            "the profile digest does not match the block, so the recorded roots "
            "are assertions rather than the bytes the kernel enforced"
        )
    return None


def verify_seal_for_coverage(seal: dict[str, Any]) -> None:
    failure = seal_coverage_failure(seal)
    if failure is not None:
        raise MetadataError(failure)


def counterbalanced_schedule(reps: int) -> list[dict[str, Any]]:
    require_reps(reps)
    schedule: list[dict[str, Any]] = []
    position = 1
    for rep in range(1, reps + 1):
        arms = ("control", "treatment") if rep % 2 else ("treatment", "control")
        for arm in arms:
            schedule.append({"position": position, "rep": rep, "arm": arm})
            position += 1
    return schedule


def validate_schedule(value: Any, reps: int) -> list[dict[str, Any]]:
    expected = counterbalanced_schedule(reps)
    if value != expected:
        raise MetadataError(
            "fixture schedule must equal the deterministic counterbalanced schedule"
        )
    return expected


def build_capture_contract(
    probe_dir: Path,
    skills_dir: Path,
    expected_probe: str,
    producer_request: dict[str, Any],
    seal: dict[str, Any],
) -> dict[str, Any]:
    seal = validate_seal(seal)
    if seal["mode"] == LEGACY_SEAL_MODE:
        raise MetadataError("a new capture contract cannot be legacy-unsealed")
    probe_meta = load_json(probe_dir / "probe.json")
    probe_contract = validate_probe_metadata(
        probe_meta, expected_probe, require_treatment=True
    )
    treatment_source = probe_contract["treatment_source"]
    assert isinstance(treatment_source, str)
    inputs = embedded_input_records(probe_dir, treatment_source)
    decoded_inputs, _ = decode_capture_inputs(inputs, expected_probe, treatment_source)
    embedded_skill = build_embedded_canonical_skill_record(
        probe_dir, skills_dir, expected_probe
    )
    _, canonical_skill_bytes = validate_embedded_canonical_skill(
        embedded_skill, probe_meta, expected_probe
    )
    payload = {
        "schema": CAPTURE_CONTRACT_SCHEMA,
        "probe": expected_probe,
        "reps": probe_contract["reps"],
        "capture_inputs": inputs,
        "canonical_skill": embedded_skill,
        "treatment_source": treatment_source,
        "prompts": prompt_records(
            decoded_inputs, canonical_skill_bytes, treatment_source
        ),
        "producer_request": validate_producer_request(
            producer_request, seal_mode=seal["mode"]
        ),
        "schedule": counterbalanced_schedule(probe_contract["reps"]),
        "scoring": {
            "response_extraction": RESPONSE_EXTRACTION,
            "transcript_format": TRANSCRIPT_FORMAT,
            "discriminator_timeout_seconds": DISCRIMINATOR_TIMEOUT_SECONDS,
        },
        "seal": seal,
    }
    return {**payload, "binding_sha256": digest_bytes(canonical_bytes(payload))}


def validate_capture_contract(value: Any, expected_probe: str) -> dict[str, Any]:
    """Validate a capture contract; legacy v2 contracts return a legacy-unsealed seal."""
    keys = {
        "schema",
        "probe",
        "reps",
        "capture_inputs",
        "canonical_skill",
        "treatment_source",
        "prompts",
        "producer_request",
        "schedule",
        "scoring",
        "binding_sha256",
    }
    schema = value.get("schema") if isinstance(value, dict) else None
    if schema == CAPTURE_CONTRACT_SCHEMA:
        keys = keys | {"seal"}
    elif schema != LEGACY_CAPTURE_CONTRACT_SCHEMA:
        raise MetadataError("unsupported capture contract schema")
    contract = require_exact_object(value, keys, "capture contract")
    if schema == CAPTURE_CONTRACT_SCHEMA:
        seal = validate_seal(contract["seal"])
        if seal["mode"] == LEGACY_SEAL_MODE:
            raise MetadataError("a v3 capture contract cannot be legacy-unsealed")
    else:
        seal = legacy_seal()
    if contract["probe"] != expected_probe:
        raise MetadataError("capture contract probe mismatch")
    reps = require_reps(contract["reps"])
    treatment_source = require_text(
        contract["treatment_source"], "capture contract treatment_source"
    )
    if treatment_source not in TREATMENT_SOURCES:
        raise MetadataError("capture contract treatment source is unsupported")
    inputs, probe_contract = decode_capture_inputs(
        contract["capture_inputs"], expected_probe, treatment_source
    )
    probe_meta = parse_json_bytes(inputs["probe.json"], "captured probe.json")
    if probe_contract["reps"] != reps:
        raise MetadataError("capture contract reps disagree with captured probe.json")
    if probe_contract["treatment_source"] != contract["treatment_source"]:
        raise MetadataError(
            "capture contract treatment source disagrees with captured probe.json"
        )
    _, canonical_skill_bytes = validate_embedded_canonical_skill(
        contract["canonical_skill"], probe_meta, expected_probe
    )
    expected_prompts = prompt_records(inputs, canonical_skill_bytes, treatment_source)
    decode_prompts(contract["prompts"])
    if contract["prompts"] != expected_prompts:
        raise MetadataError("capture contract prompts disagree with bound input bytes")
    # A v2 contract binds v2-era eligibility; the legacy-unsealed seal is what
    # makes the row ineligible, so only the one-way floor is checked here.
    validate_producer_request(
        contract["producer_request"],
        seal_mode=seal["mode"] if schema == CAPTURE_CONTRACT_SCHEMA else None,
    )
    validate_schedule(contract["schedule"], reps)
    if contract["scoring"] != {
        "response_extraction": RESPONSE_EXTRACTION,
        "transcript_format": TRANSCRIPT_FORMAT,
        "discriminator_timeout_seconds": DISCRIMINATOR_TIMEOUT_SECONDS,
    }:
        raise MetadataError("capture contract scoring semantics are unsupported")
    binding = contract["binding_sha256"]
    if not isinstance(binding, str) or not SHA256_RE.fullmatch(binding):
        raise MetadataError("capture contract binding must be a sha256 digest")
    payload = {key: item for key, item in contract.items() if key != "binding_sha256"}
    if digest_bytes(canonical_bytes(payload)) != binding:
        raise MetadataError("capture contract binding mismatch")
    if schema == CAPTURE_CONTRACT_SCHEMA:
        return contract
    return {**contract, "seal": seal}


def write_exclusive_bytes(path: Path, data: bytes, label: str) -> os.stat_result:
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
    try:
        fd = os.open(path, flags, 0o644)
    except FileExistsError as exc:
        raise MetadataError(f"refusing to replace existing immutable {label}") from exc
    try:
        identity = os.fstat(fd)
        offset = 0
        while offset < len(data):
            offset += os.write(fd, data[offset:])
        os.fsync(fd)
        current = os.stat(path, follow_symlinks=False)
        if not os.path.samestat(identity, current):
            raise MetadataError(f"{label} identity changed while writing")
        return identity
    finally:
        os.close(fd)


def write_capture_contract(
    fixture_dir: Path,
    probe_dir: Path,
    skills_dir: Path,
    expected_probe: str,
    requested_model: str | None,
    requested_effort: str | None,
    producer_override: str | None,
    seal_path: Path | None = None,
) -> dict[str, Any]:
    validate_fixture_dir(fixture_dir)
    if set(os.listdir(fixture_dir)) - {SEAL_NAME}:
        raise MetadataError(
            "capture snapshot requires an empty stage before any transcript exists"
        )
    seal = read_seal_file(fixture_dir, skills_dir, seal_path)
    producer_request = producer_runtime_identity(
        requested_model, requested_effort, producer_override, seal["mode"]
    )
    contract = build_capture_contract(
        probe_dir, skills_dir, expected_probe, producer_request, seal
    )
    encoded = (
        json.dumps(contract, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    ).encode("utf-8")
    write_exclusive_bytes(
        fixture_dir / CAPTURE_CONTRACT_NAME, encoded, CAPTURE_CONTRACT_NAME
    )
    return contract


def load_capture_contract(fixture_dir: Path, expected_probe: str) -> dict[str, Any]:
    return validate_capture_contract(
        load_json(fixture_dir / CAPTURE_CONTRACT_NAME), expected_probe
    )


def verify_network_log(fixture_dir: Path, transcripts: list[str]) -> None:
    """The published proxy log must reproduce what EVERY rep bound.

    Each rep binds the digest and the counts of its own lines, so the published
    file is checked rep by rep. That is what turns a bound digest into a
    checkable fact: without the file, the digest is a number with nothing to
    compare against.
    """
    path = fixture_dir / NETWORK_LOG_NAME
    if path.is_symlink() or not path.is_file():
        raise MetadataError(f"{NETWORK_LOG_NAME} must be a regular non-symlink file")
    raw = read_regular_bytes(path, NETWORK_LOG_NAME, maximum=MAX_TRANSCRIPT_BYTES)
    checked = 0
    for name in transcripts:
        transcript = fixture_dir / name
        if not transcript.is_file():
            continue
        first_line = transcript.read_bytes().split(b"\n", 1)[0]
        try:
            event = json.loads(first_line)
        except ValueError:
            continue
        if not isinstance(event, dict):
            continue
        egress = event.get("network_egress")
        if not isinstance(egress, dict):
            continue
        rep_key = f"{event.get('arm')}-{event.get('rep')}"
        entries, own_bytes, _ = proxy_egress_lines(raw, rep_key)
        if digest_bytes(own_bytes) != egress.get("log_sha256"):
            raise MetadataError(
                f"{NETWORK_LOG_NAME} does not reproduce the lines {rep_key} bound"
            )
        allowed = sum(1 for entry in entries if entry.get("decision") == "allowed")
        if allowed != egress.get("allowed"):
            raise MetadataError(
                f"{NETWORK_LOG_NAME} allowed count for {rep_key} disagrees with its transcript"
            )
        checked += 1
    if not checked:
        raise MetadataError(
            f"{NETWORK_LOG_NAME} is present but no transcript binds a proxy log digest"
        )


def verify_seal_sidecar(
    fixture_dir: Path, skills_dir: Path, capture_contract: dict[str, Any]
) -> None:
    """A seal.json left beside the contract must re-derive to the bound seal block."""
    if capture_contract["schema"] != CAPTURE_CONTRACT_SCHEMA:
        raise MetadataError(
            f"{SEAL_NAME} is present but the capture contract predates the seal"
        )
    observed = seal_projection(read_seal_file(fixture_dir, skills_dir))
    if observed != seal_projection(capture_contract["seal"]):
        raise MetadataError(
            f"{SEAL_NAME} disagrees with the seal bound in the capture contract"
        )


def fixture_seal(
    fixture_dir: Path, manifest: dict[str, Any], expected_probe: str
) -> dict[str, Any]:
    if manifest["schema"] != SCHEMA:
        return legacy_seal()
    return load_capture_contract(fixture_dir, expected_probe)["seal"]


def observed_producer_bytes(data: bytes, label: str) -> tuple[str, str]:
    """Read the model and reasoning effort from the first Codex header block."""
    try:
        lines = data.decode("utf-8", errors="strict").splitlines()
    except UnicodeError as exc:
        raise MetadataError(f"cannot parse producer header in {label}: {exc}") from exc

    try:
        first_boundary = lines.index("--------")
        second_boundary = lines.index("--------", first_boundary + 1)
    except ValueError as exc:
        raise MetadataError(f"{label} has no complete Codex transcript header") from exc

    header = lines[first_boundary + 1 : second_boundary]
    models = [
        line.removeprefix("model:").strip()
        for line in header
        if line.startswith("model:")
    ]
    efforts = [
        line.removeprefix("reasoning effort:").strip()
        for line in header
        if line.startswith("reasoning effort:")
    ]
    if len(models) != 1 or not models[0]:
        raise MetadataError(f"{label} must have exactly one non-empty model header")
    if len(efforts) != 1 or not efforts[0]:
        raise MetadataError(
            f"{label} must have exactly one non-empty reasoning effort header"
        )
    require_text(models[0], f"{label} observed model")
    require_text(efforts[0], f"{label} observed reasoning effort")
    return models[0], efforts[0]


def observed_producer(path: Path) -> tuple[str, str]:
    return observed_producer_bytes(
        read_regular_bytes(path, "transcript", maximum=MAX_TRANSCRIPT_BYTES),
        path.name,
    )


def observe_paths(paths: list[Path]) -> dict[str, str]:
    if not paths:
        raise MetadataError(
            "no captured transcripts are available to identify the producer"
        )
    observed: set[tuple[str, str]] = set()
    for path in paths:
        validate_transcript(path)
        observed.add(observed_producer(path))
    if len(observed) != 1:
        rendered = ", ".join(f"{model}/{effort}" for model, effort in sorted(observed))
        raise MetadataError(f"captured transcripts disagree on producer: {rendered}")
    model, effort = observed.pop()
    return {"adapter": "codex", "model": model, "effort": effort}


def transcript_paths(fixture_dir: Path) -> list[Path]:
    paths = [
        path for path in fixture_dir.iterdir() if TRANSCRIPT_RE.fullmatch(path.name)
    ]
    return sorted(paths, key=lambda path: path.name)


def build_payload(
    fixture_dir: Path,
    probe_dir: Path,
    skills_dir: Path,
    harness: Path,
    preamble: Path,
    dispatch_helper: Path,
    probe: str,
    reps: int,
    requested_model: str | None,
    requested_effort: str | None,
) -> dict[str, Any]:
    capture_path = fixture_dir / CAPTURE_CONTRACT_NAME
    if not capture_path.exists() or capture_path.is_symlink():
        raise MetadataError(
            "v3 create requires a pre-existing capture contract written before execution"
        )
    capture_contract = load_capture_contract(fixture_dir, probe)
    expected = expected_transcripts(reps)
    actual = sorted(os.listdir(fixture_dir))
    expected_before_manifest = expected + [CAPTURE_CONTRACT_NAME]
    if SEAL_NAME in actual:
        verify_seal_sidecar(fixture_dir, skills_dir, capture_contract)
        expected_before_manifest.append(SEAL_NAME)
    if NETWORK_LOG_NAME in actual:
        verify_network_log(fixture_dir, expected)
        expected_before_manifest.append(NETWORK_LOG_NAME)
    if sorted(expected_before_manifest) != sorted(actual):
        missing = sorted(set(expected_before_manifest) - set(actual))
        extra = sorted(set(actual) - set(expected_before_manifest))
        detail = []
        if missing:
            detail.append("missing=" + ",".join(missing))
        if extra:
            detail.append("extra=" + ",".join(extra))
        raise MetadataError(
            "fixture transcript inventory mismatch (" + "; ".join(detail) + ")"
        )

    paths = [fixture_dir / name for name in expected]
    transcripts = []
    threads = []
    for name, path in zip(expected, paths, strict=True):
        match = TRANSCRIPT_RE.fullmatch(name)
        assert match is not None
        arm, rep_text = match.groups()
        transcript = read_regular_bytes(
            path, "transcript", maximum=MAX_TRANSCRIPT_BYTES
        )
        thread_id, _ = validate_structured_transcript(
            transcript, capture_contract, arm, int(rep_text), name
        )
        transcripts.append({"path": name, "sha256": digest_bytes(transcript)})
        threads.append({"path": name, "thread_id": thread_id})
    if len({entry["thread_id"] for entry in threads}) != len(threads):
        raise MetadataError("each probe dispatch must have a distinct Codex thread_id")

    if reps != capture_contract["reps"]:
        raise MetadataError(
            f"capture reps {reps} do not match bound probe.json reps "
            f"{capture_contract['reps']}"
        )
    capture_inputs = capture_contract["capture_inputs"]
    producer_request = validate_producer_request(capture_contract["producer_request"])
    expected_requested = {
        "model": producer_request["model"],
        "effort": producer_request["effort"],
    }
    supplied_requested = {
        "model": require_text(requested_model, "requested model", nullable=True),
        "effort": require_text(requested_effort, "requested effort", nullable=True),
    }
    if supplied_requested != expected_requested:
        raise MetadataError(
            "create requested producer does not match the pre-execution capture contract"
        )
    validate_regular_file(harness, "probe harness")
    validate_regular_file(preamble, "probe preamble")
    validate_regular_file(dispatch_helper, "Codex dispatch helper")
    helper = Path(__file__).resolve()
    validate_regular_file(helper, "fixture metadata helper")
    canonical_skill = capture_contract["canonical_skill"]
    treatment_source = capture_contract["treatment_source"]
    producer = {**producer_request, "threads": threads}

    return {
        "schema": SCHEMA,
        "probe": require_text(probe, "probe"),
        "reps": reps,
        "producer": producer,
        "requested_producer": expected_requested,
        "transcripts": transcripts,
        "capture_contract": {
            "path": CAPTURE_CONTRACT_NAME,
            "sha256": digest_file(capture_path),
        },
        "capture_inputs": capture_inputs,
        "canonical_skill": canonical_skill,
        "treatment_source": treatment_source,
        "prompts": capture_contract["prompts"],
        "schedule": capture_contract["schedule"],
        "scoring": capture_contract["scoring"],
        "capture_evaluator": {
            "harness": {
                "path": "scripts/probe-skill.sh",
                "sha256": digest_file(harness),
            },
            "preamble": {
                "path": "scripts/lib/preamble.sh",
                "sha256": digest_file(preamble),
            },
            "metadata_helper": {
                "path": "scripts/lib/probe-fixture-metadata.py",
                "sha256": digest_file(helper),
            },
            "dispatch_helper": {
                "path": "scripts/lib/codex-exec.sh",
                "sha256": digest_file(dispatch_helper),
            },
            # The proxy is the network half of the seal: change it and what a
            # rep could reach changes, so it belongs in the identity a scorecard
            # binds exactly as much as the harness does.
            "network_proxy": {
                "path": PROXY_RELATIVE_PATH,
                "sha256": digest_file(proxy_module_path()),
            },
        },
    }


def write_manifest(fixture_dir: Path, payload: dict[str, Any]) -> dict[str, Any]:
    """Create the stage manifest once without check/replace or rollback deletion."""
    manifest_path = fixture_dir / MANIFEST_NAME
    manifest = dict(payload)
    manifest["binding_sha256"] = digest_bytes(canonical_bytes(payload))
    encoded = (
        json.dumps(manifest, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    ).encode("utf-8")
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
    try:
        fd = os.open(manifest_path, flags, 0o644)
    except FileExistsError as exc:
        raise MetadataError(
            f"refusing to replace existing immutable {MANIFEST_NAME}"
        ) from exc
    try:
        identity = os.fstat(fd)
        offset = 0
        while offset < len(encoded):
            offset += os.write(fd, encoded[offset:])
        os.fsync(fd)
        current = os.stat(manifest_path, follow_symlinks=False)
        if not os.path.samestat(identity, current):
            raise MetadataError("fixture manifest identity changed while writing")
    finally:
        os.close(fd)
    return manifest


def rename_noreplace(source: Path, target: Path) -> None:
    """Atomically rename one directory while refusing any existing target."""
    libc = ctypes.CDLL(None, use_errno=True)
    source_bytes = os.fsencode(source)
    target_bytes = os.fsencode(target)
    result: int
    if sys.platform == "darwin" and hasattr(libc, "renamex_np"):
        renamex_np = libc.renamex_np
        renamex_np.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_uint]
        renamex_np.restype = ctypes.c_int
        result = renamex_np(source_bytes, target_bytes, 0x00000004)  # RENAME_EXCL
    elif hasattr(libc, "renameat2"):
        renameat2 = libc.renameat2
        renameat2.argtypes = [
            ctypes.c_int,
            ctypes.c_char_p,
            ctypes.c_int,
            ctypes.c_char_p,
            ctypes.c_uint,
        ]
        renameat2.restype = ctypes.c_int
        result = renameat2(
            getattr(os, "AT_FDCWD", -100),
            source_bytes,
            getattr(os, "AT_FDCWD", -100),
            target_bytes,
            1,  # RENAME_NOREPLACE
        )
    else:
        raise MetadataError(
            "this platform lacks an atomic no-replace directory rename primitive"
        )
    if result == 0:
        return
    error = ctypes.get_errno()
    if error in {errno.EEXIST, errno.ENOTEMPTY}:
        raise MetadataError(
            f"refusing to replace existing immutable fixture set: {target}"
        )
    raise MetadataError(
        f"could not atomically publish immutable fixture set: {os.strerror(error)}"
    )


def publish_fixture_set(
    stage_dir: Path,
    target_dir: Path,
    probe_dir: Path,
    skills_dir: Path,
    expected_probe: str,
) -> dict[str, Any]:
    """Publish a hidden verified stage with one atomic no-replace rename."""
    validate_fixture_dir(stage_dir)
    if not re.fullmatch(
        r"fixtures(?:[_-][A-Za-z0-9][A-Za-z0-9._-]*)?", target_dir.name
    ):
        raise MetadataError(f"unsafe fixture set target name: {target_dir.name!r}")
    if stage_dir.parent.is_symlink() or target_dir.parent.is_symlink():
        raise MetadataError("fixture set parent directory must not be a symlink")
    try:
        stage_parent = stage_dir.parent.resolve(strict=True)
        target_parent = target_dir.parent.resolve(strict=True)
    except OSError as exc:
        raise MetadataError(f"fixture set parent directory not found: {exc}") from exc
    if stage_parent != target_parent:
        raise MetadataError("staged and published fixture sets must share one parent")

    directory_flags = (
        os.O_RDONLY | getattr(os, "O_DIRECTORY", 0) | getattr(os, "O_NOFOLLOW", 0)
    )
    parent_fd = os.open(stage_parent, directory_flags)
    stage_fd = os.open(stage_dir.name, directory_flags, dir_fd=parent_fd)
    try:
        stage_identity = os.fstat(stage_fd)
        manifest = validate_manifest(
            stage_dir,
            probe_dir,
            skills_dir,
            expected_probe,
            require_current_inputs=True,
        )
        if manifest["schema"] != SCHEMA:
            raise MetadataError(
                "new publication requires self-contained fixture metadata v3"
            )
        verify_evaluator_files(manifest["capture_evaluator"], repository_root())
        manifest_after = validate_manifest(
            stage_dir,
            probe_dir,
            skills_dir,
            expected_probe,
            require_current_inputs=True,
        )
        if manifest_after["binding_sha256"] != manifest["binding_sha256"]:
            raise MetadataError("staged fixture binding changed during publish")
        verify_evaluator_files(manifest_after["capture_evaluator"], repository_root())
        stage_path_identity = os.stat(
            stage_dir.name, dir_fd=parent_fd, follow_symlinks=False
        )
        if not os.path.samestat(stage_identity, stage_path_identity):
            raise MetadataError(
                "staged fixture directory identity changed during publish"
            )
        rename_noreplace(stage_dir, target_dir)
        published_identity = os.stat(
            target_dir.name, dir_fd=parent_fd, follow_symlinks=False
        )
        if not os.path.samestat(stage_identity, published_identity):
            raise MetadataError(
                "published fixture directory identity is not the staged set"
            )
        published = validate_manifest(
            target_dir,
            probe_dir,
            skills_dir,
            expected_probe,
            require_current_inputs=True,
        )
        verify_evaluator_files(published["capture_evaluator"], repository_root())
        if published["binding_sha256"] != manifest["binding_sha256"]:
            raise MetadataError("published fixture binding differs from staged binding")
        os.fsync(parent_fd)
    finally:
        os.close(stage_fd)
        os.close(parent_fd)

    return {"binding_sha256": manifest.get("binding_sha256"), "target": target_dir.name}


def validate_hash_records(
    records: Any,
    expected_names: tuple[str, ...],
    root: Path,
    label: str,
) -> None:
    if not isinstance(records, list) or len(records) != len(expected_names):
        raise MetadataError(f"{label} must contain the exact declared file inventory")
    seen: list[str] = []
    for index, entry in enumerate(records):
        if not isinstance(entry, dict) or set(entry) != {"path", "sha256"}:
            raise MetadataError(
                f"{label}[{index}] must contain exactly path and sha256"
            )
        name = entry["path"]
        if name not in expected_names or name in seen:
            raise MetadataError(f"unsafe or duplicate {label} path: {name!r}")
        digest = entry["sha256"]
        if not isinstance(digest, str) or not SHA256_RE.fullmatch(digest):
            raise MetadataError(f"invalid {label} digest for {name}")
        path = root / name
        validate_regular_file(path, label)
        actual = digest_file(path)
        if actual != digest:
            raise MetadataError(
                f"{label} digest mismatch for {name}: expected {digest}, got {actual}"
            )
        seen.append(name)
    if tuple(seen) != expected_names:
        raise MetadataError(f"{label} inventory or ordering is invalid")


def validate_evaluator(
    value: Any, field: str, *, require_dispatch: bool | None = True
) -> None:
    legacy = {"harness", "metadata_helper"}
    pass3 = legacy | {"preamble", "dispatch_helper"}
    current = pass3 | {"network_proxy"}
    allowed = {frozenset(current)}
    if require_dispatch is False:
        allowed = {frozenset(legacy)}
    elif require_dispatch is None:
        allowed.add(frozenset(legacy))
        allowed.add(frozenset(pass3))
    else:
        # Sets captured before the proxy joined the identity still replay.
        allowed.add(frozenset(pass3))
    if not isinstance(value, dict) or frozenset(value) not in allowed:
        expected = (
            "harness, preamble, metadata_helper, dispatch_helper, and network_proxy"
        )
        if require_dispatch is False:
            expected = "harness and metadata_helper"
        elif require_dispatch is None:
            expected += " (or the legacy harness and metadata_helper pair)"
        raise MetadataError(f"{field} must contain exactly {expected}")
    expected_paths = {
        "harness": "scripts/probe-skill.sh",
        "preamble": "scripts/lib/preamble.sh",
        "metadata_helper": "scripts/lib/probe-fixture-metadata.py",
        "dispatch_helper": "scripts/lib/codex-exec.sh",
        "network_proxy": PROXY_RELATIVE_PATH,
    }
    for key in value:
        expected_path = expected_paths[key]
        record = value[key]
        if not isinstance(record, dict) or set(record) != {"path", "sha256"}:
            raise MetadataError(f"{field}.{key} must contain exactly path and sha256")
        if record["path"] != expected_path:
            raise MetadataError(f"unexpected {field}.{key} path: {record['path']!r}")
        if not isinstance(record["sha256"], str) or not SHA256_RE.fullmatch(
            record["sha256"]
        ):
            raise MetadataError(f"invalid {field}.{key} digest")


def evaluator_identity(
    harness: Path, preamble: Path, dispatch_helper: Path
) -> dict[str, Any]:
    validate_regular_file(harness, "probe harness")
    validate_regular_file(preamble, "probe preamble")
    validate_regular_file(dispatch_helper, "Codex dispatch helper")
    helper = Path(__file__).resolve()
    validate_regular_file(helper, "fixture metadata helper")
    return {
        "harness": {"path": "scripts/probe-skill.sh", "sha256": digest_file(harness)},
        "preamble": {
            "path": "scripts/lib/preamble.sh",
            "sha256": digest_file(preamble),
        },
        "metadata_helper": {
            "path": "scripts/lib/probe-fixture-metadata.py",
            "sha256": digest_file(helper),
        },
        "dispatch_helper": {
            "path": "scripts/lib/codex-exec.sh",
            "sha256": digest_file(dispatch_helper),
        },
        "network_proxy": {
            "path": PROXY_RELATIVE_PATH,
            "sha256": digest_file(proxy_module_path()),
        },
    }


def proxy_module_path() -> Path:
    """The CONNECT proxy beside this helper."""
    return Path(__file__).resolve().parent / "probe-connect-proxy.py"


def repository_root() -> Path:
    helper = Path(__file__).resolve()
    root = helper.parents[2]
    if not (root / "scripts" / "probe-skill.sh").is_file():
        raise MetadataError("could not resolve the probe evaluator repository root")
    return root


def verify_evaluator_files(value: Any, repo_root: Path) -> dict[str, Any]:
    validate_evaluator(value, "capture_evaluator", require_dispatch=True)
    expected_paths = {
        "harness": repo_root / "scripts" / "probe-skill.sh",
        "preamble": repo_root / "scripts" / "lib" / "preamble.sh",
        "metadata_helper": repo_root / "scripts" / "lib" / "probe-fixture-metadata.py",
        "dispatch_helper": repo_root / "scripts" / "lib" / "codex-exec.sh",
        "network_proxy": repo_root / "scripts" / "lib" / "probe-connect-proxy.py",
    }
    actual: dict[str, Any] = {}
    for key, path in expected_paths.items():
        if key not in value:
            continue
        validate_regular_file(path, f"repo-local evaluator {key}")
        actual[key] = {"path": value[key]["path"], "sha256": digest_file(path)}
    if actual != value:
        raise MetadataError(
            "capture evaluator hashes do not match the exact repo-local evaluator files"
        )
    return actual


def safe_repo_path(
    repo_root: Path,
    relative: str,
    *,
    expected_prefix: tuple[str, ...] | None = None,
    expect_directory: bool = False,
) -> Path:
    """Resolve one normalized repo-relative path without following symlinks."""
    require_text(relative, "repository-relative path")
    if "\\" in relative:
        raise MetadataError(f"unsafe repository-relative path: {relative!r}")
    pure = PurePosixPath(relative)
    if (
        pure.is_absolute()
        or not pure.parts
        or any(part in {"", ".", ".."} for part in pure.parts)
        or pure.as_posix() != relative
    ):
        raise MetadataError(f"unsafe repository-relative path: {relative!r}")
    if (
        expected_prefix is not None
        and pure.parts[: len(expected_prefix)] != expected_prefix
    ):
        prefix = "/".join(expected_prefix) + "/"
        raise MetadataError(f"evidence path must stay under {prefix}: {relative!r}")

    try:
        root = repo_root.resolve(strict=True)
    except OSError as exc:
        raise MetadataError(f"evidence root not found: {repo_root}") from exc
    if not root.is_dir():
        raise MetadataError(f"evidence root is not a directory: {root}")

    candidate = root
    for part in pure.parts:
        candidate = candidate / part
        if candidate.is_symlink():
            raise MetadataError(
                f"evidence path must not traverse a symlink: {relative}"
            )
    try:
        resolved = candidate.resolve(strict=True)
    except OSError as exc:
        raise MetadataError(f"evidence path not found: {relative}") from exc
    try:
        resolved.relative_to(root)
    except ValueError as exc:
        raise MetadataError(
            f"evidence path escapes repository root: {relative}"
        ) from exc
    if expect_directory:
        if not resolved.is_dir():
            raise MetadataError(f"evidence directory not found: {relative}")
    elif not resolved.is_file():
        raise MetadataError(f"evidence file not found: {relative}")
    return resolved


def validate_scorecard_stats(value: Any, field: str) -> dict[str, Any]:
    stats = require_exact_object(value, {"present", "usable", "rate"}, field)
    present = require_nonnegative_int(stats["present"], f"{field}.present")
    usable = require_nonnegative_int(stats["usable"], f"{field}.usable")
    if present > usable:
        raise MetadataError(f"{field}.present cannot exceed {field}.usable")
    rate = require_rate(stats["rate"], f"{field}.rate")
    expected_rate = None if usable == 0 else round(present / usable, 4)
    if rate != expected_rate:
        raise MetadataError(
            f"{field}.rate is inconsistent with counts: expected {expected_rate}, got {rate}"
        )
    return {"present": present, "usable": usable, "rate": rate}


def extract_legacy_codex_response(transcript: bytes) -> bytes | None:
    """Return the final response from a legacy human-formatted transcript."""
    lines = transcript.splitlines(keepends=True)
    markers = [
        index for index, line in enumerate(lines) if line.rstrip(b"\r\n") == b"codex"
    ]
    if not markers:
        return None
    start = markers[-1] + 1
    end = None
    for index in range(start, len(lines)):
        stripped = lines[index].rstrip(b"\r\n")
        if stripped == b"tokens used" or re.fullmatch(
            rb"tokens used:[ \t]*[^\r\n]*", stripped
        ):
            end = index
            break
    if end is None or end <= start:
        return None
    response = b"".join(lines[start:end])
    return response if response.strip() else None


def parse_jsonl_events(data: bytes, label: str) -> list[dict[str, Any]]:
    try:
        text = data.decode("utf-8", errors="strict")
    except UnicodeError as exc:
        raise MetadataError(f"{label} is not UTF-8 JSONL") from exc
    lines = text.splitlines()
    if not lines or any(not line.strip() for line in lines):
        raise MetadataError(f"{label} must contain non-blank JSONL events")
    events: list[dict[str, Any]] = []
    for index, line in enumerate(lines, 1):
        try:
            event = json.loads(line, object_pairs_hook=no_duplicate_object)
        except (json.JSONDecodeError, MetadataError) as exc:
            raise MetadataError(f"{label} line {index} is invalid JSON: {exc}") from exc
        if not isinstance(event, dict):
            raise MetadataError(f"{label} line {index} must be a JSON object")
        events.append(event)
    return events


def scheduled_position(contract: dict[str, Any], arm: str, rep: int) -> int:
    if arm not in {"control", "treatment"}:
        raise MetadataError(f"unsupported probe arm: {arm!r}")
    require_reps(rep)
    matches = [
        entry
        for entry in contract["schedule"]
        if entry["arm"] == arm and entry["rep"] == rep
    ]
    if len(matches) != 1:
        raise MetadataError(f"capture schedule has no unique {arm}-{rep} entry")
    return matches[0]["position"]


PROBE_INPUT_KEYS = {"type", "arm", "rep", "position", "prompt"}


def probe_input_event(
    contract: dict[str, Any],
    arm: str,
    rep: int,
    prompt: bytes,
    workspace: str | None = None,
    workspace_reset: bool = False,
    network_egress: dict[str, Any] | None = None,
) -> dict[str, Any]:
    expected = decode_prompts(contract["prompts"])[arm]
    if prompt != expected:
        raise MetadataError(f"actual {arm}-{rep} prompt differs from capture contract")
    event = {
        "type": PROBE_INPUT_EVENT,
        "arm": arm,
        "rep": rep,
        "position": scheduled_position(contract, arm, rep),
        "prompt": embedded_bytes_record(prompt, f"{arm}.prompt"),
    }
    if workspace is not None:
        # The cwd the harness dispatched into (2026-09-03 sibling-prompt-read
        # fix). Optional so pre-fix v3 sets still verify.
        text = require_text(workspace, f"{arm}-{rep} workspace")
        assert isinstance(text, str)
        event["workspace"] = text
    if workspace_reset:
        # The workspace is one fixed path under the run directory, emptied
        # before this rep rather than freshly mktemp'd, so the seatbelt profile
        # stays constant across the capture and binds one digest.
        event["workspace_reset"] = True
    if network_egress is not None:
        # What this rep asked the harness proxy for, and the digest of the proxy
        # log that says so. The seal denies every destination but the proxy, and
        # the proxy allows only the bound host list, so a nonzero `refused` is a
        # rep that reached for something the capture does not permit.
        event["network_egress"] = validate_network_egress(network_egress)
    return event


NETWORK_EGRESS_KEYS = {"allowed", "refused", "log_sha256"}


def validate_network_egress(value: Any) -> dict[str, Any]:
    record = require_exact_object(value, NETWORK_EGRESS_KEYS, "network_egress")
    for field in ("allowed", "refused"):
        count = record[field]
        if not isinstance(count, int) or isinstance(count, bool) or count < 0:
            raise MetadataError(f"network_egress {field} must be a count")
    digest = record["log_sha256"]
    if not isinstance(digest, str) or not SHA256_RE.fullmatch(digest):
        raise MetadataError("network_egress log_sha256 must be a sha256 digest")
    return {
        "allowed": record["allowed"],
        "refused": record["refused"],
        "log_sha256": digest,
    }


def validate_codex_runtime_events(
    events: list[dict[str, Any]], label: str
) -> tuple[str, bytes]:
    if not events:
        raise MetadataError(f"{label} has no Codex runtime events")
    types = [event.get("type") for event in events]
    if types[0] != "thread.started":
        raise MetadataError(f"{label} must start with thread.started")
    if types.count("thread.started") != 1 or types.count("turn.started") != 1:
        raise MetadataError(f"{label} must contain one thread and one turn start")
    if types.count("turn.completed") != 1 or types[-1] != "turn.completed":
        raise MetadataError(f"{label} must end with exactly one turn.completed event")
    if any(kind in {"turn.failed", "error"} for kind in types):
        raise MetadataError(f"{label} contains a failed Codex turn")
    thread_id = require_text(events[0].get("thread_id"), f"{label} thread_id")
    assert isinstance(thread_id, str)
    messages: list[str] = []
    for event in events:
        if event.get("type") != "item.completed":
            continue
        item = event.get("item")
        if not isinstance(item, dict) or item.get("type") != "agent_message":
            continue
        message = require_message_text(item.get("text"), f"{label} agent_message")
        messages.append(message)
    if not messages:
        raise MetadataError(f"{label} has no completed agent_message")
    return thread_id, messages[-1].encode("utf-8")


def validate_structured_transcript(
    transcript: bytes,
    contract: dict[str, Any],
    arm: str,
    rep: int,
    label: str,
) -> tuple[str, bytes]:
    events = parse_jsonl_events(transcript, label)
    # `workspace` (the rep cwd) and `workspace_reset` are optional in shape:
    # sets captured before the 2026-09-03 workspace fixes carry the five bound
    # keys only. A hardened seatbelt seal REQUIRES both, checked below.
    optional = {"workspace", "workspace_reset", "network_egress"}
    present = optional & set(events[0]) if isinstance(events[0], dict) else set()
    first = require_exact_object(
        events[0], PROBE_INPUT_KEYS | present, "probe input event"
    )
    expected_prompt = decode_prompts(contract["prompts"])[arm]
    expected_event = probe_input_event(
        contract,
        arm,
        rep,
        expected_prompt,
        workspace=first.get("workspace"),
        workspace_reset=bool(first.get("workspace_reset")),
        network_egress=first.get("network_egress"),
    )
    if first != expected_event:
        raise MetadataError(f"{label} probe input event does not match bound {arm}-{rep} prompt")
    seal = contract.get("seal")
    if isinstance(seal, dict) and seal.get("mode") == "seatbelt" and set(seal) in (
        PASS2_SEAL_KEYS,
        SEAL_KEYS,
    ):
        workspace = first.get("workspace")
        if not workspace:
            raise MetadataError(
                f"{label} probe input event records no workspace; a sealed rep must "
                "bind the directory it ran in"
            )
        if not first.get("workspace_reset"):
            raise MetadataError(
                f"{label} probe input event does not record workspace_reset; a sealed "
                "rep must start from an emptied workspace"
            )
        if not seal_covers(seal["writable_roots"], workspace):
            raise MetadataError(
                f"{label} ran in {workspace}, which is not under the seal's writable roots"
            )
        if set(seal) == SEAL_KEYS:
            egress = first.get("network_egress")
            if egress is None:
                raise MetadataError(
                    f"{label} records no network_egress; a rep under the network "
                    "seal must bind what it asked the harness proxy for"
                )
            if egress["refused"]:
                raise MetadataError(
                    f"{label} was refused {egress['refused']} egress connection(s) "
                    "by the harness proxy (network-egress); it cannot be accepted"
                )
    if any(event.get("type") == PROBE_INPUT_EVENT for event in events[1:]):
        raise MetadataError(f"{label} contains more than one probe input event")
    return validate_codex_runtime_events(events[1:], label)


def assemble_transcript(
    runtime_path: Path,
    prompt_path: Path,
    fixture_dir: Path,
    expected_probe: str,
    arm: str,
    rep: int,
    workspace: str | None = None,
    workspace_reset: bool = False,
    network_egress: dict[str, Any] | None = None,
) -> bytes:
    contract = load_capture_contract(fixture_dir, expected_probe)
    prompt = read_regular_bytes(prompt_path, "actual dispatch prompt", maximum=MAX_INPUT_BYTES * 2 + 16)
    input_event = probe_input_event(
        contract,
        arm,
        rep,
        prompt,
        workspace=workspace,
        workspace_reset=workspace_reset,
        network_egress=network_egress,
    )
    runtime = read_regular_bytes(
        runtime_path, "Codex JSONL runtime stream", maximum=MAX_TRANSCRIPT_BYTES
    )
    runtime_events = parse_jsonl_events(runtime, "Codex JSONL runtime stream")
    validate_codex_runtime_events(runtime_events, "Codex JSONL runtime stream")
    all_events = [input_event, *runtime_events]
    return b"".join(
        canonical_bytes(event) + b"\n" for event in all_events
    )


def run_bounded_discriminator(discriminator_bytes: bytes, response_bytes: bytes) -> int:
    """Run the scorer on a prompt-free response envelope in private snapshots."""
    with tempfile.TemporaryDirectory(prefix="probe-score.") as directory:
        root = Path(directory)
        discriminator_path = root / "discriminator.sh"
        response_path = root / "response.txt"
        discriminator_path.write_bytes(discriminator_bytes)
        # Keep the historical `codex` boundary for discriminators that already
        # extracted the response themselves, but never expose the echoed user
        # prompt. Response-native discriminators see the same response lines.
        response_envelope = b"codex\n" + response_bytes
        response_path.write_bytes(response_envelope)
        discriminator_path.chmod(0o500)
        response_path.chmod(0o400)
        process = subprocess.Popen(
            ["bash", str(discriminator_path), str(response_path)],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
        )
        try:
            returncode = process.wait(timeout=DISCRIMINATOR_TIMEOUT_SECONDS)
        except subprocess.TimeoutExpired as exc:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait()
            raise MetadataError(
                "discriminator timed out and its process group was terminated"
            ) from exc
        if (
            sorted(os.listdir(root)) != ["discriminator.sh", "response.txt"]
            or read_regular_bytes(discriminator_path, "scoring discriminator")
            != discriminator_bytes
            or read_regular_bytes(response_path, "scoring response")
            != response_envelope
        ):
            raise MetadataError("discriminator mutated an immutable scoring snapshot")
        return returncode


SKILL_READ_PATTERN = re.compile(r"SKILL\.md")
# Harness-owned names a rep must never touch: any *.prompt file, the capture
# contract, the seal record, the fixture manifest, the raw codex stream or
# stderr of a rep, and the hidden capture/dispatch stage directories.
SIBLING_ARTIFACT_PATTERN = re.compile(
    r"\.prompt\b"
    r"|capture-contract"
    r"|seal\.json"
    r"|fixture-set\.json"
    r"|\.codex\.(?:jsonl|stderr)\b"
    r"|\.(?:capture|dispatch)\.[A-Za-z0-9]{6}\b"
    # The 2026-09-03 second pass renamed the harness temp directories: the run
    # directory is probe-run.XXXXXX holding dispatch/, home/, ws/ and tmp/, and
    # the earlier shape used probe-dispatch/probe-ws/probe-seal. The hyphen
    # forms never matched the dotted stage pattern above. Only the
    # harness-private children of a run root are traps: ws/ and tmp/ ARE the
    # rep's own cwd and TMPDIR under this design, so naming them by absolute
    # path is ordinary work, not contact with another rep's material.
    r"|probe-(?:dispatch|ws|seal)\.[A-Za-z0-9]{6}\b"
    r"|probe-run\.[A-Za-z0-9]{6}/(?:dispatch|home)\b"
)
# A sibling rep's artifact by name, as it appears in a listing (`rg --files`,
# `ls`, `find`) or a read.
SIBLING_NAME_PATTERN = re.compile(
    r"\b(?:control|treatment)-[1-9][0-9]*\.(?:prompt|txt|codex\.jsonl|codex\.stderr)\b"
)


def final_command_executions(transcript_bytes: bytes) -> list[dict[str, Any]]:
    """The final state of every command_execution item, keyed by item id."""
    final: dict[str, dict[str, Any]] = {}
    for raw in transcript_bytes.splitlines():
        raw = raw.strip()
        if not raw.startswith(b"{"):
            continue
        try:
            obj = json.loads(raw)
        except (ValueError, UnicodeDecodeError):
            continue
        item = obj.get("item") if isinstance(obj, dict) else None
        if not isinstance(item, dict) or item.get("type") != "command_execution":
            continue
        item_id = str(item.get("id", ""))
        entry = final.setdefault(item_id, {})
        if item.get("command") is not None:
            entry["command"] = str(item.get("command"))
        if item.get("exit_code") is not None:
            entry["exit_code"] = item.get("exit_code")
        if isinstance(item.get("aggregated_output"), str):
            entry["output"] = item["aggregated_output"]
    return list(final.values())


def sibling_prompt_read(transcript_bytes: bytes) -> list[str]:
    """Producer contact with a harness artifact or a sibling rep voids the rep.

    The 2026-09-03 xhigh control-2 rep ran `rg --files | head -200` in the
    then-shared live workspace, saw `treatment-1.prompt`, then `sed -n
    '1,220p' treatment-1.prompt` — and held the canonical SKILL.md bytes
    without ever naming SKILL.md, so the skill-read trap let it score. A
    successful (final exit 0) command_execution is a hit when its command
    string names any harness artifact (`*.prompt`, capture-contract,
    seal.json, fixture-set.json, a rep's raw codex stream or stderr, or the
    hidden capture/dispatch stage), or when its captured output lists a
    sibling rep's artifact by name. Same FLOOR caveats as
    skill_read_contamination; the per-rep workspace is the prevention.
    """
    hits = []
    for entry in final_command_executions(transcript_bytes):
        if entry.get("exit_code") != 0:
            continue
        command = entry.get("command", "")
        if SIBLING_ARTIFACT_PATTERN.search(command) or SIBLING_NAME_PATTERN.search(command):
            hits.append(f"command: {command}")
            continue
        listed = SIBLING_NAME_PATTERN.search(entry.get("output", ""))
        if listed:
            hits.append(f"output lists {listed.group(0)}: {command}")
    return sorted(hits)


def skill_read_contamination(transcript_bytes: bytes) -> list[str]:
    """Producer-initiated successful reads of any SKILL.md void the rep.

    An arm that fetches skill bytes off disk (repo checkout, installed
    corpus, any path) is no longer differentiated from the other arm by the
    bound treatment bytes — the exact leak the 2026-08-26 premortem capture
    proved (control reps ran `sed`/`cat` over skills/premortem/SKILL.md and
    then performed the skill).

    Honest boundary — this is a FLOOR, not a seal: the heuristic flags
    command_execution items whose FINAL exit code is 0 and whose command
    string contains the literal SKILL.md. Known evasions it does not catch:
    a compound command whose read succeeds but whose last segment fails
    (`cat .../SKILL.md; false` exits 1), glob or variable indirection that
    keeps the literal out of the command string, and copy-then-read
    laundering. Real isolation is filesystem-sealed dispatch (RUNBOOK,
    "isolation floor"); this trap exists so the KNOWN leak shape can never
    silently recur.
    """
    hits = []
    for entry in final_command_executions(transcript_bytes):
        command = entry.get("command", "")
        if entry.get("exit_code") == 0 and SKILL_READ_PATTERN.search(command):
            hits.append(command)
    return sorted(hits)


def classify_bytes(
    discriminator_bytes: bytes,
    transcript_bytes: bytes,
    *,
    structured: tuple[dict[str, Any], str, int] | None = None,
) -> str:
    if structured is None:
        response = extract_legacy_codex_response(transcript_bytes)
    else:
        contract, arm, rep = structured
        _, response = validate_structured_transcript(
            transcript_bytes, contract, arm, rep, f"{arm}-{rep}.txt"
        )
    if response is None:
        return "DEGRADED"
    contamination = skill_read_contamination(transcript_bytes)
    if contamination:
        for command in contamination:
            print(
                f"probe-skill: rep DEGRADED (skill-read-contamination): {command[:200]}",
                file=sys.stderr,
            )
        return "DEGRADED"
    sibling = sibling_prompt_read(transcript_bytes)
    if sibling:
        for hit in sibling:
            print(
                f"probe-skill: rep DEGRADED (sibling-prompt-read): {hit[:240]}",
                file=sys.stderr,
            )
        return "DEGRADED"
    try:
        returncode = run_bounded_discriminator(discriminator_bytes, response)
    except OSError as exc:
        raise MetadataError(f"could not run discriminator: {exc}") from exc
    if returncode == 0:
        return "PRESENT"
    if returncode == 1:
        return "ABSENT"
    return "DEGRADED"


def captured_input_bytes(manifest: dict[str, Any]) -> dict[str, bytes]:
    probe = require_text(manifest.get("probe"), "captured probe")
    source = require_text(
        manifest.get("treatment_source"), "captured treatment_source"
    )
    assert isinstance(probe, str) and isinstance(source, str)
    decoded, _ = decode_capture_inputs(
        manifest.get("capture_inputs"), probe, source
    )
    return decoded


def verdict_for_rates(control: dict[str, Any], treatment: dict[str, Any]) -> str:
    if control["usable"] == 0 or treatment["usable"] == 0:
        return "UNMEASURED"
    if treatment["rate"] > control["rate"]:
        return "BEHAVIORAL"
    if treatment["rate"] < control["rate"]:
        return "REGRESSIVE"
    return "INERT"


def recompute_score(
    probe_dir: Path, fixture_dir: Path, manifest: dict[str, Any]
) -> dict[str, Any]:
    reps = require_reps(manifest["reps"])
    if manifest["schema"] == SCHEMA:
        discriminator_bytes = captured_input_bytes(manifest)["discriminator.sh"]
    else:
        discriminator_bytes = read_regular_bytes(
            probe_dir / "discriminator.sh",
            "discriminator",
            maximum=MAX_INPUT_BYTES,
        )
    transcript_snapshot = {
        name: read_regular_bytes(
            fixture_dir / name, "transcript", maximum=MAX_TRANSCRIPT_BYTES
        )
        for name in expected_transcripts(reps)
    }
    per_rep: list[dict[str, Any]] = []
    counts = {
        "control": {"present": 0, "usable": 0},
        "treatment": {"present": 0, "usable": 0},
    }
    for rep in range(1, reps + 1):
        entry: dict[str, Any] = {"rep": rep}
        for arm in ("control", "treatment"):
            name = f"{arm}-{rep}.txt"
            structured = (manifest, arm, rep) if manifest["schema"] == SCHEMA else None
            outcome = classify_bytes(
                discriminator_bytes,
                transcript_snapshot[name],
                structured=structured,
            )
            entry[arm] = outcome
            if outcome != "DEGRADED":
                counts[arm]["usable"] += 1
            if outcome == "PRESENT":
                counts[arm]["present"] += 1
        per_rep.append(entry)

    scored: dict[str, Any] = {"per_rep": per_rep}
    for arm in ("control", "treatment"):
        present = counts[arm]["present"]
        usable = counts[arm]["usable"]
        scored[arm] = {
            "present": present,
            "usable": usable,
            "rate": None if usable == 0 else round(present / usable, 4),
        }
    scored["verdict"] = verdict_for_rates(scored["control"], scored["treatment"])
    return scored


def read_open_fd(fd: int, path: Path) -> tuple[bytes, os.stat_result]:
    try:
        before = os.fstat(fd)
    except OSError as exc:
        raise MetadataError(f"capture transcript fd is not open: {fd}") from exc
    if not stat.S_ISREG(before.st_mode):
        raise MetadataError("capture transcript fd is not a regular file")
    if before.st_size > MAX_TRANSCRIPT_BYTES:
        raise MetadataError("capture transcript exceeds the safety limit")
    try:
        read_fd = os.open(path, os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0))
    except OSError as exc:
        raise MetadataError("capture transcript path identity changed") from exc
    try:
        read_before = os.fstat(read_fd)
        if not os.path.samestat(before, read_before):
            raise MetadataError("capture transcript path does not name the owned sink")
        chunks: list[bytes] = []
        total = 0
        while chunk := os.read(read_fd, 1024 * 1024):
            total += len(chunk)
            if total > MAX_TRANSCRIPT_BYTES:
                raise MetadataError("capture transcript exceeds the safety limit")
            chunks.append(chunk)
        after = os.fstat(fd)
        read_after = os.fstat(read_fd)
        path_identity = os.stat(path, follow_symlinks=False)
        if (
            not os.path.samestat(before, after)
            or not os.path.samestat(before, read_after)
            or not os.path.samestat(before, path_identity)
            or before.st_size != after.st_size
            or before.st_mtime_ns != after.st_mtime_ns
            or total != before.st_size
        ):
            raise MetadataError("capture transcript identity or bytes changed")
        return b"".join(chunks), before
    finally:
        os.close(read_fd)


def classify_open_capture(
    fd: int,
    path: Path,
    contract: dict[str, Any],
    discriminator_bytes: bytes,
    arm: str,
    rep: int,
) -> dict[str, Any]:
    transcript, identity = read_open_fd(fd, path)
    thread_id, _ = validate_structured_transcript(
        transcript, contract, arm, rep, path.name
    )
    outcome = classify_bytes(
        discriminator_bytes,
        transcript,
        structured=(contract, arm, rep),
    )
    transcript_after, identity_after = read_open_fd(fd, path)
    if not os.path.samestat(identity, identity_after) or transcript_after != transcript:
        raise MetadataError("capture transcript changed during scoring")
    return {
        "outcome": outcome,
        "sha256": digest_bytes(transcript),
        "thread_id": thread_id,
        "producer": contract["producer_request"],
    }


def verify_scorecard(
    repo_root: Path,
    skills_dir: Path,
    scorecard_relative: str,
    ledger_skill: str,
    ledger_probe: str,
    ledger_verdict: str,
) -> dict[str, Any]:
    for value, field in (
        (ledger_skill, "ledger skill"),
        (ledger_probe, "ledger probe"),
        (ledger_verdict, "ledger verdict"),
    ):
        require_text(value, field)
    if not SAFE_ID_RE.fullmatch(ledger_skill) or not SAFE_ID_RE.fullmatch(ledger_probe):
        raise MetadataError("ledger skill and probe must be safe identifiers")
    if ledger_verdict not in CURRENT_VERDICTS:
        raise MetadataError(f"ledger verdict is not a current result: {ledger_verdict}")

    try:
        resolved_root = repo_root.resolve(strict=True)
        resolved_skills = skills_dir.resolve(strict=True)
    except OSError as exc:
        raise MetadataError(
            f"repository or canonical skills root not found: {exc}"
        ) from exc
    if skills_dir.is_symlink() or resolved_skills != resolved_root / "skills":
        raise MetadataError(
            "coverage canonical skills directory must be the repo-local skills/ tree"
        )

    if not scorecard_relative.endswith(".json"):
        raise MetadataError("scorecard evidence path must end in .json")
    scorecard_path = safe_repo_path(
        repo_root,
        scorecard_relative,
        expected_prefix=("docs", "evals", "scorecards"),
    )
    scorecard = load_json(scorecard_path)
    scorecard_keys = {
        "schema",
        "probe",
        "skill",
        "mode",
        "generated_at",
        "reps",
        "producer",
        "requested_producer",
        "fixture_set",
        "treatment_source",
        "evaluator",
        "capture_evaluator",
        "evaluator_matches_capture",
        "honesty",
        "schedule",
        "scoring",
        "control",
        "treatment",
        "verdict",
        "per_rep",
    }
    # The scorecard's `seal` copy (the harness record, null in replay) is
    # informational; the capture contract's bound seal block is authoritative.
    if "seal" in scorecard:
        scorecard_keys = scorecard_keys | {"seal"}
    require_exact_object(scorecard, scorecard_keys, "scorecard")
    if scorecard["schema"] != SCORECARD_SCHEMA:
        raise MetadataError(f"scorecard is not v3: {scorecard['schema']!r}")
    if scorecard["skill"] != ledger_skill:
        raise MetadataError(
            f"scorecard/ledger skill mismatch: {scorecard['skill']!r} != {ledger_skill!r}"
        )
    if scorecard["probe"] != ledger_probe:
        raise MetadataError(
            f"scorecard/ledger probe mismatch: {scorecard['probe']!r} != {ledger_probe!r}"
        )
    if scorecard["verdict"] != ledger_verdict:
        raise MetadataError(
            f"scorecard/ledger verdict mismatch: {scorecard['verdict']!r} != {ledger_verdict!r}"
        )
    if scorecard["mode"] not in {"live", "replay"}:
        raise MetadataError(f"invalid scorecard mode: {scorecard['mode']!r}")
    if scorecard["treatment_source"] not in TREATMENT_SOURCES:
        raise MetadataError(
            f"invalid scorecard treatment_source: {scorecard['treatment_source']!r}"
        )
    require_text(scorecard["generated_at"], "scorecard generated_at")
    require_text(scorecard["honesty"], "scorecard honesty")
    reps = require_reps(scorecard["reps"])
    schedule = validate_schedule(scorecard["schedule"], reps)
    scoring = require_exact_object(
        scorecard["scoring"],
        {
            "response_extraction",
            "transcript_format",
            "discriminator_timeout_seconds",
        },
        "scorecard scoring",
    )
    if scoring != {
        "response_extraction": RESPONSE_EXTRACTION,
        "transcript_format": TRANSCRIPT_FORMAT,
        "discriminator_timeout_seconds": DISCRIMINATOR_TIMEOUT_SECONDS,
    }:
        raise MetadataError("scorecard scoring contract is not the current contract")

    producer = require_exact_object(
        scorecard["producer"],
        {"adapter", "model", "effort", "identity", "threads"},
        "scorecard producer",
    )
    producer_request = validate_producer_request(
        {key: producer[key] for key in ("adapter", "model", "effort", "identity")}
    )
    identity = producer_request["identity"]
    if identity["override"] or producer["model"] is None or producer["effort"] is None:
        raise MetadataError(
            "tier coverage requires non-overrideable native Codex runtime evidence"
        )
    requested = require_exact_object(
        scorecard["requested_producer"],
        {"model", "effort"},
        "scorecard requested_producer",
    )
    require_text(requested["model"], "requested model", nullable=True)
    require_text(requested["effort"], "requested effort", nullable=True)

    fixture = require_exact_object(
        scorecard["fixture_set"],
        {"name", "metadata", "binding_sha256", "schema"},
        "scorecard fixture_set",
    )
    fixture_name = require_text(fixture["name"], "fixture set name")
    if not re.fullmatch(r"fixtures(?:[_-][A-Za-z0-9][A-Za-z0-9._-]*)?", fixture_name):
        raise MetadataError(f"unsafe fixture set name: {fixture_name!r}")
    if fixture["metadata"] != MANIFEST_NAME:
        raise MetadataError(f"scorecard fixture metadata must be {MANIFEST_NAME}")
    if fixture["schema"] != SCHEMA:
        raise MetadataError("tier coverage requires a self-contained v3 fixture set")
    if not isinstance(fixture["binding_sha256"], str) or not SHA256_RE.fullmatch(
        fixture["binding_sha256"]
    ):
        raise MetadataError("scorecard fixture binding must be a sha256 digest")

    validate_evaluator(scorecard["evaluator"], "scorecard evaluator")
    verify_evaluator_files(scorecard["evaluator"], resolved_root)
    validate_evaluator(
        scorecard["capture_evaluator"],
        "scorecard capture_evaluator",
        require_dispatch=True,
    )
    if not isinstance(scorecard["evaluator_matches_capture"], bool):
        raise MetadataError("scorecard evaluator_matches_capture must be boolean")
    if scorecard["evaluator_matches_capture"] != (
        scorecard["evaluator"] == scorecard["capture_evaluator"]
    ):
        raise MetadataError("scorecard evaluator_matches_capture is inconsistent")
    control = validate_scorecard_stats(scorecard["control"], "scorecard control")
    treatment = validate_scorecard_stats(scorecard["treatment"], "scorecard treatment")

    probe_relative = f"evals/skill-probes/{ledger_probe}"
    probe_dir = safe_repo_path(repo_root, probe_relative, expect_directory=True)
    fixture_relative = f"{probe_relative}/{fixture_name}"
    fixture_dir = safe_repo_path(repo_root, fixture_relative, expect_directory=True)
    manifest = validate_manifest(
        fixture_dir,
        probe_dir,
        skills_dir,
        ledger_probe,
        require_current_inputs=True,
    )
    if manifest["schema"] != SCHEMA:
        raise MetadataError("tier coverage requires self-contained fixture metadata v3")

    if fixture["binding_sha256"] != manifest["binding_sha256"]:
        raise MetadataError("scorecard/manifest fixture binding mismatch")
    if reps != manifest["reps"]:
        raise MetadataError("scorecard/manifest reps mismatch")
    if producer != manifest["producer"]:
        raise MetadataError("scorecard/manifest producer mismatch")
    if requested != manifest["requested_producer"]:
        raise MetadataError("scorecard/manifest requested producer mismatch")
    if scorecard["capture_evaluator"] != manifest["capture_evaluator"]:
        raise MetadataError("scorecard/manifest capture evaluator mismatch")
    if scorecard["treatment_source"] != manifest["treatment_source"]:
        raise MetadataError("scorecard/manifest treatment_source mismatch")
    if schedule != manifest["schedule"]:
        raise MetadataError("scorecard/manifest schedule mismatch")
    if scoring != manifest["scoring"]:
        raise MetadataError("scorecard/manifest scoring contract mismatch")
    if manifest["treatment_source"] != "canonical-skill":
        raise MetadataError(
            "tier coverage requires treatment_source 'canonical-skill'; "
            "injected-prelude evidence measures only the bound prelude"
        )
    # The seal is the prevention for skill-read contamination; a counted row
    # must prove the dispatch ran with the checkout and skill roots unreadable.
    seal = load_capture_contract(fixture_dir, ledger_probe)["seal"]
    # Eligibility first: a seal that cannot be coverage should say WHY, not fail
    # on a shape mismatch against its own scorecard copy.
    verify_seal_for_coverage(seal)
    # Two ties the seal block alone cannot make, because both ends live in the
    # producer identity the manifest binds.
    if seal["mode"] == "seatbelt" and set(seal) == SEAL_KEYS:
        expected_config = render_probe_config(producer.get("effort"))
        if seal["config_text"] != expected_config:
            raise MetadataError(
                "tier coverage requires the rep config to be exactly what the "
                "generator emits for the bound effort; a self-consistent digest "
                "only proves the recorded text hashes to the recorded digest"
            )
        launcher_digest = seal["launcher_sha256"]
        if launcher_digest != identity.get("executable_sha256"):
            raise MetadataError(
                "tier coverage requires the bound launcher to be the producer the "
                "manifest identifies; launcher_sha256 "
                f"{launcher_digest} is not the bound executable digest "
                f"{identity.get('executable_sha256')}"
            )
    if scorecard.get("seal") is not None:
        scorecard_seal = seal_block_from_record(
            scorecard["seal"], "scorecard seal", seal["repository_root"]
        )
        if seal_projection(scorecard_seal) != seal_projection(seal):
            raise MetadataError(
                "scorecard seal copy disagrees with the seal bound in the capture contract"
            )
    if not identity["coverage_eligible"]:
        raise MetadataError(
            "tier coverage requires a coverage-eligible producer identity bound "
            "at capture"
        )

    probe_meta = load_json(probe_dir / "probe.json")
    if probe_meta.get("id") != ledger_probe:
        raise MetadataError("probe.json id does not match ledger probe")
    if probe_meta.get("skill") != ledger_skill:
        raise MetadataError("probe.json skill does not match ledger skill")

    recomputed = recompute_score(probe_dir, fixture_dir, manifest)
    manifest_after = validate_manifest(
        fixture_dir,
        probe_dir,
        skills_dir,
        ledger_probe,
        require_current_inputs=True,
    )
    if manifest_after["binding_sha256"] != manifest["binding_sha256"]:
        raise MetadataError("fixture binding changed during discriminator replay")
    if scorecard["per_rep"] != recomputed["per_rep"]:
        raise MetadataError(
            "scorecard per_rep outcomes do not match discriminator replay"
        )
    if control != recomputed["control"]:
        raise MetadataError(
            "scorecard control totals do not match discriminator replay"
        )
    if treatment != recomputed["treatment"]:
        raise MetadataError(
            "scorecard treatment totals do not match discriminator replay"
        )
    if scorecard["verdict"] != recomputed["verdict"]:
        raise MetadataError(
            f"scorecard verdict does not match discriminator replay: {recomputed['verdict']}"
        )

    return {
        "binding_sha256": manifest["binding_sha256"],
        "producer": manifest["producer"],
        "probe": ledger_probe,
        "reps": reps,
        "seal": seal,
        "skill": ledger_skill,
        "verdict": ledger_verdict,
    }


def validate_manifest(
    fixture_dir: Path,
    probe_dir: Path,
    skills_dir: Path,
    expected_probe: str,
    *,
    require_current_inputs: bool = False,
) -> dict[str, Any]:
    manifest_path = fixture_dir / MANIFEST_NAME
    if manifest_path.is_symlink() or not manifest_path.is_file():
        raise MetadataError(
            f"verified replay requires immutable capture metadata at {manifest_path}"
        )
    manifest = load_json(manifest_path)
    common_keys = {
        "schema",
        "probe",
        "reps",
        "producer",
        "requested_producer",
        "transcripts",
        "capture_evaluator",
        "binding_sha256",
    }
    schema = manifest.get("schema")
    if schema == SCHEMA:
        expected_keys = common_keys | {
            "capture_contract",
            "capture_inputs",
            "canonical_skill",
            "treatment_source",
            "prompts",
            "schedule",
            "scoring",
        }
    elif schema == LEGACY_CANONICAL_SCHEMA:
        expected_keys = common_keys | {
            "evaluation_inputs",
            "canonical_skill",
            "treatment_source",
        }
    elif schema == LEGACY_BOUND_SCHEMA:
        expected_keys = common_keys | {"evaluation_inputs"}
    else:
        raise MetadataError(f"unsupported fixture metadata schema: {schema!r}")
    if set(manifest) != expected_keys:
        raise MetadataError("fixture metadata has unknown or missing top-level fields")
    probe = require_text(manifest["probe"], "probe")
    if probe != expected_probe:
        raise MetadataError(
            f"fixture metadata probe mismatch: expected {expected_probe}, got {probe}"
        )
    reps = require_reps(manifest["reps"])

    producer = manifest["producer"]
    if schema == SCHEMA:
        producer = require_exact_object(
            producer,
            {"adapter", "model", "effort", "identity", "threads"},
            "producer",
        )
        producer_request = validate_producer_request(
            {key: producer[key] for key in ("adapter", "model", "effort", "identity")}
        )
    else:
        producer = require_exact_object(
            producer, {"adapter", "model", "effort"}, "producer"
        )
        if producer["adapter"] != "codex":
            raise MetadataError(f"unsupported producer adapter: {producer['adapter']!r}")
        require_text(producer["model"], "producer model")
        require_text(producer["effort"], "producer effort")
        producer_request = None

    requested = manifest["requested_producer"]
    if not isinstance(requested, dict) or set(requested) != {"model", "effort"}:
        raise MetadataError("requested_producer must contain exactly model and effort")
    require_text(requested["model"], "requested model", nullable=True)
    require_text(requested["effort"], "requested effort", nullable=True)
    if schema == SCHEMA and requested != {
        "model": producer["model"],
        "effort": producer["effort"],
    }:
        raise MetadataError("requested_producer disagrees with bound producer request")

    transcripts = manifest["transcripts"]
    if not isinstance(transcripts, list):
        raise MetadataError("transcripts must be an array")
    expected = expected_transcripts(reps)
    seen: list[str] = []
    paths: list[Path] = []
    for index, entry in enumerate(transcripts):
        if not isinstance(entry, dict) or set(entry) != {"path", "sha256"}:
            raise MetadataError(
                f"transcripts[{index}] must contain exactly path and sha256"
            )
        name = entry["path"]
        if not isinstance(name, str) or not TRANSCRIPT_RE.fullmatch(name):
            raise MetadataError(f"unsafe transcript path in metadata: {name!r}")
        if not isinstance(entry["sha256"], str) or not SHA256_RE.fullmatch(
            entry["sha256"]
        ):
            raise MetadataError(f"invalid transcript digest for {name}")
        if name in seen:
            raise MetadataError(f"duplicate transcript metadata: {name}")
        seen.append(name)
        path = fixture_dir / name
        validate_transcript(path)
        actual_digest = digest_file(path)
        if actual_digest != entry["sha256"]:
            raise MetadataError(
                f"transcript digest mismatch for {name}: expected {entry['sha256']}, got {actual_digest}"
            )
        paths.append(path)
    if seen != expected:
        raise MetadataError(
            "fixture metadata transcript inventory or ordering is invalid"
        )
    actual_names = sorted(os.listdir(fixture_dir))
    has_seal_sidecar = schema == SCHEMA and SEAL_NAME in actual_names
    has_network_log = schema == SCHEMA and NETWORK_LOG_NAME in actual_names
    if has_network_log:
        verify_network_log(fixture_dir, expected)
    expected_names = sorted(
        expected
        + [MANIFEST_NAME]
        + ([CAPTURE_CONTRACT_NAME] if schema == SCHEMA else [])
        + ([SEAL_NAME] if has_seal_sidecar else [])
        + ([NETWORK_LOG_NAME] if has_network_log else [])
    )
    if actual_names != expected_names:
        raise MetadataError(
            "fixture directory contains files outside the exact bound inventory"
        )

    if schema in {LEGACY_BOUND_SCHEMA, LEGACY_CANONICAL_SCHEMA}:
        observed = observe_paths(paths)
        if observed != producer:
            raise MetadataError(
                "producer metadata disagrees with observed transcript headers: "
                f"metadata={producer['model']}/{producer['effort']} "
                f"observed={observed['model']}/{observed['effort']}"
            )
        validate_hash_records(
            manifest["evaluation_inputs"],
            LEGACY_EVALUATION_INPUT_NAMES,
            probe_dir,
            "evaluation_inputs",
        )
        current_probe = load_json(probe_dir / "probe.json")
        contract = validate_probe_metadata(
            current_probe,
            expected_probe,
            require_treatment=schema == LEGACY_CANONICAL_SCHEMA,
        )
        if contract["reps"] != reps:
            raise MetadataError("fixture reps disagree with bound probe.json reps")
    if schema == LEGACY_CANONICAL_SCHEMA:
        validate_canonical_skill_record(
            manifest["canonical_skill"], probe_dir, skills_dir, expected_probe
        )
        treatment_source = declared_treatment_source(probe_dir, expected_probe)
        if manifest["treatment_source"] != treatment_source:
            raise MetadataError(
                "fixture metadata treatment_source disagrees with bound probe.json: "
                f"{manifest['treatment_source']!r} != {treatment_source!r}"
            )
    elif schema == SCHEMA:
        capture_contract_record = require_exact_object(
            manifest["capture_contract"],
            {"path", "sha256"},
            "capture_contract",
        )
        if capture_contract_record["path"] != CAPTURE_CONTRACT_NAME:
            raise MetadataError("capture_contract path is invalid")
        capture_contract_digest = capture_contract_record["sha256"]
        if not isinstance(capture_contract_digest, str) or not SHA256_RE.fullmatch(
            capture_contract_digest
        ):
            raise MetadataError("capture_contract sha256 must be a sha256 digest")
        if digest_file(fixture_dir / CAPTURE_CONTRACT_NAME) != capture_contract_digest:
            raise MetadataError("capture contract file digest mismatch")
        capture_contract = load_capture_contract(fixture_dir, expected_probe)
        if has_seal_sidecar:
            verify_seal_sidecar(fixture_dir, skills_dir, capture_contract)
        for field in (
            "reps",
            "capture_inputs",
            "canonical_skill",
            "treatment_source",
            "prompts",
            "schedule",
            "scoring",
        ):
            if manifest[field] != capture_contract[field]:
                raise MetadataError(
                    f"fixture manifest {field} disagrees with capture contract"
                )
        assert producer_request is not None
        if producer_request != capture_contract["producer_request"]:
            raise MetadataError(
                "fixture producer identity disagrees with pre-execution capture contract"
            )
        thread_records = producer["threads"]
        if not isinstance(thread_records, list) or len(thread_records) != len(expected):
            raise MetadataError("producer threads must cover the exact transcript inventory")
        actual_threads = []
        for name, path in zip(expected, paths, strict=True):
            match = TRANSCRIPT_RE.fullmatch(name)
            assert match is not None
            arm, rep_text = match.groups()
            transcript = read_regular_bytes(
                path, "transcript", maximum=MAX_TRANSCRIPT_BYTES
            )
            thread_id, _ = validate_structured_transcript(
                transcript, capture_contract, arm, int(rep_text), name
            )
            actual_threads.append({"path": name, "thread_id": thread_id})
        if thread_records != actual_threads:
            raise MetadataError("producer thread identities disagree with Codex JSON events")
        if len({entry["thread_id"] for entry in actual_threads}) != len(actual_threads):
            raise MetadataError("each probe dispatch must have a distinct Codex thread_id")
        capture_inputs = captured_input_bytes(manifest)
        captured_probe = parse_json_bytes(
            capture_inputs["probe.json"], "captured probe.json"
        )
        captured_contract = validate_probe_metadata(
            captured_probe, expected_probe, require_treatment=True
        )
        if captured_contract["reps"] != reps:
            raise MetadataError("fixture reps disagree with captured probe.json reps")
        if manifest["treatment_source"] != captured_contract["treatment_source"]:
            raise MetadataError(
                "fixture treatment_source disagrees with captured probe.json"
            )
        embedded_skill, _ = validate_embedded_canonical_skill(
            manifest["canonical_skill"], captured_probe, expected_probe
        )
        validate_schedule(manifest["schedule"], reps)
        scoring = require_exact_object(
            manifest["scoring"],
            {
                "response_extraction",
                "transcript_format",
                "discriminator_timeout_seconds",
            },
            "scoring",
        )
        if scoring != {
            "response_extraction": RESPONSE_EXTRACTION,
            "transcript_format": TRANSCRIPT_FORMAT,
            "discriminator_timeout_seconds": DISCRIMINATOR_TIMEOUT_SECONDS,
        }:
            raise MetadataError("fixture scoring contract is unsupported")
        if require_current_inputs:
            current_records = embedded_input_records(
                probe_dir, manifest["treatment_source"]
            )
            if current_records != manifest["capture_inputs"]:
                raise MetadataError(
                    "current probe inputs differ from the self-contained capture"
                )
            current_skill = build_canonical_skill_record(
                probe_dir, skills_dir, expected_probe
            )
            if any(
                embedded_skill[field] != current_skill[field]
                for field in ("name", "path", "sha256")
            ):
                raise MetadataError(
                    "current canonical skill differs from the self-contained capture"
                )
    validate_evaluator(
        manifest["capture_evaluator"],
        "capture_evaluator",
        require_dispatch=schema != LEGACY_BOUND_SCHEMA,
    )

    binding = manifest["binding_sha256"]
    if not isinstance(binding, str) or not SHA256_RE.fullmatch(binding):
        raise MetadataError("binding_sha256 must be a sha256 digest")
    payload = {key: value for key, value in manifest.items() if key != "binding_sha256"}
    actual_binding = digest_bytes(canonical_bytes(payload))
    if binding != actual_binding:
        raise MetadataError(
            f"fixture-set binding mismatch: expected {binding}, got {actual_binding}"
        )
    return manifest


def summary(manifest: dict[str, Any], seal: dict[str, Any]) -> dict[str, Any]:
    return {
        "schema": manifest["schema"],
        "seal": seal,
        "binding_sha256": manifest["binding_sha256"],
        "producer": manifest["producer"],
        "requested_producer": manifest["requested_producer"],
        "capture_evaluator": manifest["capture_evaluator"],
        "canonical_skill": manifest.get("canonical_skill"),
        "reps": manifest["reps"],
        "treatment_source": manifest.get("treatment_source", "injected-prelude"),
        "schedule": manifest.get("schedule"),
        "scoring": manifest.get("scoring"),
        "transcripts": manifest["transcripts"],
    }


def probe_contract_summary(
    probe_dir: Path, skills_dir: Path, expected_probe: str
) -> dict[str, Any]:
    probe_meta = load_json(probe_dir / "probe.json")
    contract = validate_probe_metadata(
        probe_meta, expected_probe, require_treatment=True
    )
    treatment_source = contract["treatment_source"]
    assert isinstance(treatment_source, str)
    records = []
    for name in capture_input_names(treatment_source):
        path = probe_dir / name
        validate_regular_file(path, "evaluation input")
        records.append({"path": name, "sha256": digest_file(path)})
    return {
        "canonical_skill": build_canonical_skill_record(
            probe_dir, skills_dir, expected_probe
        ),
        "evaluation_inputs": records,
        "reps": contract["reps"],
        "treatment_source": treatment_source,
        "schedule": counterbalanced_schedule(contract["reps"]),
        "scoring": {
            "response_extraction": RESPONSE_EXTRACTION,
            "transcript_format": TRANSCRIPT_FORMAT,
            "discriminator_timeout_seconds": DISCRIMINATOR_TIMEOUT_SECONDS,
        },
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)

    create = subparsers.add_parser("create")
    create.add_argument("--fixture-dir", type=Path, required=True)
    create.add_argument("--probe-dir", type=Path, required=True)
    create.add_argument("--skills-dir", type=Path, required=True)
    create.add_argument("--harness", type=Path, required=True)
    create.add_argument("--preamble", type=Path, required=True)
    create.add_argument("--dispatch-helper", type=Path, required=True)
    create.add_argument("--probe", required=True)
    create.add_argument("--reps", type=int, required=True)
    create.add_argument("--requested-model")
    create.add_argument("--requested-effort")

    verify = subparsers.add_parser("verify")
    verify.add_argument("--fixture-dir", type=Path, required=True)
    verify.add_argument("--probe-dir", type=Path, required=True)
    verify.add_argument("--skills-dir", type=Path, required=True)
    verify.add_argument("--probe", required=True)

    observe = subparsers.add_parser("observe")
    observe.add_argument("--fixture-dir", type=Path, required=True)

    identity = subparsers.add_parser("identity")
    identity.add_argument("--harness", type=Path, required=True)
    identity.add_argument("--preamble", type=Path, required=True)
    identity.add_argument("--dispatch-helper", type=Path, required=True)

    contract = subparsers.add_parser("probe-contract")
    contract.add_argument("--probe-dir", type=Path, required=True)
    contract.add_argument("--skills-dir", type=Path, required=True)
    contract.add_argument("--probe", required=True)

    publish = subparsers.add_parser("publish")
    publish.add_argument("--stage-dir", type=Path, required=True)
    publish.add_argument("--target-dir", type=Path, required=True)
    publish.add_argument("--probe-dir", type=Path, required=True)
    publish.add_argument("--skills-dir", type=Path, required=True)
    publish.add_argument("--probe", required=True)

    scorecard = subparsers.add_parser("verify-scorecard")
    scorecard.add_argument("--repo-root", type=Path, required=True)
    scorecard.add_argument("--skills-dir", type=Path, required=True)
    scorecard.add_argument("--scorecard", required=True)
    scorecard.add_argument("--ledger-skill", required=True)
    scorecard.add_argument("--ledger-probe", required=True)
    scorecard.add_argument("--ledger-verdict", required=True)

    score = subparsers.add_parser("score")
    score.add_argument("--fixture-dir", type=Path, required=True)
    score.add_argument("--probe-dir", type=Path, required=True)
    score.add_argument("--skills-dir", type=Path, required=True)
    score.add_argument("--probe", required=True)

    classify_open = subparsers.add_parser("classify-open")
    classify_open.add_argument("--fd", type=int, required=True)
    classify_open.add_argument("--path", type=Path, required=True)
    classify_open.add_argument("--fixture-dir", type=Path, required=True)
    classify_open.add_argument("--probe", required=True)
    classify_open.add_argument("--arm", choices=("control", "treatment"), required=True)
    classify_open.add_argument("--rep", type=int, required=True)

    assemble = subparsers.add_parser("assemble-transcript")
    assemble.add_argument("--runtime-file", type=Path, required=True)
    assemble.add_argument("--prompt-file", type=Path, required=True)
    assemble.add_argument("--fixture-dir", type=Path, required=True)
    assemble.add_argument("--probe", required=True)
    assemble.add_argument("--arm", choices=("control", "treatment"), required=True)
    assemble.add_argument("--rep", type=int, required=True)
    assemble.add_argument("--workspace", default=None)
    assemble.add_argument("--workspace-reset", action="store_true")
    assemble.add_argument("--network-egress", default=None)

    tiers = subparsers.add_parser("tier-skills")
    tiers.add_argument("--skills-dir", type=Path, required=True)

    snapshot = subparsers.add_parser("snapshot")
    snapshot.add_argument("--fixture-dir", type=Path, required=True)
    snapshot.add_argument("--probe-dir", type=Path, required=True)
    snapshot.add_argument("--skills-dir", type=Path, required=True)
    snapshot.add_argument("--probe", required=True)
    snapshot.add_argument("--requested-model")
    snapshot.add_argument("--requested-effort")
    snapshot.add_argument("--producer-override-bin")
    snapshot.add_argument("--seal-file", type=Path)

    capture_file = subparsers.add_parser("capture-file")
    capture_file.add_argument("--fixture-dir", type=Path, required=True)
    capture_file.add_argument("--probe", required=True)
    capture_file.add_argument("--name", required=True)

    write_output = subparsers.add_parser("write-output")
    write_output.add_argument("--path", type=Path, required=True)

    seal_record = subparsers.add_parser("seal-record")
    seal_record.add_argument("--payload", type=Path, required=True)
    seal_record.add_argument("--output", type=Path, required=True)

    probe_config = subparsers.add_parser("probe-config")
    probe_config.add_argument("--effort", default=None)
    probe_config.add_argument("--target", type=Path, required=True)

    config_drift = subparsers.add_parser("config-drift")
    config_drift.add_argument("--path", type=Path, required=True)
    config_drift.add_argument("--expected-file", type=Path, required=True)
    config_drift.add_argument("--workspace", required=True)

    proxy_egress = subparsers.add_parser("proxy-egress")
    proxy_egress.add_argument("--log", type=Path, required=True)
    proxy_egress.add_argument("--rep", required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.command == "capture-file":
            validate_fixture_dir(args.fixture_dir)
            contract = load_capture_contract(args.fixture_dir, args.probe)
            if args.name == "canonical-skill":
                probe_meta = parse_json_bytes(
                    captured_input_bytes(contract)["probe.json"],
                    "captured probe.json",
                )
                _, data = validate_embedded_canonical_skill(
                    contract["canonical_skill"], probe_meta, args.probe
                )
            elif args.name in {"prompt-control", "prompt-treatment"}:
                arm = args.name.removeprefix("prompt-")
                data = decode_prompts(contract["prompts"])[arm]
            elif args.name in capture_input_names(contract["treatment_source"]):
                data = captured_input_bytes(contract)[args.name]
            else:
                raise MetadataError(f"unknown captured input: {args.name!r}")
            sys.stdout.buffer.write(data)
            return 0
        if args.command == "write-output":
            data = sys.stdin.buffer.read(MAX_INPUT_BYTES + 1)
            if len(data) > MAX_INPUT_BYTES:
                raise MetadataError("scorecard output exceeds the safety limit")
            parse_json_bytes(data, "scorecard output")
            parent = args.path.parent
            if parent.is_symlink() or not parent.is_dir():
                raise MetadataError("scorecard output parent must be a directory")
            write_exclusive_bytes(args.path, data, "scorecard output")
            result = {"path": str(args.path)}
            print(json.dumps(result, separators=(",", ":")))
            return 0
        if args.command == "identity":
            result = evaluator_identity(
                args.harness, args.preamble, args.dispatch_helper
            )
        elif args.command == "probe-contract":
            result = probe_contract_summary(args.probe_dir, args.skills_dir, args.probe)
        elif args.command == "publish":
            result = publish_fixture_set(
                args.stage_dir,
                args.target_dir,
                args.probe_dir,
                args.skills_dir,
                args.probe,
            )
        elif args.command == "verify-scorecard":
            result = verify_scorecard(
                args.repo_root,
                args.skills_dir,
                args.scorecard,
                args.ledger_skill,
                args.ledger_probe,
                args.ledger_verdict,
            )
        elif args.command == "classify-open":
            contract = load_capture_contract(args.fixture_dir, args.probe)
            result = classify_open_capture(
                args.fd,
                args.path,
                contract,
                captured_input_bytes(contract)["discriminator.sh"],
                args.arm,
                args.rep,
            )
        elif args.command == "assemble-transcript":
            encoded = assemble_transcript(
                args.runtime_file,
                args.prompt_file,
                args.fixture_dir,
                args.probe,
                args.arm,
                args.rep,
                workspace=args.workspace,
                workspace_reset=args.workspace_reset,
                network_egress=(
                    json.loads(args.network_egress) if args.network_egress else None
                ),
            )
            sys.stdout.buffer.write(encoded)
            return 0
        elif args.command == "seal-record":
            result = write_seal_record(args.payload, args.output)
        elif args.command == "probe-config":
            text = render_probe_config(args.effort)
            args.target.write_text(text, encoding="utf-8")
            args.target.chmod(0o600)
            result = {
                "keys": [key for key in PROBE_CONFIG_KEYS if f"{key} =" in text],
                "sha256": digest_bytes(text.encode("utf-8")),
                "text": text,
            }
        elif args.command == "config-drift":
            result = {
                "findings": probe_config_drift(
                    args.path,
                    args.expected_file.read_text(encoding="utf-8"),
                    args.workspace,
                )
            }
        elif args.command == "proxy-egress":
            result = proxy_egress_summary(args.log, args.rep)
        elif args.command == "tier-skills":
            result = {"skills": tier_skills(args.skills_dir)}
        elif args.command == "snapshot":
            validate_fixture_dir(args.fixture_dir)
            contract = write_capture_contract(
                args.fixture_dir,
                args.probe_dir,
                args.skills_dir,
                args.probe,
                args.requested_model,
                args.requested_effort,
                args.producer_override_bin,
                args.seal_file,
            )
            result = {
                "binding_sha256": contract["binding_sha256"],
                "probe": contract["probe"],
                "reps": contract["reps"],
                "schedule": contract["schedule"],
                "scoring": contract["scoring"],
                "seal": contract["seal"],
                "treatment_source": contract["treatment_source"],
                "canonical_skill": {
                    key: contract["canonical_skill"][key]
                    for key in ("name", "path", "sha256")
                },
            }
        elif args.command == "score":
            validate_fixture_dir(args.fixture_dir)
            manifest = validate_manifest(
                args.fixture_dir, args.probe_dir, args.skills_dir, args.probe
            )
            result = recompute_score(args.probe_dir, args.fixture_dir, manifest)
            manifest_after = validate_manifest(
                args.fixture_dir, args.probe_dir, args.skills_dir, args.probe
            )
            if manifest_after["binding_sha256"] != manifest["binding_sha256"]:
                raise MetadataError("fixture binding changed during scoring")
        elif args.command in {"create", "verify", "observe"}:
            validate_fixture_dir(args.fixture_dir)
            if args.command == "create":
                reps = require_reps(args.reps)
                payload = build_payload(
                    args.fixture_dir,
                    args.probe_dir,
                    args.skills_dir,
                    args.harness,
                    args.preamble,
                    args.dispatch_helper,
                    args.probe,
                    reps,
                    args.requested_model,
                    args.requested_effort,
                )
                manifest = write_manifest(args.fixture_dir, payload)
                result = summary(
                    manifest, fixture_seal(args.fixture_dir, manifest, args.probe)
                )
            elif args.command == "verify":
                manifest = validate_manifest(
                    args.fixture_dir, args.probe_dir, args.skills_dir, args.probe
                )
                result = summary(
                    manifest, fixture_seal(args.fixture_dir, manifest, args.probe)
                )
            else:
                result = {"producer": observe_paths(transcript_paths(args.fixture_dir))}
        else:
            raise MetadataError(f"unsupported command: {args.command}")
    except (MetadataError, OSError) as exc:
        fail(str(exc))
    print(json.dumps(result, ensure_ascii=False, sort_keys=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
