# Quickstart — your first membrane Gas City

Ten minutes from nothing to a Gas City whose work can only close through a
fail-closed, cross-family verification gate. You will bootstrap a city, sling
one canary quest, watch it reach CONFIRMED, and read the verdict artifact.
This is the happy path; [`README.md`](README.md) is the reference.

## Prerequisites

| Need | Check |
|---|---|
| `gc` binary (built from the gascity fork with `make build`, not `go build`) | `gc --help` |
| `bd` exactly matching the beads library your gc build pins in its `go.mod` | `bd --version` |
| `dolt` at or above gc's managed floor (2.1.x line) | `dolt version` |
| `jq`, `git`, `tmux` | `jq --version` |
| ≥2 provider CLIs from **distinct families** (e.g. `claude` + `codex`, ideally `agy` too) | each CLI runs |

A one-family city can never CONFIRM — the gate requires two independent
reviewer families. Install at least two before starting.

## 1. Bootstrap (one command)

From a clone of the agentops repo:

```bash
scripts/install-gc-city.sh ~/my-city
```

This runs the full standup contract: `gc init` with pinned packs, imports
`agentops-membrane`, copies the gate scripts into `<city>/membrane/`, writes
LAW-0-safe provider config (`print_args = []` on claude/antigravity — the
builtin defaults are headless billing sinks), declares the two always-on
verifier lanes, and pre-trusts the codex provider's city-scoped home.
Manual path, if you want to see every step: `skills/using-gc/references/standup.md`.

## 2. Boot and gate on green

```bash
source ~/my-city/env.sh    # sets GC_HOME, PATH shims, the gc wrapper
gc start                   # boots managed dolt, controller, named sessions
gc doctor                  # do not proceed on red
```

Expected in the doctor output:

```
ok  law0-print-args     all claude/antigravity providers print-safe
ok  membrane-health     close door installed; trinity present; >=2 families
ok  dolt-server         reachable
ok  beads-store         store accessible
ok  controller          controller running
```

Warnings on `jsonl-archive`, `formula-requirements`, and `codex-hooks-drift`
are known-benign. Confirm the store is native (not the file fallback):

```bash
gc status --json | jq .beads
# {"beads_store":"NativeDoltStore","native_store_eligible":true}
```

## 3. Sling a canary quest

Scaffold the quest — a default-FAIL acceptance contract plus a red test:

```bash
cd ~/my-city
membrane/scaffold-quest.sh hello --ask "print 'hello, membrane' from impl.sh"
```

Edit `quests/hello/CONTRACT.md`: replace the placeholder clauses with two real
`N. [ ]` Given/When/Then clauses (e.g. "1. [ ] Given the quest repo, when
`./impl.sh` runs, then it prints exactly `hello, membrane` and exits 0").
Keep them default-FAIL — `./quests/hello/test.sh` must exit nonzero right now.

Sling it (this creates the quest bead from the text and routes it):

```bash
gc sling agentops-membrane.builder "canary: hello quest" \
  --on membrane-quest --var quest=hello \
  --var task="implement quests/hello/impl.sh until ./test.sh is green"
```

Expected: the reconciler spawns the builder session on its own tick within
~10–15 s. Nothing to babysit — the control dispatcher advances the workflow.

## 4. Watch it reach CONFIRMED

```bash
gc events --follow      # live stream; the run_id correlates your quest
```

You will see: builder spawn → build in an isolated worktree → the **check
step**, where the dispatcher (never an agent) runs `membrane/close-gate.sh`
and routes only the diff + contract to the two reviewer lanes. Outcomes:

| Disposition | What happens |
|---|---|
| CONFIRMED | quest bead closes with an evidence-bound record; **you merge the branch** |
| REFUTED | a reviewer found a hard defect — builder respawned automatically (five ordinary attempts plus one helper-guided recovery proof) |
| DEGRADED | a lane was transiently lost — retried automatically; never a false refute. Native graph.v2 still consumes an attempt for every failed check. |

A REFUTED→redo→CONFIRMED arc is normal. Never hand-close the quest bead; the
redo path is automatic. A fifth failed round enters HOLD and `close-gate.sh`
creates one disposable helper session, submits exactly ONE-HELPER consultation to its
unique ID, and closes it after the nonce-bound outcome. UNSTUCK gets the sixth recovery attempt
and must re-earn CONFIRMED; HELPER-ESCALATE terminates that attempt without
another review and leaves the bead open for the operator.

## 5. Read the verdict

```bash
find ~/my-city/membrane/hello/runs -name pawl-verdict.json -print
jq . ~/my-city/membrane/hello/runs/<workflow-root>/pawl-verdict.json
```

Check three things: `disposition` is `CONFIRMED`; `refuters[]` shows **two
distinct families** (e.g. `gpt` and `gemini`); every `nonce_echo` matches the
round's nonce file next to it (anti-replay). Each sling has its own workflow-root
directory, so re-slinging a quest cannot reuse a prior verdict. The schema and per-round
artifacts (`pawl-verdict-round-N.json`, `lane-<family>-round-N.json`) are
documented in `skills/gc-membrane/SKILL.md`.

The membrane never merges. The branch in the quest repo is yours to review
and merge — CONFIRMED is the evidence, not the merge.

## Troubleshooting (the five you will actually hit)

| Symptom | Fix |
|---|---|
| codex/agy lane wedged at startup, round DEGRADES forever | provider trust modal — run `scripts/pretrust-codex-home.sh <city>` (codex) or run the provider once interactively (agy) |
| a lane sits idle in `draining`, round never advances | `gc session submit <target> "continue"` — the ONLY verb that works; kill/reset/send-keys all fail |
| `bd context --json` shows `dolt_mode` != `"server"`, or store isn't `NativeDoltStore` | gc fell back to per-op bd calls (perf cliff) — fix the bd/dolt version contract per `references/standup.md` §1 |
| `gc start` fails or hits another city's supervisor | port collision on the default 8372 — set an explicit port in `<GC_HOME>/supervisor.toml` |
| `gc doctor` red on `law0-print-args` | a provider regained its builtin headless print args — set `print_args = []` on every claude/antigravity provider in `city.toml` |

## Where next

- [`README.md`](README.md) — what each pack piece encodes, deploy reference, honest gaps
- [`RESIDUAL-GAPS.md`](RESIDUAL-GAPS.md) — the two known zero-nudge stalls and their mitigations
- `skills/using-gc/SKILL.md` — the day-to-day operator loop (run, admin, troubleshoot)
- Register a real repo with `gc rig add <path>` — quest dirs in the city root are for canaries; rigs are the canonical shape for real work

<!-- tracker: age-gc-adoption-u0he.4 -->
