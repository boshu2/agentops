# AgentOps Practice

> Foundation text. Engineering doctrine for this repo. Peer to `PRODUCT.md`
> (positioning), `GOALS.md` (fitness goals), `AGENTS.md` (vault contract).
> Product doesn't own this file; engineering does.

## What this is

The codification of the engineering discipline AgentOps inherits from and
applies. It names the lineage (reverse-traced, only practices proven in
production), the synthesis (what AgentOps IS as a practice), the constraint
that makes it different (consumers are AI agents with finite context windows),
and the implication for every artifact in this repo.

This document is one foundation text among several. When in doubt about HOW
to build, this is the reference. When in doubt about WHAT to build, see
`PRODUCT.md` and `GOALS.md`.

## Lineage — reverse-traced from now to the 90s

Filtered to techniques engineers actually adopted at scale, with the canonical
source. Only practices that survived contact with real production are listed.

### 2024-2026 — agent-assisted dev, post-LLM operations

- **LLM evaluation harnesses + golden-set canaries** — Anthropic eval kits,
  OpenAI evals, DSPy. Proven for drift detection on prompt and model changes.
- **Prompt-as-spec** — Karpathy's "vibe coding"; chain-of-thought elicitation
  (Wei et al). Proven by transcripts shipping working systems.
- **AI-assisted dev with verification harnesses** — Cursor / Claude Code /
  Codex with tests-as-spec. Proven where teams ship and rollback signals
  exist.
- **DORA-at-scale empirical research** — Forsgren / Humble / Kim *Accelerate*
  (2018) plus the State-of-DevOps reports. Replicated across cohorts.

### 2018-2023 — cloud-native maturity, observability, platform engineering

- **GitOps** — Weaveworks 2017, Flux/ArgoCD. Proven: declarative reconcile
  loops survive operator turnover.
- **Distributed tracing** — Google Dapper paper (2010), Zipkin, Jaeger,
  OpenTelemetry. Proven everywhere.
- **eBPF observability** — Cilium, Pixie, Parca. Proven at hyperscale.
- **Team Topologies** — Skelton / Pais 2019. Proven via inverse Conway
  maneuvers in real reorgs.
- **Data contracts / Data mesh** — Zhamak Dehghani 2019; Andrew Jones
  *Driving Data Quality with Data Contracts* 2023. Proven for streaming
  pipelines that don't silently drift.
- **Feature flags as control plane** — LaunchDarkly + DevOps Handbook
  patterns; Etsy / Facebook discipline. Proven.
- **Reproducible / hermetic builds** — Bazel, Nix. Proven for supply-chain
  integrity.
- **SLSA / SBOM / code signing** — Google + OpenSSF. Proven post-Solarwinds.
- **Service mesh** — Linkerd thrived; Istio survived after years of bruising.
  Mixed-proven.

### 2013-2017 — DevOps mainstream, microservices first wave, kubernetes

- ***The Phoenix Project*** — Kim / Behr / Spafford 2013. Synthesis of DevOps
  practices into one operational frame. Proven via mass adoption.
- **Microservices** — Newman *Building Microservices* (2015); Fowler / Lewis
  2014. Proven AND criticized — the "microservices premium" lesson is itself
  proven.
- ***Site Reliability Engineering*** — Beyer et al, Google SRE Book 2016.
  Error budgets, toil reduction, on-call rotations. Proven.
- **The 12-Factor App** — Adam Wiggins / Heroku 2011-2012. Proven by every
  modern web service.
- ***Designing Data-Intensive Applications*** — Kleppmann 2017. Reference
  synthesis of distributed systems lessons.
- ***Continuous Delivery*** — Humble / Farley 2010. Proven via deployment
  frequency and MTTR data.
- ***Release It!*** — Nygard 2007 (2nd ed 2018). Circuit breakers, bulkheads,
  decoupling. Proven in every resilient service.
- **Infrastructure as Code** — Puppet 2005, Chef 2009, Terraform 2014.
- **Docker** — 2013, mainstreamed Linux containers. Proven.

