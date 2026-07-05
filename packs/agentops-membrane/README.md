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
| `membrane/close-gate.sh` | The fail-closed door itself. Deterministic pre-gates (branch exists, non-empty diff, contract present) → route **only** the diff + acceptance contract to ≥2 **cross-family, fresh-context** reviewers (LAW 0: never `claude -p`) → deterministic `finalize` → CONFIRMED closes, hard finding REFUTES (consume an attempt), transient DEGRADES (retry, **no** attempt consumed). **Never merges or pushes — a human merges.** Writes a `pawl-verdict.v1` artifact per round. |
| `formulas/membrane-quest.toml` | The build/redo/retry are **native** (worktree isolation + `[steps.check].max_attempts` bounded auto-redo, run by the core control-dispatcher); only the CLOSE is ours. |
| `doctor/law0-print-args/` | **LAW 0 as structure**, not prose: fails (exit 2, blocking) if any claude- or agy-backed provider carries a live `print_args` (the headless `claude -p` / `--print` billing sink). |
| `agents/{planner,builder,verifier,agy-verifier}/` | The trinity with **harness-level RBAC** (`option_defaults.permission_mode = "plan"` makes planner/verifier read-only a machine fact; builder keeps write, only in its worktree) and the `VERDICT:` sentinel. Author ≠ judge; judges are a **different family**. |
| `template-fragments/{law0,sentinel}.template.md` | DRY: the two verbatim-identical blocks (LAW 0, the sentinel contract) are single-sourced and pulled in via each agent's `append_fragments`, so a downstream city can extend them without forking prompts. |

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
`agentops-membrane.agy-verifier` (gemini); override via `MEMBRANE_LANE1_TARGET` /
`MEMBRANE_LANE1_FAMILY` / `MEMBRANE_LANE2_*` if your binding differs.

## Run

```bash
gc sling agentops-membrane.builder <quest-bead-id> --on membrane-quest \
  --var quest=<slug> --var task="<build task>"
```

The quest repo lives at `<city>/quests/<slug>` with a protected `main` carrying
`CONTRACT.md` (the reviewers' ruler) and a `test.sh`. On CONFIRMED the source
bead closes with an evidence-bound work record; on exhaustion it **stays open**.

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
