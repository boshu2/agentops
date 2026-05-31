# Distillation (Opus 4.8) — Google SRE AI Reliability Method

> Single-model structured extraction feeding the [triangulated kernel](../../specs/triangulated_kernel.md). Track A is single-source, so "triangulation" here is across the paper's three internal vantage points (production-ops, eval, SDLC) and a cross-check against the AgentOps convergence cluster, not across multiple distiller models. Disagreements are surfaced, not flattened.

## Method in one line

Wrap a probabilistic agent in a deterministic, human-governed boundary; earn its autonomy against tiered human-verified evals; keep every decision auditable; and move humans from doing the work to architecting the boundary.

## Extracted principles (raw, pre-kernel)

1. The forcing function is **velocity** (~4x); manual review/ops don't scale → restructure, don't add reviewers. (§1, §25)
2. Humans **move up the abstraction ladder** — guardrails, golden data, governance — not preserved manual skill. (§3, §25)
3. **Verification independent of authorship** ("Independent Harnesses") — implementer ≠ tester/reviewer, to kill cross-bias. (§23)
4. **Spec before code**, co-authored and approved. (§24)
5. **Autonomy is earned** through tiered levels (L0–L4); the L2→L3 jump (unsupervised actuation) is the critical, high-rigor gate. (§7, §12, §13)
6. **Eval data is tiered** Bronze/Silver/Gold; Silver calibrated to Gold; measure *true* not *observed* precision. (§14, §15)
7. **Dual scoring**: LLM-as-Judge for reasoning + strict deterministic exact-match for the action. (§16)
8. **Golden data captured in-workflow** (accept/modify/reject at close), no separate chore. (§17)
9. **Reasoning decoupled from execution**: route mutations through a deterministic, caller-agnostic control plane. (§11, §18)
10. **Mandatory dry-run** before any state mutation. (§10)
11. **Bounded + interruptible loops**: rate limits, circuit breakers, stall detection, Red Button. (§9, §20)
12. **Real-time risk eval** auto-downgrades autonomy under elevated risk. (§6, §19)
13. **Non-ambient, least-privilege, attributable identity** for agents. (§8, §21)
14. **Append-only CoT + actuation traces** — full auditability. (§4, §5)
15. **Pulled, grounded context** via MCP/RAG, not ambient or training-only. (§22)
16. **Machine-speed continuous validation** replaces human-paced soak. (§27)

## Single biggest cross-bias to watch

The paper is written from a position of *operating* autonomous production mutation at planetary scale. Two of its principles (raw #9–#13 applied to live infra) are **out of AgentOps scope by design** — AgentOps encodes the same boundary for the *dev loop* (CI as the deterministic plane), not for live-production actuation. Flattening that distinction would over-claim. See kernel UNIQUE/U1.

## The one genuine disagreement

Fix-forward vs. clean rollback at velocity (§26): Google prefers fix-forward; AgentOps' coherent-arc favors clean per-arc rollback. Kept in DISPUTED/D1, routed to `/council` (`ag-7278`).
