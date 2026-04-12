# LLM Wiki — architecture and design rationale

> Full design rationale for the `llm-wiki` skill proposal. Read this after `SKILL.md` if you want the "why" behind the "what".

## The three sources we're merging

### Source 1 — Karpathy's LLM Wiki (April 2026 gist)
See `.agents/research/2026-04-11-karpathy-llm-wiki-integration.md` for the full research. The short version:

- **3-layer architecture**: raw (immutable sources) → wiki (LLM-generated markdown) → schema (`CLAUDE.md` / `AGENTS.md` config)
- **3 operations**: ingest / query / lint
- **2 special files**: `index.md` (content catalog) and `log.md` (append-only chronological)
- **Core insight**: *"The tedious part of maintaining a knowledge base is not the reading or the thinking — it's the bookkeeping."* LLMs handle bookkeeping at near-zero cost.

### Source 2 — AgentOps's existing flywheel
AgentOps already compounds **internal** work knowledge via:

- `.agents/research/` — dated deep-dive documents
- `.agents/plans/` — implementation plans
- `.agents/specs/` — specs
- `.agents/findings/` — reusable findings registry with `dedup_key`, `pattern`, `detection_question`, `applicable_when`
- `.agents/learnings/` — distilled post-mortem lessons
- `.agents/council/` — multi-judge review outputs
- `.agents/rpi/` — RPI lifecycle state
- `.agents/pool/` — pending items
- `.agents/planning-rules/` + `.agents/pre-mortem-checks/` — compiled prevention outputs

And via six-phase `post-mortem`: Council → Extract → Process Backlog → Activate → Retire → Harvest Next Work.

And via `skills/compile`: reads raw `.agents/` artifacts and compiles them into an interlinked markdown wiki at `.agents/compiled/`.

**The gap:** none of this handles **external** source material. A user reads an article, a paper, a transcript — today, that content lives wherever the user drops it, and has to be re-discovered per query. AgentOps doesn't have a compounding layer for external reading.

### Source 3 — Community implementations (adding scheduling + tiers)
Two community implementations of the Karpathy pattern add operational details worth borrowing:

- **`NicholasSpisak/second-brain`**: canonical `raw/ wiki/ output/ CLAUDE.md` layout with `wiki/{sources,entities,concepts,synthesis}/` subdirs. Four explicit skills: init, ingest, query, lint.
- **`eugeniughelbur/obsidian-second-brain`**: adds `SOUL.md` (user identity), `CRITICAL_FACTS.md` (always-load context), and **scheduled agents**:
  - Morning 8 AM — daily note creation
  - Nightly 10 PM — five-phase consolidation (close day, reconcile contradictions, synthesize patterns, heal orphans, rebuild index)
  - Weekly Friday 6 PM — structured weekly review
  - Health Sunday 9 PM — vault audit
- Plus **thinking tools** beyond ingest/query/lint: `/obsidian-challenge`, `/obsidian-emerge`, `/obsidian-connect`, `/obsidian-graduate`

The scheduling model is what makes the pattern **actually compound**. Without it, the wiki is just a more structured inbox.

## How the sources compose into the proposed skill

| Component | Source | Role in `llm-wiki` |
|---|---|---|
| `raw/` + `wiki/` layout | Karpathy + NicholasSpisak | The filesystem contract |
| `index.md` + `log.md` | Karpathy | Discovery + auditability |
| `wiki/{sources,entities,concepts,synthesis}/` | NicholasSpisak | Semantic categorization of compiled pages |
| `ingest / query / lint` operations | Karpathy | Core skill phases |
| `SOUL.md` + `CRITICAL_FACTS.md` | eugeniughelbur | Optional always-load context (host decides whether to seed them) |
| **3-tier curator model** | eugeniughelbur + session 3 Home SOC | `--tier` flag on the skill (tier-agnostic implementation) |
| **Scheduled operation** | eugeniughelbur | Host responsibility; skill is callable from any scheduler |
| `promote` operation | AgentOps flywheel (Activate phase of post-mortem) | Mature wiki page → authored content transition |
| Interop with `.agents/` | AgentOps | `llm-wiki` handles external, `.agents/` handles internal, they cross-link |

## The critical distinction: external vs internal knowledge

**This is the load-bearing design choice.** Everything else flows from it.

### Internal knowledge (AgentOps domain)
- Our plans, our research, our learnings, our post-mortems, our findings
- Produced by **our own work** (implementation, review, retrospective)
- Lives under `.agents/`
- Processed by existing skills: `research`, `plan`, `implement`, `post-mortem`, `compile`, `forge`, `harvest`
- Compounds through the six-phase flywheel

### External knowledge (LLM Wiki domain)
- Articles we read, papers we bookmarked, transcripts of talks, clipped web content, other people's docs
- Produced by **outside sources** — we consume, we don't author
- Lives under `raw/` (inbox) and `wiki/` (compiled)
- Processed by the new `llm-wiki` skill
- Compounds through ingest → draft → review → (optionally) promote-to-authored

