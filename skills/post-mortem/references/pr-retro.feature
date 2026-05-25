# Executable spec for /post-mortem --scope=pr — learning from PR outcomes (BC3 Loop).
# /post-mortem --scope=pr turns a merged or rejected pull request into durable lessons: it categorizes
# the feedback, extracts success patterns when merged and failure patterns when rejected,
# and writes a learning. Hexagon: supporting; consumes: PR feedback + outcome; produces:
# .agents/learnings/YYYY-MM-DD-pr-*.md. (soc-qk4b)

Feature: PR-retro extracts durable lessons from a PR outcome
  As an agent compounding review feedback
  I want each PR's outcome turned into a categorized lesson
  So that future PRs avoid past rejections and repeat what got merged

  Background:
    Given a pull request that was merged or rejected, with its feedback

  Scenario: Feedback is categorized by type
    When /post-mortem --scope=pr runs
    Then it categorizes the feedback (positive, requested changes, rejection reasons)

  Scenario: Outcome selects success or failure patterns
    When the PR was merged
    Then it extracts success patterns
    And when the PR was rejected it extracts failure patterns

  Scenario: A learning is written to the learnings surface
    When the retro completes
    Then it writes a dated PR learning under .agents/learnings/
