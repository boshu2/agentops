<div align="center">

<img src="docs/assets/logo.svg" alt="AgentOps" width="72" height="72">

# AgentOps

**DevOps discipline for AI coding agents: shape the work, track it as a graph,
and get each change judged by a context that didn't write it.**

[![Validate](https://github.com/boshu2/agentops/actions/workflows/validate.yml/badge.svg?branch=main)](https://github.com/boshu2/agentops/actions/workflows/validate.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Latest release](https://img.shields.io/github/v/release/boshu2/agentops)](https://github.com/boshu2/agentops/releases/latest)
[![Skills](https://img.shields.io/badge/skills-38-black.svg)](docs/SKILL-ROUTER.md)

[Install](#quickstart) · [The loop](#the-operational-loop) ·
[Goals](#goals-many-rpis-over-a-bead-graph) · [Try it](#try-it) · [Skills](#skills-at-a-glance)

</div>

AgentOps is 38 optional skills and a CLI (`ao`) for AI coding agents. You state
intent as behavior in your domain's words. The skills carry it from one change
to a goal that runs many RPIs over a Beads work graph.

<a id="why-these-skills-exist"></a>

## Why use AgentOps?

| When the agent… | AgentOps adds |
|---|---|
| Builds something different from what you meant | Given/When/Then examples shared by implementation and review |
| Uses three names for one concept | One domain term per concept, in intent, code and tests |
| Says “done” after a green test run | A fresh judge that didn't write the change |
| Loses the thread on work bigger than one session | A Beads graph holding intent, dependencies and verdicts |
| Runs off with a half-formed goal | An interview that settles the goal before agents go autonomous |

<a id="install"></a>

## Quickstart

Pick one install method so you don't get duplicate copies.

<details>
<summary><strong>Claude Code</strong></summary>

<a id="claude-code"></a>

```bash
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace
claude plugin details agentops@agentops-marketplace
```

Check that `agentops` appears in the plugin inventory. The bundle includes
skills, four agents and [tool-call guards](#optional-admission-control-hooks).

</details>

<details>
<summary><strong>Codex</strong></summary>

<a id="codex"></a>

```bash
codex plugin marketplace add boshu2/agentops
codex plugin add agentops@agentops-marketplace
codex plugin list --json
```

Check that `agentops` appears in the inventory. Skills use the `agentops:` prefix;
custom roles and read limits have [separate setup](docs/install-day2-ops.md#install-and-update-runtime-plugins).

</details>

<details>
<summary><strong>Cursor and other agents</strong></summary>

With Node.js installed, run from your project directory:

```bash
npx skills@latest add boshu2/agentops --agent cursor
```

Select the skills you want or the whole library. Omit `--agent cursor` to
choose another agent; add `-g` for a user-level install. Check the skill's
source path in Cursor's [native skill picker](https://cursor.com/docs/skills).
See [host coverage and limits](docs/contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping).

</details>

Validate needs the [`ao` CLI](#optional-ao-cli). This README shows Claude
Code's `/agentops:<skill>`; Codex uses `$agentops:<skill>`.

<a id="workflow"></a>

## The operational loop

Each change is shaped, built and judged. Intent is written as behavior (BDD)
in the domain's words (DDD). For a system that calls queued work a **Job**:

```gherkin
Given a Job has already completed
When the worker receives that Job again
Then it returns the completed result without repeating the side effect
```

<p align="center">
  <img src="docs/assets/agentops-routes.svg" alt="AgentOps routes: intent goes to Plan when unclear or straight to Implement when clear; Implement runs native checks, then a fresh judgment independent of the author. Accepted work finishes, failed behavior returns to Implement for repair, missing evidence is gathered and judged again. An existing change enters at fresh judgment. An optional learning loop turns results into reviewed .context/ pages that later work queries." width="100%">
</p>

| Step | Skill | What it does with the example |
|---|---|---|
| Shape | [`plan`](skills/plan/SKILL.md) | Pins it to one slice. Skip it when intent is clear. |
| Build | [`implement`](skills/implement/SKILL.md) | Makes the change and tests both outcomes. |
| Judge | [`validate`](skills/validate/SKILL.md) | A fresh context returns `PASS`, `FAIL` or `NOT_PROVEN` against the same example. |
| Learn | [`memory`](skills/memory/SKILL.md) | Optional: reviewed `.context/` pages that later work can query. |

Enter at the step you need; an existing change goes straight to Validate. The
author never approves its own work. Delivery follows your repository's policy.

## Goals: many RPIs over a bead graph

[`rpi`](skills/rpi/SKILL.md) runs the loop hands-off for one outcome and stops
at acceptance or a spent limit. A bigger outcome becomes a goal:

1. **[Interview](skills/interview/SKILL.md).** One question at a time, each with
   a recommended answer. You settle the outcome, its examples, domain terms,
   non-goals, authority and budget before agents go autonomous.
2. **[Craft Goal](skills/craft-goal/SKILL.md).** Returns `SAFE_TO_CREATE` with a
   prompt for `/goal` in Claude Code or Codex, `USE_RPI`, or `UNSAFE_GOAL`
   naming what's undecided. Crafting alone creates nothing.
3. **[Navigate](skills/navigate/SKILL.md) each wave.** Picks a small wave of
   ready beads; each gets one RPI and a fresh Validate. The goal ends
   `ACHIEVED`, `NOT_ACHIEVED` or `NEEDS_OPERATOR`.

<p align="center">
  <img src="docs/assets/agentops-goal-graph.svg" alt="A goal acts as orchestrator: it observes the Beads work graph, picks a bounded wave of ready beads, consumes verdicts, then ratchets or stops. The graph holds a root epic and child beads: A closed with PASS, B discovered from A and ready, C ready, D blocked by C. Picked beads B and C each get one RPI: when the goal delegates, a fresh worker that starts with only that bead plans if unclear and implements, and Validate runs in a separate fresh context. Verdicts and notes are written back to the bead." width="100%">
</p>

**Beads holds the plan.** [Beads](https://github.com/gastownhall/beads) (`bd`)
keeps work as a dependency graph outside any conversation, so a goal survives
compaction and restarts. The root epic holds acceptance; each child bead is one
RPI with its question, scope, notes and verdict. `blocks` means real ordering,
`discovered-from` marks work a result exposed, and `bd ready` is the frontier.

**One bead per worker.** When the goal delegates, the orchestrator holds the
graph and verdicts, and each worker starts with one bead instead of the
orchestrator's transcript. Validators start fresh. A wave counts only if it
proves acceptance, kills a hypothesis or settles an uncertainty.

```bash
bd create "Job redelivery is idempotent" -t epic
bd create "Return the completed result on redelivery" --parent <epic-id>
bd dep add <later-id> <earlier-id>         # real ordering only
bd ready --parent <epic-id>                # the frontier
```

Any tracker with status, dependencies and notes works; AgentOps never builds a
second work index.

## Try it

```text
# One change
/agentops:plan make Job redelivery return the completed result without repeating the side effect
/agentops:implement
/agentops:validate     # new conversation: give it the example, the change and the author's ID

# One outcome, hands-off
/agentops:rpi make Job redelivery return the completed result without repeating the side effect

# A goal
/agentops:interview make the job worker safe under redelivery, retries and crash recovery
/agentops:craft-goal   # then paste its prompt into /goal
```

<a id="how-independent-review-works"></a>

<details>
<summary><strong>Validate an existing change</strong></summary>

Pick a finished change whose accepted behavior is recorded in an issue or
conversation. Run the required checks, keep the candidate unchanged, and
[install `ao`](#optional-ao-cli). Then open a **new conversation**, fill in the
references and paste:

```text
Use the AgentOps Validate skill to judge this finished change.
Original accepted behavior: [issue link or original request text]
Candidate: [commit, branch or working tree; list every changed path]
Author context ID: [task/session ID that made the change]
Checks run: [commands and results]
I opened this new conversation for fresh review. Derive the exact subject
identity at the start and end, inspect every changed path against the original
behavior, and do not modify the candidate. Report PASS, FAIL or NOT_PROVEN with
evidence for each criterion, checked, not_checked, author and reviewer context
IDs, and freshness attestation.
```

`PASS` needs evidence for every criterion and an empty `not_checked`. `FAIL`
names failed behavior or an out-of-scope change. Missing proof, identity or
path coverage is `NOT_PROVEN`.

</details>

<a id="try-one-task"></a>

<details>
<summary><strong>Read-only skill-loading smoke test</strong></summary>

Paste this in an agent conversation in your project. It needs no `ao` CLI.

```text
Use the AgentOps Research skill to trace how this repository validates user
input. Follow one path from the input through its checks and tests. Cite the
files and line numbers, explain one edge case, and identify missing coverage.
Name the Research skill file you loaded. Answer here without changing files.
```

The reported skill path catches missing or duplicate installs. Invoke Research
directly with `/agentops:research` in Claude Code, `$agentops:research` in
Codex, or `/` and the installed Research entry in Cursor.

</details>

<a id="choose-skills-by-the-work"></a>

## Skills at a glance

38 skills, all optional. Load one when it answers a specific question. Full
catalog: **[docs/SKILL-ROUTER.md](docs/SKILL-ROUTER.md)**.

| Group | Skills | What it covers |
|---|---|---|
| Operational loop | [`plan`](skills/plan/SKILL.md) [`implement`](skills/implement/SKILL.md) [`validate`](skills/validate/SKILL.md) | Shape, build and judge every change |
| Autonomous | [`rpi`](skills/rpi/SKILL.md) | One outcome, hands-off |
| Goals | [`interview`](skills/interview/SKILL.md) [`craft-goal`](skills/craft-goal/SKILL.md) [`navigate`](skills/navigate/SKILL.md) | Shape, write and walk a goal over the bead graph |
| Coordination | [`orchestrate`](skills/orchestrate/SKILL.md) [`agent-native`](skills/agent-native/SKILL.md) | Fresh workers per bead, disjoint scopes, integration |
| On demand | [`research`](skills/research/SKILL.md) [`domain`](skills/domain/SKILL.md) [`test`](skills/test/SKILL.md) [`refactor`](skills/refactor/SKILL.md) [`review`](skills/review/SKILL.md) [`security`](skills/security/SKILL.md) [`doc`](skills/doc/SKILL.md) [`reverse-engineer`](skills/reverse-engineer/SKILL.md) | Reached for when a specific question comes up |
| Learning | [`memory`](skills/memory/SKILL.md) [`cass`](skills/cass/SKILL.md) | Reviewed project context and past-session search |
| Judgment strategies | [`council`](skills/council/SKILL.md) [`premortem`](skills/premortem/SKILL.md) [`postmortem`](skills/postmortem/SKILL.md) [`reality-check`](skills/reality-check/SKILL.md) [`idea-genie`](skills/idea-genie/SKILL.md) | Challenge, comparison and claim audits |
| Runtimes and factories | [`codex-exec`](skills/codex-exec/SKILL.md) [`agy-native`](skills/agy-native/SKILL.md) [`ntm`](skills/ntm/SKILL.md) [`agent-mail`](skills/agent-mail/SKILL.md) [`using-gc`](skills/using-gc/SKILL.md) [`using-flywheel`](skills/using-flywheel/SKILL.md) | Selected executors, panes, mail and factories |
| Guards and utilities | [`cc-hooks`](skills/cc-hooks/SKILL.md) [`dcg`](skills/dcg/SKILL.md) [`account-rotation`](skills/account-rotation/SKILL.md) [`rch`](skills/rch/SKILL.md) [`sbh`](skills/sbh/SKILL.md) [`ms`](skills/ms/SKILL.md) | Hooks, command guards, accounts, builds, disk, skill search |
| Skill craft | [`skill-builder`](skills/skill-builder/SKILL.md) [`skill-eval`](skills/skill-eval/SKILL.md) | Author skills and measure whether they help |

## Where AgentOps fits

AgentOps grew from applying DevOps experience and established engineering
practice to agents. The [Practice Registry](PRACTICE-REGISTRY.md) records the
lineage; [how it works](docs/how-it-works.md) covers responsibilities.

<details>
<summary>Compare libraries, trackers and agent factories</summary>

| Project or tool | Role alongside AgentOps |
|---|---|
| [Compound Engineering](https://github.com/EveryInc/compound-engineering-plugin) | A connected development workflow and reusable solution records |
| [Matt Pocock's skills](https://github.com/mattpocock/skills) | Composable practices for intent, domain modeling, TDD and review |
| [Beads](https://github.com/gastownhall/beads) or your existing tracker | Owns work status, dependencies and handoffs |
| Factories such as [Gas City](skills/using-gc/SKILL.md) | Own agent coordination and execution through their native control plane |

Choose which workflow leads the task. Carry accepted behavior and evidence
into [independent judgment](skills/validate/SKILL.md). Shared practices are not
proof that every combination has been tested.

</details>

## Optional `ao` CLI

Install `ao` for deterministic repository checks, evidence tools, or a skill
that requires it, including Validate.

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
ao version
ao quick-start
```

With Go installed: `go install github.com/boshu2/agentops/cli/cmd/ao@latest`.
`ao quick-start` provides read-only guidance; `ao init` is optional evidence setup.
Use `ao gate check` for repository checks and `ao config --show` to inspect
configuration. See the [command reference](cli/docs/COMMANDS.md) and
[installation guide](docs/install-day2-ops.md).

## Updating and advanced setup

<details>
<summary><strong>Upgrading to 3.8</strong></summary>

<a id="upgrading-to-38"></a>
<a id="upgrading-to-37"></a>

Version 3.8 retains existing 3.7 command and skill names. Use the
[plugin update instructions](docs/install-day2-ops.md#install-and-update-runtime-plugins)
or the [source and Skills installer guide](docs/install-day2-ops.md#update).
For Homebrew: `brew update && brew upgrade agentops`. Start a new session
afterward; new installs do not silently remove obsolete copies.

**Upgrading from 3.6 or earlier:** read the [migration guide](docs/MIGRATION.md).
Version 3.7 removed commands and skill names, including `learn`, `codebase-recon`
and `swarm`; their current owners are `memory`, `research` and `agent-native`.
See the [3.8 release notes](docs/releases/2026-09-22-v3.8.0-notes.md) and
[3.7 removals](docs/releases/2026-09-13-v3.7.0-notes.md).

</details>

<details>
<summary><strong>Source installs and skill dependencies</strong></summary>

<a id="other-installation-paths"></a>

From an AgentOps checkout with `ao` installed:

```bash
ao skills link --skill test --skill refactor --dry-run
ao skills link --skill test --skill refactor
```

Omit selectors for the whole catalog. Linking preserves existing real
directories and foreign links. See [source setup and removal](docs/install-day2-ops.md#install-source-checkout).
Skill installation does not install tool dependencies:

| Skill | Needs | Why |
|---|---|---|
| `rpi` | `ao`, conditional | delegates exact-subject checks to Validate; only persists `verdict.v2` when requested, with the fixed-dispatch adapter optional |
| `plan` | `ao`, conditional | runs `ao provenance snapshot-intent` with an explicit evidence root when the intent source is not durable |
| `validate` | `ao` | derives exact subject identity with the helper and uses `ao provenance store-verdict` when persistence is requested; Python/schema checks are developer-only |
| `reality-check` | `ao`, conditional | inspect selected goal measurements with `ao goals` or evidence-store facts with `ao status` |
| `using-gc` | `ao` | rig prep runs `ao gc prepare` and `ao gc check` |
| `doc` | `ao`, optional | a requested continuity handoff may use `ao session handoff`/`rehydrate` |
| `reverse-engineer` | `python3` | Phase 1's mechanical teardown runs `scripts/reverse_engineer.py` |
| `skill-builder` | `python3`, conditional | Create mode's `build.sh` runs `scripts/generate-skill-mesh.py`; heal/check/audit modes are bash-only |
| `ms` | `python3`, conditional, plus `ms` binary | the MCP-search fallback runs `python3 skills/ms/scripts/mcp-search.py`; the `ms` binary is required for CLI load, write, and admin operations |
| `memory` | `python3`, conditional | a selected toil investigation can use the repository helper `scripts/toil-mining/recent_human.py` on cleared Codex sources |
| `security` | `python3`, conditional | the composable suite and offline redteam surfaces run `security_suite.py` when that scan type is selected |
| `cass` | `python3`, optional | `scripts/prompt_miner.py` mines repeated prompts; one of several selectable Scripts-table entries |

</details>

<details>
<summary><strong>Shared project context</strong></summary>

<a id="shared-project-context"></a>

[Memory](skills/memory/SKILL.md) can read reviewed `.context/` notes with ordinary
filesystem tools; no `ao` or Beads is needed. See this repo's [context map](.context/README.md).

Adding notes requires authorized sources and fresh review of factual support
and disclosure before Git admission. Drafts and evidence stay in protected
external storage. These procedures do not enforce access permissions or prove
that saved notes improve later work. See [Memory's storage rules](skills/memory/SKILL.md#access-storage-and-honest-limits).

</details>

<details>
<summary><strong>Permissions, optional hooks, and removal</strong></summary>

<a id="optional-admission-control-hooks"></a>

The Claude Code plugin includes PreToolUse guards for private tracker data in
commits, manual provenance-ledger edits and installed-skill overwrites. Installing
only `ao` does not add hooks; other paths can opt in through [CC Hooks](skills/cc-hooks/SKILL.md).
Read-budget guards, Codex roles and trusted Codex hooks have [separate setup](docs/install-day2-ops.md#install-and-update-runtime-plugins).

Disable Claude's plugin with `/plugin disable agentops`. Remove it with
`claude plugin uninstall agentops@agentops-marketplace`; for Codex, use
`codex plugin remove agentops@agentops-marketplace`.

</details>

<details>
<summary><strong>Architecture and saved review evidence</strong></summary>

AgentOps is the operations layer for agentic engineering. Its federated integration graph
connects evidence while Git owns content, the tracker owns work and the coding
runtime or selected factory owns execution. Your repository owns delivery.

Native execution requires zero AgentOps skills. [RPI](skills/rpi/SKILL.md),
[Gas City](skills/using-gc/SKILL.md) and [Agentic Coding Flywheel](skills/using-flywheel/SKILL.md)
are optional; their completion reports do not replace independent review.

On request, Validate can save `verdict.v2` with exact content, checked scope and
evidence. New proof belongs in selected, protected storage outside Git;
existing evidence is preserved.

Read the [architecture](docs/ARCHITECTURE.md), [operating contract](docs/agent-workflow-reference.md)
and [storage rules](docs/adr/ADR-0016-state-tiers.md).

</details>

## Troubleshooting

<details>
<summary>Common problems and how to report one</summary>

| Symptom | What to check |
|---|---|
| `plugin` is not recognized | Update your agent to a version with plugin support |
| A skill is missing | Check its inventory or picker, then start a new session |
| `ao` is not found | Install the CLI and check PATH; Go installs usually use `$(go env GOPATH)/bin` |
| A skill needs another tool | Check its dependencies in the installation guide |
| An old skill name fails | Check the [migration guide](docs/MIGRATION.md#skills) and stale copies |

[Report a reproducible issue](https://github.com/boshu2/agentops/issues) with your
runtime version, install method, command or prompt, and observed result.
Share only evidence you are authorized to disclose.

</details>

## Limits

Skills guide agents; installation alone does not enforce their instructions.
A green test suite or an agreeing model can still miss a defect. Missing proof
stays `NOT_PROVEN`. Saving notes does not establish improved outcomes or
automatic knowledge compounding. See [product evidence and limits](PRODUCT.md).

## FAQ

<details>
<summary>CLI, reviewer model and storage questions</summary>

**Do I need the CLI, an orchestrator or several agents?**

No. Start with one coding agent and a skill such as Research, Test or Refactor.
Obtain fresh, author-distinct judgment when a change is ready. The Validate skill
requires `ao`; native independent review does not.

**Must the reviewer use another model provider?**

No. The default is a fresh context from the author's model family. Cross-model
review is an explicit choice; independence still matters.

**Must durable work live in Git?**

No. Keep source in Git, handoffs in your tracker and requested proof in protected
external storage, following each owner's access and retention rules.

</details>

## Contributing

Contributions are welcome: documentation fixes, reproducible bug reports,
tests, CLI improvements and skills. Read the [contribution guide](docs/CONTRIBUTING.md).

Licensed under [Apache-2.0](LICENSE).
