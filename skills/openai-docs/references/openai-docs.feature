# Executable spec for the /openai-docs skill — official-docs grounding (driven-adapter).
# /openai-docs answers OpenAI-API questions from the OFFICIAL docs (via the openaiDeveloperDocs
# MCP search/fetch/list tools), quoting exact sections rather than recalling from memory.
# Hexagon: driven-adapter; consumes external-api. (soc-qk4b)

Feature: Openai-docs grounds answers in official OpenAI documentation
  As the OpenAI-docs grounding skill
  I want answers pulled from the official docs and quoted from exact sections
  So that OpenAI API details are accurate and current, not hallucinated from memory

  Scenario: a query is answered from searched docs
    When /openai-docs answers an OpenAI-API question
    Then it searches the official OpenAI docs for the most relevant pages
    And it fetches the exact section to quote or paraphrase from

  Scenario: answers are doc-grounded, not from memory
    When the relevant doc section is available
    Then the answer is grounded in that fetched section, not recalled API details

  Scenario: browsing is the fallback, not the default
    When there is no clear query
    Then listing docs is used only to discover pages, not as the primary path
