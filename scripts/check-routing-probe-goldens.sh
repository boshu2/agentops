#!/usr/bin/env bash
# check-routing-probe-goldens.sh — deterministic retrieval-eval grader for
# evals/routing-probes/goldens/.
#
# WHY: evals/routing-probes/ measures P(right skill loaded | applicable task).
# Its only runs so far were three 2026-08-05 in-session subagent batches — a
# live-model method that cannot run in CI and produced n=3 with one contaminated
# row. The catalog can therefore drift (a renamed skill, a reworded description,
# a new skill that outranks the right one) with nothing that notices. This gate
# grades the OFFLINE, DETERMINISTIC half of routing: `ao skills find`, the
# repo's own token-overlap discovery surface, against hand-authored goldens in
# schemas/pack-quality-expectations.v1.schema.json shape.
#
# HONESTY — what this does and does not measure:
#   * It grades the DETERMINISTIC surface (`ao skills find`), not what a live
#     agent loads. A green run says the catalog still ranks the declared skill
#     for the declared phrasing; it does NOT say a model would route there.
#     The live half stays with the subagent batches in evals/routing-probes/.
#   * The batch-1 fixture-isolation rule (committed scenario strings leak into
#     repo-searching AGENTS) does not bind here: the grader is not an agent and
#     `ao skills find` never reads this directory, so committed literal queries
#     are safe. Do NOT reuse a golden's query as a live-dispatch prompt.
#   * ZERO goldens is a FAILURE, not a pass. A grader with an empty denominator
#     reports green forever and measures nothing.
#
# WHAT it checks, per golden, over the top_k pack `ao skills find` returns:
#   MISS       an id in expected_selected_ids is absent from the pack
#   LEAK       an id in critical_omitted_ids is present in the pack
#   MISROUTE   a forbidden_leaks regex matches the rank-1 id
#   PROVENANCE fewer than min_provenance_density of the pack cite a path that exists
#   TOKENS     the pack's descriptions exceed max_tokens whitespace tokens
#   SCHEMA     the golden itself does not satisfy the v1 contract
#
# Usage:
#   bash scripts/check-routing-probe-goldens.sh            # grade + human table
#   bash scripts/check-routing-probe-goldens.sh --json     # machine-readable
#   bash scripts/check-routing-probe-goldens.sh --goldens DIR --schema FILE
#
# Env seams (tests):
#   ROUTING_GOLDENS_DIR     goldens dir (default: $REPO_ROOT/evals/routing-probes/goldens)
#   ROUTING_GOLDENS_SCHEMA  contract (default: $REPO_ROOT/schemas/pack-quality-expectations.v1.schema.json)
#   ROUTING_GOLDENS_AO_BIN  prebuilt ao binary (default: built from $REPO_ROOT/cli)
#
# Exit: 0 every golden passes, 1 any finding / zero goldens / unusable contract,
#       2 misuse.
#
# practices: [llm-eval-harness, measurement-over-assertion, continuous-integration]
# shellcheck source=scripts/lib/preamble.sh disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

GOLDENS_DIR="${ROUTING_GOLDENS_DIR:-$REPO_ROOT/evals/routing-probes/goldens}"
SCHEMA_FILE="${ROUTING_GOLDENS_SCHEMA:-$REPO_ROOT/schemas/pack-quality-expectations.v1.schema.json}"
AO_BIN="${ROUTING_GOLDENS_AO_BIN:-}"
JSON=0

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --goldens)
            shift
            [[ $# -gt 0 ]] || { echo "--goldens requires a value" >&2; exit 2; }
            GOLDENS_DIR="$1"
            ;;
        --goldens=*) GOLDENS_DIR="${1#--goldens=}" ;;
        --schema)
            shift
            [[ $# -gt 0 ]] || { echo "--schema requires a value" >&2; exit 2; }
            SCHEMA_FILE="$1"
            ;;
        --schema=*) SCHEMA_FILE="${1#--schema=}" ;;
        --ao-bin)
            shift
            [[ $# -gt 0 ]] || { echo "--ao-bin requires a value" >&2; exit 2; }
            AO_BIN="$1"
            ;;
        --ao-bin=*) AO_BIN="${1#--ao-bin=}" ;;
        --json) JSON=1 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "Unknown flag: $1" >&2; exit 2 ;;
    esac
    shift
done

require_cmd python3

# --- fail-closed denominator guards -------------------------------------------
# A grader that reports green over zero fixtures, or that cannot read the
# contract it grades against, is measuring nothing. Both are exit 1, never a
# silent pass (the zero-denominator lesson: 0/0 must never read as 100%).
if [[ ! -d "$GOLDENS_DIR" ]]; then
    echo "FAIL: goldens directory not found: $GOLDENS_DIR" >&2
    echo "      Author at least one pack-quality-expectations.v1 fixture there." >&2
    exit 1
fi

golden_count=0
while IFS= read -r _golden; do
    golden_count=$((golden_count + 1))
