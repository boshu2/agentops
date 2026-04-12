---
type: research
date: 2026-04-11
topic: Karpathy LLM Wiki pattern — integration into AgentOps
author: boshu2 (via Codex session 4 on bushido-box)
status: proposal
maturity: provisional
---

# Karpathy LLM Wiki × AgentOps integration research

## Problem statement

AgentOps has a mature **flywheel** for internal work artifacts (research → plans → specs → implementations → post-mortems → learnings → findings → compiled planning rules / pre-mortem checks). That flywheel compounds project-lifecycle knowledge: what we did, what we learned from it, what we should check next time.

What AgentOps doesn't explicitly have is a **compounding layer for external knowledge** — the articles, papers, transcripts, and clipped web content a user reads over time. Today that material lives wherever the user dropped it (Obsidian, Notion, a random download directory), and it has to be re-discovered per query via RAG or full-text search.

In April 2026, Andrej Karpathy published a gist titled "LLM Wiki" that proposes a specific pattern for solving the external-knowledge compounding problem. It went viral and several community implementations now exist. This research evaluates the pattern, identifies what's genuinely new, and proposes an integration path into AgentOps.

## What Karpathy's LLM Wiki actually is

Per the source gist at <https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f>:

### Three-layer architecture
1. **Raw layer** — immutable curated sources (articles, papers, images, data files). The LLM reads these but never modifies them.
2. **Wiki layer** — LLM-generated markdown pages: summaries, entity pages, concept pages, comparisons, synthesis. The LLM owns this layer entirely — create, update, cross-reference, maintain consistency.
3. **Schema layer** — a config document (typically `CLAUDE.md` or `AGENTS.md`) specifying wiki structure, conventions, and the workflows for ingesting sources, answering queries, and maintaining the wiki.

### Three operations
- **Ingest**: process new sources by reading them, discussing takeaways, writing summaries, updating the index, revising relevant pages, appending to a log.
- **Query**: search relevant wiki pages, synthesize answers with citations, file valuable query results back as new wiki pages.
- **Lint**: periodic health check for contradictions, stale claims, orphan pages, missing concepts, data gaps.

### Two special files
- `index.md` — content-oriented catalog listing every page with a one-line summary, organized by category. The canonical discovery surface: "if a page isn't in the index, it effectively doesn't exist."
- `log.md` — append-only chronological record of ingests, queries, and lint passes with consistent prefixes for machine parseability.

### Core insight (direct quote)
> "The tedious part of maintaining a knowledge base is not the reading or the thinking — it's the bookkeeping."

LLMs don't abandon knowledge systems due to maintenance burden. They handle cross-references, consistency, and updates at near-zero cost. Humans stop maintaining wikis because the bookkeeping is boring; LLMs don't.

### Metaphor
> "Obsidian is the IDE. The LLM is the programmer. The wiki is the codebase."

