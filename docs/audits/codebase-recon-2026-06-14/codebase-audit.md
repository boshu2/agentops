# Codebase Audit — Release Window v3.1.0..HEAD

**Date:** 2026-06-14  |  **Verdict:** COMMENT (ship-quality release; one doc-drift defect worth fixing)
**Target:** `git diff v3.1.0..HEAD` — tag v3.1.0 (`c98836977`, 2026-06-10) → HEAD (`ab6039808`)
**Scope:** Diff-only. Unchanged code was NOT audited.
**Scale:** 147 commits · 1602 files · +37,404 / −109,471 (large net deletion).
**Method:** `skills/review/SKILL.md` SCORED pass, constrained to the diff per the orchestrator scope.

## Intent

Three concurrent arcs landed in this window, all consistent with the post-3.1 doctrine:

1. **Skill-corpus prune** (ag-s43tg / ag-if7p / ag-pj51) — cull the corpus from ~105 to ~71
   product skills: 102 `SKILL.md` deleted, 8 added, the rest folded (trigger-phrase grafts)
   into surviving targets. Codex twins (`skills-codex/`) follow: −30,079 lines. This is the
   bulk of the net deletion.
2. **Codex runtime hardening** (ag-p273x, ag-codex-runtime-enhancement-o0nds) — +1,295 LOC in
   `cli/cmd/ao/codex.go` plus new schema/dispatch/image-health surfaces: bounded dispatch,
   packet-injected-auth rejection, JSON-Schema enforcement at runtime, run-receipts.
3. **Verification/governance spine** — `ao converge` (bounded judge-panel loop), `ao provenance`
   (tamper-evident SDLC ledger), `ao skills retire` (deterministic prune operation),
   context-quorum (family-floor → fresh-context floor), workflows-as-first-class + drift gate,
   and bd/Dolt → br tracker retirement.

A late docs arc closes the window: the navigator/GPS-for-agentic-work doctrine and the
AgentOps↔Mount Olympus "two-factory machine" framing (separation of duties).

## Build & Test Status (diff surfaces only)

| Check | Result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| New codex/converge/provenance/retire tests (`cmd/ao`) | PASS (7.7s) |
| `internal/provenancegraph`, `internal/liveness`, `internal/gates/...` | PASS |
| `scripts/regen-all.sh --check` (drift sweep) | ALL GATES GREEN |

The release is internally consistent: every generated registry/context-map/codex-parity surface
is in sync, and the new Go carries real test suites (not coverage padding).

## SCORED Assessment

| Category | Rating | Notes |
|----------|--------|-------|
| Security | **pass** | New codex runtime *adds* defense-in-depth; primitives reviewed correct (below). |
| Correctness | pass | All new tests green; regen drift clean; tamper-evident provenance chain sound. |
| Observability | pass | Codex run-receipts, gate "explain selected/skipped checks", slow-check reporting. |
| Readability | pass | New Go is well-factored, godoc'd, deterministic-ordering comments where it matters. |
| Efficiency | pass | Per-check timeouts + WaitDelay on image-health prevent grandchild-pipe budget overrun. |
| Design | **warn** | Prune left dangling skill refs in the doctrine spine; the detector for it isn't wired. |

## Findings

### Critical
None.

### Warning (should fix)

- **[docs/architecture/operating-loop.md:51,90,99]** Dangling skill references introduced by the
  prune. The doc — the repo's declared *primary navigation* / 7-move doctrine spine — still
  points at `/design`, `/validation`, `/vibe`, `/harvest`, `/ratchet`, `/retro`, all of which were
  folded or cut in this window (verified: none resolve to a `skills/<dir>`). The window's own new
  detector, `scripts/check-doc-skill-refs.sh`, flags exactly these 7 refs:
  `3 doc(s) scanned, 7 unresolved skill reference(s)`. **The gate is not wired into CI**
  (`grep check-doc-skill-refs .github/workflows/ scripts/regen-all.sh Makefile` → empty), so the
  drift is live and unguarded. The doc *was* edited this window (navigator doctrine, abed16a64)
  but the stale slash-refs were not reconciled against the prune. Suggested fix: repoint each ref
  at its fold target (e.g. `/vibe`→`/validate`, `/validation`→`/validate`) or mark the line
  retired/folded, then wire `check-doc-skill-refs.sh` as a blocking gate so the detector that
  already exists actually bites — same lesson as the converge "prove the gate bites" slice.

