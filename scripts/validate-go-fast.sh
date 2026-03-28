#!/usr/bin/env bash
set -euo pipefail

# validate-go-fast.sh
# Lightweight local Go gate: run race-enabled tests only for changed packages.
#
# Exit codes:
#   0 - pass or no Go changes
#   1 - failures / setup issues

SCOPE="auto"

usage() {
    cat <<'EOF'
Usage: scripts/validate-go-fast.sh [--scope auto|upstream|staged|worktree|head]

Options:
  --scope <mode>  How to choose files to inspect.
                  auto     prefer upstream diff, then staged/worktree fallback
                  upstream commits ahead of @{upstream}
                  staged   staged changes only
                  worktree unstaged + staged + untracked files
                  head     files from HEAD commit only
  -h, --help      Show this help
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --scope)
            SCOPE="${2:-}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown arg: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

case "$SCOPE" in
    auto|upstream|staged|worktree|head) ;;
    *)
        echo "Invalid --scope: $SCOPE" >&2
        usage >&2
        exit 2
        ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

if ! command -v go >/dev/null 2>&1; then
    echo "SKIP: go not installed"
    exit 0
fi

collect_target_files() {
    local scope="$1"
    local ahead_files=""

    if ! git rev-parse --git-dir >/dev/null 2>&1; then
        return 0
    fi

    case "$scope" in
        upstream)
            if git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' >/dev/null 2>&1; then
                git diff --name-only '@{upstream}...HEAD' 2>/dev/null || true
            fi
            ;;
        staged)
            git diff --name-only --cached 2>/dev/null || true
            ;;
        worktree)
            git diff --name-only --cached 2>/dev/null || true
            git diff --name-only 2>/dev/null || true
            git ls-files --others --exclude-standard 2>/dev/null || true
            ;;
        head)
            git show --name-only --pretty=format: HEAD 2>/dev/null || true
            ;;
        auto)
            if git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' >/dev/null 2>&1; then
                ahead_files="$(git diff --name-only '@{upstream}...HEAD' 2>/dev/null || true)"
                if [[ -n "$ahead_files" ]]; then
                    printf '%s\n' "$ahead_files"
                    return 0
                fi
            fi
            git diff --name-only --cached 2>/dev/null || true
            git diff --name-only 2>/dev/null || true
            git ls-files --others --exclude-standard 2>/dev/null || true
            ;;
    esac
}

find_module_root() {
    local path="$1"
    local dir="$path"
    if [[ ! -d "$dir" ]]; then
        dir="$(dirname "$dir")"
    fi
    dir="$(cd "$dir" 2>/dev/null && pwd -P 2>/dev/null || true)"
    while [[ -n "$dir" && "$dir" != "/" ]]; do
        if [[ -f "$dir/go.mod" ]]; then
            printf '%s\n' "$dir"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    return 1
}

collect_test_names_from_file() {
    local test_file="$1"
    [[ -f "$test_file" ]] || return 0

    awk '
        /^func (Test|Fuzz|Example)[A-Za-z0-9_]*\(/ {
            name = $2
            sub(/\(.*/, "", name)
            print name
        }
    ' "$test_file"
}

tmp_files="$(mktemp)"
tmp_pairs="$(mktemp)"
tmp_runs="$(mktemp)"
tmp_full="$(mktemp)"
trap 'rm -f "$tmp_files" "$tmp_pairs" "$tmp_runs" "$tmp_full"' EXIT

collect_target_files "$SCOPE" | sed '/^[[:space:]]*$/d' | sort -u > "$tmp_files"

if [[ ! -s "$tmp_files" ]]; then
    echo "SKIP: no changed files detected"
    exit 0
fi

go_changed=0

while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    abs_path="$REPO_ROOT/$file"

    case "$file" in
        *.go|go.mod|go.sum)
            go_changed=1
            ;;
        *)
            continue
            ;;
    esac

    module_root="$(find_module_root "$abs_path" || true)"
    [[ -z "$module_root" ]] && continue

    # Dependency changes are module-wide.
    if [[ "$file" == "go.mod" || "$file" == "go.sum" || "$file" == */go.mod || "$file" == */go.sum ]]; then
        printf '%s\t%s\n' "$module_root" "./..." >> "$tmp_pairs"
        continue
    fi

    # For changed go files, test only their package directory.
    dir_path="$(dirname "$abs_path")"
    dir_path="$(cd "$dir_path" 2>/dev/null && pwd -P 2>/dev/null || true)"
    [[ -z "$dir_path" ]] && continue

    rel="${dir_path#"$module_root"/}"
    if [[ "$dir_path" == "$module_root" ]]; then
        rel="."
    fi

    if [[ "$rel" == "." ]]; then
        printf '%s\t%s\n' "$module_root" "." >> "$tmp_pairs"
    else
        printf '%s\t%s\n' "$module_root" "./$rel" >> "$tmp_pairs"
    fi
