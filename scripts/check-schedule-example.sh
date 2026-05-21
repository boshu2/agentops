#!/usr/bin/env bash
# scripts/check-schedule-example.sh
#
# Fitness gate: dream-end-user-coverage (per GOALS.md Directive #8 + amendment
# B3 of soc-hxnr.6).
#
# Asserts that the stock schedule example:
#   1. Exists at the canonical tracked path docs/templates/schedule.yaml.example
#      (preferred), OR at .agents/schedule.yaml.example (operator's local copy
#      after `ao init --with-schedule`).
#   2. Parses as valid YAML with a non-empty `schedules:` list.
#   3. Contains at least one schedule with job_type from the v1.0 safe-set:
#      dream.run, wiki.forge (real-bodied executors per soc-8inr substrate).
#
# The tracked canonical (docs/templates/) is what makes this gate flip green
# permanently across environments — `.agents/schedule.yaml.example` is
# gitignored per the no-tracked-agents policy and only exists post-init on
# operator hosts.
#
# llmwiki.loop is intentionally excluded from the starter safe-set: placeholder
# outputs are default-disabled and require allow_placeholder_outputs=true for
# test/experimental fixtures until the real-bodies follow-up lands.
#
# Exit codes:
#   0 - PASS
#   1 - FAIL (missing file, parse error, or no safe-set job_type)

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
CANONICAL_PATH="$REPO_ROOT/docs/templates/schedule.yaml.example"
RUNTIME_PATH="$REPO_ROOT/.agents/schedule.yaml.example"

# Step 1: existence — prefer the tracked canonical, fall back to operator's
# local runtime copy.
if [[ -f "$CANONICAL_PATH" ]]; then
    EXAMPLE_PATH="$CANONICAL_PATH"
elif [[ -f "$RUNTIME_PATH" ]]; then
    EXAMPLE_PATH="$RUNTIME_PATH"
else
    echo "FAIL: stock schedule example missing" >&2
    echo "  expected at: $CANONICAL_PATH (canonical, tracked)" >&2
    echo "          or: $RUNTIME_PATH (operator runtime copy)" >&2
    echo "Per Directive #8 + soc-hxnr.6, the stock starter schedule must be present." >&2
    exit 1
fi

# Step 2: parse via Python YAML (always available on dev/CI hosts) and verify
# `schedules:` is a non-empty list. Falls back to a structural grep check if
# python3/PyYAML is unavailable.
parse_ok=0
if command -v python3 >/dev/null 2>&1; then
    if python3 - "$EXAMPLE_PATH" <<'PY' >/dev/null 2>&1
import sys
try:
    import yaml
except ImportError:
    sys.exit(2)  # signal "no PyYAML — fall back"
with open(sys.argv[1], "r", encoding="utf-8") as f:
    doc = yaml.safe_load(f)
if not isinstance(doc, dict):
    sys.exit(1)
schedules = doc.get("schedules")
if not isinstance(schedules, list) or len(schedules) == 0:
    sys.exit(1)
for entry in schedules:
    if not isinstance(entry, dict):
        sys.exit(1)
    if not entry.get("name") or not entry.get("cron") or not entry.get("job_type"):
        sys.exit(1)
sys.exit(0)
PY
    then
        parse_ok=1
    else
        rc=$?
        if [[ $rc -eq 2 ]]; then
            # PyYAML missing — fall back to structural grep
            parse_ok=0
        else
            echo "FAIL: $EXAMPLE_PATH does not parse as a valid schedules document" >&2
            exit 1
        fi
    fi
fi

if [[ $parse_ok -eq 0 ]]; then
    # Fallback: structural grep — confirm a top-level `schedules:` key and at
    # least one entry with name/cron/job_type. Less rigorous than a real parse
    # but adequate when PyYAML is unavailable.
    if ! grep -qE '^schedules:[[:space:]]*$' "$EXAMPLE_PATH"; then
        echo "FAIL: $EXAMPLE_PATH missing top-level 'schedules:' key" >&2
        exit 1
    fi
    if ! grep -qE '^[[:space:]]+-[[:space:]]+name:' "$EXAMPLE_PATH"; then
        echo "FAIL: $EXAMPLE_PATH has no schedule entries with 'name:'" >&2
        exit 1
    fi
    if ! grep -qE '^[[:space:]]+cron:' "$EXAMPLE_PATH"; then
        echo "FAIL: $EXAMPLE_PATH has no schedule entries with 'cron:'" >&2
        exit 1
    fi
fi

# Step 3: at least one schedule has job_type from the safe-set.
# Robust against YAML formatting variations (quoted/unquoted values).
if ! grep -qE 'job_type:[[:space:]]*"?(dream\.run|wiki\.forge)"?' "$EXAMPLE_PATH"; then
    echo "FAIL: $EXAMPLE_PATH must contain at least one schedule with job_type = dream.run or wiki.forge (real-bodied v1.0 safe-set)" >&2
    echo "llmwiki.loop placeholder outputs are default-disabled; keep the starter safe-set to dream.run/wiki.forge." >&2
    exit 1
fi

echo "OK: $EXAMPLE_PATH exists, parses, and uses real-bodied job_types"
exit 0
