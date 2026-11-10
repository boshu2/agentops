# AgentOps

**Software engineering practice, re-tested for AI agents.**

AgentOps runs established engineering practices against AI coding agents and
ships the ones that hold up as optional skills. The
[practice ledger](practice-ledger.md) records which practices held, which
changed shape, which did not survive and which are still being tested. The
skills help an agent define the behavior you need, implement it in your
domain's language and check the result independently, while your repository
keeps its tests, tracker and delivery workflow.

## Start with the behavior

Behavior-driven development (BDD) means agreeing on examples of what the
software should do before coding, then checking the result against those same
examples. Given/When/Then expresses the starting situation, action, and expected
result. Plain text in an issue or conversation is enough.

Domain-driven design (DDD) keeps the domain's terms and rule ownership
consistent across those examples, implementation, and review. Passing tests and
CI check the change; where a mistake would be costly, a fresh reviewer also
checks the exact change against the original request.

```text
Accepted intent -> native implementation and checks -> one fresh read where a mistake is costly -> finish
```

## Leave useful improvements behind

A fix can leave a regression test, a tool can remove repeated work, and a
recorded decision can preserve context for the next agent. Engineering
compounds when subsequent work benefits from those improvements. Saving a note
or recording a completed task alone does not establish that benefit.

Native execution requires zero AgentOps skills. Start with your coding agent
and repository checks; load specialist guidance for a concrete need. Review
evidence can be saved in caller-selected storage when requested.

## Start here

- [Install and try one task](https://github.com/boshu2/agentops#quickstart)
- [First value path](first-value-path.md)
- [How it works](how-it-works.md)
- [Architecture](ARCHITECTURE.md)
- [Skill router](SKILL-ROUTER.md)
- [CLI reference](cli/commands.md)
- [Migration](MIGRATION.md)

AgentOps is the operations layer for agentic engineering. Its
[product boundary](https://github.com/boshu2/agentops/blob/main/PRODUCT.md) leaves
work, execution, and delivery with the caller's tools. The
[Practice Registry](https://github.com/boshu2/agentops/blob/main/PRACTICE-REGISTRY.md)
records the engineering traditions behind the guidance.

The `ao` CLI is a supporting tool for deterministic repository checks and
evidence inspection. The semantic loop can run in a non-Git directory without
`ao`.
