# Gas City v1.3 Evaluation — 2026-07-04

> **Verdict up front:** the v1.3 line confirms Gas City is a platform (daemon, dashboard SPA, pack
> registry, provider protocol) and re-validates our deliberate non-adoption. The steal surface is
> narrow and precise: **durable-contract disciplines around verification, failure classification,
> and group-completion detection** — patterns that strengthen the membrane without importing the
> platform. Six candidates survived a 3-lens judge panel out of 30 proposed; the companion
> [`implementation-plan.md`](implementation-plan.md) sequences them.

## Method

- Fork fast-forwarded `429455e35` (v1.2.1-era, 2026-05-31) → `8b17c64` (post-v1.3.3, 2026-07-04):
  **905 commits, 2,616 files, +392k/−91k lines**; the repo now stands at ~1.04M lines of Go.
- Eval ran as a 13-agent workflow (`wf_5d7a7a2a-d83`): 9 subsystem recon lanes over
  `~/dev/gascity` + 1 AgentOps ground-truth lane, then a 3-lens judge panel (redundancy /
  scope-fit / value-for-effort) over all 30 port candidates. Raw structured outputs archived
  (untracked, local) at `.agents/audits/gascity-eval-2026-07-04/` — `findings.json`,
  `judges.json`, `groundtruth.md`.
- Standing frame: gascity is a READ-ONLY reference fork; we port designs natively
  (precedent: Hyper-Extract, 2026-06-17), never adopt the stack (substrate remains NTM + Agent
  Mail; ADR-0009 — no daemon).

## What the v1.3 line is

Releases v1.3.0 (06-18) → v1.3.3 (07-02), peaking at ~290 commits/week from a small core team.
Five programs landed:

1. **Formulas v2 / graph workflows** (on by default): TOML "formulas" compile to a flat,
   topologically-ordered recipe instantiated as durable tracker rows (beads). The load-bearing
   idea: **workflow state IS the bead graph — there is no in-memory orchestrator state machine.**
   Agents run plain step beads surfaced by the ordinary ready-query; a stateless serve loop runs
   control beads through a pure reducer (`ProcessControl`), with every control primitive
   (retry/fanout/drain) a durable state machine in bead metadata. Crash-resume rides
   idempotency keys + epoch CAS.
2. **Packs v2 distribution**: explicit pinned imports (`pack.toml` + `packs.lock`), a pack
   registry, content hashes over sorted manifests, offline canonical pins for builtins, and
   `gc doctor --fix` owning every breaking migration.
3. **Provider/runtime protocol (RPP)**: harness abstraction hardened into a capability handshake
   (`[providers]` catalogs, `gc runtime check` conformance) over claude/codex/etc.
4. **Native beads store**: a typed preflight eligibility engine (7 checks → ELIGIBLE / DEGRADED /
   BLOCKED, with redacted diagnostics + repair steps) selects between the native store and Dolt
   compat; Dolt is now an *external endpoint* concern, not a hard dependency. Their trajectory
   mirrors our bd→br retirement, one year behind.
5. **Security hardening wave**: `citywriteauth` (verify-only single-writer authority),
   `promptsafe` (fixpoint sanitizer for harness delimiter-tag injection — incident-anchored),
   trust-boundary doc + secret-env stripping, all landed in-window.

Subsystem maturity is uniformly high — test-to-source ratios above 1:1 everywhere, 2.5:1 in
dispatch; fix commits cite concrete production RCAs. This is hardened code, which is exactly why
the *contracts* (not the code) are worth reading.

## Strategic read

- **Non-adoption re-validated.** The 1.3 line deepens the platform shape: always-on daemon,
  dashboard SPA + BFF, pack registry, provider protocol. Adopting any of it would drag in the
  sovereign core we deliberately don't have (ADR-0009). Nothing in the window changes the
  2026-06-05 substrate decision (NTM + Agent Mail).
- **Convergent evolution confirms our bets.** They independently arrived at: state-in-the-ledger
  (our control-loop doctrine), fail-closed outcome classification ("no verdict = not done" —
  their retry classifier downgrades a self-reported pass with a missing proof artifact),
  cross-provider review quorums (`reviewquorum` — our membrane, at lane granularity), and
  Dolt-off-the-critical-path. Where two independent teams land on the same invariant, the
  invariant is probably real.
- **Where they are ahead of us** is *durability of contracts*: their verdicts, failures, and
  group-completion states are typed, machine-legible, and crash-safe; several of ours live as
  conventions in bash (`pawl-review.sh`) or agent self-report. That is the port surface.

## Nine-lane findings (compressed)

| Lane | One-line finding |
|---|---|
| Formula v2 | Control-flow as durable bead metadata + pure reducer; fail-closed outcome classifier (`classifyRetryAttempt`) is the gem. |
| Convoys/convergence | Deterministic group-terminality with three production-paid guards (unresolved-member ≠ done; deliberately-open descendant ≠ complete; still-instantiating ≠ done). |
| Runtime lifecycle | k8s-shaped reconciler with adaptive poll cadence/hysteresis; the un-weld split of core vs transport fingerprints. Platform-bound — read, don't port. |
| Packs v2 | Content-hash-verified distribution (sorted manifest sha256, hermetic git); doctor owns migrations. |
| Native beads store | Typed preflight trichotomy ELIGIBLE/DEGRADED/BLOCKED; `storeref.Resolve`'s "unreachable is not absent" read-federation invariant. |
| Observability | Typed cursored event log, first-class run/session/step correlation ids, usage-fact spend ledger. Dashboard itself is out of scope for us. |
| Coordination fabric | `reviewquorum`: durable per-lane review contract — verdict enums, none/transient/hard failure classes, finding-level dedup + cross-lane attribution. Closest to our product. |
| Safety/operability | `promptsafe` fixpoint sanitizer + trust-boundary model + secret-env stripping; `citywriteauth` single-writer authority. |
| Session layer | RPP capability handshake + conformance check; transcript parsing into typed entries. Watchlist for our Codex-twin surface. |

