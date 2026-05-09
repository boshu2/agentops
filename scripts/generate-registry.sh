#!/usr/bin/env bash
# scripts/generate-registry.sh — Generate registry.json from repo source of truth
#
# Walks skills/, hooks/, .agents/, evals/, cli/cmd/ao/, and daemon types
# to produce a single queryable manifest of everything AgentOps manages.
#
# Usage:
#   bash scripts/generate-registry.sh           # write registry.json
#   bash scripts/generate-registry.sh --check   # exit 1 if registry.json is stale
#   bash scripts/generate-registry.sh --stdout  # print to stdout, don't write

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT="${REPO_ROOT}/registry.json"

MODE="write"
if [[ "${1:-}" == "--check" ]]; then MODE="check"; fi
if [[ "${1:-}" == "--stdout" ]]; then MODE="stdout"; fi

# ─── Skills ──────────────────────────────────────────────────────────────────

build_skills() {
  local skills_dir="${REPO_ROOT}/skills"
  local tiers_file="${skills_dir}/SKILL-TIERS.md"
  local result="[]"

  for skill_dir in "${skills_dir}"/*/; do
    [[ -d "$skill_dir" ]] || continue
    local name
    name="$(basename "$skill_dir")"

    local tier="unknown"
    if [[ -f "$tiers_file" ]]; then
      local valid_tiers="judgment execution knowledge product session contribute cross-vendor library background meta utility"
      local found=""
      # User-facing: | **name** | tier | description |
      while IFS= read -r line; do
        local candidate
        candidate=$(echo "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/, "", $3); print $3}')
        for vt in $valid_tiers; do
          if [[ "$candidate" == "$vt" ]]; then found="$candidate"; break 2; fi
        done
      done < <(grep -E "^\| \*\*${name}\*\* \|" "$tiers_file" 2>/dev/null || true)
      # Internal skills: | name | tier | category | purpose |
      if [[ -z "$found" ]]; then
        while IFS= read -r line; do
          local candidate
          candidate=$(echo "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/, "", $3); print $3}')
          for vt in $valid_tiers; do
            if [[ "$candidate" == "$vt" ]]; then found="$candidate"; break 2; fi
          done
        done < <(grep -E "^\| ${name} \|" "$tiers_file" 2>/dev/null || true)
      fi
      [[ -n "$found" ]] && tier="$found"
    fi

    local has_references=false
    local ref_count=0
    if [[ -d "${skill_dir}references" ]]; then
      has_references=true
      ref_count=$(find "${skill_dir}references" -name '*.md' -type f 2>/dev/null | wc -l | tr -d ' ')
    fi

    local has_skill_md=false
    [[ -f "${skill_dir}SKILL.md" ]] && has_skill_md=true

    result=$(echo "$result" | jq --arg name "$name" \
      --arg tier "$tier" \
      --arg path "skills/${name}/" \
      --argjson has_references "$has_references" \
      --argjson ref_count "$ref_count" \
      --argjson has_skill_md "$has_skill_md" \
      '. + [{
        name: $name,
        tier: $tier,
        path: $path,
        has_skill_md: $has_skill_md,
        has_references: $has_references,
        reference_count: $ref_count
      }]')
  done

  echo "$result" | jq 'sort_by(.tier, .name)'
}

# ─── Hooks ───────────────────────────────────────────────────────────────────

build_hooks() {
  local hooks_file="${REPO_ROOT}/hooks/hooks.json"
  [[ -f "$hooks_file" ]] || { echo "[]"; return; }

  jq '[
    .hooks | to_entries[] |
    .key as $lifecycle |
    .value[] |
    (.matcher // "all") as $matcher |
    .hooks[] |
    {
      name: (.command | split("/") | last | sub("\\.sh$"; "")),
      lifecycle: $lifecycle,
      matcher: $matcher,
      timeout: .timeout,
      path: (.command | sub("\\$\\{CLAUDE_PLUGIN_ROOT\\}/"; "")),
      type: .type
    }
  ] | sort_by(.lifecycle, .name)' "$hooks_file"
}

# ─── Knowledge Stores (.agents/ dirs) ────────────────────────────────────────

build_knowledge_stores() {
  # soc-k47k: don't early-return on missing .agents/ — `git ls-files` reads the
  # index, which is authoritative regardless of working-tree state. CI clean
  # checkout always populates the tracked files, so the only case where the
  # tracked list is empty is when the repo genuinely has no tracked .agents/
  # entries.

  # Known purpose map — manually maintained since .agents/ has no manifest
  local -A purposes=(
    [ao]="CLI runtime state and configuration"
    [archive]="Retired/archived knowledge entries"
    [brainstorm]="Brainstorm session outputs"
    [compaction-snapshots]="Pre-compaction context snapshots"
    [compiled]="Compiled wiki output from ao compile"
    [constraints]="Operational constraints and rules"
    [council]="Council validation session outputs"
    [crank]="Crank epic execution state"
    [daemon]="Daemon runtime state (jobs, ledger)"
    [defrag]="Defragmentation outputs"
    [design]="Design decision records"
    [discovery]="Discovery phase outputs"
    [dream-cycle]="Dream cycle runtime state"
    [evals]="Evaluation results and reports"
    [evolution]="Evolution loop state"
    [evolve]="Evolve skill session outputs"
    [findings]="Bug hunt and research findings"
    [handoff]="Session handoff documents"
    [handoffs]="Session handoff archives"
    [harvest]="Harvest consolidation outputs"
    [knowledge]="Promoted knowledge entries"
    [learnings]="Session learnings and insights"
    [ledger]="Append-only event ledger"
    [mine]="Mining extraction outputs"
    [nightly]="Nightly run outputs and digests"
    [overnight]="Overnight dream run outputs"
    [patterns]="Extracted code and workflow patterns"
    [planning-rules]="Reusable planning constraints"
    [plans]="Plan outputs from /plan skill"
    [pool]="Knowledge pool (ingested raw material)"
    [pre-mortem-checks]="Pre-mortem validation outputs"
    [pre-mortems]="Pre-mortem analysis documents"
    [products]="Product definition outputs"
    [releases]="Release notes and changelogs"
    [research]="Research outputs and reports"
    [retros]="Retrospective session outputs"
    [rpi]="RPI run state and registry"
    [sessions]="Session metadata and transcripts"
    [signals]="Quality and context signals"
    [specs]="Technical specifications"
    [staging]="Staging area for promotion"
    [test]="Test generation outputs"
    [tests]="Test results and reports"
    [validation]="Validation phase outputs"
    [vibe-context]="Vibe check context cache"
    [wiki]="Internal wiki entries"
  )

  # soc-k47k: walk `git ls-files .agents/` instead of filesystem so the registry
  # is deterministic across environments. CI's clean checkout and a session-built
  # local checkout will produce identical output. Only directories containing
  # tracked files appear in knowledge_stores; the file_count is the tracked-file
  # count (which is reproducible — `find` was not).
  local tracked
  tracked=$(cd "$REPO_ROOT" && git ls-files .agents/ 2>/dev/null || true)
  if [[ -z "$tracked" ]]; then
    echo "[]"
    return
  fi

  # Build (name → file_count) map from tracked .agents/<name>/... entries.
  declare -A counts=()
  while IFS= read -r tracked_path; do
    [[ -z "$tracked_path" ]] && continue
    # Extract the immediate child of .agents/ (e.g., "nightly" from ".agents/nightly/2026-05-07/foo.json").
    local subdir
    subdir="${tracked_path#.agents/}"
    subdir="${subdir%%/*}"
    [[ -n "$subdir" && "$subdir" != "$tracked_path" ]] || continue
    counts[$subdir]=$((${counts[$subdir]:-0} + 1))
  done <<< "$tracked"

  local result="[]"
  for name in "${!counts[@]}"; do
    local purpose="${purposes[$name]:-"Unknown — needs documentation"}"
    local file_count="${counts[$name]}"
    result=$(echo "$result" | jq --arg name "$name" \
      --arg purpose "$purpose" \
      --arg path ".agents/${name}/" \
      --argjson file_count "$file_count" \
      '. + [{
        name: $name,
        path: $path,
        purpose: $purpose,
        file_count: $file_count
      }]')
  done

  echo "$result" | jq 'sort_by(.name)'
}

# ─── Daemon Job Types ────────────────────────────────────────────────────────

build_job_types() {
  local types_file="${REPO_ROOT}/cli/internal/daemon/types.go"
  [[ -f "$types_file" ]] || { echo "[]"; return; }

  # Extract JobType constants from Go source
  grep -E 'JobType\w+\s+JobType\s*=\s*"' "$types_file" | while read -r line; do
    echo "$line" | sed -E 's/.*"([^"]+)".*/\1/'
  done | jq -R -s 'split("\n") | map(select(length > 0)) | map({
    job_type: .,
    domain: (split(".")[0]),
    action: (split(".")[1])
  }) | sort_by(.job_type)'
}

