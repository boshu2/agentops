# Volume 4: Staged Hybrid Software Factory & Host Capability-Broker Extensions

> **Scope:** Source-level engineering breakdown of the four primary host-broker extensions: **Staged Hybrid Software Factory**, **Git & Merge-Request Fault-Tree Workbench**, **Read-Only Cluster & Observability Broker**, and **Polyglot Autonomy Workbench**.

---

## 1. Staged Hybrid Software Factory (`pi-software-factory`)

### 1.1 Unforgeable Proof-Carrying Check Receipts (`check-evidence.ts`)
To prevent an LLM worker or subagent from claiming `"all tests passed"` in prose or constructing a synthetic JSON receipt:
1. **Runtime `Symbol` Branding & Deep Freeze (`check-evidence.ts:8-73`):**
   - When `executeManagedChecks` completes, it constructs a `ManagedCheckReceipt` binding `(repo_root, base_sha, head_sha, tree_sha, producer_version, required_checks[], checks[], overall_status)`.
   - The receipt is deep-cloned via JSON, stamped with a module-private `Symbol("TrustedManagedCheckReceipt")`, recursively frozen (`Object.freeze` on the root object and every nested `checks[]` entry), and registered in a private in-memory `Map<string, TrustedManagedCheckReceipt>` keyed by `receipt_id`.
2. **Cryptographic Contract Digest (`check-evidence.ts:29-48, 151-160`):**
   $$\text{contract\_digest} = \text{SHA256}\!\left(\text{canonical\_repo\_root} \;\|\; \text{norm\_ws}(\text{acceptance}) \;\|\; \text{sorted\_norm}(\text{write\_scope}) \;\|\; \text{dedup\_ordered}(\text{check\_commands})\right)$$
   - `verifyCheckEvidence()` recomputes `contract_digest` from the caller's live contract rather than trusting the digest field stored inside the receipt.
3. **Fail-Closed Verification (`check-evidence.ts:113-277`):**
   - Verifies: (a) `Symbol` brand and presence in the trusted in-memory `Map`, (b) schema validity, (c) canonical `repo_root` match, (d) exact `base_sha`, `head_sha`, and `tree_sha` match, (e) recomputed `contract_digest` match, (f) exact ordered check-ID equality (`check.id === check.command`), and (g) per-check `exit_code === 0 && status === "passed" && complete === true`.
   - Any failed check yields `failed_checks`; any skipped, timed-out, canceled, incomplete, or mismatched receipt yields `not_proven`.

### 1.2 Bracketed Subject Snapshots in Managed Checks (`managed-checks.ts`)
1. **Reviewed Shell-Free Command Grammar (`managed-checks.ts:91-216`):**
   - Parses `check_commands[]` into fixed argument vectors (`execFile`), allowing only reviewed command shapes (`npm test/run`, `git diff --check`, `pal ...`, `pytest`, `cargo test/clippy`) and rejecting shell metacharacters, `..` traversal, arbitrary binaries, or `--root` flags escaping the workspace.
2. **Pre/Post Subject Bracket (`managed-checks.ts:219-250, 277-404`):**
   - **Before** running checks, snapshots `(HEAD, HEAD^{tree}, git status --porcelain, SHA256_map(protected_oracle_files))`. Refuses to start if the working tree is dirty or mismatched (`stale_subject`).
   - **After** running checks inside `bwrap --unshare-net`, repeats the exact snapshot. If a check command mutated tracked files, dirtied the tree, or modified any protected-oracle file during execution, the run is marked `stale_subject` and rejected.