**Why separate trees:**
- **Provenance is always clear** — if it's under `wiki/sources/`, it came from a raw external doc; if it's under `.agents/learnings/`, it came from our own work.
- **Different review criteria** — external knowledge is reviewed for "is this summary faithful to the source?"; internal knowledge is reviewed for "is this learning true and actionable?"
- **Different expiration** — external knowledge can go stale as the state of the art evolves; internal knowledge is mostly stable (our past decisions don't change).
- **Different promotion targets** — a mature wiki page might become a `platform-lab/patterns/*.md` authored doc; a mature learning might become a `.agents/compiled/*.md` finding.

**Why they cross-link:**
- A research artifact can cite a wiki page (external context informs internal decision)
- A learning can reference a wiki page (this is why we chose X: we read Y)
- A finding can be enriched with wiki-sourced background (our detection question is informed by a concept we read about)

## The 3-tier curator model (optional but recommended)

The Karpathy pattern compounds best when there's **scheduled maintenance**, not just on-demand invocation. The proposed tier model:

### Tier 1 — Always-on cheap local LLM
- **Example implementation**: Gemma 4 26B A4B on a local GPU (Karpathy-style, ~4B active params, fits in 12 GB VRAM)
- **Runs**: nightly, on a schedule (cron / systemd / launchd / Codex hooks)
- **Jobs**:
  - Walk `raw/` for new files
  - Draft summary pages at `wiki/sources/<slug>.md` with `status: draft`
  - Stub entities and concepts in `wiki/entities/` and `wiki/concepts/` with `status: draft`
  - Append to `LOG.md`
  - Run `lint` (report-only, no fixes)
- **Constraints**:
  - **Never** writes `status: reviewed`
  - **Never** touches authored content dirs (`platform-lab/`, `career/`, `.agents/compiled/`, etc.)
  - **Never** edits `SOUL.md`, `CRITICAL_FACTS.md`, `CLAUDE.md`, `INDEX.md` (append-only to `LOG.md` only)
  - **Never** deletes anything
  - **If unsure, do nothing** — leave the decision for Tier 2

### Tier 2 — On-demand expert LLM
- **Example implementation**: Claude / Codex / GPT-5 / similar
- **Runs**: on user command, or on a weekly schedule for batched review
- **Jobs**:
  - Review Tier 1 drafts → promote to `status: reviewed`
  - Answer user queries by reading the wiki
  - Write synthesis pages that tie together multiple sources
  - Detect and resolve contradictions flagged by Tier 1 lint
  - Propose promotions to Tier 3 (move wiki page → authored content)
- **Constraints**:
  - Reviews own output for citation accuracy before writing
  - Never force-promotes without human approval on first pass
  - Logs every significant operation

### Tier 3 — Human
- **Runs**: weekly
- **Jobs**:
  - Review proposed promotions from Tier 2
  - Decide what belongs in long-term memory / authored dirs
  - Archive or delete material that's noise
  - Approve or reject contradictions resolution

## Design anti-patterns to avoid

### 1. Folding existing high-quality content into `wiki/`
If the host project already has authored content (e.g., `platform-lab/patterns/*.md` in bushido-box), **do not** move it into `wiki/concepts/`. Authored content is already well-organized; `wiki/` is for NEW content the LLM compiles from raw sources.

**Rule**: `wiki/` is a destination for LLM-compiled output, not a home for human-authored content.

### 2. Letting Tier 1 touch authored dirs
Tier 1 (local cheap LLM) produces drafts. It should NEVER modify `platform-lab/`, `career/`, `learning/`, `.agents/compiled/`, or any other authored dir. The Tier 1 sandbox is `wiki/sources/`, `wiki/entities/`, `wiki/concepts/`, and appends to `LOG.md`.

Enforcing this means the `--tier 1` mode of the skill has to be **mechanically restricted**, not just a convention.

### 3. Skipping `LOG.md`
Every meaningful operation writes to `LOG.md`. This is the one file that makes the wiki auditable. Without it, you can't answer "what has Tier 1 been doing overnight?" or "when was this concept first introduced?".

### 4. Chasing total coverage
The wiki is valuable when it **compounds**. That compounding comes from Tier 2 pulling out the few high-value pages and promoting them — not from every raw doc becoming a wiki page. If Tier 1 drafts 100 pages nightly, Tier 2 might only review 5. That's fine.

### 5. Syncing `raw/` to git
`raw/` is a local inbox. It may contain paywalled articles, copyrighted PDFs, personal audio transcripts, screenshots with sensitive info. **Gitignore `raw/`** by default. The wiki (LLM-compiled derivative work) is what gets committed.

### 6. Letting the skill write to `.agents/` from external sources
If a wiki page inspires a new finding, the correct path is:
1. Tier 2 notices during review
2. Tier 2 invokes `skills/forge` (or similar) to file the finding into `.agents/findings/`
3. The finding references the wiki page as context

**Do not** have `llm-wiki` write directly to `.agents/`. Keep the external/internal split clean.

### 7. Re-reading raw on every query (regressing to RAG)
The whole point of the pattern is that raw is read **once** and compiled into wiki pages. Subsequent queries read the wiki, not the raw. If the implementation finds itself reading raw on every query, it's implementing RAG, not LLM Wiki.

## Schema files (the `_CLAUDE.md` layer)

The Karpathy pattern specifies that the operator instructions live in a schema-layer file at the project root, typically `CLAUDE.md` or `AGENTS.md`. AgentOps already has both, so the `llm-wiki` skill **extends** them rather than replacing.

Concretely, `llm-wiki init` adds a section to `CLAUDE.md` (or creates it if absent) that:
1. Points at the skill
2. Describes the 3-tier model
3. Lists the wiki layout
4. Provides the 4 workflow templates (ingest/query/lint/promote)

The section is marked with a stable header so subsequent `llm-wiki init` runs can update it idempotently without disturbing the rest of `CLAUDE.md`.

## Integration with `skills/inject`

`skills/inject` already pulls relevant `.agents/*` content into session context at session start. For `llm-wiki` to be useful, inject should also pull relevant `wiki/*` content based on the current cwd or active epic.

Proposed extension to inject (separate PR, not part of this proposal):
- Read `INDEX.md` at session start
- If the current epic / cwd mentions a concept, pre-load the `wiki/concepts/<concept>.md` page
- Cap at ~3 pages to avoid context flooding

## Migration path for projects that already have an informal wiki

Many AgentOps users have an informal "notes dir" or "knowledge dir" that's conceptually similar to `wiki/` but not structured. For them, the migration is:

1. Run `$llm-wiki init --raw existing-notes --wiki wiki` (override defaults to use their existing structure)
2. Run `$llm-wiki ingest existing-notes/**/*.md` to compile into `wiki/`
3. Verify the compiled wiki makes sense
4. Optionally archive the old `existing-notes/` dir once the wiki has caught up

The skill should handle arbitrary `raw/` layouts — `articles/ papers/ transcripts/` are convention, not requirement.

## Output contract

To integrate with the rest of AgentOps, `llm-wiki` operations should emit a structured output contract similar to `skills/council/schemas/verdict.json`. Proposed output shape for `ingest`:

```json
{
  "op": "ingest",
  "skill": "llm-wiki",
  "tier": 2,
  "source": "raw/articles/karpathy-llm-wiki-gist.md",
  "timestamp": "2026-04-11T21:30:00Z",
  "pages_created": [
    "wiki/sources/karpathy-llm-wiki-gist.md",
    "wiki/concepts/llm-wiki.md",
    "wiki/entities/andrej-karpathy.md"
  ],
  "pages_updated": [
    "INDEX.md"
  ],
  "status": "success"
}
```

Proposed output shape for `lint`:

```json
{
  "op": "lint",
  "skill": "llm-wiki",
  "timestamp": "2026-04-11T22:00:00Z",
  "findings": {
    "orphans": [...],
    "broken_links": [...],
    "stale": [...],
    "contradictions": [...],
    "missing_concepts": [...],
    "index_drift": [...]
  },
  "severity": "low|medium|high",
  "report": "wiki/synthesis/lint-2026-04-11.md"
}
```

Output contracts go under `skills/llm-wiki/schemas/` once the proposal is accepted.

## What's NOT in this proposal

- **Concrete implementation** — this is a design proposal. The only artifacts committed are the SKILL.md, this reference doc, and the research that motivated it.
- **The scheduler** — scheduling Tier 1 nightly jobs is host-specific (cron on Linux, launchd on macOS, Task Scheduler on Windows, Codex hooks for always-on sessions).
- **The local LLM runtime** — Gemma 4 / llama.cpp / ollama setup is host-specific. The skill just needs a `--model` or similar flag to target different backends.
- **Plugin manifest updates** — if AgentOps uses a marketplace manifest for skill listing, that entry is a follow-up PR after the proposal is accepted.

## What's next after this proposal

1. **Council review** via `$pre-mortem --preset=product` on this research + SKILL.md
2. **Address council findings** in a second commit (same branch)
3. **Merge to main** as an experimental skill if council PASSes
4. **First implementation** — start with `init` and `ingest` phases, defer `lint` and `promote` to a second PR
5. **Dogfood** on bushido-box (Bo's personal dev box) and one other project for ~2 weeks
6. **Finalize** — promote from experimental to stable, add to marketplace listing

## Related

- `skills/llm-wiki/SKILL.md` — the skill specification
- `.agents/research/2026-04-11-karpathy-llm-wiki-integration.md` — the research artifact
- `skills/compile/SKILL.md` — the existing internal-artifact compiler (structurally identical pattern, different source)
- `skills/research/SKILL.md`, `skills/forge/SKILL.md`, `skills/inject/SKILL.md`, `skills/post-mortem/SKILL.md` — interop surfaces
