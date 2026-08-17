#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "usage: $0 <idea-portfolio.json>" >&2
  exit 2
fi

python3 - "$1" <<'PY'
import hashlib
import json
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
raw = path.read_bytes()
if len(raw) > 65536:
    raise SystemExit("invalid idea-portfolio.v1 artifact: output exceeds 65536 bytes")
try:
    value = json.loads(raw)
except json.JSONDecodeError as exc:
    raise SystemExit(f"invalid idea-portfolio.v1 artifact: unreadable JSON: {exc}")

required = {
    "schema_version", "authorization_id", "input_packet", "limits", "effects",
    "status", "observations", "assumptions", "candidates", "termination",
}
if not isinstance(value, dict) or set(value) != required:
    raise SystemExit("invalid idea-portfolio.v1 artifact: unexpected or missing fields")

def text(item, maximum):
    return isinstance(item, str) and 0 < len(item) <= maximum

if value["schema_version"] != "idea-portfolio.v1" or not text(value["authorization_id"], 256):
    raise SystemExit("invalid idea-portfolio.v1 artifact: identity or authorization")
packet = value["input_packet"]
if not isinstance(packet, dict) or set(packet) != {"source_refs", "byte_length", "sha256"}:
    raise SystemExit("invalid idea-portfolio.v1 artifact: malformed input packet")
refs = packet["source_refs"]
if (
    not isinstance(refs, list) or not 1 <= len(refs) <= 20 or len(set(refs)) != len(refs)
    or not all(text(item, 1024) for item in refs)
    or not isinstance(packet["byte_length"], int) or isinstance(packet["byte_length"], bool)
    or not 1 <= packet["byte_length"] <= 262144
    or not isinstance(packet["sha256"], str) or not re.fullmatch(r"[a-f0-9]{64}", packet["sha256"])
):
    raise SystemExit("invalid idea-portfolio.v1 artifact: input packet exceeds bound")
limits = value["limits"]
if not isinstance(limits, dict) or set(limits) != {"max_passes", "max_candidates", "max_output_bytes"}:
    raise SystemExit("invalid idea-portfolio.v1 artifact: malformed limits")
for field, maximum in (("max_passes", 3), ("max_candidates", 10), ("max_output_bytes", 65536)):
    item = limits[field]
    if not isinstance(item, int) or isinstance(item, bool) or not 1 <= item <= maximum:
        raise SystemExit(f"invalid idea-portfolio.v1 artifact: {field} exceeds bound")
if len(raw) > limits["max_output_bytes"]:
    raise SystemExit("invalid idea-portfolio.v1 artifact: output exceeds declared bound")
effects = value["effects"]
if not isinstance(effects, dict) or set(effects) != {"network_requests", "bytes_fetched", "credentials_used", "writes"}:
    raise SystemExit("invalid idea-portfolio.v1 artifact: malformed effects")
if (
    not isinstance(effects["network_requests"], int) or isinstance(effects["network_requests"], bool)
    or not 0 <= effects["network_requests"] <= 50
    or not isinstance(effects["bytes_fetched"], int) or isinstance(effects["bytes_fetched"], bool)
    or not 0 <= effects["bytes_fetched"] <= 1048576
    or not isinstance(effects["credentials_used"], list) or len(effects["credentials_used"]) > 10
    or not all(text(item, 128) for item in effects["credentials_used"])
    or not isinstance(effects["writes"], list) or len(effects["writes"]) > 1
    or not all(text(item, 1024) for item in effects["writes"])
):
    raise SystemExit("invalid idea-portfolio.v1 artifact: effects exceed bounds")
if value["status"] not in {"candidates", "no-new-work"}:
    raise SystemExit("invalid idea-portfolio.v1 artifact: invalid status")
observations = value["observations"]
if not isinstance(observations, list) or not 1 <= len(observations) <= 50:
    raise SystemExit("invalid idea-portfolio.v1 artifact: observations must be bounded")
for item in observations:
    if not isinstance(item, dict) or set(item) != {"claim", "evidence"} or not text(item["claim"], 4096) or not text(item["evidence"], 1024):
        raise SystemExit("invalid idea-portfolio.v1 artifact: malformed observation")
if not isinstance(value["assumptions"], list) or len(value["assumptions"]) > 20 or not all(text(item, 1024) for item in value["assumptions"]):
    raise SystemExit("invalid idea-portfolio.v1 artifact: assumptions exceed bounds")
candidates = value["candidates"]
if not isinstance(candidates, list) or len(candidates) > limits["max_candidates"]:
    raise SystemExit("invalid idea-portfolio.v1 artifact: candidates exceed bound")
candidate_ids = []
for candidate in candidates:
    if not isinstance(candidate, dict) or set(candidate) != {"id", "evidence", "overlaps", "scenario"}:
        raise SystemExit("invalid idea-portfolio.v1 artifact: malformed candidate")
    if (
        not text(candidate["id"], 128)
        or not isinstance(candidate["evidence"], list) or not 1 <= len(candidate["evidence"]) <= 20
        or not all(text(item, 1024) for item in candidate["evidence"])
        or not isinstance(candidate["overlaps"], list) or len(candidate["overlaps"]) > 20
        or not all(text(item, 1024) for item in candidate["overlaps"])
        or not isinstance(candidate["scenario"], dict)
        or set(candidate["scenario"]) != {"given", "when", "then"}
        or not all(text(candidate["scenario"][field], 2048) for field in ("given", "when", "then"))
    ):
        raise SystemExit("invalid idea-portfolio.v1 artifact: candidate exceeds bounds")
    candidate_ids.append(candidate["id"])
if len(set(candidate_ids)) != len(candidate_ids):
    raise SystemExit("invalid idea-portfolio.v1 artifact: duplicate candidate ID")
termination = value["termination"]
if not isinstance(termination, dict) or set(termination) != {"reason", "passes_run", "novel_candidates_last_pass"}:
    raise SystemExit("invalid idea-portfolio.v1 artifact: malformed termination")
if (
    not isinstance(termination["passes_run"], int) or isinstance(termination["passes_run"], bool)
    or not 1 <= termination["passes_run"] <= limits["max_passes"]
    or termination["novel_candidates_last_pass"] != 0
):
    raise SystemExit("invalid idea-portfolio.v1 artifact: novelty loop did not stop within bounds")
if value["status"] == "candidates":
    if termination["reason"] != "novelty-saturated" or not candidates:
        raise SystemExit("invalid idea-portfolio.v1 artifact: candidate portfolio did not saturate")
elif termination["reason"] != "all-overlap-or-unsupported" or candidates:
    raise SystemExit("invalid idea-portfolio.v1 artifact: no-new-work portfolio has candidates")

print(f"valid idea-portfolio.v1: {path}")
PY
