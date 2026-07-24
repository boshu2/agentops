#!/usr/bin/env python3
"""Run one declared contract-v3 proof and render or record its receipt."""

from __future__ import annotations

import argparse
import hashlib
from pathlib import Path
import shlex
import subprocess
import sys

sys.dont_write_bytecode = True

from compile_contracts import atomic_write, contained_output  # noqa: E402
from contract_v3 import (  # noqa: E402
    ContractError,
    canonical_bytes,
    compile_skill,
    file_sha256,
    load_frontmatter,
)


RUNNER_REF = "skills/skill-builder/scripts/run_contract_probe.py"


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Execute one declared skill-contract.v3 proof",
    )
    subparsers = parser.add_subparsers(dest="mode", required=True)
    check = subparsers.add_parser("check", help="run and render the receipt")
    check.add_argument("--skill", required=True)
    record = subparsers.add_parser("record", help="run and record the receipt")
    record.add_argument("--skill", required=True)
    record.add_argument("--output", required=True)
    return parser.parse_args(argv)


def run_probe(root: Path, skill_name: str) -> dict[str, object]:
    compiled = compile_skill(root, skill_name)
    frontmatter = load_frontmatter(root / compiled["source"]["ref"])
    contract = frontmatter["metadata"]["contract_v3"]
    proof = contract["proof"]
    command = shlex.split(proof["command"])
    try:
        completed = subprocess.run(
            command,
            cwd=root,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=120,
            check=False,
        )
        result = "PASS" if completed.returncode == 0 else "FAIL"
        exit_code = completed.returncode
        stdout = completed.stdout
        stderr = completed.stderr
    except subprocess.TimeoutExpired as exc:
        result = "NOT_PROVEN"
        exit_code = 124
        stdout = exc.stdout or b""
        stderr = exc.stderr or b""
    after = file_sha256(root / compiled["source"]["ref"])
    unchanged = after == compiled["source"]["before_sha256"]
    if not unchanged:
        result = "NOT_PROVEN"
    return {
        "schema_version": "skill-contract-probe-receipt.v1",
        "skill": skill_name,
        "source": {
            "ref": compiled["source"]["ref"],
            "before_sha256": compiled["source"]["before_sha256"],
            "after_sha256": after,
            "unchanged": unchanged,
        },
        "contract_digest": compiled["contract"]["digest"],
        "compiler_digest": compiled["compiler"]["digest"],
        "runner": {
            "ref": RUNNER_REF,
            "sha256": file_sha256(root / RUNNER_REF),
        },
        "proof": {
            "class": proof["class"],
            "command": proof["command"],
            "fixture_refs": compiled["fixtures"]["refs"],
            "fixture_digest": compiled["fixtures"]["digest"],
        },
        "execution": {
            "exit_code": exit_code,
            "stdout_sha256": hashlib.sha256(stdout).hexdigest(),
            "stderr_sha256": hashlib.sha256(stderr).hexdigest(),
        },
        "result": result,
    }


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    root = repo_root()
    try:
        receipt = run_probe(root, args.skill)
        payload = canonical_bytes(receipt)
        if args.mode == "check":
            sys.stdout.buffer.write(payload)
        else:
            output = contained_output(root, args.output)
            atomic_write(output, payload)
            print(output.relative_to(root).as_posix())
        return 0 if receipt["result"] == "PASS" else 1
    except ContractError as exc:
        print(f"[{exc.code}] {exc.message}", file=sys.stderr)
        return 1
    except OSError as exc:
        print(f"[IO_ERROR] {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
