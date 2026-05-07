# AgentOps Assurance Profile

AgentOps brings high-assurance operating discipline to AI-agent-paced software work. The lane is not "move slow because the environment is serious." The lane is: keep the rigor, shorten the cycle time, and make every agent run leave evidence a serious operator can review.

This is an engineering posture, not a certification claim. AgentOps does not make a repo accredited, classified-network approved, safety-critical, export-controlled, FedRAMP-authorized, or airworthiness-ready by itself. It gives teams a local-first operating layer for agent work that can fit into those programs when the operator supplies the required controls, approvals, boundaries, and accreditation process.

## Claim

AgentOps is a software factory for coding agents with four compounding layers:

| Layer | Assurance role |
|-------|----------------|
| **Bookkeeping** | Every run leaves file-backed evidence: attempts, decisions, citations, verdicts, handoffs, findings, retros, and post-mortems. |
| **Context Compiler** | Agents receive scoped context for the phase they are in, instead of an unbounded chat history. |
| **Validation Gates** | Plans and code are challenged before promotion by pre-mortems, councils, vibe reviews, tests, hooks, and local quality gates. |
| **Knowledge Flywheel** | Lessons are extracted, scored, promoted, decayed, and reloaded so the system compounds instead of repeating failures. |

The goal is aerospace/IC-style operational discipline at AI-agent pace: evidence before belief, boundaries before autonomy, and promotion only after gates pass.

## What Rigor Means Here

AgentOps uses "rigor" in the operator sense:

- **Traceability.** Work produces inspectable artifacts, not only chat transcripts or final diffs.
- **Separation of duties.** Planning, implementation, and validation can run with different context and different agents.
- **Least-context execution.** Workers get the context needed for their phase, not the entire accumulated conversation.
- **Independent judgment.** Councils and validators review evidence packets instead of inheriting the implementer's mental state.
- **Policy as gates.** Hooks, pre-push checks, goal gates, security scans, and validation skills block promotion rather than merely advising.
- **Desired-state discipline.** `PRODUCT.md` and `GOALS.md` can run ahead of the repo as explicit setpoints; measurements and reconcile loops show what is true now and what must close next.
- **Human authority boundaries.** The operator chooses when the system is interactive, supervised, scheduled, or unattended.
- **Local-first evidence.** AgentOps writes local files that can be inspected, archived, redacted, excluded from source control, or exported into a program's evidence system.

## Boundary Model

AgentOps controls the operating layer around coding agents. It does not control every dependency in the environment.

| Boundary | AgentOps posture | Operator responsibility |
|----------|------------------|-------------------------|
| AgentOps state | Repo-local `.agents/` artifacts, git-ignored by policy at repo root | Decide retention, backup, redaction, export, and whether any artifacts may enter source control |
| Model runtime | Runtime-neutral across Claude Code, Codex CLI, Cursor, and OpenCode | Approve model/provider use, network path, prompt/data handling, and classification boundary |
| Git and CI | Integrates with local git, hooks, tests, and release gates | Control remotes, branch policy, protected environments, and CI secrets |
| Install/update path | Public installers fetch from GitHub unless the operator vendors or mirrors them | Mirror, pin, review, or rebuild artifacts for disconnected or controlled networks |
| Secrets and data | AgentOps can help constrain context, but does not classify data or provide a DLP boundary | Enforce secret handling, data classification, redaction, and egress controls |
| Accreditation | Provides evidence artifacts and operating discipline | Map artifacts to the local control framework and obtain required approvals |

The correct high-assurance reading is: **no AgentOps-managed telemetry or hosted control plane; operator-selected dependencies remain operator-selected dependencies.**

## Operating Profiles

### Profile 0: Exploration

Use AgentOps as a disciplined agent workflow layer. The operator accepts normal model-provider and local-machine risk. Good for personal repos, experiments, and early product discovery.

Expected posture:

- `/quickstart`, `/research`, `/implement`, `/vibe`
- Local `.agents/` state
- Human review before merge

### Profile 1: Team Software Factory

Use AgentOps as the team operating model for coding agents. Work is issue-tracked, validated, and closed with evidence before merge.

Expected posture:

- Beads or equivalent issue tracking
- RPI flow for nontrivial work
- Pre-mortem before implementation
- Vibe/council before promotion
- Pre-push gate before branch publication
- Post-mortem or retro after significant work

### Profile 2: Constrained or High-Rigor Engineering

