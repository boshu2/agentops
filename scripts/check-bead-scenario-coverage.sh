#!/usr/bin/env bash
# check-bead-scenario-coverage.sh — leaf-validator scenario→test coverage check.
#
# The C2 "tests-from-scenarios" leaf gate (ag-9jle.4). Sibling — but NOT a
# duplicate — of scripts/check-scenario-test-linkage.sh:
#
#   check-scenario-test-linkage.sh   CI/corpus gate. Walks EVERY .feature under
#                                    skills/*/references/ and asserts each has a
#                                    @covered-by tag or is allowlisted. Static,
#                                    repo-wide, file-existence only.
#
#   check-bead-scenario-coverage.sh  LEAF gate (this script). Runs at /vibe and
#   (this file)                      /test time on the ONE slice/bead being
#                                    validated. Takes the bead's scenarios
#                                    (a .feature file, a bead body containing a
#                                    `## Scenarios` block, or stdin), and FAILS
#                                    if any scenario lacks a covering test. With
#                                    --run it requires the covering test to
#                                    actually PASS, not merely exist — the
#                                    distinction ag-9jle.4 calls out: leaf
#                                    validators must not accept "tests exist" or
#                                    a coverage %, they must prove every scenario
#                                    maps to a passing test.
#
# This is the FORWARD direction: input = behavior (Gherkin scenarios), the gate
# demands a test per scenario. It does not work backward from coverage gaps.
#
# Convention (identical to the corpus gate, so a scenario authored once carries
# its linkage to both gates):
#
#   @covered-by:<test-path>            tag the whole source as the cover, OR
#   @covered-by:<test-path>::<Name>    name a specific test function (Go etc.)
#
# Place the tag on its own line directly above the `Scenario:` it covers (the
# standard Gherkin tag position). Tags above `Feature:` (in a .feature file) or
# above the `## Scenarios` heading (in a bead body) apply to every scenario.
#
# Coverage gate: PASS iff M >= N where N = scenarios found and M = scenarios that
# resolve to an existing (and, with --run, passing) test. Override the threshold
# with --min-covered=<int> to require at least that many covered (default = N,
# i.e. all scenarios must be covered).
#
# Admission gate (--admission, ag-iruq3.1): plan-time structural check, run
# BEFORE any tests exist. PASS iff the body carries >= 1 scenario unit and every
# unit is structurally complete. A unit is EITHER
#   (a) a `Scenario:` / `Scenario Outline:` block — ends only on the next
#       Scenario, a `## ` section exit, or EOF (a blank line inside it does
#       NOT split it), OR
#   (b) a contiguous bare-GWT stanza: consecutive lines matching
#       ^(Given|When|Then|And|But)\b (case-sensitive), split by a blank or
#       non-GWT line — stanza splitting applies only when no Scenario block
#       is open.
# Per unit: >= 1 Given AND >= 1 Then required; a missing When is a WARN on
# stderr, not a failure. @covered-by resolution and the coverage threshold are
# skipped. Fenced (```) content is parse-inert. Empty input under --admission
# is an infra failure (exit 2), never a policy rejection. --admission with
# --run is misuse (exit 2).
#
# Usage:
#   bash scripts/check-bead-scenario-coverage.sh <source>        # .feature or bead-body file
#   bash scripts/check-bead-scenario-coverage.sh -                # read source from stdin
#   bash scripts/check-bead-scenario-coverage.sh --bead <id>      # fetch bead body via `br show`
#                                                                 # (BEADS_DIR defaults to <repo>/_beads)
#   bash scripts/check-bead-scenario-coverage.sh --run <source>   # also EXECUTE each test, require pass
#   bash scripts/check-bead-scenario-coverage.sh --admission <source>  # plan-time structural admission
#   bash scripts/check-bead-scenario-coverage.sh --min-covered=N <source>
#   bash scripts/check-bead-scenario-coverage.sh --json <source>  # machine-readable summary
#   bash scripts/check-bead-scenario-coverage.sh --warn-only <source>
#
# Exits 0 on pass, 1 on fail (unless --warn-only), 2 on misuse/infra failure.
#
# practices: [continuous-integration, design-by-contract, tdd, bdd-gherkin]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WARN_ONLY=0
JSON=0
RUN=0
ADMISSION=0
MIN_COVERED=""   # empty = "all scenarios" (=N)
SOURCE=""
BEAD_ID=""

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --warn-only) WARN_ONLY=1; shift;;
        --json)      JSON=1; shift;;
        --run)       RUN=1; shift;;
        --admission) ADMISSION=1; shift;;
        --min-covered=*) MIN_COVERED="${1#--min-covered=}"; shift;;
        --bead)      BEAD_ID="${2:-}"; shift 2;;
        -h|--help)   usage; exit 0;;
        -)           SOURCE="-"; shift;;
        --*)         echo "Unknown flag: $1" >&2; exit 2;;
        *)           SOURCE="$1"; shift;;
    esac
