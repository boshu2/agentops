#!/usr/bin/env bash
set -euo pipefail

json=false
allow_dirty=false
root=""

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/check-closeout-gate.sh [--json] [--allow-dirty] [--root <repo>]

Reports RPI execution-packet proof refs and worktree clean/dirty state.
Fails when proof refs are absent, or when the worktree is dirty unless
--allow-dirty is provided.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --json)
      json=true
      shift
      ;;
    --allow-dirty)
      allow_dirty=true
      shift
      ;;
    --root)
      if [[ $# -lt 2 ]]; then
        usage
        exit 2
      fi
      root="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$root" ]]; then
  root="$(git rev-parse --show-toplevel 2>/dev/null || pwd -P)"
fi
root="$(cd "$root" && pwd -P)"

dirty_status=""
git_error=""
if git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  dirty_status="$(git -C "$root" status --porcelain --untracked-files=all 2>/dev/null || true)"
else
  git_error="not a git worktree"
fi

export CLOSEOUT_ROOT="$root"
export CLOSEOUT_DIRTY_STATUS="$dirty_status"
export CLOSEOUT_GIT_ERROR="$git_error"
export CLOSEOUT_JSON="$json"
export CLOSEOUT_ALLOW_DIRTY="$allow_dirty"

python3 - <<'PY'
import glob
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

root = Path(os.environ["CLOSEOUT_ROOT"])
dirty_status = os.environ.get("CLOSEOUT_DIRTY_STATUS", "")
git_error = os.environ.get("CLOSEOUT_GIT_ERROR", "")
json_mode = os.environ.get("CLOSEOUT_JSON") == "true"
allow_dirty = os.environ.get("CLOSEOUT_ALLOW_DIRTY") == "true"

packet_paths = [root / ".agents" / "rpi" / "execution-packet.json"]
packet_paths.extend(Path(p) for p in glob.glob(str(root / ".agents" / "rpi" / "runs" / "*" / "execution-packet.json")))

proof_refs = []
provenance_refs = []
provenance_warnings = []
parse_errors = []
for packet_path in sorted(set(packet_paths)):
    if not packet_path.exists():
        continue
    try:
        data = json.loads(packet_path.read_text())
    except Exception as exc:
        parse_errors.append({"packet": str(packet_path.relative_to(root)), "error": str(exc)})
        continue
    packet_rel = str(packet_path.relative_to(root))
    issue_ids = []
    for key in ("bead_id", "epic_id"):
        value = str(data.get(key) or "").strip()
        if value and value not in issue_ids:
            issue_ids.append(value)
    raw_issue_ids = data.get("issue_ids") or []
    if isinstance(raw_issue_ids, list):
        for item in raw_issue_ids:
            value = str(item).strip()
            if value and value not in issue_ids:
                issue_ids.append(value)
    ref = {
        "packet": packet_rel,
        "run_id": data.get("run_id") or packet_path.parent.name,
        "epic_id": data.get("epic_id", ""),
        "bead_id": data.get("bead_id", ""),
        "issue_ids": issue_ids,
        "tracking_repo_root": data.get("tracking_repo_root", ""),
        "beads_dir": data.get("beads_dir", ""),
        "pr_url": data.get("pr_url", ""),
        "merge_commit": data.get("merge_commit", ""),
        "warnings": [],
    }
    beads_dir = str(ref["beads_dir"]).strip()
    if beads_dir and not Path(beads_dir).exists():
        ref["warnings"].append(f"beads_dir missing: {beads_dir}")
    bd = shutil.which("bd")
    if bd and issue_ids:
        for issue_id in issue_ids:
            probe = subprocess.run(
                [bd, "show", issue_id, "--json"],
                cwd=str(root),
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                check=False,
            )
            if probe.returncode != 0:
                ref["warnings"].append(f"bead not resolved in active DB: {issue_id}")
    if any(ref.get(key) for key in ("epic_id", "bead_id", "tracking_repo_root", "beads_dir", "pr_url", "merge_commit")) or issue_ids:
        provenance_refs.append(ref)
        provenance_warnings.extend(f"{packet_rel}: {warning}" for warning in ref["warnings"])
    artifacts = data.get("proof_artifacts") or []
    if not isinstance(artifacts, list) or not artifacts:
        continue
    run_id = data.get("run_id") or packet_path.parent.name
    proof_refs.append({
        "packet": str(packet_path.relative_to(root)),
        "run_id": run_id,
        "proof_updated_at": data.get("proof_updated_at", ""),
        "artifacts": [str(item) for item in artifacts],
    })

dirty_paths = [line for line in dirty_status.splitlines() if line.strip()]
errors = []
if not proof_refs:
    errors.append("no RPI execution-packet proof_refs found")
if parse_errors:
    errors.append("execution-packet parse errors")
if git_error:
    errors.append(f"worktree disposition unavailable: {git_error}")
elif dirty_paths and not allow_dirty:
    errors.append("worktree dirty")

report = {
    "schema_version": 1,
    "root": str(root),
    "result": "FAIL" if errors else "PASS",
    "closure_replay": {
        "proof_ref_count": len(proof_refs),
        "proof_refs": proof_refs,
        "parse_errors": parse_errors,
    },
    "provenance": {
        "packet_count": len(provenance_refs),
        "refs": provenance_refs,
        "warnings": provenance_warnings,
    },
    "worktree": {
        "clean": not dirty_paths and not git_error,
        "allow_dirty": allow_dirty,
        "dirty_count": len(dirty_paths),
        "dirty_paths": dirty_paths,
        "error": git_error,
    },
    "errors": errors,
}

if json_mode:
    print(json.dumps(report, indent=2, sort_keys=True))
else:
    print("closeout gate")
    print(f"result: {report['result']}")
    print(f"proof_refs: {len(proof_refs)}")
    for ref in proof_refs:
        print(f"- {ref['packet']} run={ref['run_id']} artifacts={len(ref['artifacts'])}")
    if parse_errors:
        print(f"parse_errors: {len(parse_errors)}")
    if provenance_refs:
        print(f"provenance_refs: {len(provenance_refs)}")
        for ref in provenance_refs:
            label = ref.get("epic_id") or ref.get("bead_id") or ",".join(ref.get("issue_ids", [])[:3])
            print(f"- {ref['packet']} issue={label or 'n/a'} pr={ref.get('pr_url') or 'n/a'}")
    if provenance_warnings:
        print("provenance warnings:")
        for warning in provenance_warnings[:20]:
            print(f"- {warning}")
    if git_error:
        print(f"worktree: unavailable ({git_error})")
    elif dirty_paths:
        print(f"worktree: dirty ({len(dirty_paths)} paths)")
        for path in dirty_paths[:20]:
            print(f"- {path}")
    else:
        print("worktree: clean")
    if errors:
        print("errors:")
        for error in errors:
            print(f"- {error}")

sys.exit(1 if errors else 0)
PY
