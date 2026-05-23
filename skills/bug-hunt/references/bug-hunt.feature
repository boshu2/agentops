# Executable spec for the /bug-hunt skill — root-cause investigation (domain role).
# /bug-hunt finds the ACTUAL cause of a bug (not the symptom) through a 4-phase
# investigation, or proactively audits a scope for hidden bugs. It writes a cited
# investigation artifact. Hexagon: domain; consumes beads + standards; produces
# .agents/research/YYYY-MM-DD-bug-*.md. (soc-qk4b)

Feature: Bug-hunt finds root cause and designs a complete fix
  As the bug investigator
  I want the real root cause located and a complete fix designed
  So that the bug is fixed at its source, not patched at the symptom

  Scenario: investigation mode runs the 4-phase structure
    When /bug-hunt runs on a known symptom
    Then it works through Root Cause → Pattern → Hypothesis → Fix
    And the root cause is located at a specific file:line (and originating commit when known)

  Scenario: the fix addresses the cause, not the symptom
    When the investigation produces a fix
    Then the fix targets the root cause
    And it is not a symptom-level patch that leaves the cause in place

  Scenario: audit mode sweeps a scope for hidden bugs
    When /bug-hunt --audit runs on a scope
    Then it proactively sweeps for latent bugs before they surface

  Scenario: the investigation is written as a cited artifact
    When the investigation completes
    Then findings are written to .agents/research/YYYY-MM-DD-bug-*.md with file:line evidence
