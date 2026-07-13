#!/usr/bin/env bash
# check-architecture-doc-drift.sh — mechanical acceptance for stale architecture surfaces.
#
# Fails when reconciled docs regress to stale architecture wording, when the
# first-read inventory differs from reproducible repository measurements, or
# when a tagged-only command is presented as part of the default CLI spine.
#
# Exit codes: 0 = clean, 1 = drift, 2 = usage error.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

ports_doc="$repo_root/docs/architecture/ports-and-adapters.md"
bc_yaml="$repo_root/docs/contracts/bounded-contexts.yaml"
overview_doc="${ARCHITECTURE_OVERVIEW_DOC:-$repo_root/docs/architecture/codebase-overview.md}"

failures=0

fail() {
    echo "ARCHITECTURE_DOC_DRIFT: FAIL: $*" >&2
    failures=$((failures + 1))
}

require_overview_line() {
    local expected="$1"
    if ! grep -Fqx "$expected" "$overview_doc"; then
        fail "$overview_doc must contain measured fact: $expected"
    fi
}

command_profile() {
    local tags="$1"
    local args=()
    if [[ -n "$tags" ]]; then
        args=(-tags "$tags")
    fi
    (
        cd "$repo_root/cli"
        go run "${args[@]}" ./cmd/ao --help
    ) | awk '/^  [a-z][a-z0-9-]+[[:space:]]/ { print $1 }' | sort -u
}

if rg -n 'anticipating `bd`' "$ports_doc" >/dev/null 2>&1; then
    fail "$ports_doc still contains anticipating \`bd\` wording"
fi

if rg -n '^[[:space:]]*- hooks[[:space:]]*$' "$bc_yaml" >/dev/null 2>&1; then
    fail "$bc_yaml BC5 center_of_gravity must not list bare hooks entry"
fi

if ! bash "$repo_root/scripts/check-bounded-contexts-drift.sh" --check >/dev/null; then
    fail "bounded-contexts drift gate failed (run scripts/check-bounded-contexts-drift.sh --check)"
fi

go_files="$(git -C "$repo_root" ls-files | awk '/\.go$/ { count++ } END { print count+0 }')"
skill_count="$(git -C "$repo_root" ls-files skills | awk -F/ 'NF == 3 && $3 == "SKILL.md" && $2 != "pre-mortem" && $2 != "post-mortem" && $2 != "pre_mortem" && $2 != "post_mortem" { count++ } END { print count+0 }')"
codex_skill_count="$(git -C "$repo_root" ls-files skills-codex | awk -F/ 'NF == 3 && $3 == "SKILL.md" && $2 != "pre-mortem" && $2 != "post-mortem" && $2 != "pre_mortem" && $2 != "post_mortem" { count++ } END { print count+0 }')"
shell_scripts="$(git -C "$repo_root" ls-files scripts | awk '/\.sh$/ { count++ } END { print count+0 }')"
bats_files="$(git -C "$repo_root" ls-files tests | awk '/\.bats$/ { count++ } END { print count+0 }')"
workflow_count="$(git -C "$repo_root" ls-files .claude/workflows | awk '/\.js$/ { count++ } END { print count+0 }')"
gate_checks="$(rg -c 'ID:[[:space:]]*"' "$repo_root/cli/internal/gates/checks/seed.go")"
capability_count="$(jq '.capabilities | length' "$repo_root/registry.json")"

if ! default_commands="$(command_profile '')"; then
    fail "default ao command profile did not compile"
    default_commands=""
fi
if ! combined_commands="$(command_profile 'flywheel legacy')"; then
    fail "combined ao command profile did not compile"
    combined_commands=""
fi
default_command_count="$(printf '%s\n' "$default_commands" | sed '/^$/d' | wc -l | tr -d ' ')"
combined_command_count="$(printf '%s\n' "$combined_commands" | sed '/^$/d' | wc -l | tr -d ' ')"

require_overview_line "| Go source files | $go_files (\`git ls-files '*.go'\`) |"
require_overview_line "| Active skills | $skill_count (\`git ls-files skills | awk -F/ 'NF == 3 && \$3 == \"SKILL.md\"'\`) |"
require_overview_line "| Codex skill twins | $codex_skill_count (\`git ls-files skills-codex | awk -F/ 'NF == 3 && \$3 == \"SKILL.md\"'\`) |"
require_overview_line "| CLI top-level commands | $default_command_count default / $combined_command_count with \`flywheel legacy\` (\`go run [-tags profile] ./cmd/ao --help\`) |"
require_overview_line "| Gate checks | $gate_checks (\`rg -c 'ID:' cli/internal/gates/checks/seed.go\`) |"
require_overview_line "| Shell scripts | $shell_scripts (\`git ls-files scripts | awk '/\\.sh\$/'\`) |"
require_overview_line "| Bats test files | $bats_files (\`git ls-files tests | awk '/\\.bats\$/'\`) |"
require_overview_line "| Claude workflows | $workflow_count (\`git ls-files .claude/workflows | awk '/\\.js\$/'\`) |"
require_overview_line "| Registry capabilities | $capability_count (\`jq '.capabilities | length' registry.json\`) |"

primary_commands="$(sed -n '/^### Primary CLI commands (active)$/,/^Full surface:/p' "$overview_doc" \
    | sed -nE 's/^\| `ao ([a-z][a-z0-9-]+).*$/\1/p')"
while IFS= read -r command_name; do
    [[ -z "$command_name" ]] && continue
    if ! printf '%s\n' "$default_commands" | grep -Fxq "$command_name"; then
        fail "$overview_doc presents tagged-only ao $command_name as a default primary command"
    fi
done <<<"$primary_commands"

require_overview_line "| \`ao codex *\` | \`legacy\`-tagged archive; absent from the default spine |"

if [[ "$failures" -gt 0 ]]; then
    exit 1
fi

echo "ARCHITECTURE_DOC_DRIFT: PASS (architecture facts match repository and build profiles)"
exit 0
