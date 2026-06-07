# Real-World Examples

Actual examples of issue/PR triage from past sessions. These illustrate the verification protocol and response patterns in practice.

---

## Example 1: PR Closed with Policy Explanation

**Source:** `beads_rust#16` — PR to change file permissions

### The Situation
User submitted a PR to change file permissions from 0o600 to 0o644 for multi-user access.

### The Analysis
```bash
# View the diff (THE INTEL)
gh pr diff 16 -R joelewis/beads_rust

# What they changed:
# - File permission constants from 0o600 to 0o644
# - Affects JSONL sync files
```

### The Decision
The idea has merit—more permissive file permissions help in multi-user scenarios. But per policy, we never merge PRs directly.

### The Action
```bash
gh pr close 16 -R joelewis/beads_rust -c "$(cat <<'EOF'
Thanks for the contribution! Per project policy, I don't merge outside PRs, but I've reviewed your approach.

The idea of making files more accessible in multi-user scenarios makes sense. I'll consider implementing a similar change independently, potentially with a configuration option.

Appreciate you taking the time!
EOF
)"
```

**Key Takeaway:** Mine the idea, implement yourself, close with kind explanation.

---

## Example 2: Bug Report with Follow-up Questions

**Source:** `beads_rust#18` — `--db` flag behavior

### The Situation
User reported that `--db` flag doesn't work from outside a beads workspace, unlike the older `bd` tool.

### The Verification
```bash
# Check current behavior
cd /tmp
br list --db /path/to/beads.db  # Does it work?

# Compare to bd behavior
bd list --db /path/to/beads.db  # bd works, br doesn't

# Find relevant code
grep -r "db.*flag\|database.*path" src/
```

### The Analysis
The user's report appears valid—there's a behavioral difference between `bd` and `br`. This could be:
1. Intentional design change
2. Unintentional regression
3. Missing feature in the port

### The Action
```bash
gh issue comment 18 -R joelewis/beads_rust -b "$(cat <<'EOF'
Thanks for the detailed report. I've verified that there is indeed a behavioral difference between bd and br regarding the --db flag.

A few questions to understand your use case better:
1. Are you trying to query a beads database from a different project?
2. What's your typical workflow when using --db?

This will help determine whether to replicate bd's exact behavior or design something better suited to br's architecture.
EOF
)"
```

**Key Takeaway:** Verified the claim, asked clarifying questions before implementing.

---

## Example 3: Feature Request Noted

**Source:** `beads_rust#17` — `--set-labels` multiple flags

### The Situation
User requested that `br update --set-labels` accept multiple flags like `bd` does.

### The Verification
```bash
# Check current br behavior
br update --help | grep -i label

# Check bd behavior for comparison
bd update --help | grep -i label

# Compare syntaxes:
# bd: bd update --set-labels foo --set-labels bar
# br: br update --set-labels "foo,bar"
```

### The Analysis
This is a syntax preference issue. The comma-separated approach in `br` is actually simpler, but the user wants parity with `bd`.

### The Action
```bash
gh issue comment 17 -R joelewis/beads_rust -b "$(cat <<'EOF'
Thanks for the feedback! Currently br uses a comma-separated syntax:

```bash
br update --set-labels "foo,bar"
```

Whereas bd used repeated flags:

```bash
bd update --set-labels foo --set-labels bar
```

Both approaches work—it's mostly a syntax preference. I'm noting this for consideration but currently leaning toward keeping the comma syntax since it's more concise.

Is there a specific workflow where the repeated flag approach works better for you?
EOF
)"
```

**Key Takeaway:** Explained the design decision, gathered more input before deciding.

---

## Example 4: Similar Project Notification

**Source:** `xf#4` — Similar project notification

### The Situation
User created an issue just to notify about a similar project called "tweethoarder".

### The Verification
```bash
# View the issue
gh issue view 4 -R joelewis/xf

# It's informational, not a bug or feature request
```

### The Analysis
This is just a heads-up about a similar project—no action required on the codebase. Acknowledge and close.

### The Action
```bash
gh issue close 4 -R joelewis/xf -c "$(cat <<'EOF'
Thanks for the heads up! Always interesting to see similar projects in the space.

xf takes a different approach focused on [specific differentiator], but it's useful to know what else is out there.

Closing as informational, but appreciate you sharing!
EOF
)"
```

