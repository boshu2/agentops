---
id: triage-2026-05-01-w1-issue-graph-normalization
type: triage
date: 2026-05-01
issue: soc-o6eb.2
status: accepted
---

# W1 Issue Graph Normalization

## Execution Frame

- **Assumptions:** live `bd` reads are authoritative; the open-issue graph is
  the current 142-issue snapshot from `bd list --status open --limit 0 --json`.
- **Smallest change:** record routing and cleanup decisions, then update only
  W1/W2/W3/W4 tracker notes and obvious closeout evidence.
- **Blast radius:** bd tracker state plus this triage artifact. No source code,
  hook code, generated docs, or implementation epics are edited in this pass.
- **Verification:** duplicate scan, blocker-status scan, orphan routing table,
  `bd dep cycles`, issue readback, bd storage status, git diff check, and final
  worktree-disposition gate.

## Snapshot

| Metric | Count |
|--------|-------|
| Open issues | 142 |
| Ready issues | 87 |
| Open P0/P1/P2/P3/P4 | 1 / 53 / 65 / 20 / 3 |
| Open bug/chore/epic/feature/task | 9 / 6 / 16 / 20 / 91 |
| Unparented non-epics | 34 |
| Unparented P1/P2 non-epics | 22 |
| Unparented P3/P4 non-epics | 11 |
| Missing blocker IDs | 0 |

Block-edge status scan across open issues found 122 `blocks` edges:

| Blocker status | Edge count | Interpretation |
|----------------|------------|----------------|
| open | 74 | normal active sequencing |
| closed | 37 | historical predecessor edges; not missing blockers |
| blocked | 4 | real operator or environment gates |
| deferred | 4 | intentionally not ready |
| in_progress | 3 | W2/W3/W4 blocked by this W1 pass |

## Duplicate And Merge Candidates

Exact normalized duplicate-title scan across open issues returned no duplicate
groups.

Semantic overlap still exists, but none should be merged automatically:

| Candidate | Decision |
|-----------|----------|
| `soc-ygo.38` and `soc-q4c.6` | Same Morai/T3 starvation problem. Keep both for now: `soc-ygo.38` is investigation context under `soc-ygo`; `soc-q4c.6` is the implementation slice under the Dream/OpenClaw migration epic. W3 should close or relate `soc-ygo.38` only after `soc-q4c.6` satisfies the investigation acceptance criteria. |
| `soc-2ctn`, `soc-73tk`, `soc-7wwp`, `soc-xn5s`, `soc-w7s2` | Adjacent close-loop/dry-run/binary-freshness defects, not duplicates. W0 already linked them. Keep `soc-2ctn` as the single P0 incident and keep the P2 beads as scoped follow-ups. |
| `psite-355` and `psite-agu` | `psite-355` is an argv-size mitigation path; `psite-agu` is the structural agentopsd migration path. W3/W4 should decide when `psite-355` becomes superseded by daemon migration proof. |
| `soc-23m2` user-sim cluster | The parent is typed `feature` while its title says `Epic`, and children are represented through `parent-of`/`blocked-by` edges while still showing `.parent == null` in list output. Treat as a graph-shape cleanup candidate for W4, not a W1 mass reparent. |

## Stale Blockers

No blocker IDs are missing.

The only non-open blockers that matter for readiness are below:

| Blocker | Status | Blocked issue | Route |
|---------|--------|---------------|-------|
| `soc-eco` | blocked | `soc-ylb` | Real human-gated Signal schema probe. Do not unblock unattended. |
| `soc-957` | blocked | `soc-1om` | Real T4 metrics/runbook sequencing. Leave blocked until `soc-957` is resolved. |
| `soc-fkt.4` | blocked | `soc-fkt.1`, `soc-fkt.3` | Real operator-safe Windows webhook cutover gate. Route to W4 cross-repo/operator lane. |
| `soc-cos` | deferred | `soc-ygo.26` | Real operator-driven Morai codex cutover/ramp gate. Route to W3/W4, not ready execution. |
| `soc-ygo.33` | deferred | `soc-ygo.35` | Cross-repo/upstream AgentOps compile work. Route to W4. |
| `soc-ygo.8`, `soc-ygo.9` | deferred | `soc-ygo.27` | Bo/human grading and baseline prerequisites. Route to W4. |
| `soc-o6eb.2` | in_progress | `soc-o6eb.3`, `soc-o6eb.4`, `soc-o6eb.5` | This W1 pass clears the portfolio wave gate. |

