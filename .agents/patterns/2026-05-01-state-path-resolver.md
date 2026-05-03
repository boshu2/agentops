---
type: pattern
date: 2026-05-01
status: active
related_findings:
  - f-2026-05-01-022
related_issues:
  - soc-irg1.1
  - soc-irg1.5
maturity: established
last_reward_at: 2026-05-03T09:20:16-04:00
confidence: 0.1667
last_decay_at: 2026-05-03T09:20:16-04:00
utility: 0.5000
last_reward: 0.50
reward_count: 1
---

# Pattern: State-Path Resolver

## When to Apply

Any new hook script, ao subcommand, or repo helper that reads or writes
under `.agents/` MUST source the canonical state-path resolver instead of
hardcoding `".agents/<sub>"` strings.

The two surfaces are deliberately symmetric so a shell hook and a Go
subcommand running under the same env produce identical paths:

| Surface | Resolver |
|---------|----------|
| Bash hooks, scripts, lib helpers | `lib/ao-paths.sh` (sourceable) |
| Go (cli/cmd, cli/internal) | `cli/internal/paths` (`paths.Resolve()`) |

## Why It Matters

Hardcoded `".agents/<sub>"` strings ignore the documented env precedence
(AO_HOME > CLAUDE_PLUGIN_DATA > repo-root default) and the per-subdir
overrides (AO_AGENTS_DIR, AO_KNOWLEDGE_ROOT, AO_HOOKS_DIR, AO_SCOPE_LOCK,
AO_RPI_DIR, AO_FINDINGS_DIR, AO_PLANS_DIR, AO_COUNCIL_DIR,
AO_LEARNINGS_DIR, AO_PATTERNS_DIR, AO_DECISIONS_DIR). Operators running
agentops with a non-default install (Claude plugin data dir, parallel
sandbox, multi-tenant CI) get silently broken paths.

Cited finding: `f-2026-05-01-022` — gstack absorption research surfaced
~150 occurrences of hardcoded `.agents/` literals across executable code.
The warn-only fitness gate `state-path-resolver-coverage` (GOALS.md)
tracks the long-tail migration.

## How to Apply (Bash)

```bash
# Top of any new hook (after `set -euo pipefail` and the kill-switch check):
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
if [ -z "${AO_AGENTS_DIR:-}" ] && [ -f "$ROOT/lib/ao-paths.sh" ]; then
    eval "$(bash "$ROOT/lib/ao-paths.sh" 2>/dev/null)" 2>/dev/null || true
fi
AO_DIR="${AO_AGENTS_DIR:-$ROOT/.agents}"

# Then construct paths from AO_DIR:
mkdir -p "$AO_DIR/ao" 2>/dev/null
echo "$payload" >> "$AO_DIR/ao/citations.jsonl"
```

The fail-open `:-` fallback chain is load-bearing: if `lib/ao-paths.sh` is
missing (detached install, future refactor, broken sandbox), the hook
still works against the legacy `${ROOT}/.agents` layout. Hooks are
advisory; never fail closed on resolver absence.

`lib/hook-helpers.sh` already sources the resolver — any hook that
sources `hook-helpers.sh` inherits `AO_AGENTS_DIR` and friends for free.

## How to Apply (Go)

```go
import (
    "github.com/boshu2/agentops/cli/internal/paths"
)

p := paths.Resolve()                 // honors env, defaults to ${cwd}/.agents
// or, when you need the git repo root specifically:
p := paths.ResolveFromRepo()

learningsPath := filepath.Join(p.AgentsDir, "learnings", id+".md")
scopeLock     := p.ScopeLock         // already a fully-resolved file path
```

For ao subcommands that need the agents-dir resolved relative to a
specific base (e.g. `cwd` vs `$HOME` for `--global`), prefer a small
`agentsDirIn(base string) string` shim in your package that consults
`AO_AGENTS_DIR` / `AO_HOME` first, then falls back to `filepath.Join(base,
".agents")`. See `cli/cmd/ao/maturity.go:agentsDirIn` for the canonical
example landed in soc-irg1.5.

## Anti-Pattern

```bash
# DON'T — hardcoded path string concatenation
echo "$line" >> "${ROOT}/.agents/ao/citations.jsonl"

# DON'T — Go side
profilePath := filepath.Join(cwd, ".agents", "profile.md")
```

These break under any env override. They are flagged by
`scripts/check-paths-resolver-coverage.sh` as `state-path-resolver-coverage`
warn signal.

## Retrieval Cues

State path resolver, AO_AGENTS_DIR, AO_HOME, lib/ao-paths.sh,
cli/internal/paths, hardcoded .agents path, .agents string concatenation,
warn-then-fail ratchet, soc-irg1, soc-irg1.5, hook helpers state path,
scope.lock resolver, knowledge separation, agentops-paths-coverage,
warn-only ratchet, paths.Resolve, paths.ResolveFromRepo.
