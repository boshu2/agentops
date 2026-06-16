# AgentOps — Release-Window Briefing: `v3.1.0..HEAD`

**Mode:** briefing (architecture/onboarding orientation, scoped to one release window)
**Date:** 2026-06-14
**Range:** `v3.1.0` (`c98836977`, tagged 2026-06-10) → `HEAD` (`ab6039808`, 2026-06-14)
**Scale:** 147 commits · 1,602 files · +37,404 / −109,471 (large net deletion)
**Method:** `git log/diff v3.1.0..HEAD` only. Unchanged code was NOT audited. This is a *what-changed-and-why* orientation, not a whole-repo review.

> **READ-ONLY audit.** No source/docs/skills were modified. The only writes are this report and its result record. Build verified (`cd cli && go build ./...` → exit 0) and the new Go surfaces tested green (`go test ./cmd/ao -run 'Converge|Provenance|SkillsRetire|CodexDispatch'` → ok).

---

## 1. One-paragraph orientation

This window is **not a feature release** — it is two simultaneous, opposite-signed moves. (1) A **mass corpus contraction**: 100 skills retired and 8 added (net −92), mirrored in the codex twins (102 deleted / 8 added), which is the entire −109k line story. The repo's identity as "a giant skill library" is being deliberately cut down to a product surface. (2) A **focused capability hardening** in the Go CLI: +7,351 / −509, all additive, heavily tested, concentrated in three new doctrines — **context-quorum** (the independence axis for the no-self-grade gate flipped from *model-family* to *fresh-context*), **codex dispatch security hardening** (auth-env rejection, path-bounding, runtime schema enforcement, dispatch receipts), and two new orchestration primitives (`ao converge`, `ao provenance verify`). Underneath both runs a **tracker migration** (bd/Dolt → br+bv, breaking) and a **doctrine reframing** (the repo gained a *destination*: the "navigator / GPS-for-agentic-work" goal and the AgentOps↔Mount Olympus "two-factory machine"). There is **no new CHANGELOG version entry** — this is unreleased post-3.1 work staged on `main`.

---

## 2. What changed, by surface (where to look)

| Surface | Net | Character | Where |
|---|---|---|---|
| `skills/` | −92 SKILL.md (100 retired, 8 new) | Corpus contraction | deletions dominate; survivors = 73 skills on disk |
| `skills-codex/` | −94 SKILL.md (102 retired, 8 new) | Twin contraction (mirrors `skills/`) | parallel to above |
| `cli/` | +7,351 / −509 (all additive) | Capability hardening | `cmd/ao/{codex,converge,provenance_verify,skills_retire}.go`, `internal/liveness/quorum.go`, `internal/provenancegraph/` |
| `docs/` | +new plans/learnings/audits/contracts | Doctrine + archaeology | `docs/contracts/codex-*`, `docs/plans/bdd-foundry/`, `docs/audits/codebase-skills-2026-06-11/`, `docs/learnings/2026-06-12-*` |
| `tests/` | +9 new `.bats` + Go tests | Gate coverage for new surfaces | `tests/scripts/{check-workflow-governance,resolve-skill-path,validate-provenance-ledger,...}.bats` |
| `GOALS.md` / `PRODUCT.md` | reframed | Destination set | navigator goal, two-factory machine |

---

## 3. The four load-bearing changes (why they happened)

### 3.1 Context-quorum: independence axis flipped from model-family → fresh-context
`cli/internal/liveness/quorum.go` (commits `7e64b9299`, `afb3535dc`). The significant-action gate previously required ≥2 distinct **model families** to approve. It now requires ≥2 distinct **non-author CONTEXTS**, with model-family demoted to an *optional* `RequireCrossFamily` strengthener.
- **Why:** the producing model is now permitted to judge its own work *from a fresh context* — the doctrine claim is that **context, not model, is what makes a judge independent**. This widens the set of valid quorum participants (it lifts the previous "a quorum of one model family fails" constraint while keeping author exclusion).
- **New machinery:** `CanonicalizeContextID` (lowercase + whitespace-collapse incl. NBSP/tab) so an agent cannot forge "distinct" contexts via cosmetic string variants; legacy empty-`ContextID` ACKs are **fail-closed** (not counted); belt-and-suspenders author exclusion by `AgentID` even when `ActorContextID` is unknown.
- **Risk note:** this is a *doctrinal loosening* of the cross-model rule that contradicts the standing memory `cost-law: quorum at gates` / `control-plane must be HA` (which framed cross-model families as the consensus floor). The new floor trusts "fresh context of the same model" as independent. Whether a same-model fresh context is genuinely independent is an unproven assumption — it is asserted in the godoc, not validated. Flag for human review.

