# Fork-patch surface — gascity is an SDK, not a black box (2026-07-05)

> **Frame correction (Bo).** We had been treating `~/dev/gascity` as a READ-ONLY
> black box ("needs upstream, reimplement in bash"). It is an **SDK** — open Go
> source we build ourselves (`make build` → `bin/gc`, confirmed:
> `Makefile:70` `go build -ldflags … -o $(BUILD_DIR)/$(BINARY) ./cmd/gc`).
> Customizing it = **editing Go**, not lobbying upstream. This doc nails the
> actual patch surface per 5-nudge root gap, the honest fork-maintenance cost,
> and a crisp adopt/skip per gap so the re-plan can convert "document the
> gap / bash workaround" beads into "patch the fork" beads **only where that is
> genuinely better**.
>
> **Read-only research. No product code written; no running city touched.**
> All `file:line` refs are against the checked-out tip `8b17c6403` (branch
> `main`, tracking `origin → github.com/gastownhall/gascity`).

---

## TL;DR (the honest, contrarian read)

The correction is **right that gascity is patchable Go**, but the patches do
**not** move the graduation ruler the way the framing hopes:

- The **thin-patchable** gaps (source_bead_id stamp; `reviewquorum.Finalize`
  wiring) are exactly the ones we **already worked around cheaply** — they buy
  elegance/correctness-in-engine, not fewer nudges.
- The gap that **dominated** the 5-nudge run (idle interactive pane drain /
  no headless verifier lane, **nudges 4 & 5**) is a **LARGE** fork change to
  gc's session runtime — *not* a thin patch, and it lands on gc's **hottest**
  files.
- Two of the three drafted "upstream issues" (`01-reconciler-no-tick`,
  `03-file-backend-bd-coupling`) are about the **`GC_BEADS=file` backend**. The
  membrane city runs **native bd/dolt**, where the `bd` subprocess *works*, so
  those two never bit the membrane run. They are **not membrane gaps** — do not
  spend fork budget on them.

**Bottom line: a thin fork patch set (source_bead_id + Finalize) kills ~0–1 of
the 5 nudges directly.** The 5-nudge graduation is won by the **pack + config
adoption layer (AL.1–AL.3)** — city-materialization of gate scripts (nudge 1),
CODEX_HOME/trust pre-seed (nudge 2), and the session-boundary keepalive (nudges
4/5) — none of which is a gascity Go patch. The fork patches are worth doing for
a *different* reason (remove compensation code; move the close door in-engine),
and only if we accept a gc-**build-lock** (see the Finalize catch below).

---

## The crux the correction under-weighed: `finalize.jq` is NOT a pure port

The premise "we didn't need `finalize.jq`, just wire the real Go
`reviewquorum.Finalize`" is **partly wrong**. `finalize.jq`
(`packs/agentops-membrane/membrane/finalize.jq`) is `Finalize` **plus three
membrane policies stock gc does not have** (its own header says so):

1. **Per-round NONCE** the lane must echo (anti-replay / anti-stale-verdict).
   Stock `Finalize` has no nonce concept.
2. **Cross-family precondition** — REFUTE if the *expected* posture carries
   `< 2` distinct provider families. Stock `Finalize` never counts families.
3. **DEGRADED disposition** — transient lane loss maps to a distinct DEGRADED
   outcome that does **not** consume a redo attempt. Stock `Finalize` returns
   `verdict=blocked / failure_class=transient` and leaves retry semantics to the
   caller.

So "delete `finalize.jq`, use the Go" would ship a **weaker** gate unless we
also **extend** `Finalize` (in gascity `internal/`) with all three. And because
`reviewquorum` is an `internal/` package (Go visibility), the Go is
**un-importable by any external module** — the only way to *call* it is **from
inside a gc build we control**. Net: wiring Finalize is a real option, but it
**couples the membrane's correctness to a forked gc binary** and still needs the
three policy extensions. `finalize.jq` buys **substrate-portability** (runs on
*any* gc as a toolchain-free bash+jq close-gate `exec` check, same posture as the
law0 doctor check); wiring Finalize buys **in-engine determinism** at the price
of build-lock. That trade — portability vs. build-lock — is the actual decision,
not "bash workaround vs. the real thing."

---

## Per-gap patch assessment

### Gap A — `gc.source_bead_id` not stamped on graph-v2 sling → **SMALL**

**Confirmed against source.** The graph-v2 (`isGraph`) branches pass
`sourceBeadID=""` even though `beadID` is in scope:

- `internal/sling/sling_core.go:340` — `InstantiateSlingFormula(…, ""(?), …)`
  the positional `sourceBeadID` after `molecule.Options{…}` is `""`
  (`slingOnFormula`, graph-v2 closure).
