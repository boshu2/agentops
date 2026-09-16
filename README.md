<p align="center">
  <img src="docs/assets/logo.svg" alt="AgentOps" width="72" height="72">
</p>

# AgentOps

[![Validate](https://github.com/boshu2/agentops/actions/workflows/validate.yml/badge.svg?branch=main)](https://github.com/boshu2/agentops/actions/workflows/validate.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Latest release](https://img.shields.io/github/v/release/boshu2/agentops)](https://github.com/boshu2/agentops/releases/latest)

**Agent work you can verify and build on.**

**AgentOps means agent operations:** applying years of DevOps experience to how
coding agents plan, implement, validate, and hand off work.

A coding agent can finish a task and pass its tests while missing the behavior
you asked for. AgentOps gives Claude Code and Codex engineering guidance for
defining that behavior, implementing it in your domain's language, and having a
fresh reviewer check the result against the original request.

Engineering effort should leave reusable improvements behind: working code,
regression tests, useful tools, and decisions the next task can build on.
Start with one task and the guidance it needs; your repository keeps its tests,
tracker, and Git workflow.

[Quickstart](#quickstart) · [BDD example](#from-behavior-to-verified-change) ·
[Ecosystem](#where-agentops-fits) · [Choose a skill](#choose-skills-by-the-work) ·
[Optional CLI](#optional-ao-cli) · [FAQ](#faq) ·
[Documentation](docs/documentation-index.md)

## Why use AgentOps?

| When the agent… | Reach for… | What you can inspect |
|---|---|---|
| Builds something different from what you meant | Behavior-driven planning | A concrete acceptance example used by both implementation and review |
| Uses three names for the same domain concept | Shared domain language | Consistent terms and rule ownership across the request, code, and tests |
| Says “done” after a green test run | Fresh independent validation | Evidence for each requested behavior, with missing coverage called out |
| Leaves the next session to repeat the investigation | Reusable checks and useful work records | Regression tests, tools, and decisions kept with their existing owners |

The 34-skill library packages established engineering practices. Select what
helps the current task; a clear change can proceed directly in your coding agent.

<a id="install"></a>

## Quickstart

Use an installed Claude Code or Codex with plugin support. Run the commands for
**one** runtime in your terminal. They install the full managed skill bundle;
`ao` is not needed for the first task below.

### Claude Code

```bash
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace
claude plugin details agentops@agentops-marketplace
```

### Codex

```bash
codex plugin marketplace add boshu2/agentops
codex plugin add agentops@agentops-marketplace
codex plugin list --json
```

The plugin should appear as `agentops`. Claude's inventory includes 34 skills
and four agents; Codex exposes 34 skills with `agentops:` names. Start a new
session in a project you already work on to load the installed skills.

Skills run inside your coding agent with its normal permissions. The Claude
plugin also installs its [policy dispatcher](#optional-admission-control-hooks).
Optional limits on source reads and Codex's additional agent roles have
[separate setup](docs/install-day2-ops.md#install-and-update-runtime-plugins).
To remove the bundle, use `claude plugin uninstall
agentops@agentops-marketplace` or `codex plugin remove
agentops@agentops-marketplace` in your terminal.

### Try one task

Paste this into your **Claude Code session**, not your terminal:

```text
/agentops:research Find how this repository validates user input. Trace one
path from the input through validation and its tests. Cite the relevant files
and line numbers, explain one edge case, and identify any missing coverage.
Answer in this conversation without changing files.
```

In a **Codex session**, use the same request with its skill prefix:

```text
$agentops:research Find how this repository validates user input. Trace one
path from the input through validation and its tests. Cite the relevant files
and line numbers, explain one edge case, and identify any missing coverage.
Answer in this conversation without changing files.
```

Expect a trace you can inspect: where input enters, which checks accept or reject
it, and what the tests cover. If a step cannot be established, the answer should
say what is missing. You can then request a fix or a regression test using those
file references. Research itself does not authorize a code change.

## From behavior to verified change

**Behavior-driven development (BDD)** means agreeing on concrete examples of
what the software should do before coding, then checking the result against
those same examples. **Given** describes the starting situation, **When** the
action, and **Then** the observable result. For example, in a system that calls
queued work a **Job**:

```gherkin
Given a Job has already completed
When the worker receives that Job again
Then it returns the completed result without repeating the side effect
```

Keep that example in the existing issue or conversation. The implementer fixes
the relevant path and adds a regression test that checks both the returned
result and the absence of a repeated side effect. A fresh reviewer examines the
exact change and its evidence against the same example. Passing an unrelated
test suite does not fulfill the promise.

Domain-driven design (DDD) keeps the meaning of **Job** consistent across the
request, code, tests, and review. Use the repository's established vocabulary
and rule owners; clarify a term when ambiguity would change the behavior.
Plain-text examples suffice. No `.feature` file or glossary is required.

The resulting fix and regression test remain available for future changes.
Record useful decisions and handoffs in the caller's existing tools. This is
the engineering value AgentOps aims to preserve across sessions; a saved note
alone does not establish that later work improved.

<details>
<summary>Try the example through planning, implementation, and review</summary>

Use this in a repository with a Job worker, adapting the domain terms to the
actual system. These are prompts for your coding agent, not shell commands.
Here is the Codex form; in Claude Code, replace `$agentops:` with `/agentops:`.

```text
$agentops:plan Clarify this behavior and its scope: given a Job has completed,
when the worker receives it again, return the completed result without
repeating the side effect. Reuse our existing domain terms and identify the
smallest check that distinguishes correct behavior from the current code.
```

After agreeing on the scope:

```text
$agentops:implement Implement the accepted Job behavior within the agreed scope.
Add a regression check for the returned result and the absence of a repeated
side effect. Run the owning checks and report their results.
```

Then use a **fresh reviewer context**, with access to the exact change, the
accepted scope, and the check results. Validate requires the [optional `ao` CLI](#optional-ao-cli).

```text
$agentops:validate Review this change against the accepted scope and behavior:
a completed Job received again returns its result without repeating its side
effect. Inspect the implementation and check evidence; report PASS, FAIL, or
NOT_PROVEN with checked scope and any missing evidence. Do not change the code.
```

These prompts illustrate a workflow; they are not a transcript or a claim that
the example has run in your repository. Individual skills can also be used on
their own.

</details>

## Design principles

1. **Agree on behavior before implementation.** Keep accepted examples in the
   existing conversation or issue. Tests added later can extend the evidence;
   they cannot redefine what was promised.
2. **Use the domain's own language.** Clarify consequential ambiguity and respect
   boundaries where the same word has different meanings.
3. **Separate authorship from acceptance.** A fresh reviewer checks the exact
   change. The author cannot issue its own binding PASS.
4. **Leave useful improvements behind.** Preserve regression checks, tools, and
   supported decisions where later work can use them. Add guidance when it
   resolves a real need; a new worksheet is not required for every task.

These practices come from BDD, DDD, design by contract, testing, refactoring,
and other established engineering work. The [Practice Registry](PRACTICE-REGISTRY.md)
records their lineage and how skills apply them. See [how it works](docs/how-it-works.md)
for the planning, implementation, and validation responsibilities.

## Where AgentOps fits

AgentOps grew independently from applying DevOps experience and established
engineering practices to agents. Projects such as Compound Engineering and Matt Pocock's
skills have converged on similar approaches. There is substantial overlap:
AgentOps covers planning, implementation, testing, review, and deliberate reuse.
Use it across that workflow, or combine selected practices with other libraries.

| Project or tool | What it provides | How the pieces can work together |
|---|---|---|
| **AgentOps** | Skills for planning, implementation, testing, review, and reuse, plus CLI checks and evidence tools | Use the full workflow or selected skills, with BDD examples and fresh judgment tied to the accepted behavior |
| [Compound Engineering](https://github.com/EveryInc/compound-engineering-plugin) | A connected brainstorm, plan, build, review, and learning-capture workflow | Can supply the development loop and reusable solution records; carry the accepted behavior and evidence into independent judgment |
| [Matt Pocock's skills](https://github.com/mattpocock/skills) | Composable practices for clarifying intent, domain modeling, TDD, and review | Callers can select relevant practices alongside AgentOps skills |
| [Beads](AGENTS.md#repository-work-tracker) or your existing tracker | Work status, dependencies, and handoffs | Keeps ownership of work while AgentOps reads the accepted intent and cites its source |
| Software factories such as [Gas City](skills/using-gc/SKILL.md) | Coordinating and running agents | Own execution through their native control plane; AgentOps supplies guidance, checks, and independent review of results |

For example, a factory can coordinate a Beads-tracked task, an implementer can
use a TDD skill, and a fresh reviewer can check the result against the accepted
BDD example. Choose which workflow leads the task and make those handoffs
explicit. The Compound Engineering and Matt Pocock combinations are ways to
compose practices; the linked factory skill documents its supported integration.

Review and durable context are shared ideas across these projects.
See the [validation contract](skills/validate/SKILL.md) for AgentOps' review requirements.

## Choose skills by the work

Pick guidance for the task in front of you. These are independent choices, not
steps you must run in order.

| What you need | Skill | What to expect |
|---|---|---|
| Understand a behavior before changing it | [research](skills/research/SKILL.md) | An answer grounded in code and tests, with file references and gaps |
| Add tests for a behavior or regression | [test](skills/test/SKILL.md) | Tests using your repository's framework, plus the commands and results |
| Simplify code while preserving behavior | [refactor](skills/refactor/SKILL.md) | A focused change checked against the existing behavior |
| Clarify what a change should do | [plan](skills/plan/SKILL.md) | Observable behavior, acceptance examples, and an agreed scope |
| Implement an accepted change | [implement](skills/implement/SKILL.md) | A scoped implementation, relevant checks, and direct repair of known defects |
| Resolve ambiguous domain terms | [domain](skills/domain/SKILL.md) | Shared definitions and rule ownership used in examples, code, and tests |
| Independently review a finished change | [validate](skills/validate/SKILL.md) | A fresh review of the exact change against the accepted behavior; requires `ao` |

Use `/agentops:test` in Claude Code or `$agentops:test` in Codex to select Test,
and substitute another skill name when needed. Ordinary language also works:
"Use AgentOps Test to cover the missing edge case we just traced. Preserve the
current API and run the owning package checks."

The [Skill Router](docs/SKILL-ROUTER.md) lists all 34 skills, including
implementation, documentation, security, and memory. Installing a skill makes
it available; a clear task can proceed directly in your coding agent.

## Optional `ao` CLI

Install `ao` when you need its deterministic repository checks or evidence
commands, or when a selected skill requires it. Research, Test, and Refactor
can use your coding agent and the repository's existing tools.

With Homebrew:

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
ao version
ao quick-start
```

With Go installed:

```bash
go install github.com/boshu2/agentops/cli/cmd/ao@latest
```

`ao quick-start` gives read-only guidance; `ao demo` prints a sample coding task.
Neither writes project state. `ao init` is optional local evidence setup. See
[installation](docs/install-day2-ops.md#maintainer--contributor-the-ao-binary)
for source builds and [the command reference](cli/docs/COMMANDS.md) for checks,
inspection, and evidence operations.

| Command | Use it to |
|---|---|
| `ao quick-start` | Read the native execution brief, check requirements, and review responsibilities |
| `ao capabilities` | Discover the CLI's available capabilities |
| `ao gate check` | Run the selected repository checks; add `--full` for the whole registry |
| `ao config --show` | Inspect resolved configuration and its sources |

<details>
<summary>Inspect CLI configuration and request JSON output</summary>

Inspect the resolved configuration, or request the result as JSON:

```bash
ao config --show
ao config --show --json
```

The resolver reads project configuration from `.agents/ao/config.yaml` and home
configuration from `~/.agents/ao/config.yaml`. It reports flags ahead of
`AGENTOPS_*` environment variables, then project configuration, home
configuration, and defaults. Use `AGENTOPS_CONFIG` or `--config` to select a
file explicitly. Use a command's output flags to request its format; see
[configuration help](cli/docs/COMMANDS.md#ao-config) for the CLI surface.
Requested validation proof uses its separately selected external evidence root.

</details>

<details>
<summary>Skill dependencies: which ones need ao, Python, or another tool?</summary>

Most skills need nothing beyond the coding agent and your repository's tools.
These have additional requirements. "Conditional" means a selected task path
uses the tool; "optional" means the skill can complete without it.

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

Plugin installation and `npx skills@latest add boshu2/agentops --all -g` install
the catalog even when these dependencies are absent. See the
[installation guide](docs/install-day2-ops.md) before using a dependent skill.

</details>

## Other installation paths

Choose one source for each skill to avoid duplicate copies in your coding agent.

| Path | Use it when |
|---|---|
| Runtime plugin, shown above | You want a bundle managed through Claude Code or Codex |
| `npx skills@latest add boshu2/agentops --all -g` | You want the full library copied into supported coding agents |
| Checkout + `ao skills link` | You edit skills or want to expose a selected subset from source |

From an AgentOps checkout with `ao` installed, preview and link only the skills
you want:

```bash
ao skills link --skill test --skill refactor --dry-run
ao skills link --skill test --skill refactor
```

Omit the selectors to link the whole catalog. Linking preserves existing real
directories and foreign links. Follow the complete
[source checkout instructions](docs/install-day2-ops.md#install-source-checkout)
for cloning, updating, and removing owned links.

## Upgrading to 3.7

**Read the [migration guide](docs/MIGRATION.md) before upgrading from 3.6.**
Version 3.7 removes CLI commands and former skill names, including `learn`,
`codebase-recon`, and `swarm`. Their current owners are `memory`, `research`, and
`agent-native`; the guide covers the complete mapping and preserved evidence.

Plugin updates require refreshing both the marketplace and the installed bundle:

```bash
# Claude Code
claude plugin marketplace update agentops-marketplace
claude plugin update agentops@agentops-marketplace

# Codex, for a Git-backed marketplace
codex plugin marketplace upgrade agentops-marketplace
codex plugin add agentops@agentops-marketplace
```

Start a new session afterward. If you also use the Homebrew CLI, run
`brew update` followed by `brew upgrade agentops`. For local marketplaces,
copied skills, or source links, follow the
[update instructions](docs/install-day2-ops.md). New installs do not silently
remove obsolete copied skills; inspect those separately before removing them.

See the [3.7 release notes](docs/releases/2026-09-13-v3.7.0-notes.md) for the
full change list and validation limits.

## How independent review works

AgentOps is the operations layer for agentic engineering. It connects your
request, implementation, checks, and review while your existing tools keep
ownership of the work.

```text
Accepted behavior and domain terms
        |
        v
Coding agent or selected factory --> Code, tests, and check results
                                               |
                                               v
Accepted behavior ----------------> Fresh independent reviewer
                                               |
                                               v
                                    PASS / FAIL / NOT_PROVEN
                                               |
                                               v
                                    Your repository's delivery policy
```

Describe the outcome and acceptance examples in a conversation, issue, or your
tracker. Beads is an optional tracker. Let the coding agent implement and run
your repository's checks, then have a fresh context review the exact change
against that same request. By default, the reviewer uses the author's model
family; a different model is an explicit choice.

| Result | Meaning |
|---|---|
| `PASS` | The independent reviewer checked the full accepted scope and found evidence for every criterion |
| `FAIL` | A criterion failed or the change exceeded the authorized scope |
| `NOT_PROVEN` | Missing evidence, coverage, or reviewer independence prevents a complete judgment |

A passing test is evidence for review. The author cannot issue its own binding
`PASS`. When you request durable proof, Validate can save a `verdict.v2` record
with exact content identity, checked scope, and evidence references. New proof
uses caller-selected protected external non-Git storage; existing evidence is
preserved.

<details>
<summary>Architecture, memory, and multi-agent workflows</summary>

AgentOps connects caller-owned tools as a **federated integration graph**:
Git owns content and history, the tracker owns work, and the coding agent or a
selected factory owns execution. AgentOps supplies guidance, deterministic
checks, and independent judgment. Native execution needs zero mandatory skills.
The [architecture](docs/ARCHITECTURE.md) and
[operating contract](docs/agent-workflow-reference.md) describe these boundaries.

The [RPI charter](skills/rpi/SKILL.md) packages the workflow when explicitly
selected. [Memory](skills/memory/SKILL.md) provides on-demand recall and deliberate
curation of reviewed material. Saved lessons only demonstrate benefit when
later work uses them successfully; [ADR-0016](docs/adr/ADR-0016-state-tiers.md)
covers storage, disclosure, and preservation.

One agent and one writer are the default. For selected multi-agent work,
[agent-native](skills/agent-native/SKILL.md) dispatches focused tasks.
[Gas City](skills/using-gc/SKILL.md) and
[Agentic Coding Flywheel](skills/using-flywheel/SKILL.md) are optional factory
integrations. Their completion reports do not replace independent review.
See [model dispatch](skills/agent-native/references/model-dispatch.md) and
[the evidence contract](docs/architecture/rpi-traversal.md) for the detailed rules.

</details>

## Optional admission-control hooks

The Claude Code plugin includes a PreToolUse policy dispatcher: guards against
staging private tracker data, editing the provenance ledger by hand, and
overwriting installed skill copies. It runs before matching tool calls and
points blocked actions toward the supported command. Installing only `ao` does
not install these hooks.

Other install paths can opt in through the [CC Hooks skill](skills/cc-hooks/SKILL.md).
Disable the Claude plugin with `/plugin disable agentops` in Claude Code, or use
the terminal uninstall command shown above.

Read-budget guards remain separately opt-in in both runtimes. Codex custom roles
also require a separate installer and a restart; its hooks need review and trust
in the native hook manager. See [role and hook setup](docs/install-day2-ops.md#install-and-update-runtime-plugins)
for the exact steps and update requirements.

## Troubleshooting

| Symptom | What to check |
|---|---|
| `plugin` is not a recognized command | Update Claude Code or Codex to a version with plugin support, then retry its install commands |
| A skill is missing after installation or update | Check the plugin inventory with the Quickstart commands, then start a new session |
| `ao` is not found | Install the optional CLI and check your shell's PATH; Go installs usually place it in `$(go env GOPATH)/bin` |
| A skill asks for Python or another tool | Check the dependency table above; installing the skill does not install its dependencies |
| An old skill name no longer works | Use the current owner in the [migration guide](docs/MIGRATION.md#skills) and check for stale copies |

For a reproducible problem, [open an issue](https://github.com/boshu2/agentops/issues)
with the runtime version, install method, command or prompt, and observed result.
Include a saved verdict only if you requested one and it is safe to share.

## Limits

Skills guide the coding agent; their installation alone does not enforce every
instruction. Review quality depends on the accepted examples, evidence, and
reviewer. A green test suite or an agreeing second model can still miss a defect.
Missing proof stays `NOT_PROVEN`.

Reusable code, checks, and decisions can improve later engineering work. Saving
notes alone does not demonstrate that benefit, and AgentOps does not promise
automatic knowledge compounding. See [product evidence and limits](PRODUCT.md).

Your tracker, runtime or factory, and repository policy retain responsibility
for work ownership, execution, and delivery.

## FAQ

**Do I need the CLI to get started?**

No. Install the skills and try Research, Test, or Refactor in your coding agent.
Install `ao` for deterministic gates or a skill that requires it, including Validate.

**Do I need an orchestrator and three running agents?**

No. Start with one implementer and obtain a fresh, author-distinct review when
the change is ready. An orchestrator and parallel workers are optional.

**Must the reviewer use a different model provider?**

No. The default is a fresh context from the author's model family. Cross-model
review is an explicit choice; the reviewer must still examine the accepted
behavior and exact change independently.

**Can I keep my existing workflow and skills?**

Yes. Select guidance for the task and keep clear responsibility for planning,
execution, and review. See [where AgentOps fits](#where-agentops-fits) for
Compound Engineering, Matt Pocock's skills, trackers, and factories.

**Does durable work have to live in Git?**

No. Durability means the next session can recover the relevant work and its
context. Git keeps source history; a tracker can keep decisions and handoffs;
requested proof uses protected external storage. Follow the owning system's
retention and access rules.

## Contributing

Contributions are welcome: documentation fixes, reproducible bug reports,
tests, CLI improvements, and skills. Read the [contribution guide](docs/CONTRIBUTING.md)
for scope, checks, and pull request guidance.

Licensed under [Apache-2.0](LICENSE).
