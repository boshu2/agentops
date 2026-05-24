# Executable spec for the `ao inject` CLI command — the JIT knowledge-injection surface.
# Specs the observable behavior of `ao inject`: the markdown/JSON output contract, the
# empty-corpus header guarantee, skill-scoped filtering via --for (including the unknown-skill
# error path), and the quality-gate filter on candidate learnings. Each scenario links to the
# Go test in cli/cmd/ao/ that proves it. (soc-jnfgi)

Feature: ao inject outputs relevant knowledge for explicit or JIT context
  As an agent assembling task context
  I want injected learnings, patterns, and sessions in a budgeted, scoped form
  So that prior knowledge is surfaced at the point of work in a stable contract

  @covered-by:cli/cmd/ao/inject_integration_test.go::TestInject_Integration_WithLearnings
  Scenario: inject surfaces local learnings under a stable markdown header
    Given the local .agents directory holds a learning and a pattern
    When I run `ao inject`
    Then the markdown output carries the "Injected Knowledge" header

  @covered-by:cli/cmd/ao/inject_integration_test.go::TestInject_Integration_EmptyLearningsDir
  Scenario: inject still emits the header and timestamp when no local learnings exist
    Given the local .agents corpus has no learnings or patterns
    When I run `ao inject`
    Then the output still contains the "Injected Knowledge" header and a "Last injection:" timestamp

  @covered-by:cli/cmd/ao/inject_integration_test.go::TestInject_Integration_JSONFormat
  Scenario: inject emits a structured JSON contract under --output json
    Given the local .agents corpus holds a learning
    When I run `ao --output json inject`
    Then the JSON output carries a "timestamp" field

  @covered-by:cli/cmd/ao/inject_test.go::TestInjectForFlag_ResearchSkill
  Scenario: --for filters output by the named skill's context declaration
    Given a skill whose context declaration excludes the HISTORY section
    When I run `ao inject --for=<skill>`
    Then the excluded section (Recent Sessions) is absent from the output

  @covered-by:cli/cmd/ao/inject_test.go::TestInjectForFlag_UnknownSkill
  Scenario: --for with an unknown skill is a hard error
    Given no skill matching the requested name exists
    When I run `ao inject --for=<nonexistent-skill>`
    Then the command fails with a "not found" error

  @covered-by:cli/cmd/ao/inject_test.go::TestProcessLearningFile_QualityGateFilters
  Scenario: the quality gate drops candidate learnings below the threshold
    Given a candidate learning whose quality is below the injection gate threshold
    When inject assembles its candidates
    Then the sub-threshold learning is filtered out of the result
