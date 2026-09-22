# AgentOps

AgentOps gives Claude Code and Codex reusable instructions, called skills, for
investigating code, writing useful tests, and independently reviewing changes.
Start with one task and the guidance it needs; the full skill library is available
without adopting a new workflow.

When a coding agent says a change is done, AgentOps helps a fresh reviewer check
that exact change against what you asked for. Your coding agent runs the work;
your repository keeps its existing tests, tracker, and Git workflow.

[Quickstart](#quickstart) · [Choose a skill](#choose-skills-by-the-work) ·
[Optional CLI](#optional-ao-cli) · [Upgrade](#upgrading-to-37) ·
[Documentation](docs/documentation-index.md)

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

The plugin should appear as `agentops`. Claude's inventory includes the
[current skill catalog](docs/SKILL-ROUTER.md) and four agents; Codex exposes the
same skills with `agentops:` names. Start a new
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

## Choose skills by the work

Pick guidance for the task in front of you. These are independent choices, not
steps you must run in order.

| What you need | Skill | What to expect |
|---|---|---|
| Clarify what a change should do | [plan](skills/plan/SKILL.md) | Concrete acceptance examples and an agreed scope |
| Complete an accepted change or service operation | [implement](skills/implement/SKILL.md) | A complete change, meaningful checks and factual results |
| Independently judge a finished change | [validate](skills/validate/SKILL.md) | Fresh acceptance judgment of exact content; requires `ao` |
| Coordinate authorized workers or recover assignments | [orchestrate](skills/orchestrate/SKILL.md) | Actual prerequisites, isolated scopes, review capacity and native handoffs |
| Find relevant experience or maintain shared context | [memory](skills/memory/SKILL.md) | Selective recall or reviewed corrections when useful |

Focused methods remain directly available: [Research](skills/research/SKILL.md)
traces a source question, [Test](skills/test/SKILL.md) develops behavioral tests,
and [Refactor](skills/refactor/SKILL.md) preserves behavior while simplifying code.
Advisory review and plan challenge do not replace Validate's acceptance judgment.

Use `/agentops:test` in Claude Code or `$agentops:test` in Codex to select Test,
and substitute another skill name when needed. Ordinary language also works:
"Use AgentOps Test to cover the missing edge case we just traced. Preserve the
current API and run the owning package checks."

The [Skill Router](docs/SKILL-ROUTER.md) lists every current skill, including
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
| `npx skills@latest add boshu2/agentops --all -g` | You want the full library installed for every agent supported by the Skills installer |
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
The [host/install mapping](docs/contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping)
also accounts for Cursor/OpenCode structural coverage, Gemini/Antigravity
compatibility packaging and other source-link consumers. Package installation
and CLI availability do not establish actual skill loading or execution.

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
Accepted intent -> native implementation and checks -> fresh independent judgment -> finish
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

Some engineering guidance draws on
[Matt Pocock's skills](https://github.com/mattpocock/skills), including concrete
acceptance examples, domain language, and interface-focused tests. These are
design influences; they do not establish measured improvements in coding outcomes.

Contributing: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md). License: Apache-2.0.
