---
name: ms
description: meta_skill (ms) — the skill-search/load
---
<!-- TOC: Core Insight | Constraints | Quick Start | Consume (MCP) | Write/Admin (CLI) | Output | Production Skill Handoff | Footguns | Concurrency | Scenarios | Quality | References -->

# ms — meta_skill search/load engine

> **Core Insight:** `ms` is the skill-search engine over both corpora (agentops + jsm, ~175 skills). **Consume via MCP, write/admin via CLI.** One law: after ANY reindex/wipe, every running `ms mcp serve` MUST be killed (sessions respawn fresh). A surviving server silently reads pre-wipe data and returns `recorded:true` on writes that land in orphaned files.

## Constraints

- Load with `full: true` or `--full` when the intent is to execute a skill, because metadata cards and packed overviews omit runnable guidance.
- Keep the consume/write boundary explicit: use MCP for search and load, but use the CLI for feedback and outcomes because only CLI writes are verified to land in the live database.
- Reindex only through `scripts/ms-reindex.sh`, because it sweeps stale servers and proves source equivalence after rebuilding the index.
- Treat the local index as disposable state, not a source of truth; the non-goal is editing indexed content instead of `skills/**`.
- Keep `ms` retrieval-only for production skill work. It returns search and load
  results; the caller owns authoring, validation, and every subsequent decision.

## Quick Start

Find a skill (MCP-primary — BM25, currently strictly better than CLI search), then load the FULL runnable SKILL.md in one call (always `full: true` when you mean to use it):

```bash
mcp__ms__search {query: "handle a rate limit switching accounts"}
mcp__ms__load {skill: "account-rotation", full: true}
```

CLI fallback when no MCP server is attached:

```bash
ms search "switch accounts on rate limit" -O json
ms load account-rotation --full -O json | jq -r '.data.content'
```

State root: `~/Library/Application Support/ms/`.

---

## Consume — MCP-primary (`mcp__ms__*`)

Prefer the MCP tools whenever a `ms mcp serve` is attached — they are the fast, verified read path.

| Tool | Use |
|------|-----|
| `mcp__ms__search {query}` | BM25 search. **Currently strictly better than CLI search** (see Footguns — CLI hybrid is BM25-only; ms never stores doc embeddings). |
| `mcp__ms__load {skill, full: true}` | Returns the full runnable SKILL.md in ONE call, zero extraction friction. **`full: false` returns a useless metadata card — always `full: true` when you intend to use the skill.** |
| `mcp__ms__show {skill}` | Metadata card for a skill. |
| `mcp__ms__suggest {cwd}` | Suggests skills for a directory. Works — but **ignore its project-language detection** (misdetects Makefile repos as C; cosmetic only). |

**CLI fallback** (no MCP server attached):

```bash
ms search "<query>" -O json
ms load <id> --full -O json | jq -r '.data.content'   # content lives in .data.content
```

---

## Write / Admin — CLI-only (verified landing in the live DB)

The MCP feedback tool exists, but **only the CLI write path is verified to land** — trust the CLI for writes.

```bash
ms feedback add <skill> --positive --comment "..."   # feedback on a skill
ms feedback add <skill> --negative --comment "..."

ms outcome <skill> --success   # record only AFTER downstream factory use + validation
ms outcome <skill> --failure

ms doctor                      # admin: health
scripts/ms-reindex.sh          # (re)index THE way: rebuild + sweep + probe + source-equivalence check
scripts/ms-reindex.sh --check-source  # read-only freshness proof against current skills/** source
ms list -O jsonl --limit 1000  # counting / enumeration
ms config                      # resolved config + skill_paths
```

## Output Specification

- **Path:** search, load, and admin results are returned on `stdout`; durable index state remains under `~/Library/Application Support/ms/`.
- **Filename:** no result filename is created by this skill; callers capture CLI output explicitly when they need a durable artifact.
- **Format:** MCP returns structured tool data; CLI automation uses JSON or JSONL, with full skill text at `.data.content` for `ms load --full -O json`.
- **Validation command:** run `skills/ms/scripts/validate.sh` for the retrieval boundary and `scripts/ms-reindex.sh --check-source` for normalized source equivalence.
- **Downstream handoff:** return the loaded guidance and source identity to the caller. Retrieval never chooses or starts a workflow.

## Production Skill Handoff

