#!/usr/bin/env bash
# shellcheck disable=SC2089,SC2090
# This script builds bash/yaml template strings (PY_SETUP, PREPUSH_SECTION,
# BATS_STUB) whose literal backslashes and quotes are intentional — a Python
# block at the bottom of the file substitutes them verbatim into target
# workflow/pre-push files. SC2089/SC2090 advise switching to arrays, which
# would defeat the multi-line template approach.
# scripts/add-validate-job.sh
#
# CI integration scaffolder: atomically add a new validate-* job across
# all 5 touch-points in lockstep. The same registries-drift pattern the
# soc-zxia epic closed for DDD docs, applied to CI integration.
#
# Surfaces touched (one command, one consistent change):
#   1. .github/workflows/validate.yml — new job block
#   2. .github/workflows/validate.yml — summary.needs list entry
#   3. .github/workflows/validate.yml — summary echo line
#   4. scripts/pre-push-gate.sh       — new always-runs section
#   5. tests/scripts/pre-push-gate.bats — make_stub for helper script
#   6. AGENTS.md                       — CI Jobs table row
#
# Usage:
#   scripts/add-validate-job.sh <job-name> \
#     --script <path>   (required: scripts/check-foo.sh)
#     --reason <text>   (required: what this validates, AGENTS table)
#     --failure <text>  (required: common failure mode, AGENTS table)
#     --python          (optional: emit pyyaml install step)
#     --dry-run         (optional: show changes without applying)
#
# Job name must match validate-*-* convention.
# Helper script must exist before running (caller writes the check first).
# Idempotent: re-running with same args is a no-op.
#
# Encodes soc-3oij (closes the CI-integration-touch-point drift class).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# --- parse args ---

JOB_NAME=""
HELPER_SCRIPT=""
REASON=""
FAILURE=""
PYTHON=0
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --script)   HELPER_SCRIPT="$2"; shift 2 ;;
    --reason)   REASON="$2";       shift 2 ;;
    --failure)  FAILURE="$2";      shift 2 ;;
    --python)   PYTHON=1;          shift ;;
    --dry-run)  DRY_RUN=1;         shift ;;
    -h|--help)
      sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    -*)
      echo "ERROR: unknown flag: $1" >&2
      exit 2
      ;;
    *)
      if [[ -z "$JOB_NAME" ]]; then
        JOB_NAME="$1"
      else
        echo "ERROR: unexpected positional arg: $1 (job name already set: $JOB_NAME)" >&2
        exit 2
      fi
      shift
      ;;
  esac
done

# --- validate inputs ---

if [[ -z "$JOB_NAME" || -z "$HELPER_SCRIPT" || -z "$REASON" || -z "$FAILURE" ]]; then
  echo "ERROR: missing required input. Run --help for usage." >&2
  echo "  job name: ${JOB_NAME:-<missing>}" >&2
  echo "  --script: ${HELPER_SCRIPT:-<missing>}" >&2
  echo "  --reason: ${REASON:-<missing>}" >&2
  echo "  --failure: ${FAILURE:-<missing>}" >&2
  exit 2
fi

if ! [[ "$JOB_NAME" =~ ^validate-[a-z0-9-]+$ ]]; then
  echo "ERROR: job name must match 'validate-<kebab>' format; got: $JOB_NAME" >&2
  exit 2
fi

if [[ ! -f "$REPO_ROOT/$HELPER_SCRIPT" ]]; then
  echo "ERROR: helper script not found: $REPO_ROOT/$HELPER_SCRIPT" >&2
  echo "  Write the check script first, then run this scaffolder." >&2
  exit 2
fi

# Conflict detection: any of the 5 surfaces already mention this job name?
WORKFLOW="$REPO_ROOT/.github/workflows/validate.yml"
PREPUSH="$REPO_ROOT/scripts/pre-push-gate.sh"
BATS="$REPO_ROOT/tests/scripts/pre-push-gate.bats"
AGENTS="$REPO_ROOT/AGENTS.md"

for f in "$WORKFLOW" "$PREPUSH" "$BATS" "$AGENTS"; do
  if [[ ! -f "$f" ]]; then
    echo "ERROR: required surface missing: $f" >&2
    exit 2
  fi
done

if grep -q "^  ${JOB_NAME}:" "$WORKFLOW" \
   || grep -q "$JOB_NAME" "$AGENTS"; then
  echo "INFO: job $JOB_NAME already exists; nothing to do (idempotent)." >&2
  exit 0
fi

# --- build the 5 patch blocks ---

# 1. workflow job block
PY_SETUP=""
if [[ "$PYTHON" == "1" ]]; then
  PY_SETUP="
      - uses: actions/setup-python@v6
        with:
          python-version: '3.14'

      - name: Install PyYAML (script dep)
        run: pip install pyyaml
"
fi

WORKFLOW_JOB="  ${JOB_NAME}:
    needs: [changes]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
${PY_SETUP}
      - name: ${REASON}
        run: |
          chmod +x ${HELPER_SCRIPT}
          ./${HELPER_SCRIPT}
"

