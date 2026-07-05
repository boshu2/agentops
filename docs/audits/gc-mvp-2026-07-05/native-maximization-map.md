# Gas City native-maximization map (2026-07-05)

> 8-lane fleet scan (`wf_9683713b-347`, ~908k tokens) over the full gascity v1.3 docs+code surface,
> gapping our current usage (the gc-mvp file-backend city) against the factory ideal under Bo's
> reframe: **maximize gc as designed; agentops composes on top**. Raw lane JSON archived untracked
> in `.agents/audits/gc-max-lanes.json`.

## The headline findings

1. **We hand-rolled a degenerate slice of the formula engine.** Our single-checked-step
   `quest.toml` + `control-pump.sh` re-implement what `[steps.check]`, the control dispatcher,
   scope/`abort_scope`, and `workflow-finalize` already do — and gascity **ships**
   `mol-scoped-work` (full worktree lifecycle: load-context -> setup -> preflight -> implement ->
   self-review -> submit) and **`mol-review-quorum`** (a native two-lane cross-family review +
   synthesis formula — our membrane verifier shape, upstream).
2. **The degradation-aware work (bead .1) is native.** Two lanes independently converged: the gate
   should write `gc.failure_class=transient|hard` + `gc.outcome`, and the engine's own retry
   classification separates infra loss from refutation — no bash reimplementation needed.
3. **RBAC can be harness-level, not prompt-level.** `option_defaults.permission_mode` gives
   read-only planner/verifier and unrestricted-in-worktree builder — enforcement, not vibes.
4. **The cockpit exists.** Typed events with run/session/step correlation ids (a per-quest evidence
   trail), `gc costs` + `.gc/usage.jsonl` (cost-law metering), `gc events --watch` (stall
   detection) — nothing to build.
5. **Gastown (their production city) runs housekeeping as exec-orders** — orphan-sweep,
   spawn-storm-detect, heartbeats. That is our canary/self-verification slice, gc-idiomatically.
6. **Security parallels our own steal**: `execenv` secret-strip + `RedactText` exist natively at
   spawn sites (the same design we ported into agentops promptsafe yesterday).

## Adoption map — every recommendation, ranked


### P1 (16 items)

**Express the quest as a multi-step v2 formula (spec→test→impl→verify) inside a body scope, with the membrane gate as a [steps.check] on the verify step**  _[formulas-workflows]_
- WHY: Serves factory-ideal #1 (multi-step work with acceptance contracts), #3 (fail-closed only-close-door with bounded auto-redo), #4 (control plane durable): the orchestrator's control dispatcher runs check/scope-check/finalize natively, DELETING control-pump.sh.
- HOW: Author formulas/quest.toml with [requires] formula_compiler=">=2.0.0"; steps spec/test/impl inside a gc.kind="scope" body (gc.scope_ref+gc.on_fail="abort_scope" on members); add [steps.check] with max_attempts=N and check.mode="exec", check.path="membrane/quest-gate.sh", check.timeout on the verify step. Delete control-pump.sh (the native dispatcher replaces it) and the single-step quest formula.

**Gate writes gc.failure_class=transient|hard + gc.outcome=fail so native retry does the degradation-aware work**  _[beads-flow-native-ops]_
- WHY: Directly serves the degradation-aware requirement of work bead .1 and factory-ideal #3 (fail-closed, BOUNDED auto-redo) — transient verifier-service loss must never be a false refute.
- HOW: In membrane/quest-gate.sh, split the exit-1 path: when the verifier SERVICE degraded (no verdict JSON returned / infra/timeout/spawn failure) close the iteration bead with `bd update $GC_BEAD_ID --set-metadata gc.outcome=fail --set-metadata gc.failure_class=transient --set-metadata gc.failure_reason=verifier-unavailable --status closed` → native KindRetry re-spawns without counting a refute; when the verifier REFUTED on merits write gc.failure_class=hard. Set gc.max_attempts on the quest root = our round budget so exhaustion leaves the quest bead OPEN (never a spurious close). Reference values in internal/beadmeta/values.go (FailureClassTransient/Hard) and the graph-worker.md close recipe.

