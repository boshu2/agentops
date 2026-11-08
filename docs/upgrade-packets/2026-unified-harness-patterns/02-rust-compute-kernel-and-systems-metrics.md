# Volume 2: Rust Compute Kernel (`pal-core`) & Systems Programming Metrics

> **Scope:** Deep systems-programming analysis of our Rust CLI compute kernel, memory/data-layout invariants (`SYS-01..05`), Gauntlet performance patterns (`200..240`), descriptor-pinned TOCTOU-safe I/O, bounded process-group execution, and mathematical observability models.

---

## 1. Rust Workspace Architecture & Safety Floor

The CLI workspace is split into a pure domain/analysis library (`core` crate) and a thin CLI binary (`cli` crate):

- **Compile-Time Memory Safety (`#![forbid(unsafe_code)]`):** Declared at the workspace and `core` crate root (`[workspace.lints.rust] unsafe_code = "forbid"`), ensuring zero `unsafe` blocks across all parsers, graph traversals, and linters. Any low-level Linux syscall (such as descriptor-relative `openat` with `O_NOFOLLOW`) is delegated to the audited `rustix::fs` safe wrapper.
- **Strict Static Lints:** `[workspace.lints.clippy]` enables `pedantic = "warn"` and `nursery = "warn"`, paired with `cargo clippy --all-targets --locked -- -D warnings`.
- **Functional Core / Imperative Shell:** Every command module in `core::commands::*` operates over immutable config/input snapshots and returns typed serializable structs + an exhaustive `Verdict` (`Pass | Warn | Block` — no `Default` impl, preventing accidental fail-open defaults).

---

## 2. Core Safety, Type-Level Invariants & OS Confinement Modules

