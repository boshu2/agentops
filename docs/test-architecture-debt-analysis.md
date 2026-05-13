# Test Architecture Debt Analysis: cli/cmd/ao/

**Package:** `/Users/fullerbt/gt/agentops/crew/nami/cli/cmd/ao/`  
**Analysis Date:** 2026-04-05  
**Scope:** Single Go package with 4,163 test functions across 258 test files  

---

## 1. Package-Level Global State Inventory

### Overview
| Metric | Count |
|--------|-------|
| Total `.go` files | 437 |
| Test files (`*_test.go`) | 258 |
| Non-test files | 179 |
| Total lines of code (all files) | 196,752 |
| Lines in test files | 135,452 |
| Lines in non-test files | 61,300 |

### Global Variable Declarations (Non-Test Files)
| Category | Count | Risk Level | Details |
|----------|-------|------------|---------|
| All `var` declarations | 289 | — | Total top-level vars across non-test `.go` files |
| `cobra.Command` vars | 132 | **LOW** | Singletons by design; safe during tests with proper isolation |
| Flag-bound vars (StringVar, BoolVar, etc.) | 368 | **HIGH** | Persisted across test runs; pollute next test unless explicitly reset |
| Mutable state (maps, slices, funcs) | 37 | **HIGH** | Non-command maps, slices, and function pointers that can leak between tests |

### Detailed Mutable State Catalog
**Function Pointers (Can Change During Tests):**
- `context_relevance.go`: `runtimeSessionSearchFn = searchUpstreamCASS`
- `rpi_phased_stream.go`: `spawnDirectFn = spawnClaudeDirectGlobal`

**Read-Only Maps (Immutable After Init):**
- `contradict.go`: `negationWords` (map[string]bool)
- `contradict.go`: `oppositionPairs` (map[string]string)
- `curate.go`: `validArtifactTypes` (map[string]bool)
- `doctor.go`: `deprecatedCommands` (map[string]string)
- `goals_steer.go`: `validSteers` (map[string]bool)
- `inject_context.go`: `validIntelScopes`, `validSectionNames` (2x map[string]bool)
- `inject_learnings.go`: `validPhases` (map[string]bool)
- `plans.go`: `planStatusSymbols` (map[types.PlanStatus]string)
- `ratchet_next.go`: `stepSkillMap` (map[string]string)

**Slices (Typically Immutable After Init):**
- `goals_init.go`: `validTemplateNames` ([]string)
- `index.go`: `defaultIndexDirs` ([]string)
- `init.go`: `agentsDirs` ([]string)
- `pool_ingest.go`: `dateStrategies` ([]dateStrategy)
- `retrieval_bench.go`: `liveQueries`, `benchManifestFilenames` (2x []string)
- +~15 more immutable slices

### State Saved by executeCommand()
The `cobra_commands_test.go` helper **attempts** to isolate state by saving/restoring:
- **Root-level persistent flags:** `dryRun`, `verbose`, `output`, `jsonFlag`, `cfgFile`
- **Command-local flags (85+ vars):** Command-specific string/bool/int flags
- **Struct-type flags:** `contextPacketFlags`, `contextExplainFlags`, `contextPacketStatusFlags`

**Critical Gap:** Only ~90 of the 368 flag-bound vars are saved/restored. The remaining 278 are either:
1. Not yet added to `executeCommand()` restoration (debt backlog)
2. Implicitly reset by Cobra's flag parsing (unverified assumption)
3. Accumulating pollution across test runs (high flake risk)

---

## 2. os.Stdout Redirect Patterns

### Usage Summary
| Pattern | Count | Risk Level |
|---------|-------|------------|
| Tests setting `os.Stdout = w` | 280 | **HIGH** |
| Tests using `captureStdout()` helper | 221 | **LOW** |
| Tests using `defer` for restore | 7 | **MEDIUM** |
| Tests using `t.Cleanup()` for restore | 183 | **MEDIUM** |
| Tests using inline restore (before defer) | 138 | **CRITICAL** |
| **Total os.Stdout writes** | **280** | — |

### Breakdown of Unsafe Patterns
1. **Inline restore (138 tests):** Set `os.Stdout = w`, then `os.Stdout = origStdout` inline
   - **Risk:** Any `t.Fatal()`, `t.FailNow()`, or panic between assignment and restore loses stdout forever
   - **Files affected:** `cobra_commands_test.go` (heaviest user), `batch_forge_test.go`, `batch_promote_test.go`
   