done

if [[ -n "$MIN_COVERED" && ! "$MIN_COVERED" =~ ^[0-9]+$ ]]; then
    echo "--min-covered must be a non-negative integer" >&2
    exit 2
fi

if [[ $ADMISSION -eq 1 && $RUN -eq 1 ]]; then
    echo "--admission cannot be combined with --run (admission is plan-time; tests don't exist yet)" >&2
    exit 2
fi

# Resolve the raw scenarios text into $RAW.
RAW=""
if [[ -n "$BEAD_ID" ]]; then
    if ! command -v br >/dev/null 2>&1; then
        echo "--bead given but br is not on PATH" >&2
        exit 2
    fi
    RAW="$(BEADS_DIR="${BEADS_DIR:-$REPO_ROOT/_beads}" br show "$BEAD_ID" 2>/dev/null || true)"
    if [[ -z "$RAW" ]]; then
        echo "br show $BEAD_ID returned no content" >&2
        exit 2
    fi
elif [[ "$SOURCE" == "-" ]]; then
    RAW="$(cat)"
elif [[ -n "$SOURCE" ]]; then
    if [[ ! -f "$SOURCE" ]]; then
        echo "source file does not exist: $SOURCE" >&2
        exit 2
    fi
    RAW="$(cat "$SOURCE")"
else
    echo "no source given (pass a .feature/bead-body file, '-' for stdin, or --bead <id>)" >&2
    exit 2
fi

# Admission (F5): empty input is an infra failure (tracker hiccup, broken
# pipe), never a policy rejection — exit 2, not 1.
if [[ $ADMISSION -eq 1 && -z "${RAW//[[:space:]]/}" ]]; then
    echo "no content (tracker failure?) — refusing to grade emptiness as inadmissible" >&2
    exit 2
fi

# Execute a covering test. Dispatches by extension. Optional $2 = test name to
# scope the run. Returns the test runner's exit code.
run_test() {
    local abs="$1" name="${2:-}"
    case "$abs" in
        *_test.go)
            local dir; dir="$(dirname "$abs")"
            if [[ -n "$name" ]]; then
                ( cd "$dir" && go test -run "$name" . )
            else
                ( cd "$dir" && go test . )
            fi
            ;;
        *.bats)
            if [[ -n "$name" ]]; then
                bats -f "$name" "$abs"
            else
                bats "$abs"
            fi
            ;;
        *.sh)
            bash "$abs"
            ;;
        *.py)
            if [[ -n "$name" ]]; then
                python3 -m pytest -q -k "$name" "$abs"
            else
                python3 -m pytest -q "$abs"
            fi
            ;;
        *)
            # Unknown runner: presence already verified by the caller; treat as covered.
            return 0
            ;;
    esac
}

# Resolve a @covered-by target. Echoes an error string, or empty on success.
# With RUN=1, also executes the test and requires exit 0.
# Arg: target spec after "@covered-by:" — "path" or "path::Name".
resolve_target() {
    local target="$1" path name abs
    if [[ "$target" == *"::"* ]]; then
        path="${target%%::*}"
        name="${target##*::}"
    else
        path="$target"
        name=""
    fi

    abs="$REPO_ROOT/$path"
    if [[ ! -f "$abs" ]]; then
        printf 'test path does not exist: %s' "$path"
        return
    fi
    if [[ -n "$name" ]]; then
        if ! grep -qF "$name" "$abs"; then
            printf 'test "%s" not found in %s' "$name" "$path"
            return
        fi
    fi

    if [[ $RUN -eq 1 ]]; then
        local rc=0
        run_test "$abs" "$name" >/dev/null 2>&1 || rc=$?
        if [[ $rc -ne 0 ]]; then
            printf 'covering test did not pass: %s (exit %d)' "$path" "$rc"
            return
        fi
    fi
    printf ''  # success
}

