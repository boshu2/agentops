# Audit Checks Reference (Pass 2 — 8 NEW)

> The 8 checks Pass-2 runs beyond `heal-skill`. Each row defines: detection logic, accepted forms, severity, and the PRODUCT.md anchor that motivates it.
> Auditor must validate against AgentOps' existing valid skills BEFORE shipping a check (per finding `f-2026-05-06-auditor-checks-must-fit-host-conventions`).
> Executable values are loaded from the authoritative `repo-runtime` profile in
> `skills/skill-builder/references/skill-conformance-profiles.yaml`; examples
> here are explanatory, not a second ledger.

## Severity ladder

| Severity | Effect on verdict | Examples |
|----------|-------------------|----------|
| FAIL | Aggregate verdict = FAIL | Missing output spec |
| WARN | Aggregate verdict ≥ WARN (unless any FAIL upgrades) | Missing rationale on constraints; no checkpoints; missing triggers in description |
| n/a | Skipped — does not affect verdict | references-modularization on a 200-line skill |
| pass | No effect | (default) |

## Checks

### 1. `description-has-triggers` (WARN)

> Severity downgraded from FAIL to WARN after pilot test (2026-05-06). Existing AgentOps skills (`heal-skill`, `forge`, `council`) use single-line descriptions without triggers — a real but cosmetic gap. WARN surfaces it without blocking CI. Re-promote to FAIL after pilot refactor (I9) addresses the gap.

**Why:** Pillar #6 (knowledge flywheel) — searchability requires structured description. AgentOps' established convention is single-line `description: '...'`; the auditor must NOT false-fail valid existing artifacts (per premortem F1).

**Accepted forms (any one passes):**

1. **Block marker** — a block-scalar description containing a profile marker:
   ```yaml
   description: |
     Audit a skill...
     **Use when:** ...
   ```
2. **Inline marker** — a single-line description containing a profile marker:
   ```
   description: 'Audit skill quality. Triggers: "audit skill".'
   ```
3. **Metadata list** — an array meeting the profile's minimum cardinality:
   ```yaml
   metadata:
     triggers: ["audit skill", "skill quality review", "is this skill ready"]
   ```

**Detection:** `audit.sh` consumes the validated form IDs emitted by
`conformance_profile.py`; markers found only in the Markdown body do not count.

```bash
python3 skills/skill-builder/scripts/conformance_profile.py \
  --repo-root . --audit-tsv skills/<name>/SKILL.md
```

### 2. `constraints-frontloaded` (WARN)

**Why:** Operational Principle #6 (atomic changes compose) — constraint visibility prevents large rework.

**Detection:** First H2 (`^## `) within 80 lines after closing frontmatter `---` matches `Constraints` or contains `⚠️`.

```bash
check_constraints_frontloaded() {
  local skill_md="$1"
  awk '
    BEGIN{n=0; i=0}
    /^---$/{n++; if (n==2) {start=NR}; next}
    n==2 {
      i++
      if (i > 80) exit 1
      if (/^## .*([Cc]onstraints|⚠️)/) exit 0
    }
    END{exit 1}
  ' "$skill_md"
}
```

### 3. `rationale-present` (WARN)

**Why:** Operational Principle #1 (agents are ephemeral; system carries state) — rationale must be inside the artifact.

**Detection:** Within the first H2 matching "Constraints", at least 50% of bullet items contain a rationale token (`why`, `because`, `this matters`, `to prevent`, `motivation:`, `rationale:`).

```bash
check_rationale_present() {
  local skill_md="$1"
  awk '
    BEGIN{IGNORECASE=1; in_constraints=0; bullets=0; with_why=0}
    /^## .*([Cc]onstraints|⚠️)/{in_constraints=1; next}
    in_constraints && /^## /{exit}
    in_constraints && /^[ ]*[-*] /{
      bullets++
      if (/why|because|this matters|to prevent|rationale:|motivation:/) with_why++
    }
    END{
      if (bullets == 0) exit 0   # no constraints to check, n/a → treat as pass
      exit (with_why * 2 >= bullets ? 0 : 1)
    }
  ' "$skill_md"
}
```

### 4. `verification-checkpoints` (WARN)

**Why:** Operational Principle #5 (two-tier execution) — checkpoints prevent worker drift between phases.

