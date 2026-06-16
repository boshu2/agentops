# Codebase Audit — Risk Report (v3.1.0 → HEAD)

**Date:** 2026-06-14
**Skill:** `review` (codebase-audit fold) — **report mode: risk**
**Scope (strict):** the diff `v3.1.0..HEAD` only — NOT the whole repo.
**Reference points:** `v3.1.0` = `c98836977` (2026-06-10) · `HEAD` = `ab6039808` (2026-06-14)
**Release window:** 147 commits · 1602 files changed · **+37,404 / −109,471** (large net deletion)
**Verification run (read-only):** `cd cli && go build ./...` → OK · `go vet ./...` → clean · targeted tests for every new Go surface (`provenancegraph`, `liveness`, `gates/checks`, `cmd/ao` converge/provenance/codex/quorum) → **all pass**.

---

## Executive summary

This is a **consolidation-and-teardown release**, not a feature-expansion release. The −109k LOC is dominated by **skill culls** (council references slimmed; `bug-hunt`, `vibe`, `reverse-engineer-rpi`, `gh-actions`, `bd-first-memory-migration`, and a long tail of single-use skills fully removed) and **codex-twin teardown**. Against that, ~+6.7k Go LOC of genuinely new, security-relevant machinery landed: a **tamper-evident SDLC provenance ledger**, a **context-quorum gate rewrite**, a **bounded converge judge-panel loop**, a **codex worker-as-image dispatch + read-only image-health checker**, and an `ao skills retire` tool that mechanizes the cull.

**Overall risk posture: LOW-to-MODERATE, well-controlled.** The new code is tested, builds clean, vets clean, and the most security-sensitive surfaces (quorum gate, provenance chain, codex dispatch env) are deliberately **fail-closed** and carry explicit LAW-0 guards. The release **removes** more risk than it introduces (SPOF docs, dead skill surface, stale codex twins). The residual risk is concentrated in (a) a **deliberate doctrine weakening** of the quorum default, (b) a **trust-boundary assumption** in codex dispatch's `sh -c`, (c) the **unkeyed** (tamper-*evident*, not tamper-*proof*) provenance chain, and (d) **doc-reference hygiene** drift from the culls.

No Critical findings. **0 Critical · 1 High · 4 Medium · 4 Low.**

---

## What changed (archaeology of the window)

| Surface | Direction | Notes |
|---|---|---|
| Commit mix | — | 52 feat · 26 fix · 23 docs · 20 chore · 4 merge · 2 test (+ **24 merge commits** total — heavy rebase/splice activity, mostly the `ag-xwjlc` seams epic) |
| Skill trees | **removed** | `reverse-engineer-rpi` (41+40 files codex/main), `council/references` (37+25 slimmed, SKILL.md kept), `gh-actions` (21), `bd-first-memory-migration` (19), `vibe` (15), `bug-hunt` (12+12), plus a long tail |
| Generated inventories | churned | `registry.json` (5669Δ), `skills/catalog.json` (2648Δ), `docs/contracts/skill-dispositions.yaml` (1992Δ) — regenerated, not hand-edited (correct) |
| New Go (cli/) | **added** | `+6714 / −228` across 43 files: `provenance_verify.go`, `provenancegraph/verify.go`, `converge.go`, `converge_canary.go`, `codex_schema.go`, `skills_retire.go`, `gates/checks/workflow_install.go`, plus `codex.go` (+1296), `liveness/quorum.go` rewrite |
| Tests | **added** | 11 new `_test.go` files covering every new surface (converge, provenance, codex dispatch/image-health/schema, quorum canon, workflow_install, skills_retire) |
| Planning artifacts | **added** | 60 files under `docs/plans/` (bdd-foundry run-1/run-2, behaviors.md 1069Δ) — content/process, not executable risk |
| Doctrine docs | added | provenance ledger, navigator/GPS goal, AgentOps↔MTO two-factory framing |

---

## SCORED assessment (of the diff)