errors=0
scenarios_total=0
scenarios_covered=0
declare -a ERROR_LINES=()
declare -a UNCOVERED=()

# --admission unit state. A unit = Scenario:/Scenario Outline: block OR a
# contiguous bare-GWT stanza. Complete = >=1 Given AND >=1 Then.
units_total=0
units_complete=0
admission_warnings=0
unit_open=0
unit_kind=""     # "scenario" | "stanza"
unit_name=""
unit_given=0
unit_when=0
unit_then=0

# Flush the open admission unit's verdict (F7). Called on next-unit start, on
# `## ` section exit, AND at EOF — a unit must never lose its verdict because
# the body ended.
flush_admission_unit() {
    if [[ $unit_open -eq 0 ]]; then
        return 0
    fi
    units_total=$((units_total + 1))
    if [[ $unit_given -ge 1 && $unit_then -ge 1 ]]; then
        units_complete=$((units_complete + 1))
    else
        ERROR_LINES+=("unit \"$unit_name\": structurally incomplete — needs >=1 Given and >=1 Then (got Given=$unit_given Then=$unit_then)")
        errors=$((errors + 1))
    fi
    if [[ $unit_when -eq 0 ]]; then
        # N3: missing When is advice, not rejection — WARN, exit stays 0.
        echo "WARN: unit \"$unit_name\" has no When line" >&2
        admission_warnings=$((admission_warnings + 1))
    fi
    unit_open=0; unit_kind=""; unit_name=""; unit_given=0; unit_when=0; unit_then=0
}

# Parse. When a bead body is the source, scenarios live under a `## Scenarios`
# markdown heading; we only count `Scenario:` lines once we've entered it. A
# bare .feature file has a `Feature:` line and no `## Scenarios` heading — in
# that case scenarios are in scope from the start.
file_tags=""
pending_tags=""
in_scenarios=0
has_scenarios_heading=0

# Heading regex (matched against a whitespace-trimmed line). Coverage mode
# keeps the historical strict match. Admission mode also accepts an annotation
# suffix — corpus reality: epic bodies write headings like
# "## Scenarios (epic rollup — end-state contract over the children)".
SCEN_HEADING_RE='^##[[:space:]]+Scenarios[[:space:]]*$'
if [[ $ADMISSION -eq 1 ]]; then
    SCEN_HEADING_RE='^##[[:space:]]+Scenarios:?([[:space:]].*)?$'
fi

