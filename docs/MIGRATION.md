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

The 3.7 migration baseline has 34 skills, compared with 52 in 3.6.0. Twenty former roots
were retired; `memory` and `skill-eval` are new relative to that release. The
current menu additionally exposes Review and Orchestrate; no baseline skill is retired or
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
The [generated catalog](../skills/catalog.json) and [router](SKILL-ROUTER.md)
describe the current inventory; fewer roots or improved discovery is not claimed.
For every **keep** row, its slug and explicit invocation remain compatible:
`/agentops:<name>` in Claude's plugin, `$agentops:<name>` in Codex's plugin,
and the unqualified catalog name for source-linked installs. Host loading is
qualified separately from package presence.

| Baseline entrypoint | Disposition | Maintained owner and outcome |
|---|---|---|
| `account-rotation` | keep | [Account Rotation](../skills/account-rotation/SKILL.md): caller-selected account changes and identity checks. |
| `agent-mail` | keep | [Agent Mail](../skills/agent-mail/SKILL.md): selected messaging and advisory file reservations; native tracker retains status. |
| `agent-native` | keep | [Agent Native](../skills/agent-native/SKILL.md): runtime dispatch, observation, context identity, isolation and native follow-up. Orchestrate links these mechanics. |
| `agy-native` | keep | [AGY Native](../skills/agy-native/SKILL.md): explicitly selected Antigravity execution. |
| `cass` | keep | [CASS](../skills/cass/SKILL.md): cited session retrieval; Memory owns admission of reusable claims. |
| `cc-hooks` | keep | [CC Hooks](../skills/cc-hooks/SKILL.md): authorized Claude hook and guard configuration. |
| `codex-exec` | keep | [Codex Exec](../skills/codex-exec/SKILL.md): one selected headless Codex process. |
| `council` | keep | [Council](../skills/council/SKILL.md): selected independent perspectives; advice does not supply acceptance. |
| `craft-goal` | keep | [Craft Goal](../skills/craft-goal/SKILL.md): explicitly selected persistent-goal guidance; native goals retain continuity. |
| `dcg` | keep | [DCG](../skills/dcg/SKILL.md): diagnose guard refusal and authorized policy changes. |
| `doc` | keep | [Doc](../skills/doc/SKILL.md): requested source-grounded documents and continuity handoffs. |
| `domain` | keep | [Domain](../skills/domain/SKILL.md): domain vocabulary, rule boundaries and repository conventions. |
| `idea-genie` | keep | [Idea Genie](../skills/idea-genie/SKILL.md): evidence-backed options and idea challenge. |
| `implement` | keep | [Implement](../skills/implement/SKILL.md): complete accepted changes, direct repair and authorized operations. |
| `memory` | keep | [Memory](../skills/memory/SKILL.md): selective find/recall, capture/mine and curate/qualify/retire with support and disclosure review. |
| `ms` | keep | [MS](../skills/ms/SKILL.md): selected meta_skill search and loading. |
| `ntm` | keep | [NTM](../skills/ntm/SKILL.md): selected persistent panes and native runtime facts. |
| `plan` | keep | [Plan](../skills/plan/SKILL.md): resumable discovery, uncertainty routing, optional challenge and one ready complete slice. |
| `postmortem` | keep | [Postmortem](../skills/postmortem/SKILL.md): requested outcome analysis, separate from code acceptance. |
| `premortem` | keep | [Premortem](../skills/premortem/SKILL.md): selected fresh plan challenge; Plan owns the shared challenge exchange. |
| `rch` | keep | [RCH](../skills/rch/SKILL.md): selected remote compilation and diagnostics. |
| `reality-check` | keep | [Reality Check](../skills/reality-check/SKILL.md): compare shipped-feature, repository or goal claims with evidence. |
| `refactor` | keep | [Refactor](../skills/refactor/SKILL.md): structural simplification while preserving behavior. |
| `research` | keep | [Research](../skills/research/SKILL.md): cited source investigation and pattern evidence. |
| `reverse-engineer` | keep | [Reverse Engineer](../skills/reverse-engineer/SKILL.md): authorized external-system teardown and adoption choices. |
| `rpi` | keep | [RPI](../skills/rpi/SKILL.md): explicitly selected outcome-to-judgment charter; unchanged hard dependency graph. |
| `sbh` | keep | [SBH](../skills/sbh/SKILL.md): disk-pressure diagnosis and authorized recovery. |
| `security` | keep | [Security](../skills/security/SKILL.md): concrete exposure review and selected scans. |
| `skill-builder` | keep | [Skill Builder](../skills/skill-builder/SKILL.md): skill authoring, repair, projections and exports. |
| `skill-eval` | keep | [Skill Eval](../skills/skill-eval/SKILL.md): bounded behavioral evaluation; structural conformance is not efficacy. |
| `test` | keep | [Test](../skills/test/SKILL.md): behavioral test design and consequential coverage gaps. |
| `using-flywheel` | keep | [Using Flywheel](../skills/using-flywheel/SKILL.md): selected factory through its native workflow. |
| `using-gc` | keep | [Using GC](../skills/using-gc/SKILL.md): selected Gas City through its Mayor and supported native doors. |
| `validate` | keep | [Validate](../skills/validate/SKILL.md): sole skill owner of fresh exact-content acceptance judgment and explicit missing proof. |

