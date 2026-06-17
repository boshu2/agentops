# AgentOps Docs Staleness Sweep — Synthesis Report

## 1. Executive Summary

This sweep reviewed **116 live doctrine docs** across 10 lanes and surfaced **97 findings** (after deduping near-identical line-level hits within a file). The findings cluster into four dominant stale themes, in order of blast radius: (1) **bd/Dolt presented as the current tracker** — by far the largest theme — when bd/Dolt was retired 2026-06-11 in favor of `br` (beads_rust) at `_beads/`, invoked `BEADS_DIR=$PWD/_beads br`; (2) **Gas City / gastown / `ao rpi` framed as the live out-of-session substrate** when gc was removed from the CLI and the substrate is now NTM + MCP Agent Mail; (3) **hooks and SessionStart auto-injection described as current enforcement** when 3.0 is hookless (explicit `ao session bootstrap` / `ao inject` replaced them); and (4) **"CI is the authoritative gate" / branch-protection-blocks-main** contradicting the push-to-main model where the local pre-push Go gate (`ao gate check`) is the release authority. The highest-risk items are onboarding/first-value paths and troubleshooting runbooks that would steer a new agent into running a retired binary or believing local pushes are unguarded.

## 2. P1 — Would Actively Mislead an Agent

| File | Line | Stale claim | Fix |
|---|---|---|---|
| docs/agentops-3-explainer-kit.md | 58 | `bd create ...` in the live 5-command onboarding path | `BEADS_DIR=$PWD/_beads br create "..." --body "..."` |
| docs/architecture/the-agent-factory.md | 58 | bd/Dolt = decided/current state store; `bd ready` = live scheduler queue | br (git-JSONL) + `br ready`, or annotate bd/Dolt superseded |
| docs/examples/agentops-3-council-demo-storyboard.md | 14 | Out-of-session loop dispatched on the Gas City reference City | NTM + MCP Agent Mail substrate |
| docs/examples/agentops-3-council-demo-storyboard.md | 31 | `bd show` / `bd ready` in demo pre-flight | `BEADS_DIR=$PWD/_beads br show/ready` |
| docs/examples/agentops-3-council-demo-storyboard.md | 153 | `bd show` in demo Act 4 | `BEADS_DIR=$PWD/_beads br show <id>` |
| docs/examples/agentops-3-council-demo-storyboard.md | 181 | Gas City cron Orders + `ao rpi` + using-gc skill as recommended loop | NTM + Agent Mail dispatch of operating loop; drop `ao rpi`/using-gc |
| docs/examples/agentops-3-domain-practice-packet.md | 52 | `bd create` as current tracked-follow-up command | `BEADS_DIR=$PWD/_beads br create ...` |
| docs/examples/agentops-3-domain-practice-packet.md | 89 | Gas City cron Orders / using-gc as current out-of-session lane | NTM + Agent Mail; drop using-gc + `ao rpi` |
| docs/comparisons/vs-tons-of-skills.md | 64 | Gas City reference City + `ao rpi` for unattended run | Operating loop on NTM + MCP Agent Mail (no gc, no `ao rpi`) |
| docs/contracts/remote-compute.md | 8 | Contract freezes nouns for `ao compute`, daemon jobs, GasCity sessions | Mark superseded by ADR-0009 + gc-bridge removal; substrate is NTM + Agent Mail |
| docs/contracts/remote-compute.md | 138 | "GasCity First — product remote execution path is GasCity API/SSE" | Retire section; out-of-session path is NTM + Agent Mail |
| docs/first-value-path.md | 34 | `bd show <issue-id>` in first-value onboarding | `BEADS_DIR=$PWD/_beads br show <issue-id>` |
| docs/first-value-path.md | 143 | `bd create ...` block (shows old `soc-` id) | `BEADS_DIR=$PWD/_beads br create ...`; update id/output |
| docs/first-value-path.md | 174 | NTM swarm runs `bd ready` then `ao rpi <bead>` | `BEADS_DIR=$PWD/_beads br ready` to match canonical-loop-model.md |
| docs/first-value-path.md | 188 | `.beads/issues.jsonl` = tracked work artifact | `_beads/issues.jsonl` (br ledger) |
| docs/leverage-points.md | 115 | Push gate = CI + branch protection blocks merge to main | Local pre-push Go gate blocks push; validate.yml is tag/PR/manual backstop; branch protection off |
| docs/leverage-points.md | 207 | "CI is the authoritative gate — agent cannot merge to main unless it passes" | Local pre-push Go gate is authoritative; CI is a backstop |
| docs/levels/L3-state-management/README.md | 16 | "Beads CLI installed (`pip install beads`)" | Install br (beads_rust); invoke `BEADS_DIR=$PWD/_beads br` |
| docs/levels/L3-state-management/README.md | 29 | `bd ready/list/show/update/close` taught as current | Rewrite to `br …` with BEADS_DIR |
| docs/levels/L3-state-management/demo/implement-session.md | 9 | `bd ready/update/close` as current implement loop | Rewrite using br, or label legacy bd era |
| docs/levels/L3-state-management/demo/implement-session.md | 89 | `bd vc status` Dolt check | Remove; use `git -C _beads push` |
| docs/levels/L3-state-management/implement.md | 43 | `bd close ...` in newcomer implement loop | `BEADS_DIR=$PWD/_beads br close <id>` etc. |
| docs/levels/L3-state-management/implement.md | 74 | `bd vc status` Dolt check as close protocol | Remove; `git -C _beads push` + `git push` |
| docs/levels/L3-state-management/plan.md | 18 | Issues in `.beads/` DB; `bd ready`/`bd list` | `_beads/` + `br …`, or mark legacy 2.x |
| docs/levels/L3-state-management/plan.md | 53 | `bd create` / `bd dep add` worked example | `br create` / `br dep add` / `br ready` |
| docs/levels/L5-orchestration/README.md | 10 | "Integration with gastown for parallel workers" | NTM + Agent Mail substrate |
| docs/levels/L5-orchestration/README.md | 33 | "Mayor mode: dispatches to parallel polecats via gastown" | Remove or re-doc as NTM swarm |
| docs/levels/L5-orchestration/README.md | 41 | Observe via `bd show`, `bd ready` | `br show` / `br ready` |
| docs/levels/L5-orchestration/README.md | 59 | Mayor: parallel dispatch via `gt sling` | Remove or replace with NTM swarm dispatch |
| docs/levels/L5-orchestration/demo/crank-session.md | 114 | `bd vc status # Optional Dolt status check` | Remove; use `br sync` if needed |
| docs/runbooks/bd-server-mode-closeout.md | 14 | Workspaces wired to shared bushido Dolt server (current closeout) | Mark whole runbook retired-legacy; point at `br` + `git -C _beads push` |
| docs/runbooks/bd-server-mode-closeout.md | 52 | Run `bd vc status / bd dolt push` after every tracker write | Replace with br closeout, or label historical |
| docs/runbooks/beads-failure-recovery.md | 10 | "agentops tracks issues in bd (Dolt server mode)"; gate bd-coupled | Reframe historical or rewrite for br + `_beads` recovery |
| docs/runbooks/beads-failure-recovery.md | 38 | `bd ping / bd context / bd doctor` triage against Dolt server | Replace with `br …`; ledger is local git-JSONL |
| docs/troubleshooting.md | 88 | bd/Dolt schema repair as live procedure (`brew upgrade beads`) | Replace with br equivalents or mark historical |
| docs/troubleshooting.md | 212 | "3.0 has no push gate hook — git push never blocked locally; CI is authoritative" | Rewrite: local cockpit pre-push gate blocks the push and is routine authority; CI is backstop |

