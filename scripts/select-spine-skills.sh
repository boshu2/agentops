#!/usr/bin/env bash
set -euo pipefail

# select-spine-skills.sh <skills_root>
#
# Prints (one skill directory NAME per line, sorted) each skill under
# <skills_root> whose SKILL.md declares `spine: true` in its YAML frontmatter,
# plus every `implementation: false` compatibility pointer whose `redirect_to`
# target is one of those spine skills.
# This is the selection lever behind `install.sh --tier spine`: a spine-only
# install ships just the proven spine skills instead of the whole bundle
# (age-h4y3). The spine tier is the line-3 `spine:` frontmatter key maintained in
# skills/SKILL-TIERS.md.
#
# Frontmatter-only by design: a body mention of "spine: true" in prose must not
# flip a skill into the spine tier, so the scan stops at the closing `---`.

usage() {
	echo "usage: select-spine-skills.sh <skills_root>" >&2
}

root="${1:-}"
if [[ -z "$root" ]]; then
	usage
	exit 2
fi
if [[ ! -d "$root" ]]; then
	echo "select-spine-skills: not a directory: $root" >&2
	exit 1
fi

frontmatter_value() {
    local file="$1" key="$2"
    awk -v key="$key" '
        NR == 1 { if ($0 != "---") exit; fm = 1; next }
        fm && $0 == "---" { exit }
        fm && index($0, key ":") == 1 {
            value = substr($0, length(key) + 2)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
            print value
            exit
        }
    ' "$file"
}

# Resolve the canonical spine set first. Redirects are selected from that set,
# so an alias cannot promote an experimental target into a spine installation.
direct_spine="$(
    for dir in "$root"/*/; do
        skill_md="${dir}SKILL.md"
        [[ -f "$skill_md" ]] || continue
        [[ "$(frontmatter_value "$skill_md" spine)" == "true" ]] || continue
        basename "$dir"
    done
)"

{
    printf '%s\n' "$direct_spine"
    for dir in "$root"/*/; do
        skill_md="${dir}SKILL.md"
        [[ -f "$skill_md" ]] || continue
        [[ "$(frontmatter_value "$skill_md" implementation)" == "false" ]] || continue
        target="$(frontmatter_value "$skill_md" redirect_to)"
        [[ -n "$target" ]] || continue
        if grep -qxF "$target" <<<"$direct_spine"; then
            basename "$dir"
        fi
    done
} | awk 'NF' | sort -u
