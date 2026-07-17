# Bun's Eleven-Day Rust Port: AgentOps/Gas City Research Note

Date: 2026-07-17

Question: What exactly did Bun accomplish in eleven days, which workflow
mechanisms caused the speed, and which mechanisms should AgentOps/Gas City adopt
without copying unsafe authority or integration practices?

This is a dated research artifact, not a claim about current Bun release state.

## Precise event

Jarred Sumner led an AI-driven mechanical port of Bun's Zig implementation to
Rust, merged as
[Bun PR #30412](https://github.com/oven-sh/bun/pull/30412) on May 14, 2026.
The PR preserved the existing architecture and data structures and deferred
cleanup and optimization. The eleven-day milestone was all-platform test pass
plus merge to `main`, not a ground-up redesign, complete conversion of embedded
C/C++ dependencies, idiomatic-safe-Rust completion, or stable-release finish.

Bun's [official account](https://bun.com/blog/bun-in-rust) reports a mechanical
port of 535,496 Zig lines, four worktree shards, 64 Claude agents, all six CI
platforms green, continued post-merge security review and fuzzing, and known
regressions repaired afterward. The reported $165,000 is an API-price estimate,
not an independently audited cash invoice.

## Observed mechanisms

### Contract before fan-out

Roughly three hours of expert dialogue produced a detailed `PORTING.md`. A
separate workflow classified cross-cutting ownership and lifetime decisions into
`LIFETIMES.tsv`. The committed
[Phase-A porting guide](https://github.com/oven-sh/bun/commit/46d3bc29f270fa881dd5730ef1549e88407701a5)
defines faithful-capture rules, path/type mappings, forbidden invention, and the
separation between noncompiling logic capture and later compilation repair.

This transformed a vague million-line goal into a constrained translation
contract.

### Pilot before scale

The workflow tried three files through implement, adversarial review, and fix
before expanding to the full file set. Scale followed policy stabilization; it
did not substitute for it.

### Phase-specific deterministic queues

Workers were not told merely to "rewrite Bun." Work moved through observable
phases:

```text
faithful mechanical capture
  -> compiler errors grouped by crate/file
  -> linker and startup failures
  -> CLI subcommand smoke failures
  -> local test failures grouped by file/signature
  -> platform-specific CI failures
  -> cleanup, unsafe review, security review, and fuzzing
```

At merge commit `23427dbc12fdcff30c23a96a3d6a66d62fdc091d`, the
[crate-shard workflow](https://github.com/oven-sh/bun/blob/23427dbc12fdcff30c23a96a3d6a66d62fdc091d/.claude/workflows/phase-d-crate-shard.workflow.js#L54-L130)
runs one global check, serializes and sorts errors into work artifacts, and
prevents individual agents from rerunning the expensive global command.

### Controller-owned identity and paths

The committed
[Phase-A workflow](https://github.com/oven-sh/bun/blob/23427dbc12fdcff30c23a96a3d6a66d62fdc091d/.claude/workflows/phase-a-port.workflow.js#L15-L73)
computes canonical output paths and response schemas. It later
[overwrites the model-reported path](https://github.com/oven-sh/bun/blob/23427dbc12fdcff30c23a96a3d6a66d62fdc091d/.claude/workflows/phase-a-port.workflow.js#L148-L165)
with the controller's canonical value. Agents supplied candidate content and
findings; the workflow owned identity and routing facts.

### Isolation learned through collision

The initial approach allowed stash/reset behavior and agents stepped on one
another. The workflows then prohibited global Git operations, used explicit
paths and static shard ownership, split work across four worktrees, and later
added cgroup/PID isolation. The committed
[isolated test workflow](https://github.com/oven-sh/bun/blob/23427dbc12fdcff30c23a96a3d6a66d62fdc091d/.claude/workflows/phase-g-test-swarm-isolated.workflow.js#L80-L110)
creates shard worktrees and resource fences.

### Separate adversarial contexts

Implementers and reviewers were separate contexts. Later workflows used two
reviewers and sometimes a third tiebreaker. The
[B2 verifier](https://github.com/oven-sh/bun/blob/23427dbc12fdcff30c23a96a3d6a66d62fdc091d/.claude/workflows/phase-b2-verify.workflow.js#L41-L70)
combines two votes and routes disputed logic findings to a third agent.

Reviewers caught plausible compiling defects, including an asynchronous libuv
use-after-free/double-free repaired in
[commit `f0a454376c7`](https://github.com/oven-sh/bun/commit/f0a454376c7).

### Human operated the workflows

Sumner monitored outputs and changed policies when agents collided, stubbed
functions, wrote workaround essays, or exhausted resources. The run was not an
unattended generic swarm. The closest AgentOps analogue is a domain-expert Mayor
who revises the governing contract and task shape rather than directly editing
every candidate.

### Strong existing oracle and post-merge assurance

Bun had a large language-independent TypeScript test suite. All supported
platforms being green was a strong compatibility signal, and Sumner manually
checked that tests had not simply been deleted or skipped. It still did not
prove total behavioral equivalence or Rust soundness.

Immediately after merge,
[issue #30719](https://github.com/oven-sh/bun/issues/30719) demonstrated that a
safe Rust API erased a slice lifetime and could produce a dangling reference.
The follow-up marked the API unsafe and audited call sites. The official account
also reports known regressions and continued security review and fuzzing.

## Transfer into AgentOps/Gas City

| Bun mechanism | Factory rule | Guardrail |
|---|---|---|
| Detailed porting contract and lifetime table | Mayor produces digest-bound contract and phase DAG; fresh plan Judge attacks it | Contract constrains work; it cannot waive architecture or acceptance |
| Three-file pilot | Start new factory patterns with a small representative cohort | Compilation alone does not promote the pattern |
| Phase-specific queues | Reducer materializes bounded packets from compiler/test/smoke/CI evidence | Re-run the authoritative global check after aggregation |
| One global check, serialized failures | Expensive analysis is controller work; Workers consume finite queues | Workers cannot hammer global build/test state independently |
| Four worktrees and later resource fences | One fenced writer/applier per coarse shard, disjoint worktree/index/branch/lease | Many read-only reviewers do not justify shared mutable writers |
| Separate adversarial reviewers | Advisory review may occur before candidate freeze; binding fresh Validator follows | Author/reviewer feedback is not a durable lifecycle verdict |
| Workflow changed after systematic defects | Mayor revises contract/skill/formula and creates new experiment identities | After binding non-PASS, neither reviewer nor Refinery repairs the rejected subject |
| All-platform CI and manual skip check | Bind deterministic receipts and fresh judgment to exact integration subject | Green tests do not prove equivalence, soundness, or release readiness |
| Post-merge security, fuzzing, and canary | Delivery, canary, security, and release remain separate receipts/stages | Refinery landing cannot erase residual risk |

The corresponding factory phase shape is:

```text
Mayor-owned contract
  -> fresh plan review
  -> representative pilot
  -> mechanical-capture queue
  -> compiler-error queue
  -> subcommand-smoke queue
  -> local-test queue
  -> cross-platform CI queue
  -> frozen candidate and binding Validator
  -> admission certificate
  -> fenced Refinery integration
  -> fresh integrated-subject court
  -> protected merge
  -> canary, security review, and fuzzing
```

## Advisory review versus binding validation

Bun's inner `implementer -> reviewers -> fixer` loop is useful inside an
experiment. It is weaker than AgentOps' exact-subject completion boundary because
some workflows allow a fixer to mutate code after review without a fresh final
review in the same workflow.

AgentOps/GC therefore separates the loops:

```text
Worker implementation
  -> optional advisory reviewer/fixer convergence
  -> freeze exact candidate
  -> fresh binding Validator
      PASS              -> Refinery admission
      FAIL/NOT_PROVEN   -> Mayor rescope -> new identity and fresh Worker
```

Any Refinery mutation similarly invalidates candidate-level PASS and requires
fresh integrated-subject validation.

## What not to copy

- A million-line PR as normal product-delivery policy.
- Shared mutable writers using stash, reset, or unscoped Git.
- Sixty-four agents or the API-price estimate as a default capacity target.
- One provider family as the only assurance perspective.
- The original author simultaneously holding semantic proposal, binding
  judgment, graph mutation, and merge authority.
- Green compilation/tests as proof of soundness or total equivalence.
- An assumption that repositories with weak executable acceptance can reproduce
  Bun's parallelization surface.

## Inference

Bun demonstrates that very high throughput can come from manufacturing small,
observable queues around a stable contract. It does not demonstrate the exact
Fenced Steward lifecycle, because Bun's workflow combined advisory review and
fixing differently and retained concentrated human authority.

The AgentOps inference is to retain Bun's contract, queue, isolation, resource,
and adversarial-review mechanisms while imposing stricter separation between
Mayor proposal, Validator judgment, reducer state mutation, and Refinery
delivery.

## Contradictions

- The official article says every generated line received two adversarial
  reviews, while the committed final Phase-A workflow visibly invokes one
  verifier and the lifetime workflow samples some high-confidence categories.
  Other later workflows clearly use two reviewers. Public sources do not
  independently prove universal two-review coverage.
- The article reports 6,778 total commits and 6,502 non-merge commits, while the
  GitHub PR page reports 6,755 PR commits. The counting conventions are
  unreconciled.
- The $165,000 figure is an API-price estimate, not an audited invoice.
- "Rewritten in Rust" is broadly accurate for Bun's Zig implementation but not
  literally every embedded component or dependency.

## Unknown or unchecked

- Private model/session logs and every reviewer invocation.
- Actual cash cost and complete human staffing.
- Whether the reported regression count is exhaustive.
- An independent proof of behavior equivalence or unsafe-code soundness.
- Long-term maintainability and stable-release outcomes beyond the dated
  sources reviewed here.

## Primary sources

- [Bun's official Rust-port write-up](https://bun.com/blog/bun-in-rust)
- [Bun PR #30412](https://github.com/oven-sh/bun/pull/30412)
- [Phase-A porting-guide commit](https://github.com/oven-sh/bun/commit/46d3bc29f270fa881dd5730ef1549e88407701a5)
- [Merge-commit Phase-A workflow](https://github.com/oven-sh/bun/blob/23427dbc12fdcff30c23a96a3d6a66d62fdc091d/.claude/workflows/phase-a-port.workflow.js)
- [Merge-commit crate-shard workflow](https://github.com/oven-sh/bun/blob/23427dbc12fdcff30c23a96a3d6a66d62fdc091d/.claude/workflows/phase-d-crate-shard.workflow.js)
- [Merge-commit isolated test workflow](https://github.com/oven-sh/bun/blob/23427dbc12fdcff30c23a96a3d6a66d62fdc091d/.claude/workflows/phase-g-test-swarm-isolated.workflow.js)
- [Merge-commit B2 verifier](https://github.com/oven-sh/bun/blob/23427dbc12fdcff30c23a96a3d6a66d62fdc091d/.claude/workflows/phase-b2-verify.workflow.js)
- [Post-merge soundness issue #30719](https://github.com/oven-sh/bun/issues/30719)
- [Andrew Kelley's technical response](https://andrewkelley.me/post/my-thoughts-bun-rust-rewrite.html)
