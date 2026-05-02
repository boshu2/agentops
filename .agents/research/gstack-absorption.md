---
id: research-2026-05-01-gstack-absorption
type: research
date: 2026-05-01
---

# Research: gstack -> agentops Absorption Opportunities

**Backend:** general-purpose subagent
**Scope:** map gstack's 45-skill bundle, 55-helper toolbelt, 11 host adapters, multi-host install model into candidate agentops absorptions across `skills/`, `skills-codex/`, `hooks/`, and `cli/cmd/ao/` (+ `agentopsd`).

## Summary

The highest-leverage absorptions are gstack's **operator safety primitives** (`/freeze`, `/guard`, `/careful`, `/unfreeze`) and its **browser-as-primitive** lane (`/browse`, `/qa`, `/canary`) — both are coherent product surfaces agentops lacks entirely. Most other gstack value (planning reviews, ship/deploy lifecycle, learnings, retros, model benchmarking) is **already shipped** in agentops under different names and should not be re-ported. One architectural pattern (the `bin/gstack-paths` single-resolver convention for state paths) is worth extracting at the architecture level, not as a feature.

## Methodology

- Inputs: existing reverse-engineering report at `/home/boful/dev/personal/agentops/.agents/research/gstack/`
- Surfaces inventoried: `skills/` (69), `skills-codex/` (69 parity), `hooks/` (~50), `cli/cmd/ao/` (160+ Go files)
- Ranking: leverage * (1 - already-shipped) - adaptation-risk
- Tiers: must-port / worth-porting / defer-or-skip
- Substrate constraint: ao CLI is Go; skills are markdown (with skills-codex parity); hooks are shell. Bun is gstack-specific and not adopted.

## Already Shipped (No Action Needed)

| gstack feature | agentops equivalent | Delta worth noting |
|---|---|---|
| `/office-hours`, `/plan-ceo-review`, `/plan-eng-review`, `/plan-design-review`, `/plan-devex-review` | `skills/council/SKILL.md` (multi-judge perspectives) + `skills/design/SKILL.md` (product fit gate) | gstack splits perspectives into 4 named skills; agentops collapses to one council skill with `--preset` and personas. Skip port. |
| `/autoplan` (sequences plan-* reviews) | `skills/discovery/SKILL.md` (brainstorm + research + plan + pre-mortem orchestrator) | Already an orchestrator skill; gstack's autoplan is a thinner cousin. |
| `/skillify` (codify last `/scrape` into permanent skill) | `ao flywheel close-loop` + `skills/dream/SKILL.md` + `skills/forge/SKILL.md` (pattern -> skill promotion) | Per PRODUCT.md convergence note — already shipped. |
| `/learn` (manage cross-session learnings) | `skills/retro/SKILL.md`, `hooks/ao-extract.sh`, `.agents/learnings/` corpus | agentops has richer flywheel: retro -> forge -> compile pipeline. |
| `/retro` (weekly per-person retro) | `skills/retro/SKILL.md`, `skills/post-mortem/SKILL.md` | Direct overlap. |
| `/health` (skill health dashboard) | `cli/cmd/ao/heal_skill.go` + `skills/heal-skill/SKILL.md` + `cli/cmd/ao/flywheel.go` (`ao flywheel`) | agentops surface is broader (parity drift triage too). |
| `/cso` (OWASP+STRIDE) | `skills/security-suite/SKILL.md`, `skills/security/SKILL.md` | Already shipped. |
| `/benchmark-models` (cross-model side-by-side) | `cli/cmd/ao/eval.go` + `skills/council/SKILL.md` (cross-runtime council) | agentops council is the substrate; eval lane exists. |
| `/ship` (tests + review + push + open PR) | `skills/push/SKILL.md`, `skills/release/SKILL.md`, `skills/pr-prep/SKILL.md` | gstack's `/ship` is monolithic; agentops splits push vs release vs pr-prep — keep agentops shape. |
| `/land-and-deploy`, `/canary` (post-deploy probe) | `skills/release/SKILL.md` + manual deploy gates | Partial — see Tier 2 (`canary` browser-probe pattern). |
| `/landing-report` | `skills/status/SKILL.md` (single-screen dashboard) | Already shipped. |
| `/document-release` | `skills/release/SKILL.md` (changelog gen) + `skills/doc/SKILL.md` | Already shipped. |
| `/gstack-upgrade` | `skills/update/SKILL.md` (`ao update`) + `cli/cmd/ao/update_check.go` if present | Already shipped. |
| `/context-save`, `/context-restore` | `skills/handoff/SKILL.md`, `skills/recover/SKILL.md` | Direct overlap. agentops also has session-end-maintenance hook. |
| `/codex` (second opinion) | `skills/codex-team/SKILL.md` + skills-codex parity bundle (69 skills) | agentops goes much further — full parity runtime, not just second-opinion. |
| `/setup-gbrain` (Supabase memory sync) | `cli/cmd/ao/agentopsd.go` + Dolt remote (bushido WSL) for cross-host issue tracking | Different model (issue/bd-centric vs free-form memory rows). Skip. |
| `/plan-tune` (AskUserQuestion sensitivity) | `cli/cmd/ao/intent_echo.go`-adjacent + `hooks/intent-echo.sh` + `hooks/prompt-nudge.sh` | Different control axis but the goal (calibrate prompting density) is covered. |
| `/design-consultation` (build design system) | Out of scope for ao CLI; design system tooling is a vertical agentops doesn't target. | Skip. |
| `/setup-deploy` (one-time deploy detection) | `skills/scaffold/SKILL.md`, `skills/bootstrap/SKILL.md` | Already shipped (broader). |
| `/design-html` (publication HTML) | Out of scope. | Skip. |

