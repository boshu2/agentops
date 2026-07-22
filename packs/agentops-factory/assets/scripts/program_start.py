#!/usr/bin/env python3
"""Start one bounded factory program from one exact source Bead."""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from typing import Any


def canonical(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=True, separators=(",", ":"), sort_keys=True).encode() + b"\n"


def digest(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def required(name: str) -> str:
    value = os.environ.get(name, "")
    if not value:
        raise ValueError(f"{name} is required from the managed city")
    return value


def exact_file(value: str, label: str, *, executable: bool = False) -> Path:
    requested = Path(value)
    if requested.is_symlink():
        raise ValueError(f"{label} must not be a symlink: {requested}")
    path = requested.resolve(strict=True)
    if not path.is_file() or (executable and not os.access(path, os.X_OK)):
        raise ValueError(f"{label} is not an admitted regular{' executable' if executable else ''} file: {path}")
    return path


def exact_object(path: Path, expected_digest: str, label: str) -> dict[str, Any]:
    raw = path.read_bytes()
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ValueError(f"{label} is invalid JSON") from exc
    if not isinstance(value, dict) or canonical(value) != raw or digest(raw) != expected_digest:
        raise ValueError(f"{label} bytes differ from the managed identity")
    return value


def rig_id(value: object) -> str:
    if not isinstance(value, str) or re.fullmatch(r"[A-Za-z0-9_-]+", value) is None:
        raise ValueError("native delivery rig_id has an unsafe identity")
    return value


def nested_records(value: Any, identifier: str) -> list[dict[str, Any]]:
    matches: list[dict[str, Any]] = []
    if isinstance(value, list):
        for item in value:
            matches.extend(nested_records(item, identifier))
    elif isinstance(value, dict):
        if str(value.get("id", "")) == identifier:
            matches.append(value)
        else:
            for key in ("issue", "bead", "issues", "beads", "result", "data"):
                if key in value:
                    matches.extend(nested_records(value[key], identifier))
    return matches


def atomic_bytes(path: Path, raw: bytes, label: str) -> None:
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    if path.exists():
        if path.is_symlink() or path.read_bytes() != raw:
            raise ValueError(f"existing {label} conflicts: {path}")
        return
    handle, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        os.fchmod(handle, 0o600)
        with os.fdopen(handle, "wb") as output:
            output.write(raw)
            output.flush()
            os.fsync(output.fileno())
        try:
            os.link(temporary, path)
        except FileExistsError:
            if path.is_symlink() or path.read_bytes() != raw:
                raise ValueError(f"racing {label} conflicts: {path}")
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass


def bead_intent(repository: Path, bd_bin: Path, source_bead: str) -> Path:
    intake = repository / ".gc" / "agentops" / "factory" / "intake" / digest(source_bead.encode())
    path = intake / "intent-source.json"
    if path.exists():
        value = exact_object(path, digest(path.read_bytes()), "existing Bead intent snapshot")
        if value.get("schema_version") != "factory-bead-intent.v1" or value.get("source_bead_id") != source_bead:
            raise ValueError("existing Bead intent snapshot has a different source identity")
        return path
    completed = subprocess.run(
        [str(bd_bin), "show", source_bead, "--json"], cwd=repository, check=False, capture_output=True,
    )
    if completed.returncode:
        raise ValueError((completed.stderr or completed.stdout).decode(errors="replace").strip() or "bd show failed")
    try:
        response = json.loads(completed.stdout)
    except json.JSONDecodeError as exc:
        raise ValueError("bd show did not return JSON") from exc
    matches = nested_records(response, source_bead)
    if len(matches) != 1:
        raise ValueError("bd show did not return one exact source Bead")
    atomic_bytes(path, canonical({"schema_version": "factory-bead-intent.v1", "source_bead_id": source_bead, "bead": matches[0]}), "Bead intent snapshot")
    return path


def parser() -> argparse.ArgumentParser:
    value = argparse.ArgumentParser(description=__doc__)
    value.add_argument("--source-bead", required=True, help="one existing Bead ID")
    value.add_argument("--intent", help="optional explicit intent file; default snapshots the source Bead")
    value.add_argument("--base-ref", help="must match the managed delivery base ref")
    value.add_argument("--max-parallel", type=int, default=2, choices=range(1, 65), metavar="1..64")
    return value


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    if re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,255}", args.source_bead) is None:
        raise ValueError("--source-bead has an unsafe identity")
    requested_pack = Path(required("GC_PACK_DIR"))
    if requested_pack.is_symlink():
        raise ValueError("GC_PACK_DIR must not be a symlink")
    pack = requested_pack.resolve(strict=True)
    if not pack.is_dir():
        raise ValueError("GC_PACK_DIR is not a regular managed pack directory")
    feeder = exact_file(str(pack / "assets/scripts/factory_feeder.py"), "factory feeder")
    role_adapter = exact_file(str(pack / "assets/scripts/role_adapter.py"), "role adapter")
    packet_adapter = exact_file(str(pack.parent / "agentops-executor/assets/scripts/packet.py"), "packet adapter")
    factory_check = exact_file(str(Path(required("GC_CITY")) / ".gc/scripts/agentops-factory-check"), "factory check", executable=True)
    native_path = exact_file(required("AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT"), "native delivery context")
    native = exact_object(native_path, required("AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT_DIGEST"), "native delivery context")
    native_rig_id = rig_id(native.get("rig_id"))
    requested_repository = Path(str(native.get("repository_dir", "")))
    if requested_repository.is_symlink():
        raise ValueError("native delivery repository must not be a symlink")
    repository = requested_repository.resolve(strict=True)
    if not repository.is_dir():
        raise ValueError("native delivery repository is unavailable")
    base_ref = args.base_ref or str(native.get("base_ref", ""))
    if not base_ref or base_ref != native.get("base_ref"):
        raise ValueError("requested base ref differs from the managed native context")
    gc_bin = exact_file(required("GC_BIN"), "Gas City binary", executable=True)
    bd_bin = exact_file(required("AGENTOPS_GC_BEADS_BIN"), "Beads binary", executable=True)
    git_bin = exact_file(required("AGENTOPS_GC_GIT_BIN"), "Git binary", executable=True)
    intent = exact_file(args.intent, "explicit intent") if args.intent else bead_intent(repository, bd_bin, args.source_bead)
    created_at = datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    command = [
        sys.executable, str(feeder), "start", "--root", str(repository), "--repository", str(repository),
        "--source-bead", args.source_bead, "--intent", str(intent), "--base-ref", base_ref,
        "--max-parallel", str(args.max_parallel), "--bd-bin", str(bd_bin), "--gc-bin", str(gc_bin),
        "--git-bin", str(git_bin), "--role-adapter", str(role_adapter), "--packet-adapter", str(packet_adapter),
        "--factory-check", str(factory_check), "--created-at", created_at,
        "--rig-id", native_rig_id,
    ]
    return subprocess.run(command, cwd=repository, check=False).returncode


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError) as exc:
        print(f"agentops factory program start: {exc}", file=sys.stderr)
        raise SystemExit(2)
