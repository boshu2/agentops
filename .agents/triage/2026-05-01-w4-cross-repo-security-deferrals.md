---
id: triage-2026-05-01-w4-cross-repo-security-deferrals
type: triage
date: 2026-05-01
issue: soc-o6eb.5
status: accepted
---

# W4 Cross-Repo Security And Deferrals

## P1 Security And Cross-Repo

Live `bd show` reads on 2026-05-01 were treated as authoritative. This pass
routes the work only; it does not implement downstream items, mutate bead state,
or mass-reparent existing beads.

| Issue | Owner | Decision | Validation command |
|-------|-------|----------|--------------------|
| `soc-17q` platform-lab cosign verification Audit to Enforce | `Boden.Fuller@gmail.com` | Execute in the platform-lab security lane. This is a real P1 security item under `soc-v64`; do not close from portfolio routing alone. | `cd ~/dev/personal/platform-lab && kubectl apply --dry-run=server -f examples/policy-composition/kyverno/policies/require-image-cosign.yaml && rg 'Enforce|verifyImages|cosign|attestation' examples/policy-composition/kyverno/policies/require-image-cosign.yaml docs/contracts .github/workflows` |
| `soc-5jl` platform-lab sealed-secrets backup and recovery runbook | `Boden.Fuller@gmail.com` | Execute in the platform-lab security lane after choosing the backup custody location. | `cd ~/dev/personal/platform-lab && test -f docs/guides/sealed-secrets-restore.md && rg 'sealed-secrets|backup|restore|rotation|fresh k3d' docs/guides kubernetes/apps/kube-system/sealed-secrets` |
| `soc-e4x` compound-engineering-plugin bun lockfile format | `Boden.Fuller@gmail.com` | Delegate as an upstream PR to the Every Inc repo. It is cross-repo P1 reliability, not AgentOps implementation work. | `cd ~/dev/personal/compound-engineering-plugin && bun install --frozen-lockfile && bun test && git diff --exit-code -- bun.lock bun.lockb package.json` |
| `soc-v64.2` agentopsd design-contract additions before olympus archive | `Boden.Fuller@gmail.com` | Execute as a design-contract lane in AgentOps, then validate from a non-worker session before olympus tag/archive. | `cd ~/dev/personal/agentops && rg 'worker session-id|validator session-id|PASS|WARN|VP1|VP2|VP3|VP4|VP5' .agents/council .agents/design` |
| `psite-355.1` openclaw `--message-file` local patch | `boden.fuller@gmail.com` | Execute only if the shell/openclaw path remains live before daemon migration removes the argv coupling. This is a P1 child of the `psite-355` cross-repo defect. | `/home/boful/.local/bin/openclaw agent --help | rg -- '--message-file' && printf 'a%.0s' {1..200000} > /tmp/psite-355-big.txt && /home/boful/.local/bin/openclaw agent --agent morai-t2 --session-id psite-355-smoke --message-file /tmp/psite-355-big.txt --json; test $? -ne 126` |
| `psite-355.3` morai-enrich consumer uses `--message-file` for large drafts | `boden.fuller@gmail.com` | Execute after `psite-355.1`. This is the local mitigation that proves large drafts stop failing with rc 126. | `test "$(find ~/ops/dream/morai-enrich/results -name '*.prompt' | wc -l)" -eq 0 && ! rg 'Argument list too long' ~/ops/dream/morai-enrich/results` |
| `psite-355.4` re-enqueue YT job and verify reviewed/wiki chain | `boden.fuller@gmail.com` | Execute after `psite-355.2` and `psite-355.3`; close only after bounded entity and synthesis checks are attempted. | `test -f ~/wiki/reviewed/20260429T200300-youtube-building-your-own-software-factory-eric-zakariasson-cursor-rnDm57Py54A.md && rg 'backprop_video_id:.*rnDm57Py54A' ~/wiki/entities` |

Security item `soc-5tky` is P2 by priority but should be treated as security
gated. It requires an operator-supplied `bd_writer` password and rollout-order
sign-off before execution. Validation after rollout: `bd ping` from every wired
workspace plus a Dolt user check that `root` is not reachable as empty-password
remote root.

## P2 Grouping

