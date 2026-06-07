# CM Cognitive Architecture

## Table of Contents
- [Three-Layer Model](#three-layer-model)
- [ACE Pipeline](#ace-pipeline)
- [Confidence Decay System](#confidence-decay-system)
- [Anti-Pattern Learning](#anti-pattern-learning)
- [Data Locations](#data-locations)
- [Configuration](#configuration)

---

## Three-Layer Model

```
┌─────────────────────────────────────────────────────────────────────┐
│                    EPISODIC MEMORY (cass)                           │
│   Raw session logs from all agents — the "ground truth"             │
│   Claude Code │ Codex │ Cursor │ Aider │ PI │ Gemini │ ChatGPT │ ...│
└───────────────────────────┬─────────────────────────────────────────┘
                            │ cass search
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    WORKING MEMORY (Diary)                           │
│   Structured session summaries: accomplishments, decisions, etc.    │
└───────────────────────────┬─────────────────────────────────────────┘
                            │ reflect + curate (automated)
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    PROCEDURAL MEMORY (Playbook)                     │
│   Distilled rules with confidence tracking and decay                │
└─────────────────────────────────────────────────────────────────────┘
```

Every agent's sessions feed the shared memory. A pattern discovered in Cursor **automatically** helps Claude Code on the next session.

---

## ACE Pipeline

How Rules Are Created:

```
Generator → Reflector → Validator → Curator
```

| Stage | Role | LLM? |
|-------|------|------|
| **Generator** | Pre-task context hydration (`cm context`) | No |
| **Reflector** | Extract patterns from sessions (`cm reflect`) | Yes |
| **Validator** | Evidence gate against cass history | Yes |
| **Curator** | Deterministic delta merge | **No** |

**Critical:** Curator has NO LLM to prevent context collapse from iterative drift. LLMs propose patterns; deterministic logic manages them.

### Scientific Validation

Before a rule joins your playbook, it's validated against cass history:

```
Proposed rule: "Always check token expiry before auth debugging"
    ↓
Evidence gate: Search cass for sessions where this applied
    ↓
Result: 5 sessions found, 4 successful outcomes → ACCEPT
```

Rules without historical evidence are flagged as candidates until proven.

---

## Confidence Decay System

Rules aren't immortal. Confidence decays without revalidation.

### Decay Mechanism

| Mechanism | Effect |
|-----------|--------|
| **90-day half-life** | Confidence halves every 90 days without feedback |
| **4x harmful multiplier** | One mistake counts 4× as much as one success |
| **Maturity progression** | `candidate` → `established` → `proven` |

### Score Decay Visualization

```
Initial score: 10.0 (10 helpful marks today)

After 90 days (half-life):   5.0
After 180 days:              2.5
After 270 days:              1.25
After 365 days:              0.78
```

### Effective Score Formula

```typescript
effectiveScore = decayedHelpful - (4 × decayedHarmful)

// Where decay factor = 0.5 ^ (daysSinceFeedback / 90)
```

### Maturity State Machine

```
  ┌──────────┐       ┌─────────────┐    ┌────────┐
  │ candidate│──────▶│ established │───▶│ proven │
  └──────────┘       └─────────────┘    └────────┘
       │                   │                  │
       │                   │ (harmful >25%)   │
       │                   ▼                  │
       │             ┌─────────────┐          │
       └────────────▶│ deprecated  │◀─────────┘
                     └─────────────┘
```

**Transition Rules:**

| Transition | Criteria |
|------------|----------|
| `candidate` → `established` | 3+ helpful, harmful ratio <25% |
| `established` → `proven` | 10+ helpful, harmful ratio <10% |
| `any` → `deprecated` | Harmful ratio >25% OR explicit deprecation |

---

## Anti-Pattern Learning

Bad rules don't just get deleted. They become warnings:

```
"Cache auth tokens for performance"
    ↓ (3 harmful marks)
"PITFALL: Don't cache auth tokens without expiry validation"
```

When a rule is marked harmful multiple times (>50% harmful ratio with 3+ marks), it's automatically inverted into an anti-pattern.

---

## Data Locations

```
~/.cass-memory/                  # Global (user-level)
├── config.json                  # Configuration
├── playbook.yaml                # Personal playbook
├── diary/                       # Session summaries
├── outcomes/                    # Session outcomes
├── traumas.jsonl                # Trauma patterns
├── starters/                    # Custom starter playbooks
├── onboarding-state.json        # Onboarding progress
├── privacy-audit.jsonl          # Cross-agent audit trail
├── processed-sessions.jsonl     # Reflection progress
└── usage.jsonl                  # LLM cost tracking

.cass/                           # Project-level (in repo)
├── config.json                  # Project-specific overrides
├── playbook.yaml                # Project-specific rules
├── traumas.jsonl                # Project-specific patterns
└── blocked.yaml                 # Anti-patterns to block
```

---

## Configuration

Config lives at `~/.cass-memory/config.json` (global) and `.cass/config.json` (repo).

**Precedence:** CLI flags > Repo config > Global config > Defaults

**Security:** Repo config cannot override sensitive paths or user-level consent settings.

### Key Options

```json
{
  "provider": "anthropic",
  "model": "claude-sonnet-4-6",
  "budget": {
    "dailyLimit": 0.10,
    "monthlyLimit": 2.00
  },
  "scoring": {
    "decayHalfLifeDays": 90,
    "harmfulMultiplier": 4
  },
  "maxBulletsInContext": 50,
  "maxHistoryInContext": 10,
  "sessionLookbackDays": 7,
  "crossAgent": {
    "enabled": false,
    "consentGiven": false,
    "auditLog": true
  },
  "semanticSearchEnabled": false,
  "dedupSimilarityThreshold": 0.85
}
```

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | API key for Anthropic (Claude) |
| `OPENAI_API_KEY` | API key for OpenAI |
| `GOOGLE_GENERATIVE_AI_API_KEY` | API key for Google Gemini |
| `CASS_PATH` | Path to cass binary |
| `CASS_MEMORY_LLM` | Set to `none` for LLM-free mode |
| `MCP_HTTP_TOKEN` | Auth token for non-loopback MCP server |

---

## Gap Analysis Categories

| Category | Keywords |
|----------|----------|
| `debugging` | error, fix, bug, trace, stack |
| `testing` | test, mock, assert, expect, jest |
| `architecture` | design, pattern, module, abstraction |
| `workflow` | task, CI/CD, deployment |
| `documentation` | comment, README, API doc |
| `integration` | API, HTTP, JSON, endpoint |
| `collaboration` | review, PR, team |
| `git` | branch, merge, commit |
| `security` | auth, token, encrypt, permission |
| `performance` | optimize, cache, profile |

**Category Status Thresholds:**

| Status | Rule Count |
|--------|------------|
| `critical` | 0 rules |
| `underrepresented` | 1-2 rules |
| `adequate` | 3-10 rules |
| `well-covered` | 11+ rules |
