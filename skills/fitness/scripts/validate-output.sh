#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" || -L "$1" ]]; then
  echo "usage: $0 <fitness-command-receipt.json>" >&2
  exit 2
fi

python3 - "$1" <<'PY'
import json
import os
from pathlib import Path, PurePath
import re
import sys

path = Path(sys.argv[1])
raw = path.read_bytes()
if len(raw) > 131072:
    raise SystemExit("fitness output: receipt exceeds 131072 bytes")
try:
    value = json.loads(raw)
except json.JSONDecodeError as exc:
    raise SystemExit(f"fitness output: unreadable JSON: {exc}")
required = {"schema_version", "subcommand", "authorization_id", "goals_source", "goals_digest_before", "goals_digest_after", "reads", "writes", "stdout", "render_root", "render_target", "target_existed_before", "overwrite_authorization_id", "status"}
if not isinstance(value, dict) or set(value) != required:
    raise SystemExit("fitness output: unexpected or missing fields")
def text(item, maximum=1024):
    return isinstance(item, str) and 0 < len(item) <= maximum
commands = {"measure", "validate", "drift", "history", "export", "meta", "scenarios", "render"}
if value["schema_version"] != "fitness-command-receipt.v1" or value["subcommand"] not in commands or not text(value["authorization_id"], 256) or not text(value["goals_source"]):
    raise SystemExit("fitness output: invalid identity, command, or authorization")
for field in ("goals_digest_before", "goals_digest_after"):
    if not isinstance(value[field], str) or not re.fullmatch(r"[a-f0-9]{64}", value[field]):
        raise SystemExit("fitness output: invalid goals digest")
reads = value["reads"]
writes = value["writes"]
if not isinstance(reads, list) or not 1 <= len(reads) <= 200 or len(set(reads)) != len(reads) or not all(text(item) for item in reads) or value["goals_source"] not in reads:
    raise SystemExit("fitness output: reads do not account for the goals source")
if not isinstance(writes, list) or len(writes) > 2 or len(set(writes)) != len(writes) or not all(text(item) for item in writes):
    raise SystemExit("fitness output: writes are malformed or unbounded")
if not isinstance(value["stdout"], bool) or value["status"] not in {"complete", "failed"}:
    raise SystemExit("fitness output: invalid stdout or status")
if value["status"] == "complete" and value["goals_digest_before"] != value["goals_digest_after"]:
    raise SystemExit("fitness output: successful command mutated the goals source")

subcommand = value["subcommand"]
render_fields = (value["render_root"], value["render_target"], value["target_existed_before"], value["overwrite_authorization_id"])
if subcommand != "render" and any(item is not None for item in render_fields):
    raise SystemExit("fitness output: non-render command declared render effects")

def fixed_baseline(candidate):
    parts = PurePath(candidate).parts
    marker = (".agents", "ao", "goals", "baselines")
    return any(tuple(parts[index:index + 4]) == marker for index in range(max(0, len(parts) - 3))) and ".." not in parts

if value["status"] == "complete" and subcommand in {"validate", "history", "meta", "scenarios"} and writes:
    raise SystemExit("fitness output: read-only command reported writes")
if value["status"] == "complete" and subcommand in {"measure", "drift", "export"}:
    if len(writes) != 1 or not fixed_baseline(writes[0]):
        raise SystemExit("fitness output: snapshot command wrote outside its fixed baseline root")
if subcommand == "export" and value["status"] == "complete" and value["stdout"] is not True:
    raise SystemExit("fitness output: export omitted its stdout effect")
if subcommand == "render" and value["status"] == "complete":
    render_root, render_target, existed_before, overwrite_id = render_fields
    if render_target is None:
        if render_root is not None or existed_before is not None or overwrite_id is not None or writes or value["stdout"] is not True:
            raise SystemExit("fitness output: stdout render declared file effects")
    else:
        if not text(render_root) or not text(render_target) or not isinstance(existed_before, bool) or len(writes) != 1 or writes[0] != render_target:
            raise SystemExit("fitness output: malformed render target effects")
        root = Path(render_root).resolve()
        target = Path(render_target).resolve()
        goals = Path(value["goals_source"]).resolve()
        if root == Path(root.anchor) or root == Path.home().resolve() or target == goals:
            raise SystemExit("fitness output: forbidden render target")
        try:
            target.relative_to(root)
        except ValueError:
            raise SystemExit("fitness output: render target escapes declared derived root")
        if existed_before and not text(overwrite_id, 256):
            raise SystemExit("fitness output: existing render target lacks overwrite approval")
        if not existed_before and overwrite_id is not None:
            raise SystemExit("fitness output: receipt claims unnecessary overwrite approval")
print(f"valid fitness-command-receipt.v1: {path}")
PY
