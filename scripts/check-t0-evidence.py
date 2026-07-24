#!/usr/bin/env python3
"""Verify the machine-readable T0 evidence ledgers against the live tree."""

from __future__ import annotations

import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import sys
from typing import Any


class EvidenceError(ValueError):
    pass


def load(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise EvidenceError(f"expected JSON object: {path}")
    return value


def require(condition: bool, message: str) -> None:
    if not condition:
        raise EvidenceError(message)


def check(repository: Path, evidence_root: Path) -> dict[str, Any]:
    skills = load(evidence_root / "t0-skill-ledger.json")
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
        path = repository / row["path"]
        require(path.is_file(), f"skill ledger path is missing: {row['path']}")
        actual = hashlib.sha256(path.read_bytes()).hexdigest()
        require(actual == row["sha256"], f"skill ledger digest changed: {row['path']}")

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
    require(pause.get("result") == "PASS", "pause drill is not PASS")
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
