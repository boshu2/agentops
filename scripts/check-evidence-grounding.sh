#!/usr/bin/env bash
# check-evidence-grounding.sh — ADVISORY evidence.grounding gate.
#
# WHY: this repo persists evidence documents — audits, receipts, handoffs — and
# then cites them. A citation is only worth the ground under it. Three failure
# shapes are MECHANICAL, so a human should never be the one who finds them: a
# path that does not exist, a commit id that does not resolve, and a scaffold
# placeholder that was never filled in. All three read as authoritative to the
# next agent, which is exactly the "file presence is not identity" trap.
#
# WHAT (mechanical half ONLY): over repo-tracked `*.md` under
#   docs/audits/**  ·  docs/evidence/**  ·  docs/handoffs/**
# flag three classes:
#
#   (a) dead-path      — a cited repo-relative path that does not exist. A
#                        citation is recognized only when it starts with a
#                        top-level directory that EXISTS in the tree and ends in
#                        an extension (or a `/`). Trailing `:12-18` line refs
#                        and `#anchors` are stripped before the existence test;
#                        a path that resolves relative to the citing document
#                        counts as resolved. Glob and placeholder forms
#                        (`skills/**/SKILL.md`, `<pkg>/x.go`) are NOT paths and
#                        are skipped.
#   (b) dead-sha       — a full-length 40-hex id `git cat-file` cannot resolve.
#                        Runs only when the repository has full history; see
#                        SHA CLASS below.
#   (c) scaffold       — an unrendered `{{placeholder}}`, or a heading left as
#                        `## TODO:`/`TBD`/`FIXME`. Inline code spans and fenced
#                        blocks are stripped FIRST, so an audit that quotes
#                        `{{.Agent}}` while discussing templates is not a leak.
#
# EXPLICITLY OUT OF SCOPE: whether the evidence actually supports the claim it
# is cited for. That is a semantic judgment and it already lives in the validate
# skill; duplicating it here would be a second, weaker judge.
#
# WHAT IT CANNOT SEE (read a clean run accordingly): a citation whose top-level
# directory has been deleted wholesale is not recognized as a path at all; a
# path cited in prose without an extension is skipped; a citation into
# GITIGNORED runtime state (`.agents/**`, ADR-0016) is skipped, because a clean
# checkout is supposed not to have it; and a 64-hex id is NOT checked, because
# in a SHA-1 repository no content digest resolves as a git object, so the class
# would flag every manifest digest and carry no signal.
#
# BASELINE (scripts/.evidence-grounding-baseline): the archive is pinned, not
# rewritten — the same shrink-only shape as scripts/.docs-cli-snippets-baseline.
# An entry is an exact doc path or a directory prefix ending in `/`, and carries
# its argument as a trailing `# comment`. Enforcement is two-way:
#   (a) an UNCOVERED offending doc              -> exit 1
#   (b) an entry that covers no finding at all  -> exit 1 (prune it)
# and it has teeth on live work: a finding on a line ADDED between BASE_REF and
# HEAD is NEVER excused by the baseline. Pinning the archive silences history,
# never a citation written today.
#
# SHA CLASS: an unresolvable id is indistinguishable from an unfetched one, so
# class (b) runs only on full history. `auto` (default) turns it OFF in a
# shallow clone and says so; while it is off the STALE-baseline rule is
# suppressed too — never prune an allowlist on an incomplete offender set.
# CI checks out with fetch-depth: 0, so class (b) is live there.
#
# ADVISORY-FIRST: registered Blocking:false in cli/internal/gates. The flip to
# blocking is made later, deliberately, on measured evidence — not on a calendar.
#
# Usage:
#   bash scripts/check-evidence-grounding.sh
#   BASE_REF=<ref> bash scripts/check-evidence-grounding.sh
#
# Env:
#   BASE_REF                        base ref for the added-line rule (default origin/main)
#   EVIDENCE_GROUNDING_ROOT         repo root to operate on (test seam)
#   EVIDENCE_GROUNDING_BASELINE     baseline file (test seam)
#   EVIDENCE_GROUNDING_SHA_CLASS    auto|on|off (default auto)
#
# Exit: 0 = clean or fully baselined · 1 = new offender / stale entry · 2 = misuse
#
# practices: [continuous-integration, measurement-over-assertion]
# shellcheck source=scripts/lib/preamble.sh disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

LIB_DIR="$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib"
# shellcheck source=scripts/lib/docs-scope.sh
. "$LIB_DIR/docs-scope.sh"
# shellcheck source=scripts/lib/ratchet.sh
. "$LIB_DIR/ratchet.sh"

ROOT="${EVIDENCE_GROUNDING_ROOT:-$REPO_ROOT}"
BASE_REF="${BASE_REF:-origin/main}"

