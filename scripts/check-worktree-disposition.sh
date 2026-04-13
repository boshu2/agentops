#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${GIT_DIR:-}" && -z "${GIT_WORK_TREE:-}" ]]; then
    GIT_WORK_TREE="$(pwd -P)"
    export GIT_WORK_TREE
fi

repo_root="$(git rev-parse --show-toplevel)"
current_branch="$(git branch --show-current)"
common_dir="$(git rev-parse --git-common-dir)"

run_git_external() {
    local target_root="$1"
    shift

    local -a env_args=(env)
    while IFS= read -r var_name; do
        env_args+=("-u" "$var_name")
    done < <(git rev-parse --local-env-vars)

    "${env_args[@]}" git -C "$target_root" "$@"
}

porcelain_path() {
    local status_line="$1"
    local path="${status_line:3}"
    path="${path#* -> }"
    printf '%s\n' "$path"
}

is_gate_managed_path() {
    case "$1" in
        cli/docs/COMMANDS.md|cli/embedded/*|docs/ARCHITECTURE.md|docs/SKILLS.md|docs/cli-skills-map.md|PRODUCT.md|README.md|SKILL-TIERS.md|skills-codex/*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

print_dirty_diagnostics() {
    local target_root="$1"
    local dirty_status="$2"
    local line
    local path
    local -a gate_managed_paths=()
    local -a other_paths=()

    echo "FAIL: canonical root $target_root has uncommitted changes" >&2
    echo "Dirty paths from git status --porcelain:" >&2
    while IFS= read -r line; do
        [[ -n "$line" ]] || continue
        printf '  %s\n' "$line" >&2
        path="$(porcelain_path "$line")"
        if is_gate_managed_path "$path"; then
            gate_managed_paths+=("$path")
        else
            other_paths+=("$path")
        fi
    done <<<"$dirty_status"

    if (( ${#gate_managed_paths[@]} > 0 )); then
        echo "Generated/gate-managed paths detected:" >&2
        printf '  - %s\n' "${gate_managed_paths[@]}" >&2
    fi
    if (( ${#other_paths[@]} > 0 )); then
        echo "Other dirty paths detected:" >&2
        printf '  - %s\n' "${other_paths[@]}" >&2
    fi
    echo "Commit intentional changes; if a validation command generated these files, rerun the matching generator or restore them before pushing." >&2
}

trim_field() {
    local value="$1"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    printf '%s\n' "$value"
}

field_is_missing() {
    local value
    value="$(trim_field "$1")"
    [[ -z "$value" || "$value" == "-" || "$value" == "TODO" || "$value" == "TBD" || "$value" == "todo" || "$value" == "tbd" ]]
}

validate_preserved_refs() {
    local target_root="$1"
    local manifest_path="$target_root/docs/preserved-refs.tsv"
    local line
    local ref
    local owner
    local retirement_rule
    local _reason
    local -a preserved_refs=()
    local -a manifest_failures=()
    declare -A manifest_owners=()
    declare -A manifest_retirement_rules=()

    while IFS= read -r ref; do
        [[ -n "$ref" ]] || continue
        preserved_refs+=("$ref")
    done < <(
        run_git_external "$target_root" for-each-ref \
            --format='%(refname:short)' \
            'refs/heads/codex/preserve-*' \
            'refs/remotes/origin/codex/preserve-*' |
            sed 's#^origin/##' |
            sort -u
    )

    if (( ${#preserved_refs[@]} == 0 )); then
        return 0
    fi

    if [[ ! -f "$manifest_path" ]]; then
        echo "FAIL: preserved refs exist but $manifest_path is missing" >&2
        printf '  - %s\n' "${preserved_refs[@]}" >&2
        echo "Create docs/preserved-refs.tsv entries with ref, owner, and retirement_rule fields." >&2
        exit 1
    fi

    while IFS=$'\t' read -r ref owner retirement_rule _reason; do
        [[ -n "${ref:-}" ]] || continue
        [[ "$ref" == \#* ]] && continue

        ref="$(trim_field "$ref")"
        owner="$(trim_field "${owner:-}")"
        retirement_rule="$(trim_field "${retirement_rule:-}")"

        if field_is_missing "$ref" || field_is_missing "$owner" || field_is_missing "$retirement_rule"; then
            manifest_failures+=("${ref:-<blank>} has missing owner or retirement_rule")
            continue
        fi

        manifest_owners["$ref"]="$owner"
        manifest_retirement_rules["$ref"]="$retirement_rule"
    done <"$manifest_path"

    for ref in "${preserved_refs[@]}"; do
        if [[ -z "${manifest_owners[$ref]:-}" || -z "${manifest_retirement_rules[$ref]:-}" ]]; then
            manifest_failures+=("$ref is missing from docs/preserved-refs.tsv")
        fi
    done

    if (( ${#manifest_failures[@]} > 0 )); then
        printf 'FAIL: preserved refs need owner and retirement_rule entries:\n' >&2
        printf '  - %s\n' "${manifest_failures[@]}" >&2
        exit 1
    fi
}

if [[ "$common_dir" != /* ]]; then
    common_dir="$(cd "$repo_root" && cd "$common_dir" && pwd)"
fi

if [[ "$common_dir" == */.git ]]; then
    canonical_root="${common_dir%/.git}"