## Tier 1: Must-Port (4 candidates)

### 1. Edit-scope guard (port `/freeze` + `/unfreeze` + `/guard` semantics)
- **gstack source:** `freeze/SKILL.md`, `unfreeze/SKILL.md`, `guard/SKILL.md` (hard block, not advisory)
- **Target surface:** **hooks + ao CLI** (combo). New `hooks/edit-scope-guard.sh` running on `PreToolUse:Edit|Write|Bash` checks a `.agents/scope.lock` file written by `ao scope freeze <dir>` / `ao scope unfreeze`. New `cli/cmd/ao/scope.go` subcommand manages the lock.
- **Adaptation:** shell hook reads JSON tool input, compares target path against locked dirs, exits non-zero with reason if outside scope. Go subcommand writes/reads `.agents/scope.lock` (additive set semantics; supports multiple frozen dirs). `update-config` skill registers the hook on install.
- **Leverage:** high — agentops swarms (`/swarm`, `/codex-team`, `/crank`) explicitly call out file-overlap as the #1 swarm failure mode. A first-class scope-lock would harden swarm wave isolation.
- **Risk:** hook race conditions if multiple agents touch the lock concurrently; need atomic file-replace semantics (already proven in `cli/cmd/ao/agentopsd.go` queue claim invariants per finding f-2026-04-29).
- **Already shipped?:** **no** — `dangerous-git-guard.sh` and `commit-review-gate.sh` are about git/commit operations, not edit-path scope. `swarm/SKILL.md` documents file-ownership boundaries but does not hard-enforce them.
- **Acceptance test:** L2 scenario — start swarm with two workers, freeze `cli/cmd/ao/`, confirm second worker's edit attempt outside frozen scope is rejected with structured reason.

### 2. Browser-as-primitive (port `/browse` invocation contract, not the binary)
- **gstack source:** `browse/SKILL.md`, `browse/src/`, used by `/qa`, `/design-review`, `/canary`, `/devex-review`
- **Target surface:** **skills + skills-codex** (parity). New `skills/browse/SKILL.md` documenting the `$B <command>` contract; defers actual execution to user-installed Chromium + an `AO_BROWSE_BIN` env var (mirroring `GSTACK_CLAUDE_BIN` indirection — see Pattern A below). Optionally `cli/cmd/ao/browse.go` thin wrapper if there's a Go-native option, but most likely the skill shells out to `chromium --headless` + CDP via `curl`.
- **Adaptation:** do **not** port the Bun-compiled `browse` binary (substrate mismatch, ~6 MB binary, license unclear). Port the **invocation contract** — a markdown skill that documents the verbs (`navigate`, `screenshot`, `click`, `eval`, `extract`) so any pre-installed headless browser can satisfy them. A user-side reference implementation lands as `bin/ao-browse` shell shim using `chromium-cdp` if available.
- **Leverage:** high — agentops has zero browser surface today. UI/UX validation (live-site audit, visual regression, post-deploy verification) is structurally absent and is the natural complement to `skills/vibe/SKILL.md` (code-only validation).
- **Risk:** high adaptation risk — Chromium availability varies wildly across user environments; CDP semantics drift across versions. Mitigate by shipping the skill as **contract-only** (skills/browse/SKILL.md with a "Bring your own browser" preamble) before attempting a Go wrapper.
- **Already shipped?:** **no.**
- **Acceptance test:** L2 — invoke `/browse navigate https://example.com && /browse screenshot /tmp/out.png` and assert PNG exists with non-zero size.