# Pre-scan: does the source carry a `## Scenarios` heading? If so, only count
# scenarios inside it (a bead body). Otherwise treat the whole text as scenarios
# (a .feature file). Fence tracking (F4): a heading inside a ``` fence does not
# count. In --admission mode a missing heading means inadmissible — never fall
# back to whole-text (.feature) scanning, which would count prose GWT lines.
if printf '%s\n' "$RAW" | awk -v re="$SCEN_HEADING_RE" '
    /^[[:space:]]*```/ { in_fence = !in_fence; next }
    in_fence           { next }
    {
        line = $0
        sub(/^[[:space:]]+/, "", line)
        if (line ~ re) { found = 1; exit }
    }
    END { exit !found }
'; then
    has_scenarios_heading=1
elif [[ $ADMISSION -eq 0 ]]; then
    in_scenarios=1
fi

in_fence=0
while IFS= read -r line; do
    trimmed="$(printf '%s' "$line" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"

    # Fence tracking (F4): fenced lines are parse-inert — no Scenario:, GWT,
    # tag, or `## ` section transition is recognized inside a ``` fence.
    if [[ "$trimmed" == '```'* ]]; then
        in_fence=$((1 - in_fence))
        continue
    fi
    if [[ $in_fence -eq 1 ]]; then
        continue
    fi

    # Enter/exit the `## Scenarios` block (bead-body mode).
    if [[ $has_scenarios_heading -eq 1 ]]; then
        if [[ "$trimmed" =~ $SCEN_HEADING_RE ]]; then
            in_scenarios=1
            file_tags="$pending_tags"   # tags directly above the heading apply to all
            pending_tags=""
            continue
        fi
        # A new H2 section other than Scenarios ends the block.
        if [[ $in_scenarios -eq 1 && "$trimmed" =~ ^##[[:space:]] ]]; then
            if [[ $ADMISSION -eq 1 ]]; then
                flush_admission_unit   # F7: section exit flushes the open unit
            fi
            in_scenarios=0
            pending_tags=""
            continue
        fi
    fi

    # --admission: structural unit parsing replaces tag/coverage logic.
    if [[ $ADMISSION -eq 1 ]]; then
        if [[ $in_scenarios -eq 1 ]]; then
            if [[ "$trimmed" == Scenario:* || "$trimmed" == "Scenario Outline:"* ]]; then
                flush_admission_unit
                unit_open=1
                unit_kind="scenario"
                unit_name="${trimmed#Scenario: }"
                unit_name="${unit_name#Scenario Outline: }"
            elif [[ "$trimmed" =~ ^(Given|When|Then|And|But)([^A-Za-z0-9_]|$) ]]; then
                if [[ $unit_open -eq 0 ]]; then
                    unit_open=1
                    unit_kind="stanza"
                    unit_name="bare-GWT stanza $((units_total + 1))"
                fi
                case "$trimmed" in
                    Given*) unit_given=$((unit_given + 1));;
                    When*)  unit_when=$((unit_when + 1));;
                    Then*)  unit_then=$((unit_then + 1));;
                esac
            elif [[ -z "$trimmed" ]]; then
                # N2 boundary rule: a blank line splits only a bare stanza. A
                # Scenario: unit stays open across blank lines — it ends only
                # on the next Scenario:, a `## ` section exit, or EOF.
                if [[ $unit_open -eq 1 && "$unit_kind" == "stanza" ]]; then
                    flush_admission_unit
                fi
            else
                # Non-GWT, non-blank line: ends a bare stanza; inert inside a
                # Scenario block (descriptions, tables, tags).
                if [[ $unit_open -eq 1 && "$unit_kind" == "stanza" ]]; then
                    flush_admission_unit
                fi
            fi
        fi
        continue
    fi

    if [[ "$trimmed" == @* ]]; then
        for tok in $trimmed; do
            if [[ "$tok" == @covered-by:* ]]; then
                pending_tags+="${tok#@covered-by:} "
            fi
        done
        continue
    fi

    # In .feature mode, a Feature: line promotes pending tags to file-level.
    if [[ $has_scenarios_heading -eq 0 && "$trimmed" == Feature:* ]]; then
        file_tags="$pending_tags"
        pending_tags=""
        continue
    fi

    if [[ $in_scenarios -eq 1 && ( "$trimmed" == Scenario:* || "$trimmed" == "Scenario Outline:"* ) ]]; then
        scenarios_total=$((scenarios_total + 1))
        scen_name="${trimmed#Scenario: }"
        scen_name="${scen_name#Scenario Outline: }"

        effective="$file_tags $pending_tags"
        pending_tags=""
        effective="$(printf '%s' "$effective" | xargs || true)"

        if [[ -z "$effective" ]]; then
            UNCOVERED+=("$scen_name")
            continue
        fi

        scenario_ok=1
        for tgt in $effective; do
            err="$(resolve_target "$tgt")"
            if [[ -n "$err" ]]; then
                ERROR_LINES+=("scenario \"$scen_name\": dangling @covered-by:$tgt — $err")
                errors=$((errors + 1))
                scenario_ok=0
            fi
        done
        if [[ $scenario_ok -eq 1 ]]; then
            scenarios_covered=$((scenarios_covered + 1))
        else
            UNCOVERED+=("$scen_name")
        fi
        continue
    fi

    # Any other non-blank line clears pending scenario-level tags (Gherkin: a tag
    # must be immediately above the Scenario it annotates).
    if [[ -n "$trimmed" && "$trimmed" != @* ]]; then
        pending_tags=""
    fi
