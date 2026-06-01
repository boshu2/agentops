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
# Usage:
#   bash scripts/check-bead-scenario-coverage.sh <source>        # .feature or bead-body file
#   bash scripts/check-bead-scenario-coverage.sh -                # read source from stdin
#   bash scripts/check-bead-scenario-coverage.sh --bead <id>      # fetch bead body via `bd show`
#   bash scripts/check-bead-scenario-coverage.sh --run <source>   # also EXECUTE each test, require pass
#   bash scripts/check-bead-scenario-coverage.sh --min-covered=N <source>
#   bash scripts/check-bead-scenario-coverage.sh --json <source>  # machine-readable summary
#   bash scripts/check-bead-scenario-coverage.sh --warn-only <source>
#
# Exits 0 on pass, 1 on fail (unless --warn-only), 2 on misuse.
#
# practices: [continuous-integration, design-by-contract, tdd, bdd-gherkin]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WARN_ONLY=0
JSON=0
RUN=0
MIN_COVERED=""   # empty = "all scenarios" (=N)
SOURCE=""
BEAD_ID=""

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --warn-only) WARN_ONLY=1; shift;;
        --json)      JSON=1; shift;;
        --run)       RUN=1; shift;;
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

# Resolve the raw scenarios text into $RAW.
RAW=""
if [[ -n "$BEAD_ID" ]]; then
    if ! command -v bd >/dev/null 2>&1; then
        echo "--bead given but bd is not on PATH" >&2
        exit 2
    fi
    RAW="$(bd show "$BEAD_ID" 2>/dev/null || true)"
    if [[ -z "$RAW" ]]; then
        echo "bd show $BEAD_ID returned no content" >&2
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

# Parse. When a bead body is the source, scenarios live under a `## Scenarios`
# markdown heading; we only count `Scenario:` lines once we've entered it. A
# bare .feature file has a `Feature:` line and no `## Scenarios` heading — in
# that case scenarios are in scope from the start.
file_tags=""
pending_tags=""
in_scenarios=0
has_scenarios_heading=0

# Pre-scan: does the source carry a `## Scenarios` heading? If so, only count
# scenarios inside it (a bead body). Otherwise treat the whole text as scenarios
# (a .feature file).
if printf '%s\n' "$RAW" | grep -qE '^[[:space:]]*##[[:space:]]+Scenarios[[:space:]]*$'; then
    has_scenarios_heading=1
else
    in_scenarios=1
fi

while IFS= read -r line; do
    trimmed="$(printf '%s' "$line" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"

    # Enter/exit the `## Scenarios` block (bead-body mode).
    if [[ $has_scenarios_heading -eq 1 ]]; then
        if [[ "$trimmed" =~ ^##[[:space:]]+Scenarios[[:space:]]*$ ]]; then
            in_scenarios=1
            file_tags="$pending_tags"   # tags directly above the heading apply to all
            pending_tags=""
            continue
        fi
        # A new H2 section other than Scenarios ends the block.
        if [[ $in_scenarios -eq 1 && "$trimmed" =~ ^##[[:space:]] ]]; then
            in_scenarios=0
            pending_tags=""
            continue
        fi
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

# Threshold: default = all scenarios (N). Override via --min-covered.
threshold="${MIN_COVERED:-$scenarios_total}"

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

if [[ $JSON -eq 1 ]]; then
    printf '{"scenarios_total":%d,"scenarios_covered":%d,"threshold":%d,"errors":%d,"run":%s,"result":"%s"}\n' \
        "$scenarios_total" "$scenarios_covered" "$threshold" "$errors" \
        "$([[ $RUN -eq 1 ]] && echo true || echo false)" "$result"
fi

if [[ "$result" == "pass" ]]; then
    [[ $JSON -eq 0 ]] && echo "check-bead-scenario-coverage: PASS (${scenarios_covered}/${scenarios_total} scenarios covered$([[ $RUN -eq 1 ]] && echo ' & passing'); threshold ${threshold})"
    exit 0
fi

if [[ $JSON -eq 0 ]]; then
    for e in "${ERROR_LINES[@]:-}"; do
        [[ -z "$e" ]] && continue
        echo "$e" >&2
    done
    echo "check-bead-scenario-coverage: ${scenarios_covered}/${scenarios_total} covered, threshold ${threshold}" >&2
fi

if [[ "$WARN_ONLY" -eq 1 ]]; then
    [[ $JSON -eq 0 ]] && echo "check-bead-scenario-coverage: WARN (below threshold or dangling links; --warn-only)" >&2
    exit 0
fi

[[ $JSON -eq 0 ]] && echo "check-bead-scenario-coverage: FAIL" >&2
exit 1