### 3. State-path resolver convention (port `bin/gstack-paths` pattern, not the script)
- **gstack source:** `bin/gstack-paths` (sourced via `eval "$(bin/gstack-paths)"`); honors `GSTACK_HOME`, `CLAUDE_PLUGIN_DATA`, `CLAUDE_PLANS_DIR`
- **Target surface:** **lib/ + ao CLI**. Audit existing path resolution (likely scattered across `cli/cmd/ao/context.go`, `inject.go`, `compile.go`, `harvest.go`); centralize into `lib/ao-paths.sh` (sourceable) and `cli/internal/paths/` (Go). Honor `AO_HOME`, `AO_AGENTS_DIR`, `AO_KNOWLEDGE_ROOT` with single resolver.
- **Adaptation:** Go package returning a struct with resolved roots. Shell helper emits `export AO_AGENTS_DIR=...` lines for `eval`. Existing hooks migrate to source it.
- **Leverage:** medium-high — agentops has the same scatter problem (multiple hooks reach into `.agents/` with hardcoded relative paths; harvest.go and inject.go disagree on root resolution per known finding work). Centralizing eliminates a class of bugs.
- **Risk:** low; mostly mechanical refactor with strong test coverage already.
- **Already shipped?:** **partial** — `lib/hook-helpers.sh` is the closest existing surface but does not own path resolution as its mission. Delta: extract a dedicated `lib/ao-paths.sh` and require all hooks to source it.
- **Acceptance test:** unit test that overriding `AO_HOME` flips resolution for both Go and shell call sites in lock-step.

