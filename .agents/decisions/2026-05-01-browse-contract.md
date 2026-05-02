---
id: decision-2026-05-01-browse-contract
type: decision
date: 2026-05-01
status: accepted
related_research: .agents/research/gstack-absorption.md
related_epic: soc-irg1
issue: soc-irg1.4
---

# Decision: Browser-Validation Lane Contract for AgentOps

## Context

gstack ships a coherent **browser-validation lane** (`/browse` + `/qa` + `/design-review` + `/canary` + `/devex-review`) anchored on a Bun-compiled `browse` binary that wraps headless Chromium via the Chrome DevTools Protocol. Skills invoke it as `$B <command>` (~100 ms/call) with verbs `navigate`, `screenshot`, `click`, `eval`, `extract`. AgentOps has **zero browser surface today** — `skills/vibe/SKILL.md` validates code statically, but there is no live-site / post-deploy / visual-regression lane. The gstack-absorption synthesis ranked this a Tier 1 ("Must-Port") candidate but explicitly deferred planning until a contract-only-vs-Go-wrapper-vs-defer decision was made (see `.agents/research/gstack-absorption.md`, Recommendation #1).

The decision is constrained on two axes. **First**, license + portability: gstack's `browse` binary is Bun-compiled (~6 MB), license-unclear for re-distribution, and substrate-mismatched with agentops's Go CLI. **Second**, persona fit: PRODUCT.md's three target personas are the Solo Developer (ships features faster), Agent Orchestrator (parallel agents on shared codebase), and Quality-First Maintainer (fewer, higher-confidence releases). None of these personas are framed around web/UI work. The synthesis is blunt: *"agentops users are mostly CLI/library authors today, so the live-site probe lane may not pull weight"* (`gstack-absorption.md` Recommendation #3, applied to the `/canary` follow-on).

The strategic question therefore is not "should agentops have browser primitives" — it is "what is the cheapest move that closes the structural gap without committing to a substrate (Chromium + CDP) the user base does not yet need at scale".

## Options

### Option A: Contract-only markdown skill (`skills/browse/SKILL.md` + Codex parity)

**Description.** Ship `skills/browse/SKILL.md` and `skills-codex/browse/SKILL.md` (parity from day one — see two-runtimes constraint in `.agents/research/gstack-absorption.md` Tier 1 #4). The skill documents an invocation contract — verbs (`navigate`, `screenshot`, `click`, `eval`, `extract`), expected I/O shape, exit codes — and shells out to whatever binary `AO_BROWSE_BIN` resolves to (mirroring the `GSTACK_CLAUDE_BIN` indirection pattern). Optionally ship `bin/ao-browse` as a thin reference shim that uses `chromium --headless` + `curl` against CDP if Chromium is already installed.

**What the user gets.** A documented, agent-invokable verb set (`/browse navigate ...`, `/browse screenshot ...`) the moment they install or BYO a Chromium runtime. Agents can compose live-site checks into `/vibe`, `/canary`, or any custom flow.

**What the user does not get.** Turnkey browser execution. If `AO_BROWSE_BIN` is unset and no `chromium` is on PATH, `/browse` returns a structured "no browser available — see SKILL.md install hints" error, not an installed Chromium.

**Cost.** 1–2 days. Mostly markdown + two short shell shims (`bin/ao-browse` reference impl + a `--probe` self-check). One smoke test asserts the contract responds correctly when Chromium is absent and when present.

**Dependencies.** None mandatory. Optional: user-installed Chromium / `google-chrome` / Edge in headless mode. No new Go code. No new CI surface beyond shell smoke tests.

### Option B: Go wrapper around user-installed Chromium (`cli/cmd/ao/browse.go` + `cli/internal/browse/`)

**Description.** Ship a Go subcommand `ao browse <verb>` that locates a Chromium-family binary via `AO_BROWSE_BIN` or PATH probe, launches it with `--headless --remote-debugging-port`, and speaks CDP over the WebSocket directly from Go. The skill at `skills/browse/SKILL.md` then invokes `ao browse`, giving uniform behavior across hosts.

**Surface.** New `cli/cmd/ao/browse.go` (~400–600 LOC), new `cli/internal/browse/` package (CDP client, screenshot/navigate/eval primitives, retry/timeout logic), updated `cli/docs/COMMANDS.md`, and a `make sync-hooks` impact since `/vibe`'s prompt may want to optionally invoke `/browse`.

**Cost.** 1–2 weeks engineering plus ongoing maintenance. CDP is a moving target: Chrome major versions occasionally rename methods (`Page.navigate` is stable but `Emulation.*`, `Runtime.evaluate` parameter shapes drift). Tests need a real Chromium in CI or an elaborate mock.

**Dependencies + risks.** Pulls in a CDP client library or hand-rolls one. License + portability concerns: even though we don't ship Chromium, we now own a binary that breaks if the user installs Chrome 137 with a renamed verb. License is fine for our Go code under MIT, but we inherit the Chromium-availability matrix as a support burden — and Chromium availability varies wildly across user environments (the synthesis explicitly flags this as the highest-risk dimension of any port, `.agents/research/gstack-absorption.md` Tier 1 #2 "Risk").

### Option C: Defer entirely

**Description.** Do not ship a browser lane. Mark Tier 1 #2 as deferred in the absorption backlog with explicit revisit triggers (see "Decision criteria for revisiting" below). No skill, no Go code, no shim.

**What triggers reconsidering.** A demonstrated user-base shift toward web-served work, a hard dependency from a prioritized epic (e.g., `/canary` becoming top-quartile demand), or a Tier-2 partner skill that needs `/browse` as a strict prerequisite.

## Tradeoff Matrix

| Dimension | A (contract-only) | B (Go wrapper) | C (defer) |
|---|---|---|---|
| Engineering cost | low (1–2 days, mostly docs + 1 shim) | medium-high (1–2 weeks; CDP integration + Go pkg + tests) | zero |
| Maintenance burden | low (skill prompt drift only) | high (CDP version drift, Chrome major-version churn, test infra) | zero |
| User value (today) | meaningful for users with Chromium installed; structured "not available" otherwise | turnkey if user has Chromium; same failure surface if not | none |
| PRODUCT.md persona fit | partial — Quality-First Maintainer (post-deploy probes) and Agent Orchestrator (composable lane); weak fit for Solo Developer until web work appears | same partial fit as A; B does **not** unlock new personas | n/a |
| Risk | low — markdown skill, additive, reversible by deletion | high — Chromium availability + CDP drift + Go test infra | low (opportunity cost only) |
| Reversibility | high (delete one skill dir) | medium (deprecation cycle for a Go subcommand consumed by other skills) | high |
| Aligns with agentops's "bring your own X" pattern | yes (mirrors `AO_LLM_ENDPOINT`, `GSTACK_CLAUDE_BIN`, OAuth-based Codex/Claude) | no — agentops would own a binary-dependency contract with the user's OS | n/a |
| Unblocks Tier 2 #6 (`/canary`) | yes — contract is the only dependency | yes — same | no |
| CI surface added | shell smoke test (~30 LOC) | Go tests + Chromium-in-CI matrix or extensive mocks | none |
| Closes "live-site validation" structural gap | yes, contractually | yes, executably | no |

## Recommendation

**Option A: Contract-only markdown skill, with Codex parity from day one.**

The synthesis's own Recommendation #3 already telegraphs the answer: *"agentops users are mostly CLI/library authors today"*. PRODUCT.md confirms it — none of the three target personas are characterized by web/UI work. Spending 1–2 weeks on a Go CDP wrapper now would be building executable infrastructure ahead of demonstrated demand, which is exactly the failure mode `agentops` patterns warn against (see `.claude/rules/go.md` complexity budget; the proposed B surface adds 400–600 LOC of net-new Go for a feature whose user-base pull is unproven).

Option A is **strictly Pareto-better than C** at low marginal cost: it closes the structural gap (post-deploy/UI validation lane *exists* contractually, available to any Tier-2 skill that depends on it — e.g., `/canary`) without committing to a substrate. It is **strictly Pareto-better than B** for the current persona mix because it (a) ships in days, (b) inherits agentops's proven "bring your own X" indirection (`AO_LLM_ENDPOINT`, `GSTACK_CLAUDE_BIN`, Codex OAuth, Claude OAuth — all the user-supplies-the-substrate patterns that ship today), and (c) is trivially upgradeable to Option B later if usage data warrants — the skill contract becomes the public interface, and a Go implementation slots in behind it without breaking callers.

The two failure modes Option A must address are honest: (1) users without Chromium see a "not available" error instead of a working tool — mitigated by a clear `--probe` self-check verb in the skill and an install-hints reference doc; (2) skill authors building on `/browse` (e.g., a future `/canary`) must handle the "browser unavailable" branch — this is the same posture `/dream` takes for its Ollama dependency and is well-precedented in agentops.

**Decision: ship A this cycle. Defer B until revisit-criteria fire.**

## Decision criteria for revisiting

Flip the recommendation toward **Option B (Go wrapper)** when any of the following becomes true:

- **Persona shift:** ≥30% of agentops user repos contain web-served surfaces (frontend, dashboard, OAuth flow). Detection: add a one-time `ao detect surfaces` check that flags `package.json`+`pages/`/`app/`/`server.ts`/`Dockerfile` patterns; surface in the flywheel report.
- **Demand signal:** ≥3 user-filed issues request browser-validation, OR `/browse` skill activation count exceeds the median Tier-2 skill in the flywheel.
- **Hard dependency:** a prioritized epic (e.g., a hardened `/canary` or `/design-review` lane targeting the Quality-First Maintainer persona) requires the executable surface beyond what a shim can deliver.
- **Test-coverage pressure:** users report repeated "browser unavailable" failures in flows where Chromium *is* installed but PATH/version mismatch defeats the shim — indicating the discovery + version-negotiation logic needs to live in Go.

Flip toward **Option C (full defer + skill removal)** only if 6 months pass with the skill at zero invocations and no Tier-2 dependency landed.

## Follow-up bd issues

Recommendation A requires one implementation issue and (optionally) a follow-on shim issue. Filed under epic `soc-irg1`:

- **soc-irg1.6** (filed via `bd create` 2026-05-01) — Implement `skills/browse/SKILL.md` + `skills-codex/browse/SKILL.md` contract with `AO_BROWSE_BIN` indirection, `--probe` self-check verb, and shell smoke test. Estimate: 1–2 days. Includes a `bin/ao-browse` reference shim that probes for `chromium`/`google-chrome`/`microsoft-edge` and shells the documented verbs. Parent: `soc-irg1`.

(The `/canary` Tier-2 #6 candidate from the synthesis remains its own decision and is **not** part of this decision's follow-up scope. It depends on this contract landing first per `.agents/research/gstack-absorption.md` Tier 2 #6 "Adaptation".)

## Status closeout

Status: **accepted**

Rationale: the decision is opinionated (Option A), grounded in PRODUCT.md persona analysis + the synthesis's own framing, and has explicit revisit triggers. No council validation gate is required for a contract-only decision artifact (no code change, no production surface). The follow-up implementation issue carries its own `/pre-mortem` requirement before crank pickup.