else
    canonical_root="${common_dir%%/.git/*}"
fi

if [[ -z "$current_branch" ]]; then
    echo "FAIL: current worktree is detached; run this gate from a branch-attached task worktree" >&2
    exit 1
fi

if [[ ! -d "$canonical_root" ]]; then
    echo "FAIL: canonical root not found: $canonical_root" >&2
    exit 1
fi

canonical_branch="$(run_git_external "$canonical_root" branch --show-current)"
if [[ -z "$canonical_branch" ]]; then
    echo "FAIL: canonical root $canonical_root is detached; it must stay on main" >&2
    exit 1
fi

if [[ "$canonical_branch" != "main" ]]; then
    echo "FAIL: canonical root $canonical_root is on $canonical_branch; expected main" >&2
    exit 1
fi

dirty_status="$(run_git_external "$canonical_root" status --porcelain --untracked-files=all)"
if [[ -n "$dirty_status" ]]; then
    print_dirty_diagnostics "$canonical_root" "$dirty_status"
    exit 1
fi

declare -A allowed_worktrees=()
allowed_worktrees["main"]="$canonical_root"
allowed_worktrees["$current_branch"]="$repo_root"

if [[ -n "${WORKTREE_DISPOSITION_EXTRA_ALLOWED_BRANCHES:-}" ]]; then
    IFS=',' read -r -a extra_branches <<<"$WORKTREE_DISPOSITION_EXTRA_ALLOWED_BRANCHES"
    for branch in "${extra_branches[@]}"; do
        branch="${branch//[[:space:]]/}"
        [[ -n "$branch" ]] || continue

        worktree_path="$(git for-each-ref --format='%(worktreepath)' "refs/heads/$branch")"
        if [[ -n "$worktree_path" ]]; then
            allowed_worktrees["$branch"]="$worktree_path"
        fi
    done
fi

failures=()
while IFS='|' read -r branch worktree_path; do
    [[ -n "$worktree_path" ]] || continue

    expected_path="${allowed_worktrees[$branch]:-}"
    if [[ -z "$expected_path" ]]; then
        failures+=("$branch attached at $worktree_path")
        continue
    fi

    if [[ "$worktree_path" != "$expected_path" ]]; then
        failures+=("$branch attached at $worktree_path (expected $expected_path)")
    fi
done < <(git for-each-ref --format='%(refname:short)|%(worktreepath)' refs/heads)

if (( ${#failures[@]} > 0 )); then
    printf 'FAIL: unexpected branch-attached worktrees detected:\n' >&2
    printf '  - %s\n' "${failures[@]}" >&2
    exit 1
fi

validate_preserved_refs "$canonical_root"

echo "PASS: canonical root $canonical_root is clean on main; current branch $current_branch is attached at $repo_root"