- `internal/sling/sling_core.go:332` — `doStartGraphWorkflow(mResult.RootID,
  "", a, method, deps)` — `sourceBeadID` again `""`.
- `internal/sling/sling_core.go:440` / `:444` — `slingDefaultFormula` has the
  identical empty-`sourceBeadID` pair in its graph-v2 branch (verified: line 440
  `InstantiateSlingFormula(…, "", …)`, line 444 `doStartGraphWorkflow(…, "", …)`).
- `internal/sling/sling_core.go:654-667` — the stamp code
  (`SetMetadata(rootID, beadmeta.SourceBeadIDMetadataKey, sourceBeadID)` + the
  `workflow_id` repoint) is **guarded `if sourceBeadID != ""`**, so it exists but
  never fires on this path. The legacy branch (`:366`) already passes `beadID`.

**Change sketch:** pass `beadID` in place of `""` at the four call sites
(`:340`, `:332`, `:440`, `:444`). ~4 lines.

**Risk / not a pure 1-liner:** the graph-v2 path runs
`snapshotGraphV2ReplacementRoot` + `rollbackGraphV2ReplacementLaunch`
(replacement-root semantics the legacy path lacks). The `""` may be deliberate
so a *replacement* launch doesn't double-repoint. So this needs a test proving
(a) the source bead now closes on finalize **and** (b) the replacement/rollback
path is unbroken. **SMALL diff, MEDIUM diligence.**

**Load-bearing?** No. The pack's `close-gate.sh` already **compensates**
(recovers `source_bead_id` from the input-convoy title and stamps the root).
Patching the fork **removes our compensation code**; it does not unblock a nudge.

### `reviewquorum.Finalize` never wired → **MEDIUM (patch) / but see build-lock catch**

**Zero production callers confirmed.**
`grep -rn 'reviewquorum\.Finalize\|\.Finalize(' --include='*.go' | grep -v _test`
returns only `internal/cityinit/service.go:107` — an unrelated
`Initializer.Finalize`. The `reviewquorum` package
(`finalize.go`, `classify.go`, `types.go`) is unit-tested (`finalize_test.go`,
31 KB) and **dead-called by production**. `func Finalize(subject, baseRef
string, outputs []LaneOutput) Summary` — `finalize.go:14`.

**Who consumes the lane JSONs today:** nobody deterministic. The
`mol-review-quorum` formula's third step, `synthesize-review-quorum`
(`internal/bootstrap/packs/core/formulas/mol-review-quorum.toml:151-189`), is an
**agent step** routed to `{{synthesis_target}}`. It *instructs an agent* to read
the two lanes' `gc.output_json` (schema `review-quorum.lane.v1`) and *write* a
`review-quorum.summary.v1`. The formula description states outright (`:183`):
"The `internal/reviewquorum.Finalize` Go finalizer is **not invoked** by this
step yet." So the verdict is whatever the synthesis **agent** writes — no
fail-closed door.