# 2. pre-push section (numbered 22z+, but use a marker not a fixed letter)
PREPUSH_SECTION="
# --- ${JOB_NAME} (scaffolded by add-validate-job.sh) ---
# ${REASON}
# Always runs (no needs_check guard).
if [[ -f ${HELPER_SCRIPT} ]]; then
    if scaffold_${JOB_NAME//-/_}_output=\"\$(bash ${HELPER_SCRIPT} 2>&1)\"; then
        pass \"${JOB_NAME}\"
    else
        fail \"${JOB_NAME}\"
        indent_output \"\$scaffold_${JOB_NAME//-/_}_output\"
    fi
else
    fail \"missing file: ${HELPER_SCRIPT}\"
fi
"

# 3. bats stub
BATS_STUB="    make_stub \"\$FAKE_REPO/${HELPER_SCRIPT}\""

# 4. AGENTS row
AGENTS_ROW="| **${JOB_NAME}** | ${REASON} | ${FAILURE} |"

# --- apply or preview ---

if [[ "$DRY_RUN" == "1" ]]; then
  echo "=== DRY RUN: would emit these 6 patches ==="
  echo "--- 1. .github/workflows/validate.yml (new job block) ---"
  echo "$WORKFLOW_JOB"
  echo "--- 2. .github/workflows/validate.yml (summary.needs entry) ---"
  echo "  add to validate.yml summary.needs list: $JOB_NAME"
  echo "--- 3. .github/workflows/validate.yml (summary echo line) ---"
  echo "          echo \"${JOB_NAME}: \${{ needs.${JOB_NAME}.result }}\""
  echo "--- 4. scripts/pre-push-gate.sh (always-runs section) ---"
  echo "$PREPUSH_SECTION"
  echo "--- 5. tests/scripts/pre-push-gate.bats (make_stub line) ---"
  echo "$BATS_STUB"
  echo "--- 6. AGENTS.md (CI Jobs row) ---"
  echo "$AGENTS_ROW"
  exit 0
fi

# Apply each patch via python for precise multi-line insertion.
export WORKFLOW PREPUSH BATS AGENTS JOB_NAME HELPER_SCRIPT WORKFLOW_JOB PREPUSH_SECTION BATS_STUB AGENTS_ROW

python3 - <<'PY'
import os
import re

WORKFLOW = os.environ["WORKFLOW"]
PREPUSH  = os.environ["PREPUSH"]
BATS     = os.environ["BATS"]
AGENTS   = os.environ["AGENTS"]
JOB_NAME = os.environ["JOB_NAME"]
WORKFLOW_JOB    = os.environ["WORKFLOW_JOB"]
PREPUSH_SECTION = os.environ["PREPUSH_SECTION"]
BATS_STUB       = os.environ["BATS_STUB"]
AGENTS_ROW      = os.environ["AGENTS_ROW"]


def patch_file(path, mutator):
    text = open(path).read()
    new_text = mutator(text)
    if new_text == text:
        print(f"  no-op: {path}")
        return
    with open(path, "w") as f:
        f.write(new_text)
    print(f"  patched: {path}")


# 1. workflow job block — insert before validate-flywheel-proof:
def insert_workflow_job(text):
    anchor = "  validate-flywheel-proof:"
    if anchor not in text:
        raise SystemExit("ERROR: anchor 'validate-flywheel-proof:' not found in workflow")
    return text.replace(anchor, WORKFLOW_JOB + "\n" + anchor, 1)


# 2. summary.needs — append before 'validate-flywheel-proof,'
def insert_summary_needs(text):
    pattern = r'(\bvalidate-flywheel-proof,)'
    if not re.search(pattern, text):
        raise SystemExit("ERROR: 'validate-flywheel-proof,' not in summary.needs list")
    return re.sub(pattern, f'{JOB_NAME}, \\1', text, count=1)


# 3. summary echo — insert after last validate-*-drift echo (anchor on flywheel-proof echo)
def insert_summary_echo(text):
    anchor = '          echo "validate-flywheel-proof: ${{ needs.validate-flywheel-proof.result }}"'
    if anchor not in text:
        raise SystemExit("ERROR: 'validate-flywheel-proof' echo anchor not in summary")
    new_echo = f'          echo "{JOB_NAME}: ${{{{ needs.{JOB_NAME}.result }}}}"'
    return text.replace(anchor, new_echo + "\n" + anchor, 1)


def patch_workflow(text):
    text = insert_workflow_job(text)
    text = insert_summary_needs(text)
    text = insert_summary_echo(text)
    return text


# 4. pre-push gate — insert before flywheel-proof section
def patch_prepush(text):
    anchor = "# --- 22d. Flywheel-proof"
    if anchor not in text:
        raise SystemExit("ERROR: '# --- 22d. Flywheel-proof' anchor not in pre-push-gate.sh")
    return text.replace(anchor, PREPUSH_SECTION + "\n" + anchor, 1)


# 5. bats — insert make_stub after the existing block of make_stubs (find last consecutive line)
def patch_bats(text):
    anchor = '    make_stub "$FAKE_REPO/scripts/proof-run.sh"'
    if anchor not in text:
        raise SystemExit("ERROR: bats make_stub anchor 'proof-run.sh' not found")
    return text.replace(anchor, BATS_STUB + "\n" + anchor, 1)


# 6. AGENTS — insert row before validate-flywheel-proof row
def patch_agents(text):
    anchor_pattern = r'\| \*\*validate-flywheel-proof\*\*'
    m = re.search(anchor_pattern, text)
    if not m:
        raise SystemExit("ERROR: 'validate-flywheel-proof' row anchor not in AGENTS.md")
    insertion_point = text.rfind('\n', 0, m.start()) + 1
    return text[:insertion_point] + AGENTS_ROW + "\n" + text[insertion_point:]


patch_file(WORKFLOW, patch_workflow)
patch_file(PREPUSH,  patch_prepush)
patch_file(BATS,     patch_bats)
patch_file(AGENTS,   patch_agents)
PY

echo ""
echo "Scaffolded $JOB_NAME across 4 files (6 patch points). Review the diff:"
echo "  git -C $REPO_ROOT diff"
