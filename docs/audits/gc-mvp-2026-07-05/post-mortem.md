# Gas City MVP — epic post-mortem (age-gc-mvp-shy1, 2026-07-05)

> The deliverable is a separate repo (`~/dev/gc-mvp`, its own git line — NOT on agentops main).
> This post-mortem is the agentops-side record. Capstone verdict: `~/dev/gc-mvp/MVP-VERDICT.md`.

## What we set out to do

Bo's directive: use the new gascity **SDK** (v1.3) to *build a Gas City we actually run* — a `gc`
config coexisting with NTM (complement, not swap) — that encodes everything learned: the
software-factory model, the failed Olympus, the validation membrane (cross-family pawl/quorum), the
agent role taxonomy, and the omnigent lessons. This was a clarification of an earlier "port designs,
don't adopt" frame: Bo wants a *running city* on the SDK as a testable MVP, keeping NTM.

## What shipped (all 6 slices closed, RPI: discovery → duel-gated plan → crank → verdict)

- **S0** stopped a live `com.gascity.supervisor` launchd crash-loop (2900+ init failures against a
  broken Dolt-pinned `bo-mac` city) and unregistered it (data preserved).
- **S1** stood up a fresh city under a **dedicated `GC_HOME`** (isolation beats migrating the stale
  1.2.1-era `~/.gc`), `GC_BEADS=file` (no Dolt/bd), workspace provider=codex, `[providers.claude]
  print_args=[]`. gc built from the fork (`make build`, needs `icu4c` CGO).
- **S2** a city-local `doctor/law0-print-args` check (exit-2 blocking) that structurally forbids any
  claude provider from carrying `print_args` — LAW 0 as a gate, tested PASS on the city + FAIL on a
  deliberately-broken copy.
- **S3** the trinity: `planner`/`builder`(claude·opus·interactive-tmux)/`verifier`(codex·diff-only),
  RBAC deny-by-default, sentinel `VERDICT: CONFIRMED|REFUTED|BLOCKED`. Proven with a live sling.
- **S4** the membrane door: a v2 `quest` formula whose build step is gated by a check-script that
  routes **only** diff+contract to the cross-family verifier, fail-closed, REFUTED→bounded redo
  (max 3), pawl-verdict.v1 artifact, human merges. **Live seeded-defect drill: REFUTED (codex caught
  a real missing-`--shout` defect) → auto-redo → CONFIRMED; quest bead closed only on CONFIRMED.**
- **S5** re-scoped (agile re-plan): S4 already delivered the kill-criterion proof, so S5 became the
  honest kill-criterion evaluation + MVP verdict rather than a redundant second quest.

## What went right

- **The membrane door actually works cross-family, against agent interest.** The load-bearing claim
  ("our validation membrane, as a running city's close door") is proven, not asserted — a real codex
  verifier REFUTED a real claude-built defect and the bead would not close until it was fixed.
- **The parallel-worker + orchestrator-verify shape held.** Disjoint write scopes (doctor/ vs
  agents/), each slice a fresh worker, orchestrator diff-read before every close. One PATH landmine
  (three `gc` binaries; hooks resolved a stale 1.0.0) was caught and fixed between slices.
- **LAW 0 survived contact with a platform that ships `claude -p`.** gascity's title-gen and
  `gc prompt` both shell `claude -p`; we neutralized both structurally and gated it.
- **Duel caught a real gap.** The plan-pawl round-1 BLOCKED on a judgment flag (builder-runtime
  question); resolving it by citing Bo's standing card-roles decision (not a fresh model vote) was
  the correct fail-closed behavior, and round 2 PASSed cleanly.

## What was hard / honest gaps

- **gc's file-backend control lane is brittle.** The core control-dispatcher is bd-coupled and dies
  on the file store; the session reconciler doesn't tick after startup. The city needed a custom
  `membrane/control-pump.sh` and verifier-as-named-session to advance, plus 2 controller restarts
  mid-drill. Bead state survived every restart (durability held), but this is **not yet an
  unattended daily driver** — documented, not hidden.
- **v2 DAGs have no back-edge**, so "verify step → redo build step" isn't two steps; the door is one
  checked build step whose *check is the membrane*. A clean adaptation, but worth knowing.
- **S4 was expensive** (~90 min, 336k worker tokens, manual nudges). Autonomous overnight quest runs
  on this engine would stall without an operator.

## Decisions worth keeping

- **Isolation over migration** for legacy tooling (dedicated `GC_HOME` beat `gc doctor --fix`-ing a
  2900-failure city).
- **Structural LAW-0** (a doctor check) over a convention.
- **One verdict vocabulary** (pawl-verdict.v1 dispositions) end-to-end, no WORTHY/UNWORTHY aliases.
- **Anti-Orchestrator-Gravity as a live constraint** — re-scoping S5 to stop-after-proof rather than
  build-another-quest is the exact discipline the failed-Olympus post-mortem demanded.

## Where this leaves us

The MVP goal is **met**: a running gas city that encodes the doctrine and executes the cross-family
membrane as its close door, coexisting cleanly with NTM. Its durable value is the **config + membrane
pattern** (`city.toml` + `doctor/law0-print-args` + `agents/*` + `formulas/quest.toml` +
`membrane/*`), portable to any gc city. Next steps are human-attended (merge the proof branch;
decide bd-backend-vs-keep-pumping if we want it as a real lane) — recorded in `MVP-VERDICT.md`.
Substrate decision unchanged: NTM stays; gc is now a *validated, coexisting experiment lane*, not a
replacement.
