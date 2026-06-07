# PR Creation From Linked Worktrees

Date: 2026-06-06
Related bead: `ag-9zls` (`[BUG] Background agents: fix nested repo PR creation path`)
Surface: `git worktree`, `gh`, background-agent (cursor/\*) PR tooling

## Symptom

A branch created in a linked worktree (`git worktree add -b …` or `bd worktree
create`) pushes cleanly, but the step that turns it into a PR fails:

```
No branch name available for PR creation
```

Observed live (2026-06-03) from `/home/boful/dev/agentops` nested worktrees:
branches pushed fine, but `ManagePullRequest` (the background-agent / cursor PR
tool) could not create the PR, so background-agent output stayed incomplete —
work landed as an unmergeable branch.

## Root Cause (verified)

`git worktree add -b <branch> origin/main` inherits its **upstream tracking from
the start point**. The new branch is configured to track `main`, not itself:

```console
$ git config --get-regexp '^branch\.<branch>\.'
branch.<branch>.remote origin
branch.<branch>.merge  refs/heads/main      # ← the bug

$ git rev-parse --abbrev-ref --symbolic-full-name @{u}
origin/main                                 # ← upstream points at main, not the branch
```

Any PR tool that infers the **head** branch from upstream tracking config
(rather than from `HEAD`'s ref name) resolves the head to `main`. A PR from
`main` into `main` has no distinct head → *"No branch name available for PR
creation."* The branch ref exists and is pushed; only the *inferred* head is
wrong. This is a tracking-config defect, not a push or auth failure — which is
why the branch is visibly on the remote yet the PR step still fails.

This is inherent to `worktree add -b … <remote-start-point>` under Git's default
`branch.autoSetupMerge` — it is not specific to the background-agent harness;
the harness just surfaces it because it auto-infers head.

## Fix (verified end-to-end)

Push with an **explicit refspec and upstream**. This rewrites
`branch.<branch>.merge` to the branch itself:

```console
$ git push -u origin HEAD:<branch>

$ git config --get-regexp '^branch\.<branch>\.'
branch.<branch>.remote origin
branch.<branch>.merge  refs/heads/<branch>           # ← fixed

$ git rev-parse --abbrev-ref --symbolic-full-name @{u}
origin/<branch>                                      # ← upstream now the branch
```

After this, head inference resolves correctly and PR creation succeeds.

## Robust PR-creation path (do not rely on inference)

Always name the head explicitly so the PR tool never has to guess. Three tiers,
in order of how much GitHub access the caller has:

### Tier 1 — `gh` with a write-capable token (preferred)

```bash
git push -u origin HEAD:<branch>
gh pr create \
  --head <branch> \
  --base main \
  --title '<type>(<scope>): <subject> (<bead-id> #<scenario-slug>)' \
  --body  '<trailers per CLAUDE.md>' \
  --draft
```

`--head` explicitly **skips** `gh`'s push/fork inference (the step that prompts
"Where should we push…?" and otherwise mis-resolves the head). See
`gh pr create --help`.

### Tier 2 — `gh api` when `gh pr create`'s interactive path is blocked

Headless/background agents often have a token that can call the REST API but
cannot run `gh pr create`'s interactive push flow. Create the PR directly:

```bash
git push origin HEAD:<branch>          # ensure the ref is on the remote first
gh api repos/{owner}/{repo}/pulls \
  -f head='<branch>' \
  -f base='main' \
  -f title='<title>' \
  -f body='<body>' \
  -F draft=true
```

`{owner}/{repo}` resolves from the remote (`gh repo view --json nameWithOwner -q
.nameWithOwner`).

### Tier 3 — no write token at all (compare-URL handoff)

When the agent has **no** write-capable `gh`, it still pushed the branch over
SSH/HTTPS git creds. Emit the compare URL for a human (or a downstream
write-capable step) to open the PR:

```bash
git push origin HEAD:<branch>
echo "https://github.com/<owner>/<repo>/compare/main...<branch>?expand=1"
```

Post that URL to the handoff/queue thread. This is the documented workaround
when "without using write-capable gh" is a hard constraint — the branch is
real and mergeable; only the PR-open click is deferred.

## Prevention

Pick one and make it the worktree convention:

- **Push with explicit upstream every time:** `git push -u origin HEAD:<branch>`
  (fixes tracking on first push — the path above).
- **Create the branch untracked:** `git worktree add -b <branch> --no-track
  origin/main`, so `merge` is never set to `main` in the first place.
- **Repair after the fact:** `git branch --set-upstream-to=origin/<branch>`.

Whichever you choose, **always pass `--head <branch>` to PR creation.** Never let
the tool infer the head from tracking config in a linked worktree.

## Evidence

Validated in this repo on 2026-06-06 from a linked worktree
(`docs/ag-9zls-worktree-pr-runbook` off `origin/main`): the before/after
`branch.<branch>.merge` flip shown above was captured live, and this very
document's PR was opened from that worktree using the Tier-1 path
(`git push -u origin HEAD:<branch>` then `gh pr create --head <branch> --base
main --draft`).

## See also

- `skills/swarm/references/shared-checkout-discipline.md` — when a worktree is required.
- `skills/push/SKILL.md` — the test→commit→push half (stops before PR creation; this runbook covers the rest).
