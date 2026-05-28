# Executable spec for the /system-tuning skill — safe responsiveness triage (BC5 Runtime).
# /system-tuning restores a sluggish dev box through a safe, ordered process cleanup —
# diagnose first, then clean zombies and exited sessions, kill stuck children, fix confused
# parents, renice survivors, and verify — without nuking work in flight. Hexagon: supporting;
# consumes: process/system state; produces: a triage report + protected-process warnings. (soc-qk4b)

Feature: System-tuning restores responsiveness without nuking work in flight
  As an operator on a sluggish dev box
  I want an ordered, safe cleanup that spares live work
  So that responsiveness returns without losing in-flight sessions

  Background:
    Given a sluggish development host with mixed live and stale processes

  Scenario: Diagnosis happens before any process is touched
    When /system-tuning runs
    Then it diagnoses the load before killing anything

  Scenario: Cleanup follows a safe ordered hierarchy
    When cleanup proceeds
    Then it cleans zombies and exited sessions, kills stuck children, fixes confused parents, then renices survivors

  Scenario: Protected processes are warned, not killed
    When a protected or in-flight process is encountered
    Then it is spared and a warning is emitted rather than terminating it

  Scenario: The result is verified and reported
    When cleanup completes
    Then it verifies responsiveness and emits a triage report
