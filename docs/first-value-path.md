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
6. Only then point at the optional out-of-session compounding lane, which runs the same loop on an orchestration substrate (Gas City is the reference) — AgentOps itself ships no daemon or scheduler.

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
| Optional out-of-session lane | 5 min | A substrate (Gas City) Order is registered to run the loop unattended. |

## Commands And Expected Outputs

### 1. Install

Codex path:

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
ao version
```

Codex installs hookless by default. Native hooks are an advanced opt-in:

```bash
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash -s -- --with-hooks
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
rules, and non-goals. In this repo, `PRACTICE-REGISTRY.md` is the practice registry:
it provides the lineage and stable slugs behind the packet's engineering
citations.

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

### 7. Optional Out-of-Session Lane

Only point at this after the first verdict has landed.

The first six steps all run **in session** — that is the AgentOps product and
the zero-dependency sovereignty floor. Running the same loop **out of session**
(always-on, scheduled, unattended) is a separate concern. AgentOps 3.0 ships no
daemon, scheduler, or overnight runner of its own — those surfaces were deleted
(see [AgentOps 3.0 north star](3.0.md)). Out-of-session orchestration is
delegated to a substrate, and AgentOps ships a reference one: a **Gas City**
City (`city.toml` + `packs/agentops/`).

On the reference substrate, a long-lived **mayor** agent runs `bd ready` and
dispatches the next bead to a **refinery** worker that runs `ao rpi <bead>`;
scheduled maintenance (`ao compile`, `ao maturity --scan`) runs as cron `exec`
Orders. The agents inherit the AgentOps skills via an overlay and run the same
loop you just ran by hand. See the [AgentOps 3.0 north star](3.0.md) for the
in-session / out-of-session split and the reference City.

## First Artifacts To Inspect

| Artifact | Why it matters |
|---|---|
| `.agents/packets/agentops-3-launch.md` | The shared domain and engineering practices the agents judge against. |
| `.agents/rpi/briefing-current.md` | The bounded task context assembled for the run. |
| `.agents/council/<run-id>/verdict.md` | The engineering verdict from the council. |
| `.beads/issues.jsonl` | The tracked work created from the verdict. |
| `city.toml` + `packs/agentops/` | The reference Gas City substrate for the optional out-of-session lane, only after first trust exists. |

## Friction List

| Friction | Handling |
|---|---|
| Mixed council requires both Claude Code and Codex CLI to be installed and authenticated. | Use `/council --quick` for first local proof and record that the demo used fallback mode. |
| Activation profiles are docs-backed, not executable config. | Use `docs/activation-profiles.md`; follow-up `soc-uyp6` evaluates `ao activate product-council` after PMF evidence. |
| `.agents/` is local runtime state and should not be committed. | Copy public examples into `docs/examples/`; export or summarize private verdicts before sharing. |
| The out-of-session substrate can distract from first value. | Present it as second-stage automation after a human has inspected the verdict; the in-session loop is the product, the substrate is optional. |
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
