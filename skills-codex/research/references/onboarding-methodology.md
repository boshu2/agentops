# Onboarding Methodology

> Build a working mental model of an unfamiliar codebase fast. Read the docs first, locate entry points, then trace one representative path to its sink — never random file reads.

## Problem

Landing in a new codebase, the temptation is to grep for keywords or open files at random. That burns context without producing structure. Onboarding research needs a repeatable shape: orient on docs, locate entry points, identify the 3–5 types everything revolves around, then trace one representative flow end-to-end. The output should be reusable: another agent (or future you) reads the summary and skips the cold-start cost.

---

## Phased Walk

| Phase | Goal | Time box | Output |
|-------|------|----------|--------|
| 1. Orient on docs | Pull what is already written down | 2 min | Notes on stated purpose, conventions, gotchas |
| 2. Inventory the surface | Directory layout, dependencies, build system | 3 min | Annotated tree of top-level dirs |
| 3. Locate entry points | `main`, CLI commands, HTTP routes, queue consumers | 5 min | List of `file:line` for each entry surface |
| 4. Identify core types | The 3–5 structs/classes everything else references | 5 min | Type table with location and purpose |
| 5. Trace one flow | Pick the most representative entry → output path | 10 min | Linear data-flow diagram |
| 6. Note integrations | DBs, external APIs, file I/O, queues | 3 min | Dependency table |
| 7. Skim tests | What invariants does the test suite assert? | 2 min | List of behavioral guarantees found |
| 8. Write the summary | Reusable mental-model artifact | 5 min | Document under `.agents/research/` |

If a phase has no signal in 90 seconds, skip and note the gap.

---

## Phase 1: Documentation First

Read in this order before opening source:

```bash
cat AGENTS.md     # Project rules, architecture decisions, gotchas
cat CLAUDE.md     # Same — most repos symlink one to the other
cat README.md     # Stated purpose, install, primary workflows
ls docs/ && cat docs/index.md docs/architecture.md 2>/dev/null
```

Capture three things from this pass:
1. The project's stated purpose in one sentence.
2. The top 3 conventions or rules the docs call out.
3. Any explicit "do not touch" or "load-bearing" warnings.

Skipping this phase is the most common onboarding failure — it makes you rediscover documented constraints by trial and error.

---

## Phase 2: Inventory the Surface

```bash
ls -la                          # Top-level shape
ls -la src/ lib/ cmd/ pkg/      # Source roots
cat Cargo.toml package.json pyproject.toml go.mod 2>/dev/null
```

Annotate each top-level directory with a one-line guess at its role. Confirm the guesses in later phases.

---

## Phase 3: Entry Points

Use language-aware searches — see `skills/research/references/context-discovery.md` for tier ordering. Patterns to look for:

| Surface | Signals |
|---------|---------|
| Process entry | `fn main`, `def main`, `func main`, `if __name__ == "__main__"` |
| CLI surface | clap/cobra/click/typer/commander/yargs derivations, command registration calls |
| HTTP surface | route registration calls, decorator usage, router builders |
| Queue/event surface | consumer/handler/subscriber registration |
| Scheduler surface | cron/timer/job declarations |

Record each as `file:line` — these become navigation anchors in the summary.

---

## Phase 4: Core Types

Look for the 3–5 types everything else flows through. Signals:

- Mentioned in most files when grepped by name.
- Returned or consumed by multiple entry-point handlers.
- Declared in a `model.rs`, `types.ts`, `schema.py`, or equivalent root.

Capture each in a table: name, location, purpose, key fields. If you cannot describe the purpose in one sentence, the type is not yet understood — flag it as a gap.

---

## Phase 5: Trace One Flow

Pick the most representative entry-point handler. Walk it:

1. Read the handler. Note every function it calls.
2. For each callee, decide: do I need to open it, or is the name self-describing?
3. Stop when you hit storage, an external API, or a return that closes the loop.
4. Write the path as a linear arrow chain.

One traced flow is more useful than five half-traced flows.

---

## Phase 6 & 7: Integrations and Tests

Integrations: list the DBs, HTTP clients, file paths, and queues touched by the traced flow. Note the library used for each.

Tests: read 1–2 test files for the traced flow. The asserts reveal which behaviors the team treats as invariants.

---

## Mental-Model Output Template

Write the summary as `.agents/research/YYYY-MM-DD-<project>-mental-model.md` using this shape. Keep it under one page.

```markdown
---
date: YYYY-MM-DD
type: Research
topic: "<project> onboarding mental model"
tags: [research, onboarding, architecture]
status: COMPLETE
---

# <Project> — Mental Model

## Executive Summary
<2–3 sentences: what it is, what it does, the one architectural choice that defines it.>

## Entry Points
| Surface | Location | Purpose |
|---------|----------|---------|
| CLI | `src/main.rs:15` | clap parser, dispatches to subcommand |
| HTTP | `src/routes/mod.rs:1` | axum router, mounts `/api/*` |

## Key Types
| Type | Location | Purpose |
|------|----------|---------|
| `Project` | `src/model.rs:10` | Core domain object |
| `Config` | `src/config.rs:5` | Runtime configuration loaded once |
| `Storage` | `src/storage.rs:1` | Persistence boundary |

## Data Flow (representative path)
CLI args → `Config::load()` → `Project::process()` → `Storage::save()`

## External Dependencies
| System | Library | Where touched |
|--------|---------|---------------|
| SQLite | rusqlite | `src/storage.rs` |
| HTTP   | reqwest  | `src/clients/api.rs` |

## Configuration Surfaces
| Source | Example |
|--------|---------|
| Env var | `CONFIG_PATH=/etc/tool.toml` |
| File | `~/.config/tool/config.toml` |
| Flag | `--verbose` |

## Testing Surface
- `tests/integration_test.rs` covers the CLI → storage path end-to-end.
- Property tests in `tests/prop/` assert <invariant>.
- Gaps: <untested surfaces noted during the read>.

## Gaps and Open Questions
- <Files skipped because purpose unclear>
- <Areas where docs disagree with code>
```

---

## Anti-Patterns

| Avoid | Do instead |
|-------|------------|
| Skipping `AGENTS.md`/`README.md` | Always read them first; they save hours |
| Random file reads | Walk entry → handler → core type → storage |
| Reading full files end-to-end | Skim structure, dive into the 1–2 critical functions |
| Ignoring tests | Tests reveal the invariants the team enforces |
| Filling context with raw source | Synthesize into the template; cite `file:line` |
| Summarizing everything you read | Cut to the 3–5 core types and one traced flow |

---

## Checklist

- [ ] `AGENTS.md` and `README.md` read before any source file.
- [ ] Top-level directory annotated.
- [ ] Entry points listed with `file:line`.
- [ ] 3–5 core types named with one-sentence purposes.
- [ ] One representative flow traced end-to-end.
- [ ] Integrations and tests noted.
- [ ] Summary written under `.agents/research/` using the template.
- [ ] Gaps explicitly listed — no false completeness.

---

> Pattern adopted from `codebase-archaeology` (ACFS skill corpus). Methodology only — no verbatim text.
