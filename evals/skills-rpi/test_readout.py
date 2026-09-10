"""Synthetic native-record replays; no live models, private sources or launches."""
from copy import deepcopy
import importlib.util
from pathlib import Path

import pytest

spec = importlib.util.spec_from_file_location("readout", Path(__file__).with_name("readout.py"))
readout = importlib.util.module_from_spec(spec)
spec.loader.exec_module(readout)


def evidence(data, sha="a" * 64):
    return {"sha256": sha, "data": data}


def fixture(outcomes=("failed", "success"), task="repair", rep=1):
    jobs, receipts = [], []
    for arm, outcome in zip(("control", "treatment"), outcomes):
        name = f"{task}-{arm}-{rep}"
        agent = {"name": "codex", "model_name": "openai/test-model",
                 "skills": ["/protected/package"] if arm == "treatment" else [],
                 "kwargs": {"version": "0.test", "reasoning_effort": "high"}}
        config = {"job_name": name, "jobs_dir": "/protected/jobs", "agents": [deepcopy(agent)],
                  "tasks": [{"path": "/protected/" + task}]}
        native = {"verifier_environment_mode": "separate", "agent_result": {"cost_usd": 0.25},
                  "agent_info": {"version": "0.test", "model_info": {"provider": "openai", "name": "test-model"}}}
        trial = {"directory": "/protected/" + name + "/trial", "id": name, "task_name": task,
                 "task_checksum": "b" * 64, "outcome": outcome, "started": True, "completed": True,
                 "timing": {"elapsed_seconds": 10}, "session_ids": [], "diagnostics": [],
                 "documents": {"result.json": evidence(native), "config.json": evidence({"agent": agent, "task": {"path": "/protected/" + task}})}}
        job = {"directory": "/protected/" + name, "arm": arm, "counts": {"expected": 1, "started": 1},
               "trials": [trial], "diagnostics": [], "documents": {"config.json": evidence(config)}}
        receipt = {"job_name": name, "task": task, "rep": rep, "arm": arm,
                   "configuration_sha256": "a" * 64, "task_checksum": "b" * 64, "oracle_sha256": "c" * 64,
                   "skills_sha256": "d" * 64 if arm == "treatment" else None,
                   "runtime": {"harbor_version": "0.test", "codex_version": "0.test", "model": "openai/test-model",
                               "reasoning_effort": "high", "worker_image_id": "sha256:worker", "verifier_image_id": "sha256:verifier"},
                   "isolation": {"separate_verifier": True, "private_inputs_excluded": True, "network_policy": "provider-only"}}
        jobs.append(job)
        receipts.append(receipt)
    return {"jobs": jobs, "sessions": [], "limits": []}, {"schema_version": 1, "trials": receipts}


def test_pairs_keep_repetitions_clustered_and_replay_exactly():
    report, receipts = fixture()
    for task, rep, outcomes in (("repair", 2, ("success", "failed")),
                                ("scope", 1, ("success", "success")), ("stop", 1, ("success", "failed"))):
        more, assignments = fixture(outcomes, task, rep)
        report["jobs"] += more["jobs"]
        receipts["trials"] += assignments["trials"]
    result = readout.build(report, receipts, n_required=10, resamples=500)
    assert len(result["paired_outcomes"]) == 4
    assert result["uncertainty"]["task_clusters"] == 3
    assert result["uncertainty"]["signal"] == "underpowered"
    assert result["uncertainty"]["delta_treatment_minus_control"] == pytest.approx(-1 / 3)
    assert result["uncertainty"]["status"] == "available"
    assert result["uncertainty"]["ci_low"] is not None
    assert result["uncertainty"]["confidence"] == 0.95
    assert result == readout.build(report, receipts, n_required=10, resamples=500)
    assert result["recommendation"] == "insufficient-evidence"


