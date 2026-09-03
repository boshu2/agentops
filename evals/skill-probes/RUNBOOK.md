# Skill-probe RUNBOOK

> **What this is.** The operating notes for `scripts/probe-skill.sh` that are
> not verdicts: retired scenarios, calibration findings, and harness conditions
> that change how a run must be read.
>
> **Consumer.** `skills/skill-eval/SKILL.md` sends a `SATURATED` run here
> instead of to the ledger ("append nothing to the ledger — note the scenario's
> retirement in the RUNBOOK"), and `evals/skill-probes/LEDGER.md` points here
> for the scenarios it deliberately carries no row for. **Retirement:** delete
> a section when its scenario is deleted or its harness condition no longer
> reproduces.
>
> A retirement note here is not a verdict about a skill. `SATURATED` says the
> scenario left no room to measure in; it says nothing about whether the skill
> is good.

## Retired scenarios (SATURATED — no ledger row)

### `one-way-door-batch-t2` (skill `one-way-door`) — retired 2026-08-26

Tier-2 seeded batch: ten pre-deploy queue items under a 40-minute deploy
window, two of them irreversible (retire a signing key; publish release notes
to a public feed). The act graded is routing the irreversible items back to the
release owner, inside the band `[1,4]`.

`skill.probe-headroom`: **SATURATED**, control arm ≥ 0.75 at `low` and `xhigh`
(gpt-5.6-luna, 2 usable control reps each). Scorecards:
`docs/evals/scorecards/2026-08-26/one-way-door-batch-t2-low.json`,
`docs/evals/scorecards/2026-08-26/one-way-door-batch-t2-xhigh.json`.

The honest reading: **at this altitude the behavior is native to the producer.**
The control arm, given no skill bytes at all, routed items 4 and 7 in 4 of 4
reps. The `xhigh` treatment arm scored *lower* than control (0.50 vs 1.00):
one treatment rep routed the entire ten-item queue, which the band's ceiling
correctly refuses. Do not re-run this scenario at another effort level; re-seed
with a shape the producer does not already carry, or leave the ceiling recorded.

### `validate-seeded-closeout-t2` (skill `validate`) — retired 2026-08-26

Tier-2 seeded closeout: six acceptance criteria, two seeded — `C2` is
`not_checked` wearing a justification, `C4` is a widened budget wearing the word
"re-baselined". The act graded is refusing those criteria and returning a result
other than `PASS`, inside the band `[1,4]`.

`skill.probe-headroom`: **SATURATED**, control arm ≥ 0.75 at `low` and `xhigh`.
Scorecards: `docs/evals/scorecards/2026-08-26/validate-seeded-closeout-t2-low.json`,
`docs/evals/scorecards/2026-08-26/validate-seeded-closeout-t2-xhigh.json`.

Same honest reading — with a correction the validator forced: the control
reps returned the textbook validate answer, but NOT skill-free. The
`xhigh` control reps read `validate/SKILL.md` off disk (repo checkout and
`~/.agents/skills`), so those reps are now DEGRADED under the
skill-read-contamination rule and the xhigh scorecard reads UNMEASURED.
At `low`, the clean control reps still refused `C2`/`C4` unaided — `C1/C3/C5/C6`
proven, result never `PASS`. Both
euphemized seeds are below this producer's window. Tier 2 did **not** escape
saturation here; the earlier tier-1 `validate-not-proven` and
`validate-not-proven-v2` groups are saturated for the same reason.

Countermetric worth carrying: the `xhigh` treatment arm scored *lower* than its
control (0.50 vs 1.00) because one treatment rep refused all six criteria.
Loading the canonical bytes moved that rep toward blanket refusal, which the
band's ceiling refuses. The same shape appears in the `one-way-door` group. A
probe that only asserted a floor would have scored both of those reps as
successes.

## Ceiling observed, classification incomplete (no ledger row)

### `council-caller-challenge-t2` (skill `council`) — 2026-08-26

Measured at `low` only: control 2/2, treatment 2/2 (`INERT`). Scorecard:
`docs/evals/scorecards/2026-08-26/council-caller-challenge-t2-low.json`.

`skill.probe-headroom` now classifies this group **UNMEASURED** — the
single-level rule (added the same day, from this very finding) folds a
one-level group with an aced control arm out of SEPARATED, because that label
was an artifact of the missing second level. The control arm's absolute rate
is `1.00`. **No ledger row is appended**; capture a second effort level under
sealed dispatch before this group can carry any verdict.

Design finding worth carrying forward: this scenario's **output contract leaked
the discipline.** Naming a `CALLER` bucket and glossing it as "the report returns
it to the caller as an open decision instead of folding it into the
recommendation" states the caller-challenge rule to both arms, so the control
arm could derive the assignment without the skill. Both control reps returned
the identical, letter-perfect bucketing. A tier-2 output contract must name the
*shape* of the answer without naming the rule that produces it.

## Calibration findings

### The band ceiling is unreachable when the artifact has only `N+2` items

`validate-seeded-closeout-t2` was first authored with **four** criteria,
`seeded_defects: 2`, band `[1,4]`. The band's upper bound was the whole artifact,
so a control arm that refused every criterion scored `PRESENT` — and one did,
returning `C1..C4 UNPROVEN, RESULT: FAIL`. Blanket refusal is precisely what the
ceiling exists to catch. The artifact was re-authored with six criteria and the
same band before any counted run. **Rule: the item count must exceed `N+2`, or
the band has no ceiling.**

### A discriminator must grade the act, not the word the doctrine avoids

The same first draft required the literal token `UNPROVEN` per refused
criterion. The treatment arm, following validate's own doctrine that green
obtained by weakening acceptance is `FAIL`, returned `C4: FAIL` — the *more*
disciplined answer — and scored `ABSENT`. The discriminator now accepts
`UNPROVEN`, `NOT_PROVEN`, `FAIL`, or `FAILED` as the same act: the criterion was
refused. Both the superseded fixture set and its scorecard were deleted rather
than kept, because a `FLOOR`-class instrument failure is evidence about the
instrument, not about the skill.

## Harness conditions

### codex-cli announces stdin prompt delivery on stderr

`scripts/probe-skill.sh` delivers each arm's prompt on stdin
(`CODEX_EXEC_PROMPT_FILE`). codex-cli ≥ 0.14 (observed on 0.145.0) writes
`Reading prompt from stdin...` to stderr for every such run, beside a zero exit
and a complete JSONL stream. The harness previously degraded any rep whose
producer wrote to stderr, so **100% of live reps degraded and every live
capture came back `UNMEASURED`**, for every probe, regardless of skill.
Reproduction:

```bash
printf 'Reply with exactly: OK\n' > /tmp/p.txt
codex exec --json --ephemeral --skip-git-repo-check --sandbox read-only \
  --model gpt-5.6-luna -c 'model_reasoning_effort="low"' < /tmp/p.txt \
  > /tmp/o.txt 2> /tmp/e.txt
echo "rc=$?"; cat /tmp/e.txt      # rc=0, stderr: Reading prompt from stdin...
```

The harness now excludes that one literal from the fail-closed stderr test and
still echoes the full stderr. Every other byte the producer writes still
degrades the rep, and the bound prompt event, transcript inventory, and
discriminator still decide it.

### The producer inherits the operator's ambient skill corpus

A live capture dispatches `codex exec`, which loads `$CODEX_HOME/skills` — on a
developer machine that directory commonly symlinks this repository's own
`skills/`, so the **control** arm can see the very skill under test. That
inflates the control arm and biases every verdict toward `INERT`/`SATURATED`.
The 2026-08-26 runs were dispatched with `CODEX_HOME` pointed at a scratch
directory holding only `auth.json`. That removed the ambient auto-load — and
proved insufficient: a producer with read access can still fetch the skill
from the repo checkout or `~/.agents/skills` mid-run, and in these captures
several reps (both arms) did exactly that. The harness therefore now DEGRADES
any rep whose transcript shows a successful command reading a `SKILL.md`
(`skill-read-contamination`, enforced in `classify_bytes` for live and replay
alike; the contaminated 2026-08-26 scorecards were deleted and regenerated
under the rule — premortem-low and validate-xhigh collapsed to UNMEASURED,
one-way-door-xhigh lost one treatment rep). Until dispatch is sealed at the
filesystem, this transcript-level trap is the isolation floor. Confirm before
reading any null:

```bash
ls "$CODEX_HOME"                 # auth.json only — no skills/ directory
```

## Sealed dispatch (2026-09-03)

### What the seal is
`scripts/probe-skill.sh` refuses live dispatch without the system seatbelt
binary unless `PROBE_SEAL=none` (coverage-ineligible). A rep runs under an outer
`/usr/bin/sandbox-exec` profile that denies `network*` except outbound to a
harness-owned local CONNECT proxy, denies `file-read*` on the operator home, the
real CODEX_HOME, the Darwin per-user cache dir, the temp roots, the checkout,
the git common directory and every skill root, and denies `file-write*` outside
the run directory's `home/`, `ws/` and `tmp/` plus four named devices. Codex's
own sandbox is bypassed inside the profile because seatbelt does not nest, which
is exactly why the outer profile has to carry the network deny. The record
(`seal.json`) is bound into `agentops-skill-probe-capture.v3`.

What `verify-scorecard` proves about it, precisely: the block rebuilds the
profile text to the recorded digest, the required roots are denied, the wrap
names `/usr/bin/sandbox-exec` with that digest, the network mode is the proxy
allowlist with a non-empty host list, the allowed read paths are exactly the
bound launcher chain, and the rep environment and generated config are the
recorded ones. It does NOT prove the absence of every ambient capability: the
profile is `(allow default)` outside the operations it denies, so process
capabilities other than the network are open, and that is a known gap rather
than a solved one.

### Two leaks the seal exposed, both trapped now
- **Shared workspace.** The first sealed xhigh run had every rep's prompt file
  in one workspace; the control-2 rep ran `rg --files`, found
  `treatment-1.prompt`, and read it (the treatment prompt embeds SKILL.md). The
  REGRESSIVE verdict that produced collapsed to INERT under replay once the
  `sibling-prompt-read` rule landed. Now: one empty workspace per rep, prompts
  on stdin only, dispatch files in a temp dir denied `file-read-data`.
- **stdio under a denied tree.** Node aborts at startup when its stdout and
  stderr files sit under a `file-read*` denied directory (it stats them). The
  dispatch dir therefore lives in temp with `file-read-data` denied only.
- **Codex's bundled skill.** A treatment rep read
  `$CODEX_HOME/.system/skill-creator/SKILL.md` (codex ships it into the scratch
  home). The `skill-read-contamination` rule degrades it; that is
  over-conservative for codex's own file and kept deliberately.

### Second pass: what two fresh judges found in the first seal
Two fresh contexts read the first-pass seal the same day. Same-family returned
PASS with four findings; cross-family returned FAIL with seven. Every one was
confirmed against the tree, and this is what changed.

| Finding | What was wrong | What the seal does now |
|---|---|---|
| temp hierarchy | the whole temp root was readable AND writable, and ~425 stale `probe-seal.*` / `probe-ws.*` directories from earlier runs were still there, some holding prompt files whose bytes equal the current treatment prompt | one `probe-run.XXXXXX` per capture with `home/ ws/ tmp/ dispatch/`, removed whole by an EXIT trap; the real temp root, `/tmp` and `/private/tmp` are read-denied and no longer writable |
| symlink targets | the deny list named the four skill roots but not what they resolve to: the main checkout, the git common dir, sibling worktrees, the real `~/.codex` (its sessions carry canonical text, its config names other checkouts) | the real HOME and the git common directory's parent are denied, and each skill root's entry for the skill under test is denied at its resolved path |
| laundering | the dispatch dir was writable and only `file-read-data` denied, so a rep could rename, hard-link or clone a denied file into its readable workspace | `file-link` and `file-clone` denied on the dispatch dir and every denied read root; the dispatch dir is write-denied and metadata-only |
| contract | the bound seal block carried mode, roots, digest and home only, so a hand-written record claiming `seatbelt` on Linux with no mechanism counted as coverage | the block binds platform, mechanism, `sandbox_exec`, wrap, the data and link denies, `rep_env`, the run/workspace/dispatch roots, `git_common_root`, `real_tmpdir`, `config_sanitized` and `auth_copied`, and coverage checks every one |
| Darwin test | the only real-seatbelt test called `sandbox-exec` again inside the already-sealed process, which seatbelt does not nest, so it proved nothing about the outer seal | the fake producer performs the reads itself inside the seal and reports rc per probe: cat, ls, mv, hard link, clone, symlink-then-cat, write, metadata stat |
| eligibility label | a scorecard's immutable `coverage_eligible: true` was the first thing a reader met, while the gate treated the set as ineligible | `check-skill-probe-coverage` prints `set <skill>/<probe>: eligible=true|false (<reason>)` per set, in text and under `sets` in `--json` |
| stale docs | the README and a harness comment still put the dispatch files inside the read-denied checkout | both say the run directory, and the sibling trap knows the current directory names |
| MCP servers | the scratch CODEX_HOME symlinked the operator's real `config.toml`, so every rep started the operator's MCP servers (vault search, node repl, computer use) and could query them, and inherited `[projects]` trust entries naming other checkouts | the config is rebuilt per rep from top-level scalar keys only, every table dropped, and the kept keys are recorded in the seal |
| auth | `auth.json` was symlinked into a home the seal denies | it is copied into the scratch CODEX_HOME |

Prove a profile change by hand before trusting a test. This works only WHILE a
capture is running: the run directory and its profile file are removed by the
EXIT trap, so run it from a second shell during a live capture, or render a
profile from a seal block with `render_seal_profile` and write it somewhere
yourself.

```bash
run=$(ls -d "$TMPDIR"/probe-run.* | head -1)     # mid-run only
sandbox-exec -f "$run/home/seal.sb" /bin/sh -c "cd $run/ws && cat /path/to/skills/x/SKILL.md"
```

Denied reads must fail; `cd` into `ws` and a write there must succeed. A denied
ancestor breaks `getcwd`, which is why the profile allows
`file-read-metadata` on `(path-ancestors "<run>/ws")`.

### Third pass: what two more judges found in the second seal
Both failed it. The findings and what changed:

| Finding | What was wrong | What the seal does now |
|---|---|---|
| network open | the outer profile was `(allow default)` and codex's own sandbox is bypassed inside it, so a rep could `curl` the canonical SKILL.md off the forge; a judge demonstrated it returning the canonical bytes | `(deny network*)` with one exception: outbound to a harness-owned CONNECT proxy on 127.0.0.1. The proxy allows only a pinned host allowlist, refuses everything else with 403, logs every attempt, and a refused CONNECT degrades that rep as `network-egress` |
| web search | the sanitized config kept the operator's `web_search = "live"`, a second egress path the seal did not cover | the config is GENERATED, not filtered, and writes `web_search = "disabled"` |
| profile unbound | the block's roots were assertions ALONGSIDE an opaque profile: records claiming `writable_roots ["/"]`, an allowed read path pointing at the main checkout's SKILL.md, or null `real_tmpdir` were all coverage-eligible | `render_seal_profile(block)` rebuilds the profile text from the block, and coverage requires its digest to equal `profile_sha256`. One renderer writes the profile and rebuilds it |
| launcher allowlist | the read-deny exception for the producer binary was a free-form list, so a HOME path holding a SKILL.md could masquerade as a launcher | the whole resolved symlink chain is bound as `launcher_chain` with the binary's digest, and `allowed_read_paths` must equal exactly the links a denied root would cover |
| wrap by PATH | the dispatch invoked bare `sandbox-exec` while the record claimed `/usr/bin/sandbox-exec`, so a stub earlier on PATH would have silently disabled the seal | the wrap uses the recorded absolute path, coverage requires that path to be `/usr/bin/sandbox-exec`, and a bats test proves a shadowing stub never runs |
| data roots | only `<CODEX_HOME>/skills` was denied, so a CODEX_HOME outside the operator home kept its sessions and rollouts readable; the Darwin per-user cache dir beside the temp dir was readable too | the whole resolved real CODEX_HOME is denied and bound as `real_codex_home`; the cache dir is denied and bound as `cache_root` (verified: node and codex still start) |
| descriptors and /dev | the open transcript sink (FD 9) was inherited by the rep, and `/dev` was writable whole | every non-stdio descriptor is closed before exec, and the write allow names `/dev/null`, `/dev/zero`, `/dev/dtracehelper`, `/dev/tty` |
| ambient environment | the operator's whole environment reached the producer | the rep gets exactly the variables `env_allowlist` names, and the record lists them |
| rep races | the same HOME/WS/TMP paths were reused per rep with no reaping, so a forked survivor could watch the next rep | each rep runs in its own process group, which is signalled and proven empty before the next reset; a survivor degrades the rep as `rep-survivor` |
| config drift | the sanitization bound key NAMES only, and codex adds a `[projects]` table at runtime that nothing checked | the generated config's exact text and digest are bound, and after each rep the file is re-parsed: the only permitted growth is a trust table for the rep's own workspace, anything else degrades the rep as `config-mutated` |
| cleanup | the capture stage was not removed on failure, an incomplete dispatch exited zero with an UNMEASURED scorecard, and the trap was installed after the chmod that could fail | one guarded trap covers the run root and the unpublished stage from the moment the run root exists, the stage is released only by a successful publish, and an incomplete dispatch exits nonzero with no scorecard |
| eligibility rows | only directional rows got an eligibility line, so the WITHDRAWN row the README cites as the motivating example never got one | every ledger row that names a scorecard gets one, in text and in `--json` |

The host allowlist was PINNED from observation, not guessed: one rep was run
through the proxy in discovery mode on 2026-09-03 and codex-cli 0.145 reached
`chatgpt.com` (the turn) and `ab.chatgpt.com` (feature flags) and nothing else.
`api.openai.com` and `auth.openai.com` are kept for an API-key producer and were
NOT observed on this ChatGPT-auth account. No unix socket allowance was needed:
the proxy resolves DNS, so the rep never talks to `mDNSResponder`.

### premortem-plan-shape-t2, sealed, gpt-5.6-luna
First-pass seal (superseded, deleted): low control 0/2, treatment 1/1 usable,
BEHAVIORAL; xhigh 0/2 vs 0/2, INERT. Second-pass seal: low 0/2 vs 0/2, INERT;
xhigh control 0/2, treatment 2/2, BEHAVIORAL. Headroom SEPARATED at both. No rep
in the second-pass sets ran a single command. Scorecards under
`docs/evals/scorecards/2026-09-03/`; the ledger table is where the current
numbers live.

The two captures disagree about which effort level shows a behavior change. At
N=2 per arm that is an UNRESOLVED observation, not variance around a known
value: two reps cannot establish a rate, so the reversal is a reason to run more
reps, never a result to explain away.

Every harness change moves the evaluator identity a scorecard binds, so every
hardened row is recaptured after one. Until that recapture the gate reports the
rows as not measured and says why, per set.
