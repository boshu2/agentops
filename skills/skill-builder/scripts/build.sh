#!/usr/bin/env bash
# build.sh — skill-builder mode dispatcher
# Usage:
#   build.sh from-scratch <skill-name>
#   build.sh from-template <skill-name> --like <existing-skill>
#   build.sh absorb-external <skill-name> --from <path-to-external-SKILL.md>
#   build.sh from-pattern   # alpha: passthrough to ao flywheel close-loop
#
# Always runs the heal-skill deep audit (absorbed from /skill-auditor) on the new skill as a self-check before declaring success.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# SKILL_BUILDER_REPO_ROOT overrides root discovery for fixture-driven tests
# (see init.sh — same contract, mirrors HEAL_REPO_ROOT in heal.sh).
REPO_ROOT="${SKILL_BUILDER_REPO_ROOT:-$(cd "$SCRIPT_DIR/../../.." && pwd)}"
INIT_SH="$SCRIPT_DIR/init.sh"
AUDITOR_SH="$REPO_ROOT/skills/heal-skill/scripts/audit.sh"
PROFILE_TOOL="$REPO_ROOT/skills/skill-builder/scripts/conformance_profile.py"
EXTERNAL_SOURCE=""

profile_args=(--repo-root "$REPO_ROOT")
if [[ -n "${SKILL_CONFORMANCE_PROFILE_ID:-}" ]]; then
  profile_args+=(--profile-id "$SKILL_CONFORMANCE_PROFILE_ID")
fi
if [[ ! -f "$PROFILE_TOOL" ]] || ! PROFILE_ID="$(python3 "$PROFILE_TOOL" "${profile_args[@]}")"; then
  echo "[skill-builder] ERROR: profile configuration is unavailable or invalid" >&2
  exit 2
fi
export SKILL_CONFORMANCE_PROFILE_ID="$PROFILE_ID"

usage() {
  cat <<EOF
Usage:
  $0 from-scratch <skill-name>
  $0 from-template <skill-name> --like <existing-skill>
  $0 absorb-external <skill-name> --from <path-to-external-SKILL.md>
  $0 from-pattern    # alpha: passthrough to ao flywheel close-loop

Modes:
  from-scratch     Interactive scaffold from canonical template
  from-template    Copy structure from a sibling skill
  absorb-external  Wrap an external SKILL.md in AgentOps frontmatter
  from-pattern     ALPHA — delegates to 'ao flywheel close-loop'.
                   Outputs at .agents/knowledge/promoted/, NOT shaped as SKILL.md drafts.
                   Use from-scratch or absorb-external for SKILL.md output today.
EOF
  exit 2
}