| Category | Rating | Notes |
|---|---|---|
| Security | **pass (with caveats)** | New surfaces fail-closed; env scrubbing forbids `OPENAI_API_KEY`; converge refuses LAW-0 transport. Caveats: quorum default weakening (H-1), `sh -c` trust boundary (M-1), unkeyed chain (M-2). |
| Correctness | **pass** | Quorum canonicalization + dedup logic is sound; provenance verify is in-place and byte-exact; all new tests pass. |
| Observability | **pass** | New verdicts carry structured detail (`CheckSignificantActionResult.UnmetReason`, provenance `FirstBrokenLine`+`Message`, image-health per-check `RepairHint`). |
| Readability | **pass** | New Go is well-documented (load-bearing comments explain WHY). |
| Efficiency | **pass** | No hot-path concerns in the changed code; provenance verify is single-pass streaming with a bounded scanner buffer. |
| Design | **pass (with caveats)** | Thin-wrapper backward-compat (`CheckSignificantAction` over `…Detailed`) is clean. Caveat: skills_retire ripple breadth (M-3); doc-ref drift (M-4). |

---

## Findings

### High (should fix / track)

**H-1 — Quorum default invariant was deliberately *weakened* from "≥2 families" to "≥2 contexts"; cross-family is now opt-in.**
`cli/internal/liveness/quorum.go` (rewrite). The old gate required `len(agents) >= 2 && len(families) >= 2` by default. The new gate requires only `>=2 distinct non-author contexts`; cross-family is demoted to `RequireCrossFamily` (default **off**). This is intentional and documented ("the context, not the model, is what makes a judge independent"), and it is *more* robust against same-string-forgery (see the canonicalization, which is excellent). **But the default no longer guarantees model diversity at significant-action gates** — two fresh contexts of the *same* model now satisfy the floor. This contradicts the long-standing memory `cost-law: quorum at gates` / `quorum gate exists` doctrine ("≥2 model families at one-way doors"). Risk: a homogeneous-model swarm can now self-clear merge-to-main / delete via two same-family contexts.
**Action:** confirm every *binding* caller (the significant-action types: delete, merge-main, etc.) sets `RequireCrossFamily: true`. If callers rely on the default, the family floor has silently disappeared at one-way doors. Grep the callers; the gate itself is correct, the *policy default* is the exposure.

### Medium

**M-1 — `codex` dispatch executes packet-declared commands via `sh -c` (trust-boundary assumption).**
`cli/cmd/ao/codex.go:1674` `exec.CommandContext(ctx, "sh", "-c", command)` runs each `packet.Evidence.RequiredCommands` entry through a shell. This is by design for a local dispatch executor and the env is scrubbed (`codexDispatchEnv`) + bounded by a timeout, so it is acceptable **only while task packets are an operator-trusted local artifact**. If packets ever become attacker-influenced (e.g. read from a network handoff, a PR, or `.agents/mto-handoff`), this is arbitrary command execution.
**Action:** document the trust boundary at the call site; if packet provenance ever widens, switch to argv exec or an allowlist.

**M-2 — Provenance chain is tamper-*evident*, not tamper-*proof* (unkeyed SHA-256).**
`cli/internal/provenancegraph/edge.go` — `hash = sha256(payload_hash + "\n" + prev_hash)`, no HMAC/signature/key. The commit message and gate name it "tamper-evident", which is accurate, but an adversary with write access to `docs/provenance/ledger.jsonl` can recompute a clean chain after editing a field. It defends against *accidental* edits and naive tampering, not a motivated forger.
**Action:** none required if "tamper-evident vs git history" is the intended bar (the ledger is git-committed, so git is the real anchor). Do **not** market it as tamper-proof. Consider documenting that git history + the gate is the actual integrity boundary.

