#!/usr/bin/env bats

@test "root contract routes native execution and optional RPI" {
  repo_root="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  contract="$repo_root/AGENTS.md"
  [ -f "$contract" ]

  for required in \
    "operations layer for agentic engineering" \
    "federated integration graph" \
    "zero mandatory AgentOps skills" \
    "Lean RPI operating charter" \
    "Accepted intent -> native implementation and checks -> one fresh read where a mistake is costly -> finish" \
    "Persist machine evidence only for a caller request or declared consumer" \
    "It owns no aggregate retry controller" \
    "Spend validation where a mistake is costly" \
    "repair does not start another review" \
    "unchanged accepted outcome and scope; acceptance changes need caller authority" \
    "Implement repairs ordinary known defects directly" \
    "use at most one bounded fresh helper" \
    "incident within authority and real remaining bounds" \
    "Cancellation, refusal and spent hard time/cost/quota skip help" \
    "compaction, helpers and new subjects never renew real limits" \
    "docs/architecture/rpi-traversal.md"; do
    grep -Fq -- "$required" "$contract"
  done

  # Native controls own aggregate allowances; the lean charter keeps direct
  # repair, bounded help and fresh judgment without adding delivery machinery.
  ! grep -Eq 'ao land|next-work|Plan-Pawl' "$contract"
}

@test "every relative file link in the root contract resolves" {
  repo_root="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  run python3 - "$repo_root" <<'EOF'
import re
import sys
from pathlib import Path

root = Path(sys.argv[1])
text = (root / "AGENTS.md").read_text(encoding="utf-8")
targets = re.findall(r"\]\(([^)\s]+)\)", text)
local = [t.split("#", 1)[0] for t in targets if "://" not in t and not t.startswith("#")]
missing = [t for t in local if t and not (root / t).exists()]
assert local, "AGENTS.md links no repository files"
if missing:
    print("missing:", missing)
    raise SystemExit(1)
EOF
  [ "$status" -eq 0 ]
}

@test "every .claude/rules file cites live paths and declares live path globs" {
  repo_root="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  cited_re='^(skills|scripts|cli|docs|tests)/[^ <>$*{}]+$'
  problems=()
  rules=("$repo_root"/.claude/rules/*.md)
  [ -f "${rules[0]}" ]

  for rule in "${rules[@]}"; do
    name=".claude/rules/${rule##*/}"

    # (a) A backticked repository path must exist, scoped or not: these files
    # load with no skill, so a dead pointer is read as live guidance.
    while IFS= read -r span; do
      if [[ "$span" =~ $cited_re ]] && [ ! -e "$repo_root/$span" ] &&
        [ -z "$(git -C "$repo_root" ls-files -- ":(literal)$span")" ]; then
        problems+=("$name cites missing path: $span")
      fi
    done < <(grep -o '`[^`]*`' "$rule" | tr -d '`')

    # (b) A paths: list in leading frontmatter scopes when the rule loads, so
    # each glob must match a tracked file. Scoping itself is optional.
    globs=()
    declares_paths=0
    in_paths=0
    line_no=0
    while IFS= read -r line; do
      line_no=$((line_no + 1))
      if [ "$line_no" -eq 1 ]; then
        if [ "$line" != "---" ]; then
          break
        fi
        continue
      fi
      if [ "$line" = "---" ]; then
        break
      fi
      if [[ "$line" =~ ^paths:[[:space:]]*(.*)$ ]]; then
        declares_paths=1
        in_paths=1
        inline="${BASH_REMATCH[1]}"
        inline="${inline#[}"
        inline="${inline%]}"
        IFS=',' read -r -a parts <<< "$inline"
        for part in "${parts[@]}"; do
          part="$(printf '%s' "$part" | sed -E "s/^[[:space:]]*[\"']?//; s/[\"']?[[:space:]]*\$//")"
          if [ -n "$part" ]; then
            globs+=("$part")
          fi
        done
        continue
      fi
      if [ "$in_paths" -eq 1 ] && [[ "$line" =~ ^[[:space:]]+-[[:space:]]*(.*)$ ]]; then
        item="$(printf '%s' "${BASH_REMATCH[1]}" | sed -E "s/^[\"']//; s/[\"'][[:space:]]*\$//; s/[[:space:]]+\$//")"
        if [ -n "$item" ]; then
          globs+=("$item")
        fi
        continue
      fi
      in_paths=0
    done < "$rule"

    if [ "$declares_paths" -eq 1 ] && [ "${#globs[@]}" -eq 0 ]; then
      problems+=("$name declares paths: but lists no glob")
    fi
    for g in "${globs[@]}"; do
      if [ -z "$(git -C "$repo_root" ls-files -- ":(glob)$g")" ]; then
        problems+=("$name paths glob matches no tracked file: $g")
      fi
    done
  done

  if [ "${#problems[@]}" -gt 0 ]; then
    printf '%s\n' "${problems[@]}"
    return 1
  fi
}