### 3.2 Codex dispatch security hardening (`cli/cmd/ao/codex.go` +1,296, plus 5 new test files +1,828)
Series `ag-p273x.1–.5`, `ag-7ixm9`, `ag-1sibx`. Genuinely security-meaningful, well-tested:
- **`.1` auth-env rejection** (`5c196ea64`): rejects forbidden auth env in *both* ambient env and packet `execution.environment`; `OPENAI_API_KEY` is *always* forbidden regardless of packet self-guards — a packet cannot opt back into API-key auth. Enforces ChatGPT-subscription-only dispatch (LAW-0-adjacent).
- **`.4` path-bounding** (`3f5b39ffd`): dispatch paths bounded to cwd + `allowed_paths` roots.
- **`.3` runtime schema enforcement** (`225e6203e`): task-packet and run-receipt validated against JSON Schema at runtime.
- **`.2` receipt execution** (`ee70d603e`): `required_commands` actually executed and recorded in the receipt.
- **image-health doctor** with per-check timeouts (`cf8084d22`) and `WaitDelay` so grandchild pipe holds can't block past budget (`25d9d1c85`, learning `docs/learnings/2026-06-12-go-exec-waitdelay-grandchild-pipes.md`).
- **Assessment:** this is the strongest, lowest-risk work in the window — additive, contract-backed (`docs/contracts/codex-task-packet.md`, `codex-fanout-approval-packet.md`), fail-closed, with a dedicated runtime-review learning (`docs/learnings/2026-06-12-codex-runtime-review-auth-and-scope.md`).

### 3.3 New orchestration primitives: `ao converge` + `ao provenance verify`
- **`ao converge`** (`16a67c1d4`, +353 / canary `+87` / test `+159`): a bounded judge-panel fix→re-run loop. Notable disciplines baked in: `UsesClaudePrint` hard-wired `false` in *every* transport branch (LAW 0), 3-consecutive-fail BLOCK, fail-closed on empty verdicts (never a vacuous PASS), `.agents/rpi/KILL` switch, author≠validator. A **two-sided canary entry gate** (`10b93c5d8`) proves the gate *bites* before trusting a PASS.
- **`ao provenance verify`** (`479891017`): tamper-evident SDLC provenance ledger (`docs/provenance/ledger.jsonl`) + `internal/provenancegraph/verify.go` (+120 / test +208). Makes the append-only ledger the source of truth with a verify gate.
- Both are composed from existing Go, well-tested, additive.

### 3.4 Tracker migration: bd/Dolt → br+bv (BREAKING)
`45ccff436` (`feat(tracker)!`), `2f07a9b8c`, `a14b52154`. The canonical tracker flipped to **br (beads_rust) + bv**; bd/Dolt **retired** as a single-host SPOF with no offline lane (cited P1 finding from `docs/audits/codebase-skills-2026-06-11/codebase-risk-audit.md`). Migration was loss-free: 3,988 issues / 4,301 deps (`.agents/swarm/results/br-migration.json`). The commit is deliberately **atomic** (doc flip + gate `19b` inversion in one commit so a single revert restores both). Ledger is now a private nested repo (`_beads/`, `BEADS_DIR` required). This matches and confirms the standing memories `agentops-br-private-ledger` and `track-work-in-control-plane`.

---

## 4. The corpus contraction (the −109k story)

100 skills retired in `skills/`, mirrored 102 in `skills-codex/`. Two mechanisms, both visible in the log:
1. **Phase-2 fold-in / prune** (`ag-s43tg`, `ag-if7p`, `ag-pj51`): 39 sources folded into 21 targets (`94db74318`, 105→65), then 52 non-product skills retired (`5e4f7e58a` "trim the herd"). Trigger phrases from absorbed skills were grafted into survivors (the long `feat(skills): graft … into …` run) so capability routing survives the deletion. This is exactly the pattern in memory `skill-retire-needs-a-tool-not-a-swarm`.
2. **A tool was built for it:** `ao skills retire <slug> --into <target>` (`e936314e9`, +655 / test +601) — deterministic retire that retargets the ~15 hand-maintained validator/registry surfaces. This is the right shape (the memory says removal ripples and needs a tool, not a swarm).

**8 new skills** mark the new direction: `converge`, `pre-land-refuters`, `continuity-loop`, `account-rotation` (unified host-routed, retiring `caam`), `codex-approval`, `operationalize`, `reality-check`, `toil-mining`. Net: the corpus is being pointed at the *navigator/quorum/convergence* doctrine and away from the sprawling Jeff-adapter / language-specific / one-off audit skills.

