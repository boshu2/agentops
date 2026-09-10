#!/usr/bin/env python3
"""Rebuild a compact pilot readout from skill-trial-report native JSON.

No launch, grading, lifecycle writes, model calls, or implicit directory scans.
Optional externally captured receipts join assignments to native job evidence;
their assertions are evidence with the limits documented in readout.md.
"""
from __future__ import annotations

import argparse
from collections import Counter, defaultdict
from copy import deepcopy
import hashlib
import json
import math
from pathlib import Path
import statistics
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))


def document(owner, name):
    value = (owner.get("documents", {}).get(name) or {}).get("data")
    return value if isinstance(value, dict) else {}


def number(value):
    return isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value) and value >= 0


def digest(value):
    return isinstance(value, str) and len(value.removeprefix("sha256:")) == 64 and all(
        char in "0123456789abcdef" for char in value.removeprefix("sha256:"))


def same_digest(a, b):
    return digest(a) and digest(b) and a.removeprefix("sha256:") == b.removeprefix("sha256:")


def normalized_config(config):
    """Ignore only output locations and independently bound task/package paths."""
    value = deepcopy(config)
    for key in ("job_name", "jobs_dir", "trial_name", "trials_dir", "job_id"):
        value.pop(key, None)
    for task in value.get("tasks", []) + ([value["task"]] if "task" in value else []):
        task.pop("path", None)
    for agent in value.get("agents", []) + ([value["agent"]] if "agent" in value else []):
        agent.pop("skills", None)
    return value


def validate_receipt(job, trial, receipt):
    reasons = []
    if receipt is None:
        return ["missing external launch receipt"]
    if receipt.get("arm") != job.get("arm"):
        reasons.append("receipt arm differs from report label")
    if not isinstance(receipt.get("rep"), int) or isinstance(receipt.get("rep"), bool) or receipt["rep"] < 1:
        reasons.append("missing positive repetition identity")
    if not receipt.get("task"):
        reasons.append("missing task identity")
    if not same_digest(receipt.get("configuration_sha256"),
                       (job.get("documents", {}).get("config.json") or {}).get("sha256")):
        reasons.append("native job configuration hash missing or mismatched")
    if not same_digest(receipt.get("task_checksum"), trial.get("task_checksum")):
        reasons.append("native task checksum missing or mismatched")
    if not digest(receipt.get("oracle_sha256")):
        reasons.append("missing frozen oracle identity")
    if receipt.get("skills_sha256") is not None and not digest(receipt["skills_sha256"]):
        reasons.append("invalid package identity")
    if "skills_sha256" not in receipt:
        reasons.append("missing package availability identity")
    runtime = receipt.get("runtime") or {}
    for key in ("harbor_version", "codex_version", "model", "reasoning_effort", "worker_image_id", "verifier_image_id"):
        if not runtime.get(key):
            reasons.append("missing runtime " + key)
    isolation = receipt.get("isolation") or {}
    if (isolation.get("separate_verifier") is not True or
            isolation.get("private_inputs_excluded") is not True or not isolation.get("network_policy")):
        reasons.append("missing isolation configuration evidence")
    if isolation.get("contamination_detected") is True or receipt.get("contamination_detected") is True:
        reasons.append("contamination detected")
    native = document(trial, "result.json")
    if native.get("verifier_environment_mode") != "separate":
        reasons.append("native separate verifier not evidenced")
    info = native.get("agent_info") or {}
    model_info = info.get("model_info") or {}
    actual_model = "/".join((model_info.get("provider", ""), model_info.get("name", "")))
    if info.get("version") != runtime.get("codex_version") or actual_model != runtime.get("model"):
        reasons.append("native executed model/version differs from receipt or is unknown")
    config = document(job, "config.json")
    agents = config.get("agents") or []
    if len(agents) != 1:
        reasons.append("native agent configuration ambiguous")
    else:
        agent = agents[0]
        kwargs = agent.get("kwargs") or {}
        for observed, expected, label in (
            (agent.get("model_name"), runtime.get("model"), "model"),
            (kwargs.get("version"), runtime.get("codex_version"), "Codex version"),
            (kwargs.get("reasoning_effort"), runtime.get("reasoning_effort"), "reasoning effort"),
        ):
            if not observed or observed != expected:
                reasons.append("native " + label + " differs from receipt")
        if bool(agent.get("skills")) != bool(receipt.get("skills_sha256")):
            reasons.append("native skill availability differs from receipt")
        effective = document(trial, "config.json").get("agent") or {}
        for key in ("model_name", "kwargs", "skills"):
            if effective.get(key) != agent.get(key):
                reasons.append("effective native agent " + key + " differs from launch configuration")
    if job.get("diagnostics") or trial.get("diagnostics"):
        reasons.append("native accounting diagnostics require review")
    if not document(trial, "config.json"):
        reasons.append("missing effective native trial configuration")
    return reasons


