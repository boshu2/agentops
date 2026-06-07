# Independent Verification Deep Reference

> **The Rule:** User reports are hints, not facts. Every claim must be verified. Never trust—always verify.

---

## Contents

| Section | Jump |
|---------|------|
| Philosophy | [→](#the-philosophy) |
| Verification by Claim Type | [→](#verification-by-claim-type) |
| Code Investigation | [→](#code-investigation) |
| Testing Empirically | [→](#testing-empirically) |
| Checking Commit History | [→](#checking-commit-history) |
| Documentation Verification | [→](#documentation-verification) |
| PR Analysis | [→](#pr-analysis-the-intel) |
| Red Flags | [→](#red-flags-in-user-reports) |

---

## The Philosophy

**Why independent verification matters:**

1. **Users misdiagnose** — They report symptoms, not causes
2. **Suggested fixes may be wrong** — They don't know the codebase
3. **Issues may be stale** — Already fixed in recent commits
4. **Environment-specific** — May not reproduce on your system
5. **Intended behavior** — "Bug" may actually be by design

**The mindset:**
- User reports = hints pointing you toward investigation
- Your job = understand the actual problem, implement proper fix
- Never copy user code, never merge PRs, always verify

---

## Verification by Claim Type

### "X crashes"

```bash
# 1. Get exact reproduction steps
gh issue view NUMBER -R owner/repo --json body -q '.body'

# 2. Set up identical environment if possible
# (Note OS, version, config from issue)

# 3. Attempt reproduction
cd /data/projects/REPO
# Run the exact command they claim crashes

# 4. If reproduced: find root cause
# Read stack trace, find failing code path

# 5. If NOT reproduced: ask for more details
gh issue comment NUMBER -R owner/repo -b "Could not reproduce. Please provide:
- Exact command run
- Full error output (copy-paste, not screenshot)
- OS and version
- Tool version"
```

### "X doesn't work"

```bash
# 1. Define "work" — what are they expecting?
gh issue view NUMBER -R owner/repo

# 2. Read relevant code
grep -r "feature_name" src/
cat src/relevant_file.rs

# 3. Test actual behavior
./target/debug/tool [relevant command]

# 4. Compare to documented behavior
# Check README, --help, man pages

# 5. Determine: bug or user error?
```

### "X should do Y"

```bash
# 1. Check if Y is actually intended behavior
cat README.md | grep -i "feature"
./target/debug/tool --help

# 2. Check issue history — was this decided before?
gh issue list -R owner/repo --state closed --search "feature Y"

# 3. If Y was rejected before: close as duplicate
# If Y was never discussed: evaluate on merits
```

### "X is slow"

```bash
# 1. Get baseline numbers
# What's "slow" to them? What's "fast"?

# 2. Benchmark yourself
time ./target/debug/tool [command]

# 3. Profile if genuinely slow
# Rust: cargo flamegraph
# Go: go tool pprof
# Python: cProfile

# 4. Compare to expectations
# Is this actually slow or expected for data size?
```

### "X has security issue"

```bash
# 1. Understand the claimed vulnerability
gh issue view NUMBER -R owner/repo

# 2. Find the relevant code path
grep -r "vulnerable_function" src/

# 3. Trace data flow
# Can untrusted input reach the dangerous operation?

# 4. Attempt exploit (safely)
# Only in isolated environment

# 5. Assess severity
# - Exploitability: Low/Medium/High
# - Impact: Low/Medium/High/Critical

# 6. If real: fix immediately, consider disclosure
# If false positive: explain why, close
```

---

## Code Investigation

### Finding Relevant Code

```bash
# Search for function/feature name
grep -r "function_name" src/
grep -rn "FeatureName" src/  # With line numbers

# Find type/struct definitions
grep -r "struct FeatureName" src/
grep -r "type FeatureName" src/

# Find usages
grep -r "feature_name(" src/

# Read the implementation
cat src/path/to/file.rs
```

### Understanding Code Flow

```bash
# Find entry points
grep -r "fn main" src/
grep -r "pub fn " src/lib.rs

# Find error handling
grep -r "Error" src/ | head -20
grep -r "panic!" src/

# Find where feature is implemented
grep -r "feature" src/ | grep -v test | grep -v "\.md"
```

### Checking Dependencies

```bash
# Rust
cat Cargo.toml | grep -A20 "\[dependencies\]"
cargo tree | head -50

# Go
cat go.mod
go mod graph | head -50

# Python
cat requirements.txt
cat pyproject.toml

# Node
cat package.json | jq '.dependencies'
```

---

## Testing Empirically

### Build and Run

```bash
# Rust
cargo build
./target/debug/tool --help
./target/debug/tool [command from issue]

# Go
go build
./tool --help
./tool [command from issue]

# Python
python -m tool --help
python -m tool [command from issue]
```

### Run Tests

```bash
# Rust
cargo test
cargo test -- --nocapture  # See output

# Go
go test ./...

# Python
pytest
pytest -v  # Verbose

# Node
npm test
```

### Check CI Status

```bash
# See recent CI runs
gh run list -R owner/repo --limit 5

# If CI failing, investigate
gh run view RUN_ID --log-failed
```

---

## Checking Commit History

### See If Already Fixed

```bash
# Commits since issue was filed
git log --oneline --since="ISSUE_DATE" | head -20

# Search for related fixes
git log --oneline --grep="fix" | head -20
git log --oneline --grep="bug" | head -20

# Search for file changes
git log --oneline -- src/relevant_file.rs | head -10
```

### Find When Bug Was Introduced

```bash
# git bisect for regressions
git bisect start
git bisect bad HEAD
git bisect good v1.0.0
# Test at each step, mark good/bad
```

### Check Release Notes

```bash
# See recent tags/releases
git tag --sort=-creatordate | head -10

# View release notes
gh release view v1.2.3 -R owner/repo
```

---

## Documentation Verification

### Check Official Docs

```bash
# README
cat README.md

# Help text
./tool --help
./tool subcommand --help

# Man pages (if any)
man tool
```

### Check Upstream Docs (for dependencies)

```bash
# If claim involves external API/library:
# - Check official documentation
# - Verify behavior matches docs
# - If discrepancy: is it our bug or theirs?
```

---

## PR Analysis (THE INTEL)

PRs contain valuable intel. Never merge, but always analyze.

### Reading the Diff

```bash
# Get the diff (THE INTEL)
gh pr diff NUMBER -R owner/repo

# What files changed?
gh pr view NUMBER -R owner/repo --json files -q '.files[].path'

# How many lines?
gh pr view NUMBER -R owner/repo --json additions,deletions
```

### Understanding Their Approach

Questions to answer:
1. What problem are they solving?
2. What's their approach?
3. Is this approach sound?
4. What would I do differently?

### Decision Matrix for PR Ideas

| Their Approach | Quality | Action |
|----------------|---------|--------|
| Good idea, good code | High | Understand, reimplement yourself |
| Good idea, bad code | Medium | Take idea, implement properly |
| Bad idea, any code | Low | Note why it's wrong, decline |
| Already done differently | N/A | Close, point to existing solution |

---

## Red Flags in User Reports

| Red Flag | What It Often Means |
|----------|---------------------|
| "Works on my machine" | Environment-specific, may not reproduce |
| "Just add this flag" | Doesn't understand implications |
| "Simple fix" | Usually not simple |
| "Just copy what X does" | Massive scope creep |
| No reproduction steps | May not be reproducible |
| Screenshot of error | Likely missing important context |
| "Latest version" | Need actual version number |
| "I tried everything" | Probably didn't |

---

## Automated Integrity Verification via dsr

Some issues are symptoms of automated drift rather than code bugs. Before investigating manually, check automated integrity first.

### ACFS Manifest Drift (Most Common)

The #1 recurring issue across all repos. The `curl | bash` installer breaks when `manifest_index.sh` has a stale SHA256.

```bash
# Quick check (exit 0 = ok, exit 1 = drift)
/data/projects/agentic_coding_flywheel_setup/scripts/check-manifest-drift.sh --json

# Auto-fix if drifted
/data/projects/agentic_coding_flywheel_setup/scripts/check-manifest-drift.sh --fix

# Check if systemd timer is running
systemctl --user status acfs-manifest-drift.timer
journalctl --user -u acfs-manifest-drift.service --since "24h ago"

# dsr quality check (validates all registered checks for ACFS)
dsr quality --tool agentic_coding_flywheel_setup
```

**Root cause pattern:** Any agent or human editing `acfs.manifest.yaml` without running `cd packages/manifest && bun run generate` afterwards causes drift. The systemd timer catches and fixes this automatically every 2 hours.

**If an issue reports "install fails" or "bootstrap mismatch":**
1. Run the drift check first
2. If drift was present AND the timer already fixed it, close with: "Fixed automatically by manifest integrity monitor"
3. If timer isn't running, fix manually and investigate why the timer stopped

### General dsr Quality Gates

```bash
# Run all quality checks for any registered tool
dsr quality --tool <tool_name>

# List registered tools and their checks
cat ~/.config/dsr/repos.yaml
```

---

## Verification Checklist

Before taking action on any issue:

- [ ] Read the full issue including comments
- [ ] **Checked automated integrity** (manifest drift, dsr quality gates)
- [ ] Searched commits since issue date
- [ ] Found relevant code in codebase
- [ ] Understood intended behavior (docs, --help)
- [ ] Attempted reproduction (if bug)
- [ ] Verified fix works (if implementing)
- [ ] Did NOT copy user's code
- [ ] Did NOT trust suggested fix blindly
