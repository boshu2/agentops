> Extracted from council/SKILL.md on 2026-04-11.

# Consensus Rules, Named Perspectives & Finding Extraction

## Named Perspectives

Named perspectives assign each judge a specific viewpoint. Pass `--perspectives="a,b,c"` for free-form names, or `--perspectives-file=<path>` for YAML with focus descriptions:

```bash
/council --perspectives="security-auditor,performance-critic,simplicity-advocate" validate src/auth/
/council --perspectives-file=.agents/perspectives/api-review.yaml validate src/api/
```

**YAML format** for `--perspectives-file`:

```yaml
perspectives:
  - name: security-auditor
    focus: Find security vulnerabilities and trust boundary violations
  - name: performance-critic
    focus: Identify performance bottlenecks and scaling risks
```

**Flag priority:** `--perspectives`/`--perspectives-file` override `--preset` perspectives. `--count` always overrides judge count. Without `--count`, judge count auto-escalates to match perspective count.

See [personas.md](personas.md) for all built-in presets and their perspective definitions.

## Consensus Rules

| Condition | Verdict |
|-----------|---------|
| All PASS | PASS |
| Any FAIL | FAIL |
| Mixed PASS/WARN | WARN |
| All WARN | WARN |

Disagreement handling:
- If Claude says PASS and Codex says FAIL → DISAGREE (surface both)
- Severity-weighted: Security FAIL outweighs style WARN

**DISAGREE resolution:** When vendors disagree, the spawner presents both positions with reasoning and defers to the user. No automatic tie-breaking — cross-vendor disagreement is a signal worth human attention.

### Mixed-mode pairing

When `--mixed` is set, perspectives are paired across vendors (one Claude judge + one Codex judge per perspective; see SKILL.md "Mixed-mode perspective assignment" and `cli-spawning.md` "Mixed-Mode Perspective Distribution"). DISAGREE resolution operates **per-perspective**:

- Pair each perspective's Claude verdict with its Codex verdict.
- The overall verdict still aggregates per-perspective verdicts using the existing PASS/WARN/FAIL combination table above.
- A perspective whose Claude judge says PASS and Codex judge says FAIL is reported as DISAGREE for that perspective — the per-perspective DISAGREE is itself the high-signal finding (vendor disagreement on identical input). The any-FAIL rule still applies, so that perspective's contribution to overall verdict is FAIL.
- Per-perspective consensus (both vendors agree on PASS or both agree on FAIL) is the strongest signal in the report; surface it prominently in the consolidation.

## Finding Extraction (Flywheel Closure)

After writing the council report, extract significant findings for the knowledge flywheel:

1. **Skip if PASS.** Nothing to extract from successful reviews.
2. **Filter findings:** Keep only severity >= `significant` AND confidence >= `MEDIUM`.
3. **Classify each:** `learning` (process gap), `finding` (code/design defect), or `rule` (repeatable constraint).
4. **Compute dedup key:** `sha256(finding_description)`. Skip if already in the file.
5. **Append** one JSON line per finding to `.agents/council/extraction-candidates.jsonl`.

Candidates are staged for human review or `/post-mortem` consumption — they are **never** auto-promoted to MEMORY.md.

See [finding-extraction.md](finding-extraction.md) for the full schema and classification heuristics.

All reports write to `.agents/council/YYYY-MM-DD-<type>-<target>.md`.
