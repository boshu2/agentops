# AgentOps Practice Registry

Canonical slugs for `practices: [slug]` citations in cataloged artifacts. This
is an identifier registry, not product positioning or proof that a practice is
universally effective. Product intent lives in `PRODUCT.md`; measurable fitness
lives in `GOALS.md`.

Agent context is finite and session-local, so apply these practices through
bounded, triggered artifacts. Executable checks establish facts; independent
review supplies semantic judgment. Add slugs by appending—never silently rename
an identifier that consumers may cite.

The source consumer is `scripts/validate-practice-citations.sh`. Its default
mode is a report, not a clean-conformance claim. As of 2026-07-13, strict mode
finds 160 missing declarations and 39 invalid slug citations; that known debt
must shrink before this registry can serve as a blocking proof surface. Follow
up: `age-documentation-agents-contract-sd7y1.7`.

## Practice slugs (canonical registry)

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
| `microservices` | 2013-2017 | One bounded context per service (including the premium critique) |
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

- `AGENTS.md` — always-loaded operating contract
- `skills/domain/SKILL.md` — domain vocabulary
- `docs/architecture/operating-loop.md` — execution discipline
- `docs/provenance/README.md` — durable verdict and artifact lineage
