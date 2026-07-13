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
    # Leading-underscore dirs (e.g. skills/_fixtures/) are non-skill scaffolding
    # — planted test fixtures, shared helpers — not real skills. Skip them so
    # they never inflate the registry / skill count.
    [[ "$name" == _* ]] && continue
    # Runtime compatibility pointers ship in install bundles but are aliases,
    # not canonical skills in the active registry/count surface.
    [[ "$name" == "pre-mortem" || "$name" == "post-mortem" || "$name" == "pre_mortem" || "$name" == "post_mortem" ]] && continue

    local tier="unknown"
    if [[ -f "$tiers_file" ]]; then
      local valid_tiers="judgment execution knowledge product session contribute cross-vendor library background meta utility experimental"
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

  # soc-k47k pattern: walk `git ls-files evals/` instead of filesystem `find`
  # so untracked artifacts (e.g. evals/workbench/scorecard-latest.json written
  # by local eval runs) don't drift the registry vs CI's clean checkout.
  local tracked
  tracked=$(cd "$REPO_ROOT" && git ls-files 'evals/*/*.json' 2>/dev/null || true)

  local result="[]"
  declare -A suites=()
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    local rel suite_name leaf
    rel="${line#evals/}"
    suite_name="${rel%%/*}"
    leaf="${rel#*/}"
    [[ -n "$suite_name" && -n "$leaf" && "$leaf" != */* ]] || continue
    suites[$suite_name]=1
  done <<< "$tracked"

  for suite_name in "${!suites[@]}"; do

    local eval_files=()
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      # Match only direct children of the suite (one level deep).
      [[ "$line" == "evals/${suite_name}/"*.json ]] || continue
      [[ "${line#evals/${suite_name}/}" == */*.json ]] && continue
      eval_files+=("$(basename "$line" .json)")
    done <<< "$tracked"

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

# ─── SKU catalog (schema v2) ─────────────────────────────────────────────────
#
# The SKU block (capabilities + capability_summary + cli_top_level_commands) is
# the schema-v2 capability catalog: a JOIN of SKILL.md frontmatter +
# skill-dispositions.yaml + SKILL-TIERS.md + the live `ao` cobra tree +
# validate.yml gate jobs + the packs/agentops reference City. It needs a built
# `ao` binary, so generate-registry.sh builds one once and reuses it for both the
# SKU block and the real CLI-command list. AGENTOPS_AO_BIN short-circuits the
# build (used by CI to avoid double-building).

# ensure_ao_bin echoes a path to a usable `ao` binary, or empty string if one
# cannot be produced (e.g. no cli/ dir, no Go toolchain). Callers degrade
# gracefully: an empty SKU block is emitted instead of crashing. The
# validate-sku-catalog-drift gate (which always runs in a real checkout with
# cli/) is the authoritative enforcer that the catalog is fully populated, so a
# silently-empty block on main is impossible — the gate would fail on drift.
ensure_ao_bin() {
  if [[ -n "${AGENTOPS_AO_BIN:-}" && -x "${AGENTOPS_AO_BIN}" ]]; then
    echo "${AGENTOPS_AO_BIN}"
    return
  fi
  if [[ ! -d "${REPO_ROOT}/cli" ]] || ! command -v go >/dev/null 2>&1; then
    echo ""
    return
  fi
  local bin
  bin="$(mktemp -d)/ao"
  if ( cd "${REPO_ROOT}/cli" && go build -o "$bin" ./cmd/ao ) >&2; then
    echo "$bin"
  else
    echo ""
  fi
}

# build_sku_block prints the SKU catalog JSON. Returns an empty-but-valid block
# when the ao binary or the generator helper is unavailable.
build_sku_block() {
  local ao_bin="$1"
  local empty='{"capabilities":[],"capability_summary":{"total":0,"skills":0,"cli_commands":0,"gates":0,"reference_impls":0},"cli_top_level_commands":[]}'
  if [[ -z "$ao_bin" || ! -f "${REPO_ROOT}/scripts/generate-sku-catalog.sh" ]]; then
    echo "$empty"
    return
  fi
  AGENTOPS_AO_BIN="$ao_bin" bash "${REPO_ROOT}/scripts/generate-sku-catalog.sh" 2>/dev/null || echo "$empty"
}

# ─── CLI Commands ────────────────────────────────────────────────────────────
#
# schema v2: the cli_commands surface is now the REAL top-level cobra command
# nodes (from the live tree via the SKU catalog), not a count of cli/cmd/ao/*.go
# files. The old file-count produced the misleading "163 commands" number
# (ag-cbm / oracle Gap #6); the truth is the top-level cobra command count.

build_cli_commands() {
  local sku_block="$1"
  echo "$sku_block" | jq '[.capabilities[] | select(.type == "cli-command") | {
    name: .name,
    path: .path,
    purpose: .purpose,
    bounded_context: .bounded_context,
    driven_by_skills: .driven_by_skills
  }] | sort_by(.name)'
}

# ─── Workflows surface (ag-4akl8 S0) ────────────────────────────────────────
#
# Emit the top-level `workflows:` section of skill-dispositions.yaml as a
# first-class registry surface. Workflows are kind: workflow artifacts (Claude
# Workflow .js scripts) — claude-only, parity-exempt — that the `- skill:`
# line-parsers deliberately skip. The registry surfaces them by reading `kind`
# so consumers can enumerate workflows without re-parsing the ledger.

