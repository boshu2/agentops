# Pre-Council Metadata Verification

**Mechanically verify delivered artifacts against the plan BEFORE council. Catches metadata errors that LLMs estimate instead of measure (L19, L22, L24).**

```bash
METADATA_FAILURES=""

# 1. Plan vs actual file list — did we deliver what we said we would?
if [ -n "$PLAN_DOC" ] && [ -f "$PLAN_DOC" ]; then
  # Extract file paths mentioned in plan
  for planned_file in $(grep -oE '`[^`]+\.(go|py|ts|js|md|yaml|yml|sh)`' "$PLAN_DOC" 2>/dev/null | tr -d '`' | sort -u); do
    if [ ! -f "$planned_file" ]; then
      METADATA_FAILURES="${METADATA_FAILURES}\n- PLANNED BUT MISSING: $planned_file (in plan but not on disk)"
    fi
  done
fi

# 2. Claimed metrics vs measured — line counts, issue counts, file counts
# Check git log for claimed counts in commit messages
for commit_msg in $(git log --oneline --since="7 days ago" --format="%s" 2>/dev/null); do
  # Handled inline during council context building
  :
done

# 3. File existence — all paths in recent commits exist
for f in $(git diff --name-only HEAD~10 2>/dev/null | sort -u); do
  if [ ! -f "$f" ] && ! git log --diff-filter=D --name-only --format="" HEAD~10..HEAD | grep -q "^${f}$"; then
    METADATA_FAILURES="${METADATA_FAILURES}\n- MISSING FILE: $f (in commits but not on disk, not intentionally deleted)"
  fi
done

# 4. Cross-references in delivered docs
declare -A METADATA_LINK_ALLOWLIST=()
if [ -f tests/docs/broken-links-allowlist.txt ]; then
  while IFS= read -r allowlisted_link; do
    [[ -z "$allowlisted_link" || "$allowlisted_link" == \#* ]] && continue
    METADATA_LINK_ALLOWLIST["$allowlisted_link"]=1
  done < tests/docs/broken-links-allowlist.txt
fi

is_illustrative_metadata_path() {
  local ref_path="$1"
  case "$ref_path" in
    url|./path/to/*|../path/to/*|*/path/to/*|/Users/me/*)
      return 0
      ;;
  esac
  return 1
}

for f in $(git diff --name-only HEAD~10 2>/dev/null | grep -E '\.(md|txt)$'); do
  if [ -f "$f" ]; then
    while IFS= read -r ref; do
      [[ -z "$ref" ]] && continue
      [[ "$ref" == http://* || "$ref" == https://* ]] && continue
      [[ "$ref" == mailto:* ]] && continue
      [[ "$ref" == \#* ]] && continue

      ref="${ref%%#*}"
      ref="${ref%% *}"
      [[ -z "$ref" ]] && continue
      is_illustrative_metadata_path "$ref" && continue

      ref_dir=$(dirname "$f")
      allowlist_key="${f}:${ref}"
      [[ -n "${METADATA_LINK_ALLOWLIST[$allowlist_key]+x}" ]] && continue

      if [ ! -f "$ref_dir/$ref" ] && [ ! -f "$ref" ]; then
        METADATA_FAILURES="${METADATA_FAILURES}\n- BROKEN LINK: $f references $ref (not found)"
      fi
    done < <(
      grep -oE '\]\([^)]+\)' "$f" 2>/dev/null |
        cut -c3- |
        sed 's/)$//'
    )
  fi
done

# 5. ASCII diagram verification (>3 boxes per L22)
for f in $(git diff --name-only HEAD~10 2>/dev/null | grep -E '\.(md|txt)$'); do
  if [ -f "$f" ]; then
    box_count=$(grep -cP '┌|╔|\+--' "$f" 2>/dev/null || echo 0)
    if [ "$box_count" -gt 3 ]; then
      label_count=$(grep -cP '│\s+\S' "$f" 2>/dev/null || echo 0)
      if [ "$box_count" -gt "$label_count" ]; then
        METADATA_FAILURES="${METADATA_FAILURES}\n- DIAGRAM CHECK: $f has ${box_count} boxes but only ${label_count} label lines — verify"
      fi
    fi
  fi
done

# Report
if [ -n "$METADATA_FAILURES" ]; then
  echo "METADATA VERIFICATION FAILURES:"
  echo -e "$METADATA_FAILURES"
else
  echo "Metadata verification: all checks passed"
fi
```

**If failures found:** Include them in the council packet as `context.metadata_failures`. Tag as MECHANICAL — council judges should focus on plan compliance, tech debt, and learnings instead of re-discovering broken links or wrong counts.

**If plan doc found:** Compare planned deliverables (file paths, issue counts) against actual. Mismatches become pre-loaded findings for the `plan-compliance` judge.

**Why:** Post-mortem councils were spending judge cycles on metadata errors (wrong line counts, missing files, broken cross-refs) that are mechanically verifiable. Pre-council verification frees judges to focus on structural assessment and learning extraction.
