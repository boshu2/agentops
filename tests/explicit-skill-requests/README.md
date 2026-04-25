# Explicit Skill Request Tests

Validates that natural-language trigger phrases correctly resolve to the expected skill. Each `.txt` file in `prompts/` contains a phrase that should match the skill named by the filename. The test runner feeds each prompt through the skill-matching logic and asserts the correct skill is selected, catching regressions in trigger phrase routing.

Prompt filenames must map to real `skills/<name>/SKILL.md` and `skills-codex/<name>/SKILL.md` artifacts. Do not add legacy aliases here; use the current skill name that users should invoke.

## Running

```bash
bash tests/explicit-skill-requests/run-all.sh
```
