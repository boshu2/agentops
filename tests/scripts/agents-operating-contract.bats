#!/usr/bin/env bats

@test "root contract routes the complete loop without duplicating operations" {
  local repo_root contract
  local -a failures=()
  repo_root="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  contract="$repo_root/AGENTS.md"

  if [[ ! -f "$contract" ]]; then
    echo "invalid harness: AGENTS.md is missing" >&2
    return 2
  fi

  local required
  for required in \
    "## Ordered operating loop" \
    "**Shape acceptance.**" \
    "**Pull one leaf and isolate it.**" \
    "**Build.**" \
    "**Freeze and prove.**" \
    "**Learn, deliver, verify, report.**" \
    "Discovery, Crank, Validate, and Learn are the four lifecycle umbrellas." \
    "Delivery is repository-owned" \
    "NOTE, REPAIR, REPLAN, HOLD, or ANDON" \
    "skills/rpi/references/pull-flow-governor.md" \
    "docs/contracts/repo-execution-profile.md" \
    "docs/architecture/operating-loop.md" \
    "docs/agent-workflow-reference.md" \
    "AGENTS-WORKFLOW.md" \
    "AGENTS-CI.md" \
    "AGENTS-CODEX.md" \
    "AGENTS-RUNTIME.md"; do
    if ! grep -Fq -- "$required" "$contract"; then
      failures+=("missing complete-loop route: $required")
    fi
  done

  local duplicated_heading
  for duplicated_heading in \
    "## Installing / updating skills" \
    "## Quick reference" \
    "## Project structure" \
    "## Registries and curated routers" \
    "## Footguns (read before editing)"; do
    if grep -Fqx -- "$duplicated_heading" "$contract"; then
      failures+=("duplicated operational heading: $duplicated_heading")
    fi
  done

  if (( ${#failures[@]} > 0 )); then
    printf '%s\n' "${failures[@]}" >&2
    return 1
  fi
}
