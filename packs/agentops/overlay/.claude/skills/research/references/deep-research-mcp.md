# Deep Research with MCP Integration

> Multi-source research using MCP servers (firecrawl, exa, context7) for comprehensive exploration beyond basic web search.

## When to Use

- Topic requires authoritative external sources (not just codebase exploration)
- Research question spans multiple domains or requires current data
- Basic `WebSearch` returns insufficient depth
- API documentation or technical specifications needed

## Research Pipeline

### Step 1: Decompose Topic into Sub-Questions

Break the research topic into 3-5 focused sub-questions:

```
Topic: "Impact of streaming APIs on agent architectures"
Sub-questions:
  1. What streaming API patterns exist today? (SSE, WebSocket, gRPC streams)
  2. How do major agent frameworks handle streaming? (LangChain, CrewAI, AutoGen)
  3. What are latency/throughput tradeoffs for streaming vs batch?
  4. What production deployment patterns exist for streaming agents?
  5. What's the state of streaming in Claude/OpenAI APIs?
```

### Step 2: Multi-Source Search (Per Sub-Question)

For each sub-question, search across available MCP sources:

```
# Primary: Structured web search (if firecrawl MCP connected)
mcp__firecrawl__search(query: "<sub-question keywords>", limit: 8)

# Secondary: Semantic web search (if exa MCP connected)
mcp__exa__web_search(query: "<sub-question keywords>", numResults: 8)
mcp__exa__web_search_advanced(query: "<keywords>", numResults: 5, startPublishedDate: "2025-01-01")

# Tertiary: Documentation lookup (if context7 MCP connected)
mcp__context7__resolve_library_id(libraryName: "<library>")
mcp__context7__get_library_docs(context7CompatibleLibraryID: "<id>")

# Fallback: Standard web search (always available)
WebSearch(query: "<sub-question>")
```

**Search Strategy:**
- Use 2-3 keyword variations per sub-question
- Mix general queries with news-focused queries
- Aim for 15-30 unique sources total across all sub-questions
- Prioritize: official docs > academic > reputable news > blogs > forums

### Step 3: Deep-Read Key Sources (3-5 URLs)

For the most promising results, fetch full content:

```
# Full page scrape (if firecrawl connected)
mcp__firecrawl__scrape(url: "<url>")

# Semantic content extraction (if exa connected)
mcp__exa__crawling(url: "<url>", tokensNum: 5000)

# Fallback
WebFetch(url: "<url>")
```

### Step 4: Parallel Agent Research (Optional)

For broad topics, spawn parallel research agents:

```
Agent 1: Sub-questions 1-2 (technical patterns)
Agent 2: Sub-questions 3-4 (production deployment)
Agent 3: Sub-question 5 (API state-of-art)
```

Main session synthesizes all agent findings into unified report.

### Step 5: Synthesize Report

```markdown
# Research: <Topic>

**Sources:** <N> | **Confidence:** High/Medium/Low | **Date:** <YYYY-MM-DD>

## Executive Summary
<3-5 sentences>

## 1. <Theme from Sub-Question 1>
<Findings with inline citations>
- Key point (Source Name, with URL citation)

## Key Takeaways
- <Actionable insight 1>
- <Actionable insight 2>

## Knowledge Gaps
- <What we couldn't find>
- <What needs verification>

## Sources
1. Source Title — one-line summary (with URL)
```

## Quality Rules

1. **Every claim needs a source** — no unsourced assertions
2. **Cross-reference:** If only one source says it, flag as unverified
3. **Prefer recent sources** (last 12 months) for fast-moving topics
4. **Acknowledge gaps explicitly** — "insufficient data found" > hallucination
5. **Separate fact from inference** — label estimates, projections, opinions
6. **Check MCP availability first** — gracefully degrade to WebSearch if MCPs not connected

## MCP Detection

Before attempting MCP-based search, check availability:

```
# Check which MCPs are available in the current session
# If firecrawl: use firecrawl_search + firecrawl_scrape
# If exa: use web_search_exa + crawling_exa
# If context7: use for library documentation
# If none: fall back to WebSearch + WebFetch
```

Log which sources were used for traceability in the report's Methodology section.

## Integration with /research Skill

This reference extends the research skill's Step 3 (Launch Explore Agent) with MCP-first search patterns. When the explore agent's Tier 6 (External Docs) triggers, use this pipeline instead of basic WebSearch.

The iterative retrieval pattern (`references/iterative-retrieval.md`) applies here too: score MCP results for relevance, extract new search terms, and refine across cycles.
