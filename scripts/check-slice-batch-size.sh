#!/usr/bin/env bash
# check-slice-batch-size.sh — small-batch-by-Gherkin ENFORCEMENT (age-74yi).
#
# The enforcement half of the flywheel discipline
# (docs/architecture/the-flywheel.md). Behavior-first planning
# (skills/behavior-first-planning) SAYS "seed slices small — one behavior, one
# scenario per slice; no runnable acceptance test, no bead" but nothing
# mechanically FAILS on a multi-behavior slice. This makes the batch unit
# COUNTABLE: one slice bead == one behavior == one Gherkin scenario.
#
# It reads the bead body via `ao beads exec show <id> --json` (tracker-agnostic —
# resolves to `br` or `bd`, whichever this repo/substrate uses), counts the
# Gherkin scenarios in the body (the `description` field), and:
#
#   >1 scenario  → FAIL (exit 1): the slice batches multiple behaviors. Prints
#                  "SLICE-BATCH: FAIL — <id> has N behaviors (N Gherkin
#                  scenarios); split into N one-behavior slices" + the detected
#                  scenario names, so the operator knows where to cut.
#   exactly 1    → PASS (exit 0): "SLICE-BATCH: PASS".
#   0 scenarios  → WARN (exit 0): "SLICE-BATCH: WARN — no Gherkin scenario …".
#
# 0-scenario decision — WARN, not FAIL (deliberate):
#   A plain task bead with prose-only acceptance and no Given/When/Then block is
#   still common in the tracker today. This gate is a NEW discipline; hard-
#   failing every scenario-less bead would break the existing corpus en masse.
#   So it is introduced as invocable + documented FIRST: a scenario-less slice is
#   advised (WARN) to carry exactly one scenario, never hard-blocked. The
#   companion "no runnable acceptance test, no bead" admission (the --admission
#   mode of scripts/check-bead-scenario-coverage.sh) is where a bead is REQUIRED
#   to carry ≥1 structurally-complete scenario; this gate is strictly about batch
#   SIZE (one behavior, not many).
#
# Scenario-counting rule (a "behavior" = one Given…When…Then triad):
#   * A `Scenario:` / `Scenario Outline:` header always counts as one behavior;
#     the GWT lines that follow belong to it (they do NOT each start a new one).
#     A Scenario unit stays open until the next `Scenario:`/`## ` heading or EOF —
#     a blank line inside it is body formatting, not a boundary.
#   * When there are NO `Scenario:` headers, a contiguous run of line-start
#     Given/When/Then/And/But lines (a "bare-GWT stanza") that contains at least
#     one `Given` counts as one behavior. A blank or non-GWT line ends the stanza.
#   * Fenced (```) content is parse-inert — Scenario:/GWT text inside a code fence
#     never counts (bead bodies carry fenced yaml acceptance blocks).
#   * Matching is line-start after trimming, so mid-sentence prose like
#     "one happy-path Given/When/Then" never miscounts.
#   This mirrors the proven unit model in check-bead-scenario-coverage.sh
#   --admission, minus the `## Scenarios`-heading requirement (bead bodies are
#   often prose with an embedded GHERKIN block).
#
# WIRING (light, by design — NOT a blocking hook):
#   This gate is invocable + documented first, NOT wired into a blocking release
#   gate (that would fail existing multi-scenario beads en masse). It belongs at
#   two moments in the operating loop:
#     * discovery / plan (skills/behavior-first-planning, skills/plan): validate a
#       slice BEFORE it becomes a bead — reject a multi-scenario slice, split it.
#     * crank (skills/crank): re-check a slice that GREW during build — surfaced
#       extra behavior becomes a follow-up bead, never absorbed into this slice.
#   Referenced from skills/behavior-first-planning/SKILL.md. Promote to a
#   blocking gate only after the tracker's existing slices are one-behavior clean.
#
# Usage:
#   bash scripts/check-slice-batch-size.sh <bead-id>            # check one slice
#   bash scripts/check-slice-batch-size.sh --json <bead-id>     # machine-readable
#   bash scripts/check-slice-batch-size.sh --all-ready          # every ready slice
#
# Exits 0 on PASS/WARN, 1 on FAIL (>1 behavior), 2 on misuse/infra failure.
#
# practices: [continuous-integration, design-by-contract, bdd-gherkin, small-batches]
set -euo pipefail

