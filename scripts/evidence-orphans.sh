#!/usr/bin/env bash
# evidence-orphans.sh — evidence-binding orphan receipt (NOT a gate).
#
# WHY: a probe scorecard and a fixture-set both bind an exact evaluator by
# path AND sha256 digest (docs/evals/scorecards/**/*.json's `evaluator` /
# `capture_evaluator` blocks; evals/skill-probes/**/fixture-set.json's
# `capture_evaluator` block — see scripts/lib/probe-fixture-metadata.py's
# `verify_evaluator_files`). Editing one of those bound files (the harness,
# the preamble, the metadata helper, the dispatch helper, the network proxy)
# silently orphans every scorecard and fixture-set that recorded the OLD
# digest — nobody learns until a later, unrelated `ao probe` run trips the
# published/live evaluator-hash check. This prints the exposure up front, at
# the moment the harness change is made, so a caller can decide whether that
# orphaning is intended (a new probe generation) or an accident.
#
# WHAT: walk every `docs/evals/scorecards/**/*.json` (reading its `evaluator`
# and `capture_evaluator` blocks) and every `evals/skill-probes/**/fixture-set.json`
# (reading its `capture_evaluator` block — capture-contract.json files in this
# tree carry no evaluator binding at all; confirmed against every existing
# instance, so they are not scanned). Each block maps a category name (harness,
# preamble, metadata_helper, dispatch_helper, network_proxy, ...) to
# `{"path": <repo-relative path>, "sha256": "sha256:<hex>"}`. An artifact is
# ORPHANED by a binding when either:
#   * the bound path appears in the caller-supplied <changed path> list, or
#   * the CURRENT sha256 of the file on disk already differs from the
#     recorded one (independent of what the caller passed) — this catches
#     drift the caller's changed-path list did not name.
#
# OUTPUT (--json, default):
#   {"changed": [...], "orphaned": [{"artifact":..,"binds":..,
#    "recorded_sha256":..,"current_sha256":..}], "count": N}
# --text prints one line per orphan plus a trailing count, or an OK line.
#
# This is a RECEIPT, not a gate: it always exits 0 once it has run to
# completion, whether or not it found orphans. A caller (a workflow, a human)
# decides what an orphan finding means. It is not registered in the gate
# registry.
#
# Usage:
#   bash scripts/evidence-orphans.sh [--json|--text] <changed path>...
#
# Env:
#   EVIDENCE_ORPHANS_ROOT   repo root to scan (test seam; default REPO_ROOT)
#
# Exit codes:
#   0 - ran to completion (orphans, if any, are IN the JSON/text, not the exit code)
#   2 - fail-closed error (malformed JSON in a scanned artifact, misuse)
#
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

require_cmd python3

ROOT="${EVIDENCE_ORPHANS_ROOT:-$REPO_ROOT}"
FORMAT="json"
CHANGED=()

while [ $# -gt 0 ]; do
  case "$1" in
    --json) FORMAT="json" ;;
    --text) FORMAT="text" ;;
    -h | --help)
      sed -n '2,40p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    --) shift; while [ $# -gt 0 ]; do CHANGED+=("$1"); shift; done ;;
    *) CHANGED+=("$1") ;;
  esac
  shift
done

work=""
with_tmpdir work evidence-orphans
changed_list="$work/changed.txt"
: > "$changed_list"
for p in ${CHANGED[@]+"${CHANGED[@]}"}; do
  printf '%s\n' "$p" >> "$changed_list"
done

set +e
output="$(python3 - "$ROOT" "$FORMAT" "$changed_list" <<'PY'
import hashlib
import json
import os
import sys

repo_root = sys.argv[1]
out_format = sys.argv[2]
with open(sys.argv[3], "r", encoding="utf-8") as _cl:
    changed = [line.rstrip("\n") for line in _cl if line.strip()]
changed_set = set(changed)


def sha256_of(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(65536), b""):
            digest.update(chunk)
    return "sha256:" + digest.hexdigest()


def walk_json_files(root_dir: str):
    if not os.path.isdir(root_dir):
        return
    for dirpath, _dirnames, filenames in os.walk(root_dir):
        for name in sorted(filenames):
            if name.endswith(".json"):
                yield os.path.join(dirpath, name)


def artifacts():
    scorecards_root = os.path.join(repo_root, "docs", "evals", "scorecards")
    for path in walk_json_files(scorecards_root):
        yield path, ("evaluator", "capture_evaluator")

    probes_root = os.path.join(repo_root, "evals", "skill-probes")
    if os.path.isdir(probes_root):
        for dirpath, _dirnames, filenames in os.walk(probes_root):
            if "fixture-set.json" in filenames:
                yield os.path.join(dirpath, "fixture-set.json"), ("capture_evaluator",)


orphaned = []
seen = set()
for artifact_path, keys in artifacts():
    rel_artifact = os.path.relpath(artifact_path, repo_root)
    try:
        with open(artifact_path, "r", encoding="utf-8") as fh:
            data = json.load(fh)
    except (OSError, json.JSONDecodeError) as exc:
        print(f"malformed JSON in {rel_artifact}: {exc}", file=sys.stderr)
        sys.exit(2)

    if not isinstance(data, dict):
        continue

    for key in keys:
        block = data.get(key)
        if not isinstance(block, dict):
            continue
        for _category, entry in block.items():
            if not isinstance(entry, dict):
                continue
            bound_path = entry.get("path")
            recorded_sha256 = entry.get("sha256")
            if not isinstance(bound_path, str) or not isinstance(recorded_sha256, str):
                continue

            dedupe_key = (rel_artifact, bound_path)
            if dedupe_key in seen:
                continue

            abs_bound = os.path.join(repo_root, bound_path)
            if os.path.isfile(abs_bound):
                current_sha256 = sha256_of(abs_bound)
            else:
                current_sha256 = None

            is_changed = bound_path in changed_set
            digest_drifted = current_sha256 is not None and current_sha256 != recorded_sha256

            if is_changed or digest_drifted:
                seen.add(dedupe_key)
                orphaned.append(
                    {
                        "artifact": rel_artifact,
                        "binds": bound_path,
                        "recorded_sha256": recorded_sha256,
                        "current_sha256": current_sha256,
                    }
                )

result = {"changed": changed, "orphaned": orphaned, "count": len(orphaned)}

if out_format == "text":
    if not orphaned:
        print(f"OK: no evidence binding orphaned by {len(changed)} changed path(s)")
    else:
        for item in orphaned:
            print(
                f"{item['artifact']}: binds {item['binds']} "
                f"(recorded {item['recorded_sha256']}, current {item['current_sha256']})"
            )
        print(f"count={len(orphaned)}")
else:
    print(json.dumps(result, indent=2, sort_keys=True))

sys.exit(0)
PY
)"
rc=$?
set -e

if [ "$rc" -eq 2 ]; then
  printf '%s\n' "$output" >&2
  exit 2
fi

printf '%s\n' "$output"
exit 0
