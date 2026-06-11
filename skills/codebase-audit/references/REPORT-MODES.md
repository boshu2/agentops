# Report Modes — folded sibling methods

> Folded from five retired sibling skills (skill-prune Phase 2, 2026-06-11):
> codebase-archaeology, codebase-report, codebase-briefing-report,
> codebase-pattern-extraction, codebase-risk-audit. Each mode below is that
> skill's operational core. Full originals: branch `archive/skill-prune-phase2-20260611`.

## Mode selection

| Mode | Output | Use when |
|------|--------|----------|
| **archaeology** | working mental model (notes) | onboarding, "what does this do", legacy code |
| **architecture report** | reusable architecture doc | handoff, docs, "write up what this does" |
| **briefing report** | one shareable stakeholder snapshot | report read cold by non-authors |
| **pattern extraction** | reusable artifact (lib/skill/template) | "I've seen this before", DRY across repos |
| **risk audit** | prioritized remediation plan | pre-launch, risk review, decision aid |

## archaeology — build the mental model

Documentation FIRST: `cat AGENTS.md README.md` before any code. Then:

```
Thoroughly explore this codebase. I need to understand:
1. Overall architecture and module structure
2. How data flows through the system (input → processing → output)
3. Key data structures (the 3-5 types everything revolves around)
4. The integration points (external APIs, databases, file I/O)
5. Configuration system (env vars, config files, CLI flags)
6. Test infrastructure
Map out how the pieces fit together — I need a complete mental model.
```

Phased: orientation (docs + dir structure + deps, 2 min) → entry points
(`fn main`, CLI frameworks, HTTP routers, 5 min) → core types (structs/classes/
interfaces + trait impls, 5 min) → trace one end-to-end path.

## architecture report — durable doc

```
Produce a Comprehensive Technical Architecture Report for this codebase:
1. Executive summary (what is it, key stats)
2. Entry points (main, routes, handlers)
3. Key types (3-5 core domain objects)
4. Data flow (input → processing → output)
5. External dependencies (DBs, APIs, critical libs)
6. Configuration (env, files, CLI, precedence)
7. Test infrastructure
Include file:line references. Output as markdown I can reference later.
```

Depth tiers: Quick Scan (10 min — entry + types + flow, under 150 lines),
Standard (30 min — full template), Deep Dive (1+ hr — diagrams, all paths).

## briefing report — stakeholder snapshot

One self-contained Markdown file a non-author can read cold. Disciplines:

- **Evidence, not impressions** — every claim traces to a file/command output.
- **Snapshot, not history** — one commit, SHA in the header; no investigation log.
- **Risks named and rated** — one-line statement + Low/Med/High + next action; "looks good" is a failure.
- **No fabricated metrics** — write "not measured", never estimate.
- **One artifact** — a single file, not scattered findings.

Phases: Orient (identity in one sentence) → Map (module table: name,
responsibility, key files, depends-on; every top-level source dir exactly once)
→ Measure (LOC, tests, deps — only what you actually computed) → Health (risks).

## pattern extraction — mine the recurrence

```
Extract a reusable pattern from these projects:
1. Collect: find 3+ instances of the pattern across different projects
2. Diff: what's common? what varies per-project?
3. Abstract: pull out the invariant core
4. Parameterize: make the varying parts configurable
5. Package: create library/skill/template with tests
Output: reusable artifact + usage examples from the originals.
```

Use `cass search "<technique>" --robot` to find instances across ALL past
sessions, not just the current project; `--aggregate workspace` shows which
projects share the pattern.

## risk audit — prioritized remediation

Decision aid, not general critique. Workflow: establish the system map → trace
critical paths (startup, mutation, authn/authz, persistence, recovery — prefer
paths crossing module/process boundaries) → inspect risk surfaces as separate
lenses (architecture, operations, testing, security-adjacent, maintainability)
→ verify with evidence (file:line, commands run; never naming/style/speculation)
→ prioritize by severity × likelihood × blast radius × reversibility × cost →
**report residual risk** (what was not inspected and why).

Key lens prompts: hidden ordering / ambient config dependencies; module
boundaries vs runtime ownership mismatch; missing retry/timeout/backoff/
cancellation; deployment steps depending on manual state; unsafe config
defaults; background jobs that duplicate/drop work.

Output shape: executive risk table (Pri P1-P3 + one-line risk), then per-finding
Evidence / Impact / Likelihood / Remediation / Owner boundary, then a
time-phased remediation plan (immediate / near-term / later).
