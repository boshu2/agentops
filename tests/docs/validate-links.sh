#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ALLOWLIST="$REPO_ROOT/tests/docs/broken-links-allowlist.txt"

total=0
broken=0
allowlisted=0
generated=0

# Load allowlist into associative array
declare -A allowed
if [[ -f "$ALLOWLIST" ]]; then
  while IFS= read -r line; do
    [[ -z "$line" || "$line" == \#* ]] && continue
    allowed["$line"]=1
  done < "$ALLOWLIST"
fi

# Build a set of paths that are generated at MkDocs build time (docs/_hooks/gen_*.py).
# These files don't exist on disk but resolve at build time via the mkdocs-gen-files
# plugin. Link targets pointing to these paths are intentional; MkDocs strict build
# validates them end-to-end via scripts/docs-build.sh --check.
declare -A generated_paths
# CLI reference pages (from docs/_hooks/gen_cli_reference.py)
generated_paths["$REPO_ROOT/docs/cli/index.md"]=1
generated_paths["$REPO_ROOT/docs/cli/commands.md"]=1
generated_paths["$REPO_ROOT/docs/cli/hooks.md"]=1
# Skill catalog + index (from docs/_hooks/gen_skill_pages.py)
generated_paths["$REPO_ROOT/docs/skills/index.md"]=1
generated_paths["$REPO_ROOT/docs/skills/catalog.md"]=1
# Individual skill pages — one per directory under skills/
if [[ -d "$REPO_ROOT/skills" ]]; then
  while IFS= read -r skill_dir; do
    [[ -z "$skill_dir" ]] && continue
    slug="$(basename "$skill_dir")"
    generated_paths["$REPO_ROOT/docs/skills/${slug}.md"]=1
  done < <(find "$REPO_ROOT/skills" -mindepth 1 -maxdepth 1 -type d)
fi

# Find all markdown files in specified directories
md_files=()
while IFS= read -r f; do
  md_files+=("$f")
done < <(find "$REPO_ROOT" -maxdepth 1 -name '*.md' -type f)

for dir in docs skills skills-codex cli; do
  if [[ -d "$REPO_ROOT/$dir" ]]; then
    while IFS= read -r f; do
      md_files+=("$f")
    done < <(find "$REPO_ROOT/$dir" -name '*.md' -type f -not -path '*/.agents/*')
  fi
done

for file in "${md_files[@]}"; do
  rel_file="${file#"$REPO_ROOT"/}"
  file_dir="$(dirname "$file")"

  # Extract all links with line numbers in one pass using grep -n
  while IFS= read -r match; do
    [[ -z "$match" ]] && continue
    line_num="${match%%:*}"
    target="${match#*:}"

    # Skip external URLs
    [[ "$target" == http://* || "$target" == https://* ]] && continue
    # Skip anchor-only links
    [[ "$target" == \#* ]] && continue
    # Skip mailto links
    [[ "$target" == mailto:* ]] && continue
    # Skip empty
    [[ -z "$target" ]] && continue

    # Strip anchor fragment
    target_path="${target%%#*}"
    [[ -z "$target_path" ]] && continue

    # Strip trailing whitespace and quotes (image title syntax)
    target_path="${target_path%% *}"

    total=$((total + 1))

    # Resolve path relative to the linking file's directory
    if [[ "$target_path" == /* ]]; then
      resolved="$target_path"
    else
      resolved="$file_dir/$target_path"
    fi

    if [[ ! -e "$resolved" ]]; then
      # Normalize the resolved path so generated-path lookups line up.
      # The mkdocs-generated paths have no on-disk parent dir in CI (docs/skills/
      # and docs/cli/ exist only via gen-files plugin). Fall back to the raw
      # resolved path when the parent dir does not exist — do NOT use inline
      # short-circuit `|| canonical=...` because that never fires when the
      # outer assignment itself does not fail.
      if resolved_parent="$(cd "$(dirname "$resolved")" 2>/dev/null && pwd)"; then
        canonical="$resolved_parent/$(basename "$resolved")"
      else
        canonical="$resolved"
      fi
      if [[ -n "${generated_paths[$canonical]+x}" ]]; then
        generated=$((generated + 1))
        continue
      fi
      allowlist_key="$rel_file:$target_path"
      if [[ -n "${allowed[$allowlist_key]+x}" ]]; then
        allowlisted=$((allowlisted + 1))
      else
        broken=$((broken + 1))
        echo "BROKEN: $rel_file:$line_num -> $target_path"
      fi
    fi
  done < <(grep -noE '\]\([^)]+\)' "$file" 2>/dev/null | sed 's/:\]/:/;s/)$//' | sed 's/^\([0-9]*\):\(.*\)$/\1:\2/' | sed 's/^\([0-9]*\):(/\1:/')
done

echo ""
echo "$total links checked, $broken broken ($allowlisted allowlisted, $generated mkdocs-generated)"

if [[ "$broken" -gt 0 ]]; then
  exit 1
fi

exit 0
