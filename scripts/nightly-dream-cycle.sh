#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
nightly-dream-cycle.sh

Run the nightly dream-cycle proof flow against a throwaway copy of the repo's
checked-in .agents corpus, then emit machine-readable and markdown summaries.

Options:
  --ao <path>          Path to the ao binary (default: ao from PATH)
  --repo-root <path>   Repo root to snapshot (default: current directory)
  --output-dir <path>  Output directory for reports (required)
  --help               Show this help

Outputs:
  <output-dir>/harvest/latest.json
  <output-dir>/close-loop.json
  <output-dir>/metrics-health.json
  <output-dir>/retrieval-bench.json
  <output-dir>/defrag/latest.json
  <output-dir>/summary.json
  <output-dir>/summary.md
EOF
}

die() {
  echo "nightly-dream-cycle: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

count_files() {
  local dir="$1"
  local pattern="$2"
  if [[ ! -d "$dir" ]]; then
    echo 0
    return
  fi
  find "$dir" -type f -name "$pattern" | wc -l | tr -d ' '
}

AO_BIN="${AO_BIN:-ao}"
REPO_ROOT="${REPO_ROOT:-}"
OUTPUT_DIR="${OUTPUT_DIR:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ao)
      AO_BIN="${2:-}"
      shift 2
      ;;
    --repo-root)
      REPO_ROOT="${2:-}"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown arg: $1"
      ;;
  esac
done

[[ -n "$OUTPUT_DIR" ]] || die "--output-dir is required"

if [[ -n "$REPO_ROOT" ]]; then
  REPO_ROOT="$(cd "$REPO_ROOT" && pwd)"
else
  REPO_ROOT="$(pwd)"
fi

