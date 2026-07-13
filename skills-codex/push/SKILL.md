---
name: push
description: Run repository-selected deterministic
---
# Push Skill

Run the target repository's chosen Git delivery path after deterministic checks.
Push is an optional driving adapter, not a fifth lifecycle umbrella and not
release authority.

## Constraints

- **Repository policy wins.** Read the target repository's delivery instructions
  or the operator's explicit choice. Direct push, PR, and user-owned CI are all
  valid; this skill does not replace them with an AgentOps queue.
- **Deterministic checks only.** Run declared tests, linters, builds, and gates.
  Push never requires another LLM landing verdict after Validate has emitted
  proof.
- **Proof is input, not permission.** A repository may archive or cite an
  immutable Validate result, but Push cannot change that verdict or make it a
  Git authorization token.
- **No lifecycle mutation.** Do not close trackers, run Learn, alter phase
  receipts, or declare the objective complete. Return delivery evidence to the
  caller.
- **Contain the write surface.** Stage only requested files and never include
  credentials or private ledgers.

## Delivery boundary

```text
Discovery -> Crank -> Validate -> Learn -> orchestrator
                                      |
                                      +-- immutable proof may be consumed here

repository/operator -> Push adapter -> direct push | PR | user-owned CI
```

The two paths meet only through immutable inputs. Delivery cannot retroactively
complete or rewrite the lifecycle.

## Workflow

1. **Resolve repository policy.** Read repo-local instructions and the explicit
   operator request. If no delivery path is authorized, return the prepared
   commit and deterministic evidence; do not invent one.
2. **Pin the payload.** Record the source SHA, target remote/ref or PR branch,
   and exact staged paths. Reject credentials, `_beads`, and unrelated changes.
3. **Run deterministic checks.** Execute the repository's declared commands.
   In AgentOps itself the normal fast check is
   `ao gate check --fast --scope head`; other repositories may use their own CI,
   test script, or pre-push hook. A red command stops delivery.
4. **Commit when requested.** Preserve repository message conventions and
   report the resulting SHA. Committing is not validation and does not mutate
   an existing Validate verdict.
5. **Use the selected adapter.** Direct-push repositories use ordinary Git;
   PR repositories push a branch and hand off to their PR tooling; user-owned
   CI repositories follow their documented trigger. Do not create or manage a
   global queue.
6. **Report evidence.** Return the source SHA, destination, deterministic
   commands and exits, and remote/PR identity when one was created. Stop.

## Output Specification

- **Artifact:** a delivery result containing source SHA, selected adapter,
  destination identity, and deterministic command results.
- **Serialization:** caller-selected text or JSON; it must not embed or mutate a
  Validate verdict.
- **Validator:** `bash skills/push/scripts/validate.sh` validates this adapter
  contract. Repository checks remain repository-owned.
- **Downstream:** return the result to the caller. Tracker closeout and lifecycle
  state are outside this skill.

## Guardrails

- NEVER stage files matching: `.env*`, `*credentials*`, `*secret*`, `*.key`, `*.pem`.
- Stage only files relevant to the work. Never stage `_beads`.
- Never force-push unless the operator and repository policy explicitly authorize
  that exact ref.
- Never substitute AgentOps delivery policy for the target repository's policy.

## Quality Checklist

- The repository/operator-selected adapter is named.
- The payload SHA, paths, and destination are explicit.
- Every required deterministic command is green on the delivered payload.
- No LLM landing verdict, AgentOps Git queue, tracker mutation, or lifecycle
  completion claim was introduced.

## Reference Documents

- [references/push.feature](references/push.feature) — executable deterministic
  adapter scenarios.
