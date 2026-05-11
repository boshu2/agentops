# Multi-runtime tier charter

> Source-of-truth for the multi-runtime validation tier model. References:
> `docs/contracts/hook-runtime-contract.md` (event mapping), `GOALS.md`
> directive D1, bead `soc-ymph.1`.

AgentOps supports four agentic runtimes (Claude Code, Codex, Cursor,
OpenCode). Each runtime has different surface area, hook semantics, and
authentication requirements. The validation surface is graded across
three tiers; this charter is the explicit declaration of which tier is
blocking in CI today and which is opt-in.

## Tier Definitions

### Tier S — Structural / install smoke

Files, manifests, generated bundles, installer scripts, and static
runtime-specific entrypoints are present and internally consistent.

**Gate behavior:** **Blocking in CI.** No live runtime or auth required.
Verifies that installation surfaces don't drift from runtime contracts
without anyone catching it.

**Current coverage:** all four runtimes have Tier S coverage today
(see Coverage Matrix below).

### Tier I — Live inventory / load proof

A real runtime can load AgentOps and report the visible skill inventory,
or a documented load-check fallback passes when inventory is unavailable.

**Gate behavior:** **Skip or warn** when the runtime/auth is absent;
strict env vars can make failures blocking. Not a default-blocking gate
because CI runners do not carry runtime API keys.

**Current coverage:** Claude Code + Codex have Tier I; Cursor + OpenCode
do not.

### Tier E — Live execution proof

A real runtime executes an AgentOps workflow end to end against a
scenario (e.g., a runtime actually runs `/rpi` or `/validate` and the
output matches expected envelope shape).

**Gate behavior:** **Opt-in / nightly only.** Not a default CI gate
because:

1. Requires runtime auth (Anthropic, OpenAI, or other API keys) that
   public CI cannot safely carry.
2. Has real budget cost — even a single execution may consume tokens
   priced in dollars, multiplied by every CI run.
3. Has wall-clock cost (minutes per scenario × multiple scenarios ×
   multiple runtimes) that pushes CI past reasonable cadence.
4. Brings live-service flakiness (rate limits, transient API outages)
   into the merge gate.

**Current coverage:** **No runtime has Tier E as a default CI gate.**
This is intentional, not an oversight.

## Coverage Matrix

| Runtime | Tier S | Tier I | Tier E |
|---|---|---|---|
| Claude Code | `tests/skills/test-runtime-claude-code-smoke.sh` | `scripts/validate-headless-runtime-skills.sh --runtime claude` (load-check fallback) | Opt-in (no default CI lane) |
| Codex | `tests/skills/test-runtime-codex-smoke.sh` | `scripts/validate-headless-runtime-skills.sh --runtime codex` (load-check fallback) | Opt-in (no default CI lane) |
| Cursor | `tests/skills/test-runtime-cursor-smoke.sh` (`.mdc` export converter) | Not implemented | Opt-in (no default CI lane) |
| OpenCode | `tests/skills/test-runtime-opencode-smoke.sh` | Not implemented | Opt-in (no default CI lane) |

## Tier E Opt-in Recipes

When an operator wants Tier E proof, the canonical entry points are:

### Claude Code

```bash
# Inside a real Claude Code session, run:
/rpi "smoke test the multi-runtime tier charter"
# Then verify the session produces the expected RPI envelope.
```

### Codex

```bash
# With a Codex CLI install and valid auth:
codex exec --prompt "Run /validate on the current branch and report status"
# Then verify the output matches docs/contracts/eval-verdict-pipeline.md envelope.
```

### Cursor

Cursor Tier E is intentionally manual. Open the editor with a real
project, invoke a registered `.mdc` skill, and verify the produced
artifact. There is no headless Cursor lane on the roadmap.

### OpenCode

OpenCode Tier E is intentionally manual today. The headless inventory
gate is on the roadmap; opt-in execution proofs run from an operator's
local OpenCode instance.

## Why opt-in, not blocking

The blocking surface for AgentOps merges is **Tier S structural** plus
the various contract gates declared in `GOALS.md` (council coverage,
durable learning, loop closure). Tier S catches the failure modes that
matter for "did the code ship a coherent install bundle":

- skills/hooks/manifests drift from runtime contracts
- installer scripts break for one runtime but not others
- generated artifacts (codex bundles, .mdc exports) miss a fresh runtime change

Tier E catches a different failure class — "does a real runtime actually
execute what we shipped end-to-end" — but the cost/reliability profile
makes it unsuitable as a default merge gate. It belongs in nightly
operator runs (one of the four runtimes per night, manually rotated) or
in pre-release validation, not on the merge path.

The honest framing is: **AgentOps is a hub-and-spoke library, not a
multi-runtime end-to-end test harness.** Spoke runtimes own their
own real-execution proof; AgentOps's contract is that the spoke
artifacts are structurally correct.

## When to revisit

This charter should be revisited if:

1. **A reusable mock-runtime harness emerges** that lets us run Tier E
   scenarios in CI without real auth/budget (no such harness exists
   today; the runtimes are closed systems).
2. **A specific runtime's spoke ownership changes** — e.g., if Anthropic
   ships an officially-supported headless Claude Code execution lane,
   adopting it for Tier E becomes possible.
3. **A regression class is discovered that Tier S structural cannot
   catch but Tier E live can.** Document the class in a bead before
   investing in CI Tier E lanes.

## Compliance summary

Directive D1 (`GOALS.md`): closed via this charter doc per the bead's
"OR" acceptance — explicit charter doc OR CI Tier E lane. Tier E is
declared opt-in here; no CI Tier E lane is added.

## Related contracts

- `docs/contracts/hook-runtime-contract.md` — event mapping per runtime,
  hook capability matrix, install behavior
- `docs/contracts/headless-invocation-standards.md` — what "headless
  invocation" means for each runtime
- `docs/contracts/release-readiness.md` — what release validation gates
  are blocking
- `GOALS.md` — gate roster (search for `multi-runtime` / `runtime-`)