**Where to wire it (sketch):** the lanes already emit exactly the `LaneOutput`
shape — `types.go:67-80` json tags (`lane_id/provider/model/verdict/
findings_count/findings/evidence/read_only_enforcement/mutations_delta/
failure_class/failure_reason`) are 1:1 with `review-quorum.lane.v1`. So a
**deterministic synthesis handler** = read the `needs` lanes' `gc.output_json`
beads → `json.Unmarshal` into `[]LaneOutput` → `reviewquorum.Finalize(subject,
baseRef, outputs)` → marshal the `Summary` into `gc.output_json` → stamp
`gc.outcome` fail-closed from the verdict. That is a **new code path**: either a
new deterministic formula step-*kind* (dispatched by the control-dispatcher like
today's `scope-check`/`retry` control beads, not by an agent) or a
`gc convoy quorum-finalize <root>` verb. ~100–200 LOC in a new file + a
formula/dispatch hook. **MEDIUM.**

**Patch-surface quality: excellent.** `internal/reviewquorum` is **cold** —
**0 commits in the last 30 days** (all files dated May 29). A thin patch here
rebases essentially for free.

**Catch (the crux above):** wiring stock `Finalize` alone gives a **weaker** gate
than `finalize.jq` (loses nonce + ≥2-family precondition + DEGRADED). To match,
extend `Finalize` too — bigger patch, and the result runs **only inside a forked
gc**. Keep `finalize.jq` unless we commit to a forked gc build as the membrane's
sole substrate.

### Gap C — reconciler/consume: idle interactive pane, no headless lane → **LARGE**

**First, disambiguate what Gap C actually was in the 5-nudge run.** The verdict's
Gap C = **nudges 4 & 5** = an idle `claude`/opus builder pane stuck in
`draining`; only `gc session submit` (semantic delivery) advanced it, and the
busy interactive agy verifier pane never consumed the submitted verification
without an esc-interrupt. **The reconciler *did* tick** (it repointed the
session's workdir); the failure was the **interactive-pane delivery/consume
boundary**, not a dead reconciler.

**The drafted upstream issues `01`/`03` are a different gap.** They document the
`GC_BEADS=file` backend where the pool-demand/work/serve queries shell out to a
literal `bd` subprocess that has no DB
(`internal/config/config.go:3511` `bdReadyPoolDemandShell` → `bd ready …`;
`:3945` `EffectivePoolDemandQuery`; `cmd/gc/pool.go:107` `shellScaleCheck` runs
it via `sh -c`; serve loop `cmd/gc/dispatch_runtime.go`). **The membrane city
runs native bd/dolt, so `bd` is present and these queries succeed** — this class
never bit the membrane run. **Not a membrane gap; skip.** (And it would be the
worst possible patch target anyway — see churn.)

**Sizing the real fix (headless verifier lane).** gc's session model is
fundamentally **interactive tmux panes** running provider TUIs
(`internal/session`, `internal/sessionlog`, `cmd/gc/city_runtime.go`), fed by
`gc session submit`. There is **no exec/one-shot/non-interactive provider
concept** in the tree (grep for `oneshot|OneShot|ExecMode|noninteractive`
returns nothing). A true headless lane (spawn `codex exec` / `agy -p`, capture
stdout, no pane to drain) is a **new session runtime type** touching the session
lifecycle, the reconciler's spawn/drain logic, and the provider layer. **LARGE**,
and it lands on gc's **hottest** files (`cmd/gc/city_runtime.go` 36 commits/6wk;
`internal/config` 98/30d). **Skip the fork patch; ride the pack mitigation**
(`gc session submit` everywhere + the `membrane-lane-keepalive.toml` cooldown
order, already in `RESIDUAL-GAPS.md`).

### Gap B — provider trust modal → **NOT CODE**

Confirmed environmental. The agy/codex startup **trust modal** is the provider
CLI's own gate (`~/.codex/hooks.json`), outside gascity. Fix is a **config /
setup step**: pre-trust, or point city sessions at a clean pre-seeded
`CODEX_HOME` (`RESIDUAL-GAPS.md` Gap 2; AL.2). No gascity patch exists to make.

---

## Fork strategy — the honest maintenance reality

### The current fork is not a fork we control

`~/dev/gascity` `origin` → **`github.com/gastownhall/gascity`** directly (not a
`boshu2/*` fork), checked-out `main`, HEAD `8b17c6403`. Fleet policy already
flags it: *"managed FORK of gastownhall/gascity, NO push rights … never
author/thin/push."* So "customize it" requires one of:

- **(a) Stand up our own `boshu2/gascity`** + carry an owned patch set.
- **(b) Local patch branch** we build from, pushed nowhere.
- **(c) Get push to the gastownhall managed fork** (or upstream-PR — the slow
  lobbying path the correction explicitly rejects).

### Rebase-burden evidence — upstream is HOT, but our targets are not

Upstream `main` is **very active: ~658 non-merge commits in the last 30 days**
(23 merges in-window). But **rebase cost is per-file**, and our load-bearing
target is stone-cold:

| Target (patch home) | commits / 30d (from tip) | Rebase burden |
|---|---|---|
| `internal/reviewquorum` (Finalize) | **0** | **negligible — patch floats clean** |
| `cmd/gc/pool.go` | 3 | low |
| `internal/sling` (source_bead_id) | ~19 | occasional conflict; 4-line change trivially re-applied |
| `cmd/gc/city_runtime.go` (headless lane) | ~36 | high |
| `internal/config` (file-backend fixes) | **~98** | **treadmill — avoid** |

A **thin patch set confined to `internal/reviewquorum` (+ the tiny `sling`
change)** rebases cleanly against even this hot upstream. Anything touching
`internal/config` or the session runtime is a maintenance treadmill.

### There is already a fork discipline to slot into

`skills/fork-maintenance` (installed at `~/.claude/skills/fork-maintenance`) is a
**fork-factory**: one-command, conflict-previewed upstream sync that **never
touches `main` by hand**, with divergence facts in `dotfiles/…/FORKS-MAP.md`.
Its triggers explicitly include *"add a fork" / "set up a fork."* Precedents:
`atm`(←ntm), `am`, `br` are all `boshu2/*` re-owns with `origin→upstream` kept
for cherry-picks. The **`fork-apps-pin-the-libs`** doctrine (memory, DECIDED
2026-06-07) says fork the **apps where Bo's opinions live** — and **quorum
semantics / the fail-closed close door is exactly where Bo's opinion lives**.
gascity slots into the identical pattern.

### Recommendation: **(a)**, thin, reviewquorum-first

**Stand up `boshu2/gascity`**, `origin→upstream`, register it in the
fork-factory (FORKS-MAP entry), and carry a **thin patch set**:

1. **Finalize wiring + 3-policy extension** in `internal/reviewquorum`
   (cold file → clean rebases) — the one patch that changes the product surface
   (in-engine deterministic close door), *if* we accept gc-build-lock.
2. **source_bead_id** in `internal/sling` (SMALL, removes our compensation code).

**Not (b):** a local branch works but has no sync discipline and silently drifts
against a 650-commit/month upstream — the exact hornet's-nest the fork-factory
exists to prevent. **Not (c):** no push rights, and upstream-PR is the slow path
the correction rejects.

**Gate before publishing:** gascity is **gastownhall**, not a Jeffrey/FrankenSuite
tool — the `fork-apps-pin-libs` "public publication is courtesy-gated" rule was
written for Jeffrey. **Check gascity's LICENSE** before creating a public
`boshu2/gascity`. Private fork-to-customize (build locally, don't publish) is
almost certainly fine and is enough for (a) mechanically — publish only after the
license check.

---

## The decision table

| Gap (5-nudge root) | Verdict | Where / size | One-line rationale |
|---|---|---|---|
| **A — source_bead_id not stamped (graph-v2)** | **patch-the-fork (nice-to-have)** | `internal/sling/sling_core.go:332/340/440/444`, **SMALL** (4 lines + a replacement-path test) | Cheap, cold-ish file; removes the pack's convoy-title compensation — but a nudge is not blocked on it. |
| **Finalize never wired (fail-closed door)** | **patch-the-fork *only if* committing to a gc-build-lock; else keep the pack** | new deterministic handler over `reviewquorum.Finalize` (`finalize.go:14`); **MEDIUM** + 3-policy extension; `internal/reviewquorum` **cold (0/30d)** | Moves the close door in-engine (deterministic), but `finalize.jq` = Finalize + nonce + ≥2-family + DEGRADED and is substrate-portable; Go is `internal/` → build-locked. Portability vs. determinism is the real call. |
| **C — idle interactive pane / no headless verifier lane (nudges 4/5)** | **pack-workaround (skip fork)** | new session runtime type across `cmd/gc/city_runtime.go` + reconciler + provider; **LARGE**, on the hottest files | Deepest nudge, but a new runtime shape on 36–98 commit/mo files; `gc session submit` + keepalive order already mitigate. |
| **C-adjacent — file-backend `bd` coupling (drafts 01/03)** | **document only (not a membrane gap)** | `internal/config` (~98/30d), `cmd/gc/dispatch_runtime.go` | Only bites `GC_BEADS=file`; the membrane runs native dolt where `bd` works — never hit the run. Worst rebase target in the repo. |
| **B — provider trust modal** | **config** | `CODEX_HOME` pre-seed / pre-trust (AL.2) | Provider-CLI gate, not gascity code — nothing to patch. |

---

## How many of the 5 nudges does a thin patch set actually kill?

- **Nudge 1** (copy gate scripts into the city — control-dispatcher quarantined
  the check step because `gc import` never materializes `membrane/`): **pack**
  fix (install hook) — AL.1/AL.2, **not a fork patch**.
- **Nudge 2** (agy/codex trust modal): **config** (CODEX_HOME) — AL.2, **not a
  fork patch**.
- **Nudge 3** (manual `close-gate.sh` re-fire; gc has no re-fire-a-quarantined-
  step verb): the **Finalize-wiring MEDIUM patch** can absorb this by making the
  close door a deterministic in-engine control step instead of a quarantine-prone
  `exec` check → **~1 nudge, and only if we also extend Finalize**.
- **Nudges 4 & 5** (idle pane drain + non-consuming interactive verifier): the
  **LARGE** headless-lane change — **not in a thin patch set**.

**So: a thin fork patch set (source_bead_id + Finalize) kills at most ~1 of 5
nudges (nudge 3), conditionally.** Nudges 1–2 are the pack/config adoption layer
(AL.1–AL.3); nudges 4–5 need the LARGE session-runtime change we recommend
skipping. **The graduation lever is the adoption layer, not the fork.** The fork
patches are worth carrying for correctness/hygiene (in-engine door;
compensation-code removal) on the one **cold** file — not as the path to zero
nudges.
