<div align="center">

# AgentOps

### Fresh-context validation for coding-agent work

Coding agents are stochastic and can declare work done when it is not.
AgentOps turns one behavior into one bounded experiment, gives the exact result
to a fresh validator, and stores the verdict under your control.

</div>

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

## Install

```bash
# Install the optional CLI.
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops

# Keep one canonical checkout and link it into every installed runtime.
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link
```

`ao skills link` creates source symlinks under the portable
`~/.agents/skills` root and every detected runtime skill root, including Claude,
Codex, Gemini/Antigravity, and Cursor. It never replaces a real directory or a
foreign symlink. Updates stay simple:

```bash
cd ~/.local/share/agentops
git pull --ff-only
ao skills link
```

Without Homebrew, build the optional CLI from the checkout and then link:

```bash
cd ~/.local/share/agentops/cli
go install ./cmd/ao
cd ..
"$(go env GOPATH)/bin/ao" skills link
```

The 3.x runtime plugin installers are tombstones for this release: they exit
nonzero and point here. New installs use one canonical checkout plus
`ao skills link`. See the [migration guide](docs/MIGRATION.md) for removing an
old plugin install.

## Core workflow

```text
> use plan for bead agentops-123
Plan refines acceptance and scope in the bead; it creates no second plan file

> use implement
RED -> GREEN -> refactor; runtime derives changed paths and content manifest

> use validate
verdict.v2: FAIL — burst refill violates scenario S2
checked: S1, S2, subject identity, write scope
not_checked: load behavior above declared limit
```

Or invoke `rpi` with `rate-limit /login` to run the three responsibilities once and
receive one report. RPI stops after `PASS`, `FAIL`, or `NOT_PROVEN`; the caller
decides whether to revise, deliver, or abandon the work.

## Core skills

| Skill | Responsibility |
|---|---|
| `rpi` | invoke Plan, Implement, and fresh Validate at most once |
| `plan` | refine behavior, acceptance, evidence, and scope in the existing intent source |
| `implement` | run one bounded experiment; let the runtime derive subject facts |
| `validate` | independently judge exact content and persist `verdict.v2` |

`learn` is an optional later analysis of verdict collections. `premortem`,
`postmortem`, `council`, and idea genies are caller-selected strategies.
Factory/runtime skills such as NTM, Agent Mail, Gas City, and swarms are optional
adapters. None can change core sequencing or a verdict.

## The evidence contract

A PASS binds:

- unchanged acceptance;
- a deterministic manifest of files, symlinks, deletions, executable bits, and
  content digests;
- complete changed-path coverage inside the bead or caller-defined write scope;
- distinct author and validator context IDs;
- an explicit freshness attestation;
- criterion results, evidence references, checked scope, and omissions.

Missing identity, mutation, or incomplete coverage is `NOT_PROVEN`. A proven
out-of-scope change or failed acceptance criterion is `FAIL`.

Verdicts default to `.agentops/verdicts/sha256/<digest>.json`. They are plain,
content-addressed JSON and do not require Git, `ao`, a tracker, a hosted service,
or a provenance ledger. The acceptance digest is derived from the existing
intent source; models do not author Plan, Candidate, or revision packets.

## Product boundary

AgentOps reads or refines caller-owned intent, runs one bounded experiment,
derives exact content identity, obtains independent judgment, and stores the
durable verdict. It does not own retries, budgets,
queues, work claims, Git, CI, PRs, merges, closure, release, or delivery.

Use your repository's existing direct-push, PR, merge queue, hosted CI, and
release process after validation. Local and cloud agents use the same intent,
manifest, and verdict boundaries.

## Honest status

Fresh independent judgment is a practical trust boundary, not a guarantee that
stochastic output is correct. Context identities and freshness are declared
facts, not cryptographic isolation. The longer-term learning hypothesis—that
recurring verdict findings can improve future context and deterministic
checks—remains off the critical path and must be measured.

[Product boundary](PRODUCT.md) · [Operating loop](docs/architecture/operating-loop.md) · [CLI commands](cli/docs/COMMANDS.md) · [Skill router](docs/SKILL-ROUTER.md) · [Docs index](docs/documentation-index.md)

Contributing: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md). License: Apache-2.0.