[Orchestrate](../skills/orchestrate/SKILL.md) is an additive coordination entrypoint
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
**every descendant** under it. The generated [command reference](../cli/docs/COMMANDS.md)
enumerates those descendants and flags; the executable registry in
[`cli/cmd/ao`](../cli/cmd/ao/) remains the source, checked by
[surface parity](../scripts/check-cmdao-surface-parity.sh) against
[the complete leaf inventory](cli-surface.json). `ao help` is Cobra's retained
framework helper, categorized `internal-hidden` by that inventory. Removed
commands remain covered by the breaking-boundary table above, not aliases.

| Registered consumer and all descendants | Disposition and source owner | Retained outcome / compatibility |
|---|---|---|
| [`ao capabilities`](../cli/docs/COMMANDS.md#ao-capabilities) | keep; [capabilities.go](../cli/cmd/ao/capabilities.go) | Machine-readable registered CLI contract. |
| [`ao completion`](../cli/docs/COMMANDS.md#ao-completion) | keep; [completion.go](../cli/cmd/ao/completion.go) | Native shell completion generation. |
| [`ao config`](../cli/docs/COMMANDS.md#ao-config) | keep; [config module](../cli/cmd/ao/config_module.go) | Explicit context-route bindings; no automatic context admission. |
| [`ao demo`](../cli/docs/COMMANDS.md#ao-demo) | keep; [demo composition](../cli/cmd/ao/demo_composition.go) | Read-only native example and explicitly selected RPI example. |
| [`ao doctor`](../cli/docs/COMMANDS.md#ao-doctor) | keep; [doctor module](../cli/cmd/ao/doctor_module.go) | Installation diagnosis, snapshots and explicitly authorized repair/undo. |
| [`ao gate`](../cli/docs/COMMANDS.md#ao-gate) | keep; [gate composition](../cli/cmd/ao/gate_composition.go) | Deterministic repository checks; not semantic acceptance. |
| [`ao gc`](../cli/docs/COMMANDS.md#ao-gc) | keep; [GC composition](../cli/cmd/ao/gc_composition.go) | Prepare/check selected upstream integration and authorized affinity recovery. |
| [`ao goals`](../cli/docs/COMMANDS.md#ao-goals) | keep; [goals composition](../cli/cmd/ao/goals_composition.go) | Existing goal/scenario measurement and inspection; no work ownership or scheduler. |
| [`ao init`](../cli/docs/COMMANDS.md#ao-init) | keep; [init composition](../cli/cmd/ao/init_composition.go) | Optional local evidence setup; native work needs no initialization. |
| [`ao provenance`](../cli/docs/COMMANDS.md#ao-provenance) | keep; [provenance composition](../cli/cmd/ao/provenance_composition.go) | Exact identities, generic evidence and requested judgment persistence/verification. |
| [`ao quick-start`](../cli/docs/COMMANDS.md#ao-quick-start) | keep; [quick-start composition](../cli/cmd/ao/quickstart_composition.go) | Read-only native execution brief. |
| [`ao robot-docs`](../cli/docs/COMMANDS.md#ao-robot-docs) | keep; [robot-docs composition](../cli/cmd/ao/robotdocs_composition.go) | Agent-facing CLI documentation. |
| [`ao session`](../cli/docs/COMMANDS.md#ao-session) | keep; [session composition](../cli/cmd/ao/session_composition.go) | Requested continuity and authorized source reads; no mandatory bootstrap. |
| [`ao skills`](../cli/docs/COMMANDS.md#ao-skills) | keep; [skills composition](../cli/cmd/ao/skills_composition.go) | Catalog discovery/checks and selected or full owned source links. Orchestrate is another catalog entry, not a command. |
| [`ao status`](../cli/docs/COMMANDS.md#ao-status) | keep; [status composition](../cli/cmd/ao/status_composition.go) | Evidence-store facts; tracker/runtime retain live work state. |
| [`ao version`](../cli/docs/COMMANDS.md#ao-version) | keep; [version composition](../cli/cmd/ao/version_composition.go) | Installed CLI version identity. |
| [`ao workflows`](../cli/docs/COMMANDS.md#ao-workflows) | keep; [workflows composition](../cli/cmd/ao/workflows_composition.go) | Owned project-local Claude workflow links/unlinks. |

## Install migration

AgentOps supports three optional skill install paths: `npx skills@latest add
boshu2/agentops --all -g` (all agents supported by the external Skills installer), runtime plugins for
Claude Code and Codex (managed bundles that update with the release), and one
canonical checkout plus source symlinks for users who edit skills or
contribute. The [host/install mapping](contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping)
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
| Claude Code plugin and source links | keep; [.claude-plugin](../.claude-plugin/plugin.json), [marketplace](../.claude-plugin/marketplace.json), [Claude image](../images/claude/README.md) and canonical `skills/` | First-class host. Retain qualified plugin names, full bundle, agents and policy dispatcher; source linking keeps selected names. |
| Codex plugin and source links | keep; [.codex-plugin](../.codex-plugin/plugin.json), [marketplace](../plugins/marketplace.json), [Codex image](../images/codex/README.md) and generated `skills-codex/` | First-class host. Preserve canonical/projection parity and qualified plugin names; source links retain catalog names. |
| Cursor rules and source links | keep; [converter](../skills/skill-builder/scripts/converter/convert.sh) and [destination resolver](../cli/internal/skillsapp/roots.go) | Retain `.mdc` export and detected Cursor skills root; structural coverage remains distinct from live discovery/execution. |
| OpenCode portable and explicit source roots | keep; [OpenCode guide](../.opencode/INSTALL.md) and [destination resolver](../cli/internal/skillsapp/roots.go) | Retain portable root and explicit `--dest` config-root installation, including selected optional hooks. |
| Gemini / Antigravity package and export | keep; [Gemini package](../images/gemini/README.md), generated [manifest](../images/gemini/plugin.json), [bundle generator](../scripts/generate-skill-mesh.py) and detected Gemini root | Retain migration compatibility bundle, agents, rules, hooks and optional Agent Mail configuration. Each claimed host journey remains separately qualified. |
| Skill Builder exports (`codex`, `cursor`, `test`) | keep; [converter](../skills/skill-builder/scripts/converter/convert.sh) | Preserve supported target selection and Codex modular/inline layouts. Exported files require separate host-load qualification. |
| Pi, portable and explicit-destination source consumers | keep; [destination resolver](../cli/internal/skillsapp/roots.go) | Preserve detected Pi root, always-included portable root and explicit destination semantics; no new live-host claim. |
| External `npx skills` installer | keep; canonical `skills/` and [install guide](install-day2-ops.md) | Preserve full and selected installs through the external installer's own contract; it does not install AO, runtime plugins, roles or hooks. |
| AO source, Go, Homebrew and release-binary installs | keep; [CLI installation](install-day2-ops.md#maintainer--contributor-the-ao-binary), [release build](../.goreleaser.yml), [Windows AO installer](../scripts/install-ao.ps1) | Retain source builds and published-binary consumers, including the Windows CLI installer. Skill loading is separate from binary availability. |
| Claude agent roles and optional Codex context roles | keep; [Claude agents](../agents/), [Codex role sources](../skills/agent-native/agents/) and [role installer](../scripts/install-codex-context-agents.sh) | Preserve role identity and explicit Codex activation, backups and unrelated configuration. Installing skills does not activate Codex roles. |
| Claude policy dispatcher | keep; [plugin hooks](../hooks/hooks.json), [source wrapper](../scripts/install-policy-dispatch.sh) and [packaged owner](../skills/cc-hooks/scripts/install-hooks.sh) | The dispatcher is automatically active when installed through the Claude plugin; source/copy installation uses the existing owner-selected installer. Native zero-skill work remains hookless. |
| Read-budget and installed-skill edit guards | keep; [Claude read guard](../scripts/install-read-budget-guard.sh), [Codex read guard](../scripts/install-codex-read-budget-guard.sh), [edit guard](../scripts/install-installed-skill-edit-guard.sh) | Preserve separate opt-in installation, Codex trust review, backups and documented enforcement limits. |
| Claude named workflows | keep; [canonical workflows](../workflows/), [`ao workflows`](../cli/docs/COMMANDS.md#ao-workflows), [user-level installer](../scripts/install-workflows.sh) | Keep project-local owned-link refusal semantics. The user-level installer retains its distinct backed-up replacement semantics; do not assume the two installers are interchangeable. |
| Optional BD binary installer | keep; [install-bd.sh](../scripts/install-bd.sh) | Installs selected native BD; does not create or replace the repository's work store. |
| Optional MS post-merge index hook | keep; [install-ms-reindex-hook.sh](../scripts/install-ms-reindex-hook.sh) | Retain explicit installation and its canonical-checkout/main/changed-skills guards; no mandatory background indexing. |
| Legacy 3.x skill curl/PowerShell installers | retire, retained refusal tombstones; `install.sh`, `install-claude.sh`, `install-codex.sh`, `install-agy.sh`, `install-opencode.sh`, `install-codex.ps1` under [scripts](../scripts/) | Existing breaking boundary below remains unchanged. `install-ao.ps1` is a retained CLI installer, not this retired skill installer. |

For a selected source installation, preserve the same selectors on upgrade:

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

The 3.x curl installers (`scripts/install.sh`, `install-claude.sh`,
`install-codex.sh`, `install-agy.sh`, `install-opencode.sh`, and
`install-codex.ps1`) are tombstones: they refuse to install and print the
supported paths. Internal helpers (`install-codex-plugin.sh`,
`install-codex-native-skills.sh`) were deleted.

If you switch from a plugin to source links, remove the runtime plugin through
that runtime before linking the checkout so only one corpus is visible:

- Claude Code: `claude plugin uninstall agentops@agentops-marketplace`, then
  `claude plugin marketplace remove agentops-marketplace`.
- Codex: `codex plugin remove agentops@agentops-marketplace`, then
  `codex plugin marketplace remove agentops-marketplace`. For older/manual
  installations, preserve the relevant configuration and cache first and
  identify exact owned entries using the [day-2 guide](install-day2-ops.md#switch-from-plugins-to-source-links).
- Gemini/Antigravity: `agy plugin disable agentops-core-gemini`, then
  `agy plugin uninstall agentops-core-gemini`.

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