**Detection:** If skill has multi-phase Workflow/Methodology (>=2 H3 phases under `## Workflow` or `## Methodology` or `## Process`), then body should contain `Checkpoint`, `confirm`, or `Wait for` markers.

```bash
check_verification_checkpoints() {
  local skill_md="$1"
  local phases checkpoints
  phases=$(awk '/^## (Workflow|Methodology|Process)/{in_w=1; next} in_w && /^## /{exit} in_w && /^### /{n++} END{print n+0}' "$skill_md")
  if (( phases < 2 )); then return 0; fi   # not multi-phase → n/a
  checkpoints=$(grep -cE '\*\*Checkpoint:|confirm before|Wait for' "$skill_md" || true)
  (( checkpoints >= 1 ))
}
```

### 5. `output-spec-explicit` (FAIL)

**Why:** Pillar #4 (Kubernetes control loops) — declared state must be machine-readable.

**Detection:** One declared output section must supply every profile-required
component: artifact path, filename convention, serialization/schema format,
validator command, and downstream handoff.

```bash
check_output_spec_explicit() { [[ "$OUTPUT_COMPLETE" == "true" ]]; }
```

### 6. `quality-rubric` (WARN)

**Why:** Operational Principle #3 (context quality determines output quality).

**Detection:** Has `## Quality` / `## Checklist` / `## Rubric` / `## Best Practices` H2 with at least 3 bullet items.

```bash
check_quality_rubric() {
  local skill_md="$1"
  awk '
    BEGIN{in_q=0; bullets=0}
    /^## (Quality|Checklist|Rubric|Best Practices)/{in_q=1; next}
    in_q && /^## /{exit}
    in_q && /^[ ]*[-*] /{bullets++}
    END{exit (bullets >= 3 ? 0 : 1)}
  ' "$skill_md"
}
```

### 7. `references-modularization` (WARN)

**Why:** finding `f-2026-05-01-025` (SKILL.md churn budget — every Skill() invocation reloads 5-15KB).

**Detection:** A repo-runtime kernel passes through 250 lines; line 251 receives
the profile-declared finding regardless of whether references exist.

```bash
check_references_modularization() { (( PROFILE_LINE_COUNT <= KERNEL_MAX_LINES )); }
```

### 8. `trigger-clarity` (WARN)

> Severity downgraded from FAIL to WARN after pilot test (2026-05-06). Same rationale as #1 — gap is real but cosmetic. Re-promote after pilot refactor.

**Why:** Operational Principle #1 (agents are ephemeral) — invocation criteria must be in artifact.

**Detection:** The frontmatter `description` value contains a profile-accepted
marker. Unlike `description-has-triggers`, a metadata-only trigger list does not
satisfy this check.

```bash
check_trigger_clarity() {
  local skill_md="$1"
  # Extract description block (frontmatter -> next key OR end of frontmatter)
  awk '
    BEGIN{n=0; in_desc=0}
    /^---$/{n++; if (n==2) exit; next}
    n==1 && /^description:/{in_desc=1; print; next}
    n==1 && in_desc && /^[a-z_-]+:/{in_desc=0}
    n==1 && in_desc {print}
  ' "$skill_md" | grep -qE '(Use when:|Triggers:)'
}
```

## PRODUCT.md alignment summary

| Check | PRODUCT.md anchor |
|-------|-------------------|
| `description-has-triggers` | Pillar #6 (knowledge flywheel) |
| `constraints-frontloaded` | Operational Principle #6 (atomic changes) |
| `rationale-present` | Operational Principle #1 (agents ephemeral) |
| `verification-checkpoints` | Operational Principle #5 (two-tier execution) |
| `output-spec-explicit` | Pillar #4 (Kubernetes control loops) |
| `quality-rubric` | Operational Principle #3 (context quality) |
| `references-modularization` | Finding `f-2026-05-01-025` (churn budget) |
| `trigger-clarity` | Operational Principle #1 (agents ephemeral) |

## Validation methodology (must run before shipping check)

Per finding `f-2026-05-06-auditor-checks-must-fit-host-conventions`: each check must pass on at least 5 existing AgentOps skills (sample: heal-skill, forge, council, plan, validation). If pass rate <80% on the existing population, the check is over-narrow and must be broadened OR the skills must be refactored — but the refactor decision is for the pilot wave (I9), not the auditor design.
