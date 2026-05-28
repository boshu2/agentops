#!/bin/bash
#
# check-skill-isolation.sh
#
# Lint phase-skill SKILL.md files for compression patterns that violate the
# phase-isolation contract declared in PRODUCT.md operational principle #5
# and documented at skills/rpi/references/isolation-contract.md.
#
# Compression patterns flagged:
#   1. Cross-phase first-person verbs:
#        "I will research|plan|crank|validate"
#   2. Inline research vocabulary near phase context:
#        "let me grep|read|search"  /  "I'll grep|read|search"
#   3. A phase-skill SKILL.md calling another phase skill it should not orchestrate.
#        Per-file allowlist:
#          rpi/SKILL.md         may call: discovery, crank, validation
#          discovery/SKILL.md   may call: research, plan
#          crank/SKILL.md       may NOT call: research, plan, crank, validation
#          validation/SKILL.md  may NOT call: research, plan, crank, validation
#
# False-positive guard:
#   - Lines beginning with `See [`              (markdown reference links)
#   - Lines beginning with `Read <path>`        (reference doc reads)
#   - Lines inside fenced code blocks (``` ... ```)
#
# Usage:
#   check-skill-isolation.sh                # lint default tree
#   check-skill-isolation.sh <path>         # lint a different skills/ tree
#   check-skill-isolation.sh -q             # quiet mode, exit code only
#   check-skill-isolation.sh --self-test    # internal regression check
#
# Exit codes:
#   0 = clean (no compression patterns matched)
#   1 = at least one compression pattern matched
#   2 = script error (bad invocation, missing files)

set -uo pipefail

QUIET=0
SELF_TEST=0
TARGET_PATH=""

for arg in "$@"; do
    case "$arg" in
        -q|--quiet)
            QUIET=1
            ;;
        --self-test)
            SELF_TEST=1
            ;;
        -h|--help)
            sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        -*)
            echo "check-skill-isolation: unknown flag: $arg" >&2
            exit 2
            ;;
        *)
            if [[ -z "$TARGET_PATH" ]]; then
                TARGET_PATH="$arg"
            else
                echo "check-skill-isolation: unexpected extra argument: $arg" >&2
                exit 2
            fi
            ;;
    esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_ROOT="${REPO_ROOT}/skills"

emit() {
    if [[ $QUIET -eq 0 ]]; then
        echo "$@" >&2
    fi
}

# Resolve the four phase-skill SKILL.md files under a given root.
# Echoes one path per line for files that exist.
resolve_phase_files() {
    local root="$1"
    local name
    for name in rpi discovery crank validation; do
        local f="${root}/${name}/SKILL.md"
        if [[ -f "$f" ]]; then
            echo "$f"
        fi
    done
}

# Per-file Skill() callsite check.
# Returns 0 if the line is an allowed callsite for this file, 1 if it's a violation.
# $1 = basename-of-parent-dir (rpi|discovery|crank|validation)
# $2 = sub-skill name captured from the line (research|plan|crank|validation)
is_skill_call_allowed() {
    local owner="$1"
    local target="$2"
    case "$owner" in
        rpi)
            # rpi orchestrates discovery, crank, validation.
            # research/plan are discovery's sub-skills, not rpi's — flag those.
            case "$target" in
                crank|validation) return 0 ;;
                *) return 1 ;;
            esac
            ;;
        discovery)
            # discovery orchestrates research and plan.
            case "$target" in
                research|plan) return 0 ;;
                *) return 1 ;;
            esac
            ;;
        crank|validation)
            # phase 2 and phase 3 are sealed — they should not call any of the watched phase skills.
            return 1
            ;;
        *)
            return 1
            ;;
    esac
}