Survivors on disk: **73 skills** (declared count tracked to 71 mid-window in `docs/contracts`).

---

## 5. Doctrine reframing (the "destination")

Three docs commits set a *destination* the repo previously lacked:
- **Navigator goal** (`b0b7ff1c2`, `abed16a64`): "the repo had a scope, not a destination" — GPS-for-agentic-work navigator doctrine threaded into `PRODUCT.md`/`GOALS.md`. This is the `USER.md`/`athena.md` "Navi" elixir made into a product goal.
- **Two-factory machine** (`93c2e8f59`, `7618528da`, `ab6039808`): AgentOps (worker-intelligence factory) ↔ Mount Olympus (binding-verdict gate factory) as separated duties — AgentOps standalone, MTO an optional extension. Confirms memory `Olympus = olympusd, the required-reviewers gate` and the separation-of-duties split.

---

## 6. Patterns in the changes (archaeology / health signals)

| Signal | Count | Read |
|---|---|---|
| Breaking-change commits (`)!:`) | 4 | tracker flip + two skill-prune phases — all intentional, all documented |
| Rebase/splice-repair/reconcile commits | 11 | **The cost of the `ag-xwjlc` "seams" epic** — landed across a contended `main` with repeated rebase-splice damage to marker JSONs / codex twins / registry, each needing repair. This is the dominant *churn* in the window and the clearest process smell. |
| Merge commits | 24 | includes a long `aug/*` branch-merge run (the skill-graft branches) |
| New `.bats` gates | 9 | every new surface (provenance, workflow-governance, skill-path-resolution, scenario-coverage, disposition-schema) shipped with a gate — good discipline |

**Notable archaeology:** the window *contains its own recon* — `docs/audits/codebase-skills-2026-06-11/` is a full 6-report codebase-* sweep landed mid-window, and its P1 risk finding (bd/Dolt SPOF) is the *cited justification* for the tracker migration that followed. The release also references a `inner-loop-reset-20260610` and `skill-usage-evidence-20260610` audit as the empirical basis for the corpus cull. This is the knowledge-flywheel working: audit → finding → migration, with provenance.

---

## 7. Risk introduced / removed

**Removed (net positive):**
- bd/Dolt single-host SPOF retired (control-plane availability risk down).
- Codex dispatch attack surface narrowed (auth-env injection, path traversal, vacuous-pass — all closed, tested).
- Corpus complexity (~92 skills + their twin maintenance surfaces) removed via a deterministic tool, not hand-edits.

**Introduced (watch):**
1. **Context-quorum loosening (§3.1)** — the independence floor now accepts same-model fresh contexts. The *assertion* that fresh context = independent judge is undefended by evidence and softens the prior cross-family rule that several standing memories treat as load-bearing. **Highest-judgment item; route to human / council.**
2. **Rebase-churn fragility (§6)** — 11 repair commits show the seams epic repeatedly corrupted generated marker/registry JSONs on rebase. The generated-artifact reconciliation is brittle under concurrent `main`; expect more of this until the seams engine lands.
3. **Unreleased state** — substantial breaking + doctrinal change sits on `main` with **no CHANGELOG version entry** and no 3.2 tag. Anyone installing from `main` gets post-3.1 behavior that isn't versioned or release-noted. The only CHANGELOG edit in-window is a cosmetic URL rewrite of the existing 3.1.0 block.

---

## 8. Verification performed (read-only)

- `cd cli && go build ./...` → **exit 0**.
- `go test ./cmd/ao -run 'TestConverge|TestProvenance|TestSkillsRetire|TestCodexDispatch' -count=1` → **ok** (5.6s).
- Diff topology, commit log, and per-file history cross-checked against the stated scope. New CLI surfaces confirmed additive (no Go files deleted in `cli/`).

---

## 9. Onboarding pointers (if you join now)

- **Tracker:** br, not bd. `BEADS_DIR=$PWD/_beads br <cmd>`; ledger is the private `boshu2/agentops-beads` nested repo — never `git add _beads`.
- **Quorum gate:** `cli/internal/liveness/quorum.go` — read `CheckSignificantActionDetailed` godoc; the context-axis is the new contract.
- **Codex dispatch:** `cli/cmd/ao/codex.go` + `docs/contracts/codex-task-packet.md` — auth is ChatGPT-sub-only, packets are schema-validated at runtime.
- **New loops:** `ao converge`, `ao provenance verify`, `ao skills retire --into`.
- **The why:** `docs/audits/codebase-skills-2026-06-11/codebase-risk-audit.md` (the cull/migration justification) and the navigator/two-factory sections of `GOALS.md`/`PRODUCT.md` (the destination).
