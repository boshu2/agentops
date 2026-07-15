# Dependencies

AgentOps's core protocol requires an agent runtime capable of reading and
writing files. It does not require Git, the `ao` CLI, a tracker, a queue, a
network connection, or a delivery system.

| Tool or substrate | Role | Core requirement |
|---|---|---|
| Agent runtime | Runs Plan, Implement, and a distinct fresh Validate context | Required |
| Filesystem + SHA-256 helper | Computes content identity and stores `verdict.v2` | Required |
| `ao` | Repository utilities and ordinary deterministic `ao gate check` checks | Optional |
| Git or a forge | Version control, review, and delivery owned by the caller | Optional |
| `br`, `bv`, or another tracker | Caller-owned work organization | Optional |
| NTM, Agent Mail, Gas City, Swarm, or another factory | Caller-selected execution or coordination adapter | Optional |
| Council or Dueling Idea Genies | Additional independent judgment strategy | Optional |
| Go | Builds the optional `ao` CLI from source | Optional |

Optional tools may produce factual evidence or execute explicit packets. Their
attempts, queues, leases, commits, or delivery state never become AgentOps
correctness state. Missing optional tools can reduce what was checked, but they
cannot reorder RPI or manufacture a PASS.

For CLI installation details, see the [README](../README.md). For the exact
core boundary, see the [operating loop](architecture/operating-loop.md).
