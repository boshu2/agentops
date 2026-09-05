#!/usr/bin/env bash
# evidence-orphans.sh — evidence-binding orphan receipt (NOT a gate).
#
# WHY: a probe scorecard, a fixture-set, and a capture contract all bind exact
# files by path AND sha256 digest — the evaluator harness and its helpers
# (docs/evals/scorecards/**/*.json's `evaluator` / `capture_evaluator` blocks;
# evals/skill-probes/**/fixture-set.json's `capture_evaluator` block, see
# scripts/lib/probe-fixture-metadata.py's `verify_evaluator_files`), and the
# canonical skill snapshot the capture was taken against
# (`canonical_skill` on fixture-set.json and capture-contract.json). Editing
# any of those bound files silently orphans every artifact that recorded the
# OLD digest — nobody learns until a later, unrelated `ao probe` run trips the
# published/live hash check. This prints the exposure up front, at the moment
# the change is made, so a caller can decide whether that orphaning is intended
# (a new probe generation) or an accident.
#
# WHAT is scanned, and what binds what:
#   docs/evals/scorecards/**/*.json      `evaluator`, `capture_evaluator`
#   evals/skill-probes/**/fixture-set.json      `capture_evaluator`, `canonical_skill`
#   evals/skill-probes/**/capture-contract.json `canonical_skill`
# An evaluator block maps a category name (harness, preamble, metadata_helper,
# dispatch_helper, network_proxy, ...) to `{"path": <repo-relative path>,
# "sha256": "sha256:<hex>"}`. A `canonical_skill` block binds one skill's
# `skills/<slug>/SKILL.md` the same way.
#
# A binding is ORPHANED when any of these hold, and the entry's `cause` says
# which:
#   changed_path   the bound path is in the caller-supplied <changed path> list
#   digest_drift   the file's CURRENT digest differs from the recorded one, or
#                  the bound file is missing or is not a regular file (a
#                  binding whose target is gone can never match again)
#   both           changed_path and digest_drift together
#   skill_changed  a `canonical_skill` binding whose skills/<slug>/SKILL.md no
#                  longer matches, and that was NOT in the changed list
#                  (comparison reused from scripts/lib/probe-fixture-metadata.py,
#                  not reimplemented). A skill binding takes the same three-way
#                  cause as an evaluator binding: changed_path when it is only
#                  listed, skill_changed when only the digest moved, both when
#                  each holds.
#
# OUTPUT (JSON, the only format):
#   {"changed": [...],
#    "orphaned": [{"artifact":..,"binds":..,"cause":..,
#                  "recorded_sha256":..,"current_sha256":..}],
#    "binding_count": N,   # orphaned BINDINGS
#    "artifact_count": M}  # distinct artifacts carrying them
# The two counts are reported separately: one harness edit orphans many
# bindings across few artifacts, and a single number hid which was which.
#
# This is a RECEIPT, not a gate: it exits 0 once it has run to completion,
# whether or not it found orphans. A caller (a workflow, a human) decides what
# an orphan finding means. It is not registered in the gate registry.
#
# Usage:
#   bash scripts/evidence-orphans.sh <changed path>...
#
# Env:
#   EVIDENCE_ORPHANS_ROOT   repo root to scan (test seam; default REPO_ROOT)
#
# Exit codes:
#   0 - ran to completion (orphans, if any, are IN the JSON, not the exit code)
#   2 - fail-closed error: malformed JSON, a malformed binding shape, a walk
#       error, misuse, or any unexpected exit from the scanner
#
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

require_cmd python3

ROOT="${EVIDENCE_ORPHANS_ROOT:-$REPO_ROOT}"
HELPER="$REPO_ROOT/scripts/lib/probe-fixture-metadata.py"
CHANGED=()

while [ $# -gt 0 ]; do
  case "$1" in
    -h | --help)
      sed -n '2,60p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    --) shift; while [ $# -gt 0 ]; do CHANGED+=("$1"); shift; done ;;
    -*)
      printf 'evidence-orphans: unknown option %s (JSON is the only output format)\n' "$1" >&2
      exit 2
      ;;
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
output="$(python3 - "$ROOT" "$changed_list" "$HELPER" <<'PY'
import hashlib
import importlib.util
import json
import os
import sys
from pathlib import Path

repo_root = sys.argv[1]
with open(sys.argv[2], "r", encoding="utf-8") as _cl:
    changed = [line.rstrip("\n") for line in _cl if line.strip()]
changed_set = set(changed)
helper_path = sys.argv[3]


def fail_closed(message: str):
    print(message, file=sys.stderr)
    sys.exit(2)