### 1.3 Protected-Oracle Diff Auditor (`oracle-gate.ts`)
`auditDiffAgainstContract()` (`oracle-gate.ts:265-503`) inspects the byte-exact Git diff between `base_sha` and `head_sha` and blocks on 7 reward-hacking signatures:
1. **Out-of-Scope Writes:** Any modified/added/deleted path outside normalized `write_scope[]`.
2. **Protected-Oracle Path Mutation (`oracle-gate.ts:42-85`):** Any unapproved edit to `tests/`, `features/`, `ci_templates/`, `.gitlab-ci.yml`, `check-*.py`, or `*.spec.ts` / `*_test.rs`.
3. **Net Assertion Drop:** Counts added vs. removed assertion statements (`assert`, `expect(`, `assert_eq!`, `should`) across touched test files; flags when `removed_assertions > added_assertions`.
4. **Tautological Assertions:** Flags `assert True`, `expect(true).toBe(true)`, `assert_eq!(1, 1)`.
5. **New Skips / Suppressions:** Flags newly added `@pytest.mark.skip`, `#[ignore]`, `.skip(`, `noqa`, `# type: ignore`, `#[allow(...)]`, or `eslint-disable`.
6. **CI Gate Line Deletion:** Flags removal of `script:` or `check` invocations in CI definitions.
7. **Corpus Anchor Drift:** Checks whether any symbol anchored in `.agents/corpus/**/*.md` was deleted or renamed without updating its anchor.

### 1.4 Token-Stream Translation Validation for Deterministic Plans (`deterministic-plan.ts`)
When applying deterministic JSON/config normalization without an LLM (`deterministic-plan.ts:30-302`):
1. Runs a custom lossless lexer that rejects UTF-8 BOM, CRLF line endings, control characters, non-canonical numbers, and **duplicate keys at any object nesting depth**.
2. **Dual Translation Validation:** Proves that the formatted post-image has (a) the exact same non-whitespace token stream as the pre-image, (b) deep strict value and key-order equality, and (c) **second-run idempotence** (`format(format(x)) === format(x)`).

### 1.5 Per-Commit Retained Worktree Auditing (`candidate-integration.ts` & `worktree-harvest.ts`)
When harvesting work from an isolated `implement` worktree (`verifyAndIntegrateRetainedAgentCandidate`, `candidate-integration.ts:207-384`):
- Requires linear commit history from `merge-base(target, candidate)` to `candidate_head`.
- **Per-Commit Oracle Audit:** Runs `auditDiffAgainstContract()` on **every individual commit $c_k$** in the chain (`c_{k-1}..c_k`) *in addition* to auditing the net range (`base..head`).
- *Why:* Prevents an agent from deleting a failing test in commit 1, recording a green check, and restoring the test in commit 2.
- Strips only untracked bootstrap symlinks (`node_modules`, `.venv`), verifies target `HEAD` cleanliness via CAS immediately before `git cherry-pick`, and preserves recovery state on conflict.

### 1.6 Kung-Robinson Multi-Shard Wave Serializability (`precomputed-wave.ts`)
`executePrecomputedDeterministicWave()` (`precomputed-wave.ts:32-248`) stages $N$ deterministic shard packets into one atomic commit without LLM calls:
1. **Global Write-Scope Disjointness:** Verifies $\forall i \neq j,\; \text{WriteScope}(S_i) \cap \text{WriteScope}(S_j) = \emptyset$ using conservative glob-intersection checking (`oracle-gate.ts:505-560`).
2. **Kung-Robinson Read/Write-Set Validation:**
   - Every shard $S_i$ declares `reads: Map<Path, PreimageSha256>` and `writes: Map<Path, {preimageSha256, postimageSha256}>`.
   - Rejects the wave if any shard reads a path mutated by another shard ($\text{ReadSet}(S_i) \cap \text{WriteSet}(S_j) \neq \emptyset$ for $i \neq j$, preventing write skew), or if two shards declare conflicting `preimage_sha256` digests for the same shared read path.
3. **Single Union Staging:** Applies the union of all $N$ shards in one scratch Git worktree, runs `bwrap` managed checks once, mints a branded receipt, and integrates atomically.

