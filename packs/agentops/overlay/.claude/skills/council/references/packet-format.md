> Extracted from council/SKILL.md on 2026-04-11.

# Council Packet Format (JSON)

The packet sent to each agent. **File contents are included inline** — agents receive the actual code/plan text in the packet, not just paths. This ensures both Claude and Codex agents can analyze without needing file access.

If `.agents/ao/environment.json` exists, include it in the context packet so judges can reason about available tools and environment state.

Judge prompt boundary:
- Do NOT include `.agents/` references in judge prompts.
- Do NOT instruct judges to search `.agents/` directories. Judges operate on the council packet only.

```json
{
  "council_packet": {
    "version": "1.0",
    "mode": "validate | brainstorm | research",
    "target": "Implementation of user authentication system",
    "context": {
      "files": [
        {
          "path": "src/auth/jwt.py",
          "content": "<file contents inlined here>"
        },
        {
          "path": "src/auth/middleware.py",
          "content": "<file contents inlined here>"
        }
      ],
      "diff": "git diff output if applicable",
      "spec": {
        "source": "bead na-0042 | plan doc | none",
        "content": "The spec/bead description text (optional — included when wrapper provides it)"
      },
      "prior_decisions": [
        "Using JWT, not sessions",
        "Refresh tokens required"
      ],
      "empirical_results": "(optional) test output, CLI flag verification, or Wave 0 findings — include when evaluating feasibility"
    },
    "perspective": "skeptic (only when --preset or --perspectives used)",
    "perspective_description": "What could go wrong? (only when --preset or --perspectives used)",
    "output_schema": {
      "verdict": "PASS | WARN | FAIL",
      "confidence": "HIGH | MEDIUM | LOW",
      "key_insight": "Single sentence summary",
      "findings": [
        {
          "severity": "critical | significant | minor",
          "category": "security | architecture | performance | style",
          "id": "(optional) Stable finding ID for cross-skill correlation (e.g., f-council-001)",
          "description": "What was found",
          "location": "file:line if applicable",
          "recommendation": "How to address",
          "fix": "Specific action to resolve this finding",
          "why": "Root cause or rationale",
          "ref": "File path, spec anchor, or doc reference"
        }
      ],
      "recommendation": "Concrete next step",
      "schema_version": 3
    }
  }
}
```

## Empirical Evidence Rule

When evaluating **implementation feasibility** (e.g., "will this CLI flag work?", "can these tools coexist?"), always include empirical test results in `context.empirical_results`. Judges reasoning from assumptions produce false verdicts — a Codex judge once gave a false FAIL on `-s read-only` because Wave 0 test output was not in the packet. The rule: **run the experiment first, then let judges evaluate the evidence.**

Wrapper skills (`/vibe`, `/pre-mortem`) should include relevant test output when the council target involves tooling behavior, flag combinations, or runtime compatibility.
