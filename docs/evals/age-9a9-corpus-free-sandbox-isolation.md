# age-9a9 — corpus-free arm isolation: pattern + what's necessary vs. sufficient

> **Contribution from the corpus-delta lane (2026-06-16) to `age-9a9`** (isolate
> scenario-ab arms from the corpus filesystem). Two eval rulers converged on the
> same blocker — *the control arm already has the answer* — via two mechanisms:
> the scenario-ab disk leak (`age-9a9`) and the corpus-delta self-contained-prompt
> leak (`age-rpm`). This doc hands the scenario-ab lane the isolation pattern the
> corpus-delta harness already proved, and — honestly — marks where it is
> **necessary but not sufficient**.

## The leak (age-9a9)
The scenario-ab codex-exec arms run **in the repo cwd** with `--sandbox read-only`.
`read-only` reads the *entire* filesystem by absolute path, so the **control**
(WITHOUT-corpus) arm can `cat /Users/bo/dev/agentops/.agents/...` and answer from
the corpus it's supposed to lack. Proven: the control scored 1.0 on an *invented*
sentinel; `age-707`'s ceiling pre-screen caught it. **cwd isolation alone is
insufficient** — the sandbox's read scope, not the cwd, is what leaks.

## The pattern the corpus-delta lane proved (the behavioral layer)
`corpus-delta-harness.sh` runs each arm in a **corpus-free `/tmp` sandbox**, not the
repo:
- isolated `$HOME` (a `mktemp` dir; user-global `~/.claude` context stripped per arm),
- cwd = a `/tmp` workspace (`-C <sandbox>`), **not** the repo,
- the agent is told *"you are in a project at `<sandbox>`"* — **the repo path is never
  given**,
- the corpus is copied into the WITH arm's sandbox only; the WITHOUT arm's sandbox
  has none.

Verified this session: codex launched in an empty-`$HOME` `/tmp` sandbox **did not**
reach `/Users/bo/dev/agentops/.agents` — it stayed in the sandbox and worked the task
it was given. So moving the arms **out of the repo cwd into a corpus-free `/tmp`
sandbox** removes the worst form of the leak (the agent sitting *inside* the corpus).

## Necessary, NOT sufficient (the honest part)
The `/tmp`-sandbox + withhold-the-path pattern is **behavioral** isolation: it works
because the agent has *no reason or path* to leave the sandbox. It does **not** stop
an agent that **probes absolute paths** — and `age-707`'s invented-sentinel test
demands exactly that adversarial standard. Under `--sandbox read-only`, an agent that
runs `cat /Users/bo/dev/agentops/.agents/...` still succeeds even from a `/tmp` cwd.

So airtight control isolation requires **filesystem-level denial**, ranked by cost:
1. **Read-scoped sandbox** (cheapest if it exists): a codex/sandbox mode that scopes
   *reads* to the workspace, not the whole FS. Verify whether `--sandbox` offers a
   read-confined profile; if so, this is the lightweight airtight fix.
2. **macOS `sandbox-exec` profile** that denies `file-read*` outside the `/tmp`
   workspace (+ system libs) — a local, container-free deny rule.
3. **Container / bind-mount** (most robust, most setup): run the arm in a container
   whose only mount is the workspace; the corpus path does not exist in its FS.

## Recommended `age-9a9` shape
Adopt the corpus-delta pattern as the base (arms in a `/tmp` corpus-free sandbox, repo
path withheld) **and** add filesystem-level read denial (option 1 if a read-scoped
sandbox exists, else 2, else 3). Re-run `age-707`'s invented-sentinel probe as the
acceptance test: the control must score **0** on a sentinel that exists only in the
corpus. Only then is a live scenario-ab verdict valid.

## Consolidation decision (don't maintain two rulers)
- **Carry `scenario-ab` forward** — it has the harder half (OOD holdout prompts +
  4 hardened validity layers). `age-9a9` is its one remaining gate.
- **Harvest from corpus-delta, then retire its task line**: keep `age-t8n`'s
  degraded-honesty machinery (reusable on any agent runner) and this isolation
  pattern; fold `age-rpm` (under-specified tasks + ceiling pre-screen) into the
  scenario-ab holdout-scenario authoring rather than rebuilding a second ruler — the
  cd-* prompts are self-contained (they state the contract) and are the wrong base.
- A valid ruler = **scenario-ab's holdout prompts + corpus-delta's sandbox isolation**.

## Pre-committed stop rule
The validity-hole sequence (cost → retrieval → ceiling → grading → prompt-leak →
disk-leak) is a staircase that can always find one more step. Close `age-9a9`, run
**one** valid A/B, **take the verdict**. A null is the probable, honest outcome
(in-distribution ceiling) and is the signal to pivot to D16 — not to harden a seventh
layer.