### 1.7 `sqz` Volatile Log-Template Compaction (`process.ts` & `hooks.ts`)
Installed on the `tool_result` hook (`hooks.ts:410-569`, `process.ts:76-388`):
1. **Secret Redaction First:** Scrubs bearer tokens, PEM blocks, API keys, and auth headers before any compaction or disk spill.
2. **Volatile Template Collapse (`collapseVolatileLogTemplates`):**
   - Skips valid whole-JSON payloads so structured tool outputs are never corrupted.
   - Scans up to `64 KiB` of raw lines (`128 KiB` for `codemode` envelopes).
   - Preserves git SHAs, UUIDs, URLs, unified diff hunks (`+`/`-`/`@@`), and error/diagnostic lines untouched.
   - Masks volatile timestamps, durations (`123ms`), and progress percentages (`[ 42%]`) into normalized templates; collapses consecutive runs of `>= 3` equivalent compile-progress or passing-test lines into a single summary line.
3. **Repeated Tail Block Collapse (`collapseRepeatedTailBlocks`):**
   - Searches the tail of the output for repeating `1..4`-line blocks occurring `>= 10` times consecutively and replaces the repetition with `[repeated K times]`.
4. **Session-Scoped SHA-256 Deduplication & Smart Spill (`buildSpillPreview`):**
   - Non-`read` tool outputs `>= 4 KiB` are hashed (`SHA-256`) in a 128-entry LRU session map; identical repeat outputs are replaced with a pointer to the prior spill file.
   - Outputs exceeding `12 KiB` (`codemode`) or `24 KiB` (general tools) spill to `/tmp/.../factory-outputs/<call>.log` and return a structured preview (`buildSpillPreview` apportions preview bytes across top-level JSON keys for `codemode` envelopes).

---

## 2. Git & Merge-Request Workbench (`pi-gitlab-workbench`)

| Subsystem | Source File & Lines | Exact Engineering Algorithm |
| --- | --- | --- |
| **6-Part Snapshot & Merged-Result Binding** | `change-preflight.ts:12-96` | Constructs $S = (\text{target\_proj}, \text{source\_proj}, \text{mr\_iid}, \text{base\_sha}, \text{head\_sha}, \text{target\_tip\_sha})$. Accepts direct `head_sha` pipelines or `refs/merge-requests/<iid>/merge` pipelines whose commit `parent_ids` contain `head_sha` (binding the second parent as `target_tip_sha`). |
| **Bounded BFS Child-Pipeline Fault Tree** | `change-preflight.ts:98-183` | Traverses parent pipeline $\to$ bridge jobs $\to$ downstream child pipelines via cycle-guarded BFS (`max_pipelines = 20`, `max_depth = 4`). Filters out retried jobs (`include_retried: false`), fetches trace tails for failed jobs, and classifies each node (`pass | fail | not_proven`). Unresolved downstream bridges become explicit proof gaps (`not_proven`). |
| **Section-Scoped `infra_transient` Classifier** | `pipeline-diagnostics.ts:25-126` | Classifies a job failure as `infra_transient` (downgrading to `NOT_PROVEN` rather than `FAIL`) **only** when GitLab reports a system runner `failure_reason` (`runner_system_failure`, `stuck_or_timeout_failure`) **or** an infrastructure regex matches inside one of the 5 pre-script sections (`prepare_executor`, `prepare_script`, `get_sources`, `restore_cache`, `download_artifacts`) **before** `step_script`/`build_script` begins. Any error inside `step_script` is strictly `blocking_failure`. |
| **Zero-Token Background Pipeline Watch** | `pipeline-watch.ts:67-109, 190-416` | Keyed by `host/project/branch` with an `AbortController` tied to session shutdown and mode changes. When `background: true`, starts a host timer loop (`poll_interval_seconds` clamped `[5, 300]`), returns `terminate: true` to end the LLM turn immediately (`0` tokens burned during wait), and calls `pi.sendMessage(..., { triggerTurn: true })` **once** when the pipeline reaches a terminal state or the first job fails (`fail_fast: true`). |

---

## 3. Read-Only Cluster & Observability Broker (`pi-cluster-workbench`)