def distribution(values):
    measured = [value for value in values if number(value)]
    return {"measured": len(measured), "unknown": len(values) - len(measured),
            "min": min(measured) if measured else None,
            "median": statistics.median(measured) if measured else None,
            "max": max(measured) if measured else None,
            "known_sum": sum(measured)}


def verifier_grade(trial, identity_exclusions):
    """Read only the separately captured verifier document, never worker prose."""
    evidence = (trial.get("documents") or {}).get("verifier/grade.json") or {}
    grade = document(trial, "verifier/grade.json")
    reasons = list(identity_exclusions)
    if not evidence.get("path") or not digest(evidence.get("sha256")) or not grade:
        reasons.append("missing or invalid captured verifier grade")
    if not trial.get("completed"):
        reasons.append("no native terminal verifier evidence")
    source = {key: evidence.get(key) for key in ("path", "sha256")}
    gaps = grade.get("not_checked")
    if not isinstance(gaps, list) or not all(isinstance(gap, str) for gap in gaps):
        gaps = None
    cases = grade.get("case_results")
    classifications = {"correct", "false_acceptance", "false_blocker", "justified_not_proven", "incorrect_disposition", "missing"}
    valid_cases = isinstance(cases, list) and bool(cases) and all(
        isinstance(case, dict) and isinstance(case.get("case_id"), str) and case["case_id"]
        and case.get("expected") in ("PASS", "FAIL", "NOT_PROVEN")
        and "actual" in case and case.get("classification") in classifications for case in cases)
    if valid_cases and len({case["case_id"] for case in cases}) != len(cases):
        valid_cases = False
    return {"source": source, "identity_exclusions": reasons,
            "case_results": cases if valid_cases and not reasons else None,
            "case_results_unknown": not valid_cases or bool(reasons),
            "not_checked": gaps if not reasons else None}


def validation_cases(rows):
    cases = [{"job": row["job"], **case, "source": row["verifier_grade"]["source"]}
             for row in rows for case in (row["verifier_grade"]["case_results"] or [])]
    missing_grades = sum(row["verifier_grade"]["case_results_unknown"] for row in rows)
    result = {"case_results": cases, "attempts_without_case_grade": missing_grades}
    for name, expected in (("false_acceptance", {"FAIL", "NOT_PROVEN"}),
                           ("false_blocker", {"PASS"}), ("justified_not_proven", {"NOT_PROVEN"})):
        relevant = [case for case in cases if case["expected"] in expected]
        unknown = sum(case["classification"] == "missing" for case in relevant)
        known_count = sum(case["classification"] == name for case in relevant)
        denominator = len(relevant) if cases and not missing_grades else None
        count = known_count if denominator is not None and not unknown else None
        result[name] = {"count": count, "denominator": denominator,
                        "known_count": known_count, "known_denominator": len(relevant), "unknown_cases": unknown,
                        "rate": count / denominator if count is not None and denominator else None}
    return result


def uncertainty(pairs, *, n_required=None, resamples=10000):
    if not pairs:
        return {"status": "unavailable", "reason": "no comparable endpoint pairs"}
    try:
        from _stats.bootstrap import paired_cluster_bootstrap
        from _stats.inputs import BootstrapInput, bootstrap_inputs_hash, paired_sample_ids_hash
        from _stats.seed import derive_bootstrap_seed
        from _stats.verdict import compute_verdict
    except ImportError as error:
        return {"status": "unavailable", "reason": "statistics dependency unavailable: " + str(error)}
    inputs = [BootstrapInput(pair["task"], pair["rep"], pair["treatment_score"], pair["control_score"]) for pair in pairs]
    rule = {"kind": "ci_excludes_zero", "confidence": 0.95}
    seed = derive_bootstrap_seed("skills-rpi", ["treatment", "control"], paired_sample_ids_hash(inputs), rule)
    result = paired_cluster_bootstrap(inputs, bootstrap_seed=seed, B=resamples)
    signal = compute_verdict(result, n_required=n_required).kind.value if n_required is not None else "pilot_descriptive_only"
    interval_status = "insufficient_clusters" if result.n_clusters < 2 else "degenerate_no_interval" if result.degenerate else "available"
    interval_available = interval_status == "available"
    return {"status": interval_status, "signal": signal, "delta_treatment_minus_control": result.delta_point,
            "ci_low": result.ci_low if interval_available else None,
            "ci_high": result.ci_high if interval_available else None,
            "confidence": result.confidence if interval_available else None,
            "degenerate": result.degenerate, "task_clusters": result.n_clusters, "paired_repetitions": len(inputs),
            "n_required": n_required, "resamples": result.B, "bootstrap_seed": seed,
            "inputs_sha256": bootstrap_inputs_hash(inputs),
            "interpretation": "Task-cluster bootstrap retains paired repetitions. No interval is shown with fewer than two task clusters or degenerate deltas. A zero-crossing interval or no_change is not equivalence. Pilot results do not establish general skill benefit."}


