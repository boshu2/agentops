# AgentOps Inner-Loop Reset — Prune Plan (2026-06-10)

> **Status:** PROPOSAL. Read-only audit; nothing in this document has been executed.
> Bo disposes. No beads were created; each wave below should become beads only on approval.
>
> **Inputs:** five parallel read-only censuses (skills corpus, `ao` CLI surface, top-level
> dirs + harness sprawl, root identity docs, regrowth forensics) plus an adversarial
> verification pass. Adversary vetoes are honored and marked **[VETO HONORED]**;
> adversary-found missed cuts are adopted and marked **[ADV+]**.
>
> **Prior art:** `REDUCTION.md` + `PRE-REDUCTION-SNAPSHOT.md` (2026-06-07 wave). Its
> KEEP/RELOCATE boundary (rpi/swarm vs cron/mcp/orchestrate → MTO) independently
> re-derives the razor — this plan extends that contract from Go files to every surface.

---

## 1. The identity statement

**AgentOps is the in-session operating loop + context compiler for coding agents.**
(README.md:7 — already written, already razor-true; every other doc derives from it.)

Three sentences under the razor:

1. AgentOps is what **one agent session** needs to produce work end-to-end: the loop
   (rpi/evolve), context compilation (corpus, inject, citations), the knowledge flywheel,
   beads-as-practice, validation discipline, and per-harness model/config images.
2. AgentOps is **not** the outer loop — orchestration, session dispatch, fleet tending,
   gating, and multi-lane coordination belong to Mount Olympus (per ADR-0009 the substrate
   is swappable and NTM is reference, not product).
3. AgentOps **adopts, never owns** external tools (ntm/atm, am, cass/cm, dcg, caam, rch,
   ubs, acfs, sbh, casr, ru, pt): thin pointers at most, no wrappers, no duplicated docs.

THE RAZOR (operator decision 2026-06-10): a thing is admitted to this repo only if it
serves **loop / context / flywheel / beads / validation / harness-config**. Tool docs go
to the tool's repo; orchestration goes to MTO; everything else is cruft.

---

## 2. The numbers

| Surface | Current | Target | Δ |
|---|---|---|---|
| `skills/` dirs | **169** | **53** surviving names (~50 after [ADV+] folds) | **−69%** |
| …of which: keep as-is | — | 28 (29 minus bootstrap fold [ADV+]) | |
| …merge survivors | — | 24 clusters absorbing 57 members | |
| …move out (MTO / tool repos) | — | 24 | |
| …delete / attic (loud tombstones) | — | 32 | |
| Derived skill copies (skills-codex 929 files, .agy-plugin ~30, images/gemini 75) | ~1,000 files | **0 checked-in derived copies** (manifest + generator + overrides-as-patches) | −~1,000 files |
| Top-level dirs (non-.git) | **40** | **~24** | −40% |
| Root identity/meta docs | **21** | **~8 standing files** | −62% |
| `ao` top-level commands | **84** | **~31** (35 keep − 4 [ADV+] folds; 9 census merges + 14 move-outs + 6 attics) | **−63%** |
| Local disk litter (dist, .venv-docs, .tmp, .x, wt-ag-qidx, stale worktrees) | ~520M + ~100 stale worktree registry entries | 0 + reaper | |

Usage-evidence baseline: of 84 `ao` commands, ~25 have **zero** skill references; the top
12 (goals 80+, lookup 41, rpi 35, ratchet 31, inject 29, metrics 28, session 22, loop 20,
knowledge 18, reconcile 18, codex 16, validate 13) carry nearly all load. Of 169 skills,
~80 share an identical machine-minted template with no usage evidence.

---

## 3. The skill prune (169 → ~50–53)

### 3.1 Keep (28)

Inner-loop core, each razor-justified:

- **rpi, evolve, implement, design, discovery, crank-adjacent loop family** — the loop itself.
- **pre-mortem, post-mortem, red-team, scenario, eval-outcomes, scope, push** — validation discipline.
- **flywheel, compile, inject, ratchet, trace, forge-family survivors** — context compilation + flywheel.
- **handoff, goals, product, domain, standards, shared, using-agentops, session-bootstrap-family survivors** — session continuity + contracts.
- **autodev, agent-native, cc-hooks, converter** — harness config (explicitly allowed by the razor).
- ~~bootstrap~~ → **[ADV+] folds into using-agentops** (four orientation surfaces — bootstrap, session-bootstrap, using-agentops, status — were surviving separately; one onboarding story).

### 3.2 Merge clusters (24 survivors absorbing 57 members)