### 3.1 Cross-Field Output-Contract Invariant Assertions (`src/index.ts:80-228` & `output-contracts.ts`)
Every cluster tool response passes through TypeBox `Value.Check(Schema, payload)` followed by **8 cross-field semantic assertions** that fail closed if a telemetry adapter produces self-contradictory data:
1. **Exact-UID Join Integrity:** If `coverage.join_strength === "exact_uid"`, every event in `timeline[]` must have `event.podUid === coverage.pod_uid`. Events from prior incarnations of the same pod name must be isolated in `excludedHistoricalEvents[]`.
2. **Completeness vs. Gap Mutual Exclusion:** If `coverage.completeness === "complete"`, then `coverageGaps[]`, `excludedHistoricalEvents[]`, `loki_truncated`, and `fallback_incomplete` must all be empty/false.
3. **Workload Join Honesty:** If `workloadJoinUnavailable !== null`, `totalWorkloadsMatched` must be `null` (cannot report numeric counts when the join failed).
4. **Node Capacity Count Identity:** `totalNodes === nodes.length` and `readyNodes === nodes.filter(n => n.ready).length`.
5. **Drain Safety Consistency:** If `pdbBlockers.length > 0 || assessment === "blocked"`, `safeToDrain` must be `false`.
6. **Cohort Statistical Suppression Gate:** In `cohort_diff`, if `populationEstimateAllowed === false` (because $|S| < \text{min\_cohort\_size}$ or $|R| < \text{min\_cohort\_size}$), `rankedDiffs[]` must be empty (`[]`).
7. **Rightsizing Cohort Conservation:** `coverage.cohortsCount === workloads.length` and `rankedCohortsCount + unrankedCohortsCount === workloads.length`.
8. **Loki Saturation Honesty:** If `serverLimitReached === true`, `completeness` must be `"partial"`, never `"complete"`.

### 3.2 Three-Plane Server-Side LogQL & Correlation-ID Join (`loki-tempo.ts:170-455, 704-833`)
`istio_gitlab_diagnose` executes 3 bounded server-side LogQL queries in parallel:
- **Plane 1 (Ingress Gateway):** Envoy access logs filtered for `5xx`, `0 DC`, `0 DR`, `UH`, `UC`, `UF`, `UO`, `URX`.
- **Plane 2 (Application Services):** Webservice/Workhorse/Sidekiq logs filtered for HTTP `>= 500` and `exceptions_json`.
- **Plane 3 (Mesh Transport / ztunnel):** Ambient mesh HBONE proxy logs filtered for protocol errors and zero-byte stalls (`bytes_sent=0 bytes_recv=0`).
- **Benign Noise Suppression & Correlation Join:** Automatically suppresses known benign conditions (HTTP `421` Lua connection coalescing, client `499` cancels, WebSocket `101` closes, post-transfer ztunnel closes) into `benignSuppressed` counters, while joining Workhorse `5xx` records to Rails `exceptions_json` stack traces via shared `correlation_id`.

---

## 4. Polyglot Autonomy Workbench (`pi-autonomy-workbench`)

### 4.1 Single-Pass `RepoFileIndex` & 16-Hex Diff-Scoped Fingerprint Ratchet (`autonomy_check.py`)
1. **`RepoFileIndex` (`autonomy_check.py:55-151`):** Enumerates tracked repository files once via Git and lazily memoizes raw bytes, UTF-8 text, line offsets, and parsed Python `ast.AST` trees so 20+ static checkers run in a single pass without redundant disk I/O.
2. **16-Hex Finding Fingerprints (`autonomy_check.py:154-277, 1931-1984`):**
   $$\text{fingerprint} = \text{SHA256}(\text{rule\_id} \;\|\; \text{rel\_path} \;\|\; \text{normalized\_symbol\_or\_snippet})[0..16]$$
   - Suppresses findings outside touched diff files and findings whose 16-hex fingerprint appears in `.autonomy-lint-baseline.json`.

### 4.2 The Complete Polyglot Rule Catalog (`BA101..BA602`)