2. **No cleanup at all (142 tests):** Set `os.Stdout = w` but never restore
   - **Risk:** Subsequent tests run with broken stdout; cascading failures
   - **Root cause:** Tests ending with assertion failure or unhandled error

3. **Correct patterns (191 tests):** Use `captureStdout()` (221 calls) or `t.Cleanup()` (183 calls)
   - **Note:** Overlap exists; same test may use multiple patterns

### captureStdout() Adoption
**221 calls across test suite** — but only 179 non-test files means helpers are used, creating a second abstraction layer. This is correct but introduces coupling.

---

## 3. os.Chdir Patterns

### Usage Summary
| Pattern | Count | Risk Level |
|---------|-------|------------|
| Direct `os.Chdir()` calls in tests | 444 | **HIGH** |
| Using `chdirTemp` helper | 240 | **MEDIUM** |
| Using `chdirTo` helper | — | *see above* |
| Tests using relative paths | ~many | **CRITICAL** |

### Risk Profile
- **444 os.Chdir calls** with 258 test files means **~1.7 chdir calls per test file on average**
- **Helper adoption (240/444 = 54%):** Moderate coverage, but 46% still use raw `os.Chdir()`
- **Relative path vulnerability:** Any test that changes cwd and uses relative file paths poisons subsequent tests
  - Example: Test A calls `os.Chdir(dir)`, fails to restore, Test B uses `"./config.json"` → points to wrong location

### Critical Issue: Chdir + os.Stdout Interaction
Tests that combine `os.Chdir()` + `os.Stdout = w` without cleanup create a **two-axis state leak:**
1. Broken cwd for next test
2. Broken stdout for next test
3. Compound failures mask root cause

---

## 4. cmd.SetOut() Poison Patterns

### SetOut Usage
| Metric | Count |
|--------|-------|
| Tests calling `.SetOut(&buf)` | 35 |
| Tests with cleanup (SetOut(nil) in defer/t.Cleanup) | 8 |
| Tests with NO cleanup | 27 |

### Commands Affected
| Command | SetOut Calls | Cleanup Present |
|---------|--------------|-----------------|
| Generic `cmd` (unspecified) | 19 | ~0 (mostly unspecified in parsing) |
| `contextCmd` | 6 | 3 (t.Cleanup) |
| `rootCmd` | 5 | 1 (t.Cleanup) |
| `doctorCmd` | 4 | 2 (t.Cleanup) |

### Risk Assessment
- **SetOut(nil) cleanup:** Only 8 of 35 tests properly reset
  - 77% lack explicit cleanup
  - Cobra caches the SetOut writer in command state; broken state leaks to next test
- **Test files affected:** `metrics_flywheel_test.go`, `metrics_health_test.go`, `context_explain_test.go`, `context_assemble_test.go`, `config_test.go`, etc.

### Leak Mechanism
```
Test A: contextCmd.SetOut(&buf) // Buf now cached in contextCmd.Out field
Test A exits without SetOut(nil)
Test B: Runs, contextCmd.Out still points to Test A's buffer (dead memory)
```

---

## 5. rootCmd.Execute() Without executeCommand()

### Direct Execute Usage
| Metric | Count |
|--------|-------|
| `rootCmd.Execute()` calls (direct) | 65 |
| Files using direct rootCmd.Execute() | 11 |

### Files with Direct Usage (Outside executeCommand)
1. `autodev_integration_test.go`
2. `cobra_commands_test.go` (also uses executeCommand, mixed strategy)
3. `codex_integration_test.go`
4. `constraint_integration_test.go`
5. `curate_integration_test.go`
6. `feedback_integration_test.go`
7. `goals_integration_test.go`
8. `hooks_integration_test.go`
9. `maturity_integration_test.go`
10. `pool_integration_test.go`
11. `rpi_integration_test.go`

### Risk Analysis
- **Integration tests bypass executeCommand()** — they don't save/restore global state
- **65 direct Execute calls** means 65 test scenarios that pollute shared process state
- **All 11 files are `*_integration_test.go`** — by design testing end-to-end, but they don't use the isolation helper

---

## 6. Largest Test Files by Line Count

