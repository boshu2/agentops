#!/usr/bin/env bash
set -euo pipefail

# select-spine-skills.sh <skills_root>
#
# Prints (one skill directory NAME per line, sorted) each skill under
# <skills_root> whose SKILL.md declares `spine: true` in its YAML frontmatter.
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

for dir in "$root"/*/; do
	skill_md="${dir}SKILL.md"
	[[ -f "$skill_md" ]] || continue
	# Exit 0 iff `spine: true` appears inside the frontmatter block (opened by a
	# leading `---`, closed by the next `---`).
	if awk '
		NR == 1 { if ($0 != "---") exit 1; fm = 1; next }
		fm && $0 == "---" { fm = 0 }
		fm && /^spine:[[:space:]]*true[[:space:]]*$/ { found = 1 }
		END { exit found ? 0 : 1 }
	' "$skill_md"; then
		basename "$dir"
	fi
done | sort
