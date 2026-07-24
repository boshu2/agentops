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
    "current_invocation": "docs/evidence/proof-epochs/epoch-0b/t0-pause-repair-intent.md",
    "current_candidate_identity": "enclosing subject manifest and fresh verdict",
}

PAUSE_PROOF_REF = "docs/contracts/proof-contracts/epoch-0b/descriptor.json"
PAUSE_PROOF_DIGEST = "b9735c94f7de98c4d31db93081351a6a1a78f8e03897dad61792277bb36a0302"


def load(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise EvidenceError(f"expected JSON object: {path}")
    return value


def require(condition: bool, message: str) -> None:
    if not condition:
        raise EvidenceError(message)


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def check_failed_invocation(
    *,
    verdict_path: Path,
    subject_path: Path,
    report_path: Path,
    artifact_digest: str,
    verdict_file_sha256: str,
    subject_file_sha256: str,
    report_file_sha256: str,
) -> None:
    require(sha256(verdict_path) == verdict_file_sha256, f"failed verdict bytes changed: {verdict_path}")
    require(sha256(subject_path) == subject_file_sha256, f"failed subject bytes changed: {subject_path}")
    require(sha256(report_path) == report_file_sha256, f"failed report bytes changed: {report_path}")
    verdict = load(verdict_path)
    subject = load(subject_path)
    report = load(report_path)
    require(verdict.get("verdict") == "FAIL", f"rejected verdict is not FAIL: {verdict_path}")
    require(
        verdict.get("artifact_digest") == artifact_digest,
        f"rejected verdict artifact identity changed: {verdict_path}",
    )
    require(
        verdict.get("subject_manifest_digest") == subject.get("canonical_manifest_digest"),
        f"rejected verdict and subject do not agree: {verdict_path}",
    )
    require(report.get("status") == "FAIL", f"rejected report is not FAIL: {report_path}")
    require(
        report.get("verdict_digest") == artifact_digest
        and report.get("subject_manifest_digest") == subject.get("canonical_manifest_digest"),
        f"rejected report lineage changed: {report_path}",
    )


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
    require(
        pause.get("schema_version") == "agentops.t0-pause-drill.v1",
        "pause drill schema version changed",
    )
    require(pause.get("result") == "PASS", "pause drill is not PASS")
    require(pause.get("lineage") == PAUSE_LINEAGE, "pause drill lineage is missing or inaccurate")
    require(
        pause.get("in_flight")
        == [
            "T0 pause-metadata repair is the current bounded invocation; its exact candidate identity is supplied by the enclosing subject manifest and fresh verdict",
            "T1 exact-kernel implementation has not started",
            "T2 compiler and Go CLI release tranches are reconnaissance only",
        ],
        "pause drill in-flight state is missing or inaccurate",
    )
    landed = pause.get("landed_or_frozen")
    require(isinstance(landed, list), "pause drill landed/frozen state must be an array")
    for required_entry in (
        "stable initial T0 candidate commit 932187258",
        f"immutable T0 FAIL {PAUSE_LINEAGE['initial_fail_artifact_digest']}",
        "stable T0 repair candidate commit 0e4bc0d90",
        f"immutable T0 repair FAIL {PAUSE_LINEAGE['repair_fail_artifact_digest']}",
        "active corrected epoch-0b proof descriptor and strict bootstrap transition recorder",
    ):
        require(required_entry in landed, f"pause drill omits stable state: {required_entry}")
    known_gaps = pause.get("known_gaps")
    require(isinstance(known_gaps, list), "pause drill known gaps must be an array")
    require(
        "T0 pause-metadata repair requires a fresh semantic verdict over its exact subject"
        in known_gaps,
        "pause drill omits the current validation gap",
    )
    require(
        "T1 exact-kernel implementation and epoch-1 activation have not started"
        in known_gaps,
        "pause drill falsely advances T1",
    )
    flattened_pause = json.dumps(pause, sort_keys=True).lower()
    require("t1 complete" not in flattened_pause, "pause drill falsely claims T1 complete")
    require(
        "repair invocation is not yet a stable commit" not in flattened_pause,
        "pause drill still claims the committed repair is unstable",
    )

    authority = pause.get("authority")
    require(isinstance(authority, dict), "pause drill authority must be an object")
    require(
        authority.get("proof_contract_pointer")
        == "docs/contracts/proof-contracts/active.json"
        and authority.get("proof_contract_ref") == PAUSE_PROOF_REF
        and authority.get("proof_contract_digest") == PAUSE_PROOF_DIGEST,
        "pause drill proof authority is missing or inaccurate",
    )
    pause_descriptor = repository / PAUSE_PROOF_REF
    require(
        sha256(pause_descriptor) == PAUSE_PROOF_DIGEST,
        "pause drill proof descriptor bytes changed",
    )
    active = load(repository / authority["proof_contract_pointer"])
    if active.get("epoch") == 0:
        require(
            active.get("contract_ref") == PAUSE_PROOF_REF
            and active.get("contract_digest") == PAUSE_PROOF_DIGEST,
            "live proof pointer does not match the pause authority",
        )
    else:
        transition_ref = active.get("activation_transition_ref")
        require(
            isinstance(transition_ref, str) and bool(transition_ref),
            "advanced proof pointer lacks an activation transition",
        )
        transition = load(repository / transition_ref)
        require(
            transition.get("prior", {}).get("contract_ref") == PAUSE_PROOF_REF
            and transition.get("prior", {}).get("contract_digest") == PAUSE_PROOF_DIGEST,
            "advanced proof pointer is not descended from the pause authority",
        )

    require(
        (repository / PAUSE_LINEAGE["current_invocation"]).is_file(),
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