done < <(portable_find "$GOLDENS_DIR" -maxdepth 1 -type f -name '*.json' 2>/dev/null)

if [[ "$golden_count" -eq 0 ]]; then
    echo "FAIL: no goldens in $GOLDENS_DIR — zero fixtures is zero measurement, not a pass." >&2
    exit 1
fi

if [[ ! -f "$SCHEMA_FILE" ]] || ! python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$SCHEMA_FILE" >/dev/null 2>&1; then
    echo "FAIL: retrieval-eval contract missing or not valid JSON: $SCHEMA_FILE" >&2
    exit 1
fi

# --- resolve the deterministic routing surface --------------------------------
# `ao skills find` resolves skills/ by walking up from CWD, so the grader always
# runs it from the repo root it is grading.
if [[ -z "$AO_BIN" ]]; then
    require_cmd go
    # Pre-declared so shellcheck can see the assignment: with_tmpdir assigns it
    # indirectly via `printf -v`, which SC2154 cannot follow.
    ao_build=""
    with_tmpdir ao_build routing-goldens
    AO_BIN="$ao_build/ao"
    if ! (cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao); then
        echo "FAIL: could not build the ao routing surface from $REPO_ROOT/cli" >&2
        exit 1
    fi
fi
if [[ ! -x "$AO_BIN" ]]; then
    echo "FAIL: routing surface binary is not executable: $AO_BIN" >&2
    exit 1
fi

python3 - "$SCHEMA_FILE" "$GOLDENS_DIR" "$AO_BIN" "$REPO_ROOT" "$JSON" <<'PY'
"""Validate each golden against the v1 contract, then grade its observed pack.

Kept in one place deliberately: the contract check and the grade are the same
judgment ("does this fixture, and the pack it declares expectations over, hold
up"), and splitting them would let a malformed golden be graded anyway.
"""
import json
import os
import re
import subprocess
import sys

schema_path, goldens_dir, ao_bin, repo_root, json_mode = sys.argv[1:6]
json_mode = json_mode == "1"

with open(schema_path, "r", encoding="utf-8") as handle:
    schema = json.load(handle)


def type_name(value):
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "boolean"
    if isinstance(value, int):
        return "integer"
    if isinstance(value, float):
        return "number"
    if isinstance(value, str):
        return "string"
    if isinstance(value, list):
        return "array"
    if isinstance(value, dict):
        return "object"
    return type(value).__name__


def matches_type(expected, value):
    if expected == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    if expected == "number":
        return isinstance(value, (int, float)) and not isinstance(value, bool)
    if expected == "boolean":
        return isinstance(value, bool)
    if expected == "string":
        return isinstance(value, str)
    if expected == "array":
        return isinstance(value, list)
    if expected == "object":
        return isinstance(value, dict)
    if expected == "null":
        return value is None
    return True


def validate(node, value, path, errors):
    expected = node.get("type")
    if expected is not None and not matches_type(expected, value):
        errors.append("%s: expected %s, got %s" % (path, expected, type_name(value)))
        return
    if "const" in node and value != node["const"]:
        errors.append("%s: expected const %r, got %r" % (path, node["const"], value))
    if "enum" in node and value not in node["enum"]:
        errors.append("%s: %r not in enum %r" % (path, value, node["enum"]))
    if isinstance(value, str):
        if "minLength" in node and len(value) < node["minLength"]:
            errors.append("%s: shorter than minLength %d" % (path, node["minLength"]))
        if "pattern" in node and re.search(node["pattern"], value) is None:
            errors.append("%s: %r does not match %s" % (path, value, node["pattern"]))
    if isinstance(value, (int, float)) and not isinstance(value, bool):
        if "minimum" in node and value < node["minimum"]:
            errors.append("%s: below minimum %s" % (path, node["minimum"]))
        if "maximum" in node and value > node["maximum"]:
            errors.append("%s: above maximum %s" % (path, node["maximum"]))
    if isinstance(value, list):
        if "minItems" in node and len(value) < node["minItems"]:
            errors.append("%s: needs at least %d item(s)" % (path, node["minItems"]))
        if node.get("uniqueItems") and len(value) != len({json.dumps(v, sort_keys=True) for v in value}):
            errors.append("%s: items are not unique" % path)
        if "items" in node:
            for index, item in enumerate(value):
                validate(node["items"], item, "%s[%d]" % (path, index), errors)
    if isinstance(value, dict):
        for key in node.get("required", []):
            if key not in value:
                errors.append("%s: missing required property '%s'" % (path, key))
        properties = node.get("properties", {})
        additional = node.get("additionalProperties", True)
        for key, item in value.items():
            child = "%s.%s" % (path, key)
            if key in properties:
                validate(properties[key], item, child, errors)
            elif additional is False:
                errors.append("%s: additional property '%s' not allowed" % (path, key))