# The canonical-skill comparison is IMPORTED, never reimplemented: a second
# copy of "does this snapshot still match" is a second thing to drift.
def load_helper():
    spec = importlib.util.spec_from_file_location("probe_fixture_metadata", helper_path)
    if spec is None or spec.loader is None:
        fail_closed(f"cannot load the fixture-metadata helper at {helper_path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


HELPER = load_helper()


def sha256_of(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(65536), b""):
            digest.update(chunk)
    return "sha256:" + digest.hexdigest()


def walk(root_dir: str):
    # A walk error is a fail-closed error: an unreadable directory is a scan
    # that did not happen, and reporting "no orphans" for it would be a lie.
    def onerror(exc):
        fail_closed(f"cannot walk {root_dir}: {exc}")

    if not os.path.isdir(root_dir):
        return
    yield from os.walk(root_dir, onerror=onerror)


def artifacts():
    """(path, evaluator-block keys, scan_canonical_skill) per scanned artifact."""
    scorecards_root = os.path.join(repo_root, "docs", "evals", "scorecards")
    for dirpath, _dirnames, filenames in walk(scorecards_root):
        for name in sorted(filenames):
            if name.endswith(".json"):
                yield os.path.join(dirpath, name), ("evaluator", "capture_evaluator"), False

    probes_root = os.path.join(repo_root, "evals", "skill-probes")
    for dirpath, _dirnames, filenames in walk(probes_root):
        if "fixture-set.json" in filenames:
            yield os.path.join(dirpath, "fixture-set.json"), ("capture_evaluator",), True
        if "capture-contract.json" in filenames:
            # Capture contracts carry NO evaluator binding in this repository
            # (checked against every instance), but they DO bind a canonical
            # skill snapshot.
            yield os.path.join(dirpath, "capture-contract.json"), (), True


def bound_file_state(bound_path: str):
    absolute = os.path.join(repo_root, bound_path)
    # A missing or non-regular bound file can never match its recorded digest
    # again: it is orphaned, not "unknown".
    if os.path.islink(absolute) or not os.path.isfile(absolute):
        return None
    return sha256_of(absolute)


orphaned = []
seen = set()
for artifact_path, keys, scan_skill in artifacts():
    rel_artifact = os.path.relpath(artifact_path, repo_root)
    try:
        with open(artifact_path, "r", encoding="utf-8") as fh:
            data = json.load(fh)
    except (OSError, json.JSONDecodeError) as exc:
        fail_closed(f"malformed JSON in {rel_artifact}: {exc}")

    if not isinstance(data, dict):
        fail_closed(f"malformed artifact {rel_artifact}: top level is not an object")

    for key in keys:
        if key not in data or data[key] is None:
            # An explicit null is a declared absence of binding, not a shape
            # this scanner failed to read.
            continue
        block = data[key]
        # A malformed binding SHAPE is fail-closed: a shape this scanner cannot
        # read is a binding it cannot certify, and silently skipping it reports
        # "no orphans" for evidence nobody checked.
        if not isinstance(block, dict):
            fail_closed(f"malformed {key} block in {rel_artifact}: not an object")
        for category, entry in block.items():
            if not isinstance(entry, dict):
                fail_closed(f"malformed {key}.{category} in {rel_artifact}: not an object")
            bound_path = entry.get("path")
            recorded_sha256 = entry.get("sha256")
            if not isinstance(bound_path, str) or not bound_path:
                fail_closed(f"malformed {key}.{category} in {rel_artifact}: path is not a string")
            if not isinstance(recorded_sha256, str) or not recorded_sha256:
                fail_closed(f"malformed {key}.{category} in {rel_artifact}: sha256 is not a string")

            dedupe_key = (rel_artifact, bound_path)
            if dedupe_key in seen:
                continue

            current_sha256 = bound_file_state(bound_path)
            is_changed = bound_path in changed_set
            drifted = current_sha256 != recorded_sha256

            if not (is_changed or drifted):
                continue
            seen.add(dedupe_key)
            if is_changed and drifted:
                cause = "both"
            elif is_changed:
                cause = "changed_path"
            else:
                cause = "digest_drift"
            orphaned.append(
                {
                    "artifact": rel_artifact,
                    "binds": bound_path,
                    "cause": cause,
                    "recorded_sha256": recorded_sha256,
                    "current_sha256": current_sha256,
                }
            )

    if not scan_skill or data.get("canonical_skill") is None:
        continue
    record = data["canonical_skill"]
    if not isinstance(record, dict):
        fail_closed(f"malformed canonical_skill in {rel_artifact}: not an object")
    skill = record.get("name")
    bound_path = record.get("path")
    recorded_sha256 = record.get("sha256")
    if not isinstance(skill, str) or not skill:
        fail_closed(f"malformed canonical_skill in {rel_artifact}: name is not a string")
    if not isinstance(bound_path, str) or not bound_path:
        fail_closed(f"malformed canonical_skill in {rel_artifact}: path is not a string")
    if not isinstance(recorded_sha256, str) or not recorded_sha256:
        fail_closed(f"malformed canonical_skill in {rel_artifact}: sha256 is not a string")

    dedupe_key = (rel_artifact, bound_path)
    if dedupe_key in seen:
        continue
    try:
        skill_path = HELPER.canonical_skill_path(Path(repo_root) / "skills", skill)
        current_sha256 = HELPER.digest_file(skill_path)
    except HELPER.MetadataError:
        # The snapshot's skill is gone, unsafe, or no longer a regular file.
        current_sha256 = None
    # Same three-way cause as an evaluator binding: a snapshot that is merely
    # LISTED among the changed paths but still matches is not a rewritten skill,
    # and calling it one hides which of the two actually happened.
    is_changed = bound_path in changed_set
    drifted = current_sha256 != recorded_sha256
    if is_changed or drifted:
        seen.add(dedupe_key)
        if is_changed and drifted:
            cause = "both"
        elif is_changed:
            cause = "changed_path"
        else:
            cause = "skill_changed"
        orphaned.append(
            {
                "artifact": rel_artifact,
                "binds": bound_path,
                "cause": cause,
                "recorded_sha256": recorded_sha256,
                "current_sha256": current_sha256,
            }
        )

result = {
    "changed": changed,
    "orphaned": orphaned,
    "binding_count": len(orphaned),
    "artifact_count": len({item["artifact"] for item in orphaned}),
}
print(json.dumps(result, indent=2, sort_keys=True))
sys.exit(0)
PY
)"
rc=$?
set -e

# Fail closed on ANY nonzero exit, not only the one the scanner names: a crash
# that printed a partial receipt must never read as a completed scan.
if [ "$rc" -ne 0 ]; then
  printf '%s\n' "$output" >&2
  printf 'evidence-orphans: FAIL — the scan did not run to completion (exit %s)\n' "$rc" >&2
  exit 2
fi

printf '%s\n' "$output"
exit 0
