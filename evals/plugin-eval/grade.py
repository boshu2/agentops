#!/usr/bin/env python3
"""Grade `claude plugin eval` responses against their rubric criteria with one judge call per response.

The evaluator's own LLM grader makes three judge calls per criterion. On this suite its default
judge (Haiku) failed criteria that responses plainly met, and an Opus judge cost several times the
generation itself. This script reads the evaluator's `--json` output, sends each response to the
judge model once with all of its criteria, and writes one boolean per criterion.

Usage:
    python3 evals/plugin-eval/grade.py RUN.json [RUN.json ...] --out grades.json

Each RUN.json is a file written by `claude plugin eval ... --json RUN.json`. The output maps
"<file>|<case>|<arm>|<run index>" to a list of booleans in criterion order, or to {"error": ...}.
Existing entries in --out are kept, so an interrupted run can be resumed.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor
from typing import Any

JUDGE_PROMPT = (
    "You are grading an AI assistant's response against criteria. Judge only the response text. "
    "For each criterion decide true (clearly satisfied) or false. Reply with ONLY a JSON array of "
    "{n} booleans, in order, no prose.\n\nCRITERIA:\n{criteria}\n\n<response>\n{response}\n</response>"
)


def rubric(case: dict[str, Any]) -> list[tuple[str, str]]:
    """Return (grader name, criteria text) for each LLM grader of a case, in order."""
    return [
        (g["name"], g["config"]["criteria"].strip())
        for g in case["graders"]
        if g["type"] == "llm"
    ]


def response_text(run: dict[str, Any], names: list[str]) -> str:
    """Return the response the evaluator showed its judge (the final assistant message)."""
    for grader in run.get("graders", []):
        if grader["name"] in names and grader.get("evidence"):
            return str(grader["evidence"])
    return ""


def jobs(
    paths: list[str], done: dict[str, Any], arm_filter: str | None = None
) -> list[tuple[str, list[str], str]]:
    """List (key, criteria, response) for every gradable run not already in `done`."""
    out = []
    for path in paths:
        with open(path, encoding="utf-8") as fh:
            doc = json.load(fh)
        label = os.path.basename(path)
        for case in doc["cases"]:
            crit = rubric(case)
            if not crit:
                continue
            names = [name for name, _ in crit]
            for arm, runs in case["arms"].items():
                if arm_filter and arm != arm_filter:
                    continue
                for index, run in enumerate(runs):
                    key = f"{label}|{case['name']}|{arm}|{index}"
                    text = response_text(run, names)
                    if key in done or run.get("error") or not text:
                        continue
                    out.append((key, [text_ for _, text_ in crit], text))
    return out


def grade(job: tuple[str, list[str], str], model: str, timeout: int) -> tuple[str, Any]:
    """Ask the judge model once for all criteria of one response."""
    key, criteria, response = job
    prompt = JUDGE_PROMPT.format(
        n=len(criteria),
        criteria="\n".join(f"{i}. {c}" for i, c in enumerate(criteria, 1)),
        response=response,
    )
    try:
        proc = subprocess.run(
            ["claude", "-p", "--model", model, "--tools", "", "--setting-sources", ""],
            input=prompt,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        return key, {"error": f"judge call failed: {exc}"}
    match = re.search(r"\[[^\[\]]*\]", proc.stdout)
    if proc.returncode != 0 or not match:
        return key, {
            "error": f"judge exit {proc.returncode}: {(proc.stdout or proc.stderr)[:200]}"
        }
    try:
        verdicts = json.loads(match.group(0))
    except json.JSONDecodeError as exc:
        return key, {"error": f"unparseable verdicts: {exc}"}
    if len(verdicts) != len(criteria) or not all(isinstance(v, bool) for v in verdicts):
        return key, {"error": f"expected {len(criteria)} booleans, got {verdicts!r}"}
    return key, verdicts


def main() -> int:
    """Grade every ungraded run and report how many failed to grade."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("runs", nargs="+", help="evaluator --json output files")
    parser.add_argument("--out", required=True, help="grades file to write (resumable)")
    parser.add_argument(
        "--arm", choices=["with", "without"], help="grade only this arm"
    )
    parser.add_argument("--judge-model", default="claude-opus-5-5")
    parser.add_argument("--concurrency", type=int, default=6)
    parser.add_argument(
        "--timeout", type=int, default=300, help="seconds per judge call"
    )
    args = parser.parse_args()

    done: dict[str, Any] = {}
    if os.path.exists(args.out):
        with open(args.out, encoding="utf-8") as fh:
            done = {k: v for k, v in json.load(fh).items() if isinstance(v, list)}
    todo = jobs(args.runs, done, args.arm)
    with ThreadPoolExecutor(args.concurrency) as pool:
        for key, verdicts in pool.map(
            lambda job: grade(job, args.judge_model, args.timeout), todo
        ):
            done[key] = verdicts
            with open(args.out, "w", encoding="utf-8") as fh:
                json.dump(done, fh, indent=1, sort_keys=True)
    failed = sorted(k for k, v in done.items() if not isinstance(v, list))
    print(
        f"graded {len(done) - len(failed)} responses, {len(failed)} failed",
        file=sys.stderr,
    )
    for key in failed:
        print(f"  FAILED {key}: {done[key]['error']}", file=sys.stderr)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