# ─── Scheduled Jobs (from example) ──────────────────────────────────────────

build_schedules() {
  local schedule_example="${REPO_ROOT}/docs/templates/schedule.yaml.example"
  local legacy_schedule_example="${REPO_ROOT}/.agents/schedule.yaml.example"
  local schedule_live="${REPO_ROOT}/.agents/schedule.yaml"

  local result='{"example": [], "live": []}'

  # Parse the tracked canonical example first. Ignore operator-local .agents
  # copies unless they are intentionally tracked, so registry generation is
  # deterministic across CI and developer machines.
  if [[ -f "$schedule_example" ]]; then
    local entries
    entries=$(parse_schedule_yaml "$schedule_example")
    result=$(echo "$result" | jq --argjson entries "$entries" '.example = $entries')
  elif git -C "$REPO_ROOT" ls-files --error-unmatch ".agents/schedule.yaml.example" >/dev/null 2>&1; then
    local entries
    entries=$(parse_schedule_yaml "$legacy_schedule_example")
    result=$(echo "$result" | jq --argjson entries "$entries" '.example = $entries')
  fi

  # Parse live schedules only when tracked. A live .agents/schedule.yaml is
  # normally operator-local runtime state and must not make registry.json drift.
  if git -C "$REPO_ROOT" ls-files --error-unmatch ".agents/schedule.yaml" >/dev/null 2>&1; then
    local entries
    entries=$(parse_schedule_yaml "$schedule_live")
    result=$(echo "$result" | jq --argjson entries "$entries" '.live = $entries')
  fi

  echo "$result"
}