### 2003-2012 — DDD, refactoring, agile/XP maturity, CI/CD birth

- **Domain-Driven Design** — Eric Evans 2003. Bounded contexts, ubiquitous
  language, anti-corruption layer. Twenty years of survival in real
  architecture decisions. **Load-bearing for AI-agent collaboration.**
- ***Working Effectively with Legacy Code*** — Michael Feathers 2004.
  Seam-based testing. Proven for any team inheriting a codebase.
- **Continuous Integration** — Fowler article 2006; Jenkins / Hudson 2004.
  Proven universally.
- **Refactoring** — Fowler 1999 (2nd ed 2018); Opdyke 1992 PhD. Proven
  discipline.
- **Test-Driven Development** — Kent Beck *Test-Driven Development:
  By Example* 2003. Proven where teams sustain it.
- **Behavior-Driven Development** — Dan North 2006; Cucumber / Gherkin
  (Aslak Hellesøy). Proven on product-level acceptance flows.
- **Hexagonal architecture / Ports and Adapters** — Alistair Cockburn 2005.
  Proven for testable boundaries.
- **Architecture Decision Records** — Michael Nygard 2011. Proven by every
  team that uses them — light enough to actually maintain.
- **Property-based testing** — QuickCheck, Claessen / Hughes 2000. Proven
  for finding edge cases impossible to enumerate by example.
- **Approval / golden / snapshot testing** — Llewellyn Falco, Emily Bache.
  Proven for legacy characterization and lock-in regression detection.
- **Event Sourcing / CQRS** — Greg Young 2010. Proven in financial systems
  and high-event-volume domains.
- ***The Lean Startup*** — Eric Ries 2011. Build-measure-learn. Proven
  outside startups too.

### 1996-2003 — Agile birth, Pragmatic Programmer, GoF, design contracts

- **Agile Manifesto** — Snowbird 2001 (Beck, Fowler, Cunningham, Cockburn,
  Jeffries, Schwaber, Sutherland, et al). Proven by displacing waterfall in
  general industry.
- **Extreme Programming** — Kent Beck *XP Explained* 1999, 2nd ed 2004.
  C3 project at Chrysler. Proven where teams maintain the discipline.
- ***The Pragmatic Programmer*** — Hunt / Thomas 1999, 20th-anniversary ed
  2019. Tracer bullets, orthogonality, broken-window theory, DRY at the
  *knowledge* layer (not syntactic). Proven by practitioners worldwide.
- ***Code Complete*** — Steve McConnell 1993, 2nd ed 2004. Proven reference
  manual.
- ***Design Patterns*** / GoF — Gamma / Helm / Johnson / Vlissides 1994.
  Partly proven (creational and structural patterns); partly mocked
  (over-engineering risk). Take what survived.
- ***Object-Oriented Software Construction*** / Design by Contract — Bertrand
  Meyer 1988 / 1997. Eiffel-specific in form, but contract-by-design
  influenced everything from Java assertions to TLA+.
- ***The Mythical Man-Month*** — Brooks 1975, anniversary ed 1995. Conway's
  Law, second-system effect, no-silver-bullet. Foundational.

### Pre-1996 — still load-bearing

- **TCP/IP, HTTP, robust internet engineering** — Postel's Law (RFC 793,
  1981): "be conservative in what you do, liberal in what you accept." The
  default protocol-design principle.
- **The original wiki** — Ward Cunningham 1994. The first knowledge surface
  agents and humans both edit. Pattern-language origin.
- **Garbage collection, B-trees, query planning, leader election** —
  distributed-systems and data-engineering foundations referenced throughout
  Kleppmann.
- **Capability Maturity Model** — SEI 1991. Process-maturity origins;
  partly proven, partly recipe-for-bureaucracy.

## Synthesis

AgentOps is what every practice listed above looks like when the consumer is
an AI agent with a finite context window, not just a human team.

