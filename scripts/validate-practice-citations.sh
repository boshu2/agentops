#!/usr/bin/env bash
# validate-practice-citations.sh
# practices: [adr, snapshot-testing, ddd-bounded-context]
#
# Walks the repo's primitives (skills, hooks, evals, CLI command files,
# schemas, and scripts with practice declarations)
# and reports which required ones declare a `practices: [...]` derivation from
# PRACTICE-REGISTRY.md and which don't.
#
# Default mode: REPORT-ONLY (exit 0 with findings printed).
# Strict mode exits 1 on missing required declarations or invalid slugs.
#
# Practices derive from: <repo>/PRACTICE-REGISTRY.md slug registry table.
# Primitives cite slugs in frontmatter (skills, evals) or header comments
# (hook scripts, shell scripts, CLI command files).
#
# practices: [tdd, bdd-gherkin, snapshot-testing]
#
# Usage:
#   scripts/validate-practice-citations.sh            # report-only
#   scripts/validate-practice-citations.sh --strict   # exit 1 on findings
#   scripts/validate-practice-citations.sh --json     # JSON report
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

STRICT=false
JSON_OUT=false
for arg in "$@"; do
  case "$arg" in
    --strict) STRICT=true ;;
    --json) JSON_OUT=true ;;
    -h|--help)
      echo "Usage: scripts/validate-practice-citations.sh [--strict] [--json]"
      exit 0
      ;;
    *)
      echo "Unknown arg: $arg" >&2
      exit 2
      ;;
  esac
done

PRACTICE_FILE="PRACTICE-REGISTRY.md"
if [[ ! -f "$PRACTICE_FILE" ]]; then
  echo "ERROR: $PRACTICE_FILE not found (run from repo root)" >&2
  exit 2
fi

# Extract slug catalog: rows in the canonical registry table.
# Table rows look like: | `slug-name` | era | description |
CATALOG="$(mktemp)"
trap 'rm -f "$CATALOG"' EXIT
awk '
  /^## Practice slugs/ { in_table=1; next }
  /^## / && in_table { in_table=0 }
  in_table && /^\| `[a-z0-9-]+` \|/ {
    match($0, /`[a-z0-9-]+`/)
    slug=substr($0, RSTART+1, RLENGTH-2)
    print slug
  }
' "$PRACTICE_FILE" | sort -u > "$CATALOG"

slug_count=$(wc -l < "$CATALOG")
if [[ "$slug_count" -lt 5 ]]; then
  echo "ERROR: only $slug_count slugs parsed from $PRACTICE_FILE — registry table format may have drifted" >&2
  exit 2
fi

# Scan primitives. Skills, hooks, evals, CLI command files, and schemas are
# required citation surfaces. Root scripts are a declaration-optional surface
# for now: validate any script citation that exists without reporting the many
# uncited legacy scripts as missing.
# Reads two patterns:
#   1. YAML frontmatter line: "practices: [slug, slug]" or "practices: [slug,\n  slug]"
#   2. Header comment line: "# practices: [slug, slug]"
REQUIRED_SCAN_TARGETS=(
  skills/*/SKILL.md
  hooks/*.sh
  evals/agentops-core/*.json
  cli/cmd/ao/*.go
  schemas/*.json
)
DECLARATION_OPTIONAL_SCAN_TARGETS=(
  scripts/*.sh
)

REPORT="$(mktemp)"
trap 'rm -f "$CATALOG" "$REPORT"' EXIT
total=0
declared=0
missing=0
invalid=0
optional_without_practices=0

extract_practices() {
  local file="$1"
  # Look for a 'practices: [...]' declaration in the first 200 lines.
  # Matches: practices: [a, b, c]  OR  practices: [a]  OR  "practices": ["a"]
  head -n 200 "$file" \
    | tr '\n' ' ' \
    | grep -oE '"?practices"?[[:space:]]*:[[:space:]]*\[[^]]*\]' \
    | head -1 \
    | grep -oE '\[[^]]*\]' \
    | tr -d '[](){}"' \
    | tr ',' '\n' \
    | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' \
    | grep -E '^[a-z0-9-]+$' \
    | sort -u || true
}

scan_file() {
  local f="$1"
  local require_practices="$2"

  total=$((total + 1))
  practices="$(extract_practices "$f" || true)"
  if [[ -z "$practices" ]]; then
    if [[ "$require_practices" == "true" ]]; then
      missing=$((missing + 1))
      echo "MISSING: $f" >> "$REPORT"
    else
      optional_without_practices=$((optional_without_practices + 1))
    fi
    return
  fi

  declared=$((declared + 1))
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    if ! grep -qxF "$slug" "$CATALOG"; then
      invalid=$((invalid + 1))
      echo "INVALID_SLUG: $f cites unknown slug \"$slug\"" >> "$REPORT"
    fi
  done <<<"$practices"
}

for pattern in "${REQUIRED_SCAN_TARGETS[@]}"; do
  for f in $pattern; do
    [[ -e "$f" ]] || continue
    scan_file "$f" true
  done
done

for pattern in "${DECLARATION_OPTIONAL_SCAN_TARGETS[@]}"; do
  for f in $pattern; do
    [[ -e "$f" ]] || continue
    scan_file "$f" false
  done
done

# Emit report
if [[ "$JSON_OUT" == "true" ]]; then
  mode="report"
  [[ "$STRICT" == "true" ]] && mode="strict"
  python3 - "$REPORT" "$CATALOG" "$total" "$declared" "$missing" "$invalid" "$optional_without_practices" "$mode" <<'PY'
import json, sys
report, catalog, total, declared, missing, invalid, optional_without, mode = sys.argv[1:9]
findings = []
with open(report) as fh:
    for line in fh:
        line = line.strip()
        if not line:
            continue
        kind, _, rest = line.partition(": ")
        findings.append({"kind": kind, "detail": rest})
with open(catalog) as fh:
    slugs = [l.strip() for l in fh if l.strip()]
print(json.dumps({
    "mode": mode,
    "slug_count": len(slugs),
    "primitives_scanned": int(total),
    "with_practices_field": int(declared),
    "missing_practices_field": int(missing),
    "declaration_optional_without_practices": int(optional_without),
    "invalid_slug_citations": int(invalid),
    "findings": findings,
}, indent=2))
PY
else
  echo "=== Practice citation report ==="
  echo "Slug catalog: $slug_count slugs from $PRACTICE_FILE"
  echo "Primitives scanned: $total"
  echo "  with practices field: $declared"
  echo "  missing practices field: $missing"
  echo "  declaration-optional without practices: $optional_without_practices"
  echo "  invalid slug citations: $invalid"
  echo ""
  if [[ -s "$REPORT" ]]; then
    echo "--- Findings ---"
    cat "$REPORT"
  else
    echo "No findings."
  fi
fi

if [[ "$STRICT" == "true" ]] && { [[ "$missing" -gt 0 ]] || [[ "$invalid" -gt 0 ]]; }; then
  exit 1
fi

exit 0
