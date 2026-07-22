#!/usr/bin/env python3
"""One-shot fail-closed wrapper for the native GC hook claim protocol."""
from __future__ import annotations

import json
import os
import subprocess
import sys


def fail(code: str, message: str) -> int:
    print(json.dumps({"schema_version": "agentops-claim.v1", "ok": False, "action": "uncertain", "reason": code, "message": message}, sort_keys=True))
    return 1


def main() -> int:
    gc = os.environ.get("GC_BIN", "")
    if not gc.startswith("/") or not os.access(gc, os.X_OK):
        return fail("invalid_gc_bin", "GC_BIN must be an absolute executable path")
    result = subprocess.run([gc, "hook", "--claim", "--drain-ack", "--json"], text=True, capture_output=True, check=False)
    if result.returncode != 0:
        return fail("hook_nonzero", result.stderr.strip() or "hook claim failed")
    try:
        value = json.loads(result.stdout)
    except json.JSONDecodeError:
        return fail("hook_invalid_json", "hook claim did not emit one JSON object")
    required = {"schema_version", "ok", "command", "action", "reason"}
    optional = {"bead_id", "assignee", "route", "root_bead_id", "continuation_group", "continuation_assigned", "drain_acknowledged"}
    if not isinstance(value, dict) or not required.issubset(value) or set(value) - required - optional or value.get("schema_version") != "1" or value.get("ok") is not True or value.get("command") != "hook" or not isinstance(value.get("action"), str) or not isinstance(value.get("reason"), str):
        return fail("hook_schema", "hook result is not the official hook schema")
    if any(not isinstance(value[key], str) for key in {"bead_id", "assignee", "route", "root_bead_id", "continuation_group"} & set(value)) or ("continuation_assigned" in value and (not isinstance(value["continuation_assigned"], list) or any(not isinstance(item, str) for item in value["continuation_assigned"]))) or ("drain_acknowledged" in value and not isinstance(value["drain_acknowledged"], bool)):
        return fail("hook_schema", "hook optional fields have invalid types")
    if value["action"] == "drain" and value["reason"] == "no_work" and value.get("drain_acknowledged") is True and "bead_id" not in value and "assignee" not in value:
        output = {"schema_version": "agentops-claim.v1", "ok": True, "action": "drain", "reason": "no_work"}
    elif value["action"] == "work" and value["reason"] in {"claimed", "existing_assignment", "ready_assignment"} and isinstance(value.get("bead_id"), str) and value["bead_id"] and isinstance(value.get("assignee"), str) and value["assignee"]:
        output = {"schema_version": "agentops-claim.v1", "ok": True, "action": "assigned", "reason": value["reason"], "bead_id": value["bead_id"], "assignee": value["assignee"]}
    else:
        return fail("hook_ambiguous", "hook result is not an exact assigned or no-work result")
    print(json.dumps(output, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