done < <(printf '%s\n' "$RAW")

# F7: EOF flushes the final open unit — its verdict must not be lost.
if [[ $ADMISSION -eq 1 ]]; then
    flush_admission_unit
fi

# Threshold: default = all scenarios (N). Override via --min-covered.
threshold="${MIN_COVERED:-$scenarios_total}"

if [[ $ADMISSION -eq 1 ]]; then
    # Admission verdict: >=1 unit AND every unit structurally complete.
    # @covered-by resolution and --min-covered are deliberately skipped —
    # the zero-scenario --min-covered=0 loophole does not apply here.
    result="pass"
    if [[ $has_scenarios_heading -eq 0 ]]; then
        ERROR_LINES+=("no '## Scenarios' block found — bead body is inadmissible (add a '## Scenarios' section with Given/When/Then acceptance)")
        errors=$((errors + 1))
        result="fail"
    elif [[ $units_total -eq 0 ]]; then
        ERROR_LINES+=("'## Scenarios' block carries no scenario units — free-text acceptance is inadmissible (write 'Scenario:' blocks or bare Given/When/Then stanzas)")
        errors=$((errors + 1))
        result="fail"
    elif [[ $units_complete -lt $units_total ]]; then
        result="fail"
    fi
else
    # Record uncovered scenarios as errors. A scenario with no @covered-by tag is
    # the headline ag-9jle.4 failure: "tests exist" / coverage% is NOT enough —
    # every scenario needs an explicit covering test.
    for scen in "${UNCOVERED[@]:-}"; do
        [[ -z "$scen" ]] && continue
        ERROR_LINES+=("scenario \"$scen\": no covering test — add '@covered-by:<test-path>' directly above it (forward from the scenario)")
    done

    covered_meets_threshold=0
    if [[ $scenarios_covered -ge $threshold ]]; then
        covered_meets_threshold=1
    fi

    result="pass"
    if [[ $errors -gt 0 || $covered_meets_threshold -eq 0 ]]; then
        result="fail"
    fi
fi

if [[ $JSON -eq 1 ]]; then
    printf '{"scenarios_total":%d,"scenarios_covered":%d,"threshold":%d,"errors":%d,"run":%s,"admission":%s,"units_total":%d,"structurally_complete":%d,"warnings":%d,"result":"%s"}\n' \
        "$scenarios_total" "$scenarios_covered" "$threshold" "$errors" \
        "$([[ $RUN -eq 1 ]] && echo true || echo false)" \
        "$([[ $ADMISSION -eq 1 ]] && echo true || echo false)" \
        "$units_total" "$units_complete" "$admission_warnings" "$result"
fi

if [[ "$result" == "pass" ]]; then
    if [[ $JSON -eq 0 ]]; then
        if [[ $ADMISSION -eq 1 ]]; then
            echo "check-bead-scenario-coverage: ADMISSION PASS (${units_complete}/${units_total} units structurally complete; warnings ${admission_warnings})"
        else
            echo "check-bead-scenario-coverage: PASS (${scenarios_covered}/${scenarios_total} scenarios covered$([[ $RUN -eq 1 ]] && echo ' & passing'); threshold ${threshold})"
        fi
    fi
    exit 0
fi

if [[ $JSON -eq 0 ]]; then
    for e in "${ERROR_LINES[@]:-}"; do
        [[ -z "$e" ]] && continue
        echo "$e" >&2
    done
    if [[ $ADMISSION -eq 1 ]]; then
        echo "check-bead-scenario-coverage: ADMISSION ${units_complete}/${units_total} units structurally complete" >&2
    else
        echo "check-bead-scenario-coverage: ${scenarios_covered}/${scenarios_total} covered, threshold ${threshold}" >&2
    fi
fi

if [[ "$WARN_ONLY" -eq 1 ]]; then
    [[ $JSON -eq 0 ]] && echo "check-bead-scenario-coverage: WARN (below threshold or dangling links; --warn-only)" >&2
    exit 0
fi

[[ $JSON -eq 0 ]] && echo "check-bead-scenario-coverage: FAIL" >&2
exit 1