usage() { grep '^#' "$0" | sed 's/^# \?//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --base) shift; [[ $# -gt 0 ]] || { echo "--base requires a ref" >&2; exit 2; }; BASE_REF="$1"; shift;;
        --base=*) BASE_REF="${1#--base=}"; shift;;
        -h|--help) usage; exit 0;;
        *) echo "Unknown flag: $1" >&2; exit 2;;
    esac
done

cd "$ROOT" || { echo "cannot enter '$ROOT'" >&2; exit 2; }
DOCS_ROOT="$ROOT"
export DOCS_ROOT

BASELINE="${EVIDENCE_GROUNDING_BASELINE:-$ROOT/scripts/.evidence-grounding-baseline}"
US=$'\x1f'

if ! git rev-parse --git-dir >/dev/null 2>&1; then
    echo "SKIP: not inside a git repository"
    exit 0
fi

# --- sha class ---------------------------------------------------------------
sha_class="${EVIDENCE_GROUNDING_SHA_CLASS:-auto}"
case "$sha_class" in
    auto)
        if [[ "$(git rev-parse --is-shallow-repository 2>/dev/null || echo true)" == "true" ]]; then
            sha_class="off"
        else
            sha_class="on"
        fi
        ;;
    on|off) ;;
    *) echo "EVIDENCE_GROUNDING_SHA_CLASS must be auto|on|off" >&2; exit 2;;
esac

# --- scope -------------------------------------------------------------------
mapfile -t candidate_docs < <(
    git ls-files -- 'docs/audits/*.md' 'docs/evidence/*.md' 'docs/handoffs/*.md' 2>/dev/null | LC_ALL=C sort
)

docs=()
exempt_count=0
for d in ${candidate_docs[@]+"${candidate_docs[@]}"}; do
    [[ -n "$d" && -f "$d" ]] || continue
    if docs_scope_is_exempt "$d"; then
        exempt_count=$((exempt_count + 1))
        continue
    fi
    docs+=("$d")
done

if [[ "${#docs[@]}" -eq 0 ]]; then
    echo "PASS: evidence grounding — no in-scope evidence documents (${exempt_count} exempt)"
    exit 0
fi

# --- path anchors ------------------------------------------------------------
# Recognize a citation only when it starts with a top-level directory that
# actually exists. Derived from the tree, so the gate never claims to know a
# path shape it cannot check.
anchors=()
while IFS= read -r dir; do
    dir="${dir#./}"
    [[ -n "$dir" && "$dir" != "." && "$dir" != ".git" ]] || continue
    anchors+=("$dir")
done < <(portable_find . -maxdepth 1 -type d -not -name '.' -not -name '.git' 2>/dev/null | sed 's|^\./||' | LC_ALL=C sort)

if [[ "${#anchors[@]}" -eq 0 ]]; then
    echo "check-evidence-grounding: no top-level directories under '$ROOT' — cannot anchor path citations" >&2
    exit 2
fi

anchor_ere=""
for a in "${anchors[@]}"; do
    # Escape regex metacharacters in a directory name (e.g. a leading dot).
    esc="$(printf '%s' "$a" | sed -E 's/[][(){}.*+?^$|\\]/\\&/g')"
    anchor_ere="${anchor_ere:+$anchor_ere|}$esc"
done

# --- detection ---------------------------------------------------------------
findings=()          # file<US>lineno<US>class<US>detail
declare -A seen_finding=()
declare -A sha_resolved=()

record() {
    local file="$1" lineno="$2" class="$3" detail="$4"
    # Deduplicate per LINE, not per document: the same dead citation repeated on
    # a newly-added line must still be seen by the added-line rule.
    local dedup="${file}|${lineno}|${class}|${detail}"
    [[ -z "${seen_finding[$dedup]:-}" ]] || return 0
    seen_finding["$dedup"]=1
    findings+=("${file}${US}${lineno}${US}${class}${US}${detail}")
}