Use AgentOps where process evidence, local control, and human authority boundaries matter. This is the primary target posture for platform, infrastructure, autonomy, defense-adjacent, regulated, and high-consequence software teams.

Expected posture:

- Approved model runtimes only
- Network path and installer path reviewed by the operator
- `.agents/` retention and redaction policy defined before use
- Humans in the loop for planning, validation, release, and promotion
- Councils use sealed evidence packets where possible
- No unattended source mutation unless bounded by explicit goals, gates, and rollback policy
- Local gates run before any remote push or release artifact

### Profile 3: Accredited or Safety-Critical Program

Use AgentOps only as an input to the program's approved engineering process. AgentOps can generate evidence and enforce local workflow discipline, but the program authority owns control mapping, model approval, data handling, tool qualification, and release authorization.

Expected posture:

- All Profile 2 expectations
- Program-owned control mapping
- Approved artifact retention and export workflow
- Approved model/provider boundary
- Supply-chain review for installers, binaries, dependencies, and generated artifacts
- Explicit human signoff for any artifact that crosses a program boundary

## Evidence Artifacts

AgentOps is valuable in rigorous environments because it changes agent work from "trust the chat" to "inspect the run."

| Evidence | Where it comes from | Why it matters |
|----------|---------------------|----------------|
| Run packets | RPI/discovery/crank/validation flows | Preserve goal, scope, plan, execution, and validation context |
| Council verdicts | `/council`, `/pre-mortem`, `/vibe` | Record independent PASS/WARN/FAIL judgment and rationale |
| Citations | `ao metrics cite`, lookup/search/inject flows | Show which knowledge influenced a run |
| Handoffs | `/handoff`, `/recover`, session closeout | Preserve continuity across agents and sessions |
| Retros and post-mortems | `/retro`, `/post-mortem`, `/forge` | Turn completed work into reusable lessons |
| Ratchet records | `/ratchet`, validation gates | Capture forward-progress checks and failure prevention |
| Goal measurements | `GOALS.md`, `ao goals measure`, `/evolve` | Tie autonomous work to measurable fitness criteria |
| Local gates | pre-push, tests, security scans, docs gates | Make promotion conditional on executable checks |

These artifacts are not a substitute for formal compliance evidence. They are raw material a program can review, retain, redact, or map into its own evidence system.

## Data Handling

AgentOps assumes `.agents/` may contain sensitive session context. Repo-root `.agents/` is local runtime state and should not be tracked by default.

Recommended operator rules:

- Treat `.agents/` as potentially sensitive.
- Decide retention before using scheduled or unattended loops.
- Redact or exclude artifacts that contain secrets, customer data, classified data, export-controlled data, or proprietary context.
- Mirror installers and dependencies before use in disconnected or controlled networks.
- Keep model-provider use inside the organization's approved data boundary.
- Export only reviewed evidence artifacts into long-lived records.

## Autonomy Model

AgentOps does not require maximal autonomy. It is built for variable autonomy:

| Mode | Human role | Suitable use |
|------|------------|--------------|
| **In the loop** | Human approves each meaningful phase | High-risk planning, validation, release, constrained environments |
| **On the loop** | Human supervises scheduled or bounded loops | Dream, compile, forge, feedback-drain, low-risk maintenance |
| **Off the loop** | System runs unattended inside strict bounds | Mature low-risk jobs with explicit gates, rollback, and artifact review |

For high-rigor work, the default should be in the loop for discovery, validation, release, and promotion; on the loop for scheduled compounding; and off the loop only after the operator has proved the bounds.

## Out Of Scope

AgentOps does not provide:

- Formal certification, accreditation, authorization to operate, or airworthiness approval.
- Data classification, declassification, or cross-domain transfer.
- Model approval, model monitoring, or model safety certification.
- Secret scanning as a complete DLP boundary.
- A guarantee of zero network egress when the operator uses external model runtimes, remotes, installers, or tools.
- Formal verification of generated code.
- Tool qualification for safety-critical release by itself.

## Roadmap

The current profile defines the posture. The next hardening steps are:

- Control-mapping templates for common internal assurance programs.
- Evidence export bundles for councils, RPI runs, gates, and post-mortems.
- Redaction workflows for `.agents/` artifacts.
- Stronger policy around approved model/runtime profiles.
- Retention profiles for personal, team, constrained, and accredited environments.
- More explicit supply-chain guidance for mirrored installers and pinned artifacts.

The product direction is clear: keep agent work fast, but make the speed legible to serious operators.
