#!/usr/bin/env bash
set -euo pipefail

# check-agents-write-surfaces.sh — guard the .agents/ write-surface contract.
#
# Every top-level subdir under .agents/ that production code (cli/**/*.go
# non-test, scripts/, hooks/, lib/) writes to must be catalogued in
# docs/contracts/agents-write-surfaces.md. The catalog has an explicit
# allowlist between BEGIN/END markers and a typed surfaces table; this script
# parses both and fails when production code references a subdir that isn't
# documented or when documented surfaces lack valid lifecycle/writer metadata.
#
# Skill-owned subdirs (.agents/<skill-name>/) are auto-allowed when an
# active skill exists at skills/<skill-name>/SKILL.md. New skills don't
# need a doc edit; new CLI/script/hook write surfaces do.

usage() {
  cat <<USAGE
Usage: $0 [--json]

Options:
  --json   Emit a machine-readable summary instead of human prose.

Exit codes:
  0  OK (all referenced subdirs documented or skill-owned, classifications valid)
  1  Undocumented top-level subdirs or classification drift found
  2  Bad invocation or missing contract doc
USAGE
}

JSON=false
case "${1:-}" in
  --json) JSON=true ;;
  -h|--help) usage; exit 0 ;;
  '') ;;
  *) usage; exit 2 ;;
esac

if [[ -n "${AGENTS_WRITE_SURFACES_REPO_ROOT:-}" ]]; then
  REPO_ROOT="$(cd "$AGENTS_WRITE_SURFACES_REPO_ROOT" && pwd)"
else
  REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
fi
CONTRACT_DOC="$REPO_ROOT/docs/contracts/agents-write-surfaces.md"
SKILLS_DIR="$REPO_ROOT/skills"

json_escape() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"
  s="${s//$'\r'/\\r}"
  s="${s//$'\t'/\\t}"
  printf '%s' "$s"
}

if [[ ! -f "$CONTRACT_DOC" ]]; then
  echo "ERROR: contract doc missing: $CONTRACT_DOC" >&2
  exit 2
fi

# Parse allowlist between markers. Lines starting with '#' are treated as
# comments. Empty lines are ignored. Each remaining line must be a single
# top-level subdir name with no slashes.
allowlist_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp"' EXIT
awk '
  /^[[:space:]]*<!-- BEGIN agents-write-surfaces-allowlist -->[[:space:]]*$/ { inside=1; next }
  /^[[:space:]]*<!-- END agents-write-surfaces-allowlist -->[[:space:]]*$/   { inside=0; next }
  inside { print }
' "$CONTRACT_DOC" \
  | sed -E 's/[[:space:]]+#.*$//' \
  | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' \
  | awk 'NF && $1 !~ /^#/' \
  | sort -u > "$allowlist_tmp"

if [[ ! -s "$allowlist_tmp" ]]; then
  echo "ERROR: allowlist block is empty or markers missing in $CONTRACT_DOC" >&2
  exit 2
fi

# Reject malformed entries (must be lowercase letters, digits, '-' or '_').
malformed="$(grep -vE '^[a-z0-9][a-z0-9_-]*$' "$allowlist_tmp" || true)"
if [[ -n "$malformed" ]]; then
  echo "ERROR: malformed allowlist entries in $CONTRACT_DOC:" >&2
  echo "$malformed" >&2
  exit 2
fi

