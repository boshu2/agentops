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

**Mitigation (documented, partial).** Every round-transition that must wake a
lane goes through `gc session submit`. A city that wants a belt-and-suspenders
keepalive can add a cooldown order that **re-submits** a no-op nudge to a lane
that has been idle past a budget — but the recovery verb it uses **must** be
`gc session submit`, never kill/reset/send-keys:

```toml
# your-city/orders/membrane-lane-keepalive.toml  (illustrative — city-authored)
[order]
description = "Resubmit a nudge to any membrane reviewer lane idle past budget"
trigger = "cooldown"
interval = "5m"
# The body MUST use `gc session submit` — the only verb that drains an idle pane.
exec = "gc session submit agentops-membrane.verifier 'membrane keepalive: reply READY if idle' || true"
idempotent = true
```

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
close gate then correctly DEGRADES (transient lane loss, no attempt consumed),
but the lane is effectively dead until a human clicks through.

**Mitigation (documented setup step).** Pre-trust the hooks **before** the city
runs any codex lane. Either pre-trust the user-global file, or point city codex
sessions at a clean, pre-trusted `CODEX_HOME` so the modal never appears:

```bash
# Option A — pre-trust the user-global hooks file (one-time, per operator box):
#   open codex once interactively and accept the hooks trust prompt, OR
#   ensure ~/.codex/hooks.json is already present + trusted.

# Option B — a clean, pre-trusted CODEX_HOME for city codex sessions
#   (keeps the operator's global codex config untouched):
export CODEX_HOME="$CITY/.gc/codex-home"
mkdir -p "$CODEX_HOME"
# seed a trusted (empty) hooks file so no modal fires:
printf '{}\n' > "$CODEX_HOME/hooks.json"
```

Wire `CODEX_HOME` into the city's codex provider/session environment so every
verifier lane inherits it.

**Honest status: environmental, not a membrane bug.** This is a codex-CLI
startup gate, outside the pack's reach. The mitigation is a **setup step**, not
code the pack can enforce; a city that skips it will see codex lanes DEGRADE
(never a false REFUTE — the finalize contract guarantees that) until the modal
is cleared.

---

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
