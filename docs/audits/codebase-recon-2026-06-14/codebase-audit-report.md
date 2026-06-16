# Codebase Audit Report — agentops, v3.1.0..HEAD release window

**Date:** 2026-06-14
**Skill:** `review` (report mode — release-window archaeology + risk)
**Verdict:** COMMENT (healthy release; no blocking findings introduced; one new tactical risk + several pre-existing P1s un-addressed)
**Target:** `git diff v3.1.0..HEAD` — `v3.1.0` = `c98836977` (2026-06-10) → `HEAD` = `ab6039808` (2026-06-14)
**Scale:** 147 commits · 1602 files · +37,404 / −109,471 (large net deletion)

> Scope note: This report is **constrained to what changed in this window**, per the worker brief. It does NOT re-audit unchanged code. It builds on, and cross-checks against, the prior whole-repo recon at `docs/audits/codebase-skills-2026-06-11/` (which sits inside this window — its findings are tested for "addressed since?" below).

---

## Intent of the release window

Two arcs dominate this window, and they are coherent:

1. **The great skill cull + product re-scoping.** 100 skills deleted from `skills/`, 102 from `skills-codex/`, plus the gemini image skill set — net the corpus dropped from ~166/167 to **72**. This is the back half of the `ag-s43tg` prune. It is the dominant source of the −109k lines.
2. **The navigator/destination doctrine + the assurance machinery to support it.** GOALS.md, PRODUCT.md, and `docs/3.x` were rewritten around a single thesis — *AgentOps is a GPS for stochastic agentic work, not a workflow* — with a concrete destination (Directive 16: *autonomous goal → verified done*). The new Go surfaces (`ao converge`, `ao provenance`, the codex worker-as-image dispatch lane, the context-quorum gate) are the apparatus that thesis requires.

A third, smaller arc: the **AgentOps ↔ Mount Olympus separation of duties** — AgentOps declared a *complete standalone product*; MTO demoted to an *optional* one-way extension that consumes AgentOps claims and returns binding verdicts via a tracker-independent file handoff (`.agents/mto-handoff/recurrence.json`).

`INTENT:` ship the assurance + navigator floor (provenance ledger, context-quorum, bounded converge loop, schema-validated codex dispatch), shrink the corpus to its load-bearing core, and re-anchor the product on a measurable destination.

---

## SCORED Assessment (of the diff)

| Category | Rating | Notes |
|----------|--------|-------|
| Security | pass | New `jsonschema/v6` dep is mainstream; no secrets added; codex dispatch hardens auth (refuses non-subscription auth, inherit-interactive stdin, cwd-inheriting resume). Tamper-evident provenance chain added. |
| Correctness | pass | New code is fail-closed by construction (zero-verdict round ≠ vacuous PASS; empty ContextID never counts; missing ledger file = intact-empty not error). Targeted tests for liveness/converge/codex/provenance pass; `go build`/`go vet` clean. |
| Observability | pass | New surfaces emit structured results (`VerifyResult.FirstBrokenLine`, `CheckSignificantActionResult.UnmetReason`, converge round dispositions with reasons). |
| Readability | pass | New Go files carry unusually strong doc-comments stating kernel invariants and the *why*. |
| Efficiency | pass | No hot-path concerns introduced; ledger verify is a single streaming scan with a bounded buffer. |
| Design | warn | The architecture is sound, but the change *concentrates* even more surface into the already-flagged `cli/cmd/ao` single `main` package (codex.go alone +1296 lines; converge, provenance_verify, skills_retire, codex_schema all land here). Prior recon F4 (620-file/9.2MB single package) is made worse, not better. |

---

## What changed, by surface

### 1. The skill cull (corpus 166 → 72)

- `skills/`: 100 `SKILL.md` deleted, 16 added, 112 modified, 32 renamed. `skills-codex/` mirrors it (102 deleted, 21 added). `images/gemini/skills/*` and `cli/embedded/skills/using-agentops` also pruned.
- This was executed through the `ao skills retire` tool (`cli/cmd/ao/skills_retire.go`, 655 lines, **new**) rather than a hand-swarm — directly per the prior post-mortem learning ("skill retire needs a tool, not a swarm"). The tool ripples removals through the ~15 hand-maintained surfaces (registry, domain map, context map, tiers, dispositions ledger) and runs a read-only ripple scan that aborts on `UnresolvedRefs` unless `--dry-run`.
- Doctrine docs (PRODUCT.md, GOALS.md history) were updated to the new count (166→72) in the same window — the SSOT for the count was also moved to derive-from-disk (`0626f7a19`, `tests/docs/test-skill-count-ssot.sh` added) so the number can't drift by hand-edit again.

**Risk removed:** large dead-corpus carrying cost; "too many skills" maintenance tax (per the standing "skills/ are the product — cull = count" learning). **Risk introduced:** none structurally — the retire tool is dry-run-default for reporting and guards `os.RemoveAll` behind `opts.DryRun`. The cull's correctness rides on the ripple-scan + the regen drift gates, which are present.

