# Executable spec for the /discovery skill — front-of-loop intent densifier (BC3 Loop).
# /discovery runs the artifact-first research → plan DAG and hands dense intent across
# the plan_slices port, producing an execution packet — it never inlines the Plan
# decomposition in its own prose. Promoted from the inline Feature block in SKILL.md.
# (soc-qk4b.2)

Feature: Discovery hands dense intent to planning
  As the front of the loop
  I want research and design densified and handed cleanly to Plan
  So that planning receives artifact links + density fields, not re-derived prose

  Scenario: Discovery delegates to Plan
    Given Discovery has a goal, research path, and design or brainstorm evidence
    When it crosses the `plan_slices` port
    Then it sends density fields and artifact links
    And it does not inline the Plan decomposition in Discovery prose

  Scenario: Discovery produces a durable execution packet
    When the discovery DAG completes
    Then it writes a JSON execution packet on disk for the next loop phase
    And the packet carries the goal, research, and design artifact references