def build(report, receipts=None, *, control="control", treatment="treatment", n_required=None, resamples=10000):
    if control == treatment:
        raise ValueError("control and treatment labels must differ")
    if n_required is not None and n_required < 1:
        raise ValueError("n_required must be positive and predeclared")
    if resamples < 1:
        raise ValueError("resamples must be positive")
    receipts = receipts or {"schema_version": 1, "trials": []}
    if receipts.get("schema_version") != 1:
        raise ValueError("unsupported receipt schema_version")
    by_job = defaultdict(list)
    for receipt in receipts.get("trials", []):
        by_job[receipt.get("job_name")].append(receipt)
    attempts, groups, arms = [], defaultdict(list), defaultdict(list)
    session_trials = defaultdict(set)
    suspect_sessions = set()
    for session in report.get("sessions", []):
        if session.get("diagnostics"):
            suspect_sessions.add(session.get("id"))
        for copy in session.get("copies", []):
            if copy.get("trial"):
                session_trials[session.get("id")].add(copy["trial"])
            if (copy.get("accounting") or {}).get("diagnostics"):
                suspect_sessions.add(session.get("id"))
    seen_jobs = Counter(Path(job["directory"]).name for job in report.get("jobs", []))
    for job in report.get("jobs", []):
        arm = job.get("arm", "")
        arms[arm].append(job)
        name = Path(job["directory"]).name
        candidates = by_job.get(name, [])
        receipt = candidates[0] if len(candidates) == 1 else None
        for trial in job.get("trials", []):
            reasons = validate_receipt(job, trial, receipt)
            for session_id in trial.get("session_ids", []):
                if len(session_trials[session_id]) > 1:
                    reasons.append("native session identity shared across trials")
                if session_id in suspect_sessions:
                    reasons.append("native session accounting diagnostics require review")
            if seen_jobs[name] != 1 or len(candidates) > 1 or len(job.get("trials", [])) != 1:
                reasons.append("ambiguous job, receipt, or retry assignment")
            grade = verifier_grade(trial, reasons)
            if arm not in (control, treatment):
                reasons.append("arm outside selected comparison")
            native = document(trial, "result.json")
            outcome = trial.get("outcome", "unknown_outcome")
            if outcome not in ("success", "failed", "execution_error"):
                reasons.append("endpoint comparison unavailable: " + outcome)
            if not trial.get("completed"):
                reasons.append("no native terminal endpoint")
            row = {"job": name, "directory": trial.get("directory"), "trial_id": trial.get("id"),
                   "task": receipt.get("task") if receipt else trial.get("task_name"),
                   "rep": receipt.get("rep") if receipt else None, "arm": arm, "outcome": outcome,
                   "elapsed_seconds": (trial.get("timing") or {}).get("elapsed_seconds"),
                   "timing": trial.get("timing"), "comparison_exclusions": reasons,
                   "harbor_agent_result": native.get("agent_result"),
                   "native_rewards": (native.get("verifier_result") or {}).get("rewards"),
                   "verifier_grade": grade,
                   "native_phase_endpoints": {key: native.get(key) for key in
                                              ("environment_setup", "agent_setup", "agent_execution", "verifier")},
                   "session_ids": trial.get("session_ids", []),
                   "diagnostics": (job.get("diagnostics") or []) + (trial.get("diagnostics") or [])}
            attempts.append(row)
            if receipt and isinstance(receipt.get("rep"), int) and receipt.get("task"):
                groups[(receipt["task"], receipt["rep"])].append((row, receipt, job, trial))
    pairs, pair_dispositions = [], []
    for (task, rep), members in sorted(groups.items()):
        members_a = [member for member in members if member[0]["arm"] == control]
        members_b = [member for member in members if member[0]["arm"] == treatment]
        reasons = []
        if len(members_a) != 1 or len(members_b) != 1:
            reasons.append("missing or duplicate task/repetition arm; no retry selection")
        else:
            a, b = members_a[0], members_b[0]
            reasons.extend(a[0]["comparison_exclusions"] + b[0]["comparison_exclusions"])
            for key in ("task_checksum", "oracle_sha256", "runtime", "isolation"):
                if a[1].get(key) != b[1].get(key):
                    reasons.append("paired " + key + " differs")
            if a[1].get("skills_sha256") == b[1].get("skills_sha256"):
                reasons.append("both arms expose the same package identity")
            if not b[1].get("skills_sha256"):
                reasons.append("treatment package identity missing")
            for owner in (2, 3):
                if normalized_config(document(a[owner], "config.json")) != normalized_config(document(b[owner], "config.json")):
                    reasons.append("native paired configuration differs beyond task/package/output paths")
            if not reasons:
                pairs.append({"task": task, "rep": rep,
                              "control_score": int(a[0]["outcome"] == "success"),
                              "treatment_score": int(b[0]["outcome"] == "success"),
                              "control_outcome": a[0]["outcome"], "treatment_outcome": b[0]["outcome"]})
        for row, *_ in members:
            row["comparison_exclusions"] = sorted(set(row["comparison_exclusions"] + reasons))
        pair_dispositions.append({"task": task, "rep": rep, "included": not reasons,
                                  "reasons": sorted(set(reasons))})
    # A passing within-cell match cannot silently mix treatment versions or
    # model settings across a cohort, nor task/config versions across repeats.
    cohort_identities, task_identities = set(), defaultdict(set)
    for pair in pairs:
        members = groups[(pair["task"], pair["rep"])]
        chosen = {member[0]["arm"]: member for member in members if member[0]["arm"] in (control, treatment)}
        runtime = chosen[control][1]["runtime"]
        cohort_identities.add(json.dumps({"runtime": {key: runtime[key] for key in
                                                     ("harbor_version", "codex_version", "model", "reasoning_effort")},
                                         "packages": [chosen[arm][1]["skills_sha256"] for arm in (control, treatment)]}, sort_keys=True))
        task_identities[pair["task"]].add(json.dumps({"receipt": {key: chosen[control][1][key] for key in
                                                                 ("task_checksum", "oracle_sha256", "runtime", "isolation")},
                                                     "config": normalized_config(document(chosen[control][3], "config.json"))}, sort_keys=True))
    drifted = {task for task, identities in task_identities.items() if len(identities) > 1}
    if len(cohort_identities) > 1 or drifted:
        excluded = {(pair["task"], pair["rep"]) for pair in pairs
                    if len(cohort_identities) > 1 or pair["task"] in drifted}
        reason = "comparison cohort package/model or repeated task configuration changed"
        pairs = [pair for pair in pairs if (pair["task"], pair["rep"]) not in excluded]
        for disposition in pair_dispositions:
            if (disposition["task"], disposition["rep"]) in excluded:
                disposition["included"] = False
                disposition["reasons"].append(reason)
        for key in excluded:
            for row, *_ in groups[key]:
                row["comparison_exclusions"].append(reason)
    summaries = {}
    for arm, jobs in sorted(arms.items()):
        rows = [row for row in attempts if row["arm"] == arm]
        expected = [job.get("counts", {}).get("expected") for job in jobs]
        expected_total = sum(expected) if all(isinstance(n, int) and n >= 0 for n in expected) else None
        denominator = max(expected_total, len(rows)) if expected_total is not None else None
        successes = sum(row["outcome"] == "success" for row in rows)
        costs = distribution([(row["harbor_agent_result"] or {}).get("cost_usd") for row in rows])
        summaries[arm] = {"expected": expected_total, "expected_known_subtotal": sum(n for n in expected if isinstance(n, int) and n >= 0),
                          "observed": len(rows), "started": sum(job.get("counts", {}).get("started", 0) for job in jobs),
                          "unobserved_expected": max(0, expected_total - len(rows)) if expected_total is not None else None,
                          "outcomes": dict(sorted(Counter(row["outcome"] for row in rows).items())),
                          "all_assigned_or_observed_denominator": denominator,
                          "endpoint_successes": successes,
                          "endpoint_success_rate_all_assignments": successes / denominator if denominator else None,
                          "independently_completed_outcomes": None,
                          "validation_cases": validation_cases(rows),
                          "comparison_eligible_attempts": sum(not row["comparison_exclusions"] for row in rows),
                          "harbor_cost_usd": costs, "elapsed_seconds": distribution([row["elapsed_seconds"] for row in rows]),
                          "harbor_cost_note": "Incomplete Harbor estimate; nested sessions may be omitted. Billing is unknown.",
                          "harbor_cost_per_endpoint_success": costs["known_sum"] / successes if successes and not costs["unknown"] and denominator == len(rows) else None,
                          "total_billed_cost_per_accepted_outcome": None}
    sessions = [{"id": session.get("id"), "diagnostics": session.get("diagnostics"),
                 "copies": [{"trial": copy.get("trial"), "evidence": copy.get("evidence"),
                             **{key: (copy.get("accounting") or {}).get(key) for key in
                                ("parent_id", "usage", "first_timestamp", "last_timestamp", "latest_turn_state", "diagnostics")}}
                            for copy in session.get("copies", [])]} for session in report.get("sessions", [])]
    unmeasured = ["feasibility and substantiated blocking", "worker false completion", "independent semantic acceptance",
                  "scope/acceptance drift, recovery, stopping and budget overrun", "content delivery and relevant action",
                  "total billing, orchestration and experimental grading cost"]
    if not any(summary["validation_cases"]["case_results"] for summary in summaries.values()):
        unmeasured.extend(["validator false acceptance", "needless blocking on clean candidates"])
    return {"recommendation": "insufficient-evidence",
            "recommendation_reason": "This pilot readout supports a scoped human maintenance decision; it does not establish held-out benefit, semantic acceptance, equivalence, or complete billed cost.",
            "arms": summaries, "paired_outcomes": pairs, "pair_dispositions": pair_dispositions,
            "uncertainty": uncertainty(pairs, n_required=n_required, resamples=resamples),
            "attempts": attempts, "native_sessions": sessions,
            "receipt_diagnostics": receipts.get("diagnostics", []),
            "unobserved_receipt_jobs": sorted(name for name in by_job if name not in seen_jobs),
            "unmeasured": unmeasured,
            "remaining_workflow_gaps": [{"job": row["job"], "not_checked": row["verifier_grade"]["not_checked"],
                                         "source": row["verifier_grade"]["source"]} for row in attempts],
            "limits": list(report.get("limits", [])) + [
                "All supplied attempts remain visible. Native expected counts are assignments, not a reconstructed historical start ledger. Missing jobs in receipts remain separately visible.",
                "Endpoint successes and receipt-backed comparable pairs are distinct. Isolation configuration is not proof against every contamination route; receipts must come from the external runner, never the worker.",
                "Time distributions cover recorded trial start to finish, including recorded setup/verifier work. No outer setup or analysis window is inferred; native phase endpoints are retained without fabricated allocation.",
                "Native cumulative counters are per session and evidence copy. Cached input is within input; reasoning is within output. Never sum copies, parent/child totals, or Harbor metrics with native usage.",
                "Harbor cost values and ratios are incomplete estimates, even when every attempt has a scalar: the adapter may omit nested sessions. They are separate from native usage and do not establish billing. Missing usage or billing is unknown; zero accepted outcomes makes cost per accepted outcome undefined.",
                "Infrastructure, missing and ambiguous endpoints are excluded from paired inference, retained in overall assignment accounting. Execution errors count as unsuccessful endpoints when identity is intact.",
                "Validation case metrics use only receipt-valid, separately captured verifier grades, including failed endpoints. Missing cases or grades remain unknown; recorded case judgments do not establish worker false completion or independent workflow completion.",
                "Numeric repetition IDs pair observations, not provider randomness. No live enforcement or general uplift claim follows from replay fixtures."]}