done < "$tmp_files"

if [[ "$go_changed" -eq 0 ]]; then
    echo "SKIP: no Go changes in push scope"
    exit 0
fi

if [[ ! -s "$tmp_pairs" ]]; then
    echo "SKIP: Go changes detected but no resolvable module/package paths"
    exit 0
fi

tmp_unique="$(mktemp)"
trap 'rm -f "$tmp_files" "$tmp_pairs" "$tmp_runs" "$tmp_full" "$tmp_unique"' EXIT
sort -u "$tmp_pairs" > "$tmp_unique"

echo "Running lightweight Go race checks on changed scope..."

# Duration monitoring: warn if any package exceeds this threshold (seconds).
SLOW_THRESHOLD_SECS="${SLOW_THRESHOLD_SECS:-45}"

tmp_json="$(mktemp)"
trap 'rm -f "$tmp_files" "$tmp_pairs" "$tmp_runs" "$tmp_full" "$tmp_unique" "$tmp_json"' EXIT

race_exit=0
fallback_hits=0

while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    abs_path="$REPO_ROOT/$file"

    case "$file" in
        *.go|go.mod|go.sum)
            ;;
        *)
            continue
            ;;
    esac

    module_root="$(find_module_root "$abs_path" || true)"
    [[ -z "$module_root" ]] && continue

    if [[ "$file" == "go.mod" || "$file" == "go.sum" || "$file" == */go.mod || "$file" == */go.sum ]]; then
        continue
    fi

    dir_path="$(dirname "$abs_path")"
    dir_path="$(cd "$dir_path" 2>/dev/null && pwd -P 2>/dev/null || true)"
    [[ -z "$dir_path" ]] && continue

    rel="${dir_path#"$module_root"/}"
    pattern="."
    if [[ "$dir_path" != "$module_root" ]]; then
        pattern="./$rel"
    fi

    candidate_test=""
    if [[ "$file" == *_test.go ]]; then
        candidate_test="$abs_path"
    else
        sibling_test="${abs_path%.go}_test.go"
        if [[ -f "$sibling_test" ]]; then
            candidate_test="$sibling_test"
        fi
    fi

    if [[ -n "$candidate_test" ]]; then
        while IFS= read -r test_name; do
            [[ -n "$test_name" ]] || continue
            printf '%s\t%s\t%s\n' "$module_root" "$pattern" "$test_name" >> "$tmp_runs"
        done < <(collect_test_names_from_file "$candidate_test")
    else
        printf '%s\t%s\n' "$module_root" "$pattern" >> "$tmp_full"
    fi
done < "$tmp_files"

