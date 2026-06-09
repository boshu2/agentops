# Example Extractions

These examples show the end-to-end extraction flow: collect -> diff -> abstract -> package.

## Example 1: Retry Logic (Library)

Source projects:
- project_a/src/retry.rs
- project_b/lib/network/retry.rs
- project_c/pkg/http/retry.go

Invariant core:
- Max attempts
- Delay strategy
- Error propagation

Variance:
- Error types
- Time units
- Backoff factor defaults

Packaged as:
- Rust crate `retry-core`
- Public API: `retry(config, op)`

Test plan:
- Unit tests for success, retry exhaustion, jitter edge cases
- Apply to each source project (replace local copies)

---

## Example 2: CLI Robot Mode (Skill)

Source projects:
- cass, bv, ubs, br

Invariant core:
- `--json` or `--robot` for deterministic output
- No interactive prompts in robot mode
- Stable schema and exit codes

Variance:
- Output fields per command
- Which commands support robot mode

Packaged as:
- Skill: "cli-robot-mode-pattern"
- Sections: trigger conditions, exact prompt, example schemas

---

## Example 3: Install Script Pattern (Template)

Source projects:
- mcp_agent_mail, br, bv

Invariant core:
- Detect platform
- Verify checksum
- Idempotent install

Variance:
- Binary names and URLs
- Install paths

Packaged as:
- Template with variables for binary URL and checksum