def markdown(result):
    def show(value):
        return "unknown" if value is None else str(round(value, 4)) if isinstance(value, float) else str(value)
    lines = ["Recommendation: **" + result["recommendation"] + "**.", "", result["recommendation_reason"], "",
             "| Arm | Assigned | Observed | Endpoint successes / denominator | Outcomes | Harbor estimate (incomplete) / attempts without estimate |",
             "|---|---:|---:|---|---|---|"]
    for arm, row in result["arms"].items():
        cost = row["harbor_cost_usd"]
        lines.append(f"| {arm} | {show(row['expected'])} | {row['observed']} | {row['endpoint_successes']} / {show(row['all_assigned_or_observed_denominator'])} | {json.dumps(row['outcomes'], sort_keys=True)} | ${cost['known_sum']:.4f} / {cost['unknown']} |")
    lines += ["", "Paired task outcomes (control → treatment):"]
    lines += [f"- {pair['task']} rep {pair['rep']}: {pair['control_outcome']} → {pair['treatment_outcome']}" for pair in result["paired_outcomes"]] or ["- None."]
    lines += ["", "Uncertainty: " + json.dumps(result["uncertainty"], sort_keys=True), "",
              "Attempt disposition (all observed attempts):"]
    for row in result["attempts"]:
        exclusion = "; ".join(row["comparison_exclusions"]) or "included in paired endpoint analysis"
        lines.append(f"- {row['job']} / {row['trial_id'] or row['directory']}: {row['outcome']}; {show(row['elapsed_seconds'])} seconds; {exclusion}.")
    lines += ["", "Independent validation cases (count / expected-case denominator; unknown cases):",
              "", "| Arm | False acceptance | False blocker | Justified NOT_PROVEN | Attempts without case grade |",
              "|---|---|---|---|---:|"]
    for arm, summary in result["arms"].items():
        metrics = summary["validation_cases"]
        cells = [f"{show(metrics[name]['count'])} / {show(metrics[name]['denominator'])}; {metrics[name]['unknown_cases']} unknown"
                 for name in ("false_acceptance", "false_blocker", "justified_not_proven")]
        lines.append(f"| {arm} | " + " | ".join(cells) + f" | {metrics['attempts_without_case_grade']} |")
    lines.append("")
    for summary in result["arms"].values():
        metrics = summary["validation_cases"]
        for case in metrics["case_results"]:
            lines.append(f"- {case['job']} / {case['case_id']}: expected {case['expected']}, actual {show(case['actual'])}; {case['classification']}.")
    lines += ["", "Remaining workflow gaps (verifier not_checked; absence is unknown):"]
    lines += [f"- {gap['job']}: " + ("unknown" if gap["not_checked"] is None else "; ".join(gap["not_checked"]) or "none recorded")
              for gap in result["remaining_workflow_gaps"]]
    if result["unobserved_receipt_jobs"]:
        lines += ["", "Receipt jobs without native report: " + ", ".join(result["unobserved_receipt_jobs"])]
    if result["receipt_diagnostics"]:
        lines += ["", "Receipt coverage/diagnostics: " + "; ".join(result["receipt_diagnostics"])]
    lines += ["", "Native usage by identity and copy (input / cached input / output; no sums):"]
    for session in result["native_sessions"]:
        for copy in session["copies"]:
            usage = copy.get("usage") or {}
            lines.append(f"- {session['id']} (parent {copy.get('parent_id') or 'none recorded'}), {copy.get('trial') or 'unassigned'}: " + " / ".join(show(usage.get(key)) for key in ("input_tokens", "cached_input_tokens", "output_tokens")))
    if not result["native_sessions"]:
        lines.append("- Unknown: no native sessions supplied.")
    lines += ["", "Unmeasured: " + "; ".join(result["unmeasured"]) + ".", ""]
    lines += ["- " + limit for limit in result["limits"]]
    return "\n".join(lines) + "\n"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument("--receipts", type=Path)
    parser.add_argument("--control", default="control")
    parser.add_argument("--treatment", default="treatment")
    parser.add_argument("--n-required", type=int, help="predeclared task-cluster floor; omitted for descriptive pilot")
    parser.add_argument("--resamples", type=int, default=10000)
    parser.add_argument("--format", choices=("json", "markdown"), default="markdown")
    args = parser.parse_args(argv)
    try:
        raw = args.report.read_bytes()
        receipt_raw = args.receipts.read_bytes() if args.receipts else None
        result = build(json.loads(raw), json.loads(receipt_raw) if receipt_raw else None,
                       control=args.control, treatment=args.treatment, n_required=args.n_required, resamples=args.resamples)
        result["sources"] = {"report_sha256": hashlib.sha256(raw).hexdigest(),
                             "receipts_sha256": hashlib.sha256(receipt_raw).hexdigest() if receipt_raw else None}
        print(json.dumps(result, indent=2, sort_keys=True, allow_nan=False) if args.format == "json" else markdown(result), end="\n" if args.format == "json" else "")
        return 0
    except (OSError, ValueError, KeyError, TypeError) as error:
        print("readout: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
