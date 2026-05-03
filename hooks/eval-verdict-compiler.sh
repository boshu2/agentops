#!/usr/bin/env bash
# AgentOps Hook Helper: eval-verdict-compiler
# Reads ~/.agents/evals/runs/<id>/manifest.json with status=complete and
# verdict.kind set; mutates .agents/learnings/*.md frontmatter.
# Sibling to finding-compiler.sh. Closes the AgentOps self-modification loop.
#
# Plan: ~/dev/agentops/.agents/plans/2026-05-01-eval-as-self-pruning-corpus.md
# DEVIATION: rev-2 plan called for `lib/finding-compiler-helpers.sh` extraction.
# Wave-1 inlines 3 needed helpers to keep blast radius small. Wave-1.5 extracts.

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
EVALS_ROOT="${AGENTOPS_EVALS_ROOT:-$HOME/.agents/evals}"
WATERMARK="$EVALS_ROOT/processed.jsonl"
EMA_ALPHA="${AGENTOPS_VERDICT_EMA_ALPHA:-0.7}"
EMA_BETA="${AGENTOPS_VERDICT_EMA_BETA:-0.3}"
HARMFUL_THRESHOLD="${AGENTOPS_VERDICT_HARMFUL_THRESHOLD:-3}"
LOW_UTILITY_THRESHOLD="${AGENTOPS_VERDICT_LOW_UTILITY:-0.3}"
QUIET="${AGENTOPS_VERDICT_QUIET:-0}"
DRY_RUN=0; SINCE=""; ONLY_MANIFEST=""

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) DRY_RUN=1; shift ;;
        --since) SINCE="$2"; shift 2 ;;
        --since=*) SINCE="${1#--since=}"; shift ;;
        --manifest) ONLY_MANIFEST="$2"; shift 2 ;;
        --manifest=*) ONLY_MANIFEST="${1#--manifest=}"; shift ;;
        --quiet) QUIET=1; shift ;;
        -h|--help) sed -n '1,12p' "$0" | sed 's/^# \?//'; exit 0 ;;
        *) printf 'unknown flag: %s\n' "$1" >&2; exit 2 ;;
    esac
done

note() { [ "$QUIET" -ne 1 ] && printf '%s\n' "$*" || true; }
warn() { [ "$QUIET" -ne 1 ] && printf 'WARN: %s\n' "$*" >&2 || true; }

write_atomic() {
    local target="$1" mode="${2:-644}" dir tmp
    dir="$(dirname -- "$target")"
    mkdir -p "$dir"
    tmp="$(mktemp "$dir/.tmp.XXXXXX")"
    cat > "$tmp"
    chmod "$mode" "$tmp"
    mv "$tmp" "$target"
}

require_tooling() {
    if ! command -v jq >/dev/null 2>&1; then
        warn "jq not found; eval-verdict-compiler skipped"
        exit 0
    fi
}