The constraint changes the *size* and *shape* of the artifacts those
practices produce. The practices themselves do not change.

## What the agent-context constraint changes

1. **Granularity** — smaller modules, shorter ADRs, tighter bounded contexts,
   more snapshots. Each artifact must fit one context window.
2. **Linkage over inclusion** — artifacts forward-reference rather than
   embed; agents follow links lazily.
3. **Discovery surfaces are mandatory** — an `INDEX.md` is not optional, it
   is the entry point. A skill without an index is invisible.
4. **Drift is fatal** — human teams could tolerate documentation lag; agents
   follow stale signposts blindly, so a stale signpost actively misleads.
   The forcing functions DevOps invented for code and infrastructure now
   apply to *every* signpost.
5. **Verification is the spec** — TDD/BDD wasn't optional before; it's
   load-bearing now because agents trust the verifier, not the prose.
   A slice without a snapshot or property test is wishful thinking.
6. **Knowledge compounds, or it actively rots** — agents are stateless
   between sessions. Without a structured way to capture and recall what was
   tried, why it changed course, what evidence mattered, every session
   starts from zero. The practices that worked for human teams still work,
   but the cost of skipping them is now immediate, not eventual.

## What does NOT change

- The practices listed above were correct before AI; they are correct after.
- TDD/BDD didn't fail because of agents; it failed because teams didn't keep
  specs and code in sync. Agents make that failure mode louder, not new.
- Pragmatic engineering is still pragmatic. We do not ship perfection; we
  ship the simplest tracer that proves the architecture, then thicken
  (Hunt / Thomas).
- Conway's Law still applies (Brooks). Repo structure mirrors the
  human-AI collaboration topology, not the other way around.
- Postel's Law still applies (RFC 793). Robust under input variation,
  conservative on output.

## Implication for this repo

Every artifact under `skills/`, `hooks/`, `evals/`, `docs/`, `schemas/`,
`scripts/`, and `cli/` is exactly one of:

1. A **bounded context** — a Primitive in our domain vocabulary
   (`skills/domain/references/primitive.md`)
2. A **spec** — a Slice, ADR-shaped and BDD-flavored
   (`skills/domain/references/slice.md`)
3. A **verification** — a test, a snapshot, an eval suite, a hook gate
4. A **discovery surface** — an INDEX, a manifest, or a signpost README

Each artifact is sized to fit one agent context window. Each forward-links
to others. Each has a forcing function (snapshot hash, property test, schema
check, eval suite) that fails when reality drifts from intent.

When you build something new in this repo, ask: which of those four kinds is
this? If the answer is "none", the artifact is probably misplaced. If the
answer is "all four", the artifact is too large.

That is the practice. AgentOps is its codification under the
AI-context-limited constraint.

## Practice slugs (canonical registry)

Every Primitive in this repo (skill, hook, eval suite, CLI command, schema)
declares which practices it embodies via a `practices: [slug, slug, ...]`
field in its frontmatter or header doc. Slugs are kebab-case, stable, and
listed here. Add new slugs by appending — never rename silently.

The registry is the source of truth for `scripts/validate-practice-citations.sh`.
That gate runs in **report-only** mode initially; after one clean cycle of
backfill, it promotes to required.