@pytest.mark.parametrize("mutation,reason", [
    (lambda r, x: x["trials"][1].update(configuration_sha256="f" * 64), "configuration hash"),
    (lambda r, x: x["trials"][1].update(task_checksum="f" * 64), "task checksum"),
    (lambda r, x: x["trials"][1].update(oracle_sha256="f" * 64), "oracle_sha256 differs"),
    (lambda r, x: x["trials"][1]["runtime"].update(reasoning_effort="low"), "reasoning effort"),
    (lambda r, x: x["trials"][1]["runtime"].update(worker_image_id="changed"), "runtime differs"),
    (lambda r, x: x["trials"][1]["isolation"].update(private_inputs_excluded=False), "isolation"),
    (lambda r, x: x["trials"][1].update(contamination_detected=True), "contamination"),
    (lambda r, x: x["trials"][0].update(skills_sha256="d" * 64), "same package"),
    (lambda r, x: r["jobs"][1]["documents"]["config.json"]["data"].update(n_concurrent_trials=3), "configuration differs"),
    (lambda r, x: r["jobs"][1]["trials"][0]["documents"]["result.json"]["data"]["agent_info"].update(version="wrong"), "executed model/version"),
    (lambda r, x: r["jobs"][1]["trials"][0]["documents"]["config.json"]["data"]["agent"].update(model_name="wrong"), "effective native agent"),
])
def test_bad_identity_or_contamination_never_becomes_comparison_proof(mutation, reason):
    report, receipts = fixture(("success", "success"))
    mutation(report, receipts)
    result = readout.build(report, receipts)
    assert result["paired_outcomes"] == []
    assert reason in str(result["pair_dispositions"])
    assert result["arms"]["treatment"]["endpoint_successes"] == 1
    assert result["arms"]["treatment"]["comparison_eligible_attempts"] == 0


def test_all_attempts_missing_expected_and_infrastructure_stay_visible():
    report, receipts = fixture(("infrastructure_error", "unfinished"))
    report["jobs"][0]["counts"]["expected"] = 3
    report["jobs"][1]["counts"]["expected"] = None
    report["jobs"][1]["trials"][0]["completed"] = False
    report["jobs"][1]["trials"][0]["timing"] = {}
    report["jobs"][1]["trials"][0]["documents"]["result.json"]["data"]["agent_result"] = None
    receipts["trials"].append({"job_name": "unobserved", "arm": "control", "task": "repair", "rep": 2})
    result = readout.build(report, receipts)
    assert len(result["attempts"]) == 2
    assert result["paired_outcomes"] == []
    assert result["arms"]["control"]["all_assigned_or_observed_denominator"] == 3
    assert result["arms"]["control"]["unobserved_expected"] == 2
    assert result["arms"]["treatment"]["endpoint_success_rate_all_assignments"] is None
    assert result["arms"]["treatment"]["elapsed_seconds"]["unknown"] == 1
    assert result["arms"]["treatment"]["harbor_cost_usd"]["unknown"] == 1
    assert result["unobserved_receipt_jobs"] == ["unobserved"]
    assert result["arms"]["control"]["harbor_cost_per_endpoint_success"] is None


def test_duplicate_retry_cells_and_unpaired_cells_are_not_selected():
    report, receipts = fixture()
    duplicate = deepcopy(report["jobs"][1])
    duplicate["directory"] += "-retry"
    assignment = deepcopy(receipts["trials"][1])
    assignment["job_name"] += "-retry"
    report["jobs"].append(duplicate)
    receipts["trials"].append(assignment)
    result = readout.build(report, receipts)
    assert not result["paired_outcomes"]
    assert len(result["attempts"]) == 3
    assert all(row["comparison_exclusions"] for row in result["attempts"])
    report["jobs"] = report["jobs"][:1]
    result = readout.build(report, receipts)
    assert "missing or duplicate" in str(result["pair_dispositions"])


def test_native_session_copies_and_parent_child_counters_are_not_added():
    report, receipts = fixture()
    for identity, parent, usage in (("author", "", {"input_tokens": 100, "cached_input_tokens": 20, "output_tokens": 50}),
                                    ("reviewer", "author", {"input_tokens": 80, "output_tokens": 10})):
        trial = report["jobs"][1]["trials"][0]
        trial["session_ids"].append(identity)
        copy = {"trial": trial["directory"], "evidence": {"path": "/protected/" + identity, "sha256": "a" * 64},
                "accounting": {"parent_id": parent, "usage": usage, "diagnostics": []}}
        report["sessions"].append({"id": identity, "copies": [copy, deepcopy(copy)], "diagnostics": []})
    result = readout.build(report, receipts)
    assert len(result["native_sessions"]) == 2
    assert result["native_sessions"][0]["copies"][1]["usage"]["input_tokens"] == 100
    assert result["native_sessions"][1]["copies"][0]["parent_id"] == "author"
    assert result["arms"]["treatment"]["harbor_cost_usd"]["known_sum"] == 0.25
    assert "total_tokens" not in result
    assert len(result["paired_outcomes"]) == 1
    report["sessions"][0]["copies"][1]["trial"] = report["jobs"][0]["trials"][0]["directory"]
    result = readout.build(report, receipts)
    assert not result["paired_outcomes"]
    assert "session identity shared" in str(result["attempts"])