### 2.1 Validated Newtypes (`newtypes.rs` & `types/*.rs`)
To eliminate stringly-typed bugs and CWE-22 path traversal at compile time:
- **`ConfinedPath(Utf8PathBuf)`:** `#[repr(transparent)]` wrapper with a **private inner field** and **no `Deserialize` implementation**. Untrusted JSON/YAML cannot construct a `ConfinedPath` directly; callers must invoke `ConfinedPath::try_new(root, candidate)`, which canonicalizes both paths and performs component-aware prefix verification (blocking sibling-prefix bypasses such as `/workspace-evil` matching `/workspace`).
- **`BeadId(SmolStr)` & `PrimitiveId(SmolStr)`:** Inline-allocated small strings (up to 23 bytes on stack without heap allocation) whose private constructors reject empty/whitespace strings—making the type itself proof of non-emptiness.
- **`FileId(pub u32)` & `NodeId(pub u32)`:** Zero-cost `#[repr(transparent)]`, `Copy` integer handles.
- **Fail-Closed Domain Enums:**
  - `Status::parse` maps any unrecognized tracker status token to `Status::Open`, **never** `Status::Closed`.
  - `Adoption` (`#[non_exhaustive]`) defaults unmapped/blank entries to `Unresolved`; `AdoptionVerdict` makes the `consumer` field optional so unresolved records cannot fabricate a consumer.
  - `NormalizedPath` lowercases and normalizes `\` to `/` at construction time so `.CONFIG\secret` cannot bypass `.config/` path checks.

### 2.2 Descriptor-Pinned TOCTOU-Safe File Reader (`safe_read.rs`)
Standard `std::fs::read_to_string(path)` is vulnerable to symlink following, FIFO/device hangs, and mid-read Time-Of-Check-to-Time-Of-Use (TOCTOU) file replacement. `safe_read.rs` implements a **6-step Linux descriptor-walk protocol** using `rustix::fs`:
1. Lexically normalize the path to absolute and reject root-escape or prefix anomalies.
2. Open `/`, then walk every ancestor directory descriptor-relative via `openat` with `OFlags::NOFOLLOW | OFlags::DIRECTORY | OFlags::CLOEXEC`.
3. Probe the final path component with `OFlags::PATH | OFlags::NOFOLLOW`, call `fstat`, and require `FileType::RegularFile` (rejecting symlinks, FIFOs, sockets, and device nodes).
4. Reopen the descriptor for reading (`RDONLY | NONBLOCK | NOFOLLOW | CLOEXEC`) and verify `(st_dev, st_ino)` match between probe and open to catch **swap-at-open races**.
5. Snapshot a full 9-tuple version identity: `(dev, ino, nlink, mode, uid, gid, size, mtime, ctime)`.
6. Read bounded bytes (enforcing `max_bytes` before and during read), re-verify `fstat` on the open descriptor after reading, and reopen the path one final time to verify the pathname still resolves to the identical 9-tuple snapshot (**post-read replacement defense**).
- `SafeFile` also provides `read_exact_at` byte-range reads and 1-MiB chunk streaming while preserving descriptor identity checks.

### 2.3 Bounded Subprocess Group Reaper (`process.rs`)
`run_bounded_command` executes external tools under strict `ProcessLimits { timeout: Duration, max_stdout_bytes: usize, max_stderr_bytes: usize }`:
1. **Shell-Free Execution:** Rejects zero timeouts and blocks known shell executables (`sh`, `bash`, `zsh`, `dash`, `fish`, `cmd`, `powershell`) so arguments are never shell-interpreted.
2. **Process-Group Isolation (`process_group(0)`):** Places the child in its own Unix process group with `stdin` bound to `/dev/null`.
3. **Lock-Free Pipe Overflow Signaling:** Spawns two bounded reader threads (8 KiB stack buffer) reading into capacity-capped `Vec<u8>`. If either stream exceeds its byte cap, the reader thread sets an `AtomicU8` overflow flag immediately.
4. **5ms Monotonic Poll & Descendant-Leak Prevention:** Polls child status, `AtomicU8` reader flags, reader EOF, and process-group liveness (`kill(-pgid, 0)`) every `5ms`.
   - **Completion Invariant:** A subprocess is considered cleanly finished **only** when (a) the direct child exited, (b) both stdout and stderr pipes reached EOF, **and** (c) the entire process group is gone. This prevents a leaked background grandchild holding stdout open from hanging the CLI or Masquerading as complete.
   - On timeout or byte-cap overflow (`OutputLimitExceeded`), kills the entire process group (`SIGKILL`), reaps within a 1-second grace window, and fails closed.

### 2.4 Terminal Injection Sanitizer (`terminal.rs`)
Treats terminal rendering as an untrusted security boundary:
- Strips ANSI escape sequences, `\r` carriage-return overwrites, and Unicode bidirectional override characters from all untrusted file paths and log fields before printing.
- Rejects identity fields outright if sanitization would alter their bytes.

---

## 3. Systems Programming Cost Model (`SYS-01..05`) & Gauntlet Patterns (`200..240`)

Every storage engine, log indexer, and static analyzer is designed and reviewed against the **5-Point Systems & Data-Layout Contract (`SYS-01..05`)** and **Gauntlet Rust Patterns (`200..240`)**:

| ID | Rule Name | Systems Mechanism & Complexity Guarantee | Source Implementation |
| --- | --- | --- | --- |
| **`SYS-01`** | **Stream vs. Monolith & Lazy `Cow` Projections** | Multi-record stores (`> 100` items) use line-oriented sparse `JSONL` (`#[serde(default, skip_serializing_if = "...")]` on all optional/empty fields) instead of monolithic JSON arrays or doubly-escaped JSON-in-JSON strings. Cold-path string excerpts use `Cow::Borrowed` when already present and allocate `Cow::Owned` only on demand. | `coach/types.rs:175-276` (`EventRecord`, `effective_excerpt_json`) |
| **`SYS-02`** | **Zero-Allocation Pre-Filter Before Parse (`<= 4 KiB` / `16 KiB` Bounds)** | Scan raw `&str` / `&[u8]` slices with substring or SIMD automaton checks **before** calling `serde_json::from_str` or YAML parsers. Bound regex/compaction probes to fixed prefix/suffix windows (`first 16 KiB + last 1 KiB`, UTF-8 char-boundary safe) rather than scanning multi-megabyte buffers. | `coach/parser.rs:590-619`, `coach/types.rs:512-536` |
| **`SYS-03`** | **Single-Pass In-Memory Shard Fold** | Parse each incremental source chunk once (retaining `toolCall -> arguments` maps in-memory during the pass to enrich subsequent `toolResult` records) and pass `&[Shard]` slices to downstream analyzers rather than re-reading files $K$ times. | `coach/parser.rs:537-887` (`parse_source_incremental`) |
| **`SYS-04`** | **Atomic Temp+Rename with Commit-Barrier `fsync()`** | Write outputs to a mode-`0600` temp file (`create_new`) inside a mode-`0700` non-symlink directory, flush to the OS page cache, call `sync_all()` once at the commit barrier, and atomically `rename()` to the target path. *(See §6 Gap Matrix regarding parent-dir `fsync`).* | `coach/parser.rs:139-195` (`atomic_write_private`) |
| **`SYS-05`** | **Amortized $O(1)$ Metadata Resume** | Skip unchanged completed sources (> 4 KiB) in $O(1)$ time without opening or re-hashing the file by verifying `!is_symlink() && is_file() && nlink == 1 && len == old.size && ino == old.ino && dev == old.dev` via `symlink_metadata`, plus prefix SHA-256 verification for appended logs. | `coach/report.rs:43-65`, `coach/parser.rs:545-589` |
| **Pat `200/205`** | **SIMD Multi-Pattern Fast-Path Gate (`> 10 GiB/s` is the library's published figure, not a `pal` measurement; see Vol. 7 §4)** | Compile multi-literal token sets into `LazyLock<AhoCorasick>` automata (`lint_readability.rs:18-41`) and scan the entire file buffer once before running line-by-line regexes or AST walkers; compute line numbers from byte offsets using SIMD `memchr_iter(b'\n')` (`validate.rs:1117`, `lint_layers.rs:206`). | `lint_readability.rs`, `validate.rs` |
| **Pat `215`** | **Small-Set `Vec + binary_search` ($N \le 100$) vs. `FxIndexMap` ($N > 100$)** | Use static sorted arrays + `binary_search_by_key` for frozen lookup tables (`graph_ontology.rs:1880`: $O(\log N)$ zero-allocation lookup, L1 cache-local) and `IndexMap<K, V, FxBuildHasher>` + `.sort_unstable_keys()` for large dynamic tables. | `graph_ontology.rs:1880`, `newtypes.rs:14-25` |
| **Pat `220/225`** | **Bounds-Check Elision (`as_chunks`) & Enum Devirtualization** | Use closed Rust `enum` match arms (`Operation`, `RouteProfile`, `Verdict`) rather than heap-allocated `&dyn Trait` objects so LLVM devirtualizes and inlines across passes. | `verdict.rs`, `graph_ontology.rs` |
| **Pat `235/240`** | **Zero-Clone Loops & `LazyLock` Automaton Cache** | Compile all `Regex`, `AhoCorasick`, and grammar patterns once via `std::sync::LazyLock` / `OnceLock` (`repo_map.rs:135-181`, `lint/ci.rs`, `lint/secrets.rs`). Never compile regexes inside functions or loops. | Workspace-wide |

---

## 4. Deep Dive: Rust Compute-Kernel Subsystems

### 4.1 Sealed Graph Index & Lazy Byte-Slice Hydration (`graph_ontology.rs`, `graph_index.rs`, `graph_impact.rs`)
1. **Static Ontology Table (`graph_ontology.rs`):** Defines `EdgeSemantics` across 4 `RouteProfile`s (`Execution`, `Deployment`, `Artifact`, `Variable`), edge polarity, traversal capability (`Advance | Bounded | Conditional | Frontier | Never`), and base cost + evidence surcharges. `OntologyInventory` hashes observed edge-kind counts and blocks queries if unknown edge kinds appear or policy hashes drift.
2. **Canonical Byte-Offset Sidecar Index (`graph_index.rs`):**
   - Verifies that the raw graph file ($\le 64\text{ MiB}$) matches canonical pretty ASCII JSON byte-for-byte and explicitly asserts `secret_values_recorded == false`.
   - Records exact canonical byte `(offset, length)` slices for every `NodeIndex`, `EdgeIndex`, and `BoundaryIndex`, plus sorted `forward: Vec<Vec<usize>>` and `reverse: Vec<Vec<usize>>` adjacency lists.
   - **Lazy Hydration:** At query time (`QuerySession::open_with_acceptance`), the CLI chunk-hashes the graph in 1-MiB blocks via `SafeFile`, performs search and ranking purely on the lightweight index skeleton, and **reads only the exact byte ranges `(offset, length)` of winning nodes/edges** from disk.
3. **Bounded BFS Blast-Radius & Explicit Frontiers (`graph_impact.rs`):**
   - Caps traversal at `depth <= 12`, `nodes <= 5,000`, `changed_paths <= 5,000`.
   - Edges marked `Frontier` or `Conditional` are never silently dropped: they emit explicit frontier records and provenance warnings. Unmapped file paths return `awaiting-remap` rather than a false `"0 impact"` pass.

### 4.2 Deterministic Diff Validator (`validate.rs`)
1. **Hardened Git Execution (`GIT_ENV_SCRUB`):** Scrubs `GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`, `GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY`, and `GIT_ALTERNATE_OBJECT_DIRECTORIES`, and passes `-c core.hooksPath=/dev/null -c protocol.file.allow=never` under a 15-second / 8-MiB `ProcessLimits` budget.
2. **Non-Mutating Merge Conflict Probe:** Runs `git merge-tree --write-tree <target_branch> <head_sha>` in-memory to set `merge_tree_clean: Some(true | false)` without touching the working tree index.
3. **NUL-Delimited Rename Detection:** Parses `git diff -M --name-status -z` so filenames containing spaces or newlines cannot corrupt diff attribution.
4. **Tricorder-Style Normalized Finding Signatures + Causal Invalidation:**
   - Normalizes each finding by replacing its file path and trailing line number with `<FILE>` (`normalize_finding_signature`) so untouched pre-existing findings in a modified file can be matched as a multiset between `base_sha` and `head_sha`.
   - **Causal Invalidation:** Extracts `extends:`, `before_script:`, and `script:` chains from both `base` and `head`. If a diff modifies a parent template or causal directive, baseline suppression is invalidated and downstream findings fire as blocking regressions.
5. **Static Def/Ref Symbol Delta (`scip-symbol-delta`):** Identifies deleted or renamed roles, playbooks, and CI templates between `base` and `head`, flags dynamic Jinja interpolation (`{{ ... }}`) as incomplete static proof, and runs bounded fixed-string `git grep -F -n` across `head` to block dangling references.

### 4.3 CAS Structural Codemods & Rosie Wave Sharding (`codemod.rs`)
1. **Protected-Oracle Deny Policy:** Hard-blocks rewrites touching `.gitlab-ci.yml`, `tests/`, `ci_templates/`, `features/`, or linter/checker scripts (`denied_files`).
2. **Necula-Style Translation Validation:** Before rewriting `from -> to` in a file:
   - Counts occurrences of `from` in the preimage.
   - Blocks with `collision_files` if `to` already exists in the file (and `to` does not contain `from`), preventing accidental symbol capture.
   - Applies the replacement and verifies **idempotency** (`count(from, postimage) == 0` when `to` does not contain `from`).
3. **Kung-Robinson Optimistic Concurrency (`preimage_sha256` / `postimage_sha256`):** `plan` records the SHA-256 of every file before and after transformation. `apply` re-reads all target files and verifies every `preimage_sha256` matches before writing a single byte.
4. **Rosie Disjoint Wave Sharding (`shard`):** Validates the union post-image through `validate` first, then partitions modified files into deterministic, pairwise-disjoint `RosieWavePacket`s (`id`, `prompt`, `write_scope[]`) ready for `factory_dispatch`.

### 4.4 Session Telemetry & N-Gram Trajectory Miner (`coach/report.rs`)
- Filters out secret-bearing events and raw shell history.
- Expands `codemode` `nested_tools[]` calls into their constituent tool sequence, filters to allowlisted domain tools (excluding primitive file/shell/status verbs like `read`, `edit`, `bash`, `set_agent_status`), collapses consecutive identical retries, and slides a 3-tool window (`.windows(3)`) across every session lineage.
- Ranks 3-gram trajectories by `(total_count, error_count, distinct_lineages)` to identify multi-step agent loops that should be crystallized into 1-call CLI commands or extension tools.

### 4.5 Hexagonal Impurity Scorer (`repo_map.rs`)
Scans a polyglot repository (capped at 4 MiB/file, regexes cached in `OnceLock`) and computes a deterministic **Hexagonal Impurity Score** for every non-test domain module:
$$\text{ImpurityScore}(m) = 2 \times \text{count}(\text{subprocess / network / fetch calls in } m) + \text{count}(\text{infrastructure / adapter imports in } m)$$
Ranks the top 40 impure modules alongside duplicate domain class definitions and oversized coupled entrypoints.

---

## 5. Queueing Physics, Engineering Economics & Observability Math

| Model / Metric | Exact Mathematical Formulation | Operational Threshold / Decision Rule |
| --- | --- | --- |
| **Little's Law (Review Queue & WIP Physics)** | $L = \lambda W \implies W = \frac{L}{\lambda}$ where $L$ is concurrent open PRs, $\lambda$ is merge throughput, and $W$ is lead/wait time. | Cap active PRs at **$L \le 1\text{–}2$ per pair/issue**; drain reviewable PRs before starting new slices; rotate driver after 2 consecutive solo PRs. |
| **Refactor Opportunity Matrix** | $\text{Score} = \frac{\text{Impact} \times \text{Confidence}}{\text{Effort}}$ (with $1\text{–}5$ scales). | Execute structural/performance refactors only when **$\text{Score} \ge 2.0$** and an **Isomorphism Proof** (identical ordering, tie-breaking, and golden checksums) is provided. |
| **24h CPU CFS Throttling Percentage** | $\text{CFS}_{\text{throttled}}\% = 100 \times \frac{\sum \text{increase}(\text{container\_cpu\_cfs\_throttled\_periods\_total}[24\text{h}])}{\max\!\left(\sum \text{increase}(\text{container\_cpu\_cfs\_periods\_total}[24\text{h}]),\, 1\right)}$ | Flag containers with **$\text{CFS}_{\text{throttled}}\% \ge 25\%$** as high-throttling outliers (`cohort_diff` default threshold). |
| **Counter Reset-Safe `observedIncrease`** | For Prometheus counter series $x_1, \dots, x_n$: $\Delta_{\text{obs}} = \sum_{i=2}^{n} \begin{cases} x_i - x_{i-1} & \text{if } x_i \ge x_{i-1} \\ x_i & \text{if } x_i < x_{i-1}\text{ (counter reset)} \end{cases}$ | Used in `pod_aar` (`prometheus.ts:75-164`) over cgroup-v2 `container_fs_writes_bytes_total` and `reads_bytes_total` (which accurately measure `emptyDir` scratch I/O where `container_fs_usage_bytes` is unpopulated). |
| **BubbleUp Cohort-Diff Prevalence Delta & Lift Ratio** | Partition namespace pods $P$ into outlier cohort $S$ and baseline $R = P \setminus S$. For attribute $d=v$: $$P(v \mid S) = \frac{\text{count}(v \in S)}{|S|}, \quad P(v \mid R) = \frac{\text{count}(v \in R)}{|R|}$$ $$\Delta P(v) = P(v \mid S) - P(v \mid R), \quad \text{Lift}(v) = \frac{P(v \mid S)}{P(v \mid R)}$$ | Require minimum cohort cardinality **$|S| \ge 3$ and $|R| \ge 3$** (`min_cohort_size = 3`). If violated, set `populationEstimateAllowed = false` and hard-block ranking output at the schema wrapper (`index.ts:170-176`). |
| **K8s Indexed Job DNS-1123 Name Budget (`BA401`)** | Kubernetes pod labels/hostnames cap at 63 chars: $\text{len}(\text{job\_name}) + \text{len}(\text{"-" + timestamp\_10 + "-" + index + "-" + pod\_hash\_5}) \le 63$ | Enforce **$\text{len}(\text{job\_name}) \le 41$ chars** for 1–10 indexed trials (`<= 40` for 11–100 trials) at static CUE/YAML preflight time. |

---

## 6. Honest Implemented-vs-Target Gap Matrix (For AgentOps Adoption)

Per our anti-ceremony Honesty Protocol (`just-say-no-to-process-porn-and-ceremony`), we separate **what is live in `pal-core` Rust source today** from **declared Gauntlet target optimizations** that should be completed when porting to `agentops`:

| Capability | Live in `pal-core` Source Today | Target Upgrade When Porting to `agentops` |
| --- | --- | --- |
| **Graph Adjacency Storage** | `Vec<Vec<usize>>` sorted adjacency lists + canonical JSON byte-slice index (`graph_index.rs:36-46`). `BTreeMap`/`BTreeSet` used for deterministic output. | Flatten `Vec<Vec<usize>>` into contiguous **Compressed Sparse Row (CSR)** arrays (`offsets: Vec<u32>`, `edges: Vec<NodeId>`) + `FixedBitSet` visited masks (`newtypes.rs` already defines `FileId(u32)`, `NodeId(u32)`, and `FxIndexMap`). |
| **Atomic File Write Durability** | `atomic_write_private` (`coach/parser.rs:139-195`) writes to `.tmp`, calls `file.sync_all()`, and atomically `rename()`s to target. | Add **parent-directory `fsync` (`dir_fd.sync_all()`) after `rename()`** (already implemented in `validate.py:416,443`) so directory entry updates survive kernel panic/power loss. |
| **Codemod Multi-File Apply** | `codemod apply` (`codemod.rs:221-246`) verifies all `preimage_sha256` hashes before writing any file. | Stage multi-file writes via temp files in target directories and atomically rename all, or run inside an isolated scratch Git worktree (`stageAndVerifyDeterministicPlan`). |
| **Symbol Def/Ref Delta** | `validate.rs:699-824` (`scip-symbol-delta`) detects deleted/renamed files/roles/templates and runs bounded `git grep -F -n` across `head`. | Wire optional `scip` / `tree-sitter` AST symbol tables for language-aware cross-file Def/Ref resolution while keeping `git grep -F` as the zero-config fallback. |
