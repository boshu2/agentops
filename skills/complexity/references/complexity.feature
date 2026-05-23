# Executable spec for the /complexity skill — refactor-hotspot finder (domain role).
# /complexity analyzes code complexity and surfaces a FOCUSED set of refactor hotspots ranked
# by cyclomatic complexity, rather than dumping every function. With no path it scopes to recent
# changes. Hexagon: domain; consumes doc + standards; produces stdout; shared-kernel with
# standards. (soc-qk4b)

Feature: Complexity finds focused refactor hotspots
  As the refactor-targeting analyzer
  I want the most complex functions surfaced and ranked
  So that refactoring effort goes where complexity is highest

  Scenario: a path is analyzed for complexity hotspots
    When /complexity runs on a path
    Then it reports functions ranked by cyclomatic complexity (highest first)

  Scenario: no path scopes to recent changes
    When /complexity runs without a path argument
    Then it scopes the analysis to recently changed source files

  Scenario: the output is a focused hotspot list, not a full dump
    When the analysis completes
    Then it surfaces a focused set of refactor targets, not every function in the tree
