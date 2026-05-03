> Extracted from council/SKILL.md on 2026-04-11.

# Council Examples (Extended)

```bash
/council validate recent                                        # 2 judges, recent commits
/council --deep --preset=architecture research the auth system  # 3 judges with architecture personas
/council --mixed validate this plan                             # 3 Claude + 3 Codex
/council --deep --explorers=3 research upgrade patterns         # 12 agents (3 judges x 4)
/council --preset=security-audit --deep validate the API        # attacker, defender, compliance, web-security
/council --preset=doc-review validate README.md                  # 4 doc judges with named perspectives
/council --deep --mixed --preset=leadership-quartet validate PRODUCT.md vs shipped app # 4 runtime-native + 4 Codex
/council brainstorm caching strategies for the API              # 2 judges explore options
/council --technique=scamper brainstorm API improvements               # structured SCAMPER brainstorm
/council --technique=six-hats brainstorm migration strategy            # parallel perspectives brainstorm
/council --profile=thorough validate the security architecture       # opus, 3 judges, 120s timeout
/council --profile=fast validate recent                               # haiku, 2 judges, 60s timeout
/council research Redis vs Memcached for session storage        # 2 judges assess trade-offs
/council validate the implementation plan in PLAN.md            # structured plan feedback
/council --preset=doc-review validate docs/ARCHITECTURE.md             # 4 doc review judges
/council --perspectives="security-auditor,perf-critic" validate src/   # named perspectives
/council --perspectives-file=.agents/perspectives/custom.yaml validate # perspectives from file
```

## Fast Single-Agent Validation

**User says:** `/council --quick validate recent`

**What happens:**
1. Agent gathers context (recent diffs, files) inline without spawning
2. Agent performs structured self-review using council output schema
3. Report written to `.agents/council/YYYY-MM-DD-quick-<target>.md` labeled `Mode: quick (single-agent)`

**Result:** Fast sanity check for routine validation (no cross-perspective insights or debate).

## Adversarial Debate Review

**User says:** `/council --debate validate the auth system`

**What happens:**
1. Agent spawns 2 judges (runtime-native backend) with independent perspectives
2. R1: Judges assess independently, write verdicts to `.agents/council/`
3. R2: Team lead sends other judges' verdicts via backend messaging
4. Judges revise positions based on cross-perspective evidence
5. Consolidation: Team lead computes consensus with convergence detection

**Result:** Two-round review with steel-manning and revision, useful for high-stakes decisions.

## Cross-Vendor Consensus with Explorers

**User says:** `/council --mixed --explorers=2 research Kubernetes upgrade strategies`

**What happens:**
1. Agent spawns 3 Claude judges + 3 Codex judges (6 total)
2. Each judge spawns 2 explorer sub-agents (6 x 3 = 18 total agents, exceeds MAX_AGENTS)
3. Agent auto-scales to 2 judges per vendor (4 x 3 = 12 agents at limit)
4. Explorers perform parallel deep-dives, return sub-findings to judges
5. Judges consolidate explorer findings with own research

**Result:** Cross-vendor research with deep exploration, capped at 12 total agents.