for f in "${docs[@]}"; do
    doc_dir="${f%/*}"

    # (a) dead-path
    while IFS= read -r rec; do
        [[ -n "$rec" ]] || continue
        lineno="${rec%%:*}"
        tok="${rec#*:}"
        # grep -o keeps the boundary character; drop anything before the anchor.
        while [[ -n "$tok" && "$tok" != [A-Za-z0-9.]* ]]; do tok="${tok:1}"; done
        [[ -n "$tok" ]] || continue
        # Glob / placeholder forms are not paths.
        case "$tok" in *'*'*|*'?'*) continue ;; esac
        tok="${tok%%#*}"
        while [[ "$tok" == *[.,\;:\)] ]]; do tok="${tok%?}"; done
        [[ -n "$tok" ]] || continue
        # A citation is only checkable when it names a file (extension) or an
        # explicit directory (trailing slash).
        if [[ "$tok" != */ ]] && [[ ! "$tok" =~ \.[A-Za-z0-9]+$ ]]; then continue; fi
        [[ ! -e "$tok" ]] || continue
        [[ ! -e "${doc_dir}/${tok}" ]] || continue
        record "$f" "$lineno" "dead-path" "$tok"
    done < <(grep -noE "(^|[^A-Za-z0-9._/-])(${anchor_ere})/[A-Za-z0-9._/*?-]+" "$f" 2>/dev/null || true)

    # (b) dead-sha
    if [[ "$sha_class" == "on" ]]; then
        while IFS= read -r rec; do
            [[ -n "$rec" ]] || continue
            lineno="${rec%%:*}"
            sha="${rec#*:}"
            [[ "$sha" =~ ^[0-9a-f]{40}$ ]] || continue
            if [[ -z "${sha_resolved[$sha]:-}" ]]; then
                if git cat-file -e "$sha" 2>/dev/null; then
                    sha_resolved["$sha"]="yes"
                else
                    sha_resolved["$sha"]="no"
                fi
            fi
            [[ "${sha_resolved[$sha]}" == "no" ]] || continue
            record "$f" "$lineno" "dead-sha" "$sha"
        done < <(grep -noE '\b[0-9a-f]{40}\b' "$f" 2>/dev/null || true)
    fi

    # (c) scaffold
    while IFS= read -r rec; do
        [[ -n "$rec" ]] || continue
        lineno="${rec%%"$US"*}"
        rest="${rec#*"$US"}"
        class="${rest%%"$US"*}"
        detail="${rest#*"$US"}"
        record "$f" "$lineno" "$class" "$detail"
    done < <(awk -v US="$US" '
        /^[[:space:]]*```/ { infence = !infence; next }
        infence { next }
        {
            line = $0
            gsub(/`[^`]*`/, "", line)
            if (line ~ /\{\{[^}]*\}\}/) { print NR US "scaffold-placeholder" US line; next }
            if (line ~ /^#+[ \t].*(TODO|TBD|FIXME)/) { print NR US "scaffold-todo-heading" US line }
        }
    ' "$f" 2>/dev/null || true)
done

# --- gitignored runtime state is not a dead path -----------------------------
# `.agents/**` and friends are disposable local runtime state (ADR-0016): a
# clean checkout is SUPPOSED not to have them, so a doc citing one is correct
# documentation, not a broken citation. One batched `git check-ignore` pass.
if [[ "${#findings[@]}" -gt 0 ]]; then
    declare -A path_ignored=()
    while IFS= read -r ign; do
        [[ -n "$ign" ]] || continue
        path_ignored["$ign"]=1
    done < <(
        for rec in "${findings[@]}"; do
            rest="${rec#*"$US"}"
            rest="${rest#*"$US"}"
            [[ "${rest%%"$US"*}" == "dead-path" ]] || continue
            printf '%s\n' "${rest#*"$US"}"
        done | LC_ALL=C sort -u | git check-ignore --stdin 2>/dev/null || true
    )
    if [[ "${#path_ignored[@]}" -gt 0 ]]; then
        kept=()
        for rec in "${findings[@]}"; do
            rest="${rec#*"$US"}"
            rest="${rest#*"$US"}"
            if [[ "${rest%%"$US"*}" == "dead-path" && -n "${path_ignored[${rest#*"$US"}]:-}" ]]; then
                continue
            fi
            kept+=("$rec")
        done
        findings=(${kept[@]+"${kept[@]}"})
    fi
fi

# --- baseline ----------------------------------------------------------------
# Capture through a variable, not `mapfile < <(...)`: mapfile reports ITS OWN
# exit status, so a process-substitution failure would be silently swallowed.
pinned_raw="$(ratchet_load_pinned "$BASELINE" trailing-comment)" || {
    echo "check-evidence-grounding: cannot read baseline '$BASELINE' — refusing to certify" >&2
    exit 2
}
pinned=()
while IFS= read -r line; do
    [[ -n "$line" ]] && pinned+=("$line")
done <<< "$pinned_raw"

# covering_entry FILE — echo the baseline entry that covers FILE, if any.
covering_entry() {
    local file="$1" e
    for e in ${pinned[@]+"${pinned[@]}"}; do
        [[ -n "$e" ]] || continue
        if [[ "$e" == */ ]]; then
            [[ "$file" == "$e"* ]] && { printf '%s' "$e"; return 0; }
        else
            [[ "$file" == "$e" ]] && { printf '%s' "$e"; return 0; }
        fi
    done
    return 1
}

