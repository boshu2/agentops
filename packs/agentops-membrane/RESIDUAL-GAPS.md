# agentops-membrane — residual zero-nudge gaps

Stock Gas City already ships the housekeeping substrate this pack rides on: 20
native housekeeping **orders** (reaper, orphan-sweep, gate-sweep, dolt/beads
health, prune-branches, …), `gc doctor` (73 checks), `gc events`, and the
embedded dashboard (out-of-box gap map: `docs/audits/gc-mvp-2026-07-05/`). This
pack deliberately does **not** rebuild any of that.

Two residual stalls stand between "membrane installed" and fully unattended
operation. Both are **session/provider-boundary** issues, not membrane logic
bugs, and both are honestly documented here rather than papered over. Neither is
fully solved in-pack; where the real fix is upstream, that is stated.

---

## Gap 1 — idle-interactive-pane drain (the `gc session submit` recovery verb)

**Symptom.** An idle `claude`/opus builder pane that has entered `draining` is
**not** recoverable by `gc session kill`, `gc session reset`, or a raw
`tmux send-keys` Enter. Reproduced in the out-of-box exercise (`.9`) and again
here. The pane sits there; the round never advances.

**The one verb that works.** `gc session submit <target> <text>` — gc's native
*semantic-delivery* verb — is the **only** action observed to advance a draining
pane. This is exactly why `membrane/close-gate.sh` dispatches its review lanes
with `gc session submit` and nothing else (see its `submit_lane`).

**Mitigation (SHIPPED — age-gc-adoption-u0he.2).** Every round-transition that
must wake a lane goes through `gc session submit`. The belt-and-suspenders
keepalive is now real pack content, registered on import:
[`orders/membrane-lane-keepalive.toml`](orders/membrane-lane-keepalive.toml) →
[`scripts/lane-keepalive.sh`](scripts/lane-keepalive.sh) — a cooldown order
(5m) that re-submits a no-op nudge to a membrane LANE that is `draining` or
awake-but-inactive past budget (`MEMBRANE_IDLE_BUDGET_S`, default 300s).
Conservative by contract: lanes only (never builders/dispatcher), busy lanes
left alone, and the recovery verb is `gc session submit` and nothing else —
never kill/reset/send-keys.

**Honest status: NOT fully solved.** The durable fix is upstream in gascity
(read-only fork here): either the control-dispatcher issues a `session submit`
on iteration spawn, or the builder runs as a headless/`exec` target instead of
an interactive pane so there is no pane to drain. Until then, the membrane's own
lanes are safe (they always submit), but arbitrary idle panes in the city need
the keepalive above.

---

## Gap 2 — codex verifier startup trust modal (`~/.codex/hooks.json`)

**Symptom.** A verifier `codex` session can wedge at startup on the user-global
`~/.codex/hooks.json` **trust modal** — codex blocks awaiting interactive
confirmation, and the lane never produces its `review-quorum.lane.v1` JSON. The
close gate then correctly DEGRADES without fabricating a semantic verdict;
native graph.v2 still consumes that failed check attempt,
but the lane is effectively dead until a human clicks through.

**Mitigation (SHIPPED setup step — age-gc-adoption-u0he.2).** Pre-trust the
hooks **before** the city runs any codex lane:
[`scripts/pretrust-codex-home.sh <city>`](scripts/pretrust-codex-home.sh)
seeds a clean, city-scoped `CODEX_HOME` (`<city>/.gc/codex-home` with a
trusted-empty `hooks.json` — the operator's global codex config stays
untouched) and prints the exact `[providers.codex] env = { CODEX_HOME = … }`
stanza to wire it. `install-gc-city.sh` runs this automatically. agy has no
file seed — run the provider once interactively and accept its prompt.

**Honest status: environmental, not a membrane bug.** This is a codex-CLI
startup gate, outside the pack's reach. The mitigation is a **setup step**, not
code the pack can enforce; a city that skips it will see codex lanes DEGRADE
(never a false REFUTE — the finalize contract guarantees that) until the modal
is cleared.

---

## Costs / usage facts — the honest state (not a pack gap)

`gc costs` populates run rows (wall time) out of the box: the usage sink is
default-on (`[usage] provider=""`/`"local"` → `.gc/usage.jsonl`; verified in
`internal/config/config.go` UsageConfig). What stays EMPTY is the token /
invocation columns — **sub-backed provider CLIs (claude, codex, agy) emit no
usage facts to gc**, so there is nothing to sink. That is a provider-side gap
the pack cannot fix; no `[usage]` config change helps. Consequence (also in
the `using-gc` skill): treat `gc costs` as decision-support-when-populated
and NEVER gate on it — unpriced models drop from totals (fail-open).

## Diff-frame false-positive — FIXED in the close gate

The fitness run's bonus finding (a weak lane REFUTED a correctly-placed file
because the review diff is quest-repo-relative while the contract non-goal
was city-relative) is closed at the source: `membrane/close-gate.sh`'s review
request now carries an explicit PATH FRAME paragraph — placement findings
require a path wrong in BOTH frames.

## What the pack DOES ship for self-monitoring

- `doctor/membrane-health/` — a `gc doctor` check: close door installed +
  executable, trinity present, and **>=2 provider families configured** (flags
  the 1-family "can never CONFIRM" city early).
- `scripts/e2e.sh` — the headless structural membrane smoke (both membrane
  doctor checks green + `gc lint` compile) for an operator or CI.
- `orders/membrane-canary.toml` — a native cooldown **order** that runs the
  smoke on a schedule so the city self-monitors the membrane through gc's own
  events/dashboard. (Structural sweep only — a live through-the-loop quest drill
  is `age-gc-mvp-w2-nuiw.4`, out of scope here.)
