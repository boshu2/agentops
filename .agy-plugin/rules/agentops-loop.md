# AgentOps Loop Rule

Use `br` plus git as the state store. Workers propose evidence, validators
verify in a separate context, and only the orchestrator closes a bead. Evidence
must cite commands and outputs, and commits must stage only the files in the
claimed scope.