# Parse the typed surfaces table. Each row must use this shape:
# | `surface` | `lifecycle` | `writer`, `writer` | mutation lane | purpose |
# Lifecycle and writer values are intentionally restricted to the vocabulary
# documented in the contract so new states cannot appear without policy review.
contract_tmp="$(mktemp)"
classification_errors_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp" "$contract_tmp" "$classification_errors_tmp"' EXIT
awk -v out="$contract_tmp" -v err="$classification_errors_tmp" '
  function trim(s) {
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", s)
    return s
  }
  function clean(s) {
    s = trim(s)
    gsub(/`/, "", s)
    return trim(s)
  }
  BEGIN {
    lifecycle["persistent"] = 1
    lifecycle["rolling"] = 1
    lifecycle["regenerated"] = 1
    lifecycle["runtime-only"] = 1
    lifecycle["ignored"] = 1
    writer["cli"] = 1
    writer["hooks"] = 1
    writer["scripts"] = 1
    writer["skills"] = 1
    writer["operators"] = 1
    writer["tests"] = 1
  }
  /^\|[[:space:]]*`/ {
    n = split($0, cells, "|")
    if (n < 7) {
      next
    }
    surface = clean(cells[2])
    if (surface == "Surface" || surface == "") {
      next
    }
    life = clean(cells[3])
    writers = clean(cells[4])
    lane = trim(cells[5])
    if (surface !~ /^[A-Za-z0-9][A-Za-z0-9._-]*$/) {
      print "surface " surface " has malformed surface name" >> err
      next
    }
    if (!(life in lifecycle)) {
      print surface " has unknown lifecycle: " life >> err
    }
    if (writers == "") {
      print surface " has missing allowed writers" >> err
    } else {
      writer_count = split(writers, writer_cells, ",")
      for (i = 1; i <= writer_count; i++) {
        w = clean(writer_cells[i])
        if (!(w in writer)) {
          print surface " has unknown writer: " w >> err
        }
      }
    }
    if (lane == "" || lane ~ /TBD|TODO|FIXME/) {
      print surface " has missing mutation lane" >> err
    }
    print surface "\t" life "\t" writers "\t" lane >> out
  }
' "$CONTRACT_DOC"

if [[ ! -s "$contract_tmp" ]]; then
  echo "ERROR: surfaces table is empty or malformed in $CONTRACT_DOC" >&2
  exit 2
fi

# Extract referenced top-level subdirs from production code.
# - cli/**/*.go excluding _test.go
# - scripts/*.sh, hooks/*.sh, lib/*.sh
referenced_tmp="$(mktemp)"
references_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp" "$contract_tmp" "$classification_errors_tmp" "$referenced_tmp" "$references_tmp"' EXIT

scan_agent_references() {
  local file rel match subdir
  while IFS= read -r -d '' file; do
    rel="${file#"$REPO_ROOT"/}"
    while IFS= read -r match; do
      [[ -z "$match" ]] && continue
      subdir="${match#.agents/}"
      printf '%s\t%s\n' "$subdir" "$rel"
    done < <(grep -Eo '\.agents/[a-z][a-zA-Z0-9_-]*' "$file" 2>/dev/null || true)
    while IFS= read -r match; do
      [[ -z "$match" ]] && continue
      subdir="$(sed -E 's/^.*"[[:space:]]*,[[:space:]]*"([a-z][a-zA-Z0-9_-]*).*$/\1/' <<<"$match")"
      printf '%s\t%s\n' "$subdir" "$rel"
    done < <(grep -Eo 'filepath\.Join\([^)]*\.agents"[[:space:]]*,[[:space:]]*"[a-z][a-zA-Z0-9_-]*' "$file" 2>/dev/null || true)
  done
}

scan_dirs=()
[[ -d "$REPO_ROOT/scripts" ]] && scan_dirs+=("$REPO_ROOT/scripts")
[[ -d "$REPO_ROOT/hooks" ]]   && scan_dirs+=("$REPO_ROOT/hooks")
[[ -d "$REPO_ROOT/lib" ]]     && scan_dirs+=("$REPO_ROOT/lib")

{
  if [[ -d "$REPO_ROOT/cli" ]]; then
    scan_agent_references < <(find "$REPO_ROOT/cli" -type f -name '*.go' ! -name '*_test.go' -print0 2>/dev/null)
  fi
  if [[ ${#scan_dirs[@]} -gt 0 ]]; then
    scan_agent_references < <(find "${scan_dirs[@]}" -type f \( -name '*.sh' -o -name '*.bash' \) -print0 2>/dev/null)
  fi
} | sort -u > "$references_tmp"
cut -f1 "$references_tmp" | sort -u > "$referenced_tmp"

# Compute active skill names — skill-owned subdirs are auto-allowed.
skills_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp" "$contract_tmp" "$classification_errors_tmp" "$referenced_tmp" "$references_tmp" "$skills_tmp"' EXIT
: > "$skills_tmp"
if [[ -d "$SKILLS_DIR" ]]; then
  shopt -s nullglob
  for d in "$SKILLS_DIR"/*/; do
    [[ -f "${d}SKILL.md" ]] && basename "$d" >> "$skills_tmp"
  done
  shopt -u nullglob
  if [[ -s "$skills_tmp" ]]; then
    sort -u "$skills_tmp" -o "$skills_tmp"
  fi
fi

# undocumented = referenced - allowlist - skills
undocumented_tmp="$(mktemp)"
tracked_surfaces_tmp="$(mktemp)"
needed_classifications_tmp="$(mktemp)"
policy_classifications_tmp="$(mktemp)"
contracted_surfaces_tmp="$(mktemp)"
missing_classifications_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp" "$contract_tmp" "$classification_errors_tmp" "$referenced_tmp" "$references_tmp" "$skills_tmp" "$undocumented_tmp" "$tracked_surfaces_tmp" "$needed_classifications_tmp" "$policy_classifications_tmp" "$contracted_surfaces_tmp" "$missing_classifications_tmp"' EXIT
comm -23 "$referenced_tmp" "$allowlist_tmp" \
  | comm -23 - "$skills_tmp" > "$undocumented_tmp"

# Repo-tracked top-level .agents surfaces must also be classified. In a real
# repo, use git so ignored runtime scratch does not become policy by accident.
# In tests or unpacked source trees without git metadata, fall back to existing
# top-level entries under .agents/.
: > "$tracked_surfaces_tmp"
if [[ -d "$REPO_ROOT/.agents" ]]; then
  if git -C "$REPO_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$REPO_ROOT" ls-files .agents 2>/dev/null \
      | awk -F / '$1 == ".agents" && NF >= 2 { print $2 }' \
      | sort -u > "$tracked_surfaces_tmp"
  else
    find "$REPO_ROOT/.agents" -mindepth 1 -maxdepth 1 -print 2>/dev/null \
      | sed -E 's#^.*/##' \
      | sort -u > "$tracked_surfaces_tmp"
  fi
fi

cat "$allowlist_tmp" "$tracked_surfaces_tmp" | sort -u > "$needed_classifications_tmp"
comm -23 "$needed_classifications_tmp" "$skills_tmp" > "$policy_classifications_tmp"
cut -f1 "$contract_tmp" | sort -u > "$contracted_surfaces_tmp"
comm -23 "$policy_classifications_tmp" "$contracted_surfaces_tmp" > "$missing_classifications_tmp"

UNDOC_COUNT=$(wc -l < "$undocumented_tmp" | tr -d ' ')
MISSING_CLASSIFICATION_COUNT=$(wc -l < "$missing_classifications_tmp" | tr -d ' ')
CLASSIFICATION_ERROR_COUNT=$(wc -l < "$classification_errors_tmp" | tr -d ' ')
ALLOW_COUNT=$(wc -l < "$allowlist_tmp" | tr -d ' ')
REF_COUNT=$(wc -l < "$referenced_tmp" | tr -d ' ')

if [[ "$JSON" == "true" ]]; then
  printf '{"contract":"%s","allowlist_size":%s,"referenced":%s,"undocumented":[' \
    "$(json_escape "${CONTRACT_DOC#"$REPO_ROOT"/}")" "$ALLOW_COUNT" "$REF_COUNT"
  first=1
  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    if [[ "$first" -eq 1 ]]; then first=0; else printf ','; fi
    printf '"%s"' "$(json_escape "$entry")"
  done < "$undocumented_tmp"
  printf '],"missing_classifications":['
  first=1
  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    if [[ "$first" -eq 1 ]]; then first=0; else printf ','; fi
    printf '"%s"' "$(json_escape "$entry")"
  done < "$missing_classifications_tmp"
  printf '],"classification_errors":['
  first=1
  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    if [[ "$first" -eq 1 ]]; then first=0; else printf ','; fi
    printf '"%s"' "$(json_escape "$entry")"
  done < "$classification_errors_tmp"
  printf '],"source_locations":{'
  first=1
  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    if [[ "$first" -eq 1 ]]; then first=0; else printf ','; fi
    printf '"%s":[' "$(json_escape "$entry")"
    file_first=1
    while IFS= read -r source_file; do
      [[ -z "$source_file" ]] && continue
      if [[ "$file_first" -eq 1 ]]; then file_first=0; else printf ','; fi
      printf '"%s"' "$(json_escape "$source_file")"
    done < <(awk -F '\t' -v key="$entry" '$1 == key { print $2 }' "$references_tmp" | sort -u)
    printf ']'
  done < "$referenced_tmp"
  printf '},"status":"%s"}\n' "$([ "$UNDOC_COUNT" -gt 0 ] || [ "$MISSING_CLASSIFICATION_COUNT" -gt 0 ] || [ "$CLASSIFICATION_ERROR_COUNT" -gt 0 ] && echo fail || echo ok)"
fi

if [[ "$UNDOC_COUNT" -gt 0 || "$MISSING_CLASSIFICATION_COUNT" -gt 0 || "$CLASSIFICATION_ERROR_COUNT" -gt 0 ]]; then
  if [[ "$JSON" != "true" ]]; then
    if [[ "$UNDOC_COUNT" -gt 0 ]]; then
      echo "ERROR: $UNDOC_COUNT undocumented .agents/ write surface(s) found in production code." >&2
      echo "Add an entry under '<!-- BEGIN agents-write-surfaces-allowlist -->' in:" >&2
      echo "  $CONTRACT_DOC" >&2
      echo "Undocumented subdirs:" >&2
      while IFS= read -r entry; do
        [[ -z "$entry" ]] && continue
        echo "  - $entry" >&2
        while IFS= read -r source_file; do
          [[ -z "$source_file" ]] && continue
          echo "      $source_file" >&2
        done < <(awk -F '\t' -v key="$entry" '$1 == key { print $2 }' "$references_tmp" | sort -u | head -5)
      done < "$undocumented_tmp"
    fi
    if [[ "$MISSING_CLASSIFICATION_COUNT" -gt 0 ]]; then
      echo "ERROR: $MISSING_CLASSIFICATION_COUNT .agents/ surface(s) missing table classifications in $CONTRACT_DOC:" >&2
      while IFS= read -r entry; do
        [[ -z "$entry" ]] && continue
        echo "  - $entry" >&2
      done < "$missing_classifications_tmp"
    fi
    if [[ "$CLASSIFICATION_ERROR_COUNT" -gt 0 ]]; then
      echo "ERROR: $CLASSIFICATION_ERROR_COUNT invalid .agents/ classification(s) in $CONTRACT_DOC:" >&2
      while IFS= read -r entry; do
        [[ -z "$entry" ]] && continue
        echo "  - $entry" >&2
      done < "$classification_errors_tmp"
    fi
  fi
  exit 1
fi

if [[ "$JSON" != "true" ]]; then
  echo "agents-write-surfaces contract OK: $REF_COUNT referenced subdirs, $ALLOW_COUNT allowlisted, all documented/classified or skill-owned."
fi
exit 0
