# Executable spec for the /update skill — skill sync + install (BC5 Runtime).
# /update pulls the latest skills from the repo and installs them globally across
# every agent runtime in one command, so all harnesses run the same current corpus.
# Hexagon: supporting; consumes: the repo's skills/ source of truth; produces:
# installed skill copies under the agent skills directories. (soc-qk4b)

Feature: Update syncs the latest skills across every agent runtime
  As an operator keeping agents current
  I want one command to pull and install the latest skills everywhere
  So that all harnesses share the same up-to-date skill corpus

  Background:
    Given a local AgentOps repo and one or more agent runtimes installed

  Scenario: Latest skills are pulled from the repo source of truth
    When /update runs
    Then it fetches the latest skills from the repository

  Scenario: Skills install globally across agent runtimes
    When the pull completes
    Then the skills are installed into the agent skills directories for each runtime

  Scenario: Re-running update is idempotent
    When /update runs again with no upstream change
    Then the installed copies are unchanged and no spurious diffs are produced
