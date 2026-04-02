# Knowledge Activation Productization Plan

Date: 2026-04-01
Status: proposed implementation plan
Source: local prototype validated in `/Users/fullerbt/.agents` and `/Users/fullerbt/.codex/plugins/cache/agentops-marketplace/agentops/local/skills-codex/knowledge-activation/`

## Problem

AgentOps already has strong flywheel primitives, but it is still better at capturing and maintaining knowledge than operationalizing a mature `.agents` corpus into task-time advantage.

The local prototype exposed the missing product gap:

1. packetization and mining are not enough
2. users need promoted operator surfaces, not just retrieval substrate
3. `athena` is a maintenance skill, not a corpus activation skill

The result is a common failure mode:

- knowledge exists
- knowledge is searchable
- knowledge still does not materially change how future agent sessions begin and execute

## Product Decision

Introduce a new outer-loop capability in AgentOps.

Working name:

- `knowledge-activation`

The name can change later, but the contract should not.

`athena` remains the hygiene skill for mine, validate, and defrag.

The new capability owns:

- corpus consolidation
- belief promotion
- playbook generation
- briefing compilation
- gap and thin-topic feedback into the flywheel

## What The Prototype Already Proved

The local prototype validated this end-to-end shape:

- evidence packet families
- belief book generation
- playbook candidate generation
- per-goal briefing generation
- thin-topic caution handling

Validation already passed locally:

- Python compilation
- unit tests
- skill bundle validation
- full wrapper execution

Local outputs worth treating as seed material:

- `/Users/fullerbt/.agents/knowledge/book-of-beliefs.md`
- `/Users/fullerbt/.agents/playbooks/index.md`
- `/Users/fullerbt/.agents/briefings/2026-04-01-turn-agents-into-usable-information-and-spin-the-knowledge-f.md`
- `/Users/fullerbt/.agents/knowledge/agent-interaction-operationalization.md`
- `/Users/fullerbt/.agents/retros/2026-04-01-knowledge-activation-consolidated-postmortem.md`

## What AgentOps Should Do Better

### 1. Prefer briefings over giant startup dumps

AgentOps currently has strong lifecycle and injection surfaces, but it should more explicitly support:

- a small goal-time briefing
- selected principles
- selected playbooks
- warnings and trust boundaries
- evidence links behind the scenes

This is a better startup surface than broad generic startup context.

### 2. Separate storage from operator surfaces

The product should distinguish:

- packet/chunk/evidence substrate
- belief and playbook promotion layer
- task-time briefing layer

Without this separation, users get organization without activation.

### 3. Encode trust boundaries

Thin topics must be first-class product state, not an implicit caveat buried in docs.

The system should know:

- what is healthy enough to promote
- what stays discovery-only
- what should trigger another mining or review pass

### 4. Improve prompt-surface guidance

AgentOps should give a clear rule:

- stable beliefs belong in `AGENTS.md`, `CLAUDE.md`, and memory
- dynamic guidance belongs in a briefing
- repeated failures belong in planning rules, checks, or skill logic

## Proposed CLI Surface

Add an `ao knowledge` family:

### `ao knowledge activate`

Runs the full outer loop:

1. preflight
2. evidence consolidation
3. belief and playbook promotion
4. briefing refresh
5. gap reporting

### `ao knowledge beliefs`

Builds or refreshes the belief book from promoted evidence.

### `ao knowledge playbooks`

Builds or refreshes playbook candidates from healthy topics.

### `ao knowledge brief --goal "<goal>"`

Compiles a runtime briefing using:

- relevant beliefs
- relevant playbooks
- warnings
- evidence links

### `ao knowledge gaps`

Reports:

- thin topics
- promotion gaps
- weak claims needing review
- next recommended mining work

## Proposed Skill Surface

Add a new user-invocable skill in the AgentOps repo:

- `knowledge-activation`

The skill should orchestrate the `ao knowledge` commands and explain the contract.

It should not own the heavy lifting forever. The builders belong in stable product or CLI surfaces, while the skill owns:

- user intent routing
- execution order
- output interpretation
- next-step recommendations

## Proposed Output Surfaces

The product should standardize these outputs:

- `.agents/packets/`
- `.agents/knowledge/book-of-beliefs.md`
- `.agents/playbooks/`
- `.agents/briefings/`
- `.agents/retros/`

Required trust rules:

- packet and chunk layers are substrate
- beliefs, playbooks, and briefings are consumer surfaces
- thin topics remain discovery-only until promoted health improves

## Runtime Integration

The lifecycle should treat briefings as a first-class startup aid.

Recommended behavior:

1. `ao codex start` checks for an active task or handoff goal
2. if a recent matching briefing exists, surface it
3. if no briefing exists, suggest or build one
4. keep the startup surface small and citation-backed

This keeps startup context bounded and aligned with the belief system.

## Implementation Waves

### Wave 1: Upstream the skill contract

1. Add `crew/nami/skills/knowledge-activation/`
2. Port the validated local `SKILL.md`
3. Add DAG and output-surface references
4. Add a bundle validator

### Wave 2: Productize builders

1. Decide whether to port the current Python builders into `ao` directly or wrap them first
2. Expose the command family:
   - `ao knowledge activate`
   - `ao knowledge beliefs`
   - `ao knowledge playbooks`
   - `ao knowledge brief`
   - `ao knowledge gaps`
3. Preserve deterministic behavior on unchanged inputs

### Wave 3: Runtime integration

1. Teach the Codex lifecycle to prefer briefings for startup
2. Keep prompt surfaces small
3. Record trust warnings for thin topics and weak claims

### Wave 4: Review and hardening

1. Add tests for deterministic output
2. Add tests for thin-topic handling
3. Add docs describing the operator layers
4. Run a manual review pass on belief and playbook sharpness

## Acceptance Criteria

1. A user can point AgentOps at a mature `.agents` corpus and get operational outputs, not just mined evidence.
2. AgentOps produces a belief book, playbook candidates, and a goal-time briefing.
3. Thin topics are surfaced explicitly and never silently promoted as canonical truth.
4. The new capability complements `athena` instead of bloating it.
5. Startup interactions improve because the system prefers briefings over broad context dumps.

## Risks

### Retrieval substrate instability

If `cass index` remains unreliable, refresh confidence stays weaker than it should be.

### Weak claim promotion

Artifact-title leakage into beliefs or playbooks will undermine user trust if not reviewed.

### Doc graveyard risk

If outputs are generated without clear consumers, the system will create another organized archive instead of a working outer loop.

## Recommendation

Implement this as a new AgentOps outer-loop capability now.

Do not try to stretch `athena` until it absorbs this contract. That would blur maintenance and activation and make both harder to reason about.

The winning model is:

- `athena` for hygiene
- `knowledge-activation` for corpus operationalization
- `ao knowledge brief` as the task-time bridge into actual agent behavior
