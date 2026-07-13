#!/usr/bin/env python3
"""Validate factual AGENTS behavior-verdict evidence without judging prose."""

from __future__ import annotations

import argparse
from datetime import datetime
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

from jsonschema import Draft202012Validator, FormatChecker


SCENARIOS = {
    "authority",
    "trust-boundary",
    "law0-runtime",
    "precedence",
    "ordered-loop-repair",
    "exact-done",
    "concurrency",
    "triggered-routes",
    "closeout",
}


def load_json_unique(path: Path) -> dict:
    def unique_pairs(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError(f"duplicate JSON key: {key}")
            result[key] = value
        return result

    return json.loads(path.read_text(encoding="utf-8"), object_pairs_hook=unique_pairs)


def within_repo(repo: Path, relative: str) -> Path:
    path = Path(relative)
    if path.is_absolute():
        raise ValueError(f"path must be repository-relative: {relative}")
    resolved = (repo / path).resolve()
    try:
        resolved.relative_to(repo)
    except ValueError as exc:
        raise ValueError(f"path escapes repository: {relative}") from exc
    return resolved


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument("--verdict", required=True)
    parser.add_argument("--candidate", required=True)
    parser.add_argument("--contract", required=True)
    parser.add_argument("--author-session", required=True)
    parser.add_argument("--expected-judge-session", required=True)
    parser.add_argument("--expected-head", required=True)
    parser.add_argument("--expected-candidate-sha256", required=True)
    parser.add_argument("--expected-contract-sha256", required=True)
    args = parser.parse_args()

    repo = Path(args.root).resolve()
    errors: list[str] = []
    try:
        verdict = load_json_unique(within_repo(repo, args.verdict))
        schema = load_json_unique(
            within_repo(repo, "schemas/agents-operating-contract-verdict.v1.schema.json")
        )
    except (OSError, json.JSONDecodeError, ValueError) as exc:
        print(f"check-agents-operating-contract-verdict: {exc}", file=sys.stderr)
        return 2

    validator = Draft202012Validator(schema, format_checker=FormatChecker())
    for violation in sorted(validator.iter_errors(verdict), key=lambda item: list(item.path)):
        location = ".".join(str(part) for part in violation.path) or "verdict"
        errors.append(f"schema {location}: {violation.message}")

    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        return 1

    try:
        head = subprocess.run(
            ["git", "-C", str(repo), "rev-parse", "HEAD"],
            check=True,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        ).stdout.strip()
        contract = within_repo(repo, verdict["contract_path"])
        candidate = within_repo(repo, verdict["candidate_path"])
    except (subprocess.CalledProcessError, ValueError) as exc:
        print(f"check-agents-operating-contract-verdict: {exc}", file=sys.stderr)
        return 2

    if not contract.is_file():
        errors.append(f"contract does not exist: {verdict['contract_path']}")
    else:
        contract_digest = hashlib.sha256(contract.read_bytes()).hexdigest()
        if verdict["contract_sha256"] != contract_digest:
            errors.append("contract_sha256 differs from contract bytes")
    if not candidate.is_file():
        errors.append(f"candidate does not exist: {verdict['candidate_path']}")
    if verdict["candidate_path"] != args.candidate:
        errors.append("candidate path differs from requested artifact")
    if verdict["contract_path"] != args.contract:
        errors.append("contract path differs from requested artifact")
    if verdict["pinned_head"] != head:
        errors.append(f"pinned_head {verdict['pinned_head']} differs from HEAD {head}")
    if head != args.expected_head:
        errors.append(f"HEAD {head} differs from expected head {args.expected_head}")
    if verdict["candidate_sha256"] != args.expected_candidate_sha256:
        errors.append("candidate_sha256 differs from expected candidate identity")
    if verdict["contract_sha256"] != args.expected_contract_sha256:
        errors.append("contract_sha256 differs from expected contract identity")
    if verdict["author_session"] != args.author_session:
        errors.append("author_session differs from invocation")
    if verdict["judge_session"] != args.expected_judge_session:
        errors.append("judge_session differs from trusted dispatch identity")
    if verdict["judge_session"] == verdict["author_session"]:
        errors.append("judge_session must differ from author_session")

    if candidate.is_file():
        digest = hashlib.sha256(candidate.read_bytes()).hexdigest()
        if verdict["candidate_sha256"] != digest:
            errors.append("candidate_sha256 differs from candidate bytes")
        lines = candidate.read_text(encoding="utf-8").splitlines()
        for result in verdict["scenario_results"]:
            for citation in result["citations"]:
                if citation["path"] != verdict["candidate_path"]:
                    errors.append(f"{result['scenario_id']}: citation path is not the candidate")
                    continue
                start, end = citation["line_start"], citation["line_end"]
                if end < start or end > len(lines):
                    errors.append(f"{result['scenario_id']}: citation line range is invalid")
                    continue
                actual = "\n".join(lines[start - 1 : end])
                if citation["quote"] != actual:
                    errors.append(f"{result['scenario_id']}: citation quote differs from candidate lines")

    scenario_ids = [item["scenario_id"] for item in verdict["scenario_results"]]
    if len(scenario_ids) != len(set(scenario_ids)):
        errors.append("scenario_results contains duplicate scenario IDs")
    missing = sorted(SCENARIOS - set(scenario_ids))
    extra = sorted(set(scenario_ids) - SCENARIOS)
    if missing:
        errors.append("scenario_results missing: " + ", ".join(missing))
    if extra:
        errors.append("scenario_results unknown: " + ", ".join(extra))

    has_fail = any(item["verdict"] == "FAIL" for item in verdict["scenario_results"])
    has_warn = any(item["verdict"] == "WARN" for item in verdict["scenario_results"])
    has_blocker = any(item["severity"] == "blocking" for item in verdict["findings"])
    has_nonblocker = any(item["severity"] == "nonblocking" for item in verdict["findings"])
    command_failed = any(item["exit_code"] != 0 for item in verdict["commands"])
    if has_fail or has_blocker or command_failed:
        expected_verdict = "FAIL"
    elif has_warn or has_nonblocker or verdict["not_checked"]:
        expected_verdict = "WARN"
    else:
        expected_verdict = "PASS"
    if verdict["verdict"] != expected_verdict:
        errors.append(
            f"aggregate verdict {verdict['verdict']} must be {expected_verdict} from structured facts"
        )

    if not re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})", verdict["validated_at"]):
        errors.append("validated_at must be explicit timezone-aware RFC3339")
    else:
        try:
            if datetime.fromisoformat(verdict["validated_at"].replace("Z", "+00:00")).tzinfo is None:
                errors.append("validated_at must include a timezone")
        except ValueError:
            errors.append("validated_at is not a real RFC3339 timestamp")

    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        print(f"check-agents-operating-contract-verdict: FAIL ({len(errors)} errors)", file=sys.stderr)
        return 1

    print(
        "check-agents-operating-contract-verdict: FACTS_VALID "
        f"judge={verdict['judge_session']} candidate={verdict['candidate_path']} head={head}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
