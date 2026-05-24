# Model Quality Profiles

Pre-configured model + judge count combinations for different use cases.

Use `--profile=<name>` to select a profile. Profiles set environment variables before agent spawning.

## Profiles

| Profile | COUNCIL_CLAUDE_MODEL | Judge Count | COUNCIL_TIMEOUT | Use Case |
|---------|---------------------|-------------|-----------------|----------|
| `thorough` | opus | 3 | 120 | Architecture decisions, security audits |
| `balanced` | sonnet | 2 | 120 | Default validation, routine reviews |
| `fast` | haiku | 2 | 60 | Quick checks, mid-implementation sanity |

## Cost Tiers

Model cost tiers provide a unified naming convention across all skills. Tiers map to profiles:

| Cost Tier | Profile | COUNCIL_CLAUDE_MODEL | Use Case |
|-----------|---------|---------------------|----------|
| `quality` | `thorough` | opus | High-stakes decisions, security, architecture |
| `balanced` | `balanced` | sonnet | Default validation, routine reviews |
| `budget` | `fast` | haiku | Quick checks, mid-implementation sanity |
| `inherit` | (uses default) | (from config) | Use the project/global default tier |

### Tier Resolution

Tiers are resolved from configuration with this precedence:

1. Explicit env var (`COUNCIL_CLAUDE_MODEL=...`) — highest priority
2. Explicit flags (`--profile=<name>` or `--tier=<name>`)
3. Skill-specific config override (`models.skill_overrides.council` in `.agentops/config.yaml`)
4. Global default tier (`models.default_tier` in `.agentops/config.yaml`)
5. Built-in default — `balanced`

The `--tier` flag is an alias for `--profile` using cost tier naming. Both are accepted.

### Configuration

Set tiers in `.agentops/config.yaml`:

```yaml
models:
  default_tier: balanced
  skill_overrides:
    council: quality    # council always uses opus
    vibe: balanced      # vibe uses sonnet
```

Or via environment variables:

```bash
export AGENTOPS_MODEL_TIER=budget           # global default
export AGENTOPS_COUNCIL_MODEL_TIER=quality  # council-specific override
```

## Precedence

Profiles are a convenience shortcut. Explicit flags and env vars always override:

1. Explicit env var (`COUNCIL_CLAUDE_MODEL=...`) --- highest priority
2. Explicit flags (`--count=N`, `--deep`, `--mixed`) --- override profile settings
3. `--profile=<name>` --- sets defaults
4. Built-in defaults --- lowest priority

When `--profile=thorough` is set but `--count=4` is also provided, the count comes from `--count` (4 judges), while the model comes from the profile (opus).

## Report Header

When a profile is used, include in the council report header:
```
**Profile:** <name>
```

## Env Var Mapping

Each profile sets these env vars before agent spawning:

```
thorough:
  COUNCIL_CLAUDE_MODEL=opus
  COUNCIL_JUDGE_COUNT=3
  COUNCIL_TIMEOUT=120

balanced:
  COUNCIL_CLAUDE_MODEL=sonnet
  COUNCIL_JUDGE_COUNT=2
  COUNCIL_TIMEOUT=120

fast:
  COUNCIL_CLAUDE_MODEL=haiku
  COUNCIL_JUDGE_COUNT=2
  COUNCIL_TIMEOUT=60
```
