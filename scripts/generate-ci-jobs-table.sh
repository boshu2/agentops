#!/usr/bin/env bash
# generate-ci-jobs-table.sh — render AGENTS.md "### CI Jobs and What They Check"
# table from .github/workflows/validate.yml + docs/contracts/ci-jobs.yaml.
#
# soc-3oij: AGENTS CI table generator. Eliminates hand-edit drift — adding a
# new validate-* job goes through scripts/add-validate-job.sh (soc-3oij meta-fix,
# PR #315) which writes the workflow row; the manifest gets a matching entry;
# this generator renders the AGENTS table.
#
# Modes:
#   (default)    Render table to stdout
#   --check      Render, diff against AGENTS.md section, exit 1 if drift
#   --write      Render and rewrite the section in-place
#
# Inputs:
#   AGENTS_PATH=$REPO_ROOT/AGENTS.md
#   WORKFLOW_PATH=$REPO_ROOT/.github/workflows/validate.yml
#   MANIFEST_PATH=$REPO_ROOT/docs/contracts/ci-jobs.yaml

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

AGENTS_PATH="${AGENTS_PATH:-$REPO_ROOT/AGENTS-CI.md}"
WORKFLOW_PATH="${WORKFLOW_PATH:-$REPO_ROOT/.github/workflows/validate.yml}"
MANIFEST_PATH="${MANIFEST_PATH:-$REPO_ROOT/docs/contracts/ci-jobs.yaml}"

MODE="render"
for arg in "$@"; do
    case "$arg" in
        --check) MODE="check" ;;
        --write) MODE="write" ;;
        -h|--help)
            sed -nE 's/^# ?(.*)/\1/p' "$0" | head -25
            exit 0
            ;;
        *)
            echo "unknown flag: $arg" >&2
            exit 2
            ;;
    esac
done

for f in "$AGENTS_PATH" "$WORKFLOW_PATH" "$MANIFEST_PATH"; do
    if [[ ! -f "$f" ]]; then
        echo "missing required file: $f" >&2
        exit 2
    fi
done

# Render the table to stdout using a tiny embedded Python — keeps yaml parsing
# correct (escaped pipes, multi-line reasons) and ordering deterministic.
render_table() {
    python3 - "$WORKFLOW_PATH" "$MANIFEST_PATH" <<'PYEOF'
import re
import sys
import yaml

workflow_path, manifest_path = sys.argv[1], sys.argv[2]

with open(manifest_path) as f:
    manifest = yaml.safe_load(f)
by_name = {j["name"]: j for j in manifest["jobs"]}

with open(workflow_path) as f:
    wf_text = f.read()

# Extract summary.needs job order
needs_match = re.search(
    r"^  summary:\n(?:.*\n)*?^    needs:\s*\[([^\]]+)\]",
    wf_text,
    re.MULTILINE,
)
if not needs_match:
    print("could not parse summary.needs from workflow", file=sys.stderr)
    sys.exit(2)
needs = [j.strip() for j in needs_match.group(1).split(",")]
needs = [j for j in needs if j and j != "changes"]

# Detect continue-on-error jobs (non-blocking)
coe = set()
current_job = None
for line in wf_text.splitlines():
    m = re.match(r"^  ([a-z][a-z0-9_-]+):\s*$", line)
    if m:
        current_job = m.group(1)
        continue
    if current_job and re.match(r"^    continue-on-error:\s*true\s*$", line):
        coe.add(current_job)

# Validate parity
missing = [j for j in needs if j not in by_name]
extra = [n for n in by_name if n not in needs]
if missing or extra:
    if missing:
        print(f"manifest missing entries for: {missing}", file=sys.stderr)
    if extra:
        print(f"manifest has entries not in workflow.summary.needs: {extra}", file=sys.stderr)
    sys.exit(3)

# Emit table
print("| Job | What it validates | Common failure |")
print("|-----|-------------------|----------------|")
for job in needs:
    entry = by_name[job]
    name_cell = f"**{job}**"
    if job in coe:
        name_cell = f"**{job}** (non-blocking)"
    # Escape pipes inside cells
    reason = entry["reason"].replace("|", r"\|")
    failure = entry["failure"].replace("|", r"\|")
    print(f"| {name_cell} | {reason} | {failure} |")
PYEOF
}

# Find AGENTS.md table boundaries: section header → next ### or EOF.
extract_agents_section() {
    awk '
        BEGIN { in_section=0 }
        /^### CI Jobs and What They Check$/ { in_section=1; next }
        in_section && /^### / { in_section=0 }
        in_section { print }
    ' "$AGENTS_PATH"
}

case "$MODE" in
    render)
        render_table
        ;;
    check)
        TMP_GEN="$(mktemp)"
        TMP_HAVE="$(mktemp)"
        trap 'rm -f "$TMP_GEN" "$TMP_HAVE"' EXIT

        render_table > "$TMP_GEN"
        # Extract just the table from AGENTS section (skip leading blank/heading)
        extract_agents_section | awk '/^\|/{print}' > "$TMP_HAVE"
        # Drop trailing blank lines from generated output
        sed -i '/^$/d' "$TMP_GEN"

        if diff -u "$TMP_HAVE" "$TMP_GEN" >/dev/null; then
            echo "CI_JOBS_TABLE: PASS ($(wc -l < "$TMP_GEN" | tr -d ' ') rows)"
            exit 0
        else
            echo "CI_JOBS_TABLE: FAIL — AGENTS.md table drifts from generator output"
            echo "--- AGENTS.md (have)"
            echo "+++ generator (want from docs/contracts/ci-jobs.yaml + validate.yml)"
            diff -u "$TMP_HAVE" "$TMP_GEN" || true
            echo ""
            echo "Action: run scripts/generate-ci-jobs-table.sh --write to refresh."
            exit 1
        fi
        ;;
    write)
        TMP_GEN="$(mktemp)"
        TMP_NEW="$(mktemp)"
        trap 'rm -f "$TMP_GEN" "$TMP_NEW"' EXIT

        render_table > "$TMP_GEN"
        # Walk AGENTS.md replacing the section content between
        # "### CI Jobs and What They Check" and the next "### " header.
        awk -v gen_file="$TMP_GEN" '
            BEGIN {
                while ((getline line < gen_file) > 0) {
                    gen[++n] = line
                }
                close(gen_file)
                in_section = 0
                emitted = 0
            }
            /^### CI Jobs and What They Check$/ {
                print
                print ""
                for (i = 1; i <= n; i++) print gen[i]
                print ""
                in_section = 1
                emitted = 1
                next
            }
            in_section && /^### / {
                in_section = 0
                print
                next
            }
            in_section { next }
            { print }
        ' "$AGENTS_PATH" > "$TMP_NEW"

        mv "$TMP_NEW" "$AGENTS_PATH"
        echo "CI_JOBS_TABLE: wrote $(wc -l < "$TMP_GEN" | tr -d ' ') rows to $AGENTS_PATH"
        ;;
esac
