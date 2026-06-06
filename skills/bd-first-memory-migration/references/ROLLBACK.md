# Reversibility / rollback contract

> Scaffold stub — authored by beads ag-5u50.9 (plan) and ag-5u50.26 (test).

Every destructive Phase-3 action MUST be reversible within a grace period:
backup location, restore steps, and a verifying rollback test (bead .26) that gates the real retire (Gate B).