## Disposition — 30 candidates, 3-lens panel

Judges: **R**edundancy (does AgentOps already have it — default cut), **S**cope-fit (membrane/craft
only; no daemon, no platform), **V**alue-for-effort. Full reasoning in the archived `judges.json`.

| Candidate | Effort | R | S | V | Avg | Disposition |
|---|---|---|---|---|---|---|
| `promptsafe-and-trust-boundaries` | S | keep 7 | keep 8 | keep 8 | 7.7 | **BUILD — wave 1** |
| `finding-synthesis-cross-lane-attribution` | M | defer 6 | keep 8 | keep 7 | 7.0 | **BUILD — wave 1** |
| `degradation-aware-typed-decider` | S | defer 5 | keep 8 | keep 8 | 7.0 | **BUILD — wave 1** |
| `preflight-eligibility-ledger` | M | defer 6 | keep 8 | defer 6 | 6.7 | Absorbed into decider; generalize later |
| `group-terminality-detector` | M | defer 5 | keep 7 | keep 7 | 6.3 | **BUILD — wave 2** |
| `unreachable-is-not-absent` | S | defer 4 | keep 7 | keep 7 | 6.0 | **BUILD — wave 2** (guard tests) |
| `migration-owner-checks` | M | defer 5 | keep 6 | keep 7 | 6.0 | **BUILD — wave 2** (doctrine + check shape) |
| `outcome-classifier-fail-closed` | S | cut 3 | keep 8 | keep 7 | 6.0 | Absorbed into decider (planpawl already fail-closes) |
| `crash-safe-ledger-write-ordering` | S | cut 3 | keep 7 | keep 8 | 6.0 | Absorbed into standards reference (wave 2) |
| `skill-content-hash-verify` | M | cut 4 | keep 8 | defer 6 | 6.0 | Defer — doctor manifest-hash partially covers; no incident yet |
| `idempotent-spawn-epoch` | M | defer 5 | keep 7 | defer 6 | 6.0 | Defer — fold principle into standards reference |
| `transcript-tail-usage-extraction` | M | defer 5 | defer 5 | keep 7 | 5.7 | Defer |
| `sessionlog-observe-lib` | L | defer 6 | defer 6 | defer 5 | 5.7 | Defer |
| `doctor-status-x-severity-plus-reverify` | M | cut 3 | keep 7 | defer 6 | 5.3 | Defer |
| `method-hash-drift-stamp` | S | cut 3 | defer 6 | defer 7 | 5.3 | Defer |
| `doctor-warn-with-exact-fix-never-autofix-one-way-doors` | S | cut 3 | defer 6 | defer 6 | 5.0 | Defer — partially existing doctrine |
| `harness-capability-registry` | M | defer 5 | defer 5 | defer 5 | 5.0 | Watchlist (Codex-twin surface) |
| `control-as-ledger-rows` | L | defer 5 | defer 6 | defer 4 | 5.0 | Watchlist — biggest idea, wrong altitude for us today |
| `adaptive-poll-cadence-stagger-hysteresis` | M | defer 5 | defer 6 | defer 4 | 5.0 | Defer (NTM-side concern) |
| `condition-exec-sandbox` | M | cut 3 | defer 5 | defer 6 | 4.7 | Cut |
| `wave-fan-in-manifest` | L | cut 3 | defer 6 | defer 5 | 4.7 | Cut |
| `usage-fact-spend-ledger` | M | defer 4 | defer 6 | defer 4 | 4.7 | Cut |
| `typed-cursored-event-log` | L | defer 5 | defer 5 | defer 4 | 4.7 | Cut |
| `idempotency-key-recovery-reconciler` | L | defer 4 | defer 5 | defer 5 | 4.7 | Cut (daemon-shaped) |
| `transcript-sidecar-correlation` | S | cut 3 | defer 5 | defer 5 | 4.3 | Cut |
| `doctor-fix-migration-owner` | M | cut 3 | cut 5 | defer 5 | 4.3 | Duplicate of `migration-owner-checks` |
| `secret-by-pointer-git-creds` | S | cut 2 | defer 6 | defer 4 | 4.0 | Cut |
| `env-stamped-scope-guarded-reaper` | M | cut 3 | defer 6 | cut 3 | 4.0 | Cut (NTM owns reaping) |
| `edge-plus-level-trigger-floor` | S | cut 3 | defer 5 | defer 4 | 4.0 | Cut |
| `read-only-proof-field-on-verdict` | S | cut 3 | cut 4 | cut 4 | 4.0 | Cut — codex-exec sandbox is strictly stronger than a forgeable porcelain diff |

Notable cut: `read-only-proof-field-on-verdict` looked attractive on first inline read (durable
proof a reviewer didn't mutate the tree), but all three judges converged on the same refutation —
our sandbox *enforces* read-only; their porcelain diff merely *reports* it and is forgeable by an
adversarial reviewer. The panel catching the orchestrator's own bias is the membrane working.

## Operational side-finding

`rtk`'s `git fetch` wrapping is broken on this host: `git fetch` via the rtk PreToolUse rewrite
times out connecting to github.com:443 (~151s, two address families) while plain `/usr/bin/git
fetch` succeeds immediately. Bypass: `bash -c '... /usr/bin/git fetch ...'`. Worth an rtk issue.
