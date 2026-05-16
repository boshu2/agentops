#!/usr/bin/env bash
# smoke-test-codex-skills.sh — DAG-based headless smoke test for Codex skills.
# Traverses skill dependency graph in topological order, spawns codex exec
# per skill, collects PASS/PARTIAL/FAIL verdicts.
#
# Usage:
#   scripts/smoke-test-codex-skills.sh [OPTIONS]
#
# Options:
#   --dry-run       Print what would run without spawning Codex
#   --chain N       Run only chain N (1-4)
#   --skill NAME    Run only a single skill
#   --timeout SECS  Per-skill timeout (default: 90)
#   --parallel N    Max parallel Codex invocations (default: 4)
#   --model MODEL   Codex model (default: gpt-5.4)
#   --static-only   Run only static checks (no codex exec)
#   --json          Output results as JSON
#   --verbose       Show Codex output for each skill
#   --help          Show this help
#
# Exit codes:
#   0 = PASS (all skills PASS or PARTIAL)
#   1 = FAIL (any skill BLOCKED/FAIL)
#   2 = ERROR (script error)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_CODEX="$REPO_ROOT/skills-codex"
RESULTS_DIR="$REPO_ROOT/.agents/smoke-test"

# Defaults
DRY_RUN=false
CHAIN_FILTER=""
SKILL_FILTER=""
TIMEOUT=90
PARALLEL=4
MODEL="gpt-5.4"
STATIC_ONLY=false
JSON_OUTPUT=false
VERBOSE=false

# --- Argument parsing ---
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)    DRY_RUN=true; shift ;;
    --chain)      CHAIN_FILTER="$2"; shift 2 ;;
    --skill)      SKILL_FILTER="$2"; shift 2 ;;
    --timeout)    TIMEOUT="$2"; shift 2 ;;
    --parallel)   PARALLEL="$2"; shift 2 ;;
    --model)      MODEL="$2"; shift 2 ;;
    --static-only) STATIC_ONLY=true; shift ;;
    --json)       JSON_OUTPUT=true; shift ;;
    --verbose)    VERBOSE=true; shift ;;
    --help)
      awk 'NR==1{next} /^[^#]/{exit} {sub(/^# ?/,""); print}' "$0"
      exit 0
      ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

# --- Preflight ---
if [[ ! -d "$SKILLS_CODEX" ]]; then
  echo "Error: skills-codex directory not found: $SKILLS_CODEX" >&2
  exit 2
fi

if [[ "$STATIC_ONLY" == "false" && "$DRY_RUN" == "false" ]]; then
  if ! command -v codex &>/dev/null; then
    echo "Error: codex CLI not found. Install or use --static-only." >&2
    exit 2
  fi
fi

mkdir -p "$RESULTS_DIR"

# --- DAG Definition ---
# Extracted from skill body analysis (see .agents/handoff/2026-03-14-codex-api-alignment.md)
# Format: LAYER[n]="skill1 skill2 ..."

LAYER0="standards shared beads brainstorm inject forge retro ratchet provenance compile handoff recover quickstart goals flywheel openai-docs oss-docs product security security-suite release converter update using-agentops status heal-skill codex-team pr-research pr-plan pr-implement pr-validate pr-prep pr-retro grafana-platform-dashboard reverse-engineer-rpi push"
LAYER1="council research doc implement bug-hunt trace readme"
LAYER2="pre-mortem post-mortem vibe complexity"
LAYER3="swarm validation plan"
LAYER4="crank discovery"
LAYER5="rpi evolve"

# Chains for traversal (minimum covering paths)
CHAIN1="standards council pre-mortem plan research inject brainstorm discovery implement beads swarm vibe complexity bug-hunt crank post-mortem validation retro forge rpi ratchet"
CHAIN2="pr-research pr-plan pr-implement pr-validate pr-prep pr-retro"
CHAIN3="compile evolve flywheel provenance trace"
CHAIN4="doc readme handoff recover status quickstart goals product oss-docs release security security-suite heal-skill codex-team update converter using-agentops grafana-platform-dashboard reverse-engineer-rpi push openai-docs shared"