## 3. P2

| File | Line | Stale claim | Fix |
|---|---|---|---|
| docs/CI-CD.md | 168 | CI smoke: hook install smoke + `ao init --hooks` | Remove hook-install / `ao init --hooks` from Phase 5 |
| docs/agent-workflow-reference.md | 78 | "No TODOs in SKILL.md. Use `bd` issue tracking instead." | Use `br` (`BEADS_DIR=$PWD/_beads br`) |
| docs/agentops-3-explainer-kit.md | 66 | `.beads/issues.jsonl` in expected-artifacts | `_beads/issues.jsonl` |
| docs/agentops-3-youtube-starter-series.md | 31 | "Show daemon/Dream as second-stage automation" + Daemon adoption fields | NTM + Agent Mail substrate; rename "Daemon adoption" → "Substrate adoption" |
| docs/architecture/agentpod-swarmjob-schema.proposal.md | 122 | Reconcile controller must poll `bd ready`; "bd write path" | `br ready` / br write path (or generalize) |
| docs/architecture/ao-command-customization-matrix.md | 17 | `bd` = current Tier-A configurable control-plane dep (rows 17,18,26,27,28,35) | Replace `bd` with `br` (note BEADS_DIR) or mark rows retired |
| docs/architecture/fungibility-charter.md | 33 | `bd ready` / `bd update --claim` as live claim mechanism | `br ready` / `br update --claim` |
| docs/architecture/fungibility-charter.md | 35 | "Implemented by `bd`'s role-free claim model" | Cite br's role-free claim model |
| docs/brownian-ratchet.md | 100 | `bd ready/list/update/close` examples (also 144-146, 264-283) | Rewrite to br; update legacy gt-sling/polecat framing |
| docs/comparisons/competition-rpi-memory-pipelines.md | 64 | Out-of-session dispatch on "Gas City reference City" (lines 26,35,64,405) | NTM + MCP Agent Mail substrate |
| docs/comparisons/competition-rpi-memory-pipelines.md | 405 | "Gas City out-of-session dispatch" (Dreaming row) | NTM + Agent Mail out-of-session dispatch |
| docs/comparisons/competitive-radar.md | 129 | Unattended run = Gas City mayor/refinery slinging beads to `ao rpi` | NTM + MCP Agent Mail substrate |
| docs/comparisons/vs-claude-flow.md | 238 | `SessionStart: AgentOps loads context on-demand` (hook framing) | `ao session bootstrap` → `ao inject`; operator-invoked, not hook |
| docs/comparisons/vs-everything-claude-code.md | 33 | "out-of-session dispatch via the Gas City reference City" | NTM + MCP Agent Mail substrate |
| docs/comparisons/vs-everything-claude-code.md | 143 | Gas City mayor/refinery + `ao rpi` as current dispatch | NTM + MCP Agent Mail (`ao agent` / `ao mcp serve`) |
| docs/contracts/claude-bot-delegation.md | 83 | "Push directly to main — no, branch protection blocks" | Branch protection is OFF (push-to-main); @claude path is CI backstop |
| docs/contracts/claude-bot-delegation.md | 132 | "Default required check: claude-review (branch protection requires it on main)" | Branch protection off; claude-review is CI backstop on PRs/tags |
| docs/contracts/orchestration-ports.md | 84 | Degrades to beads floor "tracked through `bd`" | Replace `bd` with `br` |
| docs/contracts/remote-compute.md | 123 | "Daemon ledger remains authoritative for daemon-owned sessions" | Remove daemon-ledger ownership; reflect in-session + NTM/Agent Mail |
| docs/contracts/repo-execution-profile.md | 145 | Example tracker_commands block uses `bd ready/show/update/close` | Update to `br …` with BEADS_DIR |
| docs/contracts/rpi-run-registry.md | 145 | "GasCity Session Correlation" mandates gc fields + daemon-mode | Strike or banner as retired; backends are auto/direct/stream/tmux |
| docs/contracts/rpi-run-registry.md | 162 | "RPI must use" GasCity-provider statuses | Remove gc-specific status rules or relabel legacy |
| docs/context-packet.md | 426 | "The deprecated `ao inject` output a flat dump" | Drop "deprecated" — `ao inject` is current; describe prior mode |
| docs/domain-practice-packets.md | 59 | `bd issues` listed as current work surface (also line 106) | `br issues (_beads/)` |
| docs/domain-practice-packets.md | 111 | "optional schedule or daemon lane" | NTM + Agent Mail out-of-session substrate |
| docs/examples/agentops-3-council-demo-storyboard.md | 231 | "Show out-of-session dispatch on Gas City / Dream" | NTM + MCP Agent Mail |
| docs/examples/agentops-3-council-demo-storyboard.md | 254 | "The out-of-session (Gas City) lane is second-stage" | NTM + MCP Agent Mail lane |
| docs/examples/agentops-3-council-verdict-example.md | 21 | "Keep the daemon and software-factory automation as second-stage" | Out-of-session NTM + Agent Mail substrate |
| docs/examples/agentops-3-council-verdict-example.md | 49 | `bd create ... --description` for tracked follow-up | `BEADS_DIR=$PWD/_beads br create ... --body` |
| docs/examples/agentops-3-domain-practice-packet.md | 63 | "loop dispatched on the Gas City reference City" | NTM + MCP Agent Mail substrate |
| docs/GLOSSARY.md | 88 | "Gate: enforced by a hook … push gate blocks until /vibe passed" | Local cockpit gate (`ao gate check`) / CI, not a hook |
| docs/GLOSSARY.md | 102 | "3.0 ships zero hooks … CI is the authoritative gate" (contradiction) | Local cockpit gate is routine authority; CI is backstop |
| docs/GLOSSARY.md | 125 | "Operational Invariant: enforced by hooks … mechanically enforced" | Enforced by gate registry / explicit `ao` checks |
| docs/GLOSSARY.md | 147 | "Ratchet … hooks enforce it going forward" | Gate/pawl enforces it going forward |
| docs/GLOSSARY.md | 180 | "Validation Gates … and runtime hook gates" | Drop "runtime hook gates"; reference `ao gate check` |
| docs/how-it-works.md | 145 | "operating loop … CI is the single authoritative gate" | Local pre-push Go gate is authority; CI is tag/PR/manual backstop |
| docs/how-it-works.md | 153 | Validation gates surface = CI + skill checks + `make test` | Local Go gate primary; CI backstop |
| docs/leverage-points.md | 325 | "CI merge gates that cannot be forgotten" | Local pre-push Go gate |
| docs/levels/L3-state-management/README.md | 35 | `bd vc status # Optional Dolt status check` | Remove; describe `br sync` / git-JSONL |
| docs/levels/L3-state-management/README.md | 42 | "use `bd vc status` only if you need Dolt state" | Drop Dolt-state ref; git-JSONL via br |
| docs/levels/L3-state-management/demo/crank-session.md | 12 | `bd show agentops-epic-xyz` | `BEADS_DIR=$PWD/_beads br show` |
| docs/levels/L3-state-management/demo/crank-session.md | 135 | `gt sling … Mode: mayor (parallel via gastown)` | Remove gastown Mayor Mode or rewrite as NTM swarm |
| docs/levels/L3-state-management/demo/crank-session.md | 165 | "Integrates with gastown for multi-agent parallelization" | NTM + Agent Mail substrate |
| docs/levels/L3-state-management/demo/plan-session.md | 39 | `bd create / bd ready / .beads/beads.db` as current state mgmt | Rewrite using br + `_beads`; remove `.beads/beads.db` |
| docs/levels/L4-parallelization/implement-wave.md | 28 | `bd ready`/`bd close` as current (also 55,57,67,79,92) | `BEADS_DIR=$PWD/_beads br ready` / `br close` |
| docs/levels/L5-orchestration/crank.md | 24 | `bd ready` (wave issues) (also 29,44,56,61) | `BEADS_DIR=$PWD/_beads br ready` / `br update` |
| docs/runbooks/autonomy-runtime-cycle-1.md | 60 | `bd create` in body (also bd update 89, show 69, sync 20) | Replace with `BEADS_DIR=$PWD/_beads br …` |
| docs/runbooks/autonomy-runtime-cycle-1.md | 89 | Runtime Surface table: `bd update <id> --claim` | `BEADS_DIR=$PWD/_beads br update <id> --claim` |
| docs/runbooks/bushido-refinery.md | 16 | Refinery files fix-bead via `bd create` | `BEADS_DIR=$PWD/_beads br create --labels refinery,blocking` |
| docs/runbooks/release-process.md | 117 | "Close the release bead/epic … `bd close <id>`" | `BEADS_DIR=$PWD/_beads br close <id>` |
| docs/seed-definition.md | 169 | "Why Exactly 6 Elements" table still lists Hooks | Replace Hooks row with "Lifecycle stages (hookless `ao` + CI gate)" |
| docs/strategic-direction.md | 11 | "One binary. Skills, hooks, knowledge flywheel, RPI …" | Drop "hooks" from product-pillar list |
| docs/strategic-direction.md | 92 | "Hooks enforce gates mechanically (push/pre-mortem/worker)" | Go pre-push gate is current mechanical enforcement |
| docs/strategic-direction.md | 94 | "Shift #4 … hook lifecycle events fire automatically …" | Reframe historical/2.x; Go gate + explicit `ao` context |
| docs/templates/intent-issue.md | 8 | "renders a Directive 12 compliant body for `bd create`" | `br create` (with BEADS_DIR) |
| docs/troubleshooting.md | 13 | "CI is the authoritative gate (validate.yml)" | Local cockpit gate is routine authority; CI backstop |
| docs/troubleshooting.md | 287 | `gt` + `bd` PATH deps; `brew install gastown/beads` | Replace with br; drop gastown/beads brew hints |
| docs/wiki-for-agents.md | 41 | "the daemon defrags overnight" | Out-of-session substrate (NTM + Agent Mail) defrags |
| docs/wiki-for-agents.md | 52 | "Daemon defrags, evolves, compounds it overnight" | Reference NTM + Agent Mail substrate |
| docs/workflows/README.md | 15 | Golden Paths: `ao reconcile --json`, then `bd ready` | `BEADS_DIR=$PWD/_beads br ready` |