### Suggestion / Nit

- **[scripts/*.sh — 16 files]** Residual `bd`-named scripts (`ship.sh`, `bd-audit.sh`,
  `bd-cluster.sh`, `seed-evolution-roadmap-beads.sh`, etc.) still invoke retired `bd` verbs.
  bd/Dolt was retired 2026-06-11 in favor of `br`. This is **pre-existing migration debt**, not
  introduced here (most untouched this window), but the tracker flip arc (ag-joto6) left it
  incomplete. Track as a follow-up; not a blocker.

## What Changed (archaeology)

- **The prune is the story** (−109K lines). 102 `SKILL.md` removed via the new deterministic
  `ao skills retire <slug> --into <target>`, which operates the ~15 hand-maintained ripple
  surfaces in a fixed order (counts → domain-map → registry → context-map) and flips the
  dispositions ledger non-lossily (active row → historical, never deleted). This directly
  codifies the prior pain point ("skill retire needs a tool, not a swarm"). Evidence trail is
  thorough: `evidence/skill-prune-*.md` (recon, dispositions, phase2, lane-b, token-audit).
- **Codex runtime became a hardened dispatch boundary.** `codex.go` now: bounds all dispatch
  output paths to cwd + packet `allowed_paths`; rejects `OPENAI_API_KEY` (and packet-declared
  `reject_env`) from both ambient env and packet `execution.environment`; requires ChatGPT
  subscription login-status; enforces task-packet and run-receipt JSON Schemas at runtime; and
  records command results in tamper-checkable receipts. This is LAW-0-adjacent: it mechanically
  prevents API-billed worker dispatch.
- **Verification got teeth.** `ao converge` is a *bounded* judge-panel loop (the two-sided canary
  entry gate proves the gate bites before trusting a PASS — directly answering the "quorum of one"
  failure mode). `ao provenance` adds an append-only, prev_hash/payload_hash/hash-chained SDLC
  ledger with a tamper-evident verifier. Context-quorum flipped from a family-floor to a
  fresh-context floor (independence by construction, not by vendor diversity).
- **Tracker flipped bd/Dolt → br** (ag-joto6, ag-yb8w). `_beads/` correctly gitignored as a
  PRIVATE nested repo; no bead data leaked into this public repo (verified `git ls-files`).
- **Docs grew +15.7K / −1.5K** — mostly the prune's evidence/plans plus the navigator and
  two-factory (AgentOps↔MTO) framing that gives the repo a destination, not just a scope.

## Patterns in the Changes

- **Detector-first, but not always wired-first.** Multiple changes ship a new gate alongside the
  thing it guards (workflow-install drift, json-marshal-checked, bdd-foundry-markers,
  am-coordination-discoverable). The one exception — `check-doc-skill-refs.sh` — is exactly where
  the live drift slipped through. The pattern works; the miss is the unwired instance.
- **Deterministic operations over swarms** for ripple-heavy edits (skills_retire, regen-all).
- **"Prove the gate bites"** as an explicit design value (canary entry gate, two-sided canary).
- **Heavy rebase-reconciliation tax** on the seams/bdd-foundry epic (ag-xwjlc): ~10 chore commits
  are pure post-rebase splice repair / twin re-registration / regen — a visible cost of the
  shared-checkout + generated-artifact model under concurrent lanes.

## Risk Introduced vs Removed

- **Removed:** API-key worker-billing exposure (codex auth guard); unbounded dispatch path
  writes (path bounding); unverifiable provenance (hash chain); quorum-of-one false PASS (canary
  gate + context-floor); ~30K lines of unmaintained codex-twin skill surface.
- **Introduced:** doc-drift in the doctrine spine (the one Warning); incomplete bd→br script
  migration carried forward. Both low-severity, neither blocks the release.

## Missing

- Wiring of `check-doc-skill-refs.sh` into the blocking gate set (the detector exists; it just
  doesn't run in anger).
- Completion of the bd→br script migration (16 scripts).