def resolve_pack(query, top_k):
    """Return the top_k pack `ao skills find` selects for query."""
    completed = subprocess.run(
        [ao_bin, "skills", "find", query, "--limit", str(top_k), "--json"],
        cwd=repo_root,
        capture_output=True,
        text=True,
        check=False,
    )
    if completed.returncode != 0:
        raise RuntimeError(
            "ao skills find exited %d: %s"
            % (completed.returncode, (completed.stderr or "").strip() or "no stderr")
        )
    return json.loads(completed.stdout)


def cites_a_real_file(item):
    """Provenance: the pack item names a source file that exists.

    A relative path is resolved against the repo the surface was run in, never
    against this process's CWD — the grader may be invoked from anywhere.
    """
    path = item.get("path")
    if not path:
        return False
    if not os.path.isabs(path):
        path = os.path.join(repo_root, path)
    return os.path.exists(path)


def grade(golden):
    """Grade one validated golden. Returns (findings, observed)."""
    top_k = golden["top_k"]
    pack = resolve_pack(golden["query"], top_k)[:top_k]
    ids = [item["name"] for item in pack]
    observed = {"pack": ids, "rank1": ids[0] if ids else None}
    findings = []

    if not pack:
        return ["EMPTY: the surface returned no pack for this query"], observed

    for expected_id in golden["expected_selected_ids"]:
        if expected_id not in ids:
            findings.append(
                "MISS: expected '%s' in the top-%d pack, got %s" % (expected_id, top_k, ids)
            )
    for leaked in golden["critical_omitted_ids"]:
        if leaked in ids:
            findings.append(
                "LEAK: '%s' must be omitted but ranked %d" % (leaked, ids.index(leaked) + 1)
            )
    for pattern in golden["forbidden_leaks"]:
        if re.search(pattern, ids[0]):
            findings.append("MISROUTE: rank-1 '%s' matches forbidden pattern %s" % (ids[0], pattern))

    resolvable = sum(1 for item in pack if cites_a_real_file(item))
    density = resolvable / len(pack)
    observed["provenance_density"] = round(density, 4)
    if density < golden["min_provenance_density"]:
        findings.append(
            "PROVENANCE: %.2f of the pack cites a resolvable path, below the declared %.2f"
            % (density, golden["min_provenance_density"])
        )

    tokens = sum(len((item.get("description") or "").split()) for item in pack)
    observed["tokens"] = tokens
    if tokens > golden["max_tokens"]:
        findings.append(
            "TOKENS: pack is %d tokens, over the declared ceiling of %d"
            % (tokens, golden["max_tokens"])
        )
    return findings, observed


paths = sorted(
    os.path.join(goldens_dir, name)
    for name in os.listdir(goldens_dir)
    if name.endswith(".json") and os.path.isfile(os.path.join(goldens_dir, name))
)

results = []
seen_ids = {}
for path in paths:
    relative = os.path.relpath(path, repo_root)
    entry = {"file": relative, "id": None, "findings": [], "observed": {}}
    try:
        with open(path, "r", encoding="utf-8") as handle:
            golden = json.load(handle)
    except (OSError, ValueError) as error:
        entry["findings"].append("SCHEMA: unreadable golden: %s" % error)
        results.append(entry)
        continue

    errors = []
    validate(schema, golden, "$", errors)
    if errors:
        entry["id"] = golden.get("id")
        entry["findings"] = ["SCHEMA: %s" % message for message in errors]
        results.append(entry)
        continue

    entry["id"] = golden["id"]
    if golden["id"] in seen_ids:
        entry["findings"].append(
            "SCHEMA: duplicate fixture id '%s' (also in %s)" % (golden["id"], seen_ids[golden["id"]])
        )
        results.append(entry)
        continue
    seen_ids[golden["id"]] = relative

    try:
        entry["findings"], entry["observed"] = grade(golden)
    except (RuntimeError, ValueError) as error:
        entry["findings"] = ["SURFACE: %s" % error]
    results.append(entry)

failed = [entry for entry in results if entry["findings"]]
summary = {
    "total": len(results),
    "passed": len(results) - len(failed),
    "failed": len(failed),
    "goldens": results,
}

if json_mode:
    print(json.dumps(summary, sort_keys=True))
else:
    for entry in results:
        status = "PASS" if not entry["findings"] else "FAIL"
        rank1 = entry["observed"].get("rank1") or "-"
        print("%-4s %-28s rank1=%s" % (status, entry["id"] or entry["file"], rank1))
        for finding in entry["findings"]:
            print("       %s" % finding)

if failed:
    # Flush first so a 2>&1 log shows the table above its own verdict line.
    sys.stdout.flush()
    print(
        "check-routing-probe-goldens: FAIL (%d/%d goldens missed their declared pack)"
        % (len(failed), len(results)),
        file=sys.stderr,
    )
    sys.exit(1)

if not json_mode:
    print(
        "check-routing-probe-goldens: PASS (%d/%d goldens matched their declared pack)"
        % (len(results), len(results))
    )
sys.exit(0)
PY