| Rule ID | Language / Plane | Defect Class Detected | Analysis Mechanism |
| --- | --- | --- | --- |
| **`BA101`** | C++17 | Identical `if / else` branches or duplicate boolean conditions | `ast-grep` structural pattern (`rules/ast_grep_rules.yml`) |
| **`BA102`** | C++17 | Unsafe `std::stoi/stod/stol`, throwing destructors, exception slicing (`catch (std::exception e)` by value), manual `mtx.lock()`, dangling `std::string_view` / local ref return, `erase()` iterator invalidation, `std::move` on `const` | `ast-grep` structural AST rules |
| **`BA103`** | C++17 (Real-Time) | Asymmetric mutex locking across callbacks/timers or re-entrant non-recursive mutex lock deadlocks | CFG/call-graph mutex lock-symmetry analyzer (`autonomy_check.py`) |
| **`BA104`** | C++17 (Real-Time) | Uninitialized primitive/pointer class members in constructors | `cppcheck` with custom middleware macro header (`edgeos_cppcheck_macros.h`) |
| **`BA105`** | C++ / CMake | Orphan `.cpp` source files not registered in any `CMakeLists.txt` target | `RepoFileIndex` vs. CMake target source closure |
| **`BA106`** | C++ Headers | `using namespace ...;` inside `.h` / `.hpp` header files (namespace pollution) | `ast-grep` header rule |
| **`BA107/108`** | IDL / Middleware | `.eos` schema syntax/field-numbering errors and missing constructor port registration | Schema & macro cross-checker |
| **`BA201/202`** | CUE Config | CUE syntax/unification errors and concrete base values blocking overlay overrides | `cue vet` + default-unification (`*val \| type`) AST check |
| **`BA203`** | CUE Overlays | Asymmetric keys across the 5 platform overlays | Multi-overlay key-set equivalence join |
| **`BA204`** | C++ $\leftrightarrow$ CUE | Port name/type mismatch between C++ `EDGEOS_*_PORTS` macros, `.eos` IDL, and CUE wiring | 3-way cross-language symbol table join |
| **`BA301`** | Python | Identical branches, `eval()`, bare `raise` outside except, literal `is` comparison, bare `except:`, unsafe `yaml.load()`, `shell=True`, `requests.*` without `timeout=` | `ast-grep` + `ast.NodeVisitor` |
| **`BA302`** | Python Ontology | Mixing legacy `C2Table` classes with modern `ModernC2Table` domain types in the same module | Import & symbol-origin taint checker |
| **`BA303`** | Python Packaging | Reverse dependency leak from new delivery package back into deprecated legacy delivery package | Strangler-fig one-way import boundary check |
| **`BA304..306`** | Python / Docker | Ruff/Mypy parity, missing runtime imports in production multi-stage `Dockerfile` (`COPY` closure), and mutable default args | Multi-stage `Dockerfile` `COPY` vs. Python import graph closure |
| **`BA401..403`** | K8s Batch / CUE | K8s Indexed Job name `> 41` chars (`DNS-1123` overflow after suffix), retry/backoff budget violations, and chart/app version drift | Static length + YAML/CUE schema evaluator |
| **`BA501/601`** | CMake / Conan | Stale CMake cache variables and `conan.lock` / `conanfile.py` dependency drift | Lockfile & build-graph consistency checker |
| **`BA602`** | GitLab CI | Unpinned container tags, missing `interruptible: true`, or broken `needs:` references | CI DAG & job hygiene analyzer |

### 4.3 Six-Plane Cross-Layer Contract Dossier (`extract_four_seam_dossier`)
`autonomy_check.py:2296-2442` (`mode="mr_validate"`) synthesizes a **6-plane engineering dossier** plus a 100% changed-file coverage ledger in one call:
1. **Plane 1:** Deterministic Exit Gates (`BA101..BA602` summary)
2. **Plane 2:** C++ $\leftrightarrow$ IDL $\leftrightarrow$ CUE Port-Contract Parity
3. **Plane 3:** Python Domain & Ontology Seam Integrity
4. **Plane 4:** Classification, Escaping & Path-Containment Security
5. **Plane 5:** Writer-Reader Serialization & CLI/Docs Flag Parity
6. **Plane 6:** Positive + Planted-Negative Test Signal & Blast-Radius Impact (`compute_impact` CMake targets, `ctest -R` regex, `pytest` files, and CI jobs).