| Survivor | Absorbs | One-line why |
|---|---|---|
| **beads** | beads-br, beads-bv, beads-workflow | Four skills for one practice; command reference + plan-conversion are sections. **[VETO HONORED]** beads-br/beads-bv are on `docs/contracts/critical-skills.txt` and named in both fleet CLAUDE.md skill tables — ships only with seal rewrite + name-preserving tombstone stubs + dotfiles edit to both tables. |
| **validate** | vibe, bead-completion-audit | Three flavors of "give a verdict"; one verdict skill with modes. |
| **council** | multi-model-triangulation | Cross-model second opinion IS multi-judge consensus. |
| **plan** | planning-workflow, brainstorm, idea-option-forge | Phases of one planning skill, not four siblings. |
| **research** | research-software, external-search-triage | Target flavor + go/no-go gate are sections. |
| **crank** | burndown | "Finite epic set to all-merged" is crank with a stop flag. |
| **ship-loop** | operating-loop-skill | Direct duplicate minted by the skill factory. |
| **forge** | curate | Both mine transcripts/.agents/git into learnings; one capture skill. |
| **session-bootstrap** | recover | Recovery is the failure path of session init. |
| **status** | quickstart | Same orientation surface. |
| **swarm** | cc-subagents, cc-worktree-isolation | In-session fan-out is the one parallel primitive inside the razor; mechanics are implementation sections. *Borderline flagged for Bo: closest call vs MTO.* |
| **codex-exec** | codex-goals, codex-mcp-plugins, codex-sandbox-evidence | One harness = one skill. |
| **agy-native** | agy-headless-evidence, agy-mcp-plugins, agy-rules-workflows, agy-sidecar-scheduled-tick*, agy-project-worktree-permissions | Six AGY skills minted in 48h. **[ADV+] stronger cut adopted: attic the whole AGY lane** with tombstone — fleet doctrine defaults to Codex + local shell; zero usage evidence; readmit on first real usage citation. (*agy-sidecar-scheduled-tick is outer-loop regardless — see forensics move-outs.) |
| **skill-builder** | skill-auditor, heal-skill, expertise-to-procedure, cross-vendor-trust-gate*, repeatedly-apply-skill | The skill factory as one skill — **which must carry THE RAZOR as its admission gate, because this factory IS the regrowth mechanism.** **[VETO HONORED]** heal-skill is on critical-skills.txt — seal + tombstone + fleet-table sweep required. (*cross-vendor-trust-gate's gating half moves to MTO per forensics.) |
| **test** | fuzz-test-design, metamorphic-test-design, golden-artifact-testing, contract-conformance-testing, live-service-e2e-testing | Five batch-minted technique pamphlets → one testing-discipline skill. |
| **bug-hunt** | layered-defect-hunt, concurrency-deadlock-remediation, native-debugger-triage | Outright duplicate + two technique sections. |
| **refactor** | complexity, behavior-preserving-simplification | Hotspot-finding and the core invariant are sections. |
| **review** | production-placeholder-audit | review already claims "find mocks, hunt placeholders". |
| **perf** | measured-performance-optimization, performance-profile-triage | Profile → triage → optimize is one workflow. |
| **security** | deps, dependency-update-safety | Dependency risk is security's lane. |
| **codebase-archaeology** | codebase-report, codebase-briefing-report, codebase-audit, codebase-risk-audit, legacy-codebase-recon, codebase-pattern-extraction, implementation-pattern-mining | EIGHT "understand a codebase and write it up" skills — worst sprawl cluster; one context-compilation skill. |
| **doc** | project-readme-craft, changelog-quality-pass | doc already generates READMEs/doc packs. |
| **release** | release-readiness-gate | The gate is release's checklist restated. |
| **pr-implement → `pr`** | pr-research, pr-prep | One upstream-PR flow with phases. |

Merge mechanics (every cluster): absorbed name gets a tombstone SKILL.md ("merged into X,
see Y") for one release cycle; cross-references inside kept skills (council, evolve,
goals, post-mortem, pre-mortem, red-team, validate reference vibe/quickstart/recover)
swept in the same wave; `skills-codex-overrides/` entries for merged-away names (vibe,
brainstorm, complexity, quickstart, recover, pr-prep, pr-research) retire with the merges.

### 3.3 Move out (24)

**[VETO HONORED — blocking precondition for every fleet-routed item below]:**
agent-mail, vibing-with-ntm, using-atm, caam, dcg (and ntm) are live symlinks from
`~/.claude/skills` into this checkout AND named routes in the skill-registry tables of
both `~/.claude/CLAUDE.md` and `~/dev/CLAUDE.md`; agent-mail is additionally on
`docs/contracts/critical-skills.txt` (the repo's own seal: a bad edit "can strand the
agent fleet"). Hard-ordered migration gate, per skill:
**new repo serves the skill → `link-skill` repoints → critical-skills.txt updated →
tombstone lands → only then delete.** Same-name preservation keeps the CLAUDE.md tables
valid until the dotfiles edit lands.

To the tool's own repo (Jeff-tool territory, adopt-not-own):

| Skill | Destination | Why |
|---|---|---|
| ntm, vibing-with-ntm, using-atm, ntm-browser-test-coordination, ntm-review-worker-orchestration | `~/dev/ntm` | NTM/ATM swarm orchestration + fleet tending = the tool's surface AND the outer loop. |
| agent-mail | `~/dev/mcp_agent_mail_rust` | Jeff tool; multi-lane coordination is outer-loop besides. Critical-skill — full migration gate above. |
| cass / cass-memory | cass / cm repos | Jeff tools; cm is additionally LAW-0 non-compliant (provider:cli) — AgentOps must not own its skill. |
| dcg, caam, rch, ubs, casr, process-triage, gh-triage-ru, ru-multi-repo-workflow | each tool's repo | Tool operating docs ride with the tool; pointer at most. |
| sbh | Dicklesworthstone repo pointer | Confirmed external tool; disk ballast is not the inner loop. |
| acfs, storage-watchdog-ops | `~/acfs` / its source | Jeff's flywheel tooling; overlaps AgentOps' flywheel confusingly. storage-watchdog-ops rides with acfs (forensics: also outer-loop fleet ops). |

To Mount Olympus (outer loop):

| Skill | Why |
|---|---|
| cc-loop-driver | Control-plane tick loop with workers + separate validators is the factory loop verbatim. |
| cc-cron-ticks | Scheduled autonomous dispatch = outer-loop scheduling. |
| operating-loop-workflow | Self-describes as multi-agent orchestration Workflow install. **[VETO HONORED]** on critical-skills.txt — MTO must actually serve it before the seal is released. |
| workflow-builder | Deterministic multi-agent orchestration scaffolding — MTO owns orchestration shapes. |
| automation-shape-routing | A router over outer-loop substrates; the inner loop never needs it. |
| agy-sidecar-scheduled-tick, cross-vendor-trust-gate (gating half) | Forensics razor-fails: scheduled sidecar ticks + skill-factory gating = outer loop. |

### 3.4 Delete / attic (32 — every one gets a loud tombstone, never a silent rm)

Grouped, one-line each:

- **Generic-knowledge noise** (models already know it; the flat-list pain Bo named):
  gcloud, gh-cli, gh-actions (release bit folds into release), ssh, ripgrep-search-discipline.
- **Batch-minted pamphlets, no usage evidence:** artifact-clarity-pass,
  automation-loop-hardening, cli-agent-ux-audit, cli-doctoring-workflow,
  filesystem-path-rationalization, installer-quality-audit, mcp-interface-design,
  project-reality-check, project-reasoning-lens-analysis, scaffold,
  spec-reliability-implementation, work-contract-portability, repeatedly-apply-skill.
- **Rust batch duplicating fleet-level skills:** rust-crate-release-readiness,
  rust-port-validation-gauntlet, rust-search-integration, rust-sqlite-cli-architecture,
  rust-ub-risk-audit, rust-unsafe-boundary-audit.
- **Git-janitor siblings:** repository-hygiene-sweep, stash-hygiene-sweep,
  worktree-branch-rationalization (swarm's worktree section covers the loop's need).
- **One-shot migrations, already executed:** bd-first-memory-migration,
  bead-tracker-migration — tombstone loudly as completed.
- **Verbatim duplicate pair (the clearest proof the admission gate is missing):**
  system-performance-remediation + system-tuning — attic both; dotfiles/ops territory if needed.
- **Stale experiment:** reverse-engineer-rpi (research + design already do its job).

**Resurrection watch [VETO/ADV+ — extended]:** the published plugin bundle
(`~/.claude/plugins/cache/agentops-marketplace/agentops/3.0.1`) still serves skills
pruned 06-07 (security-suite is live in the harness list right now). Every prune is
silently reversed at the fleet edge until a **plugin/marketplace re-release + cache
flush** ships. Extend the watch from the prior 13 pruned names to **all ~110 names cut
in this wave**.

**Catalog surfaces:** catalog.json regenerates via `scripts/generate-skill-catalog.sh`.
**[ADV+]** SKILL-TIERS.md (43K) is not rewritten — it is **deleted** and tier metadata
generated into catalog.json; it already drifted (references codex-team and retro, pruned
06-07), proving hand-curated taxonomy decays.

---

## 4. Dirs + harness sprawl (40 → ~24)

### 4.1 The one structural move that must land FIRST

**One source + per-harness generator.** (Adversary verdict: the strongest item in the
plan, and the precondition that makes the prune affordable.)

**[VETO HONORED]** Census 1's premise that `skills-codex/` is generated is **wrong**:
`scripts/regen-all.sh:16` says twins "are MANUAL — this refreshes their hashes but cannot
author a missing twin", and every `.agentops-generated.json` says
`"generator": "manual-maintained"`. A ~110-skill prune executed before fixing this pays
the manual-twin + parity-gate tax on every cut and strands `audit-codex-parity.sh` /
`regen-codex-hashes.sh --check` mid-wave. **Therefore: generator convergence is Wave 1,
the prune is Wave 3.**

Converge on the `images/claude` pattern (manifest.json + README + verify.sh, explicitly
"does NOT copy or duplicate the skill files"):

- `skills/` = the only source.
- `skills-codex/` (929 manual files), `.agy-plugin/skills/` (~30 verbatim copies),
  `images/gemini/skills/` (75 transformed copies) → replaced by manifest + transform
  recipe per harness, generated by an extended `scripts/regen-all.sh`.
- `skills-codex-overrides/` stays as the **only** hand-written per-harness layer
  (overrides-as-patches — it is already the right shape).
- CI gates flip direction: from "the copies exist and match" to "the generator output is
  current". This deletes the mechanism, not just the sprawl (~1,000 derived files).
- Unify `.agy-plugin/` with `images/gemini/` under one harness-adapter convention:
  `<root>/images/<harness>/` = manifest + verify only. The 5th harness must not mint a
  5th root dot-dir generation.

### 4.2 Other dir moves

| Item | Action |
|---|---|
| `plugins/marketplace.json` vs `.claude-plugin/marketplace.json` | **[VETO HONORED]** no longer byte-identical — they drifted (one has interface/displayName, the other owner/name/email), which proves the duplication problem but invalidates "just symlink". Identify each file's consumer (Claude marketplace spec vs codex install scripts), reconcile the schema delta deliberately, then consolidate to one canonical + generated other. |
| `deploy/agentops-refinery.service` | → MTO/fleet repo together with `docs/runbooks/bushido-refinery.md`; loud deprecation in the runbook. |
| `examples/schedules/*.yaml` (8) | Delete with pointer to ADR-0009 — they fire jobs through the deleted `agentopsd`. |
| `lib/scripts/team-runner.sh`, `watch-{claude,codex}-stream.sh`, `lib/schemas/team-spec.json` | → MTO or attic (multi-worker dispatch). `lib/orchestrate-select.sh` stays (consumed by `cli/internal/orchestration/select.go`). |
| `bin/ralph` | Attic (zero callers verified; all "ralph" doc hits are the Ralph-Wiggum concept, not the script). `bin/` disappears. |
| `agents/` (2 subagent defs) | → `.claude/agents/` (harness-owned content under the harness adapter). |
| `evidence/` | → `.agents/evidence/`. **[VETO HONORED]** not a pure file move: three Go writers (`goals_steer_auto.go`, `rpi_phased_context.go`, `tick.go`) produce root `evidence/` paths and will regrow the dir on the next run — the writer re-point lands in the same change (tick.go is moving to MTO anyway). |
| `spec/teardown-2026-05-28/` | → docs/attic or .agents/research; spec/ keeps only scenarios/. |
| Phantom `cmd/`, `internal/`, `manifests/` (empty, untracked) | rmdir. |
| `wt-ag-qidx/` (207M worktree nested in the checkout) | `git worktree remove` after confirming the branch landed. |
| ~100-entry stale worktree registry | Sweep merged-branch worktrees, `git worktree prune`, and ship a **reaper script/cron** — this WILL regrow otherwise. |
| `dist/` + `.venv-docs/` + `.tmp/` + `cli/ao` + `coverage.out` + `.x/` + `.doctor/runs/` | Local clean (~310M+); add retention rules (keep last N) to the writers. |
| `.gc/`, `.ntm/` | Local clean when stale; note they are evidence the outer loop runs in this checkout — long-term home is the orchestrator's dirs. |

Keepers unchanged: skills/, cli/, docs/, scripts/, tests/, evals/, schemas/, .github/,
.githooks/, .beads/, .agents/, .claude/ (the model source+generator pattern),
.claude-plugin/, .codex/ + .codex-plugin/, .opencode/, homebrew-tap/, spec/scenarios/,
images/ (claude + codex pattern, gemini converged into it).

### 4.3 `ao` CLI surface (84 → ~31)

**Keeps (per census, minus [ADV+] folds):** rpi, evolve, autodev, loop, session, handoff,
ratchet, validate, eval, goals, scenario, turn, lookup, search, corpus (quality probes
only), knowledge, defrag, metrics, reconcile, doctor, capabilities, robot-docs,
version/config/completion/help, init, agents+agent (merged, see below), codex (clocked,
see below), scope, ci, beads (kept THIN per the Jeff-tool boundary), anti-patterns.

**Census merges (9, upheld):** memory+notebook → `ao memory`; init+quick-start+seed+demo
→ `ao init`; status+badge+vibe-check → `ao status`; the 8-command curation pipeline
(curate/dedup/contradict/maturity/pool/gate/constraint/findings) → `ao curate`;
forge+extract+mine+harvest → `ao forge`; compile+mind+wiki+sessions → `ao compile`;
`ao corpus inject` → `ao inject`; harness → `ao skills parity`; feedback-loop →
`ao flywheel`.

**[ADV+] additional folds (no deferred merges with no owner):**
- `ao patterns` → `ao defrag` (this wave, not "future acceptable").
- `ao citation` → `ao metrics cite` (two surfaces for citation freshness).
- `ao agents` + `ao agent` → one name with subcommands (one-character-apart commands are agent-ergonomics poison).
- `ao redact` → utility verb under doctor/util namespace.
- `ao codex` keep **with a deprecation clock set now** (self-described pre-v0.115.0 fallback): min-version + removal release named, CI-enforced expiry.

**Move out to MTO (14):** orchestrate, tick, refinery, chaos-test, council-gate,
verdict-gate, ready (duplicates `br ready` — Jeff boundary), close (duplicates
`br close`; ledger half is MTO's pattern), next-work, provenance, mcp, cron (finish the
started move), operator, claim (BC2 evidence-binding half folds into `ao validate`).
All use the `cron_self_adjust.go` route-notice shim precedent: **shim + notice + removal
version, never silent delete** — and each shim carries a machine-checkable removal
version that CI fails on after expiry (the cron move stalling mid-way is the proven
failure mode).

Move-out vetoes honored:
- **`ao provenance` [VETO HONORED]:** `ao provenance trace --orphans --strict` is a
  BLOCKING CI gate (`validate.yml:1407` + `check-provenance-orphans.sh` +
  `pre-push-gate.sh`). Moves only with the gate rewire in the same change, or keeps a
  read-only shim until MTO serves it.
- **`ao mcp`:** 10 skill refs (`ao mcp serve`) — loud deprecation path, not removal.
- **`ao next-work` [VETO HONORED]:** also referenced in kept
  `skills/post-mortem/references/harvest-next-work.md:189-191` — that doc is edited in
  the same change.

**Attic (6):** registry, retrieval-bench (→ evals/), canon (no team consumer yet;
absent from installed binary — drift evidence), guard-status + install-guards (fold into
doctor / init --fix if kept), and **trace — reconciled [VETO HONORED]:** census 1 kept
`skills/trace` while census 2 deleted `ao trace`; resolution is **keep both, folded**:
`ao trace` becomes `ao lookup trace` (it overlaps lookup's provenance fields), the skill
is updated to the new spelling, and `scripts/release-smoke-test.sh:448,:506`
(`ao trace --help`) is edited in the same change so release smoke doesn't break.

**Surface-drift ratchet (new CI gate):** fail when (a) a new top-level command lands
without a skill or doc referencing it, and (b) COMMANDS.md, `ao --help` prose, and the
cobra tree disagree (today they disagree three ways: help advertises `ao factory start`
which doesn't exist; installed 3.0.1 lacks orchestrate/tick which COMMANDS.md documents).

---

## 5. Docs / identity reset (21 root files → ~8)

**Standing root files after the reset:** README.md, PRODUCT.md (rewritten), GOALS.md
(stripped), PROGRAM.md, AGENTS.md, AGENTS-WORKFLOW.md, CHANGELOG.md,
goals-affects-files.yaml (+ mkdocs.yml, registry.json as tracked-for-now generated
artifacts; CLAUDE.md as a ≤30-line pointer).

### 5.1 AGENTS collapse (6 files → 2 + pointer)

CLAUDE.md (13.6K) duplicates AGENTS-WORKFLOW.md's Workflow section and has **already
drifted**: it still describes the retired PR flow after the 06-07 push-to-main rewrite,
and self-contradicts on CI-vs-local gate authority. Collapse:

- **AGENTS.md** — the one operator card (keeps the tiered table).
- **AGENTS-WORKFLOW.md** — the one reference doc, absorbing AGENTS-CODEX.md (49 lines),
  AGENTS-RUNTIME.md (59 lines), and AGENTS-CI.md (fold or keep if the CI table earns it).
- **CLAUDE.md** — ≤30-line pointer.
- Retire `scripts/validate-agents-split.sh` — a validator enforcing the 5-way split is
  sprawl made permanent. **[ADV+]** its CI wiring at `validate.yml:1372-1373` is edited
  in the same change, or only one copy of the sprawl is deleted.

Mt-olympus does this in ONE CLAUDE.md; that is the bar.

### 5.2 PRODUCT.md rewrite (375 → ≤200 lines)

Its own never-enforced limit (`check-product-freshness.sh`, wired into **no** gate —
verified absent from validate.yml and pre-push-gate.sh). Rewrite scope:

- Open with the README tagline as the one identity sentence; drop the wider "SDLC control
  plane" framing (razor contradiction).
- Market Convergence table → docs/comparisons/; 10-Star + Lineage/Meadows essays → docs/.
- **Strip live skill counts** — `sync-skill-counts.sh` mutates identity prose on routine
  work (3 commits in 2 days, all count churn); counts live only in generated surfaces
  (registry.json, docs/SKILLS.md).
- Wire check-product-freshness.sh into `pre-push-gate.sh --fast` and extend it to GOALS.md.

### 5.3 GOALS.md (shrink in place — machine-read by `ao goals`)

- Strip every "Progress:" status paragraph — state register inside a fitness contract is
  the per-edit growth vector; progress lives in bd/evidence artifacts.
- **Move out the North Stars razor-leak:** "The control plane is HA; the compute is
  fungible" + quorum/fleet/cross-instance language → mt-olympus doctrine. Pure outer loop,
  by the razor AND by REDUCTION.md's own boundary rule.
- Reword the CLAUDE.md Session-Constraints concession ("the in-session ao rpi loop is
  retired as the live workflow — runs on NTM + MCP Agent Mail") to the razor: **ao owns
  the inner loop; the substrate dispatches it.**

### 5.4 Other root files

| File | Action |
|---|---|
| PRACTICE-REGISTRY.md | Keep the machine-validated slug table (validate-practice-citations.sh needs it); lineage essay → docs/. |
| REDUCTION.md + PRE-REDUCTION-SNAPSHOT.md | → `docs/reduction/` with redirect stubs (still load-bearing for cp-nkk phase 2). Cite REDUCTION.md as prior art for the razor. |
| release-notes.md | → `docs/releases/3.1.0.md`. **[VETO-ADJACENT caveat honored]** release.yml regenerates a root release-notes.md at release time — the workflow's output path changes in the same commit or the file reappears every release. |
| docs/CHANGELOG.md | Dedupe (byte-identical 105K copy). **[VETO HONORED]** the duplicate is mechanically re-created: `pre-push-gate.sh` contains `cp CHANGELOG.md docs/CHANGELOG.md`. Delete that gate step and move the copy into the mkdocs build (`scripts/docs-build.sh`), or it resurrects on the next push. |
| CHANGELOG.md | Keep at root (public repo needs history). **[ADV+]** root keeps only the current major; older majors archive under `docs/releases/archive/`. |
| test_budget.env | Delete — zero consumers verified across scripts/, cli/, .github/, lib/, docs/. Dead state from a retired loop driver. |
| MEMORY.md | Retire — stale projection of .agents/learnings (anti-dogfooding). Caveat: `ao session memory` / `flywheel_close_loop.go` write it, so retirement is a small code change (point writers at .agents/) — propose as a bead. |
| registry.json | Keep tracked for now; churn discipline (regen once per skill wave). **[ADV+]** file the untrack-and-regenerate-on-demand bead **in this plan**, not "later" — per-wave hash churn in a tracked generated file is the same disease the audit diagnoses elsewhere. |

---

## 6. Why the last reduction failed — and the enforcement that holds this one

### 6.1 The regrowth mechanism (verified; adversary: "understated")

The 06-07 prune bottomed at 158 skills at 18:22; regrowth began **3 hours 13 minutes
later** (+4 "coverage-gap" skills at 21:35) and recovered 62% of the prune within 49
hours (now 166→169). The only shrinking surface in the repo is the `ao` Go-file count —
the only surface with a **standing contract** (REDUCTION.md buckets). Five channels:

1. **All gates check FORM, none check EXISTENCE.** ~30 validate-skill-*.sh scripts + the
   CI skill-gates job gate schema/frontmatter/parity/token budgets/count-SYNC — nothing
   asks "should this exist?". `new-skill-landing.md` is a paved road that makes adding
   easier. The edit-seal (critical-skills.txt) guards mutation, not admission.
2. **Derived-copy gates invert the ratchet.** CI enforces that per-harness COPIES exist
   and match instead of a generator owning them; every new skill mints 2–4 sibling files;
   deletion cost > addition cost.
3. **No deprecation clocks.** The cron→MTO move stalled mid-way; old and new coexist.
   Surface drift is unpoliced (COMMANDS.md / binary / help disagree three ways).
4. **No root router / identity gates.** check-product-freshness.sh is wired into no gate;
   wave artifacts default to root; split-instead-of-cut (the 5-file AGENTS split plus a
   validator enforcing it) relocates sprawl instead of removing it.
5. **The fleet edge resurrects prunes.** `~/.claude/skills` symlinks + the published
   plugin cache re-serve pruned skills (security-suite is live right now); the 06-09
   ghost re-import of the codebase-* quartet came through this channel.

Also structural: the 06-06 git re-baseline erased provenance/staleness signals — every
skill shows created 06-06/07, so nothing can be told apart by age or usage history.

### 6.2 What would have stopped it

Against THE RAZOR, 4 of the 8 regrown skills fail outright (agy-sidecar-scheduled-tick,
cross-vendor-trust-gate, storage-watchdog-ops, agy-project-worktree-permissions) and the
other 4 would have had to show justification. The prior rules rejected **zero** of them —
vacuously, because no standing admission criterion existed for skills at all.

### 6.3 The enforcement stack (adversary-graded: what holds vs what decays)

**REAL ENFORCEMENT (will hold — fail-closed, machine-checked, wired into gates with a
track record):**

1. **The admission gate.** THE RAZOR encoded as a check inside the existing skill-gates
   CI job + pre-push-gate: a new skill/command is admitted only with (a) a razor category
   (loop/context/flywheel/beads/validation/harness-config), (b) a usage-evidence citation,
   (c) a non-duplication attestation against the surviving corpus. The merged
   skill-builder carries the same gate as its first section. Renewal at review time also
   requires a usage citation — vacuous passes are closed.
2. **The root manifest.** Allowlist of root files/dirs in pre-push-gate; new root entry =
   fail unless listed. Converts every future root addition into an explicit decision.
   Single highest-leverage cheap change.
3. **Generator convergence (Wave 1).** The only proposal that DELETES a mechanism rather
   than policing it.
4. **Dated shims with CI-enforced expiry.** Every deprecation shim carries a removal
   version the CI fails on after expiry — the cron-stall precedent forbids comment-only clocks.
5. **The surface-drift ratchet** (§4.3): COMMANDS.md / help / cobra-tree agreement +
   no-orphan-commands.
6. **Plugin re-release + cache flush + resurrection watch** over all ~110 cut names —
   without this the prune un-happens on user machines regardless of the repo.

**BUDGET + OWNER (doctrine decays without both — adversary's exact finding):**
- **Skill budget: 60.** Hard ceiling checked in CI (surviving corpus is ~50–53; headroom
  of ~10 is the admission gate's working room, not a target).
- **CLI budget: 35** top-level commands, same gate.
- **Owner: Bo**, with the review cadence being the monthly check-product-freshness run —
  a ceiling nobody is named to defend becomes check-product-freshness.sh (a 200-line
  limit no gate runs while the file sits at 375).
- `new-skill-landing.md` is itself **rewritten** in the prune wave — the paved road IS
  the default; prose elsewhere does not reroute it. Its first step becomes the admission
  question, not the six-surface regeneration.

**THE EVENTUAL BINDING MECHANISM — the mt-olympus review gate.** The durable home for
admission is not a bash script in this repo: mt-olympus is building the factory's
single-writer / review-gate seam (workers cannot close their own work; validators are
separate). When AgentOps work is dispatched through MTO, the razor check becomes a
**verdict-gated close** — a skill addition is a bead whose closure requires an
independent validator applying the razor, recorded in the provenance ledger. The CI
admission gate above is the bridge until that seam carries the load; the budget numbers
transfer to it unchanged.

---

## 7. Execution shape — ordered waves for the agentops swarm

Constraints honored throughout: repo is PUBLIC (~400 stars) — every cut is
deprecation-loud (tombstone/shim/CHANGELOG, one release cycle, never silent); repo runs
HOT — each wave lands as **one reviewed change via an `am` reservation**, with scoped
file ownership and no cross-wave overlap; every wave is independently revertible (one
revert commit restores the prior state); the fleet's `link-skill` symlinks and both
CLAUDE.md skill tables are migrated, never broken.

**Wave 0 — local hygiene (no repo change, no review needed).**
rmdir phantom cmd/ internal/ manifests/; `git worktree remove` wt-ag-qidx (after branch
check); worktree-registry sweep + `git worktree prune`; rm dist/ .venv-docs/ .tmp/
cli/ao coverage.out .x/ .doctor/runs/ (~520M). Ship the worktree reaper script.

**Wave 1 — generator convergence (MUST land first — adversary ordering veto).**
Extend regen-all.sh into a true per-harness generator; replace skills-codex/,
.agy-plugin/skills/, images/gemini/skills/ with manifest + recipe; flip parity gates from
copies-exist to generator-current; keep skills-codex-overrides/ as the only manual layer;
reconcile the two marketplace.json schemas (consumer-by-consumer, then consolidate).
Revert: restore the copies, flip gates back.

**Wave 2 — enforcement before prune.**
Admission gate (razor + usage-evidence + budgets 60/35) into skill-gates CI +
pre-push-gate; root-manifest allowlist; wire check-product-freshness into
pre-push-gate --fast; surface-drift ratchet; rewrite new-skill-landing.md around the
admission question. Revert: remove the gate additions.

**Wave 3 — the skill prune (the big one; can split into 3a merges / 3b attics /
3c move-outs by file-ownership lanes).**
Execute §3 merges with tombstone stubs + reference sweep + overrides retirement;
attic the 32 with tombstones; move-outs ONLY through the hard-ordered migration gate
(destination repo serves → link-skill repoints → critical-skills.txt updated → tombstone
→ delete) — fleet-routed skills (agent-mail, vibing-with-ntm, using-atm, caam, dcg, ntm,
beads-*, heal-skill, operating-loop-workflow) are blocked until their destination is
live and the dotfiles CLAUDE.md tables are edited in the same arc. Regenerate catalog.json;
delete SKILL-TIERS.md. Revert: tombstones are additive; merges revert per-cluster.

**Wave 4 — fleet edge: plugin re-release + cache flush + resurrection watch.**
Cut a new agentops-marketplace plugin release from the pruned corpus; flush/invalidate
`~/.claude/plugins/cache`; stand up the resurrection watch over all ~110 cut names
(simple CI check: cut name reappears in skills/ or the published bundle = fail).
**The prune is not done until this ships** — it un-happens at the fleet edge otherwise.

**Wave 5 — CLI surface reset.**
§4.3 merges and [ADV+] folds; 14 move-outs as route-notice shims with CI-enforced
removal versions (provenance only with its gate rewire; mcp on a long clock; next-work
with the post-mortem reference edit; trace folded into lookup with the
release-smoke-test edit); finish the cron move; attic the 6; land the COMMANDS.md
regeneration + drift ratchet greenlit in Wave 2. Revert: shims are additive until their
expiry release.

**Wave 6 — docs/identity reset.**
AGENTS 6→2+pointer (+ validate.yml:1372-1373 edit + validate-agents-split retirement);
PRODUCT.md ≤200; GOALS.md Progress-strip + North-Stars move to MTO; root file moves
(REDUCTION pair, release-notes + release.yml path, docs/CHANGELOG dedupe + pre-push cp
removal, test_budget delete, MEMORY.md writer re-point bead, evidence/ writer re-point);
registry.json untrack bead filed. Revert: per-file.

Suggested swarm shape: Waves 1–2 are single-lane (gate/generator code, serialized);
Wave 3 fans out by cluster with file-ownership boundaries (merges / attics / move-outs
never touch the same skill dir); Waves 5–6 are two parallel lanes (CLI vs docs) with no
shared files. Every wave: gates green before push, session-close-verify equivalent, am
reservation released.

---

## 8. What this is NOT

- **Not a silent deletion.** Every removed name — skill, command, root file — gets a
  tombstone, shim, or redirect stub for at least one release cycle, plus a CHANGELOG
  entry. ~400 stars means real users; we deprecate loudly or not at all.
- **Not a fleet-breaking change.** No skill named in `~/.claude/CLAUDE.md` or
  `~/dev/CLAUDE.md` skill-registry tables, on `docs/contracts/critical-skills.txt`, or
  symlinked via link-skill is deleted before its destination serves it and the routes are
  migrated. Same-name preservation until the dotfiles edits land.
- **Not executed by this audit.** Read-only honored: this document is the only artifact;
  no beads created, nothing committed, no files moved. Each wave becomes beads only on
  Bo's approval.
- **Not a one-time cleanup.** The 06-07 prune was exactly that, and it regrew in 3 hours.
  This plan ships with the prune's holding mechanism (admission gate, budgets+owner, root
  manifest, generator, clocked shims, fleet-edge re-release) — if Waves 1, 2, and 4 are
  cut from scope, the prune should not ship at all, because it will fail identically.
- **Not a renaming of the outer loop.** Move-outs to mt-olympus land only when MTO
  actually serves the capability (operating-loop-workflow, provenance gates, refinery);
  shims hold the line until then.
- **Not a quota exercise.** The budget (60 skills / 35 commands) is a tripwire that
  forces a conversation, not a target to fill.

---

*Plan synthesized 2026-06-10 from five censuses + adversarial verification.
File: `docs/audits/inner-loop-reset-20260610.md`. Companion evidence:
`docs/audits/skill-usage-evidence-20260610.md`, `REDUCTION.md`,
`PRE-REDUCTION-SNAPSHOT.md`, `docs/adr/ADR-0009-daemon-deletion-in-session-only.md`.*
