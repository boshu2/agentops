#!/usr/bin/env python3
"""Compile or record one shadow skill-contract.v3 receipt."""

from __future__ import annotations

import argparse
import os
from pathlib import Path
import sys
import tempfile

sys.dont_write_bytecode = True

from contract_v3 import ContractError, canonical_bytes, compile_skill  # noqa: E402


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def contained_output(root: Path, raw: str) -> Path:
    candidate = Path(raw)
    path = candidate if candidate.is_absolute() else root / candidate
    try:
        parent = path.parent.resolve(strict=True)
        parent.relative_to(root.resolve())
    except (OSError, ValueError) as exc:
        raise ContractError(
            "OUTPUT_PATH_INVALID",
            f"output parent is not inside the repository: {raw}",
        ) from exc
    if path.exists() and (path.is_symlink() or not path.is_file()):
        raise ContractError(
            "OUTPUT_PATH_INVALID",
            f"output must be a regular file or a missing path: {raw}",
        )
    return path


def atomic_write(path: Path, payload: bytes) -> None:
    descriptor, temporary = tempfile.mkstemp(
        prefix=f".{path.name}.",
        dir=path.parent,
    )
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Validate the shadow metadata.contract_v3 rail",
    )
    subparsers = parser.add_subparsers(dest="mode", required=True)
    check = subparsers.add_parser("check", help="render a receipt to stdout only")
    check.add_argument("--skill", required=True)
    record = subparsers.add_parser("record", help="write the exact checked receipt")
    record.add_argument("--skill", required=True)
    record.add_argument("--output", required=True)
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    root = repo_root()
    try:
        receipt = compile_skill(root, args.skill)
        payload = canonical_bytes(receipt)
        if args.mode == "check":
            sys.stdout.buffer.write(payload)
        else:
            output = contained_output(root, args.output)
            atomic_write(output, payload)
            print(output.relative_to(root).as_posix())
        return 0
    except ContractError as exc:
        print(f"[{exc.code}] {exc.message}", file=sys.stderr)
        return 1
    except OSError as exc:
        print(f"[IO_ERROR] {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
