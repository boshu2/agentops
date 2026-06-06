# Task: removed-job assertion pre-flight gate

Lesson from a real incident: when you delete a CI job, surviving files that *assert
the job exists* (eval fixtures, tests, docs) break the delete-it PR. The discipline is
to grep the corpus for assertions before removing a named thing.

Write an executable script `check-removed-job-assertions.sh` in the current directory.
Contract: `check-removed-job-assertions.sh <job-id> <search-root>`
- exit **1** if `<job-id>` is still referenced by any file under `<search-root>`
  (print the offending file path(s) so the user can fix them first),
- exit **0** if no surviving reference remains.

Just write the script. Do not explain.