def test_no_change_and_degenerate_are_not_equivalence():
    report, receipts = fixture(("success", "success"))
    result = readout.build(report, receipts, n_required=1)
    assert result["uncertainty"]["signal"] == "inconclusive_degenerate"
    assert result["uncertainty"]["status"] == "insufficient_clusters"
    assert result["uncertainty"]["ci_low"] is None
    assert result["uncertainty"]["ci_high"] is None
    assert result["uncertainty"]["confidence"] is None
    assert result["recommendation"] == "insufficient-evidence"
    pairs = [{"task": str(n), "rep": 1, "control_score": int(n % 2 == 0), "treatment_score": int(n % 2 != 0)} for n in range(4)]
    uncertain = readout.uncertainty(pairs, n_required=1, resamples=500)
    assert uncertain["signal"] == "no_change"
    assert "not equivalence" in uncertain["interpretation"]


@pytest.mark.parametrize("scores,expected_delta", [((1, 1), 0.0), ((0, 1), 1.0)])
def test_multiple_clusters_with_constant_deltas_have_no_inferential_interval(scores, expected_delta):
    pairs = [{"task": str(n), "rep": rep, "control_score": scores[0], "treatment_score": scores[1]}
             for n in range(3) for rep in (1, 2)]
    result = readout.uncertainty(pairs, n_required=3)
    assert result["status"] == "degenerate_no_interval"
    assert result["delta_treatment_minus_control"] == expected_delta
    assert result["task_clusters"] == 3
    assert result["paired_repetitions"] == 6
    assert all(result[key] is None for key in ("ci_low", "ci_high", "confidence"))


def test_harbor_estimates_remain_incomplete_even_with_scalars_for_every_attempt():
    report, receipts = fixture(("success", "success"))
    result = readout.build(report, receipts)
    arm = result["arms"]["treatment"]
    assert arm["harbor_cost_usd"]["known_sum"] == 0.25
    assert arm["harbor_cost_usd"]["unknown"] == 0
    assert "Incomplete Harbor estimate" in arm["harbor_cost_note"]
    assert "nested sessions" in arm["harbor_cost_note"]
    assert arm["total_billed_cost_per_accepted_outcome"] is None
    rendered = readout.markdown(result)
    assert "Harbor estimate (incomplete) / attempts without estimate" in rendered
    assert "Harbor known cost" not in rendered
    assert "do not establish billing" in rendered


def graded_fixture():
    report, receipts = fixture(("failed", "failed"))
    cases = [
        {"case_id": "defect", "expected": "FAIL", "actual": "PASS", "classification": "false_acceptance"},
        {"case_id": "clean", "expected": "PASS", "actual": "NOT_PROVEN", "classification": "false_blocker"},
        {"case_id": "incomplete", "expected": "NOT_PROVEN", "actual": "NOT_PROVEN", "classification": "justified_not_proven"},
    ]
    for job in report["jobs"]:
        grade = evidence({"case_results": deepcopy(cases), "not_checked": ["fresh native handoff not exercised"]})
        grade["path"] = job["trials"][0]["directory"] + "/verifier/grade.json"
        job["trials"][0]["documents"]["verifier/grade.json"] = grade
    return report, receipts


def test_failed_endpoint_preserves_independent_case_metrics_and_workflow_gaps():
    report, receipts = graded_fixture()
    result = readout.build(report, receipts)
    metrics = result["arms"]["control"]["validation_cases"]
    assert metrics["false_acceptance"]["count"] == 1
    assert metrics["false_acceptance"]["denominator"] == 2
    assert metrics["false_acceptance"]["rate"] == 0.5
    assert metrics["false_blocker"]["count"] == 1
    assert metrics["false_blocker"]["denominator"] == 1
    assert metrics["justified_not_proven"]["count"] == 1
    assert metrics["justified_not_proven"]["denominator"] == 1
    assert len(metrics["case_results"]) == 3
    assert result["arms"]["control"]["endpoint_successes"] == 0
    assert result["arms"]["control"]["independently_completed_outcomes"] is None
    assert result["remaining_workflow_gaps"][0]["not_checked"] == ["fresh native handoff not exercised"]
    assert "validator false acceptance" not in result["unmeasured"]
    assert "worker false completion" in result["unmeasured"]
    rendered = readout.markdown(result)
    assert "expected FAIL, actual PASS; false_acceptance" in rendered
    assert "fresh native handoff not exercised" in rendered


