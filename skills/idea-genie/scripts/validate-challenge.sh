#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "usage: $0 <idea-challenge.json>" >&2
  exit 2
fi

python3 - "$1" <<'PY'
import json
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
raw = path.read_bytes()
if len(raw) > 131072:
    raise SystemExit("invalid idea-challenge.v1 artifact: output exceeds 131072 bytes")
try:
    packet = json.loads(raw)
except json.JSONDecodeError as exc:
    raise SystemExit(f"invalid idea-challenge.v1 artifact: unreadable JSON: {exc}")

base = {
    "schema_version", "authorization_id", "input_packet", "limits", "door_class",
    "sealed_generation", "round_status", "judge_attempts", "perspectives",
    "cross_reviews", "disagreements", "refutations", "handoff", "cleanup_verified",
}
allowed = base | {"requires_ntm", "lightweight_challenge"}
if not isinstance(packet, dict) or base - set(packet) or set(packet) - allowed:
    raise SystemExit("invalid idea-challenge.v1 artifact: unexpected or missing fields")

def text(value, maximum):
    return isinstance(value, str) and 0 < len(value) <= maximum

if packet["schema_version"] != "idea-challenge.v1" or not text(packet["authorization_id"], 256):
    raise SystemExit("invalid idea-challenge.v1 artifact: identity or authorization")
input_packet = packet["input_packet"]
if not isinstance(input_packet, dict) or set(input_packet) != {"source_refs", "byte_length", "sha256"}:
    raise SystemExit("invalid idea-challenge.v1 artifact: malformed input packet")
refs = input_packet["source_refs"]
if (
    not isinstance(refs, list) or not 1 <= len(refs) <= 20 or len(set(refs)) != len(refs)
    or not all(text(item, 1024) for item in refs)
    or not isinstance(input_packet["byte_length"], int) or isinstance(input_packet["byte_length"], bool)
    or not 1 <= input_packet["byte_length"] <= 262144
    or not isinstance(input_packet["sha256"], str) or not re.fullmatch(r"[a-f0-9]{64}", input_packet["sha256"])
):
    raise SystemExit("invalid idea-challenge.v1 artifact: input packet exceeds bound")
limits = packet["limits"]
expected_limits = {"judge_count", "per_judge_timeout_seconds", "round_timeout_seconds", "max_judge_output_bytes"}
if not isinstance(limits, dict) or set(limits) != expected_limits:
    raise SystemExit("invalid idea-challenge.v1 artifact: malformed limits")
for field, minimum, maximum in (
    ("judge_count", 1, 4),
    ("per_judge_timeout_seconds", 1, 300),
    ("round_timeout_seconds", 1, 900),
    ("max_judge_output_bytes", 1, 32768),
):
    value = limits[field]
    if not isinstance(value, int) or isinstance(value, bool) or not minimum <= value <= maximum:
        raise SystemExit(f"invalid idea-challenge.v1 artifact: {field} exceeds bound")
if packet["cleanup_verified"] is not True:
    raise SystemExit("invalid idea-challenge.v1 artifact: cleanup is not verified")
if packet["door_class"] not in {"one-way", "two-way"} or packet["round_status"] not in {"sufficient", "insufficient"}:
    raise SystemExit("invalid idea-challenge.v1 artifact: invalid route status")

attempts = packet["judge_attempts"]
if not isinstance(attempts, list) or len(attempts) != limits["judge_count"]:
    raise SystemExit("invalid idea-challenge.v1 artifact: judge count differs from declaration")
completed = {}
contexts = []
for attempt in attempts:
    allowed_attempt = {"context_id", "adapter", "model_identity", "status", "output_bytes", "cleanup_confirmed", "error"}
    required_attempt = allowed_attempt - {"error"}
    if not isinstance(attempt, dict) or set(attempt) - allowed_attempt or required_attempt - set(attempt):
        raise SystemExit("invalid idea-challenge.v1 artifact: malformed judge attempt")
    if not text(attempt["context_id"], 256) or not text(attempt["adapter"], 128) or not text(attempt["model_identity"], 128):
        raise SystemExit("invalid idea-challenge.v1 artifact: unbounded judge identity")
    size = attempt["output_bytes"]
    if (
        attempt["status"] not in {"completed", "timed_out", "error"}
        or attempt["cleanup_confirmed"] is not True
        or not isinstance(size, int) or isinstance(size, bool)
        or not 0 <= size <= limits["max_judge_output_bytes"]
    ):
        raise SystemExit("invalid idea-challenge.v1 artifact: invalid judge status, output, or cleanup")
    if attempt["status"] == "completed":
        if size < 1 or "error" in attempt:
            raise SystemExit("invalid idea-challenge.v1 artifact: completed judge lacks output")
        completed[attempt["context_id"]] = attempt
    elif not text(attempt.get("error"), 1024):
        raise SystemExit("invalid idea-challenge.v1 artifact: non-returning judge needs error")
    contexts.append(attempt["context_id"])
if len(set(contexts)) != len(contexts):
    raise SystemExit("invalid idea-challenge.v1 artifact: reused judge context")

