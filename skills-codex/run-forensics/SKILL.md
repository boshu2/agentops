---
name: run-forensics
description: 'Answer questions about an agent run that already happened by reading its recording instead of the agent''s memory. Triggers: "why did it do that", "which step changed this file", "reproduce that run", "replay the failure".'
---
# run-forensics

## Purpose

An agent asked "why did you do that?" answers from a summary of its own context window. The tool
results, the shell exit codes and the files that changed without anyone mentioning them are already
gone from it. The answer comes out fluent, confident and occasionally wrong — worse than "I don't
know", because it gets believed and written into a commit message.

This skill enforces one rule: when a question is about something that already happened, read the
recording before answering. It uses [OrcaReplay](https://github.com/Continuum-AI-Corp/OrcaReplay)
(Apache-2.0, npm, Node 20+), which records an agent at the HTTP boundary to its model provider from
outside the process and can serve that recording back with the provider unreachable.

## When to Use

- A past run changed a file, ran a command or broke a build, and nobody knows which step did it.
- A colleague reports a failure you cannot reproduce, and you do not have their key or their machine.
- A failed session should become a regression test rather than a paragraph in an issue.
- Someone is treating an agent's own explanation as a conclusion and you need to know whether
  evidence supports it.

Do **not** use this for planning the next change, reviewing a diff, or debugging code no agent ran.
If there is no recording, say so and offer to start one — do not substitute recall.

## Inputs

- A run identifier, or "the most recent run".
- The specific thing to explain: a file change, a command, a build failure.
- Whether the ask is explanation, reproduction, or model comparison.
- The acceptable blast radius for a replay: may it reach a database, a container, another host?

## Instructions

1. **Confirm a recording exists.** Run `orca list`. If it is empty, state that plainly and offer
   `orca record <agent> -- <command>`; stop rather than reconstructing from memory.
2. **Classify the question and read only what it needs.**
   - *What happened?* → `orca show <run>`: model turns with token counts and stop reasons, tool
     calls with arguments and results, shell commands with exit codes, files changed.
   - *Why did this happen?* → `orca graph <run> --to <event>`: the causal chain to that one event.
     Reading a 200-event timeline and reasoning over it is slower, costs more context, and invites
     the confident guess this skill exists to prevent.
   - *Does it still reproduce?* → `orca replay <run>`.
3. **Label every causal claim** as `recorded` (the recorder watched it) or `inferred` (derived at
   query time from a named rule). Never merge the two.
4. **Read the recorded shell commands before any replay.** A replay is not a dry run: the agent
   process runs again, so every command it issued runs again. List what will repeat first.
5. **Replay into a scratch worktree** (`orca replay <run> --worktree`). Otherwise the recorded file
   tree is restored over the working tree, and uncommitted work is absent meanwhile.
6. **Report the verdict line verbatim**, then the residual uncertainty.

## Output

- One sentence answering the question asked.
- The specific events behind it: sequence numbers, types, key arguments, exit codes.
- A `recorded` / `inferred` label on every causal claim.
- If replayed: the verdict line quoted exactly (`reused` / `exact` / `divergences` / `unmatched`),
  plus which side effects actually repeated.
- What the recording does not cover.

## Examples

```text
Why did the build break in run_4f2a? Use the recording, not your memory.
Which step deleted config.yaml in the last run?
Replay run_4f2a into a worktree and tell me whether it still fails.
```

## Troubleshooting

- Symptom: `orca list` is empty after a run you watched happen.
  Fix: the agent pins its own provider origin and reads no base-URL variable, so nothing was
  captured. Record it with `orca attach --port <n>` and point the agent's config at that port.
- Symptom: `reused=3/5` looks like a partial failure.
  Fix: it usually is not. Harnesses make calls for themselves — a quota probe, a session-naming
  request — and a replay does not repeat them.
- Symptom: a vision agent replays with divergences instead of `exact`.
  Fix: expected. A re-rendered screenshot is different bytes.
- Symptom: replay fails to start in a Node project.
  Fix: a scratch worktree is built from tracked files, so `node_modules` is absent. Replay in place
  for those, and say so in the report.

## Boundaries

Three things this skill must never claim:

- **A matching replay is not a determinism result.** It shows the recorded run reproduces, not that
  the model is stable across calls.
- **`egress=blocked` means model-provider egress only.** Recorded tool calls still execute for real
  on replay — a recorded `curl` reaches the network. Replay is not a sandbox.
- **Embedding calls are not captured** by the default adapter, so a RAG step's retrieval is absent
  from the trace even when the chat turns are complete.