## 4. P3 / Dead Links

| File · Line | Issue → Fix |
|---|---|
| docs/GLOSSARY.md · 24 | Dead links: `skills/<name>.md` → `docs/skills/` doesn't exist (all ~19 links: 24,35,50,53,69,83,96,107,115,133,136,139,147,150,153,170,183,188,191) → point at `../skills/<name>/SKILL.md` or docs/SKILLS.md anchors |
| docs/reference.md · 318 | Dead link `cli/docs/HOOKS.md` (also contradicts hookless 3.0) → remove; point at `ao session bootstrap`/`ao inject` |
| docs/activation-profiles.md · 54 | "Relevant bd issue" (also 94, 119) → "br bead" |
| docs/agentops-3-pmf-evidence-loop.md · 36 | "council, packets, or daemon scheduling" → "out-of-session substrate orchestration" |
| docs/agentops-3-pmf-evidence-loop.md · 77 | "trust the daemon or a schedule" + "Daemon Adoption Metric" → reframe around NTM/MCP/managed-agents substrate |
| docs/architecture/hexagon-port-realness-audit.md · 160 | "gc bridge is the live path per CLAUDE.md" (dated 2026-05-23) → add freshness banner: gc bridge removed (soc-2rtm0); backends auto/direct/stream/tmux |
| docs/CI-CD.md · 95 | "summary is the branch protection target" → branch protection off; summary is tag/PR/manual backstop |
| docs/comparisons/vs-claude-flow.md · 181 | Diagram shows `/crank` → `/vibe` → `/implement` → `/validate` or operating loop (`/vibe` retired) |
| docs/contracts/dream-report.md · 189 | "Linked bead when bd sync succeeds" → "when br/bead sync succeeds" |
| docs/contracts/repo-execution-profile.md · 63 | Inline example `zsh -lc '… bd …'` → `br …` with BEADS_DIR |
| docs/examples/agentops-3-domain-practice-packet.md · 97 | `ao rpi <bead-id>` as live loop → operating-loop dispatch (NTM + Agent Mail); note `ao rpi` legacy |
| docs/ENV-VARS.md · 50 | `AGENTOPS_RPI_BD_COMMAND` default `bd` in live RPI section → default `br` (BEADS_DIR), or note bd retired-legacy |
| docs/runbooks/outcomes-grading.md · 108 | "Global-Dolt shared burn ledger" follow-on → Dolt-free cross-host quota mechanism |
| docs/seed-definition.md · 190 | "operational mechanics (hooks, ratchet, flywheel)" → drop "hooks" |
| docs/standards/olympus-engineering-standards.md · 13 | "agentopsd extraction" framed as forward target (13,20,46) → note agentopsd retired (ADR-0009) |
| docs/task-proposals-2026-04-29.md · 23 | Proposal #2 targets `cli/internal/daemon/reconcile_test.go` (dir GONE) → strike or mark superseded by daemon teardown |
| docs/templates/intent-issue.md · 98 | "Parent bead: `<bd id …>`" → `<br id …>` |
| docs/troubleshooting.md · 267 | "`bd --help` or `gt --help`" → `br --help` |
| docs/workflows/session-lifecycle.md · 23 | RPI lane "Recommended in Codex" → reframe RPI as legacy executor; point at operating loop / `ao session bootstrap` |

