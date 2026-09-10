# AgentOps

AgentOps is the operations layer for agentic engineering. It is a set of
deterministic tools, optional skills and evidence contracts that make one coding-agent change
independently judgeable: the context that wrote the code does not get to
declare it done. Your tracker keeps the work, Git keeps the history, and your
coding agents keep running the execution; AgentOps joins them as a
federated integration graph and adds the judgment step. A fresh context reads
the exact change and returns `PASS`, `FAIL`, or `NOT_PROVEN`. The standard
path uses your native coding agent with zero mandatory AgentOps skills:

```text
Accepted intent -> native implementation and checks -> fresh independent judgment -> finish
```

Give the agent a concrete outcome, acceptance examples and the repository's
checks. Let it implement, repair understood failures and obtain a fresh review.
A native goal can carry continuity; your tracker keeps work and handoffs.
Finish at accepted work. A clear small edit needs no planning or memory worksheet.
The [RPI charter](skills/rpi/SKILL.md) remains an optional workflow, selected
when its guidance helps the task.

[Memory](skills/memory/SKILL.md) offers on-demand recall and separately budgeted
mining/curation over reviewed caller-selected external Markdown topic pages.
Update an existing page; preserve support, limits and invalidation. Learning may
remove rules. Saved pages do not prove benefit; only later task evidence does.
This lean path uses public or already-cleared inputs and claims no native
restricted-source enforcement. Protected external drafts, review before Git and
legacy evidence preservation follow [ADR-0016](docs/adr/ADR-0016-state-tiers.md).

## Quickstart

```bash
go install github.com/boshu2/agentops/cli/cmd/ao@latest
ao quick-start
```

The native path needs no skill bundle, hook, `ao init`, or session bootstrap.
`ao quick-start` gives read-only guidance; `ao demo` prints a sample coding
task. Use `ao` for a specific check or evidence operation when useful.
See [installation](docs/install-day2-ops.md) for Homebrew and source builds.

## Optional skill library

From an AgentOps checkout, expose only guidance you want:

```bash
ao skills link --skill test --skill refactor --dry-run
ao skills link --skill test --skill refactor
```

Skills run **inside your coding agent** (Claude Code, Codex, Cursor, …).
Linking makes them discoverable; it does not require invoking them. Select a
skill for a concrete task need. An explicit full install remains supported:
`npx skills@latest add boshu2/agentops --all -g`, or `ao skills link` without
selectors from a checkout. Existing installations are preserved.

Most skills need nothing beyond the coding agent; these need more:

| Skill | Needs | Why |
|---|---|---|
| `rpi` | `ao`, conditional | delegates exact-subject checks to Validate; only persists `verdict.v2` when requested, with the fixed-dispatch adapter optional |
| `plan` | `ao`, conditional | runs `ao provenance snapshot-intent` with an explicit evidence root when the intent source is not durable |
| `validate` | `ao` | derives exact subject identity with the helper and uses `ao provenance store-verdict` when persistence is requested; Python/schema checks are developer-only |
| `fitness` | `ao` | its whole procedure is running one `ao goals` subcommand |
| `using-gc` | `ao` | rig prep runs `ao gc prepare` and `ao gc check` |
| `handoff` | `ao`, optional | `ao session handoff`/`rehydrate` cover the same artifact; the skill can write it directly |
| `status` | `ao`, optional | describes `ao status`'s output shape; the report can be read directly from `.agents/ao/` |
| `reverse-engineer` | `python3` | Phase 1's mechanical teardown runs `scripts/reverse_engineer.py` |
| `skill-builder` | `python3`, conditional | Create mode's `build.sh` runs `scripts/generate-skill-mesh.py`; heal/check/audit modes are bash-only |
| `ms` | `python3`, conditional, plus `ms` binary | the MCP-search fallback runs `python3 skills/ms/scripts/mcp-search.py`; the `ms` binary is required for CLI load, write, and admin operations |
| `toil-mining` | `python3`, conditional | the recent-human extractor runs `scripts/recent_human.py` for Codex JSONL session sources |
| `security` | `python3`, conditional | the composable suite and offline redteam surfaces run `security_suite.py` when that scan type is selected |
| `cass` | `python3`, optional | `scripts/prompt_miner.py` mines repeated prompts; one of several selectable Scripts-table entries |

The plugin and `npx skills@latest add boshu2/agentops --all -g` install the generated skill catalog, regardless of whether you have `python3` or `ao`.

