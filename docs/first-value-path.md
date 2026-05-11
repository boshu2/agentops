# AgentOps 3.0 First-Value Path

This is the path a viewer should be able to follow after a video, gist, or
README skim. It proves the product without asking them to understand the whole
factory first.

The first value is a council verdict over a visible engineering domain:

1. Install AgentOps beside an existing coding-agent runtime.
2. Create or reuse a domain/practice packet.
3. Assemble bounded context for one decision.
4. Run council across Claude and Codex, with a same-packet fallback.
5. Inspect the verdict artifact and turn it into tracked work.
6. Only then show the daemon as the optional scheduled compounding lane.

## Target Viewer

This path is for an engineer or technical founder who already uses Claude Code,
Codex CLI, Cursor, or OpenCode and wants their agent work to preserve context,
judgment, and follow-up discipline across sessions.

The viewer does not need to adopt the full software factory on day one. They
need to see one agent decision become an inspectable engineering artifact.

## Time Budget

| Step | Budget | Success signal |
|---|---:|---|
| Install or update | 2-5 min | `ao version` prints a version. |
| Repo setup | 1 min | `ao quick-start` completes and `.agents/` exists. |
| Packet setup | 2 min | `.agents/packets/<name>.md` exists and is readable. |
| Context assembly | 1 min | `.agents/rpi/briefing-current.md` exists. |
| Council run | 5-10 min | `.agents/council/<run-id>/verdict.md` exists. |
| Verdict to work | 2 min | `bd show <issue-id>` cites the verdict path. |
| Optional daemon lane | 5 min | `ao schedule list` shows an operator-approved schedule. |

## Commands And Expected Outputs

### 1. Install

Codex path:

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
ao version
```

Expected:

```text
ao version <version>
```

Use the matching install command from the README for Claude Code, OpenCode,
Cursor, macOS Homebrew, or Windows PowerShell.

### 2. Set Up The Repo

```bash
ao quick-start
```

Expected:

```text
━━━ SETUP COMPLETE ━━━
AgentOps repo readiness
```

The setup creates the local operating workspace, including `.agents/packets/`,
`.agents/rpi/`, and `.agents/council/`.

### 3. Create The First Domain/Practice Artifact

```bash
cp docs/examples/agentops-3-domain-practice-packet.md \
  .agents/packets/agentops-3-launch.md
sed -n '1,100p' .agents/packets/agentops-3-launch.md
```

Expected:

```text
# AgentOps 3.0 Domain/Practice Packet
```

This is the first artifact the viewer should understand. It names the product
identity, target user, decision, domain sources, practice sources, evidence
rules, and non-goals. In this repo, `PRACTICE.md` is the foundation practice
source: it explains why AgentOps artifacts must stay small enough for agent
context windows while linking forward to their forcing functions.

### 4. Assemble Runtime Context

```bash
ao context packet --goal "AgentOps 3.0 council-first launch demo"
ao context assemble \
  --phase planning \
  --task "Evaluate the AgentOps 3.0 launch demo against the domain/practice packet" \
  --output-file .agents/rpi/briefing-current.md
```

Expected:

```text
Briefing written to <repo>/.agents/rpi/briefing-current.md (<chars> chars)
```

If the exact stdout changes, the artifact path is the contract.

### 5. Run The Council

Primary path:

```text
/council --mixed validate "Given .agents/packets/agentops-3-launch.md, should the AgentOps 3.0 launch demo lead with council-first engineering judgment?"
```

Fallback when only one runtime is available:

```text
/council --quick validate "Given .agents/packets/agentops-3-launch.md, should the AgentOps 3.0 launch demo lead with council-first engineering judgment?"
```

Expected:

```text
Recorded: .agents/council/<run-id>/verdict.md
```

The public demo should say whether the verdict was mixed Claude/Codex or the
single-runtime fallback. The artifact must show the same packet was used.

### 6. Turn The Verdict Into Work

```bash
bd create "Apply council verdict to launch demo" \
  --description "From .agents/council/<run-id>/verdict.md" \
  --json
```

Expected:

```json
{
  "id": "soc-...",
  "status": "open",
  "title": "Apply council verdict to launch demo"
}
```

The important behavior is not the issue id. The important behavior is that the
decision leaves chat and becomes tracked engineering work.

### 7. Optional Scheduled Lane

Only show this after the first verdict has landed.

Terminal 1:

```bash
ao init --with-schedule
ao daemon run --schedule-file .agents/schedule.yaml
```

Terminal 2:

```bash
ao schedule list
```

Expected:

```text
NAME
```

If there is no schedule yet, add one intentionally:

```bash
ao schedule add --file examples/schedules/dream-nightly.yaml
```

## First Artifacts To Inspect

| Artifact | Why it matters |
|---|---|
| `.agents/packets/agentops-3-launch.md` | The shared domain and engineering practices the agents judge against. |
| `.agents/rpi/briefing-current.md` | The bounded task context assembled for the run. |
| `.agents/council/<run-id>/verdict.md` | The engineering verdict from the council. |
| `.beads/issues.jsonl` | The tracked work created from the verdict. |
| `.agents/schedule.yaml` | The optional always-on lane, only after first trust exists. |

## Friction List

| Friction | Handling |
|---|---|
| Mixed council requires both Claude Code and Codex CLI to be installed and authenticated. | Use `/council --quick` for first local proof and record that the demo used fallback mode. |
| Activation profiles are docs-backed, not executable config. | Use `docs/activation-profiles.md`; follow-up `soc-uyp6` evaluates `ao activate product-council` after PMF evidence. |
| `.agents/` is local runtime state and should not be committed. | Copy public examples into `docs/examples/`; export or summarize private verdicts before sharing. |
| The daemon can distract from first value. | Present it as second-stage automation after a human has inspected the verdict. |
| Public claims can outrun evidence. | Use the claim-safe language in the domain/practice packet and storyboard. |

## Product Gaps Found

| Gap | Disposition |
|---|---|
| `ao quick-start` did not create `.agents/packets/`, while the demo path needed it. | Fixed in the 3.0 first-value path slice. |
| `ao activate product-council` would reduce setup steps but might hide what context enters agents. | Tracked as `soc-uyp6`; do not ship until PMF runs prove the profile is stable. |

## Related Docs

- [AgentOps 3.0 Explainer Kit](agentops-3-explainer-kit.md)
- [AgentOps 3.0 YouTube Starter Series](agentops-3-youtube-starter-series.md)
- [Domain and Practice Packets](domain-practice-packets.md)
- [Activation Profiles](activation-profiles.md)
- [AgentOps 3.0 Council Demo Storyboard](examples/agentops-3-council-demo-storyboard.md)
- [AgentOps 3.0 Council Verdict Example](examples/agentops-3-council-verdict-example.md)