while IFS=$'\t' read -r module_root pattern; do
    [[ -z "$module_root" ]] && continue
    race_exit=0

    echo ""
    echo "module: ${module_root#"$REPO_ROOT"/}"
    echo "package: $pattern"

    run_filter=""
    if ! awk -F '\t' -v m="$module_root" -v p="$pattern" '$1 == m && $2 == p {found=1} END {exit(found ? 0 : 1)}' "$tmp_full"; then
        mapfile -t test_names < <(awk -F '\t' -v m="$module_root" -v p="$pattern" '$1 == m && $2 == p {print $3}' "$tmp_runs" | sort -u)
        if [[ "${#test_names[@]}" -gt 0 ]]; then
            escaped_names=()
            for test_name in "${test_names[@]}"; do
                escaped_names+=("$(printf '%s' "$test_name" | sed 's/[][(){}.^$+*?|\\]/\\&/g')")
            done
            regex_body="$(printf '%s\n' "${escaped_names[@]}" | paste -sd'|' -)"
            run_filter="^(${regex_body})$"
            echo "tests: ${test_names[*]}"
        fi
    fi

    (
        cd "$module_root"
        if [[ -n "$run_filter" ]]; then
            go test -race -count=1 -json -run "$run_filter" "$pattern" 2>&1
        else
            go test -race -count=1 -json "$pattern" 2>&1
        fi
    ) > "$tmp_json" || race_exit=$?

    # CI/provisioning fallback: retry serially when the first run fails due to
    # fork/resource limits rather than test logic failures.
    if [[ "$race_exit" -ne 0 ]] && grep -Eqi 'resource temporarily unavailable|cannot allocate memory|failed to create new os thread|newosproc|fork/exec' "$tmp_json"; then
        echo "  INFO: fork/resource failure detected; retrying serial mode (-p 1)"
        if (
            cd "$module_root"
            if [[ -n "$run_filter" ]]; then
                go test -race -count=1 -json -p 1 -run "$run_filter" "$pattern" 2>&1
            else
                go test -race -count=1 -json -p 1 "$pattern" 2>&1
            fi
        ) > "$tmp_json"; then
            race_exit=0
            fallback_hits=$((fallback_hits + 1))
        else
            race_exit=$?
        fi
    fi

    # Display human-readable summary from JSON output.
    # Uses awk instead of jq to avoid forking external processes — jq caused
    # "fork: Resource temporarily unavailable" on large race-json output (ag-m7x).
    # go test -json emits one JSON object per line, so simple string extraction works.
    if [[ -s "$tmp_json" ]]; then
        awk -v slow="$SLOW_THRESHOLD_SECS" '
        /DATA RACE/ { data_race=1 }
        /"Test"/ { next }
        /"Action"/ && /"Package"/ && /"Elapsed"/ {
            line = $0
            # Extract Action value
            sub(/.*"Action"[[:space:]]*:[[:space:]]*"/, "", line); sub(/".*/, "", line)
            action = line
            if (action != "pass" && action != "fail") next
            # Extract Package value
            line = $0
            sub(/.*"Package"[[:space:]]*:[[:space:]]*"/, "", line); sub(/".*/, "", line)
            pkg = line
            # Extract Elapsed value
            line = $0
            sub(/.*"Elapsed"[[:space:]]*:[[:space:]]*/, "", line); sub(/[,}[:space:]].*/, "", line)
            elapsed = line + 0
            label = (action == "fail") ? "FAIL" : "ok"
            printf "  %-4s  %-60s  %.1fs\n", label, pkg, elapsed
            if (elapsed >= slow + 0)
                printf "  WARNING: %s took %.1fs (threshold: %ds)\n", pkg, elapsed, slow + 0
        }
        END { if (data_race) print "\n  WARNING: DATA RACE detected — see full output above" }
        ' "$tmp_json"
    fi

    if [[ "$race_exit" -ne 0 ]]; then
        # Print raw output lines for failed tests to aid debugging.
        # Uses awk instead of jq to avoid fork exhaustion.
        echo ""
        echo "--- failure output ---"
        awk '/"Action"[[:space:]]*:[[:space:]]*"output"/ {
            line = $0
            sub(/.*"Output"[[:space:]]*:[[:space:]]*"/, "", line)
            sub(/"[[:space:]]*[,}][[:space:]]*$/, "", line)
            gsub(/\\n/, "\n", line)
            gsub(/\\t/, "\t", line)
            printf "%s", line
        }' "$tmp_json"
        echo "--- end failure output ---"
        exit 1
    fi
done < "$tmp_unique"

echo ""
if [[ "$fallback_hits" -gt 0 ]]; then
    echo "INFO: serial fallback used for $fallback_hits module(s)"
fi
echo "PASS: lightweight Go race checks succeeded"