JSON=0
ALL_READY=0
BEAD_ID=""

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --json)      JSON=1; shift;;
        --all-ready) ALL_READY=1; shift;;
        -h|--help)   usage; exit 0;;
        --*)         echo "Unknown flag: $1" >&2; exit 2;;
        *)           BEAD_ID="$1"; shift;;
    esac
done

if ! command -v ao >/dev/null 2>&1; then
    echo "ao is not on PATH — cannot read the bead body (tracker-agnostic read needs 'ao beads exec show')" >&2
    exit 2
fi

# count_scenarios <body-text>
#   Prints, on stdout: "<count>" on line 1, then one scenario name per subsequent
#   line. Implements the counting rule documented in the header.
count_scenarios() {
    printf '%s\n' "$1" | awk '
        function flush() {
            if (unit_open == 0) return
            if (unit_kind == "scenario") { names[++count] = unit_name }
            else if (unit_kind == "stanza" && unit_given >= 1) { names[++count] = unit_name }
            unit_open = 0; unit_kind = ""; unit_name = ""; unit_given = 0
        }
        BEGIN { in_fence = 0; count = 0; unit_open = 0 }
        {
            line = $0
            sub(/^[[:space:]]+/, "", line)   # left-trim
            sub(/[[:space:]]+$/, "", line)   # right-trim

            # Fence tracking: fenced lines are parse-inert.
            if (line ~ /^```/) { in_fence = !in_fence; next }
            if (in_fence) { next }

            # Scenario:/Scenario Outline: header — always a behavior.
            if (line ~ /^Scenario( Outline)?:/) {
                flush()
                unit_open = 1; unit_kind = "scenario"; unit_given = 0
                nm = line
                sub(/^Scenario( Outline)?:[[:space:]]*/, "", nm)
                unit_name = nm
                next
            }

            # Background: — Gherkin SHARED SETUP, not a behavior. Open an inert
            # unit so its Given/When/Then steps are absorbed (never counted as a
            # bare stanza). flush() never counts a "background" unit; a later
            # Scenario:/## flushes it away uncounted.
            if (line ~ /^Background:/) {
                flush()
                unit_open = 1; unit_kind = "background"; unit_given = 0; unit_name = ""
                next
            }

            # A new H2 section (other than a Scenario) ends the open unit.
            if (line ~ /^##[[:space:]]/) { flush(); next }

            # Line-start Given/When/Then/And/But — a Gherkin step line. The word
            # boundary ([^A-Za-z0-9_]|$) prevents matching "Givenness" etc.
            if (line ~ /^(Given|When|Then|And|But)([^A-Za-z0-9_]|$)/) {
                if (unit_open == 0) {
                    unit_open = 1; unit_kind = "stanza"; unit_given = 0
                    unit_name = "bare Given/When/Then block " (count + 1)
                }
                if (line ~ /^Given([^A-Za-z0-9_]|$)/) { unit_given++ }
                next
            }

            # Blank line: splits a bare stanza; a Scenario unit survives it.
            if (line == "") {
                if (unit_open == 1 && unit_kind == "stanza") flush()
                next
            }

            # Any other non-blank line: ends a bare stanza; inert inside a
            # Scenario block (descriptions, tables).
            if (unit_open == 1 && unit_kind == "stanza") flush()
        }
        END { flush(); print count; for (i = 1; i <= count; i++) print names[i] }
    '
}

# check_one <bead-id> — emits a verdict for one bead. Returns 0 (pass/warn) or 1
# (fail). With JSON=1, emits a one-line JSON object instead of prose.
check_one() {
    local id="$1"
    local raw
    raw="$(ao beads exec show "$id" --json 2>/dev/null || true)"
    if [[ -z "${raw//[[:space:]]/}" ]]; then
        echo "check-slice-batch-size: 'ao beads exec show $id --json' returned no content (tracker failure?)" >&2
        return 2
    fi

    # Extract the body (description). The passthrough emits a JSON array; also
    # tolerate a bare object. jq handles all unescaping.
    # A jq PARSE failure on non-empty tracker output means unreadable/malformed
    # JSON (a real infra failure) — NOT an empty description. Fail closed (exit 2);
    # never swallow it into a WARN, or the enforcement gate fails open on garbage.
    local body
    if ! body="$(printf '%s' "$raw" | jq -r '(if type=="array" then .[0] else . end) | (.description // "")' 2>/dev/null)"; then
        echo "check-slice-batch-size: could not parse tracker JSON for '$id' (malformed 'ao beads exec show --json' response) — infra failure" >&2
        return 2
    fi

    local out count
    out="$(count_scenarios "$body")"
    count="$(printf '%s\n' "$out" | head -n1)"
    [[ "$count" =~ ^[0-9]+$ ]] || count=0
    local scenarios
    scenarios="$(printf '%s\n' "$out" | tail -n +2)"

    local result
    if [[ "$count" -gt 1 ]]; then
        result="fail"
    elif [[ "$count" -eq 1 ]]; then
        result="pass"
    else
        result="warn"
    fi

    if [[ $JSON -eq 1 ]]; then
        local names_json
        # Read the whole scenario list as one raw string, split on newline, drop
        # empties → a JSON array. Robust on empty input (yields []); avoids the
        # pipefail+grep-returns-1 double-emit of a multi-stage pipeline.
        names_json="$(printf '%s' "$scenarios" | jq -R -s -c 'split("\n") | map(select(length > 0))' 2>/dev/null || echo '[]')"
        printf '{"bead":"%s","behaviors":%d,"result":"%s","scenarios":%s}\n' \
            "$id" "$count" "$result" "$names_json"
    else
        case "$result" in
            fail)
                echo "SLICE-BATCH: FAIL — $id has $count behaviors ($count Gherkin scenarios); split into $count one-behavior slices"
                while IFS= read -r nm; do
                    [[ -z "$nm" ]] && continue
                    echo "  - $nm"
                done <<< "$scenarios"
                ;;
            pass)
                echo "SLICE-BATCH: PASS — $id carries exactly one behavior (1 Gherkin scenario)"
                ;;
            warn)
                echo "SLICE-BATCH: WARN — $id has no Gherkin scenario; a slice should carry exactly one (add a Given/When/Then acceptance block). Advisory, not a hard fail."
                ;;
        esac
    fi

    [[ "$result" == "fail" ]] && return 1
    return 0
}

if [[ $ALL_READY -eq 1 ]]; then
    if [[ -n "$BEAD_ID" ]]; then
        echo "--all-ready takes no bead id" >&2
        exit 2
    fi
    ready_raw="$(ao beads exec ready --json 2>/dev/null || true)"
    if [[ -z "${ready_raw//[[:space:]]/}" ]]; then
        echo "check-slice-batch-size: 'ao beads exec ready --json' returned no content" >&2
        exit 2
    fi
    # ready output is either {"issues":[...]} (br) or a bare array (bd) — tolerate both.
    # A jq PARSE failure means malformed ready output — infra, fail closed (do NOT
    # swallow into an empty list and exit 0 over zero beads).
    ready_ids_raw=""
    if ! ready_ids_raw="$(printf '%s' "$ready_raw" | jq -r '((.issues // .) // [])[].id' 2>/dev/null)"; then
        echo "check-slice-batch-size: could not parse 'ao beads exec ready --json' (malformed) — infra failure" >&2
        exit 2
    fi
    mapfile -t ready_ids <<< "$ready_ids_raw"
    overall=0
    for rid in "${ready_ids[@]:-}"; do
        [[ -z "$rid" ]] && continue
        rc=0
        check_one "$rid" || rc=$?
        # Infra (rc 2, e.g. an unreadable bead) FAILS CLOSED — the sweep could not
        # verify "every ready slice", so it must not exit 0. Infra dominates a
        # policy FAIL (you must fix the unreadable bead before trusting the sweep).
        if [[ $rc -eq 2 ]]; then
            overall=2
        elif [[ $rc -eq 1 && $overall -ne 2 ]]; then
            overall=1
        fi
    done
    exit "$overall"
fi

if [[ -z "$BEAD_ID" ]]; then
    echo "usage: check-slice-batch-size.sh <bead id> | --all-ready   (no bead id given)" >&2
    exit 2
fi

rc=0
check_one "$BEAD_ID" || rc=$?
exit "$rc"
