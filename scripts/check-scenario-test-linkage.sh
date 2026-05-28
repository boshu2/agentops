#!/usr/bin/env bash
# check-scenario-test-linkage.sh — mechanize the scenario→test linkage gate.
#
# 71 .feature files live under skills/*/references/ but historically only the
# /rpi spec was actually executed; the rest were documentation. This gate makes
# the linkage mechanical: every Gherkin `Scenario:` in the corpus must EITHER
# declare the test that covers it with a `@covered-by:` tag, OR its feature file
# must be explicitly allowlisted as intentionally documentation-only.
#
# Sibling to the directive→scenario gate (soc-m8tdn / executable-spec-link-
# integrity): that one links GOALS directives → scenarios; this one links
# scenarios → tests. Together they close the spec-to-evidence chain.
#
# Convention (the lightest one that resolves "scenario X is covered by test Y"):
#
#   @covered-by:<test-path>            tag the whole file as the cover, OR
#   @covered-by:<test-path>::<Name>    name a specific test function (Go etc.)
#
# Place the tag on its own line directly above the `Scenario:` it covers (the
# standard Gherkin tag position). Multiple tags may stack. Tags on the line(s)
# above `Feature:` apply to every scenario in the file. The script:
#
#   1. parses every .feature scenario across skills/*/references/
#   2. resolves each scenario's @covered-by target(s)
#   3. FAILS on:
#        - a scenario with no @covered-by tag whose file is not allowlisted
#        - a dangling link: the test path does not exist, or the ::Name
#          function/definition is not found in that file
#
# Allowlist: scripts/.scenario-linkage-allow lists feature files (repo-relative)
# that are intentionally documentation-only in this slice. A file may NOT be
# both allowlisted AND carry @covered-by tags (ambiguous intent) — that errors.
#
# Usage:
#   bash scripts/check-scenario-test-linkage.sh             # blocking gate
#   bash scripts/check-scenario-test-linkage.sh --warn-only # advisory
#   bash scripts/check-scenario-test-linkage.sh --json      # machine-readable summary
#
# Exits 0 on pass, 1 on fail (unless --warn-only), 2 on misuse.
#
# practices: [continuous-integration, design-by-contract, code-complete]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ALLOWLIST="$REPO_ROOT/scripts/.scenario-linkage-allow"
WARN_ONLY=0
JSON=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --warn-only) WARN_ONLY=1; shift;;
        --json)      JSON=1; shift;;
        -h|--help)
            grep '^#' "$0" | sed 's/^# \?//'
            exit 0
            ;;
        *) echo "Unknown arg: $1" >&2; exit 2;;
    esac
done

# Load allowlisted feature files (repo-relative paths, one per line; '#' comments
# and blank lines ignored).
declare -A ALLOWED=()
if [[ -f "$ALLOWLIST" ]]; then
    while IFS= read -r entry; do
        entry="${entry%%#*}"                       # strip trailing comment
        entry="$(printf '%s' "$entry" | xargs)"    # trim whitespace
        [[ -z "$entry" ]] && continue
        ALLOWED["$entry"]=1
    done < "$ALLOWLIST"
fi

# Resolve a @covered-by target to an error string, or empty on success.
# Arg: the target spec after "@covered-by:" — "path" or "path::Name".
resolve_target() {
    local target="$1" path name
    if [[ "$target" == *"::"* ]]; then
        path="${target%%::*}"
        name="${target##*::}"
    else
        path="$target"
        name=""
    fi

    local abs="$REPO_ROOT/$path"
    if [[ ! -f "$abs" ]]; then
        printf 'test path does not exist: %s' "$path"
        return
    fi
    if [[ -n "$name" ]]; then
        # The named symbol must appear in the file (Go func, bats @test, bash
        # function, or step label). Substring match keeps the gate language-
        # agnostic while still catching a renamed/deleted test.
        if ! grep -qF "$name" "$abs"; then
            printf 'test "%s" not found in %s' "$name" "$path"
            return
        fi
    fi
    printf ''  # success
}

errors=0
scenarios_total=0
scenarios_linked=0
scenarios_allowlisted=0
files_total=0
files_allowlisted=0

declare -a ERROR_LINES=()