# Lint a single SKILL.md file. Echoes diagnostics to stderr (when not quiet).
# Sets the global `violations` count via stdout (one line per violation, "file:lineno:pattern").
lint_file() {
    local file="$1"
    local owner
    owner="$(basename "$(dirname "$file")")"

    awk -v file="$file" -v owner="$owner" '
        BEGIN {
            in_fence = 0
        }
        # Toggle fenced-code state. A line whose first non-space chars are ``` flips state.
        {
            line = $0
            stripped = line
            sub(/^[ \t]*/, "", stripped)
            if (substr(stripped, 1, 3) == "```") {
                in_fence = 1 - in_fence
                next
            }
        }
        # Skip lines inside fenced code blocks.
        in_fence == 1 { next }
        # False-positive guard: markdown reference link lines.
        /^See \[/ { next }
        # False-positive guard: reference-doc read instructions.
        /^Read [^[:space:]]+/ { next }
        # Pattern 1: cross-phase first-person verbs.
        {
            if (match(tolower(line), /i will (research|plan|crank|validate)/)) {
                printf("%s:%d:cross-phase first-person verb: %s\n", file, NR, line) > "/dev/stderr"
                printf("%s\t%d\tcross-phase-verb\n", file, NR)
            }
        }
        # Pattern 2: inline research vocabulary.
        {
            lc = tolower(line)
            if (match(lc, /let me (grep|read|search)/) ||
                match(lc, /i.ll (grep|read|search)/)) {
                printf("%s:%d:inline research vocabulary: %s\n", file, NR, line) > "/dev/stderr"
                printf("%s\t%d\tinline-research\n", file, NR)
            }
        }
        # Pattern 3: phase-skill calling another phase skill.
        # Capture the target sub-skill name and let the caller validate the allowlist.
        {
            if (match(line, /Skill\(skill="(research|plan|crank|validation)"/, m)) {
                printf("%s\t%d\tskill-call\t%s\n", file, NR, m[1])
            }
        }
    ' "$file"
}

run_lint() {
    local root="$1"
    local violations=0

    if [[ ! -d "$root" ]]; then
        echo "check-skill-isolation: target path is not a directory: $root" >&2
        return 2
    fi

    local files
    mapfile -t files < <(resolve_phase_files "$root")

    if [[ ${#files[@]} -eq 0 ]]; then
        # No phase-skill SKILL.md files under this root. Nothing to lint.
        # This is not an error — callers may pass a tree intended to test
        # specific files only. Emit a debug note and return clean.
        emit "check-skill-isolation: no phase-skill SKILL.md files found under $root"
        return 0
    fi

    local file
    for file in "${files[@]}"; do
        local raw
        raw="$(lint_file "$file")"
        if [[ -z "$raw" ]]; then
            continue
        fi

        # Each output line is tab-separated:
        #   file<TAB>lineno<TAB>kind[<TAB>extra]
        # kind in {cross-phase-verb, inline-research, skill-call}
        local line
        while IFS= read -r line; do
            [[ -z "$line" ]] && continue

            local kind
            kind="$(echo "$line" | cut -f3)"
            local extra
            extra="$(echo "$line" | cut -f4)"
            local hit_file
            hit_file="$(echo "$line" | cut -f1)"
            local hit_lineno
            hit_lineno="$(echo "$line" | cut -f2)"
            local owner
            owner="$(basename "$(dirname "$hit_file")")"

            case "$kind" in
                cross-phase-verb|inline-research)
                    violations=$((violations + 1))
                    ;;
                skill-call)
                    if is_skill_call_allowed "$owner" "$extra"; then
                        # Legitimate orchestration callsite — no violation.
                        :
                    else
                        emit "$hit_file:$hit_lineno:phase-skill calling another phase skill (target=$extra)"
                        violations=$((violations + 1))
                    fi
                    ;;
            esac
        done <<< "$raw"
    done

    if [[ $violations -gt 0 ]]; then
        emit ""
        emit "check-skill-isolation: FAIL ($violations compression pattern(s) found)"
        emit ""
        emit "See skills/rpi/references/isolation-contract.md for the rules."
        return 1
    fi

    if [[ $QUIET -eq 0 ]]; then
        echo "check-skill-isolation: PASS (no compression patterns in phase-skill SKILL.md files under $root)"
    fi
    return 0
}

SELF_TEST_TMP=""
self_test_cleanup() {
    if [[ -n "${SELF_TEST_TMP:-}" && -d "${SELF_TEST_TMP:-}" ]]; then
        rm -rf "$SELF_TEST_TMP"
    fi
}

self_test() {
    # Build a tmpdir mimicking skills/<phase>/SKILL.md, inject a known violation,
    # run the lint, and assert non-zero exit.
    SELF_TEST_TMP="$(mktemp -d)"
    trap self_test_cleanup EXIT

    mkdir -p "$SELF_TEST_TMP/discovery"
    cat > "$SELF_TEST_TMP/discovery/SKILL.md" <<'EOF'
---
name: discovery
---
# /discovery

I will research the codebase before doing anything else.
EOF

    if "$0" --quiet "$SELF_TEST_TMP"; then
        echo "check-skill-isolation: SELF-TEST FAILED — lint did not catch injected violation" >&2
        return 1
    fi

    echo "check-skill-isolation: self-test PASS"
    return 0
}

if [[ $SELF_TEST -eq 1 ]]; then
    self_test
    exit $?
fi

if [[ -z "$TARGET_PATH" ]]; then
    TARGET_PATH="$DEFAULT_ROOT"
fi

run_lint "$TARGET_PATH"
exit $?