**Production-intent handoff:** When the query concerns creating or editing a skill, `ms` retrieves relevant guidance and stops. The caller may separately invoke `skill-builder`, `heal-skill`, or another authoring tool.

**Authority boundary:** `skills/**` is canonical source; the generator owns the `ms` Codex twin and other projections. Never edit the index, loaded copies, or generated projections as source.

`ms` never validates or interprets downstream work. A failed search,
load, write, or reindex is returned as evidence and ends this invocation.

**Outcome timing:** Record `ms outcome` only after the caller has independent evidence about downstream usefulness, never after retrieval alone. That observation does not change core state.

---

## Footguns (all measured 2026-07-02)

| Footgun | Truth |
|---|---|
| **MCP server survives a DB wipe/reindex** | An `ms mcp serve` NEVER reopens handles — it follows renamed inodes into the backup, giving stale reads AND silent misdirected writes (`recorded:true` into orphaned files). **Reindex via `scripts/ms-reindex.sh` — THE way to reindex** (rebuilds, TERMs every server, probes a fresh server, then compares normalized local loads with current `skills/**` source); never run bare `ms index` and leave servers up. Sessions respawn fresh. |
| **`ms load --pack N`** | Trap: caps at the gutted `overview` tier for ANY N (`800` == `20000`) — drops the executable steps and returns LESS than the no-flag default. Use `--full` (CLI) or `full: true` (MCP). |
| **`-O plain`** | Prints name-only on `load`; truncates list output (`[N more lines]`). The content lives in `-O json` → `.data.content`. |
| **CLI `ms search` "hybrid"** | Effectively BM25-only — ms never stores doc embeddings (`upsert_embedding` is called only from a unit test), so hybrid ≡ BM25 under ANY backend; no config/backend change fixes it (upstream gap, feature-noted; measured 2026-07-02, age-s3jf). Still loses to MCP BM25 in practice; don't prefer it. |
| **Stale `ms.lock`** | `ms doctor` prints "Lock held" for a DEAD pid yet still says all-pass. A dead-pid lock is safe to delete. |
| **Symlinks** | ms does NOT follow directory symlinks — `skill_paths` must list BOTH roots explicitly: the `~/.codex` skills dir AND the `~/dev/agentops/skills` repo dir. |
| **Binary** | Source build only (`~/dev/meta_skill`, branch `local/frontmatter-id`); the 0.1.2 release binary corrupts IDs on Anthropic-frontmatter skills. Update: `git fetch && git rebase origin/main && cargo install --path . --locked`. |

## Concurrency

Parallel CLI + MCP `load` measured clean — no lock errors. The lock hazard is the survive-a-wipe case above (kill the serve), not concurrent reads.

---

## Scenarios

```gherkin
Scenario: Load a skill's full runnable guidance
  Given an ms mcp serve is attached
  When I call mcp__ms__load {skill: "account-rotation", full: true}
  Then the full runnable SKILL.md content is returned in one call

Scenario: Reindex invalidates every running server
  Given one or more ms mcp serve processes are running
  When I run ms index (or wipe/rebuild the DB)
  Then I kill every ms mcp serve so sessions respawn against fresh data
  And a surviving server would silently read pre-wipe data and mis-land writes

Scenario: A stale local projection fails closed
  Given AgentOps skills are authoritative and ms is a disposable local index
  When a full ms load has a different normalized name or description from source
  Then scripts/ms-reindex.sh exits nonzero and names the stale skill
```

## Quality Checklist

- Full loads preserve the complete runnable guidance rather than a metadata card or packed overview.
- Search/load reads use the verified MCP path, while feedback and outcome writes use the verified CLI boundary.
- Any rebuild finishes with stale servers swept and source equivalence reported.
- Production skill intent leaves `ms` after retrieval; generated twins and loaded/indexed copies are never hand-edited as source.
- Search and load results remain advisory inputs, never proof that downstream work is correct.
- `ms outcome` records observed usefulness only after independent downstream evidence.

## References

- Upstream: Jeffrey Emanuel's `meta_skill` (source at `~/dev/meta_skill`, branch `local/frontmatter-id`).
- Related consume-tool skill in this repo: [`cass`](../cass/SKILL.md) (session archaeology). The jsm `cass-memory` (cm) procedural-memory tool is the write-side complement (installed separately, not in this repo).
- Lifecycle contract validator: [`scripts/validate.sh`](scripts/validate.sh).
