#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "usage: $0 <council-report.json>" >&2
  exit 2
fi

python3 - "$1" <<'PY'
import json
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
raw = path.read_bytes()
if len(raw) > 524288:
    raise SystemExit("invalid council-report.v1 artifact: output exceeds 524288 bytes")
try:
    report = json.loads(raw)
except (OSError, json.JSONDecodeError) as exc:
    raise SystemExit(f"invalid council-report.v1 artifact: unreadable JSON: {exc}")

required = {
    "schema_version", "question", "subject_digest", "authorization_id",
    "input_packet", "limits", "round_status", "judge_attempts", "judges",
    "synthesis", "cleanup_verified",
}
allowed = required | {"diversity_unsatisfied"}
if not isinstance(report, dict) or set(report) - allowed or required - set(report):
    raise SystemExit("invalid council-report.v1 artifact: unexpected or missing fields")

def text(value, maximum):
    return isinstance(value, str) and 0 < len(value) <= maximum

if report["schema_version"] != "council-report.v1":
    raise SystemExit("invalid council-report.v1 artifact: wrong schema_version")
if not text(report["question"], 4096) or not re.fullmatch(r"[a-f0-9]{64}", report["subject_digest"]):
    raise SystemExit("invalid council-report.v1 artifact: question or subject digest")
if not text(report["authorization_id"], 256):
    raise SystemExit("invalid council-report.v1 artifact: missing bounded authorization ID")
if "diversity_unsatisfied" in report and not isinstance(report["diversity_unsatisfied"], bool):
    raise SystemExit("invalid council-report.v1 artifact: diversity_unsatisfied must be boolean")
if report["cleanup_verified"] is not True:
    raise SystemExit("invalid council-report.v1 artifact: cleanup is not verified")

packet = report["input_packet"]
if not isinstance(packet, dict) or set(packet) != {"source_refs", "byte_length", "sha256"}:
    raise SystemExit("invalid council-report.v1 artifact: malformed input packet")
refs = packet["source_refs"]
if (
    not isinstance(refs, list) or not 1 <= len(refs) <= 20
    or len(set(refs)) != len(refs) or not all(text(item, 1024) for item in refs)
    or not isinstance(packet["byte_length"], int) or isinstance(packet["byte_length"], bool)
    or not 1 <= packet["byte_length"] <= 262144
    or not isinstance(packet["sha256"], str) or not re.fullmatch(r"[a-f0-9]{64}", packet["sha256"])
):
    raise SystemExit("invalid council-report.v1 artifact: input packet exceeds declared bounds")

limits = report["limits"]
expected_limits = {"judge_count", "per_judge_timeout_seconds", "round_timeout_seconds", "max_judge_output_bytes"}
if not isinstance(limits, dict) or set(limits) != expected_limits:
    raise SystemExit("invalid council-report.v1 artifact: malformed limits")
bounds = {
    "judge_count": (2, 5),
    "per_judge_timeout_seconds": (1, 300),
    "round_timeout_seconds": (1, 900),
    "max_judge_output_bytes": (1, 32768),
}
for field, (minimum, maximum) in bounds.items():
    value = limits[field]
    if not isinstance(value, int) or isinstance(value, bool) or not minimum <= value <= maximum:
        raise SystemExit(f"invalid council-report.v1 artifact: {field} exceeds bound")

attempts = report["judge_attempts"]
if not isinstance(attempts, list) or len(attempts) != limits["judge_count"]:
    raise SystemExit("invalid council-report.v1 artifact: judge attempt count does not match declaration")
attempt_contexts = []
completed = set()
for attempt in attempts:
    allowed_attempt = {"context_id", "adapter", "model_identity", "status", "output_bytes", "cleanup_confirmed", "error"}
    required_attempt = allowed_attempt - {"error"}
    if not isinstance(attempt, dict) or set(attempt) - allowed_attempt or required_attempt - set(attempt):
        raise SystemExit("invalid council-report.v1 artifact: malformed judge attempt")
    if not text(attempt["context_id"], 256) or not text(attempt["adapter"], 128) or not text(attempt["model_identity"], 128):
        raise SystemExit("invalid council-report.v1 artifact: unbounded judge identity")
    if attempt["status"] not in {"completed", "timed_out", "error"} or attempt["cleanup_confirmed"] is not True:
        raise SystemExit("invalid council-report.v1 artifact: invalid judge status or cleanup")
    size = attempt["output_bytes"]
    if not isinstance(size, int) or isinstance(size, bool) or not 0 <= size <= limits["max_judge_output_bytes"]:
        raise SystemExit("invalid council-report.v1 artifact: judge output exceeds declared limit")
    if attempt["status"] == "completed":
        if size < 1 or "error" in attempt:
            raise SystemExit("invalid council-report.v1 artifact: completed judge lacks bounded output")
        completed.add(attempt["context_id"])
    elif not text(attempt.get("error"), 1024):
        raise SystemExit("invalid council-report.v1 artifact: non-returning judge needs an error")
    attempt_contexts.append(attempt["context_id"])
