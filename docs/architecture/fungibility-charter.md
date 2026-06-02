# AgentOps 3.0 Fungibility Charter

> Doctrinal commitment. The six promises AgentOps 3.0 makes about how agents relate to work, to each other, and to model choice. Companion to the [Operating Loop](operating-loop.md) (the discipline every agent runs) and [Ports and Adapters](ports-and-adapters.md) (the seams that make any runtime swappable).

AgentOps' execution primitives — the `.agents/` corpus, beads, worktrees-per-bead, decay-ranked retrieval — are *fungibility-shaped*: any agent can read shared state, claim any unit of work, and contribute back. But for most of 2.x the operational rhetoric ("Claude discovers, Codex implements"; `/council --mixed`; the Codex-parity machinery) sold model **specialization** as the value. The primitives said one thing; the surfaces said another.

The 3.0 charter resolves that tension in favor of the primitives:

**Fungible by default, specialized when you opt in.**

The primary pitch is "spawn N identical agents, watch them work." Model diversity is a real feature for the users who want it — but it is a deliberate opt-in, never the assumed shape of a swarm.

## Why a charter

Without an explicit charter, the fungibility philosophy stays implicit in the primitives and is contradicted by the surfaces. A new operator reading the README would conclude that AgentOps *requires* a model mix to function, when in fact a single-model swarm is the supported default. A charter makes the doctrinal choice legible, gives the CI gates something concrete to enforce, and gives the 3.0 product docs a frame to write toward.

This is a doctrine document, not a primitive. Each commitment below names the primitive that *implements* it — the charter is honest only because those primitives already ship. Doctrine that promises behavior the code cannot deliver is the failure mode this document exists to avoid.

## The six commitments

### 1. Default RPI mode is single-model

Whatever model the current session runs is the model that runs every RPI phase — Research, Plan, Implement, Validate. There is no built-in handoff to a different model between phases.

