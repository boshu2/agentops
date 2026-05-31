# Primary Source (reference stub) — Google SRE "AI Engineering Reliable Operations"

> **This is a reference stub, not the full text.** AgentOps is a public repository; the full verbatim whitepaper is not re-hosted here to avoid republication. Fair-use excerpts (with attribution) live in [`../quote_bank/quote_bank.md`](../quote_bank/quote_bank.md). The complete verbatim archive is kept in the private operator vault.

## Citation

- **Title:** AI in SRE: How Google is Engineering the Future of Reliable Operations
- **Authors:** Ioannis Papapanagiotou, Stevan Malesevic, Chris Heiser & Ruslan Meshenberg (Google SRE)
- **Published:** 2026-05 (Google SRE resources)
- **Canonical URL:** <https://sre.google/resources/practices-and-processes/ai-engineering-reliable-operations/>
- **Retrieved:** 2026-05-29
- **Full local archive (private vault):** `~/learning/2026-05-29-google-sre-ai-engineering-reliable-operations.md`
- **References cited by the paper:** Gemini CLI for outages (cloud.google.com blog); the Google SRE book series (sre.google/books).

## Abstract (excerpt — fair use)

> "Site Reliability Engineering (SRE) is facing a paradigm shift driven by the rapid adoption of AI throughout the software development lifecycle (SDLC)... Human code review cannot scale linearly with machine-generated code volume... By developing autonomous mitigation agents (AI Operator), strict execution guardrails (Actus), and continuous evaluation pipelines grounded in human operational memory (IRM Analyzer), Google is engineering the autonomous control planes required to safely govern high-velocity, agentic software development."

## Section outline (for anchor navigation)

1. Abstract — the 4x-velocity forcing function; reinventing SRE for the AI era.
2. Introduction — planetary scale, costly mistakes, AI as a transformative layer.
3. Governing AI in Production Operations (AI-Ops)
   - Key Challenges (Operator→Architect, explainability, data integrity, drift, security, runaway automation)
   - The Safety Trifecta (Transparency, Real-time Risk Evaluation, Progressive Authorization)
   - Architectural Guardrails (no ambient access, circuit breakers, mandatory dry-run, safe-by-default actuation)
4. SRE AI Autonomy Levels — L0–L4; progression gates; the L2→L3 critical step.
5. Evaluation Data and Memory — human trajectories (IRMA); Bronze/Silver/Gold tiers; true-vs-observed precision; Nightly Evals + LLM-as-Judge + deterministic scoring; in-workflow Golden capture.
6. AI Across the SRE Lifecycle — Detectr (observe); AI Alert (enrich); Incident Hypothesis + Investigation Dashboards + Gemini CLI (L1); AI Operator + Actus (L3 autonomous mitigation, reasoning/execution decoupling, Red Button).
7. Enabling Technologies — production data, RAG, fine-tuning, MCP, agent identity, A2A.
8. The Future of SRE — agentic SDLC; Independent Harnesses; spec-before-code; adaptive progressive rollouts; the Intervening-PR Problem + AI-assisted fix-forward; review up the abstraction ladder.

## The 11 convergence points (as mapped in `docs/convergence/google-sre.md`)

1. 4x velocity breaks human-paced review · 2. Independent Harnesses (no self-grade) · 3. Tiered eval data + judge + deterministic scoring · 4. Knowledge → deterministic boundary · 5. Humans up the abstraction ladder · 6. Reasoning/execution decoupling · 7. Spec before code · 8. Progressive authorization (L0–L4) · 9. Pulled context / MCP / no ambient access · 10. Append-only CoT + actuation provenance · 11. Fix-forward over binary rollback.

See [`../quote_bank/quote_bank.md`](../quote_bank/quote_bank.md) for the §-anchored excerpts that every kernel axiom and operator cites.