parse_schedule_yaml() {
  local file="$1"
  # Extract schedule entries from YAML using awk
  awk '
    /^  - name:/ { if (name != "") print_entry(); name=$NF; cron=""; job_type=""; timeout="" }
    /cron:/ { gsub(/^[^"]*"|"[^"]*$/, "", $0); cron=$0 }
    /job_type:/ { job_type=$NF }
    /timeout:/ { timeout=$NF; gsub(/"/, "", timeout) }
    END { if (name != "") print_entry() }
    function print_entry() {
      printf "{\"name\":\"%s\",\"cron\":\"%s\",\"job_type\":\"%s\",\"timeout\":\"%s\"}\n", name, cron, job_type, timeout
    }
  ' "$file" | jq -s '.'
}

# ─── Evals ───────────────────────────────────────────────────────────────────

build_evals() {
  local evals_dir="${REPO_ROOT}/evals"
  [[ -d "$evals_dir" ]] || { echo "[]"; return; }

  local result="[]"
  for suite_dir in "${evals_dir}"/*/; do
    [[ -d "$suite_dir" ]] || continue
    local suite_name
    suite_name="$(basename "$suite_dir")"

    local eval_files=()
    while IFS= read -r -d '' f; do
      eval_files+=("$(basename "$f" .json)")
    done < <(find "$suite_dir" -maxdepth 1 -name '*.json' -type f -print0 2>/dev/null)

    local file_count=${#eval_files[@]}
    local files_json
    files_json=$(printf '%s\n' "${eval_files[@]}" | jq -R -s 'split("\n") | map(select(length > 0)) | sort')

    result=$(echo "$result" | jq --arg suite "$suite_name" \
      --arg path "evals/${suite_name}/" \
      --argjson file_count "$file_count" \
      --argjson files "$files_json" \
      '. + [{
        suite: $suite,
        path: $path,
        eval_count: $file_count,
        evals: $files
      }]')
  done

  echo "$result" | jq 'sort_by(.suite)'
}

# ─── CLI Commands ────────────────────────────────────────────────────────────

build_cli_commands() {
  local cmd_dir="${REPO_ROOT}/cli/cmd/ao"
  [[ -d "$cmd_dir" ]] || { echo "[]"; return; }

  # Extract top-level command groups from non-test Go files
  find "$cmd_dir" -maxdepth 1 -name '*.go' -not -name '*_test.go' -type f -print0 |
    xargs -0 grep -l 'func.*Command\(\)' 2>/dev/null |
    while read -r f; do
      basename "$f" .go
    done |
    sort -u |
    jq -R -s 'split("\n") | map(select(length > 0)) | map({
      name: .,
      path: ("cli/cmd/ao/" + . + ".go")
    })'
}

# ─── Cadence Recommendations ────────────────────────────────────────────────

build_cadence_recommendations() {
  # Opinionated baseline: what should run and when
  jq -n '[
    {
      "name": "dream-cycle",
      "cadence": "nightly",
      "cron": "0 3 * * *",
      "job_type": "dream.run",
      "description": "Full knowledge consolidation: harvest → forge → inject → defrag",
      "skills": ["dream", "harvest", "forge", "compile", "inject"]
    },
    {
      "name": "knowledge-forge",
      "cadence": "hourly",
      "cron": "5 * * * *",
      "job_type": "wiki.forge",
      "description": "Mine session transcripts into learnings",
      "skills": ["forge"]
    },
    {
      "name": "eval-suite",
      "cadence": "nightly",
      "cron": "0 4 * * *",
      "job_type": "eval.suite",
      "description": "Run behavioral eval suite against skill changes",
      "skills": ["scenario"]
    },
    {
      "name": "wiki-build",
      "cadence": "weekly",
      "cron": "0 5 * * 0",
      "job_type": "wiki.build",
      "description": "Full .agents/compiled rebuild",
      "skills": ["compile"]
    },
    {
      "name": "flywheel-health",
      "cadence": "weekly",
      "cron": "0 6 * * 1",
      "job_type": null,
      "description": "Knowledge flywheel staleness and pool depth check",
      "skills": ["flywheel"]
    },
    {
      "name": "deps-audit",
      "cadence": "weekly",
      "cron": "0 7 * * 2",
      "job_type": null,
      "description": "Dependency vulnerability and license audit",
      "skills": ["deps"]
    },
    {
      "name": "security-scan",
      "cadence": "weekly",
      "cron": "0 7 * * 3",
      "job_type": null,
      "description": "Repository security scan",
      "skills": ["security"]
    },
    {
      "name": "evolve-loop",
      "cadence": "weekly",
      "cron": "0 2 * * 6",
      "job_type": "rpi.run",
      "description": "Autonomous fitness-scored improvement cycle",
      "skills": ["evolve", "rpi"]
    },
    {
      "name": "pre-push-gate",
      "cadence": "per-push",
      "cron": null,
      "job_type": null,
      "description": "CI validation before push to main",
      "skills": []
    },
    {
      "name": "session-inject",
      "cadence": "per-session",
      "cron": null,
      "job_type": null,
      "description": "Context injection at session start",
      "skills": ["inject", "recover"]
    }
  ]'
}

# ─── Assemble ────────────────────────────────────────────────────────────────

main() {
  local generated_at
  generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

  local skills hooks knowledge_stores job_types schedules evals cli_commands cadence

  # Build each surface
  skills=$(build_skills)
  hooks=$(build_hooks)
  knowledge_stores=$(build_knowledge_stores)
  job_types=$(build_job_types)
  schedules=$(build_schedules)
  evals=$(build_evals)
  cli_commands=$(build_cli_commands)
  cadence=$(build_cadence_recommendations)

  # Count totals
  local skill_count hook_count store_count job_type_count eval_suite_count cli_count
  skill_count=$(echo "$skills" | jq 'length')
  hook_count=$(echo "$hooks" | jq 'length')
  store_count=$(echo "$knowledge_stores" | jq 'length')
  job_type_count=$(echo "$job_types" | jq 'length')
  eval_suite_count=$(echo "$evals" | jq '[.[].eval_count] | add // 0')
  cli_count=$(echo "$cli_commands" | jq 'length')

  # Assemble the registry
  local registry
  registry=$(jq -n \
    --arg generated_at "$generated_at" \
    --argjson skill_count "$skill_count" \
    --argjson hook_count "$hook_count" \
    --argjson store_count "$store_count" \
    --argjson job_type_count "$job_type_count" \
    --argjson eval_suite_count "$eval_suite_count" \
    --argjson cli_count "$cli_count" \
    --argjson skills "$skills" \
    --argjson hooks "$hooks" \
    --argjson knowledge_stores "$knowledge_stores" \
    --argjson job_types "$job_types" \
    --argjson schedules "$schedules" \
    --argjson evals "$evals" \
    --argjson cli_commands "$cli_commands" \
    --argjson cadence "$cadence" \
    '{
      schema_version: 1,
      generated_at: $generated_at,
      summary: {
        skills: $skill_count,
        hooks: $hook_count,
        knowledge_stores: $store_count,
        job_types: $job_type_count,
        eval_files: $eval_suite_count,
        cli_commands: $cli_count
      },
      surfaces: {
        skills: $skills,
        hooks: $hooks,
        knowledge_stores: $knowledge_stores,
        job_types: $job_types,
        schedules: $schedules,
        evals: $evals,
        cli_commands: $cli_commands
      },
      cadence_recommendations: $cadence
    }')

  case "$MODE" in
    stdout)
      echo "$registry"
      ;;
    check)
      if [[ ! -f "$OUTPUT" ]]; then
        echo "FAIL: registry.json does not exist. Run: bash scripts/generate-registry.sh" >&2
        exit 1
      fi
      # Compare ignoring generated_at timestamp
      local current_no_ts new_no_ts
      current_no_ts=$(jq 'del(.generated_at)' "$OUTPUT")
      new_no_ts=$(echo "$registry" | jq 'del(.generated_at)')
      if [[ "$current_no_ts" != "$new_no_ts" ]]; then
        echo "FAIL: registry.json is stale. Run: bash scripts/generate-registry.sh" >&2
        diff <(echo "$current_no_ts" | jq -S .) <(echo "$new_no_ts" | jq -S .) >&2 || true
        exit 1
      fi
      echo "OK: registry.json is up to date"
      ;;
    write)
      echo "$registry" > "$OUTPUT"
      echo "Wrote ${OUTPUT} (${skill_count} skills, ${hook_count} hooks, ${store_count} stores, ${job_type_count} job types, ${eval_suite_count} evals, ${cli_count} CLI commands)"
      ;;
  esac
}

main "$@"