## 5. Recommended Remediation Order (Fix-Arcs as Beads)

**Arc 1 — Purge bd/Dolt as the current tracker in onboarding & tutorial surfaces (P1, highest user impact).** New users run these literally. Files: docs/first-value-path.md, docs/agentops-3-explainer-kit.md, docs/examples/agentops-3-council-demo-storyboard.md, docs/examples/agentops-3-domain-practice-packet.md, docs/examples/agentops-3-council-verdict-example.md, docs/levels/L3-state-management/{README.md, implement.md, plan.md, demo/implement-session.md, demo/plan-session.md}, docs/levels/L4-parallelization/implement-wave.md, docs/levels/L5-orchestration/{README.md, crank.md, demo/crank-session.md}. Standard rewrite: `bd <cmd>` → `BEADS_DIR=$PWD/_beads br <cmd>`; `.beads/issues.jsonl` → `_beads/issues.jsonl`; `pip install beads` → install br; strike all `bd vc status`/Dolt lines (use `git -C _beads push`).

**Arc 2 — Retire the bd/Dolt recovery runbooks (P1, steers agents into a retired SPOF).** Files: docs/runbooks/bd-server-mode-closeout.md, docs/runbooks/beads-failure-recovery.md, docs/troubleshooting.md (lines 88, 267, 287). Mark the two runbooks retired-legacy with a banner pointing to br + `_beads`; rewrite the troubleshooting bd section.

