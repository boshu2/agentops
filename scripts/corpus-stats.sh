#!/usr/bin/env bash
# corpus-stats.sh — derive corpus stats from tracked sources.
#
# Walks a chosen corpus root (default: this repo's .agents/, override via
# AO_CORPUS_ROOT env var) and counts learnings, patterns, planning rules,
# findings (registry entries), and citations.
#
# Outputs:
#   --json      : machine-readable JSON (default)
#   --markdown  : markdown snippet suitable for inclusion in PRODUCT.md
#   --table     : human-readable table to stdout
#
# Soc-sx99.9 deliverable: replaces the previously-fabricated 4940/1195/40
# line in PRODUCT.md with a derived-from-source artifact. Anyone running
# this against this repo's `.agents/` should get the same answer as
# "ao corpus stats" if/when that subcommand lands.
#
# Usage:
#   scripts/corpus-stats.sh                    # JSON output (default)
#   scripts/corpus-stats.sh --markdown         # markdown snippet
#   scripts/corpus-stats.sh --table            # human-readable table
#   AO_CORPUS_ROOT=~/.agents scripts/corpus-stats.sh   # custom corpus root

set -euo pipefail

CORPUS_ROOT="${AO_CORPUS_ROOT:-.agents}"
FORMAT="json"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --json)     FORMAT="json"; shift ;;
    --markdown) FORMAT="markdown"; shift ;;
    --table)    FORMAT="table"; shift ;;
    --help|-h)
      sed -n '2,/^set -euo/p' "$0" | sed -e 's/^# \?//' -e '/^set -euo/d'
      exit 0
      ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

if [[ ! -d "$CORPUS_ROOT" ]]; then
  echo "error: corpus root not found: $CORPUS_ROOT" >&2
  echo "       set AO_CORPUS_ROOT or run from a repo with .agents/" >&2
  exit 1
fi

count_md() {
  local dir="$1"
  if [[ -d "$CORPUS_ROOT/$dir" ]]; then
    find "$CORPUS_ROOT/$dir" -type f -name "*.md" 2>/dev/null | wc -l | tr -d ' '
  else
    echo 0
  fi
}

count_jsonl_lines() {
  local file="$1"
  if [[ -f "$CORPUS_ROOT/$file" ]]; then
    wc -l < "$CORPUS_ROOT/$file" | tr -d ' '
  else
    echo 0
  fi
}

learnings=$(count_md learnings)
patterns=$(count_md patterns)
planning_rules=$(count_md planning-rules)
constraints=$(count_md constraints)
findings_md=$(count_md findings)
findings_registry=$(count_jsonl_lines findings/registry.jsonl)
citations=$(count_jsonl_lines ao/citations.jsonl)
council_artifacts=$(count_md council)
research_artifacts=$(count_md research)
playbooks=$(count_md playbooks)

generated_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
corpus_root_abs=$(cd "$CORPUS_ROOT" && pwd)

case "$FORMAT" in
  json)
    cat <<JSON
{
  "schema_version": 1,
  "generated_at": "$generated_at",
  "corpus_root": "$corpus_root_abs",
  "counts": {
    "learnings": $learnings,
    "patterns": $patterns,
    "planning_rules": $planning_rules,
    "constraints": $constraints,
    "findings_md": $findings_md,
    "findings_registry": $findings_registry,
    "citations": $citations,
    "council_artifacts": $council_artifacts,
    "research_artifacts": $research_artifacts,
    "playbooks": $playbooks
  },
  "method": {
    "learnings": "find .agents/learnings -name '*.md' | wc -l",
    "patterns": "find .agents/patterns -name '*.md' | wc -l",
    "planning_rules": "find .agents/planning-rules -name '*.md' | wc -l",
    "findings_registry": "wc -l .agents/findings/registry.jsonl",
    "citations": "wc -l .agents/ao/citations.jsonl"
  }
}
JSON
    ;;

  markdown)
    cat <<MD
> Maintainer corpus stats (this repo's \`.agents/\`, derived by \`scripts/corpus-stats.sh\` at $generated_at):
>
> - **$learnings** learnings · **$patterns** patterns · **$planning_rules** planning rules · **$constraints** constraints
> - **$findings_md** finding markdown files (**$findings_registry** registry entries)
> - **$citations** citations recorded in \`.agents/ao/citations.jsonl\`
> - **$council_artifacts** council artifacts · **$research_artifacts** research artifacts · **$playbooks** playbooks
>
> These are this repo's corpus stats. Your own AgentOps install will produce its own — re-run \`scripts/corpus-stats.sh\` against \`\$AO_CORPUS_ROOT\` to derive yours.
MD
    ;;

  table)
    printf '%-22s %s\n' "Category" "Count"
    printf '%-22s %s\n' "----------------------" "-----"
    printf '%-22s %s\n' "Learnings"             "$learnings"
    printf '%-22s %s\n' "Patterns"              "$patterns"
    printf '%-22s %s\n' "Planning rules"        "$planning_rules"
    printf '%-22s %s\n' "Constraints"           "$constraints"
    printf '%-22s %s\n' "Findings (md)"         "$findings_md"
    printf '%-22s %s\n' "Findings (registry)"   "$findings_registry"
    printf '%-22s %s\n' "Citations"             "$citations"
    printf '%-22s %s\n' "Council artifacts"     "$council_artifacts"
    printf '%-22s %s\n' "Research artifacts"    "$research_artifacts"
    printf '%-22s %s\n' "Playbooks"             "$playbooks"
    printf '\nCorpus root: %s\nGenerated:    %s\n' "$corpus_root_abs" "$generated_at"
    ;;
esac