The user curates what enters the system (what's in `raw/`); the LLM handles all structural organization and knowledge-graph maintenance.

## Community implementations evaluated

### `NicholasSpisak/second-brain` — reference implementation
A minimal Codex skill set that implements the Karpathy pattern. Canonical layout:

```
vault/
├── raw/           # inbox for source materials
│   └── assets/    # images, attachments
├── wiki/
│   ├── sources/   # per-raw-doc summaries
│   ├── entities/  # people, organizations, products, tools
│   ├── concepts/  # ideas, frameworks, theories
│   ├── synthesis/ # comparisons, analyses, cross-cutting themes
│   ├── index.md
│   └── log.md
├── output/        # generated reports
└── CLAUDE.md      # agent configuration
```

Four skills drive the operations: `/second-brain` (setup wizard), `/second-brain-ingest` (source → wiki compilation), `/second-brain-query` (wiki retrieval), `/second-brain-lint` (integrity checking).

Lifecycle: **Clip → Ingest → Compile → Browse → Maintain**.

### `eugeniughelbur/obsidian-second-brain` — richer version
Adds several specific enhancements worth borrowing:

- **`SOUL.md`** — user identity file (under 500 words, loaded early by every agent)
- **`CRITICAL_FACTS.md`** — ~120-token always-load context for fast agent bootstrapping
- **`_CLAUDE.md`** — explicit operating manual (underscore prefix to sort first)
- **Scheduled agents** (this is the key innovation):
  - **Morning 8 AM**: creates daily note with calendar events and overdue tasks
  - **Nightly 10 PM**: five-phase consolidation — closes day, reconciles contradictions, synthesizes patterns, heals orphan notes, rebuilds index
  - **Weekly Friday 6 PM**: structured weekly review
  - **Health Sunday 9 PM**: vault audit checking for contradictions, gaps, and stale information
- **Thinking tools** (beyond ingest/query/lint):
  - `/obsidian-challenge` — vault argues against your ideas using your history
  - `/obsidian-emerge` — surfaces unnamed patterns
  - `/obsidian-connect` — bridges unrelated domains
  - `/obsidian-graduate` — converts ideas into full projects

The scheduled-agent model lets the system run **continuously** instead of only on-demand, which is where the real compounding happens.

## Existing tools in the ecosystem

- **openclawai — Second Brain Builder** (on Apify): automates the Karpathy pattern as a hosted service
- Multiple Medium / Substack articles in April 2026 describing variations of the same three-layer-plus-schema approach

The common thread across all of them: **the LLM does the bookkeeping; the human decides what enters the system**.

## What AgentOps already has vs what's missing

### AgentOps already has
- `.agents/learnings/` — distilled durable lessons (post-mortem artifacts)
- `.agents/findings/` — reusable findings registry with `dedup_key`, `pattern`, `detection_question`, `applicable_when`
- `.agents/research/` — dated deep-dive documents (THIS FILE)
- `.agents/plans/` — implementation plans
- `.agents/specs/` — specs
- `.agents/council/` — multi-judge review outputs
- `.agents/rpi/` — RPI lifecycle state
- `.agents/pool/` — pending items
- `.agents/planning-rules/` + `.agents/pre-mortem-checks/` — compiled prevention outputs
- `skills/post-mortem` — six-phase knowledge flywheel (Council → Extract → Process Backlog → Activate → Retire → Harvest Next Work)
- `skills/forge` — mine transcripts for knowledge (decisions, learnings, failures, patterns)
- `skills/compile` — knowledge compiler that reads raw `.agents/` artifacts and produces an interlinked wiki at `.agents/compiled/`
- `skills/inject` — inject relevant knowledge into session context
- `skills/knowledge-activation` — operationalize a mature `.agents/` corpus

### AgentOps does NOT have explicitly
- A dedicated **raw inbox** for external source material (articles, papers, transcripts) that aren't specifically tied to an epic or project
- An explicit **3-layer raw / wiki / schema** taxonomy for external knowledge
- A defined **ingest / query / lint / promote** workflow for external material
- **Tiered curation** (cheap always-on drafting + expensive on-demand review) beyond what `compile` already partially does
- **Schema-layer files** at vault/project root (`SOUL.md`, `CRITICAL_FACTS.md`, `INDEX.md`, `LOG.md`) per the Karpathy pattern

### The interesting overlap — `skills/compile`
AgentOps's existing `compile` skill is doing something **very close** to Karpathy's wiki-layer compilation:
> "Knowledge compiler. Reads raw .agents/ artifacts (learnings, research, patterns, retros, findings) and compiles them into an interlinked markdown wiki at .agents/compiled/."

But `compile` operates on **internal** artifacts (the output of other AgentOps skills). Karpathy's LLM Wiki operates on **external** artifacts (clipped articles, papers, transcripts). The two patterns are structurally identical but operate on different sources.

**Integration insight:** The Karpathy pattern can be implemented as a new skill `llm-wiki` that extends the existing `compile` approach to also handle an external `raw/` inbox, using the same wiki output conventions.

## Proposed integration path

### Option A — New skill: `skills/llm-wiki/`
Add a new skill that implements the full ingest/query/lint/promote pipeline for external sources. Key design decisions:
- **Location**: `skills/llm-wiki/` with subcommands mirroring the four Karpathy operations
- **Data layout**: `raw/` and `wiki/` directories at project root (sibling to `.agents/`), NOT nested under `.agents/` — keeps the external-knowledge layer visually distinct from the internal-work layer
- **Schema files**: `SOUL.md`, `CRITICAL_FACTS.md`, `INDEX.md`, `LOG.md` at project root — optional but recommended
- **Tier model**: document the 3-tier curator pattern in references; skill itself is tier-agnostic (a Tier 1 local-LLM caller or a Tier 2 Claude caller both use the same skill, with different `--tier` flag)
- **Interop with existing skills**:
  - `skills/compile` continues to handle **internal** artifact compilation (`.agents/*` → `.agents/compiled/`)
  - `skills/llm-wiki` handles **external** source compilation (`raw/` → `wiki/`)
  - `skills/forge` can feed findings from `wiki/` into `.agents/findings/` when a concept in the wiki crystallizes into a reusable pattern
  - `skills/inject` can learn to also inject relevant `wiki/` pages into session context alongside `.agents/*` content

### Option B — Extensions to existing skills
Instead of a new skill, extend:
- `skills/compile` to optionally process `raw/` → `wiki/` alongside `.agents/` → `.agents/compiled/`
- `skills/research` to add an "ingest from raw" mode that pulls in external material before writing the research artifact
- `skills/inject` to pull from `wiki/` as well as `.agents/`

### Option C — New plugin / extension module
Treat the LLM Wiki as a plugin surface that sits alongside AgentOps but isn't "part of" it. Cleaner separation but loses the tight integration with the flywheel.

### Recommended: **Option A (new skill)**, seeded as a proposal in this branch
The new skill avoids disturbing existing ones, keeps the external-knowledge taxonomy visually distinct, and gives us a concrete artifact to iterate on. The references doc explains how it interoperates with the flywheel without mandating changes to existing skills.

## Tier model (for reference — applies to any host vault using the skill)

The proposed `llm-wiki` skill is tier-agnostic but assumes the host project uses (or may use) a tiered curator model:

```
Tier 3: Human             ← approves promotion into authored content / MEMORY.md
Tier 2: Claude / Codex    ← on-demand expert review, full ingest/query/lint, authoring
Tier 1: Local LLM (Gemma) ← scheduled ingest draft, nightly lint, orphan detection
```

Tier 1 writes only `wiki/sources/`, `wiki/entities/`, `wiki/concepts/` with `status: draft`. Tier 2 reviews drafts and promotes to `status: reviewed`. Tier 3 (human) promotes to authored content in `platform-lab/` or similar. This mirrors the AgentOps flywheel's "collect → process backlog → activate → retire" lifecycle applied to external knowledge.

## Open questions (worth a council pass)

1. **Where do the schema files live?** Root of the project, or root of the `wiki/`? Root is more discoverable but clutters the top-level; `wiki/` nests them but agents have to walk into the wiki to find the schema that describes the wiki. **Recommendation:** root.
2. **Should `llm-wiki` ship with Tier 1 scripts or hooks?** A cron/systemd/launchd setup for the nightly jobs is non-trivial across platforms. **Recommendation:** ship the prompts + the skill; leave scheduling to the host (via hooks or external schedulers).
3. **Interop with `skills/compile`**: do they share a wiki output dir, or keep `.agents/compiled/` and `wiki/` separate? **Recommendation:** separate — compile produces `.agents/compiled/` from internal, llm-wiki produces `wiki/` from external. They can cross-link but don't merge.
4. **Does this need a plugin manifest entry?** If AgentOps uses marketplace metadata for skill listing, yes — add to the manifest. **Action:** find the manifest (if any) and add an entry.
5. **Does this conflict with `agentops:flywheel`?** No — flywheel monitors the compounding of internal artifacts; llm-wiki extends the compounding surface to include external sources. They complement.

## Sources

- [Andrej Karpathy's LLM Wiki gist (April 2026)](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) — the original pattern
- [NicholasSpisak/second-brain](https://github.com/NicholasSpisak/second-brain) — reference implementation with raw/wiki/output layout
- [eugeniughelbur/obsidian-second-brain](https://github.com/eugeniughelbur/obsidian-second-brain) — richer Codex skill with scheduled agents + thinking tools + SOUL.md / CRITICAL_FACTS.md pattern
- [Why Andrej Karpathy's "LLM Wiki" is the Future of Personal Knowledge (Medium, evoailabs)](https://evoailabs.medium.com/why-andrej-karpathys-llm-wiki-is-the-future-of-personal-knowledge-7ac398383772)
- [Karpathy shares "LLM Knowledge Base" architecture that bypasses RAG (VentureBeat)](https://venturebeat.com/data/karpathy-shares-llm-knowledge-base-architecture-that-bypasses-rag-with-an)
- [Karpathy's LLM Knowledge Base: Build an AI Second Brain (codersera)](https://ghost.codersera.com/blog/karpathy-llm-knowledge-base-second-brain/)
- [Second Brain Builder — Karpathy's LLM Wiki, Automated (Apify, openclawai)](https://apify.com/openclawai/second-brain-builder)
- [Karpathy's Instructions for Building an AI-Driven Second Brain (Techstrong.ai)](https://techstrong.ai/features/karpathys-instructions-for-building-an-ai-driven-second-brain/)
- [LLM Wiki Revolution (Analytics Vidhya, April 2026)](https://www.analyticsvidhya.com/blog/2026/04/llm-wiki-by-andrej-karpathy/)
- [Karpathy's LLM Wiki: Bye Bye RAG (Medium, Mehul Gupta)](https://medium.com/data-science-in-your-pocket/andrej-karpathys-llm-wiki-bye-bye-rag-ee27730251f7)

## Provenance

This research was produced in the context of bushido-box's personal vault project (see [[platform-lab/patterns/llm-wiki-architecture]] in that vault for the host-specific design). The vault-side doc contains the same sources and a concrete filesystem layout; this AgentOps-side doc abstracts the pattern for generalized integration into the AgentOps skill ecosystem.

**Next steps** (from this research):
1. Commit this research artifact (done)
2. Draft `skills/llm-wiki/SKILL.md` as a skill proposal (done in same branch)
3. Draft `skills/llm-wiki/references/architecture.md` (done in same branch)
4. Open PR with label `proposal` — invite council review via `agentops:pre-mortem --preset=product`
5. Iterate based on council feedback; either merge as experimental skill or archive as rejected proposal
