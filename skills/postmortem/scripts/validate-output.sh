#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 || ! -f "$1" || ! -f "$2" ]]; then
  echo "usage: $0 <postmortem-report.md> <postmortem-run.json>" >&2
  exit 2
fi

python3 - "$1" "$2" <<'PY'
import json
import re
import sys
from pathlib import Path

report_path = Path(sys.argv[1])
receipt_path = Path(sys.argv[2])
report = report_path.read_text(encoding="utf-8")
if len(report.encode("utf-8")) > 131072:
    raise SystemExit("postmortem output: report exceeds 131072 bytes")
required_headings = (
    "Causal Question", "Pinned Inputs", "Timeline", "Supported Claims",
    "Rejected Claims", "Counterfactuals", "Unknowns", "Experiments",
)
for heading in required_headings:
    if not re.search(rf"^#+\s+{re.escape(heading)}\s*$", report, re.MULTILINE | re.IGNORECASE):
        raise SystemExit(f"postmortem output: report is missing {heading}")
try:
    receipt_raw = receipt_path.read_bytes()
    if len(receipt_raw) > 131072:
        raise SystemExit("postmortem output: receipt exceeds 131072 bytes")
    receipt = json.loads(receipt_raw)
except json.JSONDecodeError as exc:
    raise SystemExit(f"postmortem output: unreadable receipt: {exc}")
required = {
    "schema_version", "authorization_id", "input_packet", "limits",
    "judge_attempts", "cleanup_verified", "subject_unchanged", "status",
}
if not isinstance(receipt, dict) or set(receipt) != required:
    raise SystemExit("postmortem output: unexpected or missing receipt fields")
def text(value, maximum):
    return isinstance(value, str) and 0 < len(value) <= maximum
if receipt["schema_version"] != "postmortem-run.v1" or not text(receipt["authorization_id"], 256):
    raise SystemExit("postmortem output: invalid identity or authorization")
packet = receipt["input_packet"]
if not isinstance(packet, dict) or set(packet) != {"source_refs", "byte_length", "sha256"}:
    raise SystemExit("postmortem output: malformed packet")
refs = packet["source_refs"]
if (
    not isinstance(refs, list) or not 1 <= len(refs) <= 20 or len(set(refs)) != len(refs)
    or not all(text(item, 1024) for item in refs)
    or not isinstance(packet["byte_length"], int) or isinstance(packet["byte_length"], bool)
    or not 1 <= packet["byte_length"] <= 262144
    or not isinstance(packet["sha256"], str) or not re.fullmatch(r"[a-f0-9]{64}", packet["sha256"])
):
    raise SystemExit("postmortem output: packet exceeds bounds")
limits = receipt["limits"]
if not isinstance(limits, dict) or set(limits) != {"max_judges", "per_judge_timeout_seconds", "overall_timeout_seconds", "max_judge_output_bytes"}:
    raise SystemExit("postmortem output: malformed limits")
for field, minimum, maximum in (
    ("max_judges", 0, 3),
    ("per_judge_timeout_seconds", 1, 300),
    ("overall_timeout_seconds", 1, 1200),
    ("max_judge_output_bytes", 1, 32768),
):
    value = limits[field]
    if not isinstance(value, int) or isinstance(value, bool) or not minimum <= value <= maximum:
        raise SystemExit(f"postmortem output: {field} exceeds bound")
attempts = receipt["judge_attempts"]
if not isinstance(attempts, list) or len(attempts) > limits["max_judges"]:
    raise SystemExit("postmortem output: judge attempts exceed declaration")
contexts = []
for attempt in attempts:
    allowed = {"context_id", "adapter", "model_identity", "status", "output_bytes", "cleanup_confirmed", "error"}
    required_attempt = allowed - {"error"}
    if not isinstance(attempt, dict) or set(attempt) - allowed or required_attempt - set(attempt):
        raise SystemExit("postmortem output: malformed judge attempt")
    if not text(attempt["context_id"], 256) or not text(attempt["adapter"], 128) or not text(attempt["model_identity"], 128):
        raise SystemExit("postmortem output: invalid judge identity")
    size = attempt["output_bytes"]
    if (
        attempt["status"] not in {"completed", "timed_out", "error"}
        or attempt["cleanup_confirmed"] is not True
        or not isinstance(size, int) or isinstance(size, bool)
        or not 0 <= size <= limits["max_judge_output_bytes"]
    ):
        raise SystemExit("postmortem output: invalid judge status, output, or cleanup")
    if attempt["status"] == "completed" and (size < 1 or "error" in attempt):
        raise SystemExit("postmortem output: completed judge lacks bounded output")
    if attempt["status"] != "completed" and not text(attempt.get("error"), 1024):
        raise SystemExit("postmortem output: non-returning judge needs an error")
    contexts.append(attempt["context_id"])
if len(set(contexts)) != len(contexts):
    raise SystemExit("postmortem output: reused judge context")
if receipt["cleanup_verified"] is not True or receipt["subject_unchanged"] is not True:
    raise SystemExit("postmortem output: cleanup or subject restoration is unverified")
if receipt["status"] not in {"complete", "incomplete"}:
    raise SystemExit("postmortem output: invalid status")
if receipt["status"] == "complete" and any(item["status"] != "completed" for item in attempts):
    raise SystemExit("postmortem output: complete run contains a non-returning judge")
if receipt["status"] == "incomplete" and not re.search(r"^#+\s+Incomplete\s*$", report, re.MULTILINE | re.IGNORECASE):
    raise SystemExit("postmortem output: incomplete run is not disclosed in report")
print(f"valid postmortem output: {report_path} + {receipt_path}")
PY
