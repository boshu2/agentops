#!/usr/bin/env bash
# check-adr-registry.sh — ADR registry integrity gate.
#
# Over docs/adr/ADR-*.md, asserts:
#   (a) every ADR filename number NNNN is unique across files;
#   (b) the filename number matches the number in the in-file H1 title
#       ("# ADR-NNNN: ...");
#   (c) every ADR carries a Status line ("**Status:**", in a list item or a
#       blockquote — both shapes exist in the corpus).
#
# On a duplicate number, both colliding files are named. Introduced by
# age-gate-the-ungated-egwt.11 after two ADRs shipped as ADR-0004.
#
# Exit codes: 0 = clean, 1 = registry violation, 2 = usage/environment error.

# Strict mode + hijack-proof REPO_ROOT + portable_find come from the shared
# preamble (scripts/lib/preamble.sh). `CDPATH=` is an intentional env-prefix
# (clears CDPATH for that one cd), not an assignment — hence the SC1007 disable.
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

adr_dir="$REPO_ROOT/docs/adr"

if [[ ! -d "$adr_dir" ]]; then
    echo "ADR_REGISTRY: FAIL: ADR directory not found: $adr_dir" >&2
    exit 2
fi

# Allow tests to point the gate at a fixture directory.
if [[ "${1:-}" == "--dir" ]]; then
    if [[ -z "${2:-}" ]]; then
        echo "ADR_REGISTRY: usage: check-adr-registry.sh [--dir <adr-dir>]" >&2
        exit 2
    fi
    adr_dir="$2"
fi

failures=0

fail() {
    echo "ADR_REGISTRY: FAIL: $*" >&2
    failures=$((failures + 1))
}

# Collect ADR files (sorted for deterministic reporting).
adr_files=()
while IFS= read -r f; do
    adr_files+=("$f")
done < <(portable_find "$adr_dir" -maxdepth 1 -type f -name 'ADR-*.md' | LC_ALL=C sort)

if [[ ${#adr_files[@]} -eq 0 ]]; then
    fail "no ADR-*.md files found under $adr_dir"
    exit 1
fi

# Map each number -> the file(s) claiming it (space-separated basenames), so a
# duplicate can name every colliding file.
declare -A number_owners

for f in "${adr_files[@]}"; do
    base="$(basename "$f")"

    # (a/b) filename number: ADR-<NNNN>-...
    if [[ ! "$base" =~ ^ADR-([0-9]{4})- ]]; then
        fail "$base does not match the ADR-NNNN-<slug>.md filename convention"
        continue
    fi
    file_num="${BASH_REMATCH[1]}"

    # Track ownership for the duplicate check.
    number_owners["$file_num"]="${number_owners["$file_num"]:-}${number_owners["$file_num"]:+ }$base"

    # (b) in-file H1 title number: "# ADR-<NNNN>: ..."
    title_line="$(grep -m1 -E '^#[[:space:]]+ADR-[0-9]{4}[[:space:]]*:' "$f" || true)"
    if [[ -z "$title_line" ]]; then
        fail "$base has no '# ADR-NNNN: <title>' H1 heading"
    elif [[ "$title_line" =~ ^#[[:space:]]+ADR-([0-9]{4}) ]]; then
        title_num="${BASH_REMATCH[1]}"
        if [[ "$title_num" != "$file_num" ]]; then
            fail "$base: filename number $file_num != in-file title number $title_num"
        fi
    fi

    # (c) Status line — accept list-item ('- **Status:**') and blockquote
    # ('> **Status:**') shapes; both occur in the corpus.
    if ! grep -q -iE '^[[:space:]]*[>*-]?[[:space:]]*\**status\**[[:space:]]*:' "$f"; then
        fail "$base has no Status line (expected e.g. '- **Status:** Accepted (YYYY-MM-DD)')"
    fi
done

# (a) duplicate-number detection — name both colliding files.
for num in "${!number_owners[@]}"; do
    owners="${number_owners[$num]}"
    # Count words (basenames) sharing this number.
    read -r -a owner_arr <<< "$owners"
    if [[ ${#owner_arr[@]} -gt 1 ]]; then
        fail "duplicate ADR number $num claimed by: $owners"
    fi
done

if [[ "$failures" -gt 0 ]]; then
    exit 1
fi

echo "ADR_REGISTRY: PASS (${#adr_files[@]} ADRs; unique numbers, filename==title, Status present)"
exit 0
