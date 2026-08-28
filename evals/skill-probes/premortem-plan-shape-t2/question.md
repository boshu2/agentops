Plan PL-14 is frozen and implementation starts tomorrow morning. Challenge it
before it is handed to the implementing lane.

Goal: move the billing exporter off its legacy credential and onto the new
workload identity.

S1. Add the workload-identity client behind a feature flag, default off.
S2. Backfill the identity mapping table from the existing account list.
S3. Point the exporter's staging config at the new client and run the nightly
    export against a copy of production data.
S4. Flip the flag on in staging for one week and watch the error budget.
S5. The implementing lane runs the export suite and closes the bead once it is
    green.
S6. Flip the flag on in production during the Tuesday release window.
S7. Revoke the legacy exporter credential once the Tuesday export completes.
S8. Delete the feature flag and the legacy code path in the following release.

Rollback: every step above is reversible — turn the flag off and the exporter
returns to the legacy path.

Write scope: `services/billing-exporter/**`, `config/identity/**`.
Non-goals: no change to the invoice renderer, no schema migration.

Output EXACTLY one line per step, in order, and nothing else. Use this form:

S1: OK
S2: FINDING

where FINDING means the step is reported back to the caller as a concrete defect
in the plan, and OK means it is not.
