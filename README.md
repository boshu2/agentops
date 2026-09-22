# AgentOps

**Stop rebuilding your coding workflow in every prompt.**

AgentOps is a library of engineering skills for coding agents. The skills tell
an agent how to turn a request into a plan, carry out the change, investigate
failed checks, and bring in a fresh reviewer. Longer jobs can add worker
coordination and shared project context.

Use one skill for a bug fix or combine them for a larger project. The instructions
are plain Markdown you can inspect and adapt. Your coding agent does the work
with your existing tests, issue tracker, and Git workflow.

[Install](#quickstart) · [Why these skills exist](#why-these-skills-exist) ·
[Skill library](#choose-skills-by-the-work) · [Documentation](docs/documentation-index.md)

<a id="install"></a>

## Quickstart

Choose one installation method. The Claude Code and Codex plugins manage the
skill bundle; the Skills installer lets you choose skills for Cursor or another
supported agent. Installing the same skill through both can leave duplicate copies.

### 1. Get the skills

<details>
<summary><strong>Claude Code</strong></summary>

<a id="claude-code"></a>

Run in your terminal:

```bash
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace
claude plugin details agentops@agentops-marketplace
```

Check that `agentops` appears in the plugin inventory. The bundle includes the
[current skill catalog](docs/SKILL-ROUTER.md), four agents, and
[tool-call guards](#optional-admission-control-hooks).

</details>

<details>
<summary><strong>Codex</strong></summary>

<a id="codex"></a>

Run in your terminal:

```bash
codex plugin marketplace add boshu2/agentops
codex plugin add agentops@agentops-marketplace
codex plugin list --json
```

Check that `agentops` appears in the plugin inventory. Its skills use the
`agentops:` prefix. Additional agent roles and optional read limits have
[separate setup](docs/install-day2-ops.md#install-and-update-runtime-plugins).

</details>

<details>
<summary><strong>Cursor and other agents</strong></summary>

With Node.js installed, run this from your project directory:

```bash
npx skills@latest add boshu2/agentops --agent cursor
```

Select `research` for the first task below, or choose the whole library. To pick
a different agent interactively, omit `--agent cursor`. Add `-g` if you want a
user-level installation instead of a project installation.

Cursor discovers skills from its [native skill directories](https://cursor.com/docs/skills).
Open its skill picker and check the installed skill's source path. The
[host coverage guide](docs/contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping)
distinguishes installation checks from observed runtime behavior.

</details>

### 2. Open a new session

Start a new agent conversation in a project you already work on. Skills use
your coding agent's normal permissions. The first task below is read-only and
doesn't need the optional `ao` CLI.

<a id="try-one-task"></a>

### 3. Try one task

Paste this into your agent conversation:

```text
Use the AgentOps Research skill to trace how this repository validates user
input. Follow one path from the input through its checks and tests. Cite the
files and line numbers, explain one edge case, and identify missing coverage.
Name the Research skill file you loaded. Answer here without changing files.
```

**Check the result:** you should get a trace you can follow in the code, with
file references and any gaps the agent couldn't resolve. Use those references
to request a fix or a regression test. The reported skill path also helps you
catch a missing or duplicate installation.

You can select Research directly with `/agentops:research` in Claude Code,
`$agentops:research` in Codex, or `/` and the installed Research entry in Cursor.

## Why these skills exist

An agent can produce a plausible patch while missing the request, repeating a
failed approach, or leaving the next session to reconstruct what happened.
AgentOps puts instructions for handling those problems into skills you can
read, choose, and reuse.

### The agent starts building before the request is clear

[Plan](skills/plan/SKILL.md) checks the code and docs, asks you about decisions
that change the outcome, and turns the request into observable examples. It
prepares one complete change to build and test. If an assumption needs checking,
it can run a small experiment before committing to an approach.

For an API change, "validate email" leaves room for guesses. "Return HTTP 400
for an empty email and keep valid requests working" gives the implementer and
reviewer the same behavior to check. When work resumes, Plan reads the handoff
and preserves decisions already made.

<a id="how-independent-review-works"></a>

### A passing test misses part of the request

[Implement](skills/implement/SKILL.md) carries the agreed change through the
repository's checks and repairs known failures. [Test](skills/test/SKILL.md)
helps choose behavioral and regression tests. [Review](skills/review/SKILL.md)
provides advice when you want another look at a plan, design, or patch.

To establish whether the job is complete, [Validate](skills/validate/SKILL.md)
brings a separate reviewer with a fresh context to the exact change and the
original request. The author cannot issue its own binding approval. Validate
requires the [optional `ao` CLI](#optional-ao-cli).

Illustrative result for the email example:

```text
Request: Reject an empty email with HTTP 400. Keep valid requests working.
Evidence: Valid requests checked. Empty-email behavior not checked.
Verdict: NOT_PROVEN. The empty-email requirement still needs evidence.
```

The result is `PASS` when every accepted criterion has evidence, `FAIL` when a
criterion fails or the change exceeds scope, and `NOT_PROVEN` when evidence or
reviewer independence is missing. Advice doesn't substitute for this judgment.
If your request could mean advice or acceptance, the agent clarifies it first.

### Parallel agents get in each other's way

[Orchestrate](skills/orchestrate/SKILL.md) checks prerequisites and active
assignments before splitting work. It gives workers separate write scopes,
brings their changes together, and reserves time for independent review of the
combined result. [Agent Native](skills/agent-native/SKILL.md) supplies optional
dispatch guidance.

Start with one agent. Add workers when the work can be separated. Your tracker
still records assignments and status; your coding runtime runs the agents.

<a id="shared-project-context"></a>

### The next session has to rediscover decisions

[Memory](skills/memory/SKILL.md) finds relevant project notes and checks them
against current code and docs. A project can opt into a `.context/README.md`
that points to topic pages and their sources. This repository's
[context map](.context/README.md) shows the layout. Reading cleared notes uses
ordinary filesystem tools and needs neither `ao` nor Beads.

Adding or correcting shared notes requires authorized sources, a selected
destination, and fresh review of factual support and disclosure before Git
admission. Drafts and review evidence stay in protected storage outside Git.
Memory creates no context store or private import automatically. These procedures
don't enforce access permissions or prove that saved notes improve later work.
See [Memory's storage rules](skills/memory/SKILL.md#access-storage-and-honest-limits).

## Choose skills by the work

These are independent entry points. Pick the one your task needs.

| Skill | Use it to |
|---|---|
| [Plan](skills/plan/SKILL.md) | Clarify a change or resume from an existing handoff |
| [Implement](skills/implement/SKILL.md) | Complete an accepted change or service operation, including checks and repairs |
| [Review](skills/review/SKILL.md) | Get findings and suggestions on a plan, design, or change |
| [Validate](skills/validate/SKILL.md) | Get an independent acceptance judgment on the exact result |
| [Orchestrate](skills/orchestrate/SKILL.md) | Coordinate workers and bring their changes together for review |
| [Memory](skills/memory/SKILL.md) | Find, capture, and maintain reviewed project context |

The library also includes focused skills for [research](skills/research/SKILL.md),
[testing](skills/test/SKILL.md), [refactoring](skills/refactor/SKILL.md),
[documentation](skills/doc/SKILL.md), and [security](skills/security/SKILL.md).
Browse the [full skill catalog](docs/SKILL-ROUTER.md).

For example, use `/agentops:test` in Claude Code or `$agentops:test` in Codex.
Ordinary language works too: "Use AgentOps Test to cover the missing edge case
we just traced. Preserve the current API and run the owning package checks."

## Optional `ao` CLI

The skills are Markdown instructions. `ao` adds deterministic repository checks
and tools for identifying and saving evidence. Install it when a selected
skill requires it or you want those commands.

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
ao version
ao quick-start
```

With Go installed, use `go install github.com/boshu2/agentops/cli/cmd/ao@latest`.
`ao quick-start` gives read-only guidance; `ao demo` prints a sample coding task.
Neither writes project state. `ao init` is optional local evidence setup.
See the [command reference](cli/docs/COMMANDS.md) and
[installation guide](docs/install-day2-ops.md) for source builds and skill dependencies.

## Updating and advanced setup

<details>
<summary><strong>Upgrading to 3.7</strong></summary>

<a id="upgrading-to-37"></a>

Read the [migration guide](docs/MIGRATION.md) before upgrading from 3.6. Version
3.7 removes CLI commands and former skill names, including `learn`,
`codebase-recon`, and `swarm`; their current owners are `memory`, `research`, and
`agent-native`.

Refresh both the marketplace and the installed plugin:

```bash
# Claude Code
claude plugin marketplace update agentops-marketplace
claude plugin update agentops@agentops-marketplace

# Codex, for a Git-backed marketplace
codex plugin marketplace upgrade agentops-marketplace
codex plugin add agentops@agentops-marketplace
```

Start a new session afterward. For Homebrew, run `brew update` followed by
`brew upgrade agentops`. For Skills installer copies, use `npx skills update`.
Follow the [update guide](docs/install-day2-ops.md#update) for other install paths
and obsolete copies; new installs don't silently remove old skills.
See the [3.7 release notes](docs/releases/2026-09-13-v3.7.0-notes.md) for the full changes.

</details>

<details>
<summary><strong>Source installs and skill dependencies</strong></summary>

<a id="other-installation-paths"></a>

From an AgentOps checkout with `ao` installed, preview and link selected skills:

```bash
ao skills link --skill test --skill refactor --dry-run
ao skills link --skill test --skill refactor
```

Omit selectors to link the whole catalog. Linking preserves existing real
directories and foreign links. See the [source checkout instructions](docs/install-day2-ops.md#install-source-checkout)
for cloning, updating, choosing a destination, and removing owned links.

Some skills need `ao`, Python, or a specialist tool. Installation doesn't
install those dependencies. Check the [installation guide](docs/install-day2-ops.md)
and selected skill before using it. The [host mapping](docs/contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping)
documents coverage and limits for each runtime.

</details>

<details>
<summary><strong>Permissions, optional hooks, and removal</strong></summary>

<a id="optional-admission-control-hooks"></a>

The Claude Code plugin includes a PreToolUse policy dispatcher. Its guards
check for private tracker data in commits, manual provenance-ledger edits, and
overwrites of installed skills. Installing only `ao` doesn't install these hooks.
Other install paths can opt in through [CC Hooks](skills/cc-hooks/SKILL.md).

Read-budget guards and Codex custom roles require separate setup. Codex hooks
need review and trust in its native hook manager. See the
[role and hook instructions](docs/install-day2-ops.md#install-and-update-runtime-plugins).

Disable the Claude plugin with `/plugin disable agentops` in Claude Code.
To remove a plugin from your terminal, use `claude plugin uninstall
agentops@agentops-marketplace` or `codex plugin remove
agentops@agentops-marketplace`.

</details>

<details>
<summary><strong>Architecture and saved review evidence</strong></summary>

AgentOps is the operations layer for agentic engineering. Git stores content
and history, your tracker records work, and your coding agent or selected
factory runs it. AgentOps supplies guidance, checks, and independent judgment.
Your repository keeps its delivery rules.

Native execution needs no mandatory AgentOps skills. The [RPI charter](skills/rpi/SKILL.md)
packages a workflow when selected. [Gas City](skills/using-gc/SKILL.md) and
[Agentic Coding Flywheel](skills/using-flywheel/SKILL.md) are optional integrations.
Their completion reports don't replace independent review.

By default, the reviewer uses the author's model family; a different model is
an explicit choice. On request, Validate can save a `verdict.v2` record identifying
the exact content, checked scope, and evidence. New proof belongs in selected,
protected storage outside Git; existing evidence is preserved. Beads is optional.

Read the [architecture](docs/ARCHITECTURE.md), [operating contract](docs/agent-workflow-reference.md),
and [storage rules](docs/adr/ADR-0016-state-tiers.md) for the details.

</details>

## Troubleshooting

| Symptom | What to check |
|---|---|
| `plugin` is not a recognized command | Update Claude Code or Codex to a version with plugin support, then retry |
| A skill is missing | Check its plugin inventory or skill picker, then start a new session |
| `ao` is not found | Install the CLI and check PATH; Go installs usually use `$(go env GOPATH)/bin` |
| A skill asks for Python or another tool | Check the skill's dependencies in the installation guide |
| An old skill name no longer works | Check the [migration guide](docs/MIGRATION.md#skills) and stale copies |

For a reproducible problem, [open an issue](https://github.com/boshu2/agentops/issues)
with your runtime version, install method, command or prompt, and observed result.
Include saved evidence only when it's safe to share.

Some engineering guidance draws on [Matt Pocock's skills](https://github.com/mattpocock/skills),
including concrete acceptance examples, domain language, and interface-focused
tests. These are design influences; they don't establish measured improvements
in coding outcomes.

Contributing: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md). License: Apache-2.0.