**Arc 3 — Replace Gas City / gastown / `ao rpi` with the NTM + MCP Agent Mail substrate (P1/P2).** Files: docs/contracts/remote-compute.md, docs/contracts/rpi-run-registry.md, docs/comparisons/{competition-rpi-memory-pipelines.md, competitive-radar.md, vs-everything-claude-code.md, vs-tons-of-skills.md, vs-claude-flow.md}, docs/levels/L5-orchestration/{README.md, demo/crank-session.md}, docs/workflows/session-lifecycle.md, docs/examples/agentops-3-domain-practice-packet.md (line 97), docs/architecture/hexagon-port-realness-audit.md (freshness banner). Drop `gt sling`/mayor/polecats and using-gc; frame `ao rpi` as load-bearing legacy.

**Arc 4 — De-PR-ify / correct the gate-authority docs to push-to-main (P1/P2 contradictions).** Files: docs/troubleshooting.md (13, 212), docs/how-it-works.md (145, 153), docs/leverage-points.md (115, 207, 325), docs/CI-CD.md (95, 168), docs/contracts/claude-bot-delegation.md (83, 132). Standard rewrite: local pre-push Go gate (`ao gate check`) is the routine release authority; branch protection OFF; validate.yml is a tag/PR/manual backstop.

**Arc 5 — Remove hooks-as-current-enforcement framing (P2/P3, hookless 3.0).** Files: docs/GLOSSARY.md (88, 102, 125, 147, 180), docs/strategic-direction.md (11, 92, 94), docs/seed-definition.md (169, 190), docs/comparisons/vs-claude-flow.md (238), docs/CI-CD.md (168), docs/context-packet.md (426 — un-deprecate `ao inject`), docs/reference.md (318). Reframe gates as the Go gate registry / `ao gate check` + explicit `ao session bootstrap`/`ao inject`.