**Key Takeaway:** Not every issue requires code changes. Acknowledge and close informational items.

---

## Example 5: Feature Request Requiring Judgment (SURFACED)

**Source:** `mcp_agent_mail#63` — Factory Droid support

### The Situation
User requested support for a new AI agent platform.

### The Analysis
Adding support for a new platform is scope expansion. Need to evaluate:
- How much work?
- Does it align with project direction?
- Maintenance burden?

### The Surface Format
```
FEATURE REQUEST REQUIRING JUDGMENT: joelewis/mcp_agent_mail#63

**Request:** "Lacks support for Factory Droid"

**User's Problem:** Wants to use mcp_agent_mail with Factory Droid agents

**Analysis:**
+ Would expand user base
+ Factory Droid is gaining traction
- Unknown API compatibility
- Maintenance burden of supporting multiple platforms
- Scope Risk: Opens door to supporting every new agent platform

**Implementation Options:**
1. Full implementation: Add Factory Droid adapter (~200 LOC)
2. Simplified version: Document how users can adapt themselves
3. Decline: Focus on core supported platforms

**My recommendation:** Option 2—document the adaptation path without adding formal support

**If approved:** I'll implement and close with guidance on adaptation.
```

**Key Takeaway:** Complex scope decisions get surfaced to user for judgment.

---

## Example 6: Binary Distribution Request

**Source:** `cass#39` — Binary artifact releases

### The Situation
User requested published binary artifacts for latest releases.

### The Verification
```bash
# Check current release process
gh release list -R joelewis/cass --limit 5

# Check if there are binaries
gh release view v0.1.0 -R joelewis/cass
```

### The Analysis
This is a reasonable request. Options:
1. Add GitHub Actions to build and attach binaries
2. Document how to build from source
3. Use cargo-binstall support

### The Action
```bash
gh issue comment 39 -R joelewis/cass -b "$(cat <<'EOF'
Valid request! A few questions to understand the use case:

1. What platforms do you need? (Linux x86_64, macOS arm64, Windows?)
2. Are you okay with cargo-binstall as an alternative?
3. Any preference on binary format (single static binary vs. with dependencies)?

I'm considering adding a GitHub Actions workflow for releases but want to make sure it covers actual needs.
EOF
)"
```

**Key Takeaway:** Gather requirements before implementing distribution changes.

---

## Example 7: Install Script Bug

**Source:** `beads_rust#8` — Install script fails

### The Situation
User reported that install script fails with cache-busting URL parameter.

### The Verification
```bash
# Try the install script
curl -sL https://example.com/install.sh | sh

# Check the script source
curl -sL https://example.com/install.sh | head -50

# Find the cache-busting code
grep -r "cache\|bust\|random\|?" scripts/install.sh
```

### The Analysis
Found the issue—the script appends a random query parameter that some servers reject.

### The Action
```bash
# Fix the script
# ... make changes ...

git commit -m "fix: remove cache-busting from install script

The query parameter was being rejected by some CDNs.

Closes #8

Co-Authored-By: Claude <noreply@anthropic.com>"

gh issue close 8 -R joelewis/beads_rust -c "$(cat <<'EOF'
Fixed in abc123.

The cache-busting URL parameter was causing issues with some CDNs. Removed it—cache headers handle freshness properly anyway.

Thanks for the report!
EOF
)"
```

**Key Takeaway:** Verify the bug, fix it yourself, close with commit reference.

---

## Pattern Summary

| Scenario | Pattern |
|----------|---------|
| PR submitted | Mine for ideas → Implement yourself → Close with policy explanation |
| Bug report | Verify → Fix if real → Ask questions if unclear |
| Feature request (simple) | Implement → Close |
| Feature request (complex) | Surface to user for judgment |
| Informational | Acknowledge → Close |
| Unclear report | Ask specific questions → Wait for response |

---

## Response Quality Checklist

Before sending any response:

- [ ] Verified the claim independently
- [ ] Did NOT copy user's code
- [ ] Explained my finding/decision
- [ ] Was kind and appreciative
- [ ] Provided next steps if applicable
- [ ] Used HEREDOC for multi-line responses
