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
# Claude Code
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace

# Codex CLI (macOS/Linux/WSL)
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash

# Codex CLI (Windows)
irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.ps1 | iex

# Gemini / Antigravity
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash

# Other skills-compatible agents
npx skills@latest add boshu2/agentops --cursor -g
```

The optional `ao` CLI supplies deterministic repository checks and inspection:

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
ao gate check --fast
```

Installers are hookless. An agent runtime and Git are the only hard external
requirements; individual skills declare any additional tool needs.

## Core workflow

```text
> /plan "rate-limit /login"
PlanPacket: normal + burst edge scenarios, exact scope, first acceptance check

> /implement
CandidatePacket: RED -> GREEN -> refactor, actual paths, content manifest

> /validate
verdict.v2: FAIL — burst refill violates scenario S2
checked: S1, S2, subject identity, write scope
not_checked: load behavior above declared limit
```

Or invoke `/rpi "rate-limit /login"` to run the three responsibilities once and
receive one report. RPI stops after `PASS`, `FAIL`, or `NOT_PROVEN`; the caller
decides whether to revise, deliver, or abandon the work.

## Core skills

| Skill | Responsibility |
|---|---|
| `/rpi` | invoke Plan, Implement, and fresh Validate at most once |
| `/plan` | define one behavior, acceptance, evidence, and write scope |
| `/implement` | run one bounded experiment and describe the candidate |
| `/validate` | independently judge exact content and persist `verdict.v2` |

`/learn` is an optional later analysis of verdict collections. `/premortem`,
`/postmortem`, `/council`, and idea genies are caller-selected strategies.
Factory/runtime skills such as NTM, Agent Mail, Gas City, and swarms are optional
adapters. None can change core sequencing or a verdict.

## The evidence contract

A PASS binds:

- unchanged acceptance;
- a deterministic manifest of files, symlinks, deletions, executable bits, and
  content digests;
- complete changed-path coverage inside the Plan write scope;
- distinct author and validator context IDs;
- an explicit freshness attestation;
- criterion results, evidence references, checked scope, and omissions.

Missing identity, mutation, or incomplete coverage is `NOT_PROVEN`. A proven
out-of-scope change or failed acceptance criterion is `FAIL`.

Verdicts default to `.agentops/verdicts/sha256/<digest>.json`. They are plain,
content-addressed JSON and do not require Git, `ao`, a tracker, a hosted service,
or a provenance ledger.

## Product boundary

AgentOps owns intent shaping, one bounded experiment, exact content identity,
independent judgment, and the durable verdict. It does not own retries, budgets,
queues, work claims, Git, CI, PRs, merges, closure, release, or delivery.

Use your repository's existing direct-push, PR, merge queue, hosted CI, and
release process after validation. Local and cloud agents use the same packet and
verdict contracts.

## Honest status

Fresh independent judgment is a practical trust boundary, not a guarantee that
stochastic output is correct. Context identities and freshness are declared
facts, not cryptographic isolation. The longer-term learning hypothesis—that
recurring verdict findings can improve future context and deterministic
checks—remains off the critical path and must be measured.

[Product boundary](PRODUCT.md) · [Operating loop](docs/architecture/operating-loop.md) · [CLI commands](cli/docs/COMMANDS.md) · [Skill router](docs/SKILL-ROUTER.md) · [Docs index](docs/documentation-index.md)

Contributing: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md). License: Apache-2.0.
