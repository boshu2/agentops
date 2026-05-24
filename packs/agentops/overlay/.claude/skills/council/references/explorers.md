# Explorer Sub-Agents

Judges can spawn explorer sub-agents for parallel deep-dive research. This is the key differentiator for `research` mode -- massive parallel exploration.

## Flag

| Flag | Default | Max | Description |
|------|---------|-----|-------------|
| `--explorers=N` | 0 | 5 | Number of explorer sub-agents per judge |

## Architecture

```
+------------------------------------------------------------------+
|  Judge (independent or with perspective)                          |
|                                                                   |
|  1. Receive packet + perspective                                  |
|  2. Identify N sub-questions to explore                           |
|  3. Spawn N explorers in parallel (Task tool, background)         |
|  4. Collect explorer results                                      |
|  5. Synthesize into final judge response                          |
+------------------------------------------------------------------+
        |              |              |
        v              v              v
  +----------+  +----------+  +----------+
  |Explorer 1|  |Explorer 2|  |Explorer 3|
  |Sub-Q: "A"|  |Sub-Q: "B"|  |Sub-Q: "C"|
  |          |  |          |  |          |
  |Codebase  |  |Codebase  |  |Codebase  |
  |search +  |  |search +  |  |search +  |
  |analysis  |  |analysis  |  |analysis  |
  +----------+  +----------+  +----------+
```

**Total agents:** `judges * (1 + explorers)`

**MAX_AGENTS = 12** (hard limit). If total agents (judges x (1 + explorers)) exceeds 12, exit with error: "Error: Total agent count {N} exceeds MAX_AGENTS (12). Reduce --explorers or remove --mixed."

| Example | Judges | Explorers | Total Agents | Status |
|---------|--------|-----------|--------------|--------|
| `/council research X` | 2 | 0 | 2 | Valid |
| `/council --explorers=3 research X` | 2 | 3 | 8 | Valid |
| `/council --deep --explorers=3 research X` | 3 | 3 | 12 | Valid (at cap) |
| `/council --mixed --explorers=3 research X` | 6 | 3 | 24 | BLOCKED (exceeds 12) |
| `/council --mixed research X` | 6 | 0 | 6 | Valid |
| `/council --mixed --explorers=1 research X` | 6 | 1 | 12 | Valid (at cap) |

## Explorer Prompt

```
You are Explorer {M} for Council Judge {N}{PERSPECTIVE_SUFFIX}.
(PERSPECTIVE_SUFFIX is " -- THE {PERSPECTIVE}" when using presets, or empty for independent judges)

## Your Sub-Question

{SUB_QUESTION}

## Context

Working directory: {CWD}
Target: {TARGET}

## Instructions

1. Use available tools (Glob, Grep, Read, Bash) to investigate the sub-question
2. Search the codebase, documentation, and any relevant sources
3. Be thorough -- your findings feed directly into the judge's analysis
4. Return a structured summary:

### Findings
<what you discovered>

### Evidence
<specific files, lines, patterns found>

### Assessment
<your interpretation of the findings>
```

## Explorer Execution

Explorers are spawned as lightweight subagents optimized for search/read (not editing). Each explorer receives a sub-question and returns findings to its parent judge.

**Model selection:** Explorers use `sonnet` by default (fast, good at search). Judges also use `sonnet` by default (use `--profile=thorough` or `COUNCIL_CLAUDE_MODEL=opus` for high-stakes reviews). Override explorer model with `--explorer-model=<model>`.

## Sub-Question Generation

When `--explorers=N` is set, the judge prompt includes:

```
Before analyzing, identify {N} specific sub-questions that would help you
answer thoroughly. For each sub-question, spawn an explorer agent to
investigate it. Use the explorer findings to inform your final analysis.

Sub-questions should be:
- Specific and searchable (not vague)
- Complementary (cover different aspects)
- Relevant to your analysis angle (perspective if assigned, or general if independent)
```

## Timeout

Explorer timeout: 60s (half of judge timeout). Judge timeout starts after all explorers complete.