**Arc 6 — Retire the deleted daemon/agentopsd surface (P2/P3, ADR-0009).** Files: docs/wiki-for-agents.md (41, 52), docs/domain-practice-packets.md (59, 106, 111), docs/agentops-3-youtube-starter-series.md, docs/agentops-3-pmf-evidence-loop.md, docs/standards/olympus-engineering-standards.md, docs/task-proposals-2026-04-29.md, docs/contracts/remote-compute.md (123), docs/runbooks/outcomes-grading.md (Dolt burn ledger). Replace "daemon"/"agentopsd extraction"/"Daemon adoption" with the out-of-session NTM + Agent Mail substrate.

**Arc 7 — Mop-up bd→br in contracts, architecture & misc reference (P2/P3, lower-traffic).** Files: docs/agent-workflow-reference.md (78), docs/contracts/{orchestration-ports.md, repo-execution-profile.md, dream-report.md}, docs/architecture/{the-agent-factory.md, fungibility-charter.md, agentpod-swarmjob-schema.proposal.md, ao-command-customization-matrix.md}, docs/brownian-ratchet.md, docs/activation-profiles.md, docs/templates/intent-issue.md, docs/ENV-VARS.md, docs/workflows/README.md, docs/runbooks/{autonomy-runtime-cycle-1.md, release-process.md, bushido-refinery.md}, docs/GLOSSARY.md (dead skills/* links). Mechanical bd→br swap + fix the ~19 dead GLOSSARY skills/* links to `../skills/<name>/SKILL.md`.

*(Arcs 1 and 4 carry the most agent-misleading risk and should land first; Arc 7 is the largest file count but lowest per-file risk and can be a single mechanical sweep with a follow-up grep gate.)*

## 6. Coverage & Limits

This sweep covered only **LIVE doctrine docs that mention a retired concept** (bd/Dolt, Gas City/gastown/gc, hooks/SessionStart, daemon/agentopsd, CI-as-authoritative-gate, `ao rpi`/`/vibe`/`/crank` as primary). It did **not** cover dated archive lanes that are historical by design: `docs/audits`, `docs/plans`, `docs/brainstorms`, `docs/council`, `docs/handoffs`, `docs/learnings`, `docs/evidence`, `docs/releases`, `docs/convergence`, `docs/rescope`, `docs/reduction`, `docs/migration-trackers`, `docs/sovereignty-proof`, `docs/rfcs`, and `docs/code-map` — these intentionally preserve point-in-time state and a stale reference there is a record, not a defect. Docs with no retired-concept mention were out of scope. Two caveats on the findings themselves: (1) line numbers are as-reported and may have drifted if any of these files were edited after the review snapshot — re-grep the literal stale string before editing rather than trusting the line; and (2) a few flagged docs carry partial reconciliation banners up top (e.g. autonomy-runtime-cycle-1.md) while the body remains stale, so a blanket "this doc is already fixed" assumption is unsafe — each body command must be checked individually.