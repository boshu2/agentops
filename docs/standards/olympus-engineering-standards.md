---
date: 2026-04-29
source: ported from ~/dev/personal/olympus/docs/standards/ENGINEERING-STANDARDS.md
status: reference
---

# Olympus Engineering Standards (Reference)

> **Status:** Reference port from olympus v5.0.1. Where this doc disagrees with agentops's canonical `.claude/rules/{go,python}.md` or `skills/standards/references/{go,python}.md`, **the canonical agentops rules win**. This doc is preserved for cross-reference only.

## What carried (added value vs existing agentops rules)

These rules existed in olympus's engineering standards and are NOT covered (or only weakly covered) by the canonical agentops standards. Treat them as advisory addenda for agentopsd-class daemon work.

### CLI composition over library imports (architectural)

> Do not import external CLI tools (`bd`, `ao`) as Go libraries. Use CLI composition.
> Keep daemon mechanical. No direct model-provider dependencies in daemon paths.

Forces a clean boundary between the daemon and external tools. The agentops Go rules talk about interfaces and struct contracts but do not state this layering rule. Useful for `agentopsd` extraction.

### Decision Discipline (process)

Verbatim from olympus:

1. Favor falsifiable claims over agreement language. Recommendations must cite concrete repo evidence, tests, or measurable constraints.
2. Push back on proposals that skip problem definition, invariants, or failure modes.
3. Do not move to implementation until the following are explicit: problem statement, success criteria, non-goals, and rollback path.
4. For architecture changes, provide at least two alternatives and justify the selected option with tradeoffs (correctness, operability, and cost).
5. If uncertainty is material, state it directly and define the experiment or data needed before committing to a build path.

Agentops's `.claude/rules/*` cover code-level conventions but not this pre-implementation decision gate. Closest equivalent is `/pre-mortem` and `/brainstorm` skills, which are interactive flows rather than written standards.

### Logging and observability hygiene

> 1. Log actionable context: operation, IDs, error reason.
> 2. Do not log secrets or tokens.
> 3. Keep log formats stable for scripts that parse outputs.

Agentops `skills/standards/references/go.md` covers HTTP-handler security (XSS, path traversal) but does not state these three log-hygiene rules. Worth pulling in for any daemon emitting structured logs that downstream tooling parses.

### Determinism for state transitions and event emission

> Keep deterministic behavior for state transitions and event emission.

Implicit in agentops's "exact assertion rule" but not stated as a positive design constraint for emitters. Relevant for `agentopsd` event surfaces.

### Graceful shutdown closes goroutines and releases resources

> Ensure graceful shutdown paths close goroutines and release resources.

Agentops `references/go.md` covers concurrency primitives (channels, atomics, `context.AfterFunc`) but does not state the graceful-shutdown invariant explicitly.

## What was already covered (no action)

All olympus rules in this category are equally or more thoroughly covered by the canonical agentops standards. Use the agentops doc — do not re-port.

| Olympus rule | Canonical agentops source |
|---|---|
| `gofmt` clean, `go vet ./...` passes | `.claude/rules/go.md` ("Before Committing Go Changes"), `skills/standards/references/go.md` ("Required") |
| Always check returned errors; wrap with `fmt.Errorf("%w", err)`; no `panic` outside `main`/tests; never drop errors silently | `.claude/rules/go.md` ("Error Handling"), `skills/standards/references/go.md` ("Error Handling") — agentops also adds `errors.Is`, `errors.Join`, `context.WithCancelCause` |
| Accept interfaces, return concrete structs; small interfaces; define at call sites | `.claude/rules/go.md` ("Style"), `skills/standards/references/go.md` ("Interfaces") |
| Pass `context.Context` as first parameter | `skills/standards/references/go.md` ("Concurrency") |
| Use channels for ownership transfer; `sync` primitives for shared state | `skills/standards/references/go.md` ("Concurrency") — agentops additionally specifies `atomic.Bool` / `atomic.Int64` over older typed atomics |
| Code-path changes require unit/integration tests; assert exact values; deterministic over flaky | `.claude/rules/go.md` ("Testing"), `skills/standards/references/go.md` ("Exact Assertion Rule", "Structural Invariant Tests"), `references/test-pyramid.md` (L2-first AI-native shape) |
| Coverage with behavior tests, not happy-path padding | `.claude/rules/go.md` ("No coverage-padding tests"), `skills/standards/references/go.md` ("Test Conventions") |
| `#!/usr/bin/env bash` + `set -euo pipefail`; quote variables; `command -v`; cleanup traps; non-interactive in CI | `skills/standards/references/shell.md` (full coverage), agentops `~/CLAUDE.md` ("Non-interactive shell defaults") |
| One H1 per document; consistent heading hierarchy; runnable command blocks | `skills/standards/references/markdown.md` |
| Start from synced `main`; do not finish session until commits pushed | agentops `~/CLAUDE.md` ("Task tracking protocol"), `/push` skill |

## What did NOT carry (rejected with reason)

| Olympus rule | Why rejected |
|---|---|
| `internal/` for implementation, `cmd/` for entrypoints | Already idiomatic Go layout; agentops follows it (`cli/cmd/ao`, `cli/internal/`) without needing it written down. Restating would be noise. |
| Source-of-truth order naming `docs/specs/index.md`, `SPEC-CONTRACT.md`, etc. | Olympus-specific doc tree. Agentops has its own precedence ladder in `CLAUDE.md` ("Source-of-Truth Precedence") — that one wins. |
| `make test`, `make build`, `make testing-check`, `make daemon-smoke`, `make serve-smoke`, `make throughput` as gate names | Olympus Makefile target names. Agentops's gate is `scripts/pre-push-gate.sh` + `cd cli && make build && make test` — different surface. |
| Coverage ratchet via `scripts/check-coverage-floors.sh` | Olympus-specific script path. Agentops handles coverage through `/vibe`, `/complexity`, and CI checks rather than a ratcheted-floor script. |
| Suite definitions in `testing/suites/*.md`, scenarios in `testing/scenarios/catalog.md` | Olympus-specific paths and catalog. Agentops uses `tests/` + `/scenario` skill (holdout scenarios in `.agents/holdout/`) — different model. |
| Long-tail merge checklist (6 items: spec match, tests, daemon/serve safety, throughput, docs/runbooks, goals/traceability) | Heavily olympus-coupled (daemon/serve binaries, throughput as a tracked metric, GOALS-yaml ratchets). Agentops's merge gate is `scripts/pre-push-gate.sh` and CI's 24 jobs — already enforced mechanically. Re-stating as prose would drift. |
| Per-clone runtime isolation (`OL_HOME`, `BEADS_DIR`) | Olympus crew-clone workflow. Agentops uses repo-local `.agents/` and `.beads/` without `OL_HOME` indirection. |
| Source-of-truth precedence (`docs/specs` > spec-contract > engineering-standards > workflow guides) | Olympus-specific document hierarchy. Agentops's executable-first precedence (CLI > schemas > docs) supersedes it. |
| "Portable Template Intent" closing section | Meta-commentary about reuse, not a standard. Self-fulfilling: this port is the reuse. |