### 4. Skill health dashboard (port `bun run skill:check` -> `ao skills check`)
- **gstack source:** `scripts/skill-check.ts` invoked by `/health` skill
- **Target surface:** **ao CLI**. New `cli/cmd/ao/skills.go` `ao skills check` subcommand that walks `skills/` and `skills-codex/`, validates frontmatter (name, description, allowed-tools), checks every `references/*.md` is linked from SKILL.md, flags codex parity drift, reports on a single dashboard.
- **Adaptation:** Go binary equivalent of the gstack TS script; this consolidates work currently scattered across `scripts/sync-skill-counts.sh`, `heal.sh --strict`, `audit-codex-parity.sh`, `scripts/regen-codex-hashes.sh`. The `/heal-skill` skill already does the fix half — `ao skills check` would do the read-only audit half as a fast first call.
- **Leverage:** medium-high — agentops has the gates (`scripts/pre-push-gate.sh`, validate.yml's 24 jobs) but no single `ao skills check` health probe. Operator pain.
- **Risk:** low — read-only, additive.
- **Already shipped?:** **partial** — `cli/cmd/ao/heal_skill.go` exists for repair; `flywheel.go` reports knowledge health. Delta: add a `skills` subcommand that aggregates all the sync/parity/lint shell scripts behind one Go entrypoint. Codex parity check must be included from day one (per the two-runtimes constraint).
- **Acceptance test:** `ao skills check --json` output contains entries for all 69 skills with parity status, frontmatter validity, broken-reference list.

## Tier 2: Worth-Porting (6 candidates)

### 5. Telemetry / timeline / question / review log convention
- **gstack source:** `bin/gstack-{telemetry,timeline,review,question,learnings}-{log,read,search,preference}` (10 scripts)
- **Target surface:** **hooks + ao CLI**. agentops has citation-tracker.sh, ao-extract.sh, learnings/. The gstack pattern is more uniform: one `*-log` writer + one `*-read` reader per concept, all writing JSONL into `${STATE}/<concept>/`.
- **Adaptation:** Standardize agentops's append-only writer convention (e.g., `ao log append --kind=citation`, `ao log read --kind=citation --since=...`). Migrate ad-hoc JSONL writers (citations.jsonl, registry.jsonl, next-work.jsonl) to a single substrate.
- **Leverage:** medium — current JSONL writers have inconsistent schemas (per .agents/findings/registry.jsonl conflict in git status).
- **Risk:** medium — touches many existing surfaces; risk of breaking inject ranker / decay calculations that read these files.
- **Already shipped?:** **partial** — append-only JSONL is the convention; what's missing is the unified read/write CLI.
- **Acceptance test:** all `.agents/**/*.jsonl` writers route through one Go function; schema validation at write time.

### 6. `/canary` post-deploy probe lane
- **gstack source:** `canary/SKILL.md` (post-deploy verification loop using browse)
- **Target surface:** **skills + skills-codex + agentopsd**. New `skills/canary/SKILL.md` invokes `/browse` (Tier 1 #2) against deployed URLs; agentopsd schedules recurring probes via cron-cadence (per finding f-2026-05-01-017 — cron triggers go in agentopsd, not hooks).
- **Adaptation:** depends on Tier 1 #2 landing first. Codex parity required.
- **Leverage:** medium — fits the agentops "validation continues post-deploy" gap. Real value only if a meaningful chunk of users deploy web surfaces; currently most agentops users target CLI/library work.
- **Risk:** depends on browse port. Skip if Tier 1 #2 punted.
- **Already shipped?:** **no.**
- **Acceptance test:** scheduled probe writes JSONL outcome under `.agents/canary/`; failure escalates via existing alert hook.

### 7. `/investigate` systematic root-cause skill (delta vs `/bug-hunt`)
- **gstack source:** `investigate/SKILL.md` — systematic root-cause debugging
- **Target surface:** **skills + skills-codex**. agentops has `bug-hunt/SKILL.md` already. Delta is whether gstack's investigate has a distinct "narrow-cause-funnel" workflow (5-whys, hypothesis ranking) vs bug-hunt's repro-first focus.
- **Adaptation:** read gstack's investigate verbatim (under license guidelines — name + lane indexing only per spec-clone-vs-use.md), extract the **process delta**, fold into bug-hunt as an optional `--mode=systematic` path.
- **Leverage:** low-medium — bug-hunt is well-shipped; this is a refinement.
- **Risk:** scope creep on bug-hunt.
- **Already shipped?:** **partial** (covered by bug-hunt with overlap).
- **Acceptance test:** bug-hunt skill gains a documented `systematic` mode with 5-step funnel.

### 8. Operator-friendly settings hook (`bin/gstack-settings-hook`)
- **gstack source:** `bin/gstack-settings-hook` family — registers/unregisters hooks in host settings.json
- **Target surface:** **ao CLI**. New `ao hooks install` / `ao hooks uninstall` / `ao hooks list` subcommand that updates `~/.claude/settings.json` (and Codex equivalent) idempotently.
- **Adaptation:** Go subcommand that knows hook schema; lives next to `cli/cmd/ao/agentopsd.go`. Replaces manual edits the user does today via the `update-config` Claude skill.
- **Leverage:** medium — eases install path (today, hook activation requires the user to edit settings.json by hand or run `update-config`).
- **Risk:** low — additive, idempotent.
- **Already shipped?:** **no** — `scripts/install.sh` installs skills but does not wire hooks into settings.
- **Acceptance test:** `ao hooks install --hook=ao-extract` sets the entry; re-run is no-op; `ao hooks uninstall` removes cleanly.

### 9. Multi-host install adapter pattern (`hosts/*.ts`)
- **gstack source:** `hosts/*.ts` (factory + adapter per host)
- **Target surface:** **ao CLI + scripts**. agentops already supports 4 runtimes (Claude Code / Codex / Cursor / OpenCode per GOALS.md). Today's install is mostly script-based. Port the **factory + adapter** pattern: `cli/internal/hosts/{claude,codex,cursor,opencode}.go` each implementing a small interface (skills-dir path, render-skill, install-hook).
- **Adaptation:** Go interface mirroring gstack's TS shape. Drop unsupported hosts (Cursor untested per constraints, OpenClaw / Hermes / Kiro / Slate explicitly out per GOALS.md).
- **Leverage:** medium — codifies install logic that is currently ad-hoc shell.
- **Risk:** medium — risks rewriting working `scripts/install.sh` for marginal gain. Consider only if a 5th runtime lands.
- **Already shipped?:** **partial** (shell-script install works but is not extensible).
- **Acceptance test:** adding a new host = one new Go file, no changes to install entrypoint.

### 10. Slop-scan content lint (`bun run slop`, `slop-scan.config.json`)
- **gstack source:** `scripts/slop-diff.ts`, `slop-scan.config.json`
- **Target surface:** **hooks + ao CLI**. Add an "AI-slop phrase" lint pass that flags low-signal patterns ("In summary,", "It is important to note", emoji-laden bullets) in committed markdown.
- **Adaptation:** Go regex pass with config file (`.agents/slop-config.json`); hook integrates into `pre-push-gate.sh`.
- **Leverage:** medium — agentops authored skills/docs are AI-written and drift toward slop. A targeted lint reduces review toil.
- **Risk:** false positives if regex is too aggressive; ship as `--warn` first.
- **Already shipped?:** **no** — `hooks/skill-lint-gate.sh` enforces structural rules, not stylistic ones.
- **Acceptance test:** lint flags known slop sample with line numbers; passes a known-clean SKILL.md.

## Tier 3: Defer-or-Skip

### 11. `gstack-brain-*` cross-machine memory (Supabase)
- **Reason to defer/skip:** agentops's cross-host story is Dolt-on-bushido (per CLAUDE.md). Adding a Supabase lane creates a second source of truth and conflicts with the design. **Skip.**

### 12. Bun runtime adoption
- **Reason:** Substrate mismatch. ao CLI is Go; runtime change is a non-starter. **Skip per constraints.**

### 13. `/pair-agent` browser bridge
- **Reason:** Niche pairing-with-remote-agent flow that depends on the browser primitive landing first AND on hosts agentops doesn't ship for (OpenClaw). **Defer until Tier 1 #2 lands.**

### 14. `/design-shotgun` multi-variant AI design board
- **Reason:** Design-system vertical, not in agentops scope. **Skip.**

### 15. `/document-release` (sync docs to what shipped)
- **Reason:** Already covered by `release/SKILL.md` + `doc/SKILL.md`. **Skip (already shipped).**

### 16. `/builder-profile`, `/developer-profile`, `/specialist-stats`
- **Reason:** Operator-stats vertical (gstack telemetry-driven). Possibly a future flywheel feature, but agentops's `flywheel.go` already covers the user-facing read. **Defer.**

### 17. Conductor workspace integration
- **Reason:** Third-party (Conductor.app) tie-in. agentops doesn't ship a workspace abstraction. **Skip.**

### 18. `/setup-browser-cookies` (import cookies from real browser)
- **Reason:** Depends on browse primitive + raises security concerns (cookie exfiltration). Defer indefinitely. **Skip.**

## Cross-Cutting Pattern Extracts

### Pattern A: Single-resolver state-path indirection

gstack centralizes every state path through `bin/gstack-paths`, sourced via `eval "$(bin/gstack-paths)"`. Every helper that needs `${STATE}/plans/`, `${STATE}/learnings/`, etc. gets it through one resolver. The win: `CLAUDE_PLUGIN_DATA > GSTACK_HOME > default` precedence is encoded **once**, not in every consumer. Path migrations become a one-file change.

agentops would gain consistent behavior across `inject.go`, `harvest.go`, `compile.go`, `flywheel.go`, and the hook fleet by extracting `lib/ao-paths.sh` + `cli/internal/paths/`. Today the same path is recomputed (often inconsistently) in 10+ places. **This is the architectural absorption with the highest ROI** — it's invisible to users but eliminates a recurring class of "agent A wrote to .agents/X but agent B reads from ~/.agents/X" bugs.

### Pattern B: Skills-shell-out-to-helpers (thin prompts, fat toolbelt)

gstack's discipline: SKILL.md describes intent + invokes `bin/gstack-*`. Logic lives in shell/TS, not in the prompt. This keeps prompts auditable, testable in isolation, and hot-swappable without re-reading skill markdown. agentops follows this pattern partially (`hooks/` are shell, `cli/cmd/ao/` is Go, skills mostly invoke `ao` subcommands), but several skills still embed multi-step procedural logic in markdown. A targeted refactor pass would convert "do X, then Y, then Z" SKILL.md sections into `ao <skill> step1 / step2 / step3` Go entrypoints, leaving the SKILL.md as an orchestration shell. Lower priority than Pattern A; flag as a refactor target during `/heal-skill` reviews.

### Pattern C: One-source-many-hosts via host adapters

gstack's `hosts/*.ts` factory + per-host adapter is the model agentops's `skills/` -> `skills-codex/` parity should aspire to. Today, parity is enforced by `audit-codex-parity.sh` AFTER the fact (drift detection); gstack's pattern enforces it BEFORE the fact (one source `.tmpl`, N renders). If a 5th runtime is added (or if Cursor support firms up per GOALS.md), refactor to a per-host adapter pattern at that point. Don't pre-build it.

### Pattern D: Tiered tests by API spend

gstack exposes 9 test lanes (`free`, `audit`, `gate`, `e2e`, `evals`, `gemini`, `codex`, `periodic`, `windows`) with explicit cost tiers. agentops has `make test`, `pre-push-gate.sh --fast`, `validate.yml` (24 jobs) but no explicit "this lane costs API spend" labeling. Adding `make test:eval` / `make test:free` aliases that mirror gstack's taxonomy would clarify which lanes a contributor can run for free vs. requires `ANTHROPIC_API_KEY`. Low cost, modest clarity win.

## Findings to Persist

1. `dedup_key: gstack-edit-scope-guard | pattern: hook-enforced edit-scope lock | detection_question: Does agentops have a hard-block mechanism preventing edits outside a declared swarm-wave scope? | checklist_item: Add /scope freeze + PreToolUse hook before any new multi-worker swarm primitive | applicable_when: spawning 2+ workers via /swarm or /codex-team | confidence: 0.85`

2. `dedup_key: gstack-state-path-resolver | pattern: single-source state-path resolver vs scattered hardcoded paths | detection_question: Are .agents/ subpaths resolved through one library or recomputed in each consumer? | checklist_item: Audit hooks + cli/cmd/ao/ for ad-hoc .agents/ path strings; centralize via lib/ao-paths.sh + cli/internal/paths/ | applicable_when: any new hook or cli subcommand reads or writes under .agents/ | confidence: 0.9`

3. `dedup_key: gstack-browser-as-primitive | pattern: skill bundle that lacks a browser-validation lane has a structural blind spot for live-site issues | detection_question: Can the operator validate post-deploy UI from inside the agent loop? | checklist_item: Before declaring validation complete on a web-surface change, confirm a /browse or equivalent live probe ran | applicable_when: shipped change touches HTTP-served surface (web app, dashboard, OAuth flow) | confidence: 0.7`

## Recommendations

1. `/plan` should turn **Tier 1 candidates 1, 3, and 4** into bd issues this cycle. Tier 1 #2 (browse) needs its own design doc first — defer planning until the contract-only-vs-Go-wrapper question is decided.
2. `/pre-mortem` attention required on **Tier 1 #1 (edit-scope guard)**: the lock-file race risk is the same class of failure that bit `agentopsd` queue-claim invariants (see findings f-2026-04-29 series). Reuse the proven atomic-replace pattern; do not invent new locking semantics.
3. `/design` should validate that **Tier 2 #6 (canary)** is on-strategy before any port — agentops users are mostly CLI/library authors today, so the live-site probe lane may not pull weight.
4. **Do not port** anything from the "Already Shipped" table even if a delta is tempting; agentops has stronger primitives (council, flywheel, dream) that the gstack equivalents would dilute.
5. **Architectural Pattern A (state-path resolver)** is the highest-ROI absorption regardless of whether any specific gstack feature ships; treat it as a foundation for any new ao subcommand or hook landing in the next two cycles.
