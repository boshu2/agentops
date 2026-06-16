#!/bin/bash
#
# check-skill-isolation.sh
#
# Lint SKILL.md files for compression patterns that violate the
# phase-isolation contract declared in PRODUCT.md operational principle #5
# and documented at skills/rpi/references/isolation-contract.md.
#
# Compression patterns flagged:
#   1. Cross-phase first-person verbs:
#        "I will research|plan|crank|validate"
#   2. Inline research vocabulary near phase context:
#        "let me grep|read|search"  /  "I'll grep|read|search"
#   3. A phase SKILL.md calling another phase skill it should not orchestrate.
#        Per-file allowlist:
#          rpi/SKILL.md         may call: discovery, crank, validate
#          discovery/SKILL.md   may call: research, plan
#          crank/SKILL.md       may NOT call: research, plan, crank, validate
#          validate/SKILL.md    may NOT call: research, plan, crank, validate
#
# Inventory-only lanes:
#   - flywheel, gold, wiki, and BC1 corpus skills are reported but never counted
#     as failures. See bead ag-skill-isolation-ci-gate-jxpbx.
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
#   0 = clean (no enforcing compression patterns matched)
#   1 = at least one enforcing compression pattern matched
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

# Resolve skill SKILL.md files under a given root.
# Echoes one path per line for files that exist.
resolve_skill_files() {
    local root="$1"
    local f
    for f in "$root"/*/SKILL.md; do
        if [[ -f "$f" ]]; then
            echo "$f"
        fi
    done
}

is_phase_skill() {
    local owner="$1"
    case "$owner" in
        rpi|discovery|crank|validate|validation) return 0 ;;
        *) return 1 ;;
    esac
}

is_inventory_only_skill() {
    local owner="$1"
    case "$owner" in
        # Explicit Mossy Lantern lanes named in ag-skill-isolation-ci-gate-jxpbx.
        flywheel|gold|wiki|corpus|*flywheel*|*gold*|*wiki*|*corpus*)
            return 0
            ;;
        # BC1 Corpus skills from docs/reference/agentops-skill-domain-map.md.
        cass|compile|curate|forge|handoff|inject|operationalize|recover|research|toil-mining)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Per-file Skill() callsite check.
# Returns 0 if the line is an allowed callsite for this file, 1 if it's a violation.
# $1 = basename-of-parent-dir (rpi|discovery|crank|validate|validation)
# $2 = sub-skill name captured from the line (research|plan|crank|validation)
is_skill_call_allowed() {
    local owner="$1"
    local target="$2"
    case "$owner" in
        rpi)
            # rpi orchestrates discovery, crank, validate.
            # research/plan are discovery's sub-skills, not rpi's — flag those.
            case "$target" in
                discovery|crank|validate|validation) return 0 ;;
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
        crank|validate|validation)
            # phase 2 and phase 3 are sealed — they should not call any of the watched phase skills.
            return 1
            ;;
        *)
            return 1
            ;;
    esac
}

# Lint a single SKILL.md file.
# Emits tab-separated matches on stdout:
#   file<TAB>lineno<TAB>kind<TAB>extra
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
                printf("%s\t%d\tcross-phase-verb\t%s\n", file, NR, line)
            }
        }
        # Pattern 2: inline research vocabulary.
        {
            lc = tolower(line)
            if (match(lc, /let me (grep|read|search)/) ||
                match(lc, /i.ll (grep|read|search)/)) {
                printf("%s\t%d\tinline-research\t%s\n", file, NR, line)
            }
        }
        # Pattern 3: phase-skill calling another phase skill.
        # Capture the target sub-skill name and let the caller validate the allowlist.
        {
            if (match(line, /Skill\(skill="(discovery|research|plan|crank|validate|validation)"/)) {
                skill_call = substr(line, RSTART, RLENGTH)
                sub(/^Skill\(skill="/, "", skill_call)
                sub(/"$/, "", skill_call)
                printf("%s\t%d\tskill-call\t%s\n", file, NR, skill_call)
            }
        }
    ' "$file"
}

emit_match() {
    local hit_file="$1"
    local hit_lineno="$2"
    local kind="$3"
    local extra="$4"
    local prefix="${5:-}"

    case "$kind" in
        cross-phase-verb)
            emit "$hit_file:$hit_lineno:${prefix}cross-phase first-person verb: $extra"
            ;;
        inline-research)
            emit "$hit_file:$hit_lineno:${prefix}inline research vocabulary: $extra"
            ;;
        skill-call)
            emit "$hit_file:$hit_lineno:${prefix}phase-skill calling another phase skill (target=$extra)"
            ;;
    esac
}

run_lint() {
    local root="$1"
    local violations=0
    local inventory_only_hits=0

    if [[ ! -d "$root" ]]; then
        echo "check-skill-isolation: target path is not a directory: $root" >&2
        return 2
    fi

    local file
    local found_files=0
    while IFS= read -r file; do
        found_files=1
        local raw
        if ! raw="$(lint_file "$file")"; then
            emit "check-skill-isolation: script error while linting $file"
            return 2
        fi
        if [[ -z "$raw" ]]; then
            continue
        fi

        # Each output line is tab-separated:
        #   file<TAB>lineno<TAB>kind<TAB>extra
        # kind in {cross-phase-verb, inline-research, skill-call}
        local line
        while IFS= read -r line; do
            [[ -z "$line" ]] && continue

            local hit_file hit_lineno kind extra rest
            IFS=$'\t' read -r hit_file hit_lineno kind extra rest <<< "$line"
            if [[ -n "${rest:-}" ]]; then
                extra="${extra}"$'\t'"${rest}"
            fi
            local owner
            owner="$(basename "$(dirname "$hit_file")")"

            if is_inventory_only_skill "$owner"; then
                emit_match "$hit_file" "$hit_lineno" "$kind" "$extra" "inventory-only: "
                inventory_only_hits=$((inventory_only_hits + 1))
                continue
            fi

            case "$kind" in
                cross-phase-verb|inline-research)
                    emit_match "$hit_file" "$hit_lineno" "$kind" "$extra"
                    violations=$((violations + 1))
                    ;;
                skill-call)
                    if ! is_phase_skill "$owner"; then
                        # Non-phase skills may mention phase Skill(...) callsites
                        # without violating this narrow phase-isolation check.
                        :
                    elif is_skill_call_allowed "$owner" "$extra"; then
                        # Legitimate orchestration callsite — no violation.
                        :
                    else
                        emit_match "$hit_file" "$hit_lineno" "$kind" "$extra"
                        violations=$((violations + 1))
                    fi
                    ;;
            esac
        done <<< "$raw"
    done < <(resolve_skill_files "$root")

    if [[ $found_files -eq 0 ]]; then
        # No SKILL.md files under this root. Nothing to lint.
        # This is not an error — callers may pass a tree intended to test
        # specific files only. Emit a debug note and return clean.
        emit "check-skill-isolation: no SKILL.md files found under $root"
        return 0
    fi

    if [[ $violations -gt 0 ]]; then
        emit ""
        emit "check-skill-isolation: FAIL ($violations enforcing compression pattern(s) found; $inventory_only_hits inventory-only hit(s))"
        emit ""
        emit "See skills/rpi/references/isolation-contract.md for the rules."
        return 1
    fi

    if [[ $QUIET -eq 0 ]]; then
        if [[ $inventory_only_hits -gt 0 ]]; then
            echo "check-skill-isolation: PASS (no enforcing compression patterns; $inventory_only_hits inventory-only hit(s) under $root)"
        else
            echo "check-skill-isolation: PASS (no compression patterns in SKILL.md files under $root)"
        fi
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
    # Build a tmpdir mimicking skills/<phase>/SKILL.md, inject known violations,
    # run the lint, and assert the lint failure exit specifically. Exit 2 is a
    # script/dialect error and must not count as a passing self-test.
    SELF_TEST_TMP="$(mktemp -d)"
    trap self_test_cleanup EXIT

    mkdir -p "$SELF_TEST_TMP/crank" "$SELF_TEST_TMP/discovery"
    cat > "$SELF_TEST_TMP/crank/SKILL.md" <<'EOF'
---
name: crank
---
# /crank

Skill(skill="research", args="inline scope")
EOF

    cat > "$SELF_TEST_TMP/discovery/SKILL.md" <<'EOF'
---
name: discovery
---
# /discovery

I will research the codebase before doing anything else.
EOF

    "$0" --quiet "$SELF_TEST_TMP"
    local rc=$?
    if [[ $rc -ne 1 ]]; then
        echo "check-skill-isolation: SELF-TEST FAILED — expected lint exit 1 for injected violations, got $rc" >&2
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