| Decision | Issues | Route |
|----------|--------|-------|
| Execute | `soc-4oue` | Start user-sim with Wave 1 only. Apply the pre-mortem amendments: tier-specific persona blocks, `ScriptDriver` fail-fast when `persona.SIL == nil`, paired tests, skill count sync, and pre-push gate stub. Do not start Wave 2 until Wave 1 closes. |
| Execute with operator gate | `soc-5tky` | Dolt auth hardening is the right next Dolt security item, but it cannot run unattended because it needs a new secret, 1Password storage, and rollout-order approval. |
| Delegate | `soc-egzh` | Showcase doctrine restoration belongs to an agentops-showcase worker. Keep as a P2 showcase epic; do not absorb it into `soc-o6eb`. |
| Delegate | `soc-hn27` | Preserved worktree diff audit belongs to the existing `brain-2026-04-29-mayor`/release-hygiene lane. Route as a bounded audit with unique diff signatures, cherry-pick/discard decisions, then patch-bundle deletion. |
| Delegate | `soc-gqch` | Port 8765 daemon collision should be handled in the daemon/cell-isolation lane, not as a one-off W4 fix. |
| Defer | `soc-tq42` | Dolt per-repo remote provisioning should wait for `soc-5tky` auth hardening or a tested embedded-to-server conversion recipe. Already migrated databases are evidence, not a reason to continue broad rollout without the auth gate. |
| Defer | `soc-ktta`, `soc-gof9`, `soc-1m15` | These user-sim waves remain ordered behind `soc-4oue`. Wave 2 wraps `RuntimeRunner`; Wave 3 must serialize writers to `cli/cmd/ao/eval_user_sim.go`; closeout runs after Wave 3. |
| Defer or close later | `psite-355`, `psite-355.2`, `psite-355.5` | Keep as the tactical argv-size mitigation unless `psite-agu` proves the daemon path has removed shell `-m "$VAR"` callers. Close as superseded only after that proof exists. |
| Close now | None | W4 has no safe close-only candidate under the no-mutation assignment. |

## Operator-Gated Work

| Issue | Gate | Decision | Validation command |
|-------|------|----------|--------------------|
| `soc-ylb` | `soc-eco` is blocked on Bo/human Signal delivery confirmation. | Defer until a human can confirm the openclaw Signal schema. Then execute T4 consumer with the required pre-flight, retry, STOP-file, and gateway-unavailable branches. | `cd ~/ops/dream/t4-surface && ./consumer.sh --window=24h && bats tests/consumer.bats && systemctl --user start t4-surface-nightly.service && journalctl --user -u openclaw-gateway --since '-60 seconds' | rg 'outbound-signal-delivery|12026426289'` |
| `soc-957` | Blocked by `soc-ylb`; metrics implementation has partial proof but final consumer acceptance is not available. | Keep blocked until T4 consumer is stable, then validate heartbeat and success metrics through node-exporter. | `test -f ~/home-soc/metrics/t4-surface.prom && rg 't4_surface_last_run_timestamp_seconds|t4_surface_last_success_timestamp_seconds' ~/home-soc/metrics/t4-surface.prom && curl -fsS http://127.0.0.1:9100/metrics | rg 't4_surface_'` |
| `soc-2or` | Blocked by `soc-ylb`. | Defer systemd units until the consumer path is accepted. | `systemctl --user list-timers | rg 't4-surface-(nightly|weekly)' && systemctl --user start t4-surface-nightly.service && journalctl --user -u t4-surface-nightly.service --since '-5 minutes'` |
| `soc-fkt.4` | Existing Windows `HomeSocWebhook` owns `127.0.0.1:8000`; replacement is operator-safe cutover work. | Defer until the operator approves restart/replacement of the Windows handler. Do not unregister blindly. | `cd /home/boful/home-soc && go test ./... && bats tests/alert-webhook.bats && curl -s -X POST localhost:8000/analyze-alert -d '{"test":true}' -w '%{http_code}'` |

## P3/P4 Deferrals