Ran it? Tell us what it judged. Open an issue, and paste the `verdict.v2` if
you asked `validate` to persist one:
<https://github.com/boshu2/agentops/issues>.

## Plugins (Claude Code / Codex)

For an explicit full managed bundle that updates with the release:

```bash
# Claude Code
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace

# Codex
codex plugin marketplace add boshu2/agentops
codex plugin add agentops@agentops-marketplace
```

Three optional skill installation paths:

- **npx / [skills.sh](https://skills.sh)**: universal; copies skills you can edit.
- **Plugins**: a read-only bundle that stays current with the repo.
- **Checkout + `ao skills link`**: source-tracked symlinks for contributors
  (see [Install and day-2 operations](docs/install-day2-ops.md)).

## Optional admission-control hooks

AgentOps ships a PreToolUse **policy dispatcher**: deterministic guards that
block a small set of known-destructive commands (staging the private bead
ledger, hand-editing the hash-chained provenance ledger, overwriting installed
skill copies) and route you to the correct tool instead. Silent on every clean
call; every block is one line.

- **Claude Code plugin installs:** active automatically; nothing to run.
- **npx / skills.sh copies:** run `~/.claude/skills/cc-hooks/scripts/install-hooks.sh` once.
- **git clone / brew:** opt in with `scripts/install-policy-dispatch.sh`.

AO-only installation does not install hooks. The Claude plugin enables its
bundled hooks when that optional installation is selected.

Disable anytime (`/plugin disable agentops`, or remove the two PreToolUse
matchers from settings). Policy list and design:
`skills/cc-hooks/SKILL.md`.

Remove with your runtime's plugin uninstall, or delete the linked skill
directories.

## Intent lives in a bead

[Beads](https://github.com/steveyegge/beads) is the preferred tracker
(optional; `brew install beads`). Use [BDD](https://cucumber.io/docs/bdd/)
acceptance examples and DDD [ubiquitous language](https://martinfowler.com/bliki/UbiquitousLanguage.html)
when they clarify behavior and ownership boundaries. The native agent implements
that intent; a fresh reviewer judges exact content against it. An issue or
conversation also works. Snapshot mutable intent only when needed; new persisted
proof requires protected external non-Git storage (ADR-0016).

Review runs in a fresh context from the author's model family by default:
Codex reviews Codex work, and Claude reviews Claude work. Request
`--cross-model [model]` in Validate or RPI to add a different-family reviewer;
an unavailable requested leg leaves the combined result unproven. Review time
comes from caller/native bounds, with no fixed ten-minute cap. See the
[model-dispatch recipe](skills/agent-native/references/model-dispatch.md).

## Multi-agent systems

The default is one agent, one writer. When you need a fleet,
[`swarm`](skills/swarm/SKILL.md), [`agent-native`](skills/agent-native/SKILL.md),
[`ntm`](skills/ntm/SKILL.md), and [`using-gc`](skills/using-gc/SKILL.md)
orchestrate multi-agent work. They dispatch; they do not own the verdict.

### Choose a software factory

AgentOps supplies skills and evidence contracts, not another software-factory
runtime or a competing Gas City pack. Install the skills in the agent runtime
used by the factory you choose; its Mayor, coordinator, and workers can then use
`plan`, `implement`, `test`, `validate`, and the rest of the catalog.

Two factory stacks are supported:

- [Gas City](https://github.com/gastownhall/gascity) is the preferred choice
  for durable, supervised workflows. Use the upstream
  [`gascity` build pack](https://github.com/gastownhall/gascity-packs/tree/main/gascity),
  the workflow family used by Maintainer City. It owns formulas, roles,
  worktrees, dispatch, draining, and run state. The
  [`using-gc`](skills/using-gc/SKILL.md) skill covers installation, launch,
  observation, and recovery.
- Jeffrey Emanuel's
  [Agentic Coding Flywheel](https://agent-flywheel.com) is a supported
  alternative built from Beads, Agent Mail, NTM, and the wider Flywheel tool
  stack. Use its native workflow and let its agents consume the same AgentOps
  skills. The [`using-flywheel`](skills/using-flywheel/SKILL.md) skill covers
  provisioning, skill visibility, and the evidence boundary.

AgentOps does not wrap either factory or translate factory completion into
semantic PASS. When proof is required, a fresh reviewer judges the
exact candidate and evidence.

## Deterministic `ao` CLI

Deterministic checks, inspection, and optional skill linking. Dependency
requirements vary by skill as listed above. Install
steps (Homebrew or `go install`), and `ao skills link` for
tracking skills from a local checkout:
[Install and day-2 operations](docs/install-day2-ops.md#maintainer--contributor-the-ao-binary).

## Why AgentOps exists

### 1. The agent said it was done

Same session that wrote the code also declared victory. AgentOps separates
authorship from judgment: the coding agent produces a candidate; a reviewer
runs in a fresh context and may use a different model. It issues `PASS`,
`FAIL`, or `NOT_PROVEN`.

### 2. One perspective rubber-stamped another

A single context can share blind spots with the author. Opt into
[`idea-genie`](skills/idea-genie/SKILL.md) or [`council`](skills/council/SKILL.md)
for sealed or multi-judge review. They return a report; an author-distinct
reviewer issues the binding result, optionally guided by [`validate`](skills/validate/SKILL.md).

### 3. Acceptance drifted mid-flight

Keep accepted behavior and write scope in the existing intent source. Use
`plan` when they need shaping; revise the approach when evidence requires it,
without silently changing acceptance. Validation binds to that accepted intent.

### 4. Nobody can replay what was judged

Chat scrolls away. When replay or automation needs durable evidence, the reviewer
writes a content-addressed `verdict.v2` in caller-selected protected external
non-Git storage, with checked scope, omissions, and evidence refs. Existing
`.agents/` proof remains preserved under owner policy.
Plain JSON. No hosted service required. Interactive validation does not create
one unless requested.

## Optional workflow skills

| Skill | Job |
|---|---|
| [`rpi`](skills/rpi/SKILL.md) | own the authorized outcome through checks, direct repair and fresh final judgment |
| [`plan`](skills/plan/SKILL.md) | shape existing intent when needed; revise disproved approaches within accepted scope |
| [`implement`](skills/implement/SKILL.md) | implement and repair known defects with discriminating checks |
| [`validate`](skills/validate/SKILL.md) | fresh context (optionally different model); optionally persist `verdict.v2` |
| [`memory`](skills/memory/SKILL.md) | recall reviewed topic pages or separately mine and curate when useful |

Optional later: [`learn`](skills/learn/SKILL.md). Optional strategies:
[`anti-ceremony`](skills/anti-ceremony/SKILL.md),
[`council`](skills/council/SKILL.md), [`idea-genie`](skills/idea-genie/SKILL.md),
[`premortem`](skills/premortem/SKILL.md), [`postmortem`](skills/postmortem/SKILL.md),
[`one-way-door`](skills/one-way-door/SKILL.md) (is this decision reversible?),
[`reality-check`](skills/reality-check/SKILL.md) (does the repo match the claim?).
Not sure which skill owns a request? Ask [`route`](skills/route/SKILL.md).

## One skill, many shapes

AgentOps prefers a smaller skill set you can steer over dozens of near-duplicate
skills. Modes and flags change behavior inside one contract.

| Skill | Steer with | Examples |
|---|---|---|
| [`doc`](skills/doc/SKILL.md) | `--mode` | `readme`, `oss`, default API/docs; README mode runs a docs-prose (de-slop) pass |
| [`codebase-recon`](skills/codebase-recon/SKILL.md) | mode · view · lens · depth | `baseline`/`delta`; emphasize audit or mental model; one domain lens per pass |
| [`idea-genie`](skills/idea-genie/SKILL.md) | elicit \| duel | portfolio vs sealed multi-perspective challenge |
| [`rpi`](skills/rpi/SKILL.md) | bead / intent ref | one full traversal against a frozen bead |

Read the skill's mode table before inventing a sibling skill. Full inventory:
[Skill Router](docs/SKILL-ROUTER.md).

## Evidence contract

A `PASS` binds unchanged acceptance, a deterministic subject manifest, complete
changed-path coverage inside write scope, distinct author and validator context
IDs, a freshness attestation, and criterion-level evidence.

Missing identity, mutation, or incomplete coverage → `NOT_PROVEN`. Proven
out-of-scope change or failed criterion → `FAIL`.

[RPI traversal](docs/architecture/rpi-traversal.md) · [CLI](cli/docs/COMMANDS.md) · [Docs](docs/documentation-index.md)

Contributing: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md). License: Apache-2.0.
