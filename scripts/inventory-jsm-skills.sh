#!/usr/bin/env bash
# inventory-jsm-skills.sh - read-only inventory for JSM skill roots or backups.
#
# Usage:
#   scripts/inventory-jsm-skills.sh
#   scripts/inventory-jsm-skills.sh --root "$HOME/.claude/skills" --json
#   scripts/inventory-jsm-skills.sh --backup-root "$HOME/backups/jsm-skills-YYYYMMDD-HHMMSS" --markdown
set -euo pipefail

FORMAT="json"
ROOTS=()

usage() {
    sed -n '2,8p' "$0"
}

count_lines() {
    wc -l | tr -d ' '
}

count_find() {
    local root=$1
    shift
    find "$root" "$@" 2>/dev/null | count_lines
}

extension_counts_from_paths() {
    awk '
      {
        name=$0
        sub(/^.*\//, "", name)
        if (name !~ /\./ || name ~ /^\.[^.]+$/) {
          ext="[none]"
        } else {
          ext=name
          sub(/^.*\./, ".", ext)
          ext=tolower(ext)
        }
        counts[ext]++
      }
      END {
        for (ext in counts) {
          printf "%s\t%d\n", ext, counts[ext]
        }
      }
    ' | sort | jq -Rn '[inputs | select(length > 0) | split("\t") | {extension: .[0], count: (.[1] | tonumber)}]'
}

direct_skill_dirs_with() {
    local root=$1
    local child=$2
    find "$root" -mindepth 2 -maxdepth 2 -type d -name "$child" 2>/dev/null | count_lines
}

direct_skill_files_named() {
    local root=$1
    local name=$2
    find "$root" -mindepth 2 -maxdepth 2 -type f -name "$name" 2>/dev/null | count_lines
}

dir_summary() {
    local root=$1
    local ext_json
    ext_json=$(find "$root" -type f 2>/dev/null | extension_counts_from_paths)

    jq -n \
        --arg root "$root" \
        --arg kind "directory" \
        --argjson top_level_dirs "$(count_find "$root" -mindepth 1 -maxdepth 1 -type d)" \
        --argjson user_skill_dirs "$(find "$root" -mindepth 1 -maxdepth 1 -type d ! -name '.*' 2>/dev/null | count_lines)" \
        --argjson system_dirs "$(find "$root" -mindepth 1 -maxdepth 1 -type d -name '.*' 2>/dev/null | count_lines)" \
        --argjson top_level_skill_md_files "$(find "$root" -mindepth 2 -maxdepth 2 -type f -name SKILL.md 2>/dev/null | count_lines)" \
        --argjson all_skill_md_files "$(count_find "$root" -type f -name SKILL.md)" \
        --argjson total_files "$(count_find "$root" -type f)" \
        --argjson symlinks "$(count_find "$root" -type l)" \
        --argjson packages_with_references "$(direct_skill_dirs_with "$root" references)" \
        --argjson packages_with_scripts "$(direct_skill_dirs_with "$root" scripts)" \
        --argjson packages_with_assets "$(direct_skill_dirs_with "$root" assets)" \
        --argjson packages_with_subagents "$(direct_skill_dirs_with "$root" subagents)" \
        --argjson packages_with_prompts "$(direct_skill_dirs_with "$root" prompts)" \
        --argjson packages_with_self_test "$(direct_skill_files_named "$root" SELF-TEST.md)" \
        --argjson packages_with_readme "$(direct_skill_files_named "$root" README.md)" \
        --argjson executable_script_files "$(find "$root" -path '*/scripts/*' -type f -perm -111 2>/dev/null | count_lines)" \
        --argjson reference_files "$(find "$root" -path '*/references/*' -type f 2>/dev/null | count_lines)" \
        --argjson script_files "$(find "$root" -path '*/scripts/*' -type f 2>/dev/null | count_lines)" \
        --argjson asset_files "$(find "$root" -path '*/assets/*' -type f 2>/dev/null | count_lines)" \
        --argjson subagent_files "$(find "$root" -path '*/subagents/*' -type f 2>/dev/null | count_lines)" \
        --argjson extension_counts "$ext_json" \
        '{
          root: $root,
          kind: $kind,
          top_level_dirs: $top_level_dirs,
          user_skill_dirs: $user_skill_dirs,
          system_dirs: $system_dirs,
          top_level_skill_md_files: $top_level_skill_md_files,
          all_skill_md_files: $all_skill_md_files,
          total_files: $total_files,
          symlinks: $symlinks,
          packages_with_references: $packages_with_references,
          packages_with_scripts: $packages_with_scripts,
          packages_with_assets: $packages_with_assets,
          packages_with_subagents: $packages_with_subagents,
          packages_with_prompts: $packages_with_prompts,
          packages_with_self_test: $packages_with_self_test,
          packages_with_readme: $packages_with_readme,
          executable_script_files: $executable_script_files,
          reference_files: $reference_files,
          script_files: $script_files,
          asset_files: $asset_files,
          subagent_files: $subagent_files,
          extension_counts: $extension_counts
        }'
}

archive_contains_count() {
    local list_file=$1
    local pattern=$2
    while IFS= read -r archive; do
        if tar -tzf "$archive" | grep -Eq "$pattern"; then
            printf '%s\n' "$archive"
        fi
    done < "$list_file" | count_lines
}