[[ $# -lt 1 ]] && usage

MODE="$1"
shift

case "$MODE" in
  from-pattern)
    # Alpha passthrough — explicitly documented in SKILL.md
    echo "[skill-builder] from-pattern is ALPHA — delegating to 'ao flywheel close-loop'"
    echo "[skill-builder] Output will NOT be a SKILL.md draft; it lands at .agents/knowledge/promoted/"
    exec ao flywheel close-loop "$@"
    ;;

  from-scratch)
    [[ $# -lt 1 ]] && { echo "Error: from-scratch requires <skill-name>" >&2; usage; }
    SKILL_NAME="$1"; shift
    bash "$INIT_SH" --interactive "$SKILL_NAME" "$@"
    ;;

  from-template)
    [[ $# -lt 1 ]] && { echo "Error: from-template requires <skill-name>" >&2; usage; }
    SKILL_NAME="$1"; shift
    bash "$INIT_SH" --like-flag-mode "$SKILL_NAME" "$@"
    ;;

  absorb-external)
    [[ $# -lt 1 ]] && { echo "Error: absorb-external requires <skill-name>" >&2; usage; }
    SKILL_NAME="$1"; shift
    if [[ "${1:-}" == "--from" ]]; then
      EXTERNAL_SOURCE="${2:-}"
    fi
    bash "$INIT_SH" --absorb "$SKILL_NAME" "$@"
    ;;

  *)
    echo "Error: unknown mode '$MODE'" >&2
    usage
    ;;
esac

# Post-build self-audit (mandatory per Critical Constraints)
NEW_SKILL_DIR="$REPO_ROOT/skills/$SKILL_NAME"
if [[ ! -d "$NEW_SKILL_DIR" ]]; then
  echo "[skill-builder] ERROR: expected $NEW_SKILL_DIR to exist after init.sh" >&2
  exit 1
fi

# The build report (written by init.sh) carries audit_pass=null as a pre-audit
# placeholder. Patch it to the REAL audit outcome here so the report records what
# actually happened and stays schema-valid (build-report.json requires
# audit_pass to be a boolean). Without this the report's audit_pass was always
# null — the original defect this fixes (age-fix-skill-factory-mcc).
BUILD_REPORT="$REPO_ROOT/.agents/audits/${SKILL_NAME}-build.json"

# patch_audit_pass <true|false> — record the audit outcome in the build report.
# Never fails the build: a missing report or absent jq/python3 only warns.
patch_audit_pass() {
  [[ -f "$BUILD_REPORT" ]] || return 0
  local val="$1" tmp
  # Create the temp in the report's OWN directory so the mv is an atomic
  # same-filesystem rename (a cross-device mv can degrade to a non-atomic copy
  # and corrupt the report on failure).
  tmp="$(mktemp "$(dirname "$BUILD_REPORT")/.audit-patch.XXXXXX")" || return 0
  if command -v jq >/dev/null 2>&1; then
    # Fold the mv into the tested condition so a failed write only WARNs (never
    # aborts the build under set -e).
    if jq --argjson ap "$val" '.audit_pass = $ap' "$BUILD_REPORT" >"$tmp" 2>/dev/null \
       && mv "$tmp" "$BUILD_REPORT" 2>/dev/null; then
      :
    else
      rm -f "$tmp"; echo "[skill-builder] WARN: could not patch audit_pass in $BUILD_REPORT" >&2
    fi
  elif command -v python3 >/dev/null 2>&1; then
    if python3 - "$BUILD_REPORT" "$val" >"$tmp" 2>/dev/null <<'PY' && mv "$tmp" "$BUILD_REPORT" 2>/dev/null
import json, sys
d = json.load(open(sys.argv[1]))
d["audit_pass"] = (sys.argv[2] == "true")
json.dump(d, sys.stdout, indent=2)
PY
    then
      :
    else
      rm -f "$tmp"; echo "[skill-builder] WARN: could not patch audit_pass in $BUILD_REPORT" >&2
    fi
  else
    rm -f "$tmp"; echo "[skill-builder] WARN: no jq/python3 to record audit_pass in $BUILD_REPORT" >&2
  fi
  return 0
}

# The generated kernel and any external adoption must satisfy the selected
# profile before the auditor can declare the build successful.
if ! PROFILE_EVAL="$(python3 "$PROFILE_TOOL" --repo-root "$REPO_ROOT" \
  --profile-id "$PROFILE_ID" --audit-tsv "$NEW_SKILL_DIR/SKILL.md")"; then
  patch_audit_pass false
  exit 1
fi
KERNEL_LIMIT="$(printf '%s\n' "$PROFILE_EVAL" | awk -F '\t' '$1 == "kernel_max_lines" {print $2}')"
KERNEL_LINES="$(printf '%s\n' "$PROFILE_EVAL" | awk -F '\t' '$1 == "line_count" {print $2}')"
if [[ -z "$KERNEL_LIMIT" || -z "$KERNEL_LINES" ]] || (( KERNEL_LINES > KERNEL_LIMIT )); then
  patch_audit_pass false
  echo "[skill-builder] ERROR: generated SKILL.md has ${KERNEL_LINES:-unknown} lines; profile $PROFILE_ID allows $KERNEL_LIMIT" >&2
  exit 1
fi

if [[ -n "$EXTERNAL_SOURCE" ]]; then
  if ! python3 "$PROFILE_TOOL" --repo-root "$REPO_ROOT" \
    --profile-id "$PROFILE_ID" --verify-clean-room "$EXTERNAL_SOURCE" \
    --generated-dir "$NEW_SKILL_DIR" \
    --generated-dir "$REPO_ROOT/skills-codex/$SKILL_NAME"
  then
    patch_audit_pass false
    echo "[skill-builder] ERROR: clean-room verification failed" >&2
    exit 1
  fi
fi

if [[ -x "$AUDITOR_SH" ]]; then
  echo ""
  echo "[skill-builder] Running self-audit on $NEW_SKILL_DIR..."
  if bash "$AUDITOR_SH" "$NEW_SKILL_DIR"; then
    patch_audit_pass true
    echo "[skill-builder] Self-audit PASS or WARN — build complete (audit_pass=true)"
  else
    # Record the failure before aborting so the report reflects reality.
    patch_audit_pass false
    echo "[skill-builder] Self-audit FAIL — build aborted (audit_pass=false)" >&2
    exit 1
  fi
else
  # An unaudited build cannot claim a pass: record false rather than leaving the
  # null placeholder (which would be schema-invalid and read as "audited").
  patch_audit_pass false
  echo "[skill-builder] WARN: heal-skill audit script not found at $AUDITOR_SH; skipping self-audit (audit_pass=false)" >&2
fi