| Issue | Priority | Disposition |
|-------|----------|-------------|
| `soc-bpre` | P3 | Explicit next action: choose whether `search.AtomicWriteFile` should auto-create parent directories or keep strict bad-dir failure before re-attempting quest delegation. |
| `soc-vj6x` | P3 | Defer until an operator-approved tmux outage window. Retirement criterion: default tmux server runs linuxbrew 3.6a and bare `tmux a -t bo` works without the `/usr/bin/tmux` alias. |
| `agentops-e1k` | P3 | Defer until 2026-10-30 or until 3-5 third-party AgentOps-shaped plugins exist. Next action then: contracts-first public docs before any SDK posture. |
| `soc-yfwd` | P3 | Explicit next action: inspect `~/cities/bushido` bd config and set the missing issue prefix without disturbing running gc jobs. |
| `soc-h0g5` | P3 | Defer to phased restore. Retirement criterion: pending-quarantined directory is empty and `ao maturity --scan` shows no new degraded learnings from the restore. |
| `psite-lb8` | P3 | Defer until a repo-publication scenario. Retirement criterion: privacy redaction gate exists before visibility change, or repo remains private. |
| `soc-wozm` | P3 | Explicit next action: either correct the brain research command to `--workers <count>` or improve CLI flag UX/help. Retire when stale command syntax no longer appears in research/docs. |
| `soc-1om` | P3 | Defer until `soc-2or` and `soc-957` are accepted. Next action: write the T4 runbook and update MAP/CRITICAL_FACTS. |
| `psite-355.6` | P3 | Defer until local `--message-file` mitigation is proven. Retirement criterion: upstream PR URL is captured, or maintainer rejection is recorded and local patch is declared long-lived. |
| `psite-355.7` | P3 | Defer until the local mitigation is accepted. Retirement criterion: `~/AGENTS.md` and `~/.claude/CLAUDE.md` mention `message-file`, argv limits, and the cited learning. |
| `psite-355.8` | P3 | Defer unless the shell/openclaw path remains live after daemon migration. Retirement criterion: `/vibe` argv-size rule exists with tests, or daemon migration removes the vulnerable caller class. |
| `soc-jdo` | P4 | Already deferred from 2026-04-26. Reopen only if `delivery-queue/failed/` accumulates more than one item in any 14-day window. |
| `soc-q39` | P4 | Defer until the next laptop bootstrap. Explicit next action then: reconcile dotfiles agent entrypoints with current WSL/Windows/Mac addresses. |
| `soc-4bd` | P4 | Defer until 2026-06-12 or later. Retirement criterion: WSL still lives on `D:\WSL\Ubuntu-24.04\ext4.vhdx`, no Track 1 regressions exist, then delete the 157 GB archive. |

## Tracker notes to apply

No tracker mutations were performed by W4. Suggested notes for the lead/operator
to apply later:

- `soc-o6eb.5`: W4 artifact recorded at
  `.agents/triage/2026-05-01-w4-cross-repo-security-deferrals.md`; no bead
  updates, closes, creates, or links were run by the worker.
- `soc-v64`: keep as a cross-repo portfolio parent. Execute P1 children in their
  target repos with validation commands from the W4 artifact; do not crank the
  parent directly.
- `soc-23m2`: execute `soc-4oue` first. Keep `soc-ktta`, `soc-gof9`, and
  `soc-1m15` deferred behind existing dependencies. Do not mass-reparent the
  user-sim graph during portfolio cleanup.
- `soc-5tky`: mark as operator-gated if tracker mutation is allowed later; it
  needs password creation/storage and rollout-order approval before execution.
- `soc-tq42`: defer broad Dolt provisioning until `soc-5tky` or a tested
  embedded-to-server conversion recipe removes the current security and migration
  risks.
- `psite-355`: keep tactical hotfix children open unless `psite-agu` proves the
  daemon path has retired all shell `-m "$VAR"` large-payload callers.
- `soc-ylb`, `soc-957`, `soc-2or`, `soc-fkt.4`: preserve operator gates. Do not
  unblock unattended work that depends on Signal receipt, T4 consumer stability,
  or Windows webhook cutover.
- P3/P4 items: apply explicit defer dates, retirement criteria, or next actions
  from the table above instead of leaving them as ambiguous backlog.

## Discoveries

- `.agents/crank/results/` did not exist in this worktree before W4; it was
  created only to hold the allowed result artifact.
- `psite-agu.9` is closed even though `psite-355` and its children remain open,
  while `psite-agu` acceptance still says `psite-355` should close as
  superseded. Treat this as a tracker-note mismatch, not proof of supersedence.
- `soc-23m2` is typed `feature` while acting as an epic, and its child edges use
  `parent-of`/`blocked-by` while the children still show no `parent`. This
  confirms W1's warning to avoid mass reparenting during W4.
- `soc-957` has partial implementation evidence in comments, but its status is
  still blocked by `soc-ylb`; do not route it as ready execution.
- `soc-5tky` is P2 in priority but security-sensitive in substance because it
  phases out empty-password remote Dolt root access.
