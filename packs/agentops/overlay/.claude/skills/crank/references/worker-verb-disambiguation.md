# Worker Verb Disambiguation

Ambiguous verbs cause workers to implement the wrong operation. Use explicit instructions:

| Verb | Clarified Instruction |
|------|----------------------|
| "Extract (file)" | "Remove from source AND write to new file. Source line count must decrease." |
| "Extract (spec)" | "Generate a specification document from issue/task metadata. Source is unchanged." |
| "Remove" | "Delete the content. Verify it no longer appears in the file." |
| "Update" | "Change [specific field] from [old] to [new]." |
| "Consolidate" | "Merge from [A, B] into [C]. Delete [A, B] after merge." |

Include `wc -l` assertions in task metadata when content moves between files.