| Slug | Era | What it names |
|------|------|---------------|
| `llm-eval-harness` | 2024-2026 | LLM evaluation harnesses + golden-set canaries |
| `prompt-as-spec` | 2024-2026 | Prompt-as-spec / chain-of-thought elicitation |
| `ai-assisted-dev` | 2024-2026 | AI-assisted dev with verification harnesses |
| `dora-metrics` | 2024-2026 | DORA-at-scale empirical research (*Accelerate*) |
| `gitops` | 2018-2023 | Declarative-reconcile-loop deployment (Flux / ArgoCD) |
| `distributed-tracing` | 2018-2023 | Dapper / Zipkin / Jaeger / OpenTelemetry |
| `ebpf-observability` | 2018-2023 | eBPF-based introspection (Cilium / Pixie / Parca) |
| `team-topologies` | 2018-2023 | Skelton / Pais inverse-Conway maneuvers |
| `data-contracts` | 2018-2023 | Schema-enforced streaming data contracts |
| `feature-flags` | 2018-2023 | Feature flags as runtime control plane |
| `hermetic-builds` | 2018-2023 | Bazel / Nix reproducible builds |
| `supply-chain-integrity` | 2018-2023 | SLSA / SBOM / code signing |
| `service-mesh` | 2018-2023 | Linkerd / Istio cross-cutting transport |
| `devops` | 2013-2017 | *The Phoenix Project* operational frame |
| `microservices` | 2013-2017 | One bounded context per service (incl. premium critique) |
| `sre` | 2013-2017 | Google SRE Book: error budgets, toil, on-call |
| `twelve-factor-app` | 2013-2017 | Heroku 12-Factor App |
| `distributed-systems-design` | 2013-2017 | Kleppmann DDIA synthesis |
| `continuous-delivery` | 2013-2017 | Humble / Farley deployment pipeline |
| `resilience-patterns` | 2013-2017 | *Release It!* — circuit breakers, bulkheads |
| `infrastructure-as-code` | 2013-2017 | Puppet / Chef / Terraform |
| `containers` | 2013-2017 | Docker / Linux namespaces + cgroups |
| `ddd-bounded-context` | 2003-2012 | Evans DDD: bounded context + ubiquitous language |
| `legacy-code-seams` | 2003-2012 | Feathers *Working Effectively with Legacy Code* |
| `continuous-integration` | 2003-2012 | Fowler CI; Jenkins / Hudson |
| `refactoring` | 2003-2012 | Fowler 1999 / Opdyke 1992 |
| `tdd` | 2003-2012 | Beck Test-Driven Development |
| `bdd-gherkin` | 2003-2012 | North BDD + Cucumber/Gherkin |
| `hexagonal-architecture` | 2003-2012 | Cockburn Ports and Adapters |
| `adr` | 2003-2012 | Nygard Architecture Decision Records |
| `property-based-testing` | 2003-2012 | QuickCheck (Claessen/Hughes) |
| `snapshot-testing` | 2003-2012 | Approval / golden / snapshot (Falco / Bache) |
| `event-sourcing-cqrs` | 2003-2012 | Greg Young event sourcing + CQRS |
| `lean-startup` | 2003-2012 | Ries build-measure-learn |
| `agile-manifesto` | 1996-2003 | Snowbird 2001 Agile principles |
| `xp` | 1996-2003 | Kent Beck Extreme Programming |
| `pragmatic-programmer` | 1996-2003 | Hunt / Thomas tracer bullets + orthogonality |
| `code-complete` | 1996-2003 | McConnell reference manual |
| `design-patterns` | 1996-2003 | GoF (with over-engineering caveat) |
| `design-by-contract` | 1996-2003 | Meyer OOSC — preconditions / postconditions / invariants |
| `mythical-man-month` | 1996-2003 | Brooks — Conway's Law, no-silver-bullet |
| `postels-law` | pre-1996 | RFC 793 — robust under input variation |
| `wiki-knowledge-surface` | pre-1996 | Cunningham wiki — first knowledge surface |
| `distributed-systems-foundations` | pre-1996 | Lamport / Brewer / leader election / B-trees |
| `cmm-process-maturity` | pre-1996 | SEI Capability Maturity Model (with bureaucracy warning) |

Slug count: 45.

## See also

- `PRODUCT.md` — what AgentOps is as a product
- `GOALS.md` — measurable fitness goals
- `AGENTS.md` — operator vault contract for this repo
- `skills/domain/SKILL.md` — vocabulary corpus citing this practice
- `docs/architecture/primitive-chains.md` — the concrete primitive layers
  (Mission / Discovery / Risk / Execution / Validation / Learning / Ratchet
  / Continuity) that compose the practice into chains