### 2. Context-quorum: the independence axis moved from *model family* to *fresh context*

`cli/internal/liveness/quorum.go` (+135/−27) is the most architecturally significant code change in the window.

- **Old rule:** a significant action needed ≥2 distinct non-actor ACKs spanning ≥2 model families.
- **New rule (`CheckSignificantActionDetailed`):** ≥2 distinct **non-author CONTEXTS** that APPROVE; model family is demoted to an **optional** `RequireCrossFamily` strengthener (off by default). New `CanonicalizeContextID` collapses whitespace/case/unicode variants so one agent can't forge "distinct" contexts cosmetically.
- **Fail-closed details that are correct:** empty (or empty-after-canon) ContextID is never counted (legacy ACKs predating the axis can't be proven independent); belt-and-suspenders author exclusion by `AgentID` even when `ActorContextID` is unknown; `ActorID=="" && ActorContextID==""` → Denied with `"cannot identify author context"`.

This is a real doctrine reversal vs the standing memory ("cross-model quorum = control-plane consensus") and is *intentional*: the producing model may now judge its own work **from a fresh context**. The rationale is in PRODUCT.md ("the context, not the model, is what makes a judge independent"). Tests were flipped accordingly (`a860589f9` flips the family-floor test to context-floor; `quorum_canon_test.go` added for the canonicalization). **Watch item:** anything downstream still assuming the family-floor (e.g. external olympusd, briefs, or docs in other repos) now disagrees with the code. Within this repo the flip is consistent.

### 3. `ao converge` — bounded judge-panel convergence loop (new, `ag-slice2..4`)

`converge.go` (353) + `converge_canary.go` (87). A bounded fix→re-run-judge-panel loop to terminal agreement or a 3-consecutive-fail BLOCK. Notable, well-built patterns:

- **Two-sided canary entry gate** (`convergeRunCanary`): before any judge dispatch it feeds the gate a planted *known-bad* (self-judge) AND a planted *known-good* (independent-context) fixture and only proceeds if the gate rejects the bad AND accepts the good — catching both a gate that bites nothing and a degenerate all-reject gate. This is the "prove the gate bites before trusting a PASS" discipline encoded in code. Strong.
- Fail-closed round evaluation: a round with zero usable verdicts is never a vacuous PASS.
- LAW 0 respected by construction: every transport branch keeps `UsesClaudePrint=false`; the Claude→Codex leg uses Go headless `ao codex dispatch` (Codex Pro sub), the Codex→Claude leg is delegated to an NTM/codex-approval pane.

### 4. SDLC provenance ledger (new, `ag-8jf97`, landed `479891017`)

- `cli/internal/provenancegraph/verify.go` (120) + `provenance_verify.go` + ledger now **exists** at `docs/provenance/ledger.jsonl`.
- `VerifyFile` checks the committed bytes in place: per-record field validity + intact hash chain (prev_hash links, payload_hash/hash recompute), naming the first broken **file line**. Missing file = intact-empty (a fresh clone doesn't fail the gate). Tamper of a field, a forged hash, or a reordered row is caught.
- **This directly resolves prior recon F6** ("`docs/provenance/ledger.jsonl` does not exist while CLAUDE.md declares it the SOT"). Now it exists, is tamper-evident, and has a gate.

### 5. Codex worker-as-image dispatch lane (new, large)

`codex.go` +1296, with `codex_schema.go` (116), embedded `codex-task-packet.schema.json` + `codex-run-receipt.schema.json`, and 3 new test files (~1450 lines of tests). Schema-validates packets and receipts (incl. `additionalProperties`, auth constants, enums) before/after dispatch, so a dispatch cannot persist a receipt that violates its own contract. The dispatch path has tight refusal guards: refuses inherit-interactive stdin, refuses `resume.policy=last-session-in-cwd`, refuses non-ChatGPT-subscription auth, and the judge leg is non-mutating (`MutatesRepo`/`mode` checked). `image-health` is read-only and aborts if lifecycle state changes mid-check. This is the heaviest single-file growth in the window and is well-tested.

### 6. Workflows as first-class contracts + the gate

- `ship-beads` workflow shipped (additive rename of `bead-crank`), `bdd-foundry` canonicalized into `.claude/workflows`, plus a bidirectional `.js`↔ledger drift gate (`check-workflow-governance.sh`) and a new always-run blocking `workflow.install-drift` gate (`cli/internal/gates/checks/workflow_install.go`).
- `errcheck` lint rule added + unchecked `json.Marshal/Unmarshal` returns fixed across `cli/` (`agentops-tqc.3`).

---

## Risk ledger for this window

### New / increased risk

- **[Design / cli/cmd/ao] Surface concentration worsened (was prior F4, P2 → still P2, trending worse).** This window added `converge.go`, `converge_canary.go`, `provenance_verify.go`, `skills_retire.go`, `codex_schema.go`, and +1296 lines to `codex.go` — all into the single `cli/cmd/ao` `main` package the prior recon already flagged as a 620-file/9.2MB coupling hazard that blocks deletions. Change-risk continues to pool in one namespace. *Smallest credible step: peel cohesive new lanes (converge, provenance, codex-dispatch) into their own packages or subcommand packages as they stabilize.*
- **[Doctrine drift surface] The family-floor → context-floor flip is local-consistent but cross-surface risky.** Standing fleet memory and any external consumer (olympusd, other-repo briefs) still encode "≥2 model families." The code is now "≥2 contexts, family optional." Until those external surfaces are reconciled, an orchestrator citing the old rule will mis-state the gate. *In-repo: fine. Cross-repo: a reconciliation item.*

### Pre-existing P1s from 2026-06-11 recon — addressed status (all inside this window)

- **F6 (provenance ledger missing) — RESOLVED.** Ledger now exists + tamper-evident verify + gate (§4).
- **F1 (bd/Dolt remote single-host SPOF) — PARTIALLY ADDRESSED in doctrine, not yet structurally.** Tracking migrated to **`br` at `_beads/`** (private nested repo `boshu2/agentops-beads`, git-JSONL, no server) per CLAUDE.md — this removes the server dependency for the tracker. But the legacy `.beads/` Dolt config is preserved byte-for-byte pending reconciliation, so the SPOF is *deprecated, not removed*. The window's commits cite `ag-*` beads via br; the bd circuit-breaker risk no longer blocks delivery here.
- **F2 (detached/diverged skill SSOT checkout) — not addressable by code; an operator/host state issue, out of diff scope.**
- **F3 (2,210-line bash pre-push gate is the wall, fail-open edges) — NOT addressed; trending neutral-to-worse.** `scripts/pre-push-gate.sh` is still present and now **2,252 lines** (grew ~42). The Go gate (`ao gate check`) gained checks (workflow-install, seed) and is the stated authority, but the bash monolith remains the default/escape-hatch fallback. The "12/79 native parity" gap from the prior state is the open epic, not closed in this window.

### Removed risk

- Dead-corpus carrying cost (94 skills retired via a tool with ripple-safety, not a swarm).
- Provenance SOT-vs-reality contradiction (F6).
- Unchecked `json.Marshal/Unmarshal` returns across `cli/` (errcheck lint now enforces).

---

## Notable archaeology / patterns in the changes

- **"Prove the gate bites" is now load-bearing, not advisory.** The two-sided canary in `converge_canary.go` and the codex receipt-validates-its-own-contract pattern are the same instinct as the prior fixture-fidelity learning (ag-mjlg): a green that can't be planted-positive-rejected is treated as a lie. This is the healthiest pattern in the window.
- **Fail-closed is the house default.** Every new decision path (quorum empty-context, converge zero-verdict round, ledger missing-vs-tampered, dispatch auth/stdin refusals) defaults to deny/strict. No fail-open paths introduced.
- **Doctrine and code shipped together.** The navigator/destination thesis (GOALS Directive 16, PRODUCT "GPS for agentic work") landed in the same window as its enabling apparatus (provenance position-signal, converge resilience, validation membrane). The docs honestly mark milestone status as *floor poured, unfed* / *recovery half is a stub* rather than overclaiming — consistent with the "report scope precisely: built ≠ migrated" learning.
- **Heavy rebase-splice churn around `ag-xwjlc` (seams epic).** ~12 `chore(codex)`/`chore(contracts)` commits are post-rebase reconciliation (twin re-registration, marker-JSON splice repair, regen). Normal for a contended hot repo landing a multi-wave epic, but it inflates the commit count and is a smell that the multi-surface regen is fragile under rebase.

---

## Missing / would-have-expected

- A package split accompanying the `cli/cmd/ao` growth (the prior recon already named this; the window added to the pile).
- Reconciliation of the family-floor → context-floor flip in cross-repo/external-consumer surfaces (in-repo is done; the broader fleet memory still says "families").
- Final retirement of legacy `.beads/` (bd/Dolt) — still preserved alongside the new `br` tracker, leaving the deprecated SPOF physically present.

---

## Verification performed (read-only)

- `cd cli && go build ./...` → clean. `go vet ./...` → clean.
- `go test ./internal/liveness ./internal/provenancegraph ./cmd/ao -run 'Converge|Quorum|Provenance|CodexSchema|SkillsRetire'` → all `ok`.
- `git diff --stat / --name-status v3.1.0..HEAD`, per-surface `git log`, and targeted reads of the new Go files (`quorum.go`, `converge*.go`, `provenancegraph/verify.go`, `codex_schema.go`, `skills_retire.go`, `codex.go` guards).
- Cross-checked prior recon `docs/audits/codebase-skills-2026-06-11/codebase-risk-audit.md` P1–P3 findings for addressed-status within the window.
