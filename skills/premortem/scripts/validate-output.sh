#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "usage: $0 <premortem-plan-review.json>" >&2
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
    raise SystemExit("premortem plan review: output exceeds 524288 bytes")
try:
    value = json.loads(raw)
except (OSError, json.JSONDecodeError) as exc:
    print(f"premortem plan review: unreadable JSON: {exc}", file=sys.stderr)
    raise SystemExit(1)

required = {
    "schema_version", "intent_digest", "author_context_id",
    "judge_context_id", "authorization_id", "judge_adapter",
    "judge_model_identity", "judge_status", "judge_output_bytes",
    "judge_process_group_reaped", "disposable_state_clean", "input_packet",
    "limits", "findings", "checked", "not_checked",
}
if set(value) != required:
    print("premortem plan review: unexpected or missing fields", file=sys.stderr)
    raise SystemExit(1)
if value["schema_version"] != "premortem-plan-review.v1":
    raise SystemExit("premortem plan review: wrong schema_version")
if not re.fullmatch(r"[a-f0-9]{64}", value["intent_digest"]):
    raise SystemExit("premortem plan review: invalid intent digest")
author = value["author_context_id"]
judge = value["judge_context_id"]
if not isinstance(author, str) or not author or not isinstance(judge, str) or not judge or author == judge:
    raise SystemExit("premortem plan review: author and judge identities must be nonempty and distinct")
for field, maximum in (("authorization_id", 256), ("judge_adapter", 128), ("judge_model_identity", 128)):
    item = value[field]
    if not isinstance(item, str) or not item or len(item) > maximum:
        raise SystemExit(f"premortem plan review: {field} must be nonempty and bounded")
status = value["judge_status"]
if status not in {"completed", "timed_out", "error"}:
    raise SystemExit("premortem plan review: invalid judge_status")
output_bytes = value["judge_output_bytes"]
if not isinstance(output_bytes, int) or isinstance(output_bytes, bool) or not 0 <= output_bytes <= 32768:
    raise SystemExit("premortem plan review: judge_output_bytes exceeds bound")
if value["judge_process_group_reaped"] is not True or value["disposable_state_clean"] is not True:
    raise SystemExit("premortem plan review: judge cleanup and disposable state must be verified")
packet = value["input_packet"]
if not isinstance(packet, dict) or set(packet) != {"source_refs", "byte_length", "sha256"}:
    raise SystemExit("premortem plan review: malformed input_packet")
refs = packet["source_refs"]
if (
    not isinstance(refs, list) or not 1 <= len(refs) <= 20
    or len(set(refs)) != len(refs)
    or not all(isinstance(item, str) and 0 < len(item) <= 1024 for item in refs)
):
    raise SystemExit("premortem plan review: input_packet source_refs exceed bounds")
if not isinstance(packet["byte_length"], int) or isinstance(packet["byte_length"], bool) or not 1 <= packet["byte_length"] <= 262144:
    raise SystemExit("premortem plan review: input_packet byte_length exceeds bound")
if not isinstance(packet["sha256"], str) or not re.fullmatch(r"[a-f0-9]{64}", packet["sha256"]):
    raise SystemExit("premortem plan review: input_packet digest is invalid")
limits = value["limits"]
if not isinstance(limits, dict) or set(limits) != {"judge_timeout_seconds", "overall_timeout_seconds", "max_judge_output_bytes"}:
    raise SystemExit("premortem plan review: malformed limits")
for field, maximum in (("judge_timeout_seconds", 300), ("overall_timeout_seconds", 600), ("max_judge_output_bytes", 32768)):
    item = limits[field]
    if not isinstance(item, int) or isinstance(item, bool) or not 1 <= item <= maximum:
        raise SystemExit(f"premortem plan review: {field} exceeds bound")
if output_bytes > limits["max_judge_output_bytes"]:
    raise SystemExit("premortem plan review: judge output exceeds declared limit")
for field in ("checked", "not_checked"):
    if (
        not isinstance(value[field], list) or len(value[field]) > 50
        or not all(isinstance(item, str) and 0 < len(item) <= 1024 for item in value[field])
    ):
        raise SystemExit(f"premortem plan review: {field} must be a string array")
if not isinstance(value["findings"], list) or len(value["findings"]) > 50:
    raise SystemExit("premortem plan review: findings must be an array")
for finding in value["findings"]:
    if not isinstance(finding, dict) or set(finding) != {"id", "statement", "evidence"}:
        raise SystemExit("premortem plan review: malformed finding")
    if (
        not isinstance(finding["id"], str) or not 0 < len(finding["id"]) <= 128
        or not isinstance(finding["statement"], str) or not 0 < len(finding["statement"]) <= 4096
    ):
        raise SystemExit("premortem plan review: finding id and statement are required")
    evidence = finding["evidence"]
    if (
        not isinstance(evidence, list) or not 1 <= len(evidence) <= 20
        or not all(isinstance(item, str) and 0 < len(item) <= 1024 for item in evidence)
    ):
        raise SystemExit("premortem plan review: each finding needs evidence")
if status == "completed":
    if output_bytes < 1 or not value["checked"]:
        raise SystemExit("premortem plan review: completed judge needs output and checked scope")
else:
    if value["findings"] or value["checked"] or not value["not_checked"]:
        raise SystemExit("premortem plan review: incomplete judge cannot synthesize findings")
print("premortem plan review: valid")
PY
