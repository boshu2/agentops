---
id: learning-2026-04-12-tier1-forge-wins
type: learning
date: 2026-04-12
category: architecture
confidence: high
maturity: provisional
utility: 0.7
---

# Learning: Spike-validated prompt templates should be copied verbatim

## What We Learned

The W0-6 empirical spike validated a specific PROMPT_TEMPLATE on real session chunks (3/3 PASS). The implementation copied it verbatim into `summarizer.go:PromptTemplate` with a "DO NOT edit without re-running the spike" comment. This eliminated a class of bugs where prompt drift degrades output quality without any test catching it — the output shape is validated by the spike's scoring contract, not by unit tests.

## Why It Matters

Local LLM output quality is fragile. Prompt changes that seem harmless can break structured output. The spike acts as an empirical gate; the verbatim copy preserves the gate's validity.

## Source

gemma4 spike (.agents/rpi/spike-2026-04-11-gemma4-output-shape.md) → summarizer.go PromptTemplate.

---

# Learning: 40-LOC cobra wiring budget forced clean architecture

## What We Learned

The plan capped forge.go additions at ≤40 LOC (because forge.go was already 807 LOC). This forced all logic into `cli/internal/llm/` as a self-contained package, with forge.go reduced to a thin dispatcher (31 LOC actual). The constraint produced better separation than organic development would have — the llm package is fully testable without cobra, and forge.go has zero business logic.

## Why It Matters

LOC budgets on "hot" files (already large) are a cheap constraint that produces good architecture. Worth adding to plan templates for any file >500 LOC.

## Source

Council S6.6 finding + plan W4 constraint. Delivered at 31/40 LOC.

---

# Learning: Generator interface enables test injection without httptest

## What We Learned

Defining a narrow `Generator` interface (Generate, Digest, ContextBudget, ModelName) let tests use a `fakeLLM` struct for unit-level summarizer/session-writer tests, while the L2 integration test used `httptest.NewServer` to mock the full ollama HTTP protocol. Both test levels run in <1s without a real LLM daemon. The `SetGeneratorFactory` injection point on `Tier1Options` made the L2 test clean without exposing internals.

## Why It Matters

Local LLM dependencies are inherently slow and flaky (cold start 70s, daemon may be offline). Interface-based injection lets CI run the full test suite without any LLM infrastructure.

## Source

ollama_client.go Generator interface, summarizer_test.go fakeLLM, forge_tier1_test.go factory injection.
