# agentops-membrane

The AgentOps **verification membrane** packaged as a composable Gas City pack.
Stock Gas City is a strong orchestration substrate — store, DAG, worktree
lifecycle, bounded retry, control-dispatcher, provenance, and even a two-lane
cross-family review fan-out (`mol-review-quorum`). The **one** thing it
structurally lacks is a fail-closed, verdict-bound **close door**: its Go
finalizer `reviewquorum.Finalize` has *zero production callers*, so the quorum
verdict is agent-written and an agent can synthesize "pass" over failing lanes.

This pack adds exactly that door and nothing else. It **rides on** stock gc and
rebuilds none of it.

## Import (one line)

The consuming city must already import `core`. Then add:

```toml
# your-city/pack.toml
[imports.agentops-membrane]
source  = "https://github.com/boshu2/agentops/tree/main/packs/agentops-membrane"
version = "sha:<pin>"
```

```bash
gc import install     # resolves + caches the pack
gc doctor             # law0-print-args now active
gc agent list         # agentops-membrane.{planner,builder,verifier,agy-verifier}
gc formula list       # membrane-quest slingable
```

## What each piece encodes (the doctrine)

| Piece | Doctrine it encodes |
|---|---|
| `membrane/finalize.{sh,jq}` | **No verdict = not done.** The deterministic close-door verdict — a faithful port of `reviewquorum.Finalize`'s rollup (`hard > transient > findings > pass`), plus a per-round **nonce** (anti-stale), a **cross-family precondition** (≥2 distinct families), and **degradation-awareness** (transient lane loss ⇒ DEGRADED, never a false REFUTE). Pure, side-effect-free, `bash`+`jq` only — so correctness is unit-proven cheaply (`tests/finalize.bats`). |
| `membrane/close-gate.sh` | The fail-closed door itself. Deterministic pre-gates (branch exists, non-empty diff, contract present) → route **only** the diff + acceptance contract to ≥2 **cross-family, fresh-context** reviewers (LAW 0: never `claude -p`) → deterministic `finalize` → CONFIRMED closes, hard finding REFUTES, transient loss DEGRADES without fabricating judgment. On the fifth failure it creates one disposable breaker-helper session, submits by its unique ID, nonce-validates the outcome, and closes the session; UNSTUCK gets one recovery proof, ESCALATE terminates without another review. **Never merges or pushes — a human merges.** |
| `formulas/membrane-quest.toml` | The build/redo/retry are **native** (worktree isolation + `[steps.check].max_attempts` bounded auto-redo, run by the core control-dispatcher); only the CLOSE is ours. Five ordinary attempts are followed by one helper-guided recovery-proof attempt (`max_attempts = 6`). |
| `doctor/law0-print-args/` | **LAW 0 as structure**, not prose: fails (exit 2, blocking) if any claude- or agy-backed provider carries a live `print_args` (the headless `claude -p` / `--print` billing sink). |
| `agents/{planner,builder,verifier,agy-verifier,breaker-helper}/` | Work/review roles plus the plan-only one-shot breaker advisor, with **harness-level RBAC** and the `VERDICT:` sentinel. Author ≠ judge; judges are a **different family**. |
| `template-fragments/{law0,breaker-escalation,sentinel}.template.md` | DRY: LAW 0, the HOLD → one-helper escalation contract, and the sentinel contract are single-sourced and pulled in via every agent's `append_fragments`, so a downstream city can extend them without forking prompts. |

## Deploy (city.toml — the "how", not the "what")

The pack is deployment-agnostic. In the consuming city's `city.toml`, declare the
verifier lanes so `gc session submit` can deliver, and configure the families:

```toml
[providers.claude]      # LAW 0: kill the print sinks
base = "builtin:claude"
print_args = []
[providers.codex]
base = "builtin:codex"
[providers.antigravity] # AGY = sanctioned Gemini path (never `gemini -p`)
base = "builtin:antigravity"
print_args = []

# reviewer lanes reachable for the close gate (always-on, or a min0/maxK pool)
[[named_session]]
template = "agentops-membrane.verifier"
mode = "always"
[[named_session]]
template = "agentops-membrane.agy-verifier"
mode = "always"
```