**Native claim/close protocol (gc hook --claim --json + bd update --set-metadata --status closed)**  _[beads-flow-native-ops]_
- WHY: Replaces the supervisor-API curls (gc-124) and control-pump.sh workaround; makes workers disposable claim/close units (factory-ideal #4) and puts every close on the typed bead contract instead of a side channel.
- HOW: Rewrite the builder/canary prompts (agents/builder, gc-124) to run `gc hook --claim --drain-ack --json` for work discovery and close via `bd update <id> --status closed`, per internal/bootstrap/packs/core/assets/prompts/graph-worker.md. Delete the http://localhost:8372 supervisor-API self-close. Keep the quest check-step (membrane) as the close DOOR — the agent closes the work bead, the dispatcher's check-step gates the root.

**Evidence-bound close record: gc.work_outcome=shipped + gc.work_commit + gc.work_branch + gc.work_verification (ADR-0009)**  _[beads-flow-native-ops]_
- WHY: Factory-ideal #5 (provenance recorded for every close) — binds bead→artifact→verdict so audit/eval can trace work. We already PRODUCE pawl-verdict.json; we just aren't stamping it on the bead.
- HOW: Have quest-gate.sh, on CONFIRMED, stamp the source quest bead: `bd update <quest> --set-metadata gc.work_outcome=shipped --set-metadata gc.work_commit=$SHA --set-metadata gc.work_branch=$BRANCH --set-metadata gc.work_verification=membrane/<quest>/pawl-verdict.json`. Mirror the mol-do-work.toml close block (internal/bootstrap/packs/core/formulas/mol-do-work.toml). The close gate validates shipped requires a reachable commit — free provenance enforcement.

**gc events per-quest evidence trail (run_id/session_id/step_id correlation)**  _[observability-ops]_
- WHY: Evidence & provenance ideal (5): every close needs a recorded trail; gascity already stamps step_id=work-bead on every event and persists to .gc/events.jsonl — we are re-deriving nothing and capturing nothing.
- HOW: In quest-gate.sh at CONFIRMED, snapshot the correlated trail into the evidence artifact: `$GC_BIN events --city "$CITY" --since "$RUN_START" --json | jq --arg s "$ROOT_ID" 'select(.step_id==$s or .session_id==$s)'` and write it next to the pawl-verdict.v1 artifact. Correlation ids are top-level fields, so jq-filter the JSONL (gc events native filters are --type/--since/--payload-match; run/session/step are top-level). Serves 'no verdict = not done' with a real trail, not just a verdict.

**gc costs + local usage provider (.gc/usage.jsonl per-run token/cost metering)**  _[observability-ops]_
- WHY: Cost-law ideal (6): need per-run generation-vs-gate spend to justify quorum only at one-way doors; today we measure nothing.
- HOW: Set the city usage provider to "local" (config, not exec:/discard so Facts land locally), then during the fitness run capture `gc costs` per quest and roll up by run_id. Fact carries run_id/session_id/step_id + provider/model + input/output/cache tokens + wall_seconds + cost_usd_estimate — join to the same step_id as the evidence trail. Treat cost as decision-support only (list-price, 'unpriced' excluded), never an absolute-dollar gate.

**Native stall/completion detection via typed lifecycle events + gc events --watch**  _[observability-ops]_
- WHY: Unattended-reliability ideal (8): zero operator nudges; replaces control-pump.sh (the workaround being deleted) with the mechanism gascity already emits.
- HOW: Watch `gc events --city . --watch --type session.crashed --type session.idle_killed --type session.cold_start_timeout --timeout <budget>` (or follow the SSE stream) as the stall sensor in the control loop; on fire, re-dispatch or fail the quest. Delete control-pump.sh in favor of this. These are exactly the events gc analyze reliability already tracks.

**Native exec-order housekeeping suite (replace control-pump.sh)**  _[gastown-reference]_
- WHY: Factory-ideal #4 (durable control plane) + #8 (zero-nudge reliability): gascity's controller runs all mechanical loops as config, not as a bespoke pump or an LLM agent — 'No LLM judgment needed, so the controller runs it via exec'.
- HOW: Copy internal/bootstrap/packs/core/orders/{beads-health,gate-sweep,reaper,nudge-on-route,cascade-nudge-on-blocker-close}.toml + their assets/scripts/*.sh into the mvp pack's orders/ + assets/scripts/; keep the cadences (30s health/gate, 30m reaper, event-driven nudges). Delete control-pump.sh. Leave the membrane gate as its own script — this only replaces the plumbing loop.

**Orphan-sweep + spawn-storm-detect + gc bd heartbeat**  _[gastown-reference]_
- WHY: Factory-ideal #8 + #4 (disposable workers, durable plane): gives unattended stall/orphan recovery the trinity currently lacks — dead-agent beads return to the pool, crash-loops escalate instead of silently spinning.
- HOW: Add `gc bd heartbeat "$WORK_BEAD_ID"` to the builder formula before long steps (stamps gc.last_heartbeat_at). Install orphan-sweep.toml (5m) to reset beads assigned to dead agents and spawn-storm-detect.toml (5m) to escalate repeated resets to the planner. Scripts in internal/bootstrap/packs/core/assets/scripts/{orphan-sweep,spawn-storm-detect,escalate}.sh — port the false-orphan live-session guard, it is load-bearing.

**option_defaults.permission_mode="plan" for planner + verifier (harness-level RBAC)**  _[agents-roles-scaling]_
- WHY: Factory ideal #2: roles RBAC-hard, judges never self-grade, planner never edits. Today it's prompt-only; the plan flag makes read-only a harness fact — a claude/codex agent in plan mode literally cannot write.
- HOW: Add to agents/planner/agent.toml and agents/verifier/agent.toml: `option_defaults = { permission_mode = "plan" }` (builder keeps default write mode). Verify the resolved flag with `gc prime <agent>`. Do NOT rely on the provider-level [providers.*].permission_modes map — it is inert (no runtime reads it).

**Native worker pools (min/max_active_sessions + routed_to work_query) replacing control-pump.sh and always-on singletons**  _[agents-roles-scaling]_
- WHY: Factory ideal #4 (workers disposable) + #6 (cheap at generation) + N-builder scale. control-pump.sh is a reimplementation of the native control-dispatcher; always-on builder wastes tokens.
- HOW: Delete control-pump.sh (rely on core control-dispatcher). Give builder `min_active_sessions=0, max_active_sessions=N, idle_timeout="5m", lifecycle="one_shot"` (mirror examples/hyperscale/.../worker/agent.toml). Let sling's default `gc.routed_to=builder` + the 3-tier work_query drive spawn. Keep verifier as the formula check-step (not a chat agent).

**gc.outcome × gc.failure_class tri-state classification in quest-gate.sh (the bead .1 degradation-aware ask)**  _[verification-native]_
- WHY: Serves the fail-closed membrane + cost law: an infra-degraded verifier must NOT burn a REFUTED/rebuild round. This is exactly bead .1's need and gascity already specifies it precisely.
- HOW: Replace quest-gate.sh's binary exit-0/1 with the native failure contract: on verifier unreachable/rate-limited/timeout/no-service, write gc.outcome=fail + gc.failure_class=transient + a reason token from reviewquorum/classify.go's set (rate_limited, provider_unavailable, provider_timeout, transport_interrupted), and RE-RUN the verifier lane; reserve REFUTED (hard) for a real content verdict with findings, and only that re-spawns the builder. Port ClassifyFailure() logic verbatim (internal/reviewquorum/classify.go, ~40 lines) as the gate's classifier. Add gc.on_exhausted=hard_fail so an infra-starved gate hard-fails after N re-runs rather than false-passing.

**Repackage the membrane as a real shareable pack composed by ONE import line**  _[packs-composition]_
- WHY: Factory-ideal: the membrane is the product; a bespoke city can't be distributed or dogfooded onto a stock gascity. Native packs compose with a single `[imports.agentops-membrane]`.
- HOW: Create an `agentops-membrane/` pack dir: `pack.toml` (schema=2, `[imports.core]` pinned by sha, `[[named_session]]` for verifier+mayor), `agents/{planner,builder,verifier}/`, `formulas/quest.toml`, `doctor/`, `template-fragments/`, `assets/scripts/quest-gate.sh`. Replace every absolute `/Users/bo/dev/gc-mvp/...` path with pack-relative (`assets/...`, `{{.ConfigDir}}`, `{{.RigRoot}}`). A stock city then adopts the whole membrane with `gc import add <source> --name agentops-membrane` — one line. Publish via `gc pack registry publish .`; pin consumers by `version = "sha:..."`.

**Hoist shared trinity doctrine into template-fragments + append_fragments (DRY, patch-extensible)**  _[packs-composition]_
- WHY: Factory-ideal RBAC-hard roles + LAW 0 must be identical and single-sourced across planner/builder/verifier; today they're copy-pasted across three prompts. Fragments let a downstream city extend our workers (or us extend core's) WITHOUT forking prompts.
- HOW: Author `template-fragments/law0.template.md`, `evidence-bound.template.md`, `rbac-deny-by-default.template.md` as `{{- define "law0" -}}…{{- end -}}` blocks. In each agent's `agent.toml` set `append_fragments = ["law0","evidence-bound","rbac-deny-by-default"]` (keep prompts `.template.md` — plain `.md` is inert). To layer membrane doctrine onto an imported worker without forking, ship `[[patches.agent]] name="core.<worker>"` with `inject_fragments_append`/`append_fragments`.

**Per-agent permission_mode: read-only planner/verifier, unrestricted builder-in-worktree**  _[security-isolation]_
- WHY: Factory ideal #2 (RBAC-hard roles). Today planner/verifier run --dangerously-bypass-approvals-and-sandbox identical to the builder, so 'planner never edits / verifier never self-grades' is enforced only by prompt text a stochastic agent can ignore. gc gives a mechanical flag that makes the role uneditable.
- HOW: In each non-builder agent config set option_defaults = { permission_mode = "plan" } (planner) and for the codex verifier permission_mode = "suggest" (resolves to --ask-for-approval untrusted --sandbox read-only) or add sandbox = "read-only". Keep builder unrestricted but ONLY inside its worktree. Crucially OVERRIDE the builtin OptionDefaults (profiles.go ships permission_mode: unrestricted for claude/codex/gemini/grok) for read-only roles. Add a doctor check asserting planner/verifier never resolve to unrestricted/bypass flags.

**execenv secret-strip + RedactText on every worker/provider/gate spawn**  _[security-isolation]_
- WHY: Factory ideal #5/#8. A leaked ambient TOKEN/API_KEY into a builder or the membrane gate subprocess is a silent exfil path; trust-boundaries.md treats ambient env as untrusted.
- HOW: Route the file-backend spawn path and membrane/quest-gate.sh invocation through execenv.FilterInherited/MergeMap (strip inherited secrets, re-add only explicit config env) and pipe all logged command output/events through execenv.RedactText. Add a doctor/gate probe that injects a fake FAKE_API_KEY and asserts it appears in neither a child environ nor events.jsonl.


### P2 (15 items)

**Attach the membrane gate via [[advice]]/aspect instead of editing each formula**  _[formulas-workflows]_
- WHY: Serves factory-ideal #3 (membrane is THE close door on ANY workflow) and #6 (cheap at generation, gate centrally): one advice rule bolts the verify/gate step onto every implement step across all formulas without rewriting them.
- HOW: Add an [[advice]] rule targeting "*.implement" (or an aspect formula with [[pointcuts]] matching type/label) whose `after` step is the membrane check/gate. CAVEAT (spec §1.7): advice/pointcuts are DROPPED through `extends` — keep the membrane advice in the leaf formula that is actually cooked, never in a base others extend.

**Adopt mol-review-quorum as the cross-family verifier shape**  _[formulas-workflows]_
- WHY: Serves factory-ideal #2 (verifier is a DIFFERENT model family, judges never self-grade) and #6 (quorum at one-way doors): it is a shipped formula that fans out two read-only reviewer lanes keyed by per-lane provider/model/target vars, then synthesizes — exactly our cross-family verdict.
- HOW: Cook mol-review-quorum (internal/bootstrap/packs/core/formulas/mol-review-quorum.toml) with lane_one/lane_two provider+model set to different families (e.g. one Claude interactive pane, one codex exec — never claude -p, honoring LAW 0 #7). CAVEAT: its synthesis step is agent-executed, NOT wired to the deterministic Go finalizer, so the quorum DECISION is not itself fail-closed — keep a deterministic [steps.check] script as the actual close door and use the quorum output as bound evidence.

**Canary quest as a native Order (cooldown/cron, exec mode); control-dispatcher via gc convoy control --serve as the durable control plane**  _[beads-flow-native-ops]_
- WHY: Factory-ideal #8 (unattended, zero operator nudges) + #4 (durable control plane). control-pump.sh is a self-described workaround; Orders + the serve loop ARE the native mechanism it reimplements.
- HOW: Author quests/canary as orders/canary.toml with `trigger="cooldown" interval="…"` (or cron schedule) and `exec="$PACK_DIR/…/canary.sh"` (mechanical, no agent). Model on internal/bootstrap/packs/core/orders/gate-sweep.toml + beads-health.toml. Delete membrane/control-pump.sh and run the core-pack control-dispatcher agent (gc convoy control --serve --follow) for the loop. Adopt the ~13 core housekeeping orders (reaper, orphan-sweep, prune-branches) for unattended reliability.

**gc analyze reliability --json as the fitness-run + daily-ops crash-rate sensor**  _[observability-ops]_
- WHY: Fitness measurement + evidence: crash-rate per (model, prompt_version, rig) with a real denominator (worker.operation events) is the canonical unattended-reliability metric.
- HOW: Run `gc analyze reliability --json --since <run-window>` at end of the fitness run and in daily ops; store the JSON as a run artifact. Read-only, safe against the live controller. Lets us report reliability per model family (relevant since verifier is a DIFFERENT family).

**Extend custom gc doctor Checks with SeverityBlocking to gate the run**  _[observability-ops]_
- WHY: We already ship one native check (law0-print-args); the Check interface + Blocking-severity exit-code gating is the sanctioned pre-flight guard for control-plane readiness (ideal 4).
- HOW: Add a quest-readiness Check (verifier session reachable, usage provider=local, events log writable) as a doctor Check returning StatusError+SeverityBlocking; run `gc doctor --json` in preflight and fail-closed on blocking. Follow cmd/gc/doctor_codex_hooks.go registration pattern.

**Typed work-record close contract (mol-do-work metadata keys)**  _[gastown-reference]_
- WHY: Factory-ideal #5 (evidence & provenance for every close): a machine-traceable work→artifact→outcome trail beyond a pass/fail exit code, cheap to add.
- HOW: On membrane-PASS close, stamp gc.work_outcome∈{shipped,no-op,blocked,abandoned} + gc.work_commit=<sha> + gc.work_branch + gc.work_verification=<commands> onto the bead, exactly as internal/bootstrap/packs/core/formulas/mol-do-work.toml does. Keep gc.outcome=pass disjoint (control-plane result) from gc.work_outcome (what happened).

**mol-review-quorum durable JSON verdict schema + transient/hard failure classing**  _[gastown-reference]_
- WHY: Factory-ideal #3 + #6: matures our single-exit-code membrane into a structured cross-family verdict (findings, evidence, read-only baseline) and distinguishes transient infra failure from a real refute — directly maps to our DecisionDegraded/exit-5 lane. Quorum only at the close door honors the cost law.
- HOW: Adopt the review-quorum.lane.v1/summary.v1 output keys (verdict∈{pass,pass_with_findings,fail,blocked}, findings[], evidence[], failure_class∈{none,transient,hard}, retry max_attempts=3 on_exhausted) from internal/bootstrap/packs/core/formulas/mol-review-quorum.toml. Set lane_one_provider≠lane_two_provider for cross-family. CRITICAL DIVERGENCE: wire it as the MANDATORY close gate — gascity left it an opt-in scaffold ('lifecycle owners decide when to invoke', synthesis not wired to the Go finalizer); we make it the only door (no verdict = not done).

**Formula fan-out with context="separate" + member_access="exclusive" for N parallel builders**  _[agents-roles-scaling]_
- WHY: Factory ideal #2 (builder edits only its own worktree, no scope collision) + dependency-ordered waves. Native per-unit git worktrees and per-bead reservation eliminate the #1 swarm failure (file collisions) deterministically.
- HOW: For multi-slice epics, model the build as a build-from-convoy-style v2 formula: scatter beads into unit convoys, run the item formula per unit with `[steps.<x>] context="separate"` (own worktree) and `member_access="exclusive"`; keep the membrane `[steps.check]` gate per unit. Reference gascity/formulas/build-from-convoy.formula.toml (already in gc-mvp's pack cache).

**Rigs for the multi-repo factory (routes.jsonl cross-rig routing, <rig>/agent scoping)**  _[agents-roles-scaling]_
- WHY: Factory ideal: one durable control plane over many repos. quests/ subdirs don't give per-repo beads DBs, prefixes, or cross-rig sling targets.
- HOW: When the factory spans repos, `gc rig add <repo>` per repo; make builder/committer `scope="rig"` and mayor/verifier/dogs `scope="city"`. Sling to `<rig>/builder`. Migrate quests/hello to a rig once single-repo proof is banked.

**reviewquorum.Finalize() + LaneOutput durable contract as the multi-lane merge**  _[verification-native]_
- WHY: Serves 'membrane is the only close door', cross-family verdict, evidence-bound, fail-closed. When we go from one verifier to >=2 cross-family lanes, this is the deterministic synthesizer we'd otherwise hand-roll in jq — and it already encodes degraded-coverage-not-false-refute.
- HOW: Emit each verifier lane as LaneOutput JSON (types.go), then port Finalize() (internal/reviewquorum/finalize.go, ~130 lines, 30KB of tests) as a small gc-side or bash-callable finalizer. Get for free: hard-fail on any lane hard failure or read-only violation, BLOCKED/transient on degraded coverage, awaiting_reviewers on no output, and finding dedup+merge with lane attribution. Wrap its Summary into pawl-verdict.v1.

**ReadOnlyEnforcement contract on verifier lanes (baseline/after command proof)**  _[verification-native]_
- WHY: Serves RBAC-hard 'verifier is a different family and NEVER edits' — makes the no-mutation invariant mechanical instead of trusted. Our current quest.toml does not prove the verifier stayed read-only.
- HOW: Have the verifier session record ReadOnlyEnforcement{Enabled,BaselineCommand,AfterCommand,Passed} (a git-status/rev-parse before-and-after diff of its worktree); Finalize already hard-fails the quorum if Passed==false. Adopt the mol-review-quorum.toml read-only lane shape (internal/bootstrap/packs/core/formulas/mol-review-quorum.toml) as the formula skeleton.

**Ship pack doctor checks that enforce the RBAC/membrane invariants deterministically**  _[packs-composition]_
- WHY: Factory-ideal #2 (roles RBAC-hard) + #3 (fail-closed) need a deterministic windshield, not prose. `gc doctor` is the native surface; we already ship law0-print-args and prove the pattern.
- HOW: Add `doctor/verifier-cross-family/run.sh` (fail if resolved verifier provider family == builder family), `doctor/verifier-durable/run.sh` (fail unless verifier is an always-on `[[named_session]]`), `doctor/no-agent-merge-authority/run.sh` (fail if any agent prompt/config grants push/merge). Each gets a `doctor.toml` description; surfaces as `agentops-membrane:<check>`. Wire `gc doctor` into the release gate.

**Make packs.lock + content-hash verification a fail-closed gate sensor**  _[packs-composition]_
- WHY: Factory-ideal #5 (provenance for every close) + #3 (fail-closed). gascity natively pins by sha and can verify a sha256 content-hash of the resolved pack; drift = unproven substrate.
- HOW: Commit `packs.lock`; pin every `[imports.*]` with `version = "sha:..."` (already done for core). Add `gc import check` to the membrane gate so stale/uncached imports fail the run. When publishing our own pack, record its registry content hash and verify it in CI via the `PackContentHash` path (git ls-tree manifest of mode+blob-sha).

**Native andon: emergency spool + gc runtime drain, retire control-pump.sh**  _[security-isolation]_
- WHY: Factory ideal #4/#8 (durable control plane, obvious stop/undo). The andon must fire when the substrate (dolt/beads) is down, which a hand-rolled pump cannot guarantee.
- HOW: Delete membrane/control-pump.sh; use gc runtime drain/undrain/drain-check for graceful wind-down and gc stop for hard halt. Wire the membrane fail-closed verdict to write an emergency record (severity=critical) via the native spool (dolt-independent .gc/emergency/, OS-notify+dedupe) so a BLOCK/escape raises an operator andon even with the tracker offline.

**citywriteauth ed25519 write-grant on the membrane close door**  _[security-isolation]_
- WHY: Factory ideal #2/#3. Makes 'human authorizes the close, judges never self-grade' a crypto invariant: only the key-holder (orchestrator/human, after a CONFIRMED cross-family verdict) can mint a single-use request-bound grant to close a bead / mutate city config; everything else fails closed.
- HOW: Configure a verifying public key on the supervisor so the per-city write middleware installs (fail-closed on missing X-GC-City-Write). Build the minter it deliberately omits: the membrane gate mints a grant ONLY on a CONFIRMED cross-family pawl verdict, binding it to the exact close request. P2 by effort (build minter + verdict-to-grant wiring) but the correct mechanical form of our 'only close door'.


### P3 (6 items)

**Use drain for wave discipline (fan-out over N ready beads)**  _[formulas-workflows]_
- WHY: Serves factory-ideal #1 (dependency-ordered waves) and #5 (per-item evidence): drain scatters an input convoy into unit convoys running an item formula each, with max_units cap and on_item_failure policy — native fan-out-over-N-quests replacing hand-managed waves.
- HOW: Author a wave formula with [steps.drain] { formula="quest", context="separate", max_units=N, on_item_failure="continue" }; requires a targeted invocation (gc sling --on). CAVEAT: ZERO bundled formulas exercise drain (unproven in practice) — validate on a throwaway convoy before trusting it in the epic; defer until the P1 single-quest formula is proven.

**Convoys with track membership + ConvoyFields.target for grouped/waved work**  _[beads-flow-native-ops]_
- WHY: Factory-ideal #1 (dependency-ordered waves): convoys give native grouping, auto-close-on-all-children, and branch inheritance without hand-rolled wave bookkeeping.
- HOW: For multi-bead epics, `gc convoy` create with tracked members (internal/convoy/membership.go) and set ConvoyFields.target so child work beads inherit the branch. Let bead.closed→controller auto-close the convoy. Keep merge=local/mr only.

**max_session_age(+jitter) + demand-scaled verifier pool on the bd/dolt backend**  _[agents-roles-scaling]_
- WHY: Unattended reliability #8 (token-expiry recycling across a fleet without synchronized restarts) and dropping the file-backend always-on-verifier workaround once on a ticking backend.
- HOW: On the native bd/dolt city, set `max_session_age="5h", max_session_age_jitter="15m"` on long-lived agents; convert verifier from `[[named_session]] mode="always"` to a min=0/max=K pool now that the reconciler ticks on sling. Keep the cross-family provider split.

**scope-check / gc.on_fail=abort_scope for fail-closed waves**  _[verification-native]_
- WHY: Serves dependency-ordered waves + fail-closed: a hard failure in one wave member should abort the scope and skip the rest, not let the wave grind on. We have no native wave-abort primitive today.
- HOW: When we run multi-member waves, tag members gc.on_fail=abort_scope and let the native paired scope-check controls (control.go applyAttemptRecipeScopeChecks / runtime.go processScopeCheck) short-circuit the scope on a terminal member failure. Model after mol-scoped-work.toml.

**Use overlay/per-provider ONLY to stage provider config (e.g. kill claude -p print sinks), never as the membrane mechanism**  _[packs-composition]_
- WHY: Overlay JSON-merges provider settings into the agent workdir — the clean native way to enforce LAW 0 config (print_args=[]) and stage provider config across the ~13 supported providers, matching how core does it.
- HOW: Ship `overlay/per-provider/claude/.claude/settings.json` that neutralizes print sinks; `MergeSettingsJSON` folds it into any existing city settings non-destructively. Keep the CLOSE decision in the deterministic gate/doctor, not a runtime overlay hook (see Avoid).

**gitcred secret-by-pointer credentials**  _[security-isolation]_
- WHY: Factory ideal #5/#8. Replace inline env.sh secrets with pointer-based creds that fail closed on loose file perms and never hit argv/logs.
- HOW: Move git/API tokens to a credentials.toml using token_file/token_env/ssh_key_file rules (chmod 600; gitcred rejects group/world-readable). Lets buildimage secret-exclusion and execenv redaction compose — the secret exists only behind a pointer.


## Deliberate non-adoptions (avoid)

- **gc converge (v1) for iterate-until-verified** _[formulas-workflows]_: converge is v1-only and hard-rejects v2 formulas ("convergence wisps do not support v2 formula"); the v2 idiom for iterate-until-verified is the [steps.check] loop. Building our redo loop on converge would tie us to the legacy contract.
- **`until` loops for bounded redo** _[formulas-workflows]_: Spec §4: accepted-but-INERT — the until label is written but no runtime consumer reads it, so an until loop runs exactly ONE iteration. Silent false-redo. Use [steps.check]/retry max_attempts instead.
- **`gate` type vocabulary and `waits_for` all-children/any-children modes** _[formulas-workflows]_: Spec §4: both inert — gate `type` (gh:run, human, mail...) is doc-comment vocabulary with no bundled watcher; waits_for modes have no dispatcher consumer. Zero bundled formulas use either. Do not design group readiness on them.
- **Relying on advice/aspects/compose branch+gate inheriting through `extends`** _[formulas-workflows]_: Spec §1.7 warning: `extends` DROPS advice and pointcuts entirely (incl. the child's own), and compose keeps only bond_points/hooks/expand/map — branch/gate/aspects are dropped from both sides. A membrane attached in a base formula silently vanishes in children.
- **Treating gascity scope/worktree as RBAC-hard role enforcement** _[formulas-workflows]_: mol-scoped-work isolates the builder in a worktree by CONVENTION (git worktree + scope beads), not a permission boundary — nothing stops a mis-routed agent editing elsewhere. Keep our own RBAC hardness (factory-ideal #2); adopt the scope shape for fail-closed semantics, not for role security.
- **gascity's mol-review-quorum / refinery formulas as the verification layer** _[beads-flow-native-ops]_: Its quorum agents are config-supplied and open-world (ZERO-hardcoded-roles) with no model-FAMILY RBAC — a quorum could be same-family self-grading. Our membrane's cross-family verifier is strictly stronger for factory-ideal #2 (verifier is a DIFFERENT model family, judges never self-grade). Adopt the metadata + retry plumbing, NOT their review formula.
- **ConvoyFields.merge="direct" (auto-merge on convoy completion)** _[beads-flow-native-ops]_: Auto-merging on child completion violates factory-ideal #3 (human merges) — the membrane is the only close door and a human merges. Use merge=local or merge=mr only; never direct.
- **Full molecule/wisp/graph.v2 formula machinery beyond our single checked-build quest formula** _[beads-flow-native-ops]_: Huge surface (formula compiler v2, wisp GC, drain/fanout/scope controllers) whose value is multi-step agent pipelines. We deliberately run ONE quest formula whose check-step IS the membrane; expanding into mol-polecat/scoped-work/prompt-synth would dilute the single-close-door invariant without serving any factory-ideal need.
- **Letting gc.routed_to's open-world role vocabulary erase our role separation** _[beads-flow-native-ops]_: gascity forbids hardcoded roles by design, but factory-ideal #2 requires RBAC-HARD planner/orchestrator (never edits) / builder (own worktree) / cross-family verifier. Adopt the routing key mechanism but keep our roles hard-enforced, not config-soft.
- **Dashboard SPA / dashboardbff as the automated evidence/fitness-run surface** _[observability-ops]_: The BFF is a lazy, same-origin human web view (runtailer/rundiff projections) — good for daily human ops, but our evidence & provenance ideal wants deterministic JSONL/CLI artifacts, not scraped browser state. Consume gc events/costs/analyze JSON directly; keep the dashboard as an optional human window, don't build automation against the BFF.
- **Treating gc costs as authoritative billing / gating on absolute dollar thresholds** _[observability-ops]_: cost_usd_estimate is explicitly list-price decision-support; models without pricing are flagged 'unpriced' and dropped from the total. Use it for relative quorum-vs-generation reasoning only, never as a hard-dollar fail gate (would fail-open on unpriced models).
- **exec:/discard usage providers during the fitness run** _[observability-ops]_: Both forward Facts out-of-process or drop them, so gc costs shows nothing local — silently defeats the cost-law adoption above. Keep provider=local to retain the metering lane.
- **Self-reported read-only enforcement as the trust boundary** _[gastown-reference]_: Factory-ideal #2 (RBAC-hard: verifier cannot edit): gascity's read_only_enforcement is the reviewer AGENT running its own `git status --porcelain` baseline and reporting its own delta — self-attestation, not a sandbox. Our verifier isolation must be a real worktree/filesystem boundary, not a prompt asking the agent not to touch files plus trusting its self-report.
- **Refinery auto-merge as the close/land path** _[gastown-reference]_: Factory-ideal #3 (human merges): refinery is an AGENT that merges clean work into the integration branch autonomously — no human in the merge. Keep a refinery-shaped reviewer for prep, but never let an agent be the merge authority.
- **Agent-closes-its-own-bead default (mol-do-work / mol-polecat close)** _[gastown-reference]_: Factory-ideal #3 (judges never self-grade; no verdict = not done): the polecat sets gc.outcome=pass and closes its own work bead — the exact self-grading our membrane forbids. The builder must never hold the close door; only the cross-family verdict closes.
- **Optional/unwired membrane posture** _[gastown-reference]_: gascity's quorum is a scaffold that lifecycle owners choose to invoke and is 'not invoked by this step yet'. Adopting its schema is right; adopting its optionality would make the membrane bypassable, violating fail-closed (#3).
- **Provider-level [providers.*].permission_modes map as an RBAC mechanism** _[agents-roles-scaling]_: internal/config/provider.go:137 documents it as a config-only lookup table for external client dropdowns — "currently no runtime code reads this field." It enforces nothing. Real per-agent capability limiting comes from option_defaults.permission_mode rendering options_schema flag_args, not this map.
- **extmsg / [extmsg.default_route] for agent-to-agent coordination** _[agents-roles-scaling]_: internal/config/extmsg.go is the EXTERNAL messaging fabric (inbound telegram/discord/slack → an agent). It is pure inbound transport, not internal coordination. Agent-to-agent must stay `gc mail` + sling through the store (docs/tutorials/04-communication.md); don't overload extmsg for the trinity handoff.
- **Keeping the builder/verifier as always-on named sessions at factory scale** _[agents-roles-scaling]_: gc-mvp's `[[named_session]] mode="always"` verifier is an explicit file-backend workaround (reconciler doesn't tick on sling), not the target shape. Making workers durable violates factory ideal #4 (workers disposable) and burns tokens idle; only the mayor (control plane) should be always-on.
- **Wiring reviewquorum via mol-review-quorum.toml's agent-executed synthesis step** _[verification-native]_: The formula's own description admits its synthesis step is agent-executed, NOT wired to the Go Finalize. An agent synthesizing lane verdicts re-introduces self-grading/non-determinism at the merge point — the exact thing the membrane exists to prevent. Adopt the deterministic Finalize() instead; this is the one place to diverge from gascity's own (incomplete) wiring.
- **The witness agent as the verifier/close-door** _[verification-native]_: witness (examples/.../witness/agent.toml) is a read-only advisory rig auditor — it reports risks and blockers but has zero verdict authority and no close semantics. Using it as the membrane would violate 'the validation membrane is the only close door.' Keep it (if at all) as a non-gating health/orphan auditor, never as the gate.
- **Replacing the ralph converge loop or its gate outcome set** _[verification-native]_: converge/ralph is already the right bounded-iteration engine and gc-mvp already uses it correctly. Its {pass,fail,timeout,error} vocabulary is fine; the fix is to enrich what the gate SCRIPT classifies (adopt items above), not to swap the loop. Don't rebuild it.
- **Porting gascity's cross-family ROUTING expectation** _[verification-native]_: reviewquorum's LaneConfig carries provider/model fields but gascity has NO primitive that actually routes to a different model family or enforces LAW 0 (no claude -p). That dispatch + the pawl-verdict.v1 artifact + the deterministic pre-gates (branch-exists, non-empty-diff windshield) are genuinely our custom layer — keep them; gascity offers no native replacement.
- **Runtime provider hooks (overlay per-provider hooks.json) as the membrane / close authority** _[packs-composition]_: AgentOps 3.0 is deliberately hookless — the gate is the release authority, not runtime hooks (CLAUDE.md, ADR-0009). Overlay hook staging is fine for provider config, but making the CLOSE decision fire from an injected PreToolUse/session hook re-introduces the hook-dependent enforcement 3.0 removed. Keep close deterministic in `[steps.check]` + `gc doctor`.
- **Baking the supervisor-API curl / file-backend step-completion glue into the portable pack** _[packs-composition]_: gc-mvp's quest formula POSTs to `localhost:8372` supervisor and works around file-backend reconciler gaps (the control-pump.sh being deleted). That's backend-specific city glue, not membrane doctrine — a redistributable pack must be backend-agnostic. Let the engine's native source-chain close fire; don't hard-code a supervisor endpoint.
- **`[[service]]` long-lived HTTP services in the membrane pack** _[packs-composition]_: Rig-level pack loading fails if an imported pack declares a service (pack-spec.md §1.2.6), and a durable HTTP service is a control-plane concern (factory-ideal #4: control plane durable / workers disposable) that belongs in the substrate (NTM + Agent Mail), not smuggled into the membrane pack.
- **Inheriting the builtin OptionDefaults permission_mode: unrestricted for all roles** _[security-isolation]_: profiles.go ships unrestricted as the DEFAULT for claude/codex/gemini/grok; inheriting it silently defeats mechanical RBAC. Only the builder-in-worktree should ever resolve unrestricted; planner/verifier must explicitly override to a read-only mode.
- **Treating gc trust-boundaries as a sandbox for untrusted packs/providers** _[security-isolation]_: trust-boundaries.md is explicit: operator-configured commands, exec: provider scripts, and imported packs are 'a feature, not a sandbox' — trusted code with shell-script review expectations. Do not run unreviewed third-party packs expecting isolation.
- **Containerized/prebaked builders (buildimage) as the near-term default** _[security-isolation]_: For our single-host worktree factory, git worktrees already give sufficient filesystem isolation at far lower ops weight; docker-baked images add build/registry overhead. Reserve buildimage for the multi-tenant / untrusted-pack case (P3), not the current epic.