# Corpus Delta Held Task Set

Bead: ag-nfux, W1b of ag-8p8o.

The original design document named in the bead,
`.agents/research/2026-06-03-corpus-delta-experiment-design.md`, is absent from
this workspace. The local replacement plan records it as lost in the 2026-05-07
`.agents` wipe and restates the W1b constraint: expand the held set to at least
10 realistic, under-specified tasks whose relevant prior decisions exist in the
corpus, with deterministic graders and no LLM judge.

## Selection Rule

Tasks were selected from recurring AgentOps operational failures that the corpus
documents as decisions, guardrails, or incident lessons. They are not selected
because the exact requested script already exists in the corpus. The expected
answer is a small new guard script, while the helpful context is the local
doctrine: what counts as a violation, what is allowed, and why the rule matters.

Every task ships:

- `prompt.md`: the agent-facing task prompt.
- `setup.sh`: an isolated workspace fixture.
- `score.sh`: a deterministic pass/fail grader.
- `golden-solution.sh`: a reference implementation used only by fixture tests.

The grader-discrimination gate requires each golden solution to pass and an
empty workspace to fail.

## Held Tasks

| Task | Corpus decision it exercises | Why it plausibly benefits from corpus |
|---|---|---|
| `cd-ci-1` | No advisory middle tier in CI: a continue-on-error job cannot also be a PR check. | The rule is local CI doctrine, not generic GitHub Actions trivia. |
| `cd-ci-4` | Removing a named CI job requires a corpus grep for surviving assertions. | The failure mode is a local release/eval artifact pattern. |
| `cd-am-1` | Agent Mail reservation responses must fail closed on any file conflict. | The AM conflict-prevention contract is local operating doctrine. |
| `cd-beads-1` | `br` must be scoped with `BEADS_DIR=$PWD/_beads`; legacy/default stores are unsafe. | The root tracker migration and file-backed DB rule are repo-specific. |
| `cd-bv-1` | `bv` is never run bare; only robot modes are safe in automation. | This prevents a real local TUI hang failure encoded in the beads skill. |
| `cd-door9-1` | `claude -p` / `claude --print` are forbidden command surfaces. | The ban is a local quota/safety law, not a universal shell rule. |
| `cd-git-1` | PR-only lanes must never push directly to `main`. | The release gate and orchestrator merge boundary are local workflow rules. |
| `cd-worktree-1` | Work starts in a per-bead worktree whose path names the bead. | The durable-lane model depends on bead-scoped worktrees, not shared root edits. |
| `cd-generated-1` | Generated inventories cannot be hand-edited without source changes. | The generated artifact list and source-of-truth order are local contracts. |
| `cd-agents-1` | Runtime `.agents` evidence must not be committed as product surface. | The repo distinguishes corpus/runtime artifacts from tracked source. |

The set deliberately mixes CI, tracker, coordination, runtime, and generated
artifact surfaces so success is not reducible to one regex trick or one domain.
