---
name: performance-profile-triage
description: |-
  Use when investigating slowness with baselines, profiler evidence, and ranked bottlenecks.
  Triggers:
practices:
- code-complete
- empirical-performance
- validate
hexagonal_role: driving-adapter
consumes:
- source-code
- runtime-metrics
- profiler-output
produces:
- performance-report
- validation-evidence
context_rel:
- kind: customer-of
  with: validate
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: execution
  stability: stable
  dependencies:
  - validate
output_contract: "performance report with baseline, profiler evidence, ranked bottlenecks, change plan, and verification results"
user-invocable: false
---

# Profiling Software Performance

Use this skill to make performance work empirical. The goal is to identify the constraint that matters, change only what the evidence supports, and prove the result against a comparable baseline.

## Required Inputs

- A target workload or user-visible operation.
- A measurable objective, such as latency percentile, throughput, memory, allocation rate, startup time, build time, or CPU time.
- The command, scenario, fixture, request, trace, benchmark, or reproduction path that exercises the workload.
- Relevant constraints: hardware, environment, data size, concurrency, external services, caching state, and acceptable tradeoffs.

If the user gives only a vague symptom, first turn it into a measurable workload and say what will be measured.

## Workflow

1. Define the performance question.
   - Name the operation under test.
   - State the metric and unit.
   - Record environment details that can change results.
   - Separate correctness bugs from performance symptoms before optimizing.

2. Capture a baseline.
   - Run the smallest realistic workload that reproduces the cost.
   - Use multiple samples when variance is expected.
   - Preserve raw command output, benchmark output, trace IDs, or profiler artifacts.
   - Do not treat one uninstrumented timing as enough evidence unless no better measurement path exists.

3. Choose instrumentation that matches the suspected resource.
   - CPU-bound: CPU profiles, flamegraphs, sampling profilers, benchmark attribution, or trace spans.
   - Memory-bound: allocation profiles, heap snapshots, retained-object analysis, or GC telemetry.
   - IO-bound: query plans, request traces, syscall summaries, storage metrics, network timing, or cache hit rates.
   - Contention-bound: lock profiles, goroutine or thread dumps, scheduler traces, queue metrics, or blocking profiles.
   - Frontend-bound: browser performance traces, long task analysis, bundle size, network waterfalls, layout and paint timing.

4. Rank bottlenecks.
   - Sort findings by measured contribution to the target metric.
   - Prefer cumulative impact over surprising implementation details.
   - Mark each candidate as confirmed, plausible, or ruled out.
   - Include the evidence that supports the ranking.

5. Select the narrowest fix.
   - Change the hot path rather than adjacent code.
   - Preserve behavior before changing algorithms, caching, concurrency, batching, or IO shape.
   - Document tradeoffs such as memory growth, stale data, startup cost, precision loss, or operational complexity.
   - Avoid broad refactors unless the profile shows they are needed.

6. Verify the result.
   - Re-run the same workload and compare to the baseline.
   - Run relevant correctness tests.
   - Check for regressions in secondary metrics, especially memory, error rate, output quality, and concurrency behavior.
   - If results are noisy, report the range and whether the change is still defensible.

## Output Format

Return a concise performance report:

- Workload: what was measured and how.
- Baseline: metric values and evidence location.
- Evidence: profiler, trace, benchmark, or diagnostic artifacts used.
- Ranked bottlenecks: ordered list with measured contribution and confidence.
- Change: what was changed or recommended.
- Verification: after values, comparison to baseline, tests run, and residual risk.

## Guardrails

- Do not optimize without a baseline unless the task is explicitly only about designing a measurement plan.
- Do not claim a speedup from code inspection alone.
- Do not hide measurement variance.
- Do not trade correctness, security, or durability for speed without explicit user approval.
- Do not use synthetic microbenchmarks as the only proof for a user-facing workflow unless they directly model the hot path.
