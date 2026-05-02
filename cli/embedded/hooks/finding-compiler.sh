#!/usr/bin/env bash
# AgentOps Hook Helper: finding-compiler
# Promotes structured findings into durable artifacts and compiles them into
# planning rules, pre-mortem checks, and declarative constraint metadata.
#
# Usage:
#   bash hooks/finding-compiler.sh                  # promote registry + compile all findings
#   bash hooks/finding-compiler.sh <path> [...]    # compile registry, finding artifacts, or legacy learnings
set -euo pipefail

[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

# shellcheck disable=SC2034 # available for helper sourcing
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"

# Source the canonical state-path resolver (soc-irg1.1) so AO_HOME /
# CLAUDE_PLUGIN_DATA precedence is honored. Fail-open if the resolver is
# missing — fall back to the legacy ${ROOT}/.agents layout.
if [ -z "${AO_AGENTS_DIR:-}" ] && [ -f "$ROOT/lib/ao-paths.sh" ]; then
    eval "$(bash "$ROOT/lib/ao-paths.sh" 2>/dev/null)" 2>/dev/null || true
fi

AGENTS_DIR="${AO_AGENTS_DIR:-$ROOT/.agents}"
FINDINGS_DIR="${AO_FINDINGS_DIR:-$AGENTS_DIR/findings}"
PLANNING_RULES_DIR="$AGENTS_DIR/planning-rules"
PREMORTEM_CHECKS_DIR="$AGENTS_DIR/pre-mortem-checks"
CONSTRAINT_DIR="$AGENTS_DIR/constraints"
CONSTRAINT_INDEX="$CONSTRAINT_DIR/index.json"
CONSTRAINT_LOCK="$CONSTRAINT_DIR/compile.lock"

QUIET=0
declare -a INPUTS=()

for arg in "$@"; do
    case "$arg" in
        --quiet)
            QUIET=1
            ;;
        *)
            INPUTS+=("$arg")
            ;;
    esac
done

note() {
    if [ "$QUIET" -ne 1 ]; then
        printf '%s\n' "$*"
    fi
}

warn() {
    if [ "$QUIET" -ne 1 ]; then
        printf 'WARN: %s\n' "$*" >&2
    fi
}

require_tooling() {
    if ! command -v jq >/dev/null 2>&1; then
        warn "jq not found; skipping finding compilation"
        exit 0
    fi
    if ! command -v ruby >/dev/null 2>&1; then
        warn "ruby not found; skipping finding compilation"
        exit 0
    fi
}

write_atomic() {
    local target="$1"
    local mode="${2:-644}"
    local dir tmp

    dir=$(dirname -- "$target")
    mkdir -p "$dir"
    tmp=$(mktemp "$dir/.tmp.XXXXXX")
    cat > "$tmp"
    chmod "$mode" "$tmp"
    mv "$tmp" "$target"
}

remove_file_if_exists() {
    local target="$1"
    if [ -e "$target" ]; then
        rm -f "$target"
    fi
}