A phase boundary is a *context* boundary (bounded packets and summaries cross it, per the Operating Loop's "context crosses boundaries as artifacts" principle), not a *model* boundary. Crossing models mid-loop is a thing you can choose to do, not a thing the loop does on your behalf.

*Implemented by `/rpi` runtime detection: the active session's model is detected and reused across phases; the prior multi-model default is demoted to an opt-in flag.*

### 2. Any agent can claim any bead — no role gating

There are no "frontend agents," "testing agents," or "review agents." There is one pool of interchangeable generalists, and any agent can claim any ready bead from it.

The bead claim model already enforces this mechanically: `bd ready` surfaces unblocked work to every agent equally, and `bd update <id> --claim` is an atomic first-come claim with no role predicate. The charter makes explicit what the tracker already does — claiming is by availability, never by assigned specialty.

*Implemented by `bd`'s role-free claim model (`bd ready` / `bd update --claim`), which the [Operating Loop](operating-loop.md) move 2 ("track as a bead when it leaves the head") already assumes.*

### 3. Stateless agent assumption

Agents are interchangeable consumers of the `.agents/` corpus. **No agent owns a domain. No agent carries irreplaceable session state.**

Everything an agent needs to do the next unit of work is reconstructable from shared, durable surfaces: the bead (linked intent, acceptance examples, accumulating evidence), the corpus (decay-ranked prior context, retrieved on demand), and the worktree (the change in flight). An agent's in-context memory is a cache, not a system of record. When the cache is lost, another agent rebuilds it from the same surfaces and continues.

This is what makes commitments 1, 2, 5, and 6 coherent: fungibility is only real if losing any single agent loses no irreplaceable state.

*Implemented by the corpus-as-source-of-truth contract: `ao inject` / `ao corpus inject --query` reconstruct orientation from durable state; worktree-per-bead isolates in-flight change to a recoverable surface (the multi-agent discipline codified in `AGENTS.md`).*

### 4. Universal init prompt is standard

Every spawned agent runs the same orientation step — `ao session bootstrap` — regardless of which model it is. Same starting frame, same standard orientation report, same on-ramp into the corpus.

Identical orientation is a precondition for interchangeability: two agents given the same bead and the same starting frame should behave equivalently against the acceptance examples, whatever model backs them. A bespoke per-model or per-role init prompt would reintroduce specialization through the back door.

*Implemented by `ao session bootstrap` (the universal init prompt) and the `session-bootstrap` skill, which AgentOps 3.0's hookless startup makes the explicit replacement for the old SessionStart context injection.*

### 5. Death is normal; recovery is automatic

An agent dying mid-work — context compaction, crash, rate limit, a closed tab — is an expected event, not an incident. Recovery is an operational primitive, not a heroic human intervention.

When an agent dies, its bead stays `in_progress` under a dead claim. In a 30-agent overnight swarm this happens constantly; without recovery the swarm degrades over hours as live agents skip "taken" work that no one is doing. The recovery primitive surfaces these stale claims with evidence (last touch, claim age, last evidence event) and atomically transfers them to a live agent. **Dead agent? Start another. No role replacement needed** — because there were no roles to replace, and the corpus + bead carry the state forward.

*Implemented by `ao beads stale-claims` (detect stale `in_progress` claims with staleness evidence) and `ao beads resume <id>` (atomic claim transfer crediting both agents), composing with the session-bootstrap heartbeat from commitment 4.*

### 6. Specialization is opt-in

Model diversity is a feature for the users who want it. Flags such as `--mixed`, `--pool=N`, `--diverse`, and `--codex` exist precisely so that a team can choose model diversity *as a deliberate strategy* — cross-validation, harness-specific strengths, redundancy.

They are never the default. Reaching for one is an explicit decision the operator makes and owns, surfaced as an advanced option, not the assumed shape of a swarm. The 2.x framings that sold the mix as the primary pitch move to advanced sections; the front door is single-model fungibility.

*Implemented by the opt-in diversity flags on `/rpi` and `/council` (`--mixed`, `--pool=N`, `--diverse`, `--codex`), each off by default.*

## How the commitments compose

The six are not independent — they form one closed loop that lets a swarm run unattended:

```text
universal init (4)  →  any agent claims any bead (2)  →  single-model RPI (1)
       ↑                                                          │
       │                                                          ▼
specialization stays opt-in (6)  ←  stateless agents (3)  ←  death → auto-recovery (5)
```

Stateless agents (3) make death survivable; automatic recovery (5) makes death routine; a universal init (4) makes any replacement equivalent; role-free claiming (2) lets the replacement pick up any work; single-model default (1) keeps the swarm homogeneous unless the operator opts into diversity (6). Remove any one and the unattended-swarm property breaks: specialized agents (¬2) create bottlenecks; stateful agents (¬3) make death lossy; bespoke init (¬4) makes replacements non-equivalent.

## Boundaries

Fungibility is the right default for **software development**, where output matters more than discourse and the bottleneck is throughput against a large pool of independent work. It is the *wrong* default where role separation is the mechanism rather than overhead — adversarial debate where distinct roles produce the value, or strict compliance review where separation of duties is a hard requirement. Those are deliberate specialization choices under commitment 6, not violations of the charter.

The charter governs *agent fungibility*, not *runtime fungibility* — the latter (any harness behind the same ports) is the [Ports and Adapters](ports-and-adapters.md) concern. The two reinforce each other: swappable runtimes make single-model defaults cheap to honor, because the runtime is not load-bearing for correctness.

## Related

- [Operating Loop](operating-loop.md) — the discipline every fungible agent executes; commitments 1–3 are doctrinal reads of its moves.
- [Ports and Adapters](ports-and-adapters.md) — runtime swappability, the structural complement to agent fungibility.
- An NTM tmux swarm coordinated through mcp-agent-mail and equipped with `ao mcp serve` — the out-of-session industrial version of ready Claude/Codex skill sessions.
- Skill: `agent-fungibility-philosophy` — the operating playbook (spawn, init, recover) the charter codifies into doctrine.
- Skill: `session-bootstrap` — the universal init prompt named in commitment 4.
