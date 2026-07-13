#!/usr/bin/env python3
"""Authorize AGENTS cutover only from two pinned, distinct PASS judgments."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import subprocess
import sys


IDENTITY_FIELDS = (
    "contract_path",
    "contract_sha256",
    "candidate_path",
    "candidate_sha256",
    "pinned_head",
    "author_session",
)


def repo_path(repo: Path, relative: str) -> Path:
    path = Path(relative)
    if path.is_absolute():
        raise ValueError(f"path must be repository-relative: {relative}")
    resolved = (repo / path).resolve()
    resolved.relative_to(repo)
    return resolved


def load_unique(path: Path) -> dict:
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError(f"duplicate JSON key: {key}")
            result[key] = value
        return result

    return json.loads(path.read_text(encoding="utf-8"), object_pairs_hook=pairs)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument("--verdict-a", required=True)
    parser.add_argument("--verdict-b", required=True)
    parser.add_argument("--candidate", required=True)
    parser.add_argument("--contract", required=True)
    parser.add_argument("--expected-head", required=True)
    parser.add_argument("--expected-candidate-sha256", required=True)
    parser.add_argument("--expected-contract-sha256", required=True)
    parser.add_argument("--author-session", required=True)
    parser.add_argument("--judge-a-session", required=True)
    parser.add_argument("--judge-b-session", required=True)
    args = parser.parse_args()

    repo = Path(args.root).resolve()
    errors: list[str] = []
    if args.judge_a_session == args.judge_b_session:
        errors.append("trusted judge session identities must be distinct")
    if args.author_session in (args.judge_a_session, args.judge_b_session):
        errors.append("trusted judge session identity must differ from author")

    try:
        verdict_paths = [repo_path(repo, args.verdict_a), repo_path(repo, args.verdict_b)]
    except (OSError, ValueError) as exc:
        print(f"reconcile-agents-operating-contract-verdicts: {exc}", file=sys.stderr)
        return 2
    if verdict_paths[0] == verdict_paths[1]:
        errors.append("verdict artifacts must be distinct paths")

    checker = Path(__file__).resolve().with_name("check-agents-operating-contract-verdict.py")
    common = [
        "--root", str(repo),
        "--candidate", args.candidate,
        "--contract", args.contract,
        "--author-session", args.author_session,
        "--expected-head", args.expected_head,
        "--expected-candidate-sha256", args.expected_candidate_sha256,
        "--expected-contract-sha256", args.expected_contract_sha256,
    ]
    verdict_args = (
        (args.verdict_a, args.judge_a_session),
        (args.verdict_b, args.judge_b_session),
    )
    verdicts: list[dict] = []
    for label, (verdict_path, judge_session) in zip(("A", "B"), verdict_args):
        command = [
            sys.executable,
            str(checker),
            "--verdict", verdict_path,
            "--expected-judge-session", judge_session,
            *common,
        ]
        result = subprocess.run(command, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if result.returncode != 0:
            errors.append(f"verdict {label} failed factual validation: {result.stdout.strip()}")
        try:
            verdicts.append(load_unique(repo_path(repo, verdict_path)))
        except (OSError, json.JSONDecodeError, ValueError) as exc:
            errors.append(f"verdict {label} cannot be loaded: {exc}")

    if len(verdicts) == 2:
        for field in IDENTITY_FIELDS:
            if verdicts[0].get(field) != verdicts[1].get(field):
                errors.append(f"verdict artifact identity mismatch: {field}")
        if verdicts[0].get("judge_session") == verdicts[1].get("judge_session"):
            errors.append("verdicts claim the same judge_session")
        if verdicts[0].get("verdict") != "PASS" or verdicts[1].get("verdict") != "PASS":
            errors.append(
                "two independent PASS verdicts required: "
                f"A={verdicts[0].get('verdict')} B={verdicts[1].get('verdict')}"
            )

    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        print(f"reconcile-agents-operating-contract-verdicts: FAIL ({len(errors)} errors)", file=sys.stderr)
        return 1

    print(
        "reconcile-agents-operating-contract-verdicts: PASS "
        f"judges={args.judge_a_session},{args.judge_b_session} "
        f"candidate={args.candidate} head={args.expected_head}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