Closed blocker edges that are worth later cleanup:

- `soc-ylb` still references closed `soc-1dm`, `soc-bpk`, and `soc-wpo`;
  the real remaining blocker is `soc-eco`.
- `soc-4sal` is effectively deferred by its own notes until a release-N+1
  green soak window, but its status is still `open` and its only explicit
  blocker `soc-a43x` is closed. W4 should either set `--defer` or record the
  activation date.
- Several `psite-agu.*` and `psite-mto.*` children retain closed predecessor
  edges. These are normal migration-history edges unless W3 selects that lane.

## Unparented P1/P2 Routing

| Issue | Route |
|-------|-------|
| `psite-agu` | W3 migration lane candidate; likely structural replacement for argv-size pipeline work. |
| `agentops-ikm` | Close during W1 closeout if final `check-worktree-disposition.sh` passes. |
| `soc-ylb` | W4 operator/human-gated T4 lane; blocked by `soc-eco`. |
| `soc-73tk` | W2/W4 local AgentOps dry-run mutation bug; keep related to P0 cluster. |
| `soc-7wwp` | W2/W4 local AgentOps RPI dry-run mutation bug; canonical after W0. |
| `soc-3wh7` | W2/W4 local bd infrastructure hygiene. |
| `soc-1m15` | W4 user-sim closeout; depends on user-sim waves. |
| `soc-gof9` | W4 user-sim Wave 3; do not parallelize CLI-file writers. |
| `soc-ktta` | W4 user-sim Wave 2; runs after Wave 1. |
| `soc-4oue` | W4 user-sim Wave 1; first executable user-sim slice. |
| `soc-23m2` | W4 user-sim parent/feature typed as epic; graph-shape cleanup candidate. |
| `soc-hns4` | W2 local daemon performance slice. |
| `soc-b0eq` | W2/W4 decide close vs downgrade for operator control plane beads. |
| `soc-ey2h` | W4 external/upstream GC coordination. |
| `soc-v7s8` | W2 local eval determinism harness. |
| `soc-5tky` | W4 Dolt security hardening. |
| `soc-w7s2` | W2/W4 local binary distribution policy bug, related to P0. |
| `soc-xn5s` | W2/W4 local close-loop dedup edge-case, related to P0. |
| `soc-hn27` | W4 preserved worktree diff audit. |
| `psite-355` | W3/W4 argv-size mitigation; decide supersedence once daemon path proves out. |
| `soc-gqch` | W3/W4 daemon port binding/dogfood operational defect. |
| `soc-2or` | W4 T4 systemd lane; follows `soc-ylb`/`soc-957`. |

## Deferred P3/P4 Disposition

| Issue | Disposition |
|-------|-------------|
| `soc-bpre` | Keep P3; requires test-contract update before refactor. |
| `soc-vj6x` | Keep P3; infrastructure upgrade, no current portfolio blocker. |
| `agentops-e1k` | Keep P3; side-quest doctrine/SDK posture. |
| `soc-yfwd` | Keep P3; cross-repo bd prefix cleanup. |
| `soc-h0g5` | Keep P3; batch restore only after higher-priority tracker hazards are clear. |
| `psite-lb8` | Keep P3; privacy redaction gate. |
| `soc-wozm` | Keep P3; dogfood command-syntax bug. |
| `soc-1om` | Keep P3; blocked by T4 metrics stability. |
| `soc-jdo` | Keep P4; already deferred dated maintenance. |
| `soc-q39` | Keep P4; laptop-bootstrap modernization. |
| `soc-4bd` | Keep P4; archive deletion after 2026-06-12 grace window. |

## Applied Routing Decision

Do not mass-reparent existing implementation beads in W1. The explicit next
waves are:

1. `soc-o6eb.3` W2: drain active local execution epics and local AgentOps
   hygiene with one selected child at a time.
2. `soc-o6eb.4` W3: select exactly one daemon/Dream migration lane before
   pursuing parallel migration tracks.
3. `soc-o6eb.5` W4: route cross-repo, security, user-sim, Dolt, operator-gated,
   and P3/P4 deferral work.

`agentops-ikm` is the only W1 closure candidate because W0 already recorded the
validator worktree disappearance and the final W1 worktree-disposition gate can
prove the acceptance criteria mechanically.