# Added-line sets for covered-but-changed docs. Unresolvable base ref means the
# added-line rule cannot run; the gate says so instead of pretending.
base_sha=""
added_lines_available=1
if ! base_sha="$(git rev-parse --verify --quiet "${BASE_REF}^{commit}")"; then
    added_lines_available=0
fi

declare -A added_line_set=()
declare -A added_scanned=()
scan_added_lines() {
    local file="$1" hdr start count
    [[ "$added_lines_available" -eq 1 ]] || return 0
    [[ -z "${added_scanned[$file]:-}" ]] || return 0
    added_scanned["$file"]=1
    while IFS= read -r hdr; do
        [[ -n "$hdr" ]] || continue
        # @@ -a,b +c,d @@  ->  c,d  (d defaults to 1)
        hdr="${hdr#*+}"
        hdr="${hdr%% *}"
        start="${hdr%%,*}"
        count="${hdr#*,}"
        [[ "$count" == "$hdr" ]] && count=1
        [[ "$start" =~ ^[0-9]+$ && "$count" =~ ^[0-9]+$ ]] || continue
        local i
        for (( i = 0; i < count; i++ )); do
            added_line_set["${file}:$(( start + i ))"]=1
        done
    done < <(git diff --no-color -U0 "${base_sha}...HEAD" -- "$file" 2>/dev/null | grep '^@@' || true)
}

new_offenders=()
excused_files=()
declare -A entry_hits=()
declare -A offender_files=()

for rec in ${findings[@]+"${findings[@]}"}; do
    [[ -n "$rec" ]] || continue
    file="${rec%%"$US"*}"
    rest="${rec#*"$US"}"
    lineno="${rest%%"$US"*}"
    rest="${rest#*"$US"}"
    class="${rest%%"$US"*}"
    detail="${rest#*"$US"}"
    offender_files["$file"]=1

    entry=""
    if entry="$(covering_entry "$file")"; then
        entry_hits["$entry"]=1
        scan_added_lines "$file"
        if [[ -n "${added_line_set[${file}:${lineno}]:-}" ]]; then
            new_offenders+=("${file}:${lineno}: ${class}: ${detail} (on a line added since ${BASE_REF} — the baseline does not excuse new citations)")
        else
            excused_files+=("$file")
        fi
    else
        new_offenders+=("${file}:${lineno}: ${class}: ${detail}")
    fi
done

stale_entries=()
for e in ${pinned[@]+"${pinned[@]}"}; do
    [[ -n "$e" ]] || continue
    [[ -z "${entry_hits[$e]:-}" ]] || continue
    stale_entries+=("$e")
done

baseline_rel="${BASELINE#"$ROOT"/}"
failed=0

if [[ "${#new_offenders[@]}" -gt 0 ]]; then
    failed=1
    {
        echo "check-evidence-grounding: FAIL — evidence document(s) cite ground that does not exist:"
        printf '  %s\n' "${new_offenders[@]}"
        echo ""
        echo "Fix the citation (correct the path/id, or fill the placeholder). Only a"
        echo "point-in-time record whose citation was true when written belongs in"
        echo "$baseline_rel, and every entry there carries its argument."
    } >&2
fi

# Never prune an allowlist on an incomplete offender set: with class (b) off,
# a doc whose only finding is a dead sha looks clean and its entry looks stale.
if [[ "${#stale_entries[@]}" -gt 0 ]]; then
    if [[ "$sha_class" == "off" ]]; then
        echo "NOTE: stale-baseline rule suppressed — class (b) dead-sha is UNCHECKED, so the offender set is incomplete (${#stale_entries[@]} entr(ies) unmatched here)"
    else
        failed=1
        {
            echo "check-evidence-grounding: FAIL — stale baseline entr(ies): they no longer cover any finding (prune them):"
            printf '  %s\n' "${stale_entries[@]}"
            echo ""
            echo "The allowlist only shrinks. Remove the above line(s) from $baseline_rel."
        } >&2
    fi
fi

if [[ "$failed" -ne 0 ]]; then
    exit 1
fi

if [[ "$sha_class" == "off" ]]; then
    echo "NOTE: class (b) dead-sha UNCHECKED (shallow repository or explicitly disabled) — CI runs it on full history"
fi
if [[ "$added_lines_available" -eq 0 ]]; then
    echo "NOTE: base ref '${BASE_REF}' is unresolvable — the added-line rule did not run (baseline coverage applied whole-file)"
fi
echo "PASS: evidence grounding — ${#docs[@]} document(s) scanned, ${exempt_count} exempt, ${#offender_files[@]} with findings (all covered), ${#pinned[@]} baseline entr(ies) active"
exit 0