if [[ "$AO_BIN" == */* ]]; then
  [[ -x "$AO_BIN" ]] || die "ao binary is not executable: $AO_BIN"
  AO_BIN="$(cd "$(dirname "$AO_BIN")" && pwd)/$(basename "$AO_BIN")"
else
  require_cmd "$AO_BIN"
fi

require_cmd jq
require_cmd cp
require_cmd find

[[ -d "$REPO_ROOT/.agents" ]] || die "repo root does not contain .agents/: $REPO_ROOT"

OUTPUT_DIR="$(mkdir -p "$OUTPUT_DIR" && cd "$OUTPUT_DIR" && pwd)"
WORKSPACE_ROOT="$OUTPUT_DIR/workspace"
HARVEST_DIR="$OUTPUT_DIR/harvest"
PROMOTE_DIR="$OUTPUT_DIR/promoted"
DEFRAG_DIR="$OUTPUT_DIR/defrag"
SUMMARY_JSON="$OUTPUT_DIR/summary.json"
SUMMARY_MD="$OUTPUT_DIR/summary.md"
CLOSE_LOOP_JSON="$OUTPUT_DIR/close-loop.json"
METRICS_JSON="$OUTPUT_DIR/metrics-health.json"
RETRIEVAL_JSON="$OUTPUT_DIR/retrieval-bench.json"
FORGE_LOG="$OUTPUT_DIR/forge.log"

rm -rf "$WORKSPACE_ROOT" "$HARVEST_DIR" "$PROMOTE_DIR" "$DEFRAG_DIR"
mkdir -p "$WORKSPACE_ROOT"
cp -Rp "$REPO_ROOT/.agents" "$WORKSPACE_ROOT/.agents"

# Reset the forge-only outputs so the nightly proof measures what this run
# generated rather than inheriting previously checked-in session artifacts.
rm -rf \
  "$WORKSPACE_ROOT/.agents/knowledge/pending" \
  "$WORKSPACE_ROOT/.agents/ao/sessions" \
  "$WORKSPACE_ROOT/.agents/ao/index" \
  "$WORKSPACE_ROOT/.agents/ao/provenance"
rm -f "$WORKSPACE_ROOT/.agents/ao/forged.jsonl"
mkdir -p \
  "$WORKSPACE_ROOT/.agents/knowledge/pending" \
  "$WORKSPACE_ROOT/.agents/ao"

source_dirs=(
  "$WORKSPACE_ROOT/.agents/learnings"
  "$WORKSPACE_ROOT/.agents/patterns"
  "$WORKSPACE_ROOT/.agents/research"
  "$WORKSPACE_ROOT/.agents/retros"
  "$WORKSPACE_ROOT/.agents/findings"
)
markdown_sources=()
for dir in "${source_dirs[@]}"; do
  [[ -d "$dir" ]] || continue
  while IFS= read -r -d '' file; do
    markdown_sources+=("$file")
  done < <(find "$dir" -type f -name '*.md' -print0 | LC_ALL=C sort -z)
done
markdown_source_count="${#markdown_sources[@]}"

(
  cd "$WORKSPACE_ROOT"
  "$AO_BIN" harvest \
    --roots "$WORKSPACE_ROOT" \
    --output-dir "$HARVEST_DIR" \
    --promote-to "$PROMOTE_DIR" \
    --quiet >/dev/null
)

HARVEST_JSON="$HARVEST_DIR/latest.json"
[[ -f "$HARVEST_JSON" ]] || die "harvest catalog missing: $HARVEST_JSON"

: >"$FORGE_LOG"
if (( markdown_source_count > 0 )); then
  (
    cd "$WORKSPACE_ROOT"
    "$AO_BIN" forge markdown "${markdown_sources[@]}" --quiet >"$FORGE_LOG" 2>&1
  )
fi

forge_session_count="$(count_files "$WORKSPACE_ROOT/.agents/ao/sessions" '*.md')"
forge_pending_count="$(count_files "$WORKSPACE_ROOT/.agents/knowledge/pending" '*.md')"
harvest_promoted_count="$(count_files "$PROMOTE_DIR" '*.md')"

(
  cd "$WORKSPACE_ROOT"
  "$AO_BIN" flywheel close-loop --threshold 0h --json >"$CLOSE_LOOP_JSON"
)
[[ -f "$CLOSE_LOOP_JSON" ]] || die "close-loop report missing: $CLOSE_LOOP_JSON"

(
  cd "$WORKSPACE_ROOT"
  "$AO_BIN" defrag \
    --prune \
    --dedup \
    --oscillation-sweep \
    --output-dir "$DEFRAG_DIR" \
    --quiet >/dev/null
)
DEFRAG_JSON="$DEFRAG_DIR/latest.json"
[[ -f "$DEFRAG_JSON" ]] || die "defrag report missing: $DEFRAG_JSON"

(
  cd "$WORKSPACE_ROOT"
  "$AO_BIN" metrics health --json >"$METRICS_JSON"
)
[[ -f "$METRICS_JSON" ]] || die "metrics report missing: $METRICS_JSON"

(
  cd "$WORKSPACE_ROOT"
  "$AO_BIN" retrieval-bench --live --json >"$RETRIEVAL_JSON"
)
[[ -f "$RETRIEVAL_JSON" ]] || die "retrieval report missing: $RETRIEVAL_JSON"

harvest_rigs="$(jq -r '.rigs_scanned // 0' "$HARVEST_JSON")"
harvest_total_files="$(jq -r '.total_files // 0' "$HARVEST_JSON")"
harvest_artifacts="$(jq -r '(.artifacts | length) // 0' "$HARVEST_JSON")"
harvest_candidates="$(jq -r '(.promoted | length) // 0' "$HARVEST_JSON")"
harvest_duplicates="$(jq -r '[.duplicates[]? | (.count - 1)] | add // 0' "$HARVEST_JSON")"

close_ingest_scanned="$(jq -r '.ingest.files_scanned // 0' "$CLOSE_LOOP_JSON")"
close_ingest_found="$(jq -r '.ingest.candidates_found // 0' "$CLOSE_LOOP_JSON")"
close_ingest_added="$(jq -r '.ingest.added // 0' "$CLOSE_LOOP_JSON")"
close_auto_considered="$(jq -r '.auto_promote.considered // 0' "$CLOSE_LOOP_JSON")"
close_auto_promoted="$(jq -r '.auto_promote.promoted // 0' "$CLOSE_LOOP_JSON")"
close_feedback_rewarded="$(jq -r '.citation_feedback.rewarded // 0' "$CLOSE_LOOP_JSON")"
close_memory_promoted="$(jq -r '.memory_promoted // 0' "$CLOSE_LOOP_JSON")"
close_store_indexed="$(jq -r '.store.indexed // 0' "$CLOSE_LOOP_JSON")"

if (( close_ingest_added > 0 && close_auto_promoted == 0 )); then
  close_throughput_stalled="true"
else
  close_throughput_stalled="false"
fi

defrag_total_learnings="$(jq -r '.prune.total_learnings // 0' "$DEFRAG_JSON")"
defrag_stale="$(jq -r '.prune.stale_count // 0' "$DEFRAG_JSON")"
defrag_checked="$(jq -r '.dedup.checked // 0' "$DEFRAG_JSON")"

metrics_sigma="$(jq -r '.sigma // 0' "$METRICS_JSON")"
metrics_rho="$(jq -r '.rho // 0' "$METRICS_JSON")"
metrics_delta="$(jq -r '.delta // 0' "$METRICS_JSON")"
metrics_escape="$(jq -r '.escape_velocity // false' "$METRICS_JSON")"
metrics_total="$(jq -r '.knowledge_stock.total // 0' "$METRICS_JSON")"
metrics_learnings="$(jq -r '.knowledge_stock.learnings // 0' "$METRICS_JSON")"
metrics_patterns="$(jq -r '.knowledge_stock.patterns // 0' "$METRICS_JSON")"
metrics_findings="$(jq -r '.knowledge_stock.findings // 0' "$METRICS_JSON")"
metrics_constraints="$(jq -r '.knowledge_stock.constraints // 0' "$METRICS_JSON")"
metrics_r1="$(jq -r '.loop_dominance.r1 // 0' "$METRICS_JSON")"
metrics_b1="$(jq -r '.loop_dominance.b1 // 0' "$METRICS_JSON")"
metrics_dominant="$(jq -r '.loop_dominance.dominant // "unknown"' "$METRICS_JSON")"
retrieval_mode="$(jq -r '.mode // "live-local"' "$RETRIEVAL_JSON")"
retrieval_total_learnings="$(jq -r '.total_learnings // 0' "$RETRIEVAL_JSON")"
retrieval_queries="$(jq -r '.queries // 0' "$RETRIEVAL_JSON")"
retrieval_k="$(jq -r '.k // 0' "$RETRIEVAL_JSON")"
retrieval_queries_with_hits="$(jq -r '.queries_with_hits // 0' "$RETRIEVAL_JSON")"
retrieval_coverage="$(jq -r '.coverage // 0' "$RETRIEVAL_JSON")"
retrieval_hit_summary="$(jq -c '[.results[]? | {query, count, top_ids: (.top_ids // [])}]' "$RETRIEVAL_JSON")"
retrieval_coverage_pct="$(awk "BEGIN { printf \"%.0f\", $retrieval_coverage * 100 }")"

jq -n \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg repo_root "$REPO_ROOT" \
  --arg workspace_root "$WORKSPACE_ROOT" \
  --argjson markdown_source_count "$markdown_source_count" \
  --argjson forge_session_count "$forge_session_count" \
  --argjson forge_pending_count "$forge_pending_count" \
  --argjson harvest_rigs "$harvest_rigs" \
  --argjson harvest_total_files "$harvest_total_files" \
  --argjson harvest_artifacts "$harvest_artifacts" \
  --argjson harvest_candidates "$harvest_candidates" \
  --argjson harvest_duplicates "$harvest_duplicates" \
  --argjson harvest_promoted_count "$harvest_promoted_count" \
  --argjson close_ingest_scanned "$close_ingest_scanned" \
  --argjson close_ingest_found "$close_ingest_found" \
  --argjson close_ingest_added "$close_ingest_added" \
  --argjson close_auto_considered "$close_auto_considered" \
  --argjson close_auto_promoted "$close_auto_promoted" \
  --argjson close_feedback_rewarded "$close_feedback_rewarded" \
  --argjson close_memory_promoted "$close_memory_promoted" \
  --argjson close_store_indexed "$close_store_indexed" \
  --arg close_throughput_stalled "$close_throughput_stalled" \
  --argjson defrag_total_learnings "$defrag_total_learnings" \
  --argjson defrag_stale "$defrag_stale" \
  --argjson defrag_checked "$defrag_checked" \
  --argjson metrics_sigma "$metrics_sigma" \
  --argjson metrics_rho "$metrics_rho" \
  --argjson metrics_delta "$metrics_delta" \
  --argjson metrics_total "$metrics_total" \
  --argjson metrics_learnings "$metrics_learnings" \
  --argjson metrics_patterns "$metrics_patterns" \
  --argjson metrics_findings "$metrics_findings" \
  --argjson metrics_constraints "$metrics_constraints" \
  --argjson metrics_r1 "$metrics_r1" \
  --argjson metrics_b1 "$metrics_b1" \
  --arg metrics_dominant "$metrics_dominant" \
  --arg metrics_escape "$metrics_escape" \
  --arg retrieval_mode "$retrieval_mode" \
  --argjson retrieval_total_learnings "$retrieval_total_learnings" \
  --argjson retrieval_queries "$retrieval_queries" \
  --argjson retrieval_k "$retrieval_k" \
  --argjson retrieval_queries_with_hits "$retrieval_queries_with_hits" \
  --argjson retrieval_coverage "$retrieval_coverage" \
  --argjson retrieval_hit_summary "$retrieval_hit_summary" \
  '{
    generated_at: $generated_at,
    repo_root: $repo_root,
    workspace_root: $workspace_root,
    harvest: {
      rigs_scanned: $harvest_rigs,
      total_files: $harvest_total_files,
      artifacts: $harvest_artifacts,
      promotion_candidates: $harvest_candidates,
      duplicates: $harvest_duplicates,
      promoted_files: $harvest_promoted_count
    },
    forge: {
      markdown_sources: $markdown_source_count,
      session_artifacts: $forge_session_count,
      pending_learnings: $forge_pending_count
    },
    close_loop: {
      files_scanned: $close_ingest_scanned,
      candidates_found: $close_ingest_found,
      added: $close_ingest_added,
      considered: $close_auto_considered,
      auto_promoted: $close_auto_promoted,
      feedback_rewarded: $close_feedback_rewarded,
      memory_promoted: $close_memory_promoted,
      indexed: $close_store_indexed,
      throughput_stalled: ($close_throughput_stalled == "true")
    },
    defrag: {
      total_learnings: $defrag_total_learnings,
      stale_count: $defrag_stale,
      dedup_checked: $defrag_checked
    },
    metrics: {
      sigma: $metrics_sigma,
      rho: $metrics_rho,
      delta: $metrics_delta,
      escape_velocity: ($metrics_escape == "true"),
      knowledge_stock: {
        total: $metrics_total,
        learnings: $metrics_learnings,
        patterns: $metrics_patterns,
        findings: $metrics_findings,
        constraints: $metrics_constraints
      },
      loop_dominance: {
        r1: $metrics_r1,
        b1: $metrics_b1,
        dominant: $metrics_dominant
      }
    },
    retrieval_live: {
      report_path: "retrieval-bench.json",
      mode: $retrieval_mode,
      total_learnings: $retrieval_total_learnings,
      queries: $retrieval_queries,
      k: $retrieval_k,
      queries_with_hits: $retrieval_queries_with_hits,
      coverage: $retrieval_coverage,
      query_hit_summary: $retrieval_hit_summary
    }
  }' >"$SUMMARY_JSON"

if [[ "$close_throughput_stalled" == "true" ]]; then
  throughput_banner="> **Throughput stall detected.** ${close_ingest_added} candidate(s) ingested but 0 auto-promoted. See \`.agents/learnings/2026-04-22-close-loop-citation-gate-deadlock.md\`.

"
else
  throughput_banner=""
fi

cat >"$SUMMARY_MD" <<EOF
## Nightly Dream Cycle

${throughput_banner}This run snapshots the checked-in \`.agents/\` corpus into an ephemeral nightly workspace, runs the dream-cycle primitives there, and uploads the resulting reports with the workflow run.

| Stage | Result |
|-------|--------|
| Harvest | ${harvest_artifacts} artifacts from ${harvest_rigs} rig(s); ${harvest_candidates} promotion candidates; ${harvest_promoted_count} promoted into the temp store |
| Forge | ${markdown_source_count} markdown source file(s); ${forge_session_count} session artifact(s); ${forge_pending_count} pending learning(s) |
| Close Loop | ${close_ingest_added} candidate(s) ingested; ${close_auto_promoted} auto-promoted; ${close_feedback_rewarded} citation reward(s); ${close_store_indexed} search index update(s) |
| Defrag | ${defrag_checked} learning(s) checked; ${defrag_stale} stale candidate(s) flagged |
| Health | sigma=${metrics_sigma}; rho=${metrics_rho}; delta=${metrics_delta}; escape_velocity=${metrics_escape} |

### Knowledge Stock

- Total artifacts: ${metrics_total}
- Learnings: ${metrics_learnings}
- Patterns: ${metrics_patterns}
- Findings: ${metrics_findings}
- Constraints: ${metrics_constraints}

### Loop Dominance

- Dominant loop: ${metrics_dominant}
- R1: ${metrics_r1}
- B1: ${metrics_b1}

### Live Retrieval

- Mode: ${retrieval_mode}
- Corpus learnings: ${retrieval_total_learnings}
- Coverage: ${retrieval_coverage_pct}% (${retrieval_queries_with_hits}/${retrieval_queries} queries with hits)
- K: ${retrieval_k}
- Report: \`retrieval-bench.json\`

\`\`\`json
${retrieval_hit_summary}
\`\`\`
EOF

echo "Dream-cycle summary written to $SUMMARY_MD"

# Throughput-deadlock gate. Reports (and by default fails) when close-loop
# ingested candidates but auto-promoted zero — the pattern that quietly
# persisted through multiple nightly runs before
# .agents/learnings/2026-04-22-close-loop-citation-gate-deadlock.md.
# Set NIGHTLY_DREAM_STRICT=0 to downgrade to a warning (e.g. for diagnostic
# runs where you only want to collect numbers).
THROUGHPUT_SCRIPT="$(dirname "$0")/check-dream-throughput.sh"
if [[ -x "$THROUGHPUT_SCRIPT" ]]; then
  if "$THROUGHPUT_SCRIPT" "$CLOSE_LOOP_JSON"; then
    :
  else
    rc=$?
    if [[ "${NIGHTLY_DREAM_STRICT:-1}" == "0" ]]; then
      echo "WARN: throughput gate returned $rc (NIGHTLY_DREAM_STRICT=0 — not failing)" >&2
    else
      exit "$rc"
    fi
  fi
else
  echo "WARN: $THROUGHPUT_SCRIPT not found or not executable; skipping throughput gate" >&2
fi