resolve_artifact_path() {
    local path="$1"
    [ -z "$path" ] && { printf '\n'; return 0; }
    if [[ "$path" == /* ]]; then printf '%s\n' "$path"; return 0; fi
    if [[ "$path" == ~* ]]; then printf '%s\n' "${path/#\~/$HOME}"; return 0; fi
    printf '%s/%s\n' "$ROOT" "$path"
}

new_manifests_available() {
    [ -n "$ONLY_MANIFEST" ] && return 0
    if [ -d "$EVALS_ROOT/runs" ]; then
        if [ -r "$WATERMARK" ]; then
            find "$EVALS_ROOT/runs" -name 'manifest.json' -newer "$WATERMARK" 2>/dev/null | head -1 | grep -q . && return 0
        else
            find "$EVALS_ROOT/runs" -name 'manifest.json' 2>/dev/null | head -1 | grep -q . && return 0
        fi
    fi
    return 1
}

derive_artifacts_from_harness_lock() {
    local manifest="$1" run_dir lock
    run_dir="$(dirname "$manifest")"
    lock="$run_dir/harness.lock.json"
    if [ ! -r "$lock" ]; then printf '[]\n'; return 0; fi
    jq -c '[(.imports[]?.target // empty),(.files[]?.path // empty)]
        | map(select(test(".agents/(learnings|patterns|playbooks)/")))
        | unique' "$lock" 2>/dev/null || printf '[]\n'
}

read_learning_frontmatter() {
    awk '
        BEGIN { utility="0.5"; harmful="0"; reward="0"; in_fm=0 }
        NR==1 && $0=="---" { in_fm=1; next }
        in_fm && $0=="---" { in_fm=0; exit }
        in_fm && /^utility:/        { gsub(/^utility:[[:space:]]*|[[:space:]]+$/,""); utility=$0 }
        in_fm && /^harmful_count:/  { gsub(/^harmful_count:[[:space:]]*|[[:space:]]+$/,""); harmful=$0 }
        in_fm && /^reward_count:/   { gsub(/^reward_count:[[:space:]]*|[[:space:]]+$/,""); reward=$0 }
        END { printf "%s %s %s\n", utility, harmful, reward }
    ' "$1"
}

write_learning_frontmatter() {
    local path="$1" new_utility="$2" new_harmful="$3" new_reward="$4" last_verdict="$5"
    awk -v utility="$new_utility" -v harmful="$new_harmful" -v reward="$new_reward" -v last_verdict="$last_verdict" '
        BEGIN { in_fm=0; saw_u=0; saw_h=0; saw_r=0; saw_l=0 }
        NR==1 && $0=="---" { in_fm=1; print; next }
        in_fm && $0=="---" {
            if (!saw_u) printf "utility: %s\n", utility
            if (!saw_h) printf "harmful_count: %s\n", harmful
            if (!saw_r) printf "reward_count: %s\n", reward
            if (!saw_l) printf "last_verdict: %s\n", last_verdict
            in_fm=0; print; next
        }
        in_fm && /^utility:/        { printf "utility: %s\n", utility; saw_u=1; next }
        in_fm && /^harmful_count:/  { printf "harmful_count: %s\n", harmful; saw_h=1; next }
        in_fm && /^reward_count:/   { printf "reward_count: %s\n", reward; saw_r=1; next }
        in_fm && /^last_verdict:/   { printf "last_verdict: %s\n", last_verdict; saw_l=1; next }
        { print }
    ' "$path"
}

ema_update() {
    awk -v alpha="$EMA_ALPHA" -v beta="$EMA_BETA" -v old="$1" -v fresh="$2" \
        'BEGIN { printf "%.6f\n", alpha*old + beta*fresh }'
}

float_lt() { awk -v a="$1" -v b="$2" 'BEGIN { exit !(a+0.0 < b+0.0) }'; }
int_ge()   { [ "$1" -ge "$2" ] 2>/dev/null; }

mutate_one_artifact() {
    local artifact_path="$1" verdict_kind="$2" verdict_utility="$3" run_id="$4" seed_source="$5"
    if [ ! -f "$artifact_path" ]; then
        warn "  artifact not found, skipping: $artifact_path"
        return 0
    fi
    local triple old_utility old_harmful old_reward
    triple="$(read_learning_frontmatter "$artifact_path")"
    old_utility="$(printf '%s' "$triple" | awk '{print $1}')"
    old_harmful="$(printf '%s' "$triple" | awk '{print $2}')"
    old_reward="$(printf '%s'  "$triple" | awk '{print $3}')"

    local new_utility new_harmful new_reward
    new_utility="$(ema_update "$old_utility" "$verdict_utility")"
    new_harmful="$old_harmful"
    new_reward="$old_reward"
    case "$verdict_kind" in
        regressed) new_harmful="$((old_harmful + 1))" ;;
        improved)  new_reward="$((old_reward + 1))" ;;
    esac

    note "  mutate: $artifact_path"
    note "    utility: $old_utility -> $new_utility ($seed_source, kind=$verdict_kind)"
    note "    harmful_count: $old_harmful -> $new_harmful   reward_count: $old_reward -> $new_reward"

    [ "$DRY_RUN" -eq 1 ] && return 0

    write_learning_frontmatter "$artifact_path" "$new_utility" "$new_harmful" "$new_reward" "$run_id" \
        | write_atomic "$artifact_path"

    if int_ge "$new_harmful" "$HARMFUL_THRESHOLD" && float_lt "$new_utility" "$LOW_UTILITY_THRESHOLD"; then
        local entry
        entry="$(jq -nc \
            --arg title "Review and refactor verdict-flagged artifact: $artifact_path" \
            --arg type tech-debt \
            --arg severity medium \
            --arg source eval-verdict-compiler \
            --arg description "harmful_count=$new_harmful >= $HARMFUL_THRESHOLD AND utility=$new_utility < $LOW_UTILITY_THRESHOLD; verdict-driven retire candidate" \
            --arg target_repo agentops \
            --arg artifact_path "$artifact_path" \
            '{title:$title,type:$type,severity:$severity,source:$source,description:$description,target_repo:$target_repo,evidence:{artifact_path:$artifact_path}}')"
        mkdir -p "$ROOT/.agents/rpi"
        printf '%s\n' "$entry" >> "$ROOT/.agents/rpi/next-work.jsonl"
        note "    queued retire: $ROOT/.agents/rpi/next-work.jsonl"
    fi
}

process_manifest() {
    local manifest="$1" status verdict_kind verdict_utility artifacts seed_source run_id
    if [ ! -r "$manifest" ]; then warn "manifest not readable: $manifest"; return 0; fi
    status="$(jq -r '.status // ""' "$manifest" 2>/dev/null || echo "")"
    if [ "$status" != "complete" ]; then note "skip $manifest: status=$status (not complete)"; return 0; fi

    verdict_kind="$(jq -r '
        if (.verdict == null) then ""
        elif (.verdict | type) == "string" then .verdict
        else (.verdict.kind // "")
        end
    ' "$manifest" 2>/dev/null || echo "")"
    if [ -z "$verdict_kind" ] || [ "$verdict_kind" = "null" ]; then
        note "skip $manifest: no verdict"; return 0
    fi
    verdict_utility="$(jq -r '
        if (.verdict | type) == "string" then 0.5
        else (.verdict.utility // 0.5)
        end
    ' "$manifest")"
    artifacts="$(jq -c '
        if (.verdict | type) == "string" then []
        else (.verdict.applicable_artifacts // [])
        end
    ' "$manifest")"

    seed_source="manifest_explicit"
    if [ "$artifacts" = "[]" ]; then
        artifacts="$(derive_artifacts_from_harness_lock "$manifest")"
        seed_source="harness_lock"
    fi
    if [ "$artifacts" = "[]" ] || [ -z "$artifacts" ]; then
        note "skip $manifest: no applicable artifacts (no manifest array, no harness lock — by design on legacy runs)"
        return 0
    fi

    run_id="$(jq -r '.id // "unknown"' "$manifest")"
    note "process $manifest (run_id=$run_id, kind=$verdict_kind, utility=$verdict_utility, seed=$seed_source)"

    local count=0
    while IFS= read -r artifact; do
        local resolved
        resolved="$(resolve_artifact_path "$artifact")"
        [ -z "$resolved" ] && continue
        mutate_one_artifact "$resolved" "$verdict_kind" "$verdict_utility" "$run_id" "$seed_source"
        count=$((count + 1))
    done < <(printf '%s' "$artifacts" | jq -r '.[]')

    note "  total mutations for $run_id: $count"

    if [ "$DRY_RUN" -eq 0 ]; then
        local watermark_entry
        watermark_entry="$(jq -nc --arg manifest "$manifest" --arg run_id "$run_id" \
            --arg processed_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
            '{manifest:$manifest,run_id:$run_id,processed_at:$processed_at}')"
        mkdir -p "$EVALS_ROOT"
        printf '%s\n' "$watermark_entry" >> "$WATERMARK"
    fi
}

require_tooling

if [ -n "$ONLY_MANIFEST" ]; then
    process_manifest "$ONLY_MANIFEST"
    exit 0
fi

if ! new_manifests_available; then
    note "no new verdicts; skipping (corpus-active precondition)"
    exit 0
fi

while IFS= read -r m; do
    if [ -n "$SINCE" ]; then
        local_finished="$(jq -r '.finished_at_unix_ms // 0' "$m" 2>/dev/null || echo 0)"
        if [ "$local_finished" -lt "$SINCE" ]; then continue; fi
    fi
    process_manifest "$m"
done < <(find "$EVALS_ROOT/runs" -name 'manifest.json' -print 2>/dev/null | sort)

note "eval-verdict-compiler: done"
