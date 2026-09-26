# AgentOps migration

AgentOps now owns one small product boundary:

```text
Accepted intent -> native implementation and checks -> fresh independent judgment -> finish
```

(The 3.0 through 3.6 releases stopped after one validation; ADR-0017 added the
bounded repair phase.)

Known defects can be repaired directly and an approach can change within the
accepted outcome, scope and real allowance. The caller owns changes to that
acceptance, new allowances, scheduling, trackers, Git and delivery. Deterministic repository checks stay
under `ao gate check`; semantic judgment belongs to a fresh reviewer.

## Native default — 2026-09-10

The default requires zero AgentOps skills. `ao quick-start` and `ao demo` give
read-only native guidance. `ao demo --rpi` retains the explicit workflow example.
`ao init` remains optional evidence setup. No session bootstrap is required.

The skill sources, explicit full plugins and no-selector `ao skills link` are
preserved. From a checkout, repeat `--skill NAME` to link selected skills only.
Selection does not uninstall existing links or rewrite user configuration.
The optional RPI and Validate skills retain their contracts; native work uses
the same exact-content, author-distinct judgment bar without invoking them.

## Removed responsibilities

| Removed command or surface | Surviving alternative |
|---|---|
| `ao pawl` | Invoke the Validate skill for one independent semantic verdict. |
| `ao plan-pawl` | Invoke `premortem` when the caller wants an advisory plan challenge. |
| `ao validate` | Use the Validate skill; use `ao gate check` only for deterministic checks. |
| `ao land` | Use the repository's Git or CI delivery process. |
| `ao done`, `ao close` | Report the result to the caller; AgentOps does not close work. |
| `ao governor`, `ao converge` | The caller decides whether to start a new invocation or revision. |
| `ao yield` | Observe throughput in the selected runtime or external system. |
| `ao claim`, `ao next-work` | Use the caller's tracker or substrate directly. |
| `ao state`, `ao reconcile` | Inspect the bead or caller intent, derived subject manifest, verdict, and generic provenance as read-only evidence. |
| `ao worktree` | Use Git directly. |
| `ao membrane` | Record observations as Validate findings or generic provenance. |
| `ao crank` | Call an executor directly or use the optional `dispatch_once` adapter. |
| `ao constraint` | Encode accepted mechanical policy in repository-owned linters or checks; AgentOps no longer promotes findings into blocking state. |
| `ao skills edit` | Edit canonical `skills/<slug>/` sources directly; use normal repository Git policy outside `ao`. |
| `ao goals trace` | Inspect current goal/scenario artifacts directly; the retired directive-to-bead lifecycle chain has no replacement. |
| `ao inject` | Use optional Memory recall over caller-owned, authorized context sources; there is no replacement CLI retrieval controller. |
| `ao session memory` | Use caller-authored `ao session handoff` evidence or maintain repository memory through the caller's own policy. |
| `ao config models` | Model-tier configuration was removed; nothing consumed it. Model choice belongs to the caller's runtime. Existing `models:` config sections still parse and are ignored. |
| `ao verify` | Use the Validate skill for semantic judgment and `ao gate check` for deterministic checks. Delete any `ao verify init` pre-push ratchet from `.git/hooks/pre-push` (restore `pre-push.agentops-orig` if one was set aside); `ao verify init --remove` no longer exists, and `git push --no-verify` bypasses a stale hook once. |
| `ao flywheel` | The CLI surface remains retired; existing `flywheel:` config sections still parse and are ignored. The optional Memory skill can review useful episodes and curate topic pages. They do not automatically compute compounding or claim benefit without later work. |
| `ao eval` | The offline eval surface was retired unconsumed (no gate, workflow, or script ran it); use a repository-selected evaluator and record the result as generic `ao provenance` evidence. |
| `ao redact` | Its only declared caller (the compile skill's render-write) never existed. Use owner-authorized disclosure review before storage; removing or replacing this command does not authorize reading restricted sources. |

These names are no longer registered commands. Invoking one fails as an
unknown command (exit 1) and prints the matching replacement pointer from the
table above; nothing forwards to old code or mutates old state.

Other 3.2 bookkeeping and knowledge verbs (`ao beads`, `ao agents`, `ao canon`,
`ao ci`, `ao citation`, `ao findings`, `ao forge`, `ao knowledge`,
`ao mcp`, `ao metrics`, `ao notebook`, `ao patterns`, `ao pool`, `ao ratchet`,
`ao registry`, `ao scope`, `ao sessions`, `ao wiki`) were pruned from the
default build without tombstones. They have no replacement inside AgentOps; use
the caller's own tools, `ao gate check` for deterministic checks, or generic
`ao provenance` records.

## Config file location

`~/.agentops/config.yaml` and `./.agentops/config.yaml` moved to
`~/.agents/ao/config.yaml` and `./.agents/ao/config.yaml`. The legacy paths are
still read as a fallback for this release (with a deprecation warning on
stderr) when no file exists at the new path; move the file to silence the
warning:

```bash
mkdir -p ~/.agents/ao && mv ~/.agentops/config.yaml ~/.agents/ao/config.yaml
```

## Upgrade from 3.6 to 3.7

Version 3.7 removes published commands and skill entry points. Update explicit
invocations before upgrading automation; the new names are not compatibility
aliases. Ordinary native coding requires no replacement invocation.

- Replace `ao eval` calls with the evaluator selected by your repository. Replace
  `ao redact` calls with the owner's disclosure-review process. Both command
  names now fail with a migration pointer. Generic `ao provenance` records can
  retain the resulting facts, but do not run either retired service.
- Replace scripted `workflows/rpi.js` invocations with native execution, or invoke
  the optional RPI skill explicitly. The skill retains the outcome-to-judgment
  contract; it does not recreate the old script's retry controller.
- Existing `.agents/` evidence stays where its owner placed it. New CDLC proof
  requires a selected protected external non-Git destination; a missing route
  must not silently write new proof into the repository.
- The build toolchain is Go 1.27.1 (`cli/go.mod` still declares Go 1.26.0 as its
  language floor). An older local toolchain may download the selected toolchain
  or fail according to `GOTOOLCHAIN`.

## Skills

The current source distribution no longer bundles the Flywheel tool adapters
(`account-rotation`, `agent-mail`, `cass`, `cc-hooks`, `dcg`, `ms`, `ntm`, `rch`, `sbh`,
`using-flywheel`). Obtain these tools and skills from their authors. Existing
installed copies are not automatically deleted.

The 3.7 migration baseline has 34 skills, compared with 52 in 3.6.0. Twenty former roots
were retired; `memory` and `skill-eval` are new relative to that release. The
current menu additionally exposes Review, Orchestrate, Interview and Navigate; no baseline skill is retired or
renamed by that addition. Use
[the current menu](SKILL-ROUTER.md) to choose guidance for the actual task.

### Maintained entrypoints and baseline dispositions

The primary menu exposes Plan, Implement, Review, Validate, Orchestrate and Memory as
independent choices. Advisory review supplies findings; Validate alone owns
skill acceptance semantics. The optional RPI workflow and every focused method
remain explicitly invocable. Native work requires zero mandatory skills, and
selecting Orchestrate does not require a skill chain.

This is the literal disposition of all 34 canonical roots at migration baseline
`3842ea0e0040ddc06ee4d45c98b9fc8c0eeffb6a`, derived from their
`skills/<name>/SKILL.md` contracts. Each linked contract remains the method owner.
The [generated catalog](https://github.com/boshu2/agentops/blob/main/skills/catalog.json) and [router](SKILL-ROUTER.md)
describe the current inventory; fewer roots or improved discovery is not claimed.
For every **keep** row, its catalog slug remains compatible. Managed plugin
invocations remain `/agentops:<name>` in Claude and `$agentops:<name>` in Codex.
For source-linked installs, use the host's actual registered name, including any
namespace: observed Codex source Plan was registered as `agentops:plan`, invoked
as `$agentops:plan`. Check native inventory for the installed host version;
path, scope and plugin identity establish installation mechanism separately from
invocation spelling. The [host evidence limits](contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping)
apply, and host loading is qualified separately from package presence.

| Baseline entrypoint | Disposition | Maintained owner and outcome |
|---|---|---|
| `account-rotation` | external | Obtain the tool and its guidance from its author; see [recommendations](../README.md#recommended-tools-and-skills). |
| `agent-mail` | external | Obtain the tool and its guidance from its author; see [recommendations](../README.md#recommended-tools-and-skills). |
| `agent-native` | keep | [Agent Native](https://github.com/boshu2/agentops/blob/main/skills/agent-native/SKILL.md): runtime dispatch, observation, context identity, isolation and native follow-up. Orchestrate links these mechanics. |
| `agy-native` | keep | [AGY Native](https://github.com/boshu2/agentops/blob/main/skills/agy-native/SKILL.md): explicitly selected Antigravity execution. |
| `cass` | external | Obtain the tool and its guidance from its author; see [recommendations](../README.md#recommended-tools-and-skills). |
| `cc-hooks` | removed | AgentOps-native guard scripts now live under `hooks/guards/`; the Claude plugin retains its existing guards. No hook-configuration skill is bundled. |
| `codex-exec` | keep | [Codex Exec](https://github.com/boshu2/agentops/blob/main/skills/codex-exec/SKILL.md): one selected headless Codex process. |
| `council` | keep | [Council](https://github.com/boshu2/agentops/blob/main/skills/council/SKILL.md): selected independent perspectives; advice does not supply acceptance. |
| `craft-goal` | keep | [Craft Goal](https://github.com/boshu2/agentops/blob/main/skills/craft-goal/SKILL.md): explicitly selected persistent-goal guidance; native goals retain continuity. |
| `dcg` | external | Obtain the tool and its guidance from its author; see [recommendations](../README.md#recommended-tools-and-skills). |
| `doc` | keep | [Doc](https://github.com/boshu2/agentops/blob/main/skills/doc/SKILL.md): requested source-grounded documents and continuity handoffs. |
| `domain` | keep | [Domain](https://github.com/boshu2/agentops/blob/main/skills/domain/SKILL.md): domain vocabulary, rule boundaries and repository conventions. |
| `idea-genie` | keep | [Idea Genie](https://github.com/boshu2/agentops/blob/main/skills/idea-genie/SKILL.md): evidence-backed options and idea challenge. |
| `implement` | keep | [Implement](https://github.com/boshu2/agentops/blob/main/skills/implement/SKILL.md): complete accepted changes, direct repair and authorized operations. |
| `memory` | keep | [Memory](https://github.com/boshu2/agentops/blob/main/skills/memory/SKILL.md): selective find/recall, capture/mine and curate/qualify/retire with support and disclosure review. |
| `ms` | external | Obtain the tool and its guidance from its author; see [recommendations](../README.md#recommended-tools-and-skills). |
| `ntm` | external | Obtain the tool and its guidance from its author; see [recommendations](../README.md#recommended-tools-and-skills). |
| `plan` | keep | [Plan](https://github.com/boshu2/agentops/blob/main/skills/plan/SKILL.md): resumable discovery, uncertainty routing, optional challenge and one ready complete slice. |
| `postmortem` | keep | [Postmortem](https://github.com/boshu2/agentops/blob/main/skills/postmortem/SKILL.md): requested outcome analysis, separate from code acceptance. |
| `premortem` | keep | [Premortem](https://github.com/boshu2/agentops/blob/main/skills/premortem/SKILL.md): selected fresh plan challenge; Plan owns the shared challenge exchange. |
| `rch` | external | Obtain the tool and its guidance from its author; see [recommendations](../README.md#recommended-tools-and-skills). |
| `reality-check` | keep | [Reality Check](https://github.com/boshu2/agentops/blob/main/skills/reality-check/SKILL.md): compare shipped-feature, repository or goal claims with evidence. |
| `refactor` | keep | [Refactor](https://github.com/boshu2/agentops/blob/main/skills/refactor/SKILL.md): structural simplification while preserving behavior. |
| `research` | keep | [Research](https://github.com/boshu2/agentops/blob/main/skills/research/SKILL.md): cited source investigation and pattern evidence. |
| `reverse-engineer` | keep | [Reverse Engineer](https://github.com/boshu2/agentops/blob/main/skills/reverse-engineer/SKILL.md): authorized external-system teardown and adoption choices. |
| `rpi` | keep | [RPI](https://github.com/boshu2/agentops/blob/main/skills/rpi/SKILL.md): explicitly selected outcome-to-judgment charter; unchanged hard dependency graph. |
| `sbh` | external | Obtain the tool and its guidance from its author; see [recommendations](../README.md#recommended-tools-and-skills). |
| `security` | keep | [Security](https://github.com/boshu2/agentops/blob/main/skills/security/SKILL.md): concrete exposure review and selected scans. |
| `skill-builder` | keep | [Skill Builder](https://github.com/boshu2/agentops/blob/main/skills/skill-builder/SKILL.md): skill authoring, repair, projections and exports. |
| `skill-eval` | keep | [Skill Eval](https://github.com/boshu2/agentops/blob/main/skills/skill-eval/SKILL.md): bounded behavioral evaluation; structural conformance is not efficacy. |
| `test` | keep | [Test](https://github.com/boshu2/agentops/blob/main/skills/test/SKILL.md): behavioral test design and consequential coverage gaps. |
| `using-flywheel` | external | Obtain the tool and its guidance from its author; see [recommendations](../README.md#recommended-tools-and-skills). |
| `using-gc` | keep | [Using GC](https://github.com/boshu2/agentops/blob/main/skills/using-gc/SKILL.md): selected Gas City through its Mayor and supported native doors. |
| `validate` | keep | [Validate](https://github.com/boshu2/agentops/blob/main/skills/validate/SKILL.md): sole skill owner of fresh exact-content acceptance judgment and explicit missing proof. |

[Orchestrate](https://github.com/boshu2/agentops/blob/main/skills/orchestrate/SKILL.md) is an additive coordination entrypoint
for actual prerequisites, native assignments, isolated scopes, integration/review
capacity and affected-work feedback. Agent Native remains its runtime-mechanics
owner; Plan owns uncertainty and slicing; Memory owns shared-context procedures;
Validate owns acceptance. This is instruction-level coordination, not a new
scheduler, work account, queue, retry controller or delivery mechanism.

The old `learn` name maps to current `memory`. The proposed return to `learn`
is not selected: Memory keeps its public name and all three operation families.
Do not create a Learn alias or rename installed copies. Inspect stale owned
Learn links/copies and Memory collisions using the install recovery policy below;
retaining the name does not establish collision-free upgrades. Final installed
route, cold-resume and upgrade/recovery qualification remains required on the
integrated candidate. Structural checks establish no efficacy or host-load claim.

| Retired 3.6 skill | Current owner or migration |
|---|---|
| `learn`, `toil-mining` | `memory` for explicitly requested recall, mining or curation. |
| `codebase-recon`, `pattern-mining` | `research` for cited local questions, recon packs and pattern evidence. |
| `bootstrap`, `handoff` | `doc` for requested missing documents and factual continuity handoffs; native work needs no bootstrap. |
| `standards` | `domain` for repository conventions and their existing owners. |
| `converter`, `operationalize` | `skill-builder` for exports and supported expertise proposals. |
| `swarm` | `agent-native` for explicitly selected delegation; use the native runtime for ordinary execution. |
| `fitness`, `status` | `reality-check` for a requested comparison of claims with evidence; CLI status remains available. |
| `scope`, `product` | `plan` for missing acceptance/scope; `domain` for vocabulary; `doc` for a requested product document. |
| `goals` | Use the native goal/tracker; `craft-goal` remains optional guidance for an explicitly selected persistent-goal workflow. |
| `scaffold`, `workflow-builder` | Implement the requested repository change natively; use `skill-builder` only when the output is a skill. No generic workflow generator replaces these names. |
| `anti-ceremony`, `automation-shape-routing`, `shared` | No standalone invocation. The operating contract retains the artifact-creation boundary, runtime choice stays with the caller, and surviving skills link their needed references. |

### Unified discovery batch

The discovery batch retained all 34 baseline skill names. Use Plan
for resumable discovery of the next complete slice; focused entrypoints remain
available when their narrower method is the requested work.

| Existing entrypoint or method | Current owner and disposition in this batch |
|---|---|
| `plan`, including former `scope`/`product` intent shaping | Plan owns uncertainty routing, compact native resumption and the next complete slice. Existing caller intent remains the output owner. |
| `research` | Keep as the cited investigation, tracing and pattern-evidence specialist; Plan links it only for a source question. |
| `domain` | Keep as the vocabulary, bounded-context and standards specialist; settled terms return to Plan without a new interview. |
| Plan ground-truth routing and optional prototype | Plan's existing ground-truth reference owns a question-driven disposable probe. Mandatory stock controls and deviation ledgers are removed; useful integration comparisons remain. |
| `premortem` and `council` | Keep as caller-selected strategies. Plan owns the common optional challenge exchange; Premortem retains its evidence-shape, reversibility and defeat methods, and Council retains its selected broader review. Neither supplies acceptance. |

No new discovery root, mandatory stage, work ledger or automatic context
admission is introduced. The complete dispositions above preserve those methods.
A compact plan or native handoff can carry assignment references and the next
discriminator; status and ownership remain authoritative only in the native
tracker/runtime. Existing Plan/Implement/Validate authority and fresh exact-content
judgment are unchanged. This contract change makes no comparative benefit claim.

### Advisory Review batch

This batch adds the `review` public entrypoint, bringing the intermediate source
inventory from 34 to 35. That count is not a completed whole-library disposition
or proof of installed-host qualification.

| Existing request or consumer | Current owner and disposition in this batch |
|---|---|
| General feedback on a plan, design or change | New `review` gives supported advisory findings or an honest no-finding result with checked scope and gaps. It owns no acceptance, native work-state mutation or delivery. |
| Acceptance request, including a near-match phrased as review | `validate` retains sole acceptance/verdict ownership and genuinely fresh exact-subject judgment. Review hands off original acceptance and the exact subject; ambiguity is clarified. |
| Plan challenge, broader selected review or claim audit | Keep `premortem`, `council` and `reality-check`; Review selectively links their existing procedures and Plan's one shared optional challenge owner. |
| Engineering advice and prior evidence | Keep the relevant engineering specialists and `memory`; references create no compulsory chain, editing authority or curation. |
| README, skill menu, catalog, registry and runtime projections | Add Review discovery and regenerate from canonical metadata with a declared Codex parity twin. Existing explicit specialist routes remain available. |

RPI's hard dependencies remain Plan, Implement and Validate. Review has no hard
dependency; native clear work still needs zero mandatory skills. No old root is
deleted in this batch. For source links, `ao skills link --skill review` uses
the existing conflict-preserving installer: a real `review` directory or unowned
link stays untouched. Resolve a reported conflict through its owner before
linking; do not remove foreign content to match the inventory. Remaining library
dispositions and final host upgrade/recovery qualification are separate work.

For source-linked installations, inspect old links before removing them: a
retired name may still be visible as a dangling link after updating the checkout.
Use the install's owned unlink path and relink the selected surviving names;
never remove a real directory or another tool's link just to match the count.
Managed plugin upgrades should use the runtime's update mechanism. Avoid loading
both a plugin copy and source links for the same skill.

The new context-budget roles are optional. Claude's plugin includes the
`agentops:bulk-reader` and `agentops:code-writer` subagent definitions. Codex's
plugin includes their generated TOML resources, but role activation is separate:
from a source checkout run `bash scripts/install-codex-context-agents.sh`, or add
`--project` for the current project, then restart Codex. The installer preserves
unrelated configuration and makes backups when replacing owned values.

Read-budget hooks are also separate opt-ins. Use
`scripts/install-read-budget-guard.sh` for Claude or
`scripts/install-codex-read-budget-guard.sh` for Codex. Codex requires native
hook review/trust. Do not infer that installing a skill or role activates a hook;
see [the Codex runtime contract](design/codex-context-budget.md) for discovery,
linked-worktree restrictions and the sandbox limitation.

### Interview and Navigate batch

This batch adds two optional goal entrypoints, bringing the source inventory to 38.

| Existing request or consumer | Owner after this batch |
|---|---|
| Settle a big outcome with the caller before autonomous work | New `interview`: one question per turn with a labeled recommendation; acceptance as Given/When/Then, one domain term per concept. Human-invoked only; creates no goal or bead. |
| Pick a goal's next wave and keep its bead graph honest | New `navigate`: acceptance matrix, ready frontier, wave choice, verdicts and discoveries recorded on beads. It never dispatches, judges, closes or edits acceptance. |
| Bead graph contract, ratchet definition and wave loop | Moved from `craft-goal` to `navigate`; Craft Goal keeps admission, envelopes, HOLD, the frozen prompt and its lint rubric. |

RPI's hard dependencies remain Plan, Implement and Validate.

## Verdicts and identity

When persistence is requested, `verdict.v2` binds acceptance and a deterministic
`subject-manifest.v1` to distinct declared author and validator context
identities. Freshness is an attested trust fact, not cryptographic proof of
process isolation. New CDLC proof uses caller-selected protected external
non-Git storage, with atomic content-addressed verdict writes when requested.
Preserve existing `.agents/` evidence under owner policy. Missing destination
routing is not permission to fall back to repository storage (ADR-0016).

Historical Pawl, queue, claim, landing, and lifecycle artifacts remain inert
evidence. They no longer influence phase sequencing, verdict validity, or CLI
outcomes.

## Registered CLI consumer dispositions

Every currently registered public command is **kept**, with unchanged names,
flags and authority. The rows below cover each public top-level command and
**every descendant** under it. The generated [command reference](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md)
enumerates those descendants and flags; the executable registry in
[`cli/cmd/ao`](../cli/cmd/ao/) remains the source, checked by
[surface parity](https://github.com/boshu2/agentops/blob/main/scripts/check-cmdao-surface-parity.sh) against
[the complete leaf inventory](cli-surface.json). `ao help` is Cobra's retained
framework helper, categorized `internal-hidden` by that inventory. Removed
commands remain covered by the breaking-boundary table above, not aliases.

| Registered consumer and all descendants | Disposition and source owner | Retained outcome / compatibility |
|---|---|---|
| [`ao capabilities`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-capabilities) | keep; [capabilities.go](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/capabilities.go) | Machine-readable registered CLI contract. |
| [`ao completion`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-completion) | keep; [completion.go](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/completion.go) | Native shell completion generation. |
| [`ao config`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-config) | keep; [config module](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/config_module.go) | Explicit context-route bindings; no automatic context admission. |
| [`ao demo`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-demo) | keep; [demo composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/demo_composition.go) | Read-only native example and explicitly selected RPI example. |
| [`ao doctor`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-doctor) | keep; [doctor module](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/doctor_module.go) | Installation diagnosis, snapshots and explicitly authorized repair/undo. |
| [`ao gate`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-gate) | keep; [gate composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/gate_composition.go) | Deterministic repository checks; not semantic acceptance. |
| [`ao gc`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-gc) | keep; [GC composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/gc_composition.go) | Prepare/check selected upstream integration and authorized affinity recovery. |
| [`ao goals`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-goals) | keep; [goals composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/goals_composition.go) | Existing goal/scenario measurement and inspection; no work ownership or scheduler. |
| [`ao init`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-init) | keep; [init composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/init_composition.go) | Optional local evidence setup; native work needs no initialization. |
| [`ao provenance`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-provenance) | keep; [provenance composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/provenance_composition.go) | Exact identities, generic evidence and requested judgment persistence/verification. |
| [`ao quick-start`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-quick-start) | keep; [quick-start composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/quickstart_composition.go) | Read-only native execution brief. |
| [`ao robot-docs`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-robot-docs) | keep; [robot-docs composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/robotdocs_composition.go) | Agent-facing CLI documentation. |
| [`ao session`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-session) | keep; [session composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/session_composition.go) | Requested continuity and authorized source reads; no mandatory bootstrap. |
| [`ao skills`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-skills) | keep; [skills composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/skills_composition.go) | Catalog discovery/checks and selected or full owned source links. Orchestrate is another catalog entry, not a command. |
| [`ao status`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-status) | keep; [status composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/status_composition.go) | Evidence-store facts; tracker/runtime retain live work state. |
| [`ao version`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-version) | keep; [version composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/version_composition.go) | Installed CLI version identity. |
| [`ao workflows`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-workflows) | keep; [workflows composition](https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/workflows_composition.go) | Owned project-local Claude workflow links/unlinks. |

## Install migration

AgentOps supports three skill install paths: the Claude Code plugin, the Codex
plugin, and `npx skills@latest add boshu2/agentops` for every other agent
(Cursor, OpenCode, Gemini CLI/Antigravity, Pi, Grok Build, OpenClaw and others
the external Skills installer lists). Contributors who edit skills link one
canonical checkout with `ao skills link` instead. The [host/install mapping](contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping)
accounts for the retained runtime, export and package consumers; each host's
live qualification remains separate from structural checks.

### Retained package and installation consumers

The [host/install contract](contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping)
owns host scope and qualification. This migration table records the disposition
of its consumers plus the repository's standalone installers and optional
roles/hooks/workflows. **Keep** preserves the declared behavior and all upgrade,
cold-resume, collision and recovery obligations; it does not silently retire an
untested promise. Every live claim still needs evidence on the final installation.

| Consumer | Disposition and owner | Compatibility treatment |
|---|---|---|
| Claude Code plugin and source links | keep; [.claude-plugin](https://github.com/boshu2/agentops/blob/main/.claude-plugin/plugin.json), [marketplace](https://github.com/boshu2/agentops/blob/main/.claude-plugin/marketplace.json), [Claude image](https://github.com/boshu2/agentops/blob/main/images/claude/README.md) and canonical `skills/` | First-class host. Retain qualified plugin names, full bundle, agents and policy dispatcher; source linking keeps selected names. |
| Codex plugin and source links | keep; [.codex-plugin](https://github.com/boshu2/agentops/blob/main/.codex-plugin/plugin.json), [marketplace](https://github.com/boshu2/agentops/blob/main/plugins/marketplace.json), [Codex image](https://github.com/boshu2/agentops/blob/main/images/codex/README.md) and generated `skills-codex/` | First-class host. Preserve canonical/projection parity and qualified plugin names; source links retain catalog names. |
| Cursor rules and source links | keep; npx `-a cursor`, [converter](https://github.com/boshu2/agentops/blob/main/skills/skill-builder/scripts/converter/convert.sh) and [destination resolver](https://github.com/boshu2/agentops/blob/main/cli/internal/skillsapp/roots.go) | Install through npx. Retain `.mdc` export and the contributor-detected Cursor skills root; structural coverage remains distinct from live discovery/execution. |
| OpenCode portable and explicit source roots | keep; npx `-a opencode`, [OpenCode guide](https://github.com/boshu2/agentops/blob/main/.opencode/INSTALL.md) and [destination resolver](https://github.com/boshu2/agentops/blob/main/cli/internal/skillsapp/roots.go) | Install through npx into the portable root. Contributors retain explicit `--dest` config-root linking; optional hooks stay selectable. |
| Gemini / Antigravity package and export | retire; the `images/gemini` package and its [bundle generator](https://github.com/boshu2/agentops/blob/main/scripts/generate-skill-mesh.py) branch were deleted | Gemini CLI and Antigravity install through npx (`-a gemini-cli`, `-a antigravity`). Remove an installed package with `agy plugin disable agentops-core-gemini`, then `agy plugin uninstall agentops-core-gemini`. |
| Skill Builder exports (`codex`, `cursor`, `test`) | keep; [converter](https://github.com/boshu2/agentops/blob/main/skills/skill-builder/scripts/converter/convert.sh) | Preserve supported target selection and Codex modular/inline layouts. Exported files require separate host-load qualification. |
| Pi, portable and explicit-destination source consumers | keep; npx `-a pi` and [destination resolver](https://github.com/boshu2/agentops/blob/main/cli/internal/skillsapp/roots.go) | Install through npx. For contributors, preserve detected Pi root, always-included portable root and explicit destination semantics; no new live-host claim. |
| External `npx skills` installer | keep; canonical `skills/` and [install guide](install-day2-ops.md) | The install path for every agent except the two plugin hosts. Name agents with `-a`; never `--all`. It does not install AO, runtime plugins, roles or hooks. |
| AO source, Go, Homebrew and release-binary installs | keep; [CLI installation](install-day2-ops.md#maintainer-contributor-the-ao-binary), [release build](https://github.com/boshu2/agentops/blob/main/.goreleaser.yml), [Windows AO installer](https://github.com/boshu2/agentops/blob/main/scripts/install-ao.ps1) | Retain source builds and published-binary consumers, including the Windows CLI installer. Skill loading is separate from binary availability. |
| Claude agent roles and optional Codex context roles | keep; [Claude agents](../agents/), [Codex role sources](../skills/agent-native/agents/) and [role installer](https://github.com/boshu2/agentops/blob/main/scripts/install-codex-context-agents.sh) | Preserve role identity and explicit Codex activation, backups and unrelated configuration. Installing skills does not activate Codex roles. |
| Claude policy dispatcher | keep; [plugin hooks](https://github.com/boshu2/agentops/blob/main/hooks/hooks.json), [source wrapper](https://github.com/boshu2/agentops/blob/main/scripts/install-policy-dispatch.sh) and [packaged owner](https://github.com/boshu2/agentops/blob/main/hooks/guards/scripts/install-hooks.sh) | The dispatcher is automatically active when installed through the Claude plugin; source/copy installation uses the existing owner-selected installer. Native zero-skill work remains hookless. |
| Read-budget and installed-skill edit guards | keep; [Claude read guard](https://github.com/boshu2/agentops/blob/main/scripts/install-read-budget-guard.sh), [Codex read guard](https://github.com/boshu2/agentops/blob/main/scripts/install-codex-read-budget-guard.sh), [edit guard](https://github.com/boshu2/agentops/blob/main/scripts/install-installed-skill-edit-guard.sh) | Preserve separate opt-in installation, Codex trust review, backups and documented enforcement limits. |
| Claude named workflows | keep; [canonical workflows](../workflows/), [`ao workflows`](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md#ao-workflows), [user-level installer](https://github.com/boshu2/agentops/blob/main/scripts/install-workflows.sh) | Keep project-local owned-link refusal semantics. The user-level installer retains its distinct backed-up replacement semantics; do not assume the two installers are interchangeable. |
| Optional BD binary installer | keep; [install-bd.sh](https://github.com/boshu2/agentops/blob/main/scripts/install-bd.sh) | Installs selected native BD; does not create or replace the repository's work store. |
| Optional MS post-merge index hook | retired | Obtain MS maintenance guidance from [upstream](https://github.com/Dicklesworthstone/meta_skill). AgentOps no longer ships an MS index hook installer. |
| Legacy 3.x skill curl/PowerShell installers | retire; deleted: `install.sh`, `install-claude.sh`, `install-codex.sh`, `install-agy.sh`, `install-opencode.sh`, `install-codex.ps1` | Their raw URLs now 404; see the breaking boundary below. `install-ao.ps1` is a retained CLI installer, not this retired skill installer. |

For a contributor source installation, preserve the same selectors on upgrade:

```bash
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link --skill test --skill refactor --dry-run
ao skills link --skill test --skill refactor
```

Omit selectors for an explicit full installation. Keep the original `--dest`
for an explicit destination. OpenCode's dedicated config root needs `--dest`;
its portable `~/.agents/skills` discovery root is already included in fan-out.
Selection does not remove previously installed
skills, stale copied names or user-owned collisions.

The 3.x curl and PowerShell installers (`scripts/install.sh`,
`install-claude.sh`, `install-codex.sh`, `install-agy.sh`, `install-opencode.sh`
and `install-codex.ps1`) were deleted, and their raw URLs now 404. A
`curl -fsSL <url> | bash` without `pipefail` then does nothing and exits 0, so
switch old scripts to a plugin or npx. Internal helpers
(`install-codex-plugin.sh`, `install-codex-native-skills.sh`) were deleted
earlier.

If you switch from a plugin to source links, remove the runtime plugin through
that runtime before linking the checkout so only one corpus is visible:

- Claude Code: `claude plugin uninstall agentops@agentops-marketplace`, then
  `claude plugin marketplace remove agentops-marketplace`.
- Codex: `codex plugin remove agentops@agentops-marketplace`, then
  `codex plugin marketplace remove agentops-marketplace`. For older/manual
  installations, preserve the relevant configuration and cache first and
  identify exact owned entries using the [day-2 guide](install-day2-ops.md#switch-from-plugins-to-source-links).
- Retired Gemini/Antigravity package: `agy plugin disable agentops-core-gemini`,
  then `agy plugin uninstall agentops-core-gemini`.

`ao skills link` refuses to replace real directories and foreign links. Resolve
each reported conflict deliberately; never delete a user-owned skill merely to
make the counts match. Use `ao skills unlink` to remove only links that point
into the current checkout.
That unlink operation removes all owned links at the selected destinations;
preview its removal set before using it. Follow [cold resume](install-day2-ops.md#cold-resume)
and [failed/partial-upgrade recovery](install-day2-ops.md#recover), retaining
the original selection, prior revision and user-owned entries. Upgrade and
recovery are required host journeys, not established by a clean package check.

## Optional runtimes

NTM, Agent Mail, Gas City, councils, and model-specific executors remain
caller-selected adapters or strategies. None is a hard dependency of RPI,
Plan, Implement, or Validate, and none may translate its own attempts, leases,
queues, or delivery state into AgentOps correctness state.