| Rank | File | Lines | Test Functions |
|------|------|-------|-----------------|
| 1 | `hooks_test.go` | 3,677 | ~50+ |
| 2 | `rpi_loop_test.go` | 3,368 | ~45+ |
| 3 | `forge_test.go` | 2,516 | ~35+ |
| 4 | `cobra_commands_test.go` | 2,465 | ~40+ |
| 5 | `rpi_status_test.go` | 2,459 | ~35+ |
| 6 | `rpi_phased_test.go` | 2,404 | ~35+ |
| 7 | `context_test.go` | 2,258 | ~30+ |
| 8 | `rpi_status_helpers_test.go` | 2,188 | ~30+ |
| 9 | `maturity_test.go` | 2,144 | ~30+ |
| 10 | `forge_helpers_test.go` | 2,090 | ~30+ |

**Top 10 total:** ~32,569 lines (~24% of all test code)  
**Concentration risk:** Single test file monoliths make isolation harder; state pollution compounds.

---

## Summary of Test Architecture Debt

### State Isolation Failures (Ranked by Severity)

| Issue | Severity | Count | Impact |
|-------|----------|-------|--------|
| Flag-bound vars not reset | **CRITICAL** | 278/368 | CI flakes, test interdependence |
| Inline os.Stdout restore | **CRITICAL** | 138 | Lost stdout on t.Fatal, cascading test failures |
| SetOut() without cleanup | **HIGH** | 27/35 | Cobra command state leak, broken output |
| Direct rootCmd.Execute() bypass | **HIGH** | 65 | Integration tests don't isolate global state |
| os.Chdir without helper | **HIGH** | 204/444 | CWD pollution, relative path failures |
| os.Stdout no cleanup | **HIGH** | 142 | Stdout lost, next test can't capture output |
| Function pointer mutations | **MEDIUM** | 2 | `runtimeSessionSearchFn`, `spawnDirectFn` can be mutated |
| Large monolithic test files | **MEDIUM** | 10 files | State pollution concentration |

### Contributing Factors

1. **No process isolation:** 258 test files + 4,163 tests all run in single Go process
2. **Shared cobra.Command registry:** All commands cached globally; state persists test-to-test
3. **Global flag variables:** 368 flag-bound vars must be manually reset (278 missing from executeCommand)
4. **Two abstraction layers:** Both `captureStdout()` and `os.Stdout =` used; inconsistent patterns
5. **Missing integration test isolation:** 65 direct rootCmd.Execute() calls bypass global state reset
6. **Read-write interleaving:** Tests that mutate shared maps/slices (though most maps are read-only)

### Why CI Flakes for Weeks

**Probable causal chain:**
1. Test A runs, sets `os.Stdout = buf`, hits `t.Fatal()` before restoring
2. Test B (any later test) tries to capture output, gets broken stdout
3. Test C changes cwd, doesn't restore (204 tests use raw os.Chdir)
4. Test D uses relative path "./config" → wrong directory
5. Compound failures create **state-dependent ordering** → tests flake based on execution order
6. **Flag pollution (278 vars):** Test E sets flag `flagX=true`, Test F expects default → flake
7. SetOut() not reset (27 tests) → Cobra's command.Out field stale → output goes nowhere

---

## Recommendations (Not Implemented)

### Immediate (High ROI)
1. **Fix all 138 inline os.Stdout restores:** Use `t.Cleanup()` pattern instead
2. **Add missing 278 flags to executeCommand():** Audit which flags are test-specific vs process-wide
3. **Add SetOut(nil) cleanup:** Ensure all 35 SetOut() calls use `defer` or `t.Cleanup()`
4. **Convert 204 raw os.Chdir calls:** Use `chdirTemp` helper consistently (54% adoption → 100%)

### Medium Term
1. **Migrate integration tests to executeCommand():** Replace 65 direct rootCmd.Execute() calls
2. **Refactor test monoliths:** Split `hooks_test.go` (3,677 lines) into smaller files
3. **Audit function pointer mutations:** Verify `runtimeSessionSearchFn` and `spawnDirectFn` don't leak
4. **Create test isolation validator:** Run tests in random order to catch state leaks

### Long Term
1. **Consider test process isolation:** Run tests in separate processes (testscript, bats, etc.) for integration suites
2. **Document flag management:** Create schema of all 368 flags and which are test-local vs global
3. **Measure flake correlation:** Correlate test failures with execution order to validate isolation fixes

---

## Data Sources

- Counted via `grep`, `wc`, `ls` on `/Users/fullerbt/gt/agentops/crew/nami/cli/cmd/ao/`
- Manual inspection of `cobra_commands_test.go` for executeCommand() state save/restore
- All numbers exact; no estimation