archive_summary() {
    local root=$1
    local archives entries files ext_json
    archives=$(mktemp)
    entries=$(mktemp)
    trap 'rm -f "$archives" "$entries"' RETURN

    find "$root" -path '*/skills/*.tar.gz' -type f 2>/dev/null | sort > "$archives"
    while IFS= read -r archive; do
        tar -tzf "$archive"
    done < "$archives" > "$entries"

    files=$(grep -Ev '/$' "$entries" | count_lines)
    ext_json=$(grep -Ev '/$' "$entries" | extension_counts_from_paths)

    jq -n \
        --arg root "$root" \
        --arg kind "backup_archives" \
        --argjson archive_count "$(wc -l < "$archives" | tr -d ' ')" \
        --argjson total_files "$files" \
        --argjson skill_md_files "$(grep -Ec '(^|/)SKILL[.]md$' "$entries" || true)" \
        --argjson packages_with_references "$(archive_contains_count "$archives" '/references/')" \
        --argjson packages_with_scripts "$(archive_contains_count "$archives" '/scripts/')" \
        --argjson packages_with_assets "$(archive_contains_count "$archives" '/assets/')" \
        --argjson packages_with_subagents "$(archive_contains_count "$archives" '/subagents/')" \
        --argjson packages_with_self_test "$(archive_contains_count "$archives" '/SELF-TEST[.]md$')" \
        --argjson packages_with_readme "$(archive_contains_count "$archives" '/README[.]md$')" \
        --argjson extension_counts "$ext_json" \
        '{
          root: $root,
          kind: $kind,
          archive_count: $archive_count,
          total_files: $total_files,
          skill_md_files: $skill_md_files,
          packages_with_references: $packages_with_references,
          packages_with_scripts: $packages_with_scripts,
          packages_with_assets: $packages_with_assets,
          packages_with_subagents: $packages_with_subagents,
          packages_with_self_test: $packages_with_self_test,
          packages_with_readme: $packages_with_readme,
          extension_counts: $extension_counts
        }'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --root)
            ROOTS+=("$2")
            shift 2
            ;;
        --backup-root)
            ROOTS+=("$2")
            shift 2
            ;;
        --json)
            FORMAT="json"
            shift
            ;;
        --markdown)
            FORMAT="markdown"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [[ "${#ROOTS[@]}" -eq 0 ]]; then
    [[ -d "${HOME}/.claude/skills" ]] && ROOTS+=("${HOME}/.claude/skills")
    [[ -d "${HOME}/.codex/skills" ]] && ROOTS+=("${HOME}/.codex/skills")
fi

objects=$(mktemp)
trap 'rm -f "$objects"' EXIT
for root in "${ROOTS[@]}"; do
    if [[ ! -e "$root" ]]; then
        echo "Missing root: $root" >&2
        exit 1
    fi
    if [[ -n "$(find "$root" -path '*/skills/*.tar.gz' -type f -print -quit 2>/dev/null)" ]]; then
        archive_summary "$root" >> "$objects"
    else
        dir_summary "$root" >> "$objects"
    fi
done

generated_at=$(date -Iseconds)
if [[ "$FORMAT" == "json" ]]; then
    jq -s --arg generated_at "$generated_at" '{generated_at: $generated_at, roots: .}' "$objects"
else
    jq -s -r --arg generated_at "$generated_at" '
      "# JSM Skill Inventory\n\nGenerated: \($generated_at)\n\n" +
      (.[] | "## \(.root)\n\nKind: `\(.kind)`\n\n" +
      (if .kind == "directory" then
        "| Measure | Value |\n|---|---:|\n" +
        "| Top-level dirs | \(.top_level_dirs) |\n" +
        "| User skill dirs | \(.user_skill_dirs) |\n" +
        "| System dirs | \(.system_dirs) |\n" +
        "| Top-level SKILL.md files | \(.top_level_skill_md_files) |\n" +
        "| All SKILL.md files | \(.all_skill_md_files) |\n" +
        "| Total files | \(.total_files) |\n" +
        "| Symlinks | \(.symlinks) |\n" +
        "| Packages with references | \(.packages_with_references) |\n" +
        "| Packages with scripts | \(.packages_with_scripts) |\n" +
        "| Packages with assets | \(.packages_with_assets) |\n" +
        "| Packages with subagents | \(.packages_with_subagents) |\n" +
        "| Packages with SELF-TEST.md | \(.packages_with_self_test) |\n" +
        "| Executable script files | \(.executable_script_files) |\n\n"
      else
        "| Measure | Value |\n|---|---:|\n" +
        "| Archives | \(.archive_count) |\n" +
        "| Total files in archives | \(.total_files) |\n" +
        "| SKILL.md files | \(.skill_md_files) |\n" +
        "| Packages with references | \(.packages_with_references) |\n" +
        "| Packages with scripts | \(.packages_with_scripts) |\n" +
        "| Packages with assets | \(.packages_with_assets) |\n" +
        "| Packages with subagents | \(.packages_with_subagents) |\n" +
        "| Packages with SELF-TEST.md | \(.packages_with_self_test) |\n\n"
      end))' "$objects"
fi