perspectives = packet["perspectives"]
if not isinstance(perspectives, list) or len(perspectives) > 4:
    raise SystemExit("invalid idea-challenge.v1 artifact: perspectives are unbounded")
ids = []
perspective_contexts = []
for perspective in perspectives:
    if not isinstance(perspective, dict) or set(perspective) != {"id", "context_id", "model_identity", "proposal", "evidence"}:
        raise SystemExit("invalid idea-challenge.v1 artifact: malformed perspective")
    if (
        not text(perspective["id"], 128) or perspective["context_id"] not in completed
        or perspective["model_identity"] != completed[perspective["context_id"]]["model_identity"]
        or not text(perspective["proposal"], 8192)
        or not isinstance(perspective["evidence"], list) or not 1 <= len(perspective["evidence"]) <= 20
        or not all(text(item, 1024) for item in perspective["evidence"])
    ):
        raise SystemExit("invalid idea-challenge.v1 artifact: perspective violates declaration or bounds")
    ids.append(perspective["id"])
    perspective_contexts.append(perspective["context_id"])
if len(set(ids)) != len(ids) or len(set(perspective_contexts)) != len(perspective_contexts):
    raise SystemExit("invalid idea-challenge.v1 artifact: duplicate perspective identity")

reviews = packet["cross_reviews"]
if not isinstance(reviews, list) or len(reviews) > 20:
    raise SystemExit("invalid idea-challenge.v1 artifact: cross reviews are unbounded")
for review in reviews:
    if not isinstance(review, dict) or set(review) != {"reviewer", "subject", "dimensions"}:
        raise SystemExit("invalid idea-challenge.v1 artifact: malformed cross review")
    dimensions = review["dimensions"]
    if (
        review["reviewer"] not in ids or review["subject"] not in ids or review["reviewer"] == review["subject"]
        or not isinstance(dimensions, dict) or not 1 <= len(dimensions) <= 10
        or not all(text(key, 128) and text(value, 2048) for key, value in dimensions.items())
    ):
        raise SystemExit("invalid idea-challenge.v1 artifact: cross review violates bounds")
if not isinstance(packet["disagreements"], list) or len(packet["disagreements"]) > 20 or not all(text(item, 4096) for item in packet["disagreements"]):
    raise SystemExit("invalid idea-challenge.v1 artifact: disagreements exceed bounds")
if not isinstance(packet["refutations"], list) or len(packet["refutations"]) > 20:
    raise SystemExit("invalid idea-challenge.v1 artifact: refutations exceed bounds")
for item in packet["refutations"]:
    if not isinstance(item, dict) or set(item) != {"claim", "attempt", "result"} or not all(text(item[field], 4096) for field in ("claim", "attempt", "result")):
        raise SystemExit("invalid idea-challenge.v1 artifact: malformed refutation")
handoff = packet["handoff"]
if not isinstance(handoff, dict) or set(handoff) != {"owner", "artifact_dir", "route"} or handoff["owner"] != "plan" or not text(handoff["artifact_dir"], 1024):
    raise SystemExit("invalid idea-challenge.v1 artifact: malformed handoff")

if packet["door_class"] == "one-way":
    if limits["judge_count"] < 2 or packet["sealed_generation"] is not True or "requires_ntm" in packet or "lightweight_challenge" in packet:
        raise SystemExit("invalid idea-challenge.v1 artifact: one-way route declaration")
    if set(perspective_contexts) != set(completed):
        raise SystemExit("invalid idea-challenge.v1 artifact: completed attempts differ from perspectives")
    if packet["round_status"] == "sufficient":
        if len(perspectives) < 2 or not reviews or not packet["disagreements"] or not packet["refutations"] or handoff["route"] != "sealed-challenge":
            raise SystemExit("invalid idea-challenge.v1 artifact: sufficient one-way challenge lacks evidence")
    elif len(perspectives) >= 2 or reviews or packet["disagreements"] or packet["refutations"] or handoff["route"] != "stop-insufficient":
        raise SystemExit("invalid idea-challenge.v1 artifact: insufficient one-way challenge synthesized output")
else:
    if limits["judge_count"] != 1 or packet["sealed_generation"] is not False or packet.get("requires_ntm") is not False:
        raise SystemExit("invalid idea-challenge.v1 artifact: two-way route declaration")
    if perspectives or reviews or packet["disagreements"] or packet["refutations"]:
        raise SystemExit("invalid idea-challenge.v1 artifact: two-way route grew panel state")
    if packet["round_status"] == "sufficient":
        if len(completed) != 1 or not text(packet.get("lightweight_challenge"), 8192) or handoff["route"] != "single-fresh-context":
            raise SystemExit("invalid idea-challenge.v1 artifact: incomplete two-way challenge")
    elif completed or "lightweight_challenge" in packet or handoff["route"] != "stop-insufficient":
        raise SystemExit("invalid idea-challenge.v1 artifact: failed two-way challenge claimed output")

print(f"valid idea-challenge.v1: {path}")
PY