def test_missing_case_is_unknown_with_known_expected_denominator():
    report, receipts = graded_fixture()
    case = report["jobs"][0]["trials"][0]["documents"]["verifier/grade.json"]["data"]["case_results"][2]
    case.update(actual=None, classification="missing")
    metrics = readout.build(report, receipts)["arms"]["control"]["validation_cases"]
    assert metrics["false_acceptance"]["count"] is None
    assert metrics["false_acceptance"]["known_count"] == 1
    assert metrics["false_acceptance"]["denominator"] == 2
    assert metrics["false_acceptance"]["unknown_cases"] == 1
    assert metrics["false_acceptance"]["rate"] is None
    assert metrics["false_blocker"]["count"] == 1


@pytest.mark.parametrize("invalid", ["missing", "worker_only", "no_cases", "wrong_receipt", "no_evidence_hash"])
def test_missing_or_untrusted_grade_never_manufactures_zero_case_errors(invalid):
    report, receipts = graded_fixture()
    trial = report["jobs"][0]["trials"][0]
    if invalid in ("missing", "worker_only"):
        grade = trial["documents"].pop("verifier/grade.json")
        if invalid == "worker_only":
            trial["documents"]["result.json"]["data"].update(grade["data"])
    elif invalid == "no_cases":
        trial["documents"]["verifier/grade.json"]["data"].pop("case_results")
    elif invalid == "wrong_receipt":
        receipts["trials"][0]["task_checksum"] = "f" * 64
    else:
        trial["documents"]["verifier/grade.json"]["sha256"] = None
    metrics = readout.build(report, receipts)["arms"]["control"]["validation_cases"]
    assert metrics["case_results"] == []
    assert metrics["attempts_without_case_grade"] == 1
    assert metrics["false_acceptance"]["count"] is None
    assert metrics["false_acceptance"]["denominator"] is None
    assert metrics["false_acceptance"]["rate"] is None


def test_no_receipt_keeps_observations_and_missing_usage_unknown():
    report, _ = fixture()
    result = readout.build(report)
    assert not result["paired_outcomes"]
    assert len(result["attempts"]) == 2
    assert result["arms"]["treatment"]["independently_completed_outcomes"] is None
    rendered = readout.markdown(result)
    assert "missing external launch receipt" in rendered
    assert "Unknown: no native sessions supplied" in rendered
    assert "insufficient-evidence" in rendered


def test_configuration_comparison_does_not_mutate_input():
    report, receipts = fixture()
    original = deepcopy(report)
    readout.build(report, receipts)
    assert report == original


def test_within_pair_matching_does_not_hide_changed_treatment_between_repetitions():
    report, receipts = fixture()
    later, later_receipts = fixture(rep=2)
    later_receipts["trials"][1]["skills_sha256"] = "e" * 64
    report["jobs"] += later["jobs"]
    receipts["trials"] += later_receipts["trials"]
    result = readout.build(report, receipts)
    assert not result["paired_outcomes"]
    assert "cohort package/model" in str(result["pair_dispositions"])
    assert result["arms"]["treatment"]["observed"] == 2


def test_receipt_coverage_and_malformed_native_document_are_not_lost():
    report, receipts = fixture()
    receipts["diagnostics"] = ["not observed: future-repetition"]
    report["jobs"][1]["trials"][0]["documents"]["result.json"]["data"] = []
    result = readout.build(report, receipts)
    assert not result["paired_outcomes"]
    assert "not observed: future-repetition" in readout.markdown(result)


@pytest.mark.parametrize("kwargs", [{"control": "same", "treatment": "same"}, {"n_required": 0}, {"resamples": 0}])
def test_invalid_analysis_settings_fail(kwargs):
    report, receipts = fixture()
    with pytest.raises(ValueError):
        readout.build(report, receipts, **kwargs)