# --- Static Validation ---
static_check() {
  local skill_name="$1"
  local skill_md="$SKILLS_CODEX/$skill_name/SKILL.md"
  local issues=()

  if [[ ! -f "$skill_md" ]]; then
    echo "MISSING"
    return
  fi

  # Check 1: Frontmatter — only name + description
  local fm
  fm=$(awk 'NR==1 && /^---$/{in_fm=1; next} in_fm && /^---$/{exit} in_fm{print}' "$skill_md")
  local bad_fields
  bad_fields=$(echo "$fm" | grep -oE '^[a-z_-]+:' | sed 's/:$//' | grep -vE '^(name|description)$' || true)
  if [[ -n "$bad_fields" ]]; then
    issues+=("bad-frontmatter:$bad_fields")
  fi

  # Check 2: Claude primitives
  local body
  body=$(awk 'BEGIN{skip=0} NR==1 && /^---$/{skip=1; next} skip && /^---$/{skip=0; next} !skip{print}' "$skill_md")
  local primitives='TaskCreate|TaskList|TaskUpdate|TaskGet|TaskStop|TeamCreate|TeamDelete|SendMessage|EnterPlanMode|ExitPlanMode|EnterWorktree|todo_write|update_plan'
  if echo "$body" | grep -qE "\b($primitives)\b" 2>/dev/null; then
    issues+=("claude-primitives")
  fi

  # Check 3: Claude paths
  if grep -q '~/\.claude/' "$skill_md" 2>/dev/null; then
    issues+=("claude-paths")
  fi

  # Check 4: Skill() invocations
  if grep -q 'Skill(skill=' "$skill_md" 2>/dev/null; then
    issues+=("skill-tool-invocation")
  fi

  # Check 5: Reference files
  local refs_dir="$SKILLS_CODEX/$skill_name/references"
  if [[ -d "$refs_dir" ]]; then
    while IFS= read -r ref_file; do
      if grep -qE "\b($primitives)\b" "$ref_file" 2>/dev/null; then
        issues+=("ref-primitives:$(basename "$ref_file")")
      fi
      if grep -q '~/\.claude/' "$ref_file" 2>/dev/null; then
        issues+=("ref-claude-paths:$(basename "$ref_file")")
      fi
    done < <(find "$refs_dir" -name '*.md' -type f 2>/dev/null)
  fi

  if [[ ${#issues[@]} -eq 0 ]]; then
    echo "CLEAN"
  else
    echo "ISSUES:$(IFS=','; echo "${issues[*]}")"
  fi
}

# --- Codex Smoke Test ---
codex_smoke() {
  local skill_name="$1"
  local result_file="$RESULTS_DIR/${skill_name}.json"

  local prompt="Smoke test the '\$${skill_name}' skill. You are in read-only mode — do NOT try to execute the skill or write files.

Steps:
1. Find and read the SKILL.md for '${skill_name}' (check skills-codex/${skill_name}/SKILL.md)
2. Verify the skill loads (has valid frontmatter with name + description)
3. Check if the instructions reference tools/primitives that don't exist in Codex (e.g. TaskCreate, TeamCreate, SendMessage, EnterPlanMode — these are Claude-only)
4. Check if \$skill invocations reference skills that exist in skills-codex/

Rate the skill:
- PASS: loads correctly, all referenced tools/skills exist in Codex
- PARTIAL: loads but references some unavailable tools/features (list which ones)
- FAIL: won't load, has broken frontmatter, or references only non-existent primitives

IMPORTANT: Read-only sandbox and missing network access are NOT reasons to FAIL — those are test environment limits, not skill defects.

Output EXACTLY one JSON line at the end:
{\"skill\": \"${skill_name}\", \"verdict\": \"PASS|PARTIAL|FAIL\", \"reason\": \"brief explanation\"}"

  local codex_output
  if codex_output=$(timeout "$TIMEOUT" codex exec -s read-only -m "$MODEL" -C "$REPO_ROOT" "$prompt" 2>&1); then
    # Extract JSON verdict from output
    local json_line
    json_line=$(echo "$codex_output" | tr -d '\n' | grep -oE '\{[^}]*"verdict"[^}]*\}' | tail -1 || true)
    if [[ -n "$json_line" ]]; then
      echo "$json_line" > "$result_file"
      local verdict
      verdict=$(echo "$json_line" | sed -n 's/.*"verdict"[[:space:]]*:[[:space:]]*"\([A-Z]*\)".*/\1/p' | tail -1)
      verdict="${verdict:-UNKNOWN}"
      if [[ "$VERBOSE" == "true" ]]; then
        echo "$codex_output" >&2
      fi
      echo "$verdict"
    else
      echo "{\"skill\": \"$skill_name\", \"verdict\": \"FAIL\", \"reason\": \"No JSON verdict in output\"}" > "$result_file"
      if [[ "$VERBOSE" == "true" ]]; then
        echo "$codex_output" >&2
      fi
      echo "FAIL"
    fi
  else
    local exit_code=$?
    if [[ $exit_code -eq 124 ]]; then
      echo "{\"skill\": \"$skill_name\", \"verdict\": \"FAIL\", \"reason\": \"Timeout after ${TIMEOUT}s\"}" > "$result_file"
      echo "TIMEOUT"
    else
      echo "{\"skill\": \"$skill_name\", \"verdict\": \"FAIL\", \"reason\": \"Codex exit code $exit_code\"}" > "$result_file"
      echo "ERROR"
    fi
  fi
}

# --- Build skill list based on filters ---
get_skills() {
  local skills=""

  if [[ -n "$SKILL_FILTER" ]]; then
    echo "$SKILL_FILTER"
    return
  fi

  if [[ -n "$CHAIN_FILTER" ]]; then
    case "$CHAIN_FILTER" in
      1) skills="$CHAIN1" ;;
      2) skills="$CHAIN2" ;;
      3) skills="$CHAIN3" ;;
      4) skills="$CHAIN4" ;;
      *) echo "Invalid chain: $CHAIN_FILTER (must be 1-4)" >&2; exit 2 ;;
    esac
    echo "$skills"
    return
  fi

  # All skills in topological order (deduped across chains)
  echo "$LAYER0 $LAYER1 $LAYER2 $LAYER3 $LAYER4 $LAYER5"
}