relative_to_root() {
    local target="$1"
    case "$target" in
        "$ROOT"/*)
            printf '%s\n' "${target#"$ROOT"/}"
            ;;
        *)
            printf '%s\n' "$target"
            ;;
    esac
}

slugify() {
    printf '%s' "$1" \
        | tr '[:upper:]' '[:lower:]' \
        | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-+/-/g'
}

derive_dedup_key() {
    local category="$1"
    local pattern="$2"
    local primary_when="$3"
    printf '%s|%s|%s\n' \
        "$(slugify "$category")" \
        "$(slugify "$pattern")" \
        "$(slugify "$primary_when")"
}

extract_markdown_body() {
    local path="$1"
    awk '
        BEGIN { in_frontmatter = 0; seen_frontmatter = 0 }
        NR == 1 && $0 == "---" { in_frontmatter = 1; seen_frontmatter = 1; next }
        in_frontmatter && $0 == "---" { in_frontmatter = 0; next }
        !in_frontmatter { print }
    ' "$path"
}

first_paragraph() {
    awk '
        BEGIN { paragraph = "" }
        /^[[:space:]]*#/ { next }
        /^[[:space:]]*$/ {
            if (paragraph != "") {
                print paragraph
                exit
            }
            next
        }
        {
            line = $0
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", line)
            if (line == "") {
                next
            }
            if (paragraph != "") {
                paragraph = paragraph " " line
            } else {
                paragraph = line
            }
        }
        END {
            if (paragraph != "") {
                print paragraph
            }
        }
    '
}

frontmatter_json_from_markdown() {
    local path="$1"
    ruby - "$path" <<'RUBY'
require "date"
require "yaml"
require "json"

path = ARGV.fetch(0)
text = File.read(path)
match = text.match(/\A---\s*\n(.*?)\n---\s*(?:\n|\z)/m)
if match.nil?
  puts "{}"
  exit 0
end

data = YAML.safe_load(match[1], permitted_classes: [Date, Time], aliases: false)
data = {} if data.nil?
puts JSON.generate(data)
RUBY
}

title_from_pattern() {
    local pattern="$1"
    local title="$pattern"
    if [[ "$title" == *". "* ]]; then
        title="${title%%. *}"
    fi
    if [ "${#title}" -gt 88 ]; then
        title="$(printf '%.88s' "$title")..."
    fi
    printf '%s\n' "$title"
}

yaml_line() {
    local json="$1"
    local key="$2"
    local expr="$3"
    local value

    value=$(printf '%s' "$json" | jq -ce "$expr" 2>/dev/null || true)
    if [ -n "$value" ] && [ "$value" != "null" ] && [ "$value" != "\"\"" ]; then
        printf '%s: %s\n' "$key" "$value"
    fi
}

emit_finding_artifact() {
    local artifact_json="$1"
    local path="$2"
    local applicable_when applicable_languages scope_tags body

    applicable_when=$(printf '%s' "$artifact_json" | jq -r '
        if (.applicable_when // []) | length > 0 then
            .applicable_when | join(", ")
        else
            "n/a"
        end
    ')
    applicable_languages=$(printf '%s' "$artifact_json" | jq -r '
        if (.applicable_languages // []) | length > 0 then
            .applicable_languages | join(", ")
        else
            "n/a"
        end
    ')
    scope_tags=$(printf '%s' "$artifact_json" | jq -r '
        if (.scope_tags // []) | length > 0 then
            .scope_tags | join(", ")
        else
            "n/a"
        end
    ')

    body=$(
        cat <<EOF
---
$(yaml_line "$artifact_json" "id" '.id')
$(yaml_line "$artifact_json" "type" '.type')
$(yaml_line "$artifact_json" "version" '.version')
$(yaml_line "$artifact_json" "date" '.date')
$(yaml_line "$artifact_json" "source_skill" '.source_skill')
$(yaml_line "$artifact_json" "source_artifact" '.source_artifact')
$(yaml_line "$artifact_json" "source_bead" '.source_bead')
$(yaml_line "$artifact_json" "source_phase" '.source_phase')
$(yaml_line "$artifact_json" "title" '.title')
$(yaml_line "$artifact_json" "summary" '.summary')
$(yaml_line "$artifact_json" "pattern" '.pattern')
$(yaml_line "$artifact_json" "detection_question" '.detection_question')
$(yaml_line "$artifact_json" "checklist_item" '.checklist_item')
$(yaml_line "$artifact_json" "severity" '.severity')
$(yaml_line "$artifact_json" "detectability" '.detectability')
$(yaml_line "$artifact_json" "status" '.status')
$(yaml_line "$artifact_json" "compiler_targets" '.compiler_targets')
$(yaml_line "$artifact_json" "scope_tags" '.scope_tags')
$(yaml_line "$artifact_json" "dedup_key" '.dedup_key')
$(yaml_line "$artifact_json" "applicable_when" '.applicable_when')
$(yaml_line "$artifact_json" "applicable_languages" '.applicable_languages')
$(yaml_line "$artifact_json" "tier" '.tier')
$(yaml_line "$artifact_json" "confidence" '.confidence')
$(yaml_line "$artifact_json" "ttl_days" '.ttl_days')
$(yaml_line "$artifact_json" "hit_count" '.hit_count')
$(yaml_line "$artifact_json" "last_cited" '.last_cited')
$(yaml_line "$artifact_json" "supersedes" '.supersedes')
$(yaml_line "$artifact_json" "superseded_by" '.superseded_by')
$(yaml_line "$artifact_json" "retired_by" '.retired_by')
$(yaml_line "$artifact_json" "utility" '.utility')
$(yaml_line "$artifact_json" "compiler" '.compiler')
---
# Finding: $(printf '%s' "$artifact_json" | jq -r '.title')

## Summary
$(printf '%s' "$artifact_json" | jq -r '.summary')

## Pattern
$(printf '%s' "$artifact_json" | jq -r '.pattern')

## Detection Question
$(printf '%s' "$artifact_json" | jq -r '.detection_question')

## Checklist Item
$(printf '%s' "$artifact_json" | jq -r '.checklist_item')

## Applicability
- Work shapes: $applicable_when
- Languages: $applicable_languages
- Scope tags: $scope_tags

## Lifecycle
- Status: $(printf '%s' "$artifact_json" | jq -r '.status')
- Detectability: $(printf '%s' "$artifact_json" | jq -r '.detectability')
- Confidence: $(printf '%s' "$artifact_json" | jq -r '.confidence // "n/a"')

## Source
- Skill: $(printf '%s' "$artifact_json" | jq -r '.source_skill')
- Artifact: $(printf '%s' "$artifact_json" | jq -r '.source_artifact')
EOF
    )

    printf '%s\n' "$body" | write_atomic "$path" 0644
}

emit_rule_artifact() {
    local artifact_json="$1"
    local path="$2"
    local kind="$3"
    local title heading rule_type source_artifact

    title=$(printf '%s' "$artifact_json" | jq -r '.title')
    if [ "$kind" = "plan" ]; then
        heading="Planning Rule"
        rule_type='"planning-rule"'
    else
        heading="Pre-Mortem Check"
        rule_type='"pre-mortem-check"'
    fi
    source_artifact=$(printf '.agents/findings/%s.md' "$(printf '%s' "$artifact_json" | jq -r '.id')" | jq -Rcs 'split("\n")[0]')

    cat <<EOF | write_atomic "$path" 0644
---
id: $(printf '%s' "$artifact_json" | jq -c '.id')
type: $rule_type
finding_id: $(printf '%s' "$artifact_json" | jq -c '.id')
source_artifact: $source_artifact
status: $(printf '%s' "$artifact_json" | jq -c '.status')
applicable_when: $(printf '%s' "$artifact_json" | jq -c '.applicable_when')
applicable_languages: $(printf '%s' "$artifact_json" | jq -c '.applicable_languages')
---
# ${heading}: ${title}

Prevent this known failure mode:

- Pattern: $(printf '%s' "$artifact_json" | jq -r '.pattern')
- Ask: $(printf '%s' "$artifact_json" | jq -r '.detection_question')
- Do: $(printf '%s' "$artifact_json" | jq -r '.checklist_item')
- Source: .agents/findings/$(printf '%s' "$artifact_json" | jq -r '.id').md
EOF
}

emit_constraint_review_file() {
    local artifact_json="$1"
    local path="$2"
    local detector applies_to

    detector=$(printf '%s' "$artifact_json" | jq -c '.compiler.detector')
    applies_to=$(printf '%s' "$artifact_json" | jq -c '.compiler.applies_to')

    cat <<EOF | write_atomic "$path" 0755
#!/usr/bin/env bash
# Review companion for compiled finding constraint.
# Runtime contract: .agents/constraints/index.json
# Finding ID: $(printf '%s' "$artifact_json" | jq -r '.id')
# Title: $(printf '%s' "$artifact_json" | jq -r '.title')
# Source artifact: .agents/findings/$(printf '%s' "$artifact_json" | jq -r '.id').md
# Status: $(printf '%s' "$artifact_json" | jq -r '.status')
# Detector: $detector
# Applies to: $applies_to
# Pattern: $(printf '%s' "$artifact_json" | jq -r '.pattern')
# Detection question: $(printf '%s' "$artifact_json" | jq -r '.detection_question')
# Checklist item: $(printf '%s' "$artifact_json" | jq -r '.checklist_item')
exit 0
EOF
}

load_constraint_index() {
    if [ -f "$CONSTRAINT_INDEX" ]; then
        jq -c '.' "$CONSTRAINT_INDEX" 2>/dev/null || {
            warn "constraint index was malformed; rebuilding"
            printf '{"schema_version":1,"constraints":[]}\n'
        }
    else
        printf '{"schema_version":1,"constraints":[]}\n'
    fi
}

upsert_constraint_entry() {
    local entry_json="$1"
    local updated

    updated=$(
        load_constraint_index | jq -c --argjson entry "$entry_json" '
            .schema_version = 1
            | .constraints = (((.constraints // []) | map(select(.id != $entry.id))) + [$entry] | sort_by(.id))
        '
    )
    printf '%s\n' "$updated" | write_atomic "$CONSTRAINT_INDEX" 0600
}

remove_constraint_entry() {
    local finding_id="$1"
    local updated

    updated=$(
        load_constraint_index | jq -c --arg id "$finding_id" '
            .schema_version = 1
            | .constraints = ((.constraints // []) | map(select(.id != $id)))
        '
    )
    printf '%s\n' "$updated" | write_atomic "$CONSTRAINT_INDEX" 0600
}

normalize_finding_artifact_json() {
    local input_json="$1"
    local source_path="$2"
    local fallback_id="$3"
    local fallback_title="$4"
    local fallback_summary="$5"
    local fallback_pattern="$6"
    local fallback_question="$7"
    local fallback_checklist="$8"
    local fallback_dedup_key="$9"
    local review_file=".agents/constraints/${fallback_id}.sh"

    printf '%s' "$input_json" | jq -ce \
        --arg source_artifact "$(relative_to_root "$source_path")" \
        --arg id "$fallback_id" \
        --arg date "$(date -u +%Y-%m-%d)" \
        --arg title "$fallback_title" \
        --arg summary "$fallback_summary" \
        --arg pattern "$fallback_pattern" \
        --arg detection_question "$fallback_question" \
        --arg checklist_item "$fallback_checklist" \
        --arg dedup_key "$fallback_dedup_key" \
        --arg review_file "$review_file" '
        . as $root
        | {
            id: ($root.id // $id),
            type: "finding",
            version: ($root.version // 1),
            date: ($root.date // $date),
            source_skill: ($root.source_skill // "finding-compiler"),
            source_artifact: ($root.source_artifact // $source_artifact),
            source_bead: ($root.source_bead // ""),
            source_phase: ($root.source_phase // ""),
            title: ($root.title // $title),
            summary: ($root.summary // $summary),
            pattern: ($root.pattern // $pattern),
            detection_question: ($root.detection_question // $detection_question),
            checklist_item: ($root.checklist_item // $checklist_item),
            severity: ($root.severity // "significant"),
            detectability: ($root.detectability // (if (($root.compiler_targets // []) | index("constraint")) then "mechanical" else "advisory" end)),
            status: ($root.status // "draft"),
            compiler_targets: (
                if ($root.compiler_targets | type) == "array" and ($root.compiler_targets | length) > 0 then
                    $root.compiler_targets
                elif ($root.detectability // "") == "mechanical" then
                    ["plan", "pre-mortem", "constraint"]
                else
                    ["plan", "pre-mortem"]
                end
            ),
            scope_tags: (
                if ($root.scope_tags | type) == "array" and ($root.scope_tags | length) > 0 then
                    $root.scope_tags
                elif ($root.applicable_when | type) == "array" and ($root.applicable_when | length) > 0 then
                    $root.applicable_when
                else
                    ["validation-gap"]
                end
            ),
            dedup_key: ($root.dedup_key // $dedup_key),
            applicable_when: (
                if ($root.applicable_when | type) == "array" and ($root.applicable_when | length) > 0 then
                    $root.applicable_when
                else
                    ["validation-gap"]
                end
            ),
            applicable_languages: (
                if ($root.applicable_languages | type) == "array" then
                    $root.applicable_languages
                else
                    []
                end
            ),
            tier: ($root.tier // ""),
            confidence: ($root.confidence // "medium"),
            ttl_days: ($root.ttl_days // 30),
            hit_count: ($root.hit_count // 0),
            last_cited: ($root.last_cited // null),
            supersedes: ($root.supersedes // ""),
            superseded_by: ($root.superseded_by // null),
            retired_by: ($root.retired_by // ""),
            utility: ($root.utility // null),
            compiler: ($root.compiler // null)
        }
        | if .detectability == "mechanical" and .compiler != null then
            .compiler = (.compiler + {review_file: ((.compiler.review_file // null) // $review_file)})
          else
            .
          end
        | with_entries(
            select(
                (.value != "")
                and (.value != null or (.key == "last_cited" or .key == "superseded_by" or .key == "utility"))
            )
        )
    '
}

artifact_json_from_registry_entry() {
    local registry_json="$1"
    local id pattern title primary_when dedup_key

    id=$(printf '%s' "$registry_json" | jq -r '.id')
    pattern=$(printf '%s' "$registry_json" | jq -r '.pattern')
    title=$(title_from_pattern "$pattern")
    primary_when=$(printf '%s' "$registry_json" | jq -r '(.applicable_when // ["validation-gap"])[0]')
    dedup_key=$(printf '%s' "$registry_json" | jq -r '.dedup_key // empty')
    if [ -z "$dedup_key" ]; then
        dedup_key=$(derive_dedup_key "$(printf '%s' "$registry_json" | jq -r '.category // "finding"')" "$pattern" "$primary_when")
    fi

    normalize_finding_artifact_json \
        "$(printf '%s' "$registry_json" | jq -c '{
            id,
            version,
            date,
            source_skill: .source.skill,
            source_artifact: .source.file,
            title: .pattern,
            summary: .pattern,
            pattern,
            detection_question,
            checklist_item,
            severity,
            detectability: "advisory",
            status,
            compiler_targets: ["plan", "pre-mortem"],
            scope_tags: (([.category] + (.applicable_when // [])) | map(select(type == "string" and length > 0)) | unique),
            dedup_key,
            applicable_when,
            applicable_languages,
            tier,
            confidence,
            ttl_days,
            hit_count,
            last_cited,
            superseded_by,
            utility
        }')" \
        "$ROOT/.agents/findings/${id}.md" \
        "$id" \
        "$title" \
        "$pattern" \
        "$pattern" \
        "$(printf 'Did the current plan or review account for this failure mode: %s?' "$title")" \
        "$(printf 'Use this finding as an explicit prevention check: %s' "$pattern")" \
        "$dedup_key"
}

artifact_json_from_finding_file() {
    local path="$1"
    local fm_json body summary id title pattern primary_when dedup_key

    fm_json=$(frontmatter_json_from_markdown "$path")
    body=$(extract_markdown_body "$path")
    summary=$(printf '%s\n' "$body" | first_paragraph)
    id=$(printf '%s' "$fm_json" | jq -r '.id // empty')
    if [ -z "$id" ]; then
        id=$(basename "$path" .md)
    fi
    title=$(printf '%s' "$fm_json" | jq -r '.title // empty')
    if [ -z "$title" ]; then
        title=$(title_from_pattern "$(printf '%s' "$fm_json" | jq -r '.pattern // .summary // empty')")
    fi
    if [ -z "$title" ] || [ "$title" = "null" ]; then
        title="$id"
    fi
    pattern=$(printf '%s' "$fm_json" | jq -r '.pattern // .summary // empty')
    if [ -z "$pattern" ] || [ "$pattern" = "null" ]; then
        pattern="$title"
    fi
    primary_when=$(printf '%s' "$fm_json" | jq -r '(.applicable_when // ["validation-gap"])[0]')
    dedup_key=$(printf '%s' "$fm_json" | jq -r '.dedup_key // empty')
    if [ -z "$dedup_key" ]; then
        dedup_key=$(derive_dedup_key "finding" "$pattern" "$primary_when")
    fi

    normalize_finding_artifact_json \
        "$fm_json" \
        "$path" \
        "$id" \
        "$title" \
        "${summary:-$pattern}" \
        "$pattern" \
        "$(printf 'Did the current work account for this finding: %s?' "$title")" \
        "$(printf 'Apply the prevention checklist for finding %s.' "$id")" \
        "$dedup_key"
}

artifact_json_from_legacy_learning() {
    local path="$1"
    local fm_json body summary id title pattern primary_when dedup_key category compiler_json

    fm_json=$(frontmatter_json_from_markdown "$path")
    body=$(extract_markdown_body "$path")
    summary=$(printf '%s\n' "$body" | first_paragraph)

    if ! printf '%s' "$fm_json" | jq -e '
        (.tags // []) as $tags
        | if ($tags | type) == "array" then
            any($tags[]; tostring == "constraint" or tostring == "anti-pattern")
          elif ($tags | type) == "string" then
            (($tags | ascii_downcase) | test("constraint|anti-pattern"))
          else
            false
          end
    ' >/dev/null 2>&1; then
        warn "skipping non-constraint learning: $(relative_to_root "$path")"
        return 1
    fi

    id=$(printf '%s' "$fm_json" | jq -r '.id // empty')
    if [ -z "$id" ]; then
        id=$(basename "$path" .md)
    fi
    title=$(printf '%s' "$fm_json" | jq -r '.title // empty')
    if [ -z "$title" ]; then
        title="$id"
    fi
    pattern=$(printf '%s' "$fm_json" | jq -r '.pattern // empty')
    if [ -z "$pattern" ]; then
        pattern="${summary:-$title}"
    fi
    primary_when=$(printf '%s' "$fm_json" | jq -r '(.applicable_when // ["validation-gap"])[0]')
    category=$(printf '%s' "$fm_json" | jq -r '
        if (.category // "") != "" then
            .category
        elif ((.tags // []) | type) == "array" and (((.tags // []) | index("anti-pattern")) != null) then
            "anti-pattern"
        else
            "legacy-constraint"
        end
    ')
    dedup_key=$(printf '%s' "$fm_json" | jq -r '.dedup_key // empty')
    if [ -z "$dedup_key" ]; then
        dedup_key=$(derive_dedup_key "$category" "$pattern" "$primary_when")
    fi

    compiler_json=$(
        printf '%s' "$fm_json" | jq -c --arg review_file ".agents/constraints/${id}.sh" '
            {
                review_file: $review_file,
                applies_to: {
                    scope: "repo",
                    issue_types: ["feature", "bug", "task"]
                },
                detector: {
                    kind: "content_pattern",
                    mode: "must_not_contain",
                    pattern: "__manual_review_required__",
                    message: "Replace placeholder detector before activating this constraint."
                }
            } as $defaults
            | ($defaults + (.compiler // {}))
            | .applies_to = ($defaults.applies_to + ((.compiler // {}).applies_to // {}))
            | .detector = ($defaults.detector + ((.compiler // {}).detector // {}))
        '
    )

    normalize_finding_artifact_json \
        "$(printf '%s' "$fm_json" | jq -c --arg summary "${summary:-$pattern}" \
            --arg pattern "$pattern" \
            --arg dedup_key "$dedup_key" \
            --argjson compiler "$compiler_json" '
            . + {
                type: "finding",
                summary: (.summary // $summary),
                pattern: (.pattern // $pattern),
                detectability: "mechanical",
                status: (.status // "draft"),
                compiler_targets: (
                    if (.compiler_targets | type) == "array" and (.compiler_targets | length) > 0 then
                        .compiler_targets
                    else
                        ["plan", "pre-mortem", "constraint"]
                    end
                ),
                dedup_key: (.dedup_key // $dedup_key),
                compiler: $compiler
            }')" \
        "$path" \
        "$id" \
        "$title" \
        "${summary:-$pattern}" \
        "$pattern" \
        "$(printf 'How would this change repeat the legacy constraint learning: %s?' "$title")" \
        "$(printf 'Review the generated rule and replace placeholder detection before activation for %s.' "$id")" \
        "$dedup_key"
}

compile_outputs_from_artifact_json() {
    local artifact_json="$1"
    local id status artifact_path plan_path premortem_path review_path
    local compiled_at has_plan_target has_premortem_target has_constraint_target

    id=$(printf '%s' "$artifact_json" | jq -r '.id')
    status=$(printf '%s' "$artifact_json" | jq -r '.status // "draft"')
    artifact_path="$FINDINGS_DIR/${id}.md"
    plan_path="$PLANNING_RULES_DIR/${id}.md"
    premortem_path="$PREMORTEM_CHECKS_DIR/${id}.md"
    review_path="$CONSTRAINT_DIR/${id}.sh"
    compiled_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

    has_plan_target=$(printf '%s' "$artifact_json" | jq -e '(((.compiler_targets // []) | index("plan")) != null)' >/dev/null 2>&1 && printf '1' || printf '0')
    has_premortem_target=$(printf '%s' "$artifact_json" | jq -e '(((.compiler_targets // []) | index("pre-mortem")) != null)' >/dev/null 2>&1 && printf '1' || printf '0')
    has_constraint_target=$(printf '%s' "$artifact_json" | jq -e '
        (((.compiler_targets // []) | index("constraint")) != null)
        and ((.detectability // "") == "mechanical")
        and ((.compiler | type) == "object")
        and ((.compiler.applies_to | type) == "object")
        and ((.compiler.detector | type) == "object")
        and ((.compiler.detector.kind | type) == "string")
    ' >/dev/null 2>&1 && printf '1' || printf '0')

    if [ "$status" = "superseded" ]; then
        remove_file_if_exists "$plan_path"
        remove_file_if_exists "$premortem_path"
        remove_file_if_exists "$review_path"
        remove_constraint_entry "$id"
        note "Retired compiled outputs for superseded finding: $id"
        return 0
    fi

    if [ "$status" != "retired" ] && [ "$has_plan_target" = "1" ]; then
        emit_rule_artifact "$artifact_json" "$plan_path" "plan"
    else
        remove_file_if_exists "$plan_path"
    fi

    if [ "$status" != "retired" ] && [ "$has_premortem_target" = "1" ]; then
        emit_rule_artifact "$artifact_json" "$premortem_path" "pre-mortem"
    else
        remove_file_if_exists "$premortem_path"
    fi

    if [ "$has_constraint_target" = "1" ]; then
        local constraint_entry
        emit_constraint_review_file "$artifact_json" "$review_path"
        constraint_entry=$(printf '%s' "$artifact_json" | jq -c \
            --arg source_artifact ".agents/findings/${id}.md" \
            --arg review_file ".agents/constraints/${id}.sh" \
            --arg compiled_at "$compiled_at" '
            {
                id: .id,
                finding_id: .id,
                title: .title,
                status: (
                    if .status == "active" or .status == "retired" then
                        .status
                    else
                        "draft"
                    end
                ),
                source_artifact: $source_artifact,
                review_file: (.compiler.review_file // $review_file),
                compiled_at: $compiled_at,
                applies_to: .compiler.applies_to,
                detector: .compiler.detector,
                source: $source_artifact,
                source_type: "finding",
                compiler_targets: .compiler_targets,
                detectability: .detectability,
                file: (.compiler.review_file // $review_file)
            }
        ')
        upsert_constraint_entry "$constraint_entry"
    else
        if printf '%s' "$artifact_json" | jq -e '(((.compiler_targets // []) | index("constraint")) != null)' >/dev/null 2>&1; then
            warn "skipping constraint target for ${id}: missing compiler metadata"
        fi
        remove_file_if_exists "$review_path"
        remove_constraint_entry "$id"
    fi
}

promote_registry_file() {
    local registry_path="$1"
    local line artifact_json artifact_path

    [ -f "$registry_path" ] || return 0

    while IFS= read -r line || [ -n "$line" ]; do
        [ -n "$line" ] || continue
        artifact_json=$(printf '%s' "$line" | jq -ce '.' 2>/dev/null || true)
        if [ -z "$artifact_json" ]; then
            warn "ignoring malformed registry line in $(relative_to_root "$registry_path")"
            continue
        fi
        artifact_json=$(artifact_json_from_registry_entry "$artifact_json")
        artifact_path="$FINDINGS_DIR/$(printf '%s' "$artifact_json" | jq -r '.id').md"
        emit_finding_artifact "$artifact_json" "$artifact_path"
        note "Promoted finding artifact: $(relative_to_root "$artifact_path")"
    done < "$registry_path"
}

compile_existing_finding_artifacts() {
    local artifact_path fm_json type artifact_json

    [ -d "$FINDINGS_DIR" ] || return 0

    for artifact_path in "$FINDINGS_DIR"/*.md; do
        [ -e "$artifact_path" ] || continue
        fm_json=$(frontmatter_json_from_markdown "$artifact_path")
        type=$(printf '%s' "$fm_json" | jq -r '.type // empty')
        if [ "$type" != "finding" ]; then
            continue
        fi
        artifact_json=$(artifact_json_from_finding_file "$artifact_path")
        compile_outputs_from_artifact_json "$artifact_json"
    done
}

compile_input_path() {
    local input_path="$1"
    local artifact_json artifact_path fm_json type

    if [ ! -f "$input_path" ]; then
        warn "input not found: $input_path"
        return 1
    fi

    case "$input_path" in
        *.jsonl)
            promote_registry_file "$input_path"
            compile_existing_finding_artifacts
            ;;
        *.md)
            fm_json=$(frontmatter_json_from_markdown "$input_path")
            type=$(printf '%s' "$fm_json" | jq -r '.type // empty')
            if [ "$type" = "finding" ]; then
                artifact_json=$(artifact_json_from_finding_file "$input_path")
                compile_outputs_from_artifact_json "$artifact_json"
            else
                artifact_json=$(artifact_json_from_legacy_learning "$input_path") || return 0
                artifact_path="$FINDINGS_DIR/$(printf '%s' "$artifact_json" | jq -r '.id').md"
                emit_finding_artifact "$artifact_json" "$artifact_path"
                compile_outputs_from_artifact_json "$artifact_json"
                note "Promoted legacy learning: $(relative_to_root "$artifact_path")"
            fi
            ;;
        *)
            warn "unsupported finding compiler input: $(relative_to_root "$input_path")"
            ;;
    esac
}

with_constraint_lock() {
    mkdir -p "$FINDINGS_DIR" "$PLANNING_RULES_DIR" "$PREMORTEM_CHECKS_DIR" "$CONSTRAINT_DIR"

    if command -v flock >/dev/null 2>&1; then
        exec 9>"$CONSTRAINT_LOCK"
        flock -n 9 || exit 0
        "$@"
        return 0
    fi

    local lock_dir="${CONSTRAINT_LOCK}.d"
    if mkdir "$lock_dir" 2>/dev/null; then
        # shellcheck disable=SC2064 # intentional early expansion — lock_dir is local
        trap "rmdir '$lock_dir' 2>/dev/null || true" EXIT
        "$@"
    fi
}

compile_all() {
    promote_registry_file "$FINDINGS_DIR/registry.jsonl"
    compile_existing_finding_artifacts
}

compile_inputs() {
    local input_path
    for input_path in "${INPUTS[@]}"; do
        compile_input_path "$input_path"
    done
}

main() {
    require_tooling

    if [ "${#INPUTS[@]}" -eq 0 ]; then
        with_constraint_lock compile_all
        exit 0
    fi

    with_constraint_lock compile_inputs
}

main "$@"