**M-3 — `ao skills retire` mutates ~6 hand-correlated surfaces in one run; ordering is load-bearing.**
`cli/cmd/ao/skills_retire.go` removes trees, flips `skill-dispositions.yaml`, then regenerates (domain-map → registry → context-map, in a fixed order documented at line 27). This is the right design (matches memory `skill-retire-needs-a-tool-not-a-swarm`), and it fails-closed on phantom slugs (line 214). Risk is residual: a regen-script failure mid-run can leave the ledger flipped but inventories stale. Tests exist, but this is a high-blast-radius mutator wielded heavily this window (105→66→target-77 churn).
**Action:** confirm `retireSkill` is transactional or at least reports partial-failure state clearly; verify `regen-all --check` is run post-retire in the gate.

**M-4 — Dangling doc references to fully-retired skills.**
`bug-hunt`, `vibe`, `reverse-engineer-rpi` SKILL.md are gone, but stale `skills/<name>/` references remain in `docs/context-lifecycle.md`, `docs/comparisons/competition-rpi-memory-pipelines.md`, `docs/releases/v2.39.0-claims/…`, and `skills/validate/references/quick-mode-vibe.md`. The new `check-doc-skill-refs` validator landed this window, so these are either allowlisted or in tolerated paths (releases/comparisons archive). Low-severity hygiene drift.
**Action:** sweep dead-skill references or confirm the allowlist intentionally covers archival dirs.

### Low

**L-1 — Declared-vs-on-disk skill count is 77 declared / 73 dirs.** Expected (dispositions ledger includes historical rows), but worth a one-line assertion in the count SSOT test (`tests/docs/test-skill-count-ssot.sh`, added this window) that the *active* count, not the row count, is what's gated.

**L-2 — 24 merge commits / heavy rebase-splice activity (the `ag-xwjlc` seams epic).** Multiple "rebase-N splice repair / re-register twins / regen" commits indicate the seams epic fought the shared hot checkout. No defect found, but splice-repair commits are a known correctness-risk smell (clobbered upstream files were restored per the messages). Spot-check that no upstream doc/marker was lost net-net.

**L-3 — Committed prior-audit content carries SPOF/SPOF-down language into the tree.** The 2026-06-11 audit (bd/Dolt circuit-breaker-open, single-host SPOF) is now committed under `docs/audits/`. That is *documentation of removed/known* risk, not live risk — bd/Dolt is retired per CLAUDE.md. No reintroduced live `bd`/`dolt` call sites were found in the diff (all hits are docs/comments/disposition ledger).

**L-4 — `converge` uses `os.Getenv("CLAUDE_SESSION_ID")` / `CODEX_THREAD_ID` for context identity.** Sound, but env-sourced context IDs feed the quorum independence axis (H-1). If these env vars are unset or shared across panes, distinct judges could collapse to one context (fail-closed → NeedsAdmission, so it errs safe). Confirm panes export distinct values.

---

## Risk removed by this window (credit where due)

- **Dead skill surface** (bug-hunt/vibe/reverse-engineer-rpi/gh-actions + tail) removed → smaller attack/maintenance surface, fewer drifting references.
- **Stale codex twins** torn down; codex dispatch now has a **read-only image-health checker** that asserts twin completeness + hash sync and **must not mutate lifecycle state** (guarded).
- **`OPENAI_API_KEY` forbidden** in dispatch env unconditionally — packets cannot opt back into API-key auth (LAW-0-adjacent hardening).
- **converge refuses headless `claude --print` transport** explicitly (LAW 0 enforced in code, not just docs).
- **Quorum forgery resistance** via `CanonicalizeContextID` (whitespace/case/NBSP/tab collapse) — genuinely closes a "fake distinct contexts" hole.
- **Provenance gate** makes ledger tampering *visible* with exact line+reason.

---

## Method & limits

- Constrained strictly to `git diff/log v3.1.0..HEAD`; unchanged code was **not** audited.
- Read the new Go surfaces in full where security-relevant (quorum, provenance, codex dispatch env, skills_retire ordering); sampled the large doc/plan/registry churn rather than line-reading 1602 files.
- Verified build + vet + targeted tests (read-only). Did **not** run: full `validate.yml`, `golangci-lint`/gosec, dependency CVE scan, or runtime behavior of the dispatch executor against a real codex image.
- Strictly read-only on the codebase; only outputs are this report + the result record.