if len(set(attempt_contexts)) != len(attempt_contexts):
    raise SystemExit("invalid council-report.v1 artifact: reused judge context")

judges = report["judges"]
if not isinstance(judges, list) or len(judges) > 5:
    raise SystemExit("invalid council-report.v1 artifact: judges must be bounded")
judge_contexts = []
judge_methodologies = set()
for judge in judges:
    allowed_judge = {"context_id", "methodology", "model_identity", "judgment", "evidence", "omissions"}
    required_judge = allowed_judge - {"omissions"}
    if not isinstance(judge, dict) or set(judge) - allowed_judge or required_judge - set(judge):
        raise SystemExit("invalid council-report.v1 artifact: malformed completed judge")
    if (
        judge["context_id"] not in completed or not text(judge["methodology"], 256)
        or not text(judge["model_identity"], 128) or not text(judge["judgment"], 8192)
        or not isinstance(judge["evidence"], list) or not 1 <= len(judge["evidence"]) <= 20
        or not all(text(item, 1024) for item in judge["evidence"])
        or not isinstance(judge.get("omissions", []), list)
        or len(judge.get("omissions", [])) > 20
        or not all(text(item, 1024) for item in judge.get("omissions", []))
    ):
        raise SystemExit("invalid council-report.v1 artifact: completed judge violates bounds")
    attempt = next(item for item in attempts if item["context_id"] == judge["context_id"])
    if judge["model_identity"] != attempt["model_identity"]:
        raise SystemExit("invalid council-report.v1 artifact: judge model differs from declaration")
    judge_contexts.append(judge["context_id"])
    judge_methodologies.add(judge["methodology"])
if len(set(judge_contexts)) != len(judge_contexts) or set(judge_contexts) != completed:
    raise SystemExit("invalid council-report.v1 artifact: completed attempts and judgments differ")

synthesis = report["synthesis"]
if not isinstance(synthesis, dict) or set(synthesis) != {"consensus", "divergence", "minority", "unresolved"}:
    raise SystemExit("invalid council-report.v1 artifact: malformed synthesis")
for field in synthesis:
    if not isinstance(synthesis[field], list) or len(synthesis[field]) > 20:
        raise SystemExit("invalid council-report.v1 artifact: synthesis is unbounded")
for item in synthesis["consensus"]:
    if (
        not isinstance(item, dict) or set(item) != {"claim", "methodologies"}
        or not text(item["claim"], 4096) or not isinstance(item["methodologies"], list)
        or not 2 <= len(item["methodologies"]) <= 5
        or len(set(item["methodologies"])) != len(item["methodologies"])
        or not all(text(method, 256) for method in item["methodologies"])
        or not set(item["methodologies"]).issubset(judge_methodologies)
    ):
        raise SystemExit("invalid council-report.v1 artifact: malformed consensus")
for item in synthesis["divergence"]:
    if (
        not isinstance(item, dict) or set(item) != {"point", "positions"}
        or not text(item["point"], 4096) or not isinstance(item["positions"], list)
        or not 1 <= len(item["positions"]) <= 10
        or not all(text(position, 4096) for position in item["positions"])
    ):
        raise SystemExit("invalid council-report.v1 artifact: malformed divergence")
if not all(text(item, 4096) for item in synthesis["minority"] + synthesis["unresolved"]):
    raise SystemExit("invalid council-report.v1 artifact: malformed synthesis text")

status = report["round_status"]
if status == "sufficient":
    if len(judges) < 2:
        raise SystemExit("invalid council-report.v1 artifact: sufficient round needs two judgments")
elif status == "insufficient":
    if len(judges) >= 2 or synthesis["consensus"] or synthesis["divergence"] or synthesis["minority"]:
        raise SystemExit("invalid council-report.v1 artifact: insufficient round cannot synthesize consensus")
    if not synthesis["unresolved"]:
        raise SystemExit("invalid council-report.v1 artifact: insufficient round must disclose unresolved work")
else:
    raise SystemExit("invalid council-report.v1 artifact: invalid round_status")

print(f"valid council-report.v1: {path}")
PY