The gate's lane targets/families default to `agentops-membrane.verifier` (gpt) and
`agentops-membrane.agy-verifier` (gemini). The breaker helper is not named or
always-on: the gate creates and closes a fresh session for each breaker nonce.
Override via
`MEMBRANE_LANE1_TARGET` / `MEMBRANE_LANE1_FAMILY` / `MEMBRANE_LANE2_*` or
`MEMBRANE_HELPER_TARGET` if your binding differs.

## Run

```bash
gc sling agentops-membrane.builder <quest-bead-id> --on membrane-quest \
  --var quest=<slug> --var task="<build task>"
```

The quest repo lives at `<city>/quests/<slug>` with a protected `main` carrying
`CONTRACT.md` (the reviewers' ruler) and a `test.sh`; evidence is isolated under
`<city>/membrane/<slug>/runs/<workflow-root>/`. On CONFIRMED the source
bead closes with an evidence-bound work record. A fifth failed round enters
HOLD; `close-gate.sh` creates one disposable helper session and submits exactly
ONE-HELPER consultation with the cumulative evidence to its unique ID. UNSTUCK supplies
the sixth and final recovery approach, which must re-earn CONFIRMED. ESCALATE
makes that final attempt terminate before reviewer dispatch, leaving the bead
open for the operator.

<!-- BEGIN planner-intake (age-gc-mvp-w2-nuiw.7) -->
## Planner intake — one-line ask → shaped quest (operating-loop move 1)

Stock gc has no "shape intent as a BDD acceptance contract" step: `mol-scoped-work`'s
submit is *intentionally generic* (self-review only), so the close door has no
default-FAIL ruler to judge against. This pack fills that move-1 gap with an
**activated planner** and a **quest template** that ships as pack content.

```bash
# the mechanical half (deterministic; the planner invokes it):
membrane/scaffold-quest.sh <slug> --ask "<one-line ask>"
#   → copies quests/_template → quests/<slug>/, substitutes {{QUEST}}/{{ASK}},
#     inits a git repo whose `main` carries the skeleton. Fail-closed: BLOCKED
#     (exit 3) on a bad slug or an existing quest — it NEVER edits impl code.
# the judgment half (the planner does this by hand, its ONE authored file):
#   → author quests/<slug>/CONTRACT.md's numbered, default-FAIL Given/When/Then
#     acceptance clauses, then create the quest bead + sling the builder.
```

`quests/_template/` (see its README) is the scaffold: `CONTRACT.md` (the ruler —
numbered `N. [ ]` default-FAIL clauses, read from `main` by the close gate),
`test.sh` (executable harness, red until implemented), and a placeholder
`impl.sh`. On a **malformed/ambiguous** ask the planner emits
`VERDICT: BLOCKED reason=<questions>` rather than guessing a contract.

**RBAC (honest):** the planner's one write surface is scaffolding + authoring
`CONTRACT.md`; it never touches impl code. gc's `permission_mode` is coarse
(no path-scoped allowlist — see §Honest gaps #5), so the planner runs `auto-edit`
and the *enforced* boundary is `scaffold-quest.sh` being the sole write path
(fail-closed on existing quests), not a harness flag. The mechanical half is
unit-proven in `tests/intake.bats` (well-formed scaffold + malformed→BLOCKED);
the live planner-shapes-a-real-ask drill is a separate fitness exercise.
<!-- END planner-intake -->

## Honest gaps (where gc's pack system couldn't express the membrane)

1. **`reviewquorum.Finalize` is un-callable from a pack.** It lives in gascity's
   `internal/` package (Go internal visibility) and gascity is a read-only fork,
   so wiring the real finalizer would need an upstream PR we can't make. We port
   its rollup contract into `finalize.jq` (bash+jq, toolchain-free — the right
   shape for a control-dispatcher `exec` check anyway) and pin parity with unit
   tests. This is the one deliberate divergence.
2. **`mol-review-quorum`'s fixed lane prompts can't carry our per-round nonce,
   custom output path, or cross-family-family RBAC.** Its step descriptions are
   fixed; only lane id/provider/model/target are vars. So we reuse its durable
   `review-quorum.lane.v1` **schema** and dispatch the lanes directly (via the
   native `gc session submit` semantic-delivery verb) rather than slinging the
   core formula. The core formula stays available as optional bound evidence.
3. **Reviewer read-only is self-attested, not sandboxed.** `read_only_enforcement`
   is the reviewer running its own `git status` baseline — `finalize` hard-fails
   on a reported mutation, but true isolation would need a filesystem boundary
   the pack layer can't impose. `permission_mode = "plan"` is the mechanical
   backstop.
4. **Lane-target qualified names depend on the city's chosen import binding.** We
   default to the conventional `agentops-membrane` binding; a city that binds it
   differently must set the `MEMBRANE_LANE*` env overrides. Packs can't read
   their own binding name.
5. **Planner write-scope is coarse, not path-allowlisted.** gc's harness
   `permission_mode` is one of `plan | auto-edit | unrestricted` (gascity
   `internal/worker/builtin/profiles.go`) — there is no "read-only EXCEPT
   `quests/<slug>/`" mode. The planner must write its scaffold, so it runs
   `auto-edit`; the *enforced* RBAC boundary is therefore mechanical, not a
   harness flag: `membrane/scaffold-quest.sh` is the planner's sole write path
   and is fail-closed on an existing quest (never edits impl code). The deny-list
   in the planner prompt is the backstop.

<!-- BEGIN section: city self-verification (age-gc-mvp-w2-nuiw.6) -->
## Self-verification — the membrane applied to itself

Stock gc already ships the housekeeping substrate (20 native orders, `gc doctor`
with 73 checks, `gc events`, dashboard), so this pack does **not** rebuild
stall-detection or notification infra. It adds only the membrane-specific
self-check: *is the fail-closed close door itself installed and sound in the city
that imports the pack?*

| Piece | What it proves |
|---|---|
| `doctor/membrane-health/` | A `gc doctor` check (exit 2 blocking): the close door (`membrane/close-gate.sh`, `finalize.sh`, `finalize.jq`) is present + executable, the `membrane-quest` formula resolves, the trinity agents exist, and **>=2 distinct provider families are configured** — the cross-family precondition the gate requires. A 1-family city can never CONFIRM (`finalize.jq` rejects `fewer_than_two_families`), so it is flagged EARLY here, not on the first close. |
| `scripts/e2e.sh` | The headless **membrane smoke** an operator/CI runs: asserts the two membrane doctor checks (`law0-print-args` + `membrane-health`) are green and the pack `gc lint`-compiles. Structural + doctor-level only — **not** a live agent drill. Exits non-zero on a broken install. |
| `orders/membrane-canary.toml` | A native **cooldown order** (not launchd/cron) that runs the smoke on a schedule (default 30m). A broken close door / missing trinity / sub-2-family city makes the next sweep exit non-zero, surfacing through gc's existing `gc events`/dashboard channel. Structural self-check only — a live through-the-loop quest canary is `age-gc-mvp-w2-nuiw.4`. |

```bash
gc doctor                         # membrane-health + law0-print-args now active
bash scripts/e2e.sh --city <city> # headless membrane smoke; non-zero = broken
gc order list                     # membrane-canary scheduled
```

**Residual zero-nudge gaps** (idle-pane drain → only `gc session submit`
recovers; codex `~/.codex/hooks.json` trust modal → pre-trust in setup) are
documented honestly in [`RESIDUAL-GAPS.md`](RESIDUAL-GAPS.md).
<!-- END section: city self-verification (age-gc-mvp-w2-nuiw.6) -->

## Cost metering (age-gc-adoption-u0he.1)

Sub-backed provider CLIs (claude, codex, agy) emit no usage facts, so `gc
costs` is empty out of the box. Writing `[usage] provider = "local"` into the
city's `city.toml` makes gc populate the run rows (wall time) itself — the
fragment, with the honest limits (token columns stay empty; unpriced models
drop from totals, fail-open — never gate on costs), is single-sourced at
[`template-fragments/usage-local.toml`](template-fragments/usage-local.toml).
`scripts/install-gc-city.sh` applies it automatically.