# --- Main ---
main() {
  local skills
  skills=$(get_skills)

  local total=0
  local pass=0
  local partial=0
  local fail=0
  # shellcheck disable=SC2034
  # reserved for future use
  local blocked=0

  declare -A static_results
  declare -A smoke_results

  echo "=== Codex Skill Smoke Test ==="
  echo "Mode: $(if $STATIC_ONLY; then echo 'static-only'; elif $DRY_RUN; then echo 'dry-run'; else echo "live (model=$MODEL, timeout=${TIMEOUT}s, parallel=$PARALLEL)"; fi)"
  echo "Skills: $(echo "$skills" | wc -w | tr -d ' ')"
  echo ""

  # Phase 1: Static validation (always runs)
  echo "--- Phase 1: Static Validation ---"
  for skill in $skills; do
    if [[ -d "$SKILLS_CODEX/$skill" ]]; then
      local result
      result=$(static_check "$skill")
      static_results["$skill"]="$result"
      if [[ "$result" == "CLEAN" ]]; then
        printf "  %-30s %s\n" "$skill" "CLEAN"
      else
        printf "  %-30s %s\n" "$skill" "$result"
      fi
    else
      static_results["$skill"]="MISSING"
      printf "  %-30s %s\n" "$skill" "MISSING (no skills-codex/ dir)"
    fi
    total=$((total + 1))
  done

  echo ""

  # Phase 2: Headless Codex smoke test (unless --static-only)
  if [[ "$STATIC_ONLY" == "true" ]]; then
    echo "--- Phase 2: Skipped (--static-only) ---"
    echo ""

    # Score from static results only
    for skill in $skills; do
      case "${static_results[$skill]}" in
        CLEAN)   pass=$((pass + 1)) ;;
        MISSING) fail=$((fail + 1)) ;;
        *)       partial=$((partial + 1)) ;;
      esac
    done
  elif [[ "$DRY_RUN" == "true" ]]; then
    echo "--- Phase 2: Dry Run ---"
    for skill in $skills; do
      echo "  Would run: codex exec -s read-only -m $MODEL -C $REPO_ROOT \"smoke test $skill\""
    done
    echo ""
    # In dry-run, score from static only
    for skill in $skills; do
      case "${static_results[$skill]}" in
        CLEAN)   pass=$((pass + 1)) ;;
        MISSING) fail=$((fail + 1)) ;;
        *)       partial=$((partial + 1)) ;;
      esac
    done
  else
    echo "--- Phase 2: Headless Codex Smoke Test ---"

    # Parallel execution with job control
    local running=0
    local pids=()
    local pid_skills=()

    for skill in $skills; do
      # Skip skills that failed static check badly
      if [[ "${static_results[$skill]}" == "MISSING" ]]; then
        smoke_results["$skill"]="SKIP"
        printf "  %-30s %s\n" "$skill" "SKIP (missing)"
        fail=$((fail + 1))
        continue
      fi

      # Throttle parallel jobs
      while [[ $running -ge $PARALLEL ]]; do
        # Wait for any child to finish
        for i in "${!pids[@]}"; do
          if ! kill -0 "${pids[$i]}" 2>/dev/null; then
            wait "${pids[$i]}" 2>/dev/null || true
            local completed_skill="${pid_skills[$i]}"
            local result_file="$RESULTS_DIR/${completed_skill}.verdict"
            if [[ -f "$result_file" ]]; then
              local verdict
              verdict=$(cat "$result_file")
              smoke_results["$completed_skill"]="$verdict"
              printf "  %-30s %s\n" "$completed_skill" "$verdict"
              case "$verdict" in
                PASS)    pass=$((pass + 1)) ;;
                PARTIAL) partial=$((partial + 1)) ;;
                *)       fail=$((fail + 1)) ;;
              esac
            else
              smoke_results["$completed_skill"]="ERROR"
              printf "  %-30s %s\n" "$completed_skill" "ERROR (no verdict file)"
              fail=$((fail + 1))
            fi
            unset 'pids[i]'
            unset 'pid_skills[i]'
            running=$((running - 1))
          fi
        done
        # Reindex arrays
        pids=("${pids[@]}")
        pid_skills=("${pid_skills[@]}")
        sleep 0.5
      done

      # Launch smoke test in background
      (
        verdict=$(codex_smoke "$skill")
        echo "$verdict" > "$RESULTS_DIR/${skill}.verdict"
      ) &
      pids+=($!)
      pid_skills+=("$skill")
      running=$((running + 1))
    done

    # Wait for remaining jobs
    for i in "${!pids[@]}"; do
      wait "${pids[$i]}" 2>/dev/null || true
      local completed_skill="${pid_skills[$i]}"
      local result_file="$RESULTS_DIR/${completed_skill}.verdict"
      if [[ -f "$result_file" ]]; then
        local verdict
        verdict=$(cat "$result_file")
        smoke_results["$completed_skill"]="$verdict"
        printf "  %-30s %s\n" "$completed_skill" "$verdict"
        case "$verdict" in
          PASS)    pass=$((pass + 1)) ;;
          PARTIAL) partial=$((partial + 1)) ;;
          *)       fail=$((fail + 1)) ;;
        esac
      else
        smoke_results["$completed_skill"]="ERROR"
        printf "  %-30s %s\n" "$completed_skill" "ERROR (no verdict file)"
        fail=$((fail + 1))
      fi
    done

    echo ""
  fi

  # --- Summary ---
  echo "=== Release Gate Verdict ==="
  echo "Total: $total  PASS: $pass  PARTIAL: $partial  FAIL: $fail"
  echo ""

  if [[ "$JSON_OUTPUT" == "true" ]]; then
    echo "{"
    echo "  \"total\": $total,"
    echo "  \"pass\": $pass,"
    echo "  \"partial\": $partial,"
    echo "  \"fail\": $fail,"
    echo "  \"verdict\": \"$(if [[ $fail -eq 0 ]]; then echo "PASS"; else echo "FAIL"; fi)\","
    echo "  \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
    echo "  \"model\": \"$MODEL\","
    echo "  \"static_only\": $STATIC_ONLY"
    echo "}"
  fi

  if [[ $fail -gt 0 ]]; then
    echo "VERDICT: FAIL ($fail skill(s) failed)"
    echo ""
    echo "Failed skills:"
    for skill in $skills; do
      if [[ "${static_results[$skill]:-}" == "MISSING" ]] || \
         [[ "${smoke_results[$skill]:-}" == "FAIL" ]] || \
         [[ "${smoke_results[$skill]:-}" == "TIMEOUT" ]] || \
         [[ "${smoke_results[$skill]:-}" == "ERROR" ]]; then
        echo "  - $skill: static=${static_results[$skill]:-n/a} smoke=${smoke_results[$skill]:-n/a}"
      fi
    done
    exit 1
  else
    echo "VERDICT: PASS (all skills operational)"
    exit 0
  fi
}

main