build_workflows() {
  local disp_yaml="${REPO_ROOT}/docs/contracts/skill-dispositions.yaml"
  if [[ ! -f "$disp_yaml" ]] || ! command -v python3 >/dev/null 2>&1; then
    echo "[]"
    return
  fi
  DISP_YAML="$disp_yaml" python3 - <<'PY' 2>/dev/null || echo "[]"
import json
import os
import sys

try:
    import yaml
except ImportError:
    print("[]")
    sys.exit(0)

data = yaml.safe_load(open(os.environ["DISP_YAML"], encoding="utf-8")) or {}
out = []
for wf_id, e in (data.get("workflows") or {}).items():
    if not isinstance(e, dict):
        continue
    targets = e.get("runtime_targets") or []
    reach = "cross" if len(targets) > 1 else (targets[0] if targets else "")
    out.append({
        "id": wf_id,
        "kind": e.get("kind", "workflow"),
        "bounded_context": (str(e.get("domain", "")).split(" ", 1)[0] or ""),
        "hex_role": e.get("hexagonal_role", ""),
        "capability_class": e.get("capability_class", ""),
        "runtime_targets": targets,
        "runtime_reach": reach,
        "parity_policy": e.get("parity_policy", ""),
        "aliases": e.get("aliases", []) or [],
        "path": e.get("path", ""),
        "supersedes": (e.get("supersedes") if e.get("supersedes") not in (None, "null") else None),
    })
out.sort(key=lambda w: w["id"])
print(json.dumps(out))
PY
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
      "skills": ["curate", "forge", "compile", "inject"]
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
      "description": "Local cockpit validation before push to main",
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
  # No wall-clock stamp in the committed artifact: a `generated_at` timestamp
  # made back-to-back regens non-byte-identical (rewrote every `make regen-all`),
  # which collided/re-noised at nearly every serial landing. registry.json is now
  # a pure content-derived projection of its sources — matching the other
  # generators (generate-context-map.sh / generate-skill-domain-map.sh, which
  # also carry no timestamp). "When was this generated" = git log on the file.
  local skills hooks knowledge_stores job_types schedules evals cli_commands cadence
  local ao_bin sku_block

  # Build ao once; the SKU block + the real CLI-command list both need it.
  ao_bin="$(ensure_ao_bin)"
  sku_block="$(build_sku_block "$ao_bin")"

  # Build each surface
  skills=$(build_skills)
  hooks=$(build_hooks)
  knowledge_stores=$(build_knowledge_stores)
  job_types=$(build_job_types)
  schedules=$(build_schedules)
  evals=$(build_evals)
  cli_commands=$(build_cli_commands "$sku_block")
  cadence=$(build_cadence_recommendations)
  workflows=$(build_workflows)

  # Count totals
  local skill_count hook_count store_count job_type_count eval_suite_count cli_count workflow_count
  skill_count=$(echo "$skills" | jq 'length')
  hook_count=$(echo "$hooks" | jq 'length')
  store_count=$(echo "$knowledge_stores" | jq 'length')
  job_type_count=$(echo "$job_types" | jq 'length')
  eval_suite_count=$(echo "$evals" | jq '[.[].eval_count] | add // 0')
  cli_count=$(echo "$cli_commands" | jq 'length')
  workflow_count=$(echo "$workflows" | jq 'length')

  # SKU catalog block (schema v2)
  local capabilities capability_summary cli_top_level capability_count
  capabilities=$(echo "$sku_block" | jq '.capabilities')
  capability_summary=$(echo "$sku_block" | jq '.capability_summary')
  cli_top_level=$(echo "$sku_block" | jq '.cli_top_level_commands')
  capability_count=$(echo "$capabilities" | jq 'length')

  # Assemble the registry (schema v2 — superset of v1; existing consumers read
  # summary/surfaces/cadence_recommendations unchanged, new consumers read
  # capabilities/capability_summary).
  local registry
  registry=$(jq -n \
    --argjson skill_count "$skill_count" \
    --argjson hook_count "$hook_count" \
    --argjson store_count "$store_count" \
    --argjson job_type_count "$job_type_count" \
    --argjson eval_suite_count "$eval_suite_count" \
    --argjson cli_count "$cli_count" \
    --argjson capability_count "$capability_count" \
    --argjson skills "$skills" \
    --argjson hooks "$hooks" \
    --argjson knowledge_stores "$knowledge_stores" \
    --argjson job_types "$job_types" \
    --argjson schedules "$schedules" \
    --argjson evals "$evals" \
    --argjson cli_commands "$cli_commands" \
    --argjson cadence "$cadence" \
    --argjson capabilities "$capabilities" \
    --argjson capability_summary "$capability_summary" \
    --argjson cli_top_level "$cli_top_level" \
    --argjson workflows "$workflows" \
    --argjson workflow_count "$workflow_count" \
    '{
      schema_version: 2,
      summary: {
        skills: $skill_count,
        hooks: $hook_count,
        knowledge_stores: $store_count,
        job_types: $job_type_count,
        eval_files: $eval_suite_count,
        cli_commands: $cli_count,
        workflows: $workflow_count,
        capabilities: $capability_count
      },
      surfaces: {
        skills: $skills,
        hooks: $hooks,
        knowledge_stores: $knowledge_stores,
        job_types: $job_types,
        schedules: $schedules,
        evals: $evals,
        cli_commands: $cli_commands,
        workflows: $workflows
      },
      capability_summary: $capability_summary,
      cli_top_level_commands: $cli_top_level,
      capabilities: $capabilities,
      workflows: $workflows,
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
      # generated_at was removed from the body (was wall-clock, caused churn);
      # del(.generated_at) stays as a defensive no-op so an older committed
      # registry.json that still carries the field still compares clean during
      # rollout (and tolerates the field being reintroduced elsewhere).
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
      echo "Wrote ${OUTPUT} (schema v2: ${skill_count} skills, ${hook_count} hooks, ${store_count} stores, ${job_type_count} job types, ${eval_suite_count} evals, ${cli_count} CLI commands, ${capability_count} SKU capabilities)"
      ;;
  esac
}

main "$@"