while IFS= read -r -d '' feature; do
    rel="${feature#"$REPO_ROOT"/}"
    files_total=$((files_total + 1))

    is_allowed=0
    [[ -n "${ALLOWED[$rel]:-}" ]] && is_allowed=1

    # Collect file-level tags (tags appearing before the Feature: line) and,
    # per scenario, the tags on the contiguous block of lines above it.
    file_tags=""
    pending_tags=""
    seen_feature=0
    has_any_tag=0

    while IFS= read -r line; do
        trimmed="$(printf '%s' "$line" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"

        if [[ "$trimmed" == @* ]]; then
            # A tag line. Accumulate any @covered-by: targets it carries.
            for tok in $trimmed; do
                if [[ "$tok" == @covered-by:* ]]; then
                    has_any_tag=1
                    pending_tags+="${tok#@covered-by:} "
                fi
            done
            continue
        fi

        if [[ "$trimmed" == Feature:* && $seen_feature -eq 0 ]]; then
            seen_feature=1
            file_tags="$pending_tags"
            pending_tags=""
            continue
        fi

        if [[ "$trimmed" == Scenario:* || "$trimmed" == "Scenario Outline:"* ]]; then
            scenarios_total=$((scenarios_total + 1))
            scen_name="${trimmed#Scenario: }"
            scen_name="${scen_name#Scenario Outline: }"

            # Effective targets = file-level tags + this scenario's pending tags.
            effective="$file_tags $pending_tags"
            pending_tags=""
            # Normalize whitespace.
            effective="$(printf '%s' "$effective" | xargs || true)"

            if [[ $is_allowed -eq 1 ]]; then
                scenarios_allowlisted=$((scenarios_allowlisted + 1))
                # An allowlisted file with @covered-by tags is ambiguous intent.
                if [[ -n "$effective" ]]; then
                    ERROR_LINES+=("$rel: scenario \"$scen_name\" is in an allowlisted (doc-only) file but also declares @covered-by — remove the allowlist entry or the tag")
                    errors=$((errors + 1))
                fi
                continue
            fi

            if [[ -z "$effective" ]]; then
                ERROR_LINES+=("$rel: scenario \"$scen_name\" has no @covered-by tag and the file is not allowlisted")
                ERROR_LINES+=("    fix: add '@covered-by:<test-path>' above the Scenario, OR add '$rel' to scripts/.scenario-linkage-allow")
                errors=$((errors + 1))
                continue
            fi

            scenario_ok=1
            for tgt in $effective; do
                err="$(resolve_target "$tgt")"
                if [[ -n "$err" ]]; then
                    ERROR_LINES+=("$rel: scenario \"$scen_name\" has a dangling @covered-by:$tgt — $err")
                    errors=$((errors + 1))
                    scenario_ok=0
                fi
            done
            [[ $scenario_ok -eq 1 ]] && scenarios_linked=$((scenarios_linked + 1))
            continue
        fi

        # Any other non-tag, non-blank line clears pending scenario tags only if
        # we have not yet reached the scenario it would annotate. In Gherkin,
        # tags must be immediately above their scenario, so a Background or step
        # line between a tag and a Scenario invalidates the tag association.
        if [[ -n "$trimmed" && $seen_feature -eq 1 ]]; then
            pending_tags=""
        fi
    done < "$feature"

    # Tally an allowlisted file even when it has zero @covered-by tags (expected).
    if [[ $is_allowed -eq 1 ]]; then
        files_allowlisted=$((files_allowlisted + 1))
        if [[ $has_any_tag -eq 1 && ${#ERROR_LINES[@]} -eq 0 ]]; then
            : # handled per-scenario above
        fi
    fi
done < <(find "$REPO_ROOT/skills" -path '*/references/*.feature' -type f -print0 | sort -z)

# Detect stale allowlist entries: a listed file that no longer exists.
stale_allow=0
for rel in "${!ALLOWED[@]}"; do
    if [[ ! -f "$REPO_ROOT/$rel" ]]; then
        ERROR_LINES+=("allowlist: '$rel' is listed in scripts/.scenario-linkage-allow but the file no longer exists — remove the stale entry")
        errors=$((errors + 1))
        stale_allow=$((stale_allow + 1))
    fi
done

if [[ $JSON -eq 1 ]]; then
    printf '{"feature_files":%d,"allowlisted_files":%d,"scenarios_total":%d,"scenarios_linked":%d,"scenarios_allowlisted":%d,"errors":%d,"stale_allowlist_entries":%d,"result":"%s"}\n' \
        "$files_total" "$files_allowlisted" "$scenarios_total" "$scenarios_linked" "$scenarios_allowlisted" "$errors" "$stale_allow" \
        "$([[ $errors -eq 0 ]] && echo pass || echo fail)"
fi

if [[ $errors -eq 0 ]]; then
    [[ $JSON -eq 0 ]] && echo "check-scenario-test-linkage: PASS (${scenarios_total} scenarios across ${files_total} feature files; ${scenarios_linked} linked, ${scenarios_allowlisted} allowlisted in ${files_allowlisted} doc-only file(s))"
    exit 0
fi

if [[ $JSON -eq 0 ]]; then
    for e in "${ERROR_LINES[@]}"; do
        echo "$e" >&2
    done
fi

if [[ "$WARN_ONLY" -eq 1 ]]; then
    [[ $JSON -eq 0 ]] && echo "check-scenario-test-linkage: WARN ($errors issue(s); --warn-only)" >&2
    exit 0
fi

[[ $JSON -eq 0 ]] && echo "check-scenario-test-linkage: FAIL ($errors issue(s))" >&2
exit 1
