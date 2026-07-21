#!/usr/bin/env python3
"""Probe real pinned Beads against one owned disposable Dolt server.

The probe does not start Gas City or exercise its events.  The GC entry in the
receipt is lock provenance only; this executable attests the supplied Beads and
Dolt binaries and the observed Beads/Dolt behavior.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import sys
import tempfile
import time
from typing import Any

from jsonschema import Draft202012Validator, FormatChecker


ROOT = Path(__file__).resolve().parents[2]
LOCK = ROOT / "deploy/gc/toolchain.lock.json"
SCHEMAS = ROOT / "packs/agentops-factory/assets/schemas"
BD_COMMIT = "8e4e59d39f3459a43cf21a3236a13eca4dd874f7"
SAFE_METADATA_KEY_BYTES = 128
SAFE_METADATA_VALUE_BYTES = 4096
OWNED_SERVER: subprocess.Popen[str] | None = None


class ProbeError(RuntimeError):
    pass


def command(*argv: str, cwd: Path | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(argv, cwd=cwd, text=True, capture_output=True, check=False, timeout=30)
    if check and result.returncode:
        raise ProbeError(f"{' '.join(argv)} failed ({result.returncode}): {result.stderr.strip()}")
    return result


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def canonical_write(path: Path, value: dict[str, Any]) -> None:
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as listener:
        listener.bind(("127.0.0.1", 0))
        return int(listener.getsockname()[1])


def wait_for_server(port: int) -> None:
    deadline = time.monotonic() + 12
    while time.monotonic() < deadline:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as probe:
            probe.settimeout(0.25)
            if probe.connect_ex(("127.0.0.1", port)) == 0:
                return
        time.sleep(0.05)
    raise ProbeError(f"private Dolt server did not listen on {port}")


def start_server(dolt: str, data_dir: Path, port: int, log: Path) -> subprocess.Popen[str]:
    data_dir.mkdir(parents=True, exist_ok=True)
    process = subprocess.Popen([dolt, "sql-server", "--data-dir", str(data_dir), "--host", "127.0.0.1", "--port", str(port), "--loglevel", "error"], text=True, stdout=log.open("a", encoding="utf-8"), stderr=subprocess.STDOUT)
    try:
        wait_for_server(port)
    except ProbeError as exc:
        if process.poll() is not None:
            raise ProbeError(f"{exc}; Dolt exited {process.returncode}: {log.read_text(encoding='utf-8')[-1200:]}") from exc
        raise
    return process


def stop_server(process: subprocess.Popen[str]) -> bool:
    if process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=8)
        except subprocess.TimeoutExpired:
            process.kill(); process.wait(timeout=8)
    try:
        os.kill(process.pid, 0)
    except ProcessLookupError:
        return True
    return False


def owned_signal_handler(signum: int, _frame: Any) -> None:
    global OWNED_SERVER
    if OWNED_SERVER is not None:
        stop_server(OWNED_SERVER)
    raise KeyboardInterrupt(f"probe interrupted by signal {signum}")


def bd_call(bd: str, repo: Path, *args: str, actor: str | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    argv = [bd, "-C", str(repo)]
    if actor:
        argv.extend(["--actor", actor])
    return command(*argv, *args, check=check)


def issue(bd: str, repo: Path, identifier: str) -> dict[str, Any]:
    result = bd_call(bd, repo, "show", identifier, "--json")
    value = json.loads(result.stdout)
    if isinstance(value, list):
        if len(value) != 1:
            raise ProbeError(f"expected one issue for {identifier}, got {len(value)}")
        value = value[0]
    if not isinstance(value, dict):
        raise ProbeError(f"bd show did not return an object for {identifier}")
    return value


def issue_or_none(bd: str, repo: Path, identifier: str) -> dict[str, Any] | None:
    result = bd_call(bd, repo, "show", identifier, "--json", check=False)
    if result.returncode:
        return None
    value = json.loads(result.stdout)
    return value[0] if isinstance(value, list) and len(value) == 1 else value


def metadata(value: dict[str, Any]) -> dict[str, str]:
    raw = value.get("metadata") or {}
    if not isinstance(raw, dict):
        raise ProbeError("bd show metadata is not an object")
    return {str(key): str(item) for key, item in raw.items()}


def create(bd: str, repo: Path, identifier: str, title: str, *, external_ref: str | None = None) -> str:
    args = ["create", title, "--id", identifier, "--force", "--silent"]
    if external_ref:
        args.extend(["--external-ref", external_ref])
    result = bd_call(bd, repo, *args)
    if result.stdout.strip() != identifier:
        raise ProbeError(f"create returned {result.stdout.strip()!r}, expected {identifier!r}")
    return identifier


def schema_validator(name: str) -> Draft202012Validator:
    schema = json.loads((SCHEMAS / name).read_text(encoding="utf-8"))
    Draft202012Validator.check_schema(schema)
    return Draft202012Validator(schema, format_checker=FormatChecker())


VALIDATORS = {name: schema_validator(name) for name in ("handoff-prepared.v1.schema.json", "delivery.v1.schema.json", "handoff-committed.v1.schema.json")}


def validate_payload(schema: str, payload: dict[str, Any]) -> None:
    errors = sorted(VALIDATORS[schema].iter_errors(payload), key=lambda error: list(error.path))
    if errors:
        raise ProbeError(f"{schema} payload is invalid: {errors[0].message}")


def load_valid(path: Path, schema: str) -> tuple[dict[str, Any] | None, str | None]:
    if not path.is_file():
        return None, None
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(value, dict):
            return None, "non_object"
        validate_payload(schema, value)
        return value, None
    except (OSError, json.JSONDecodeError, ProbeError):
        return None, "invalid"


def external_ref(handoff_id: str, epoch: int) -> str:
    return f"handoff:{handoff_id}:epoch:{epoch}"


def child_identity_matches(prepared: dict[str, Any], child: dict[str, Any] | None) -> bool:
    if child is None:
        return False
    expected_ref = external_ref(str(prepared["handoff_id"]), int(prepared["epoch"]))
    return (child.get("id") == prepared["expected_delivery_bead_id"] and child.get("external_ref") == prepared["expected_external_ref"] == expected_ref)


def create_or_discover_child(bd: str, repo: Path, prepared: dict[str, Any]) -> tuple[dict[str, Any], bool]:
    """Discover by deterministic ID first; create only when absent, then reread."""
    child = issue_or_none(bd, repo, str(prepared["expected_delivery_bead_id"]))
    if child is not None:
        return child, True
    create(bd, repo, str(prepared["expected_delivery_bead_id"]), "non-routable delivery successor", external_ref=str(prepared["expected_external_ref"]))
    child = issue(bd, repo, str(prepared["expected_delivery_bead_id"]))
    return child, False


def handoff_selection_observed(cuts: list[dict[str, Any]], negative_child_ok: bool, late_terminal_ok: bool) -> bool:
    return all(row["exactly_one_delivery_bead"] and row["converged"] and row["precommit_authorizations"] == 0 and all(row["schema_valid_artifacts"].values()) for row in cuts) and negative_child_ok and late_terminal_ok


def delivery_payload(prepared: dict[str, Any], publication: str) -> dict[str, Any]:
    return {
        "schema_version": "delivery.v1", "kind": "delivery", "handoff_id": prepared["handoff_id"],
        "semantic_bead_id": prepared["semantic_bead_id"], "semantic_terminal_ref": prepared["semantic_terminal_ref"],
        "admission_certificate_digest": prepared["admission_certificate_digest"], "delivery_bead_id": prepared["expected_delivery_bead_id"],
        "external_ref": prepared["expected_external_ref"], "epoch": prepared["epoch"], "predecessor_receipt_digest": None,
        "mode": prepared["mode"], "state": prepared["state"], "publication": publication, "deadline": prepared["deadline"],
        "effect_gate": None, "successor_bead_id": None,
    }


def payload_matches_prepared(prepared: dict[str, Any], delivery: dict[str, Any], publication: str) -> bool:
    expected = delivery_payload(prepared, publication)
    return all(delivery.get(key) == value for key, value in expected.items())


def terminal_status(bd: str, repo: Path, prepared: dict[str, Any]) -> str:
    semantic = issue(bd, repo, str(prepared["semantic_bead_id"]))
    encoded = metadata(semantic).get("gc33.terminal")
    if semantic.get("status") != "closed":
        return "await_terminal"
    if not encoded:
        return "refuse_conflicting_terminal"
    try:
        terminal = json.loads(encoded)
    except json.JSONDecodeError:
        return "refuse_conflicting_terminal"
    return "matched" if terminal == {"verdict": "PASS", "certificate_digest": prepared["admission_certificate_digest"]} else "refuse_conflicting_terminal"


def reconcile_handoff(markers: Path, bd: str, repo: Path) -> dict[str, Any]:
    """Read filesystem plus real Beads once and perform at most one transition."""
    prepared, error = load_valid(markers / "prepared.json", "handoff-prepared.v1.schema.json")
    if error:
        return {"action": "refuse_malformed_prepared", "effect_authorized": False}
    if not prepared:
        return {"action": "refuse_missing_prepared", "effect_authorized": False}
    terminal = terminal_status(bd, repo, prepared)
    if terminal != "matched":
        return {"action": terminal, "effect_authorized": False}
    child = issue_or_none(bd, repo, str(prepared["expected_delivery_bead_id"]))
    if child is not None and not child_identity_matches(prepared, child):
        return {"action": "refuse_conflicting_delivery_identity", "effect_authorized": False}
    nonroutable, error = load_valid(markers / "delivery.nonroutable.json", "delivery.v1.schema.json")
    if error:
        return {"action": "refuse_malformed_nonroutable_delivery", "effect_authorized": False}
    if nonroutable and not payload_matches_prepared(prepared, nonroutable, "non_routable"):
        return {"action": "refuse_conflicting_nonroutable_delivery", "effect_authorized": False}
    if not nonroutable:
        child, discovered = create_or_discover_child(bd, repo, prepared)
        if not child_identity_matches(prepared, child):
            return {"action": "refuse_conflicting_delivery_identity", "effect_authorized": False}
        payload = delivery_payload(prepared, "non_routable")
        validate_payload("delivery.v1.schema.json", payload)
        canonical_write(markers / "delivery.nonroutable.json", payload)
        return {"action": "rediscovered_successor" if discovered else "created_successor", "effect_authorized": False}
    published, error = load_valid(markers / "delivery.published.json", "delivery.v1.schema.json")
    if error:
        return {"action": "refuse_malformed_published_delivery", "effect_authorized": False}
    if published and not payload_matches_prepared(prepared, published, "published"):
        return {"action": "refuse_conflicting_published_delivery", "effect_authorized": False}
    if not published:
        payload = delivery_payload(prepared, "published")
        validate_payload("delivery.v1.schema.json", payload)
        canonical_write(markers / "delivery.published.json", payload)
        return {"action": "published", "effect_authorized": False}
    committed, error = load_valid(markers / "committed.json", "handoff-committed.v1.schema.json")
    if error:
        return {"action": "refuse_malformed_committed", "effect_authorized": False}
    if not committed:
        payload = {"schema_version": "handoff-committed.v1", "handoff_id": prepared["handoff_id"], "prepared_digest": sha256(markers / "prepared.json"), "semantic_bead_id": prepared["semantic_bead_id"], "semantic_terminal_verdict": "PASS", "semantic_terminal_ref": prepared["semantic_terminal_ref"], "admission_certificate_digest": prepared["admission_certificate_digest"], "delivery_bead_id": prepared["expected_delivery_bead_id"], "delivery_payload_ref": "delivery.published.json", "delivery_payload_digest": sha256(markers / "delivery.published.json"), "mode": published["mode"], "state": published["state"], "deadline": published["deadline"], "committed_at": "2026-07-21T00:00:03Z"}
        validate_payload("handoff-committed.v1.schema.json", payload)
        canonical_write(markers / "committed.json", payload)
        return {"action": "committed", "effect_authorized": False}
    exact_commit = (committed.get("handoff_id") == prepared["handoff_id"] and committed.get("prepared_digest") == sha256(markers / "prepared.json") and committed.get("delivery_bead_id") == prepared["expected_delivery_bead_id"] and committed.get("delivery_payload_ref") == "delivery.published.json" and committed.get("delivery_payload_digest") == sha256(markers / "delivery.published.json") and committed.get("admission_certificate_digest") == prepared["admission_certificate_digest"] and committed.get("semantic_terminal_ref") == prepared["semantic_terminal_ref"] and committed.get("mode") == published["mode"] and committed.get("state") == published["state"] and committed.get("deadline") == published["deadline"])
    if not exact_commit:
        return {"action": "refuse_conflicting_committed", "effect_authorized": False}
    return {"action": "converged", "effect_authorized": True}


def terminalize_handoff(bd: str, repo: Path, prepared: dict[str, Any], certificate: str) -> None:
    terminal = json.dumps({"verdict": "PASS", "certificate_digest": certificate}, sort_keys=True)
    bd_call(bd, repo, "update", str(prepared["semantic_bead_id"]), "--set-metadata", f"gc33.terminal={terminal}")
    bd_call(bd, repo, "close", str(prepared["semantic_bead_id"]))


def lock_provenance() -> dict[str, Any]:
    lock = json.loads(LOCK.read_text(encoding="utf-8"))
    pair = next((item for item in lock["accepted_pairs"] if item["id"] == "gascity-v1.3.5-beads-v1.1.0"), None)
    expected_gc = {"repository": "https://github.com/gastownhall/gascity.git", "ref": "v1.3.5", "version": "1.3.5", "source_commit": "8ffc009ded781a2ada2077f3a29bd712b2def0bf"}
    expected_bd = {"repository": "https://github.com/steveyegge/beads.git", "ref": "v1.1.0", "version": "1.1.0", "source_commit": BD_COMMIT}
    if not pair or pair.get("status") != "qualified" or any(pair.get("gc", {}).get(key) != value for key, value in expected_gc.items()) or any(pair.get("bd", {}).get(key) != value for key, value in expected_bd.items()):
        raise ProbeError("toolchain lock lacks the qualified official GC 1.3.5 / Beads 1.1.0 pair")
    return {"path": str(LOCK.relative_to(ROOT)), "sha256": sha256(LOCK), "pair_id": pair["id"], "status": "LOCK_PROVENANCE_NOT_EXERCISED", "gc": expected_gc, "bd": expected_bd}


def verify_toolchain(bd: Path, dolt: Path) -> dict[str, Any]:
    if not bd.is_file() or not os.access(bd, os.X_OK) or not dolt.is_file() or not os.access(dolt, os.X_OK):
        raise ProbeError("--bd-bin and --dolt-bin must name executable files")
    provenance = lock_provenance()
    bd_version = command(str(bd), "--version").stdout.strip()
    expected_bd = provenance["bd"]
    if f"bd version {expected_bd['version']} ({expected_bd['source_commit'][:7]})" not in bd_version:
        raise ProbeError("bd runtime does not match the qualified Beads lock entry")
    return {"bd": {"path": str(bd), "sha256": sha256(bd), "version": bd_version, "source_commit": expected_bd["source_commit"]}, "dolt": {"path": str(dolt), "sha256": sha256(dolt), "version": command(str(dolt), "version").stdout.strip()}, "gc_lock_provenance": provenance, "gc_runtime": {"status": "NOT_EXERCISED", "reason": "Beads/Dolt-only probe; Gas City event wake disabled"}}


def prepared_payload(cut: int) -> dict[str, Any]:
    handoff_id = hashlib.sha256(f"GC33-2 handoff {cut}".encode()).hexdigest()
    return {"schema_version": "handoff-prepared.v1", "handoff_id": handoff_id, "semantic_bead_id": f"cap-semantic-{cut}", "semantic_terminal_ref": f"beads:cap-semantic-{cut}#gc33.terminal", "admission_certificate_ref": f"certificate:sha256:{'a' * 64}", "admission_certificate_digest": "a" * 64, "expected_delivery_bead_id": f"cap-delivery-{handoff_id[:20]}-e000001", "expected_external_ref": external_ref(handoff_id, 1), "epoch": 1, "mode": "auto", "state": "queued", "deadline": "2026-07-22T00:00:00Z", "prepared_at": "2026-07-21T00:00:00Z"}


def probe(output: Path, bd_path: Path, dolt_path: Path, trials: int) -> dict[str, Any]:
    global OWNED_SERVER
    if not 2 <= trials <= 8:
        raise ProbeError("--trials must be between 2 and 8")
    bd, dolt = str(bd_path.resolve()), str(dolt_path.resolve())
    runtime, properties, cleanup = verify_toolchain(Path(bd), Path(dolt)), {}, {"no_attributed_process_remains": False}
    with tempfile.TemporaryDirectory(prefix="agentops-gc-capability-") as temporary:
        root = Path(temporary); repo, data_dir, log = root / "repo", root / "dolt", root / "dolt.log"; repo.mkdir(); port = free_port()
        prior_term, prior_int = signal.getsignal(signal.SIGTERM), signal.getsignal(signal.SIGINT)
        signal.signal(signal.SIGTERM, owned_signal_handler); signal.signal(signal.SIGINT, owned_signal_handler)
        server = start_server(dolt, data_dir, port, log); OWNED_SERVER = server
        try:
            command(bd, "init", "--server", "--external", "--database", "capability", "--server-host", "127.0.0.1", "--server-port", str(port), "--prefix", "cap", "--non-interactive", "--skip-agents", "--skip-hooks", cwd=repo)
            claim_rows = []
            for index in range(1, trials + 1):
                identifier = "cap-claim-race" if index == 1 else f"cap-claim-race-{index}"
                create(bd, repo, identifier, f"claim race {index}")
                actors = ("claimant-a", "claimant-b")
                procs = [subprocess.Popen([bd, "-C", str(repo), "--actor", actor, "update", identifier, "--claim", "--json"], text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE) for actor in actors]
                [process.communicate(timeout=30) for process in procs]
                codes, winner = [process.returncode for process in procs], issue(bd, repo, identifier).get("assignee")
                successful_actors = [actor for actor, code in zip(actors, codes) if code == 0]
                expected = successful_actors[0] if len(successful_actors) == 1 else None
                claim_rows.append({"trial": index, "claimants": list(actors), "exit_codes": codes, "successful_claimants": len(successful_actors), "successful_actor": expected, "expected_final_assignee": expected, "final_assignee": winner, "observed": expected is not None and winner == expected})
            properties["claim_race_exclusive"] = {"observed": all(row["observed"] for row in claim_rows), "trial_count": trials, "trials": claim_rows}

            claim_id, winner = "cap-claim-race", claim_rows[0]["final_assignee"]
            bd_call(bd, repo, "update", claim_id, "--assignee", "reaper", "--status", "in_progress", actor="reaper")
            stale = bd_call(bd, repo, "update", claim_id, "--set-metadata", "gc33.stale=accepted", actor=str(winner), check=False)
            properties["stale_claimant_can_mutate"] = {"observed": stale.returncode == 0 and metadata(issue(bd, repo, claim_id)).get("gc33.stale") == "accepted", "returncode": stale.returncode}

            writer_id = create(bd, repo, "cap-metadata-race", "metadata race")
            writers = [subprocess.Popen([bd, "-C", str(repo), "--actor", actor, "update", writer_id, "--set-metadata", pair], text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE) for actor, pair in (("writer-a", "gc33.writer_a=1"), ("writer-b", "gc33.writer_b=1"))]
            [process.communicate(timeout=30) for process in writers]
            raced = metadata(issue(bd, repo, writer_id)); properties["metadata_race_visibility"] = {"result": "BOTH_WRITES_VISIBLE" if raced.get("gc33.writer_a") == "1" and raced.get("gc33.writer_b") == "1" else "WRITE_CLOBBER_OR_UNPROVEN", "writers": [process.returncode for process in writers], "metadata_cas": "NOT_EXPOSED_BY_CLI"}
            bd_call(bd, repo, "update", writer_id, "--set-metadata", "gc33.phase=prepared", "--set-metadata", "gc33.receipt=pair")
            pair = metadata(issue(bd, repo, writer_id)); properties["multifield_metadata_update"] = {"result": "OBSERVED_COMPLETE_PAIR" if pair.get("gc33.phase") == "prepared" and pair.get("gc33.receipt") == "pair" else "TORN_OR_UNPROVEN", "atomicity_claim": "sampled CLI call only; no CAS or claimant fence"}
            for key in ("gc33.kill_phase", "gc33.kill_receipt"): bd_call(bd, repo, "update", writer_id, "--unset-metadata", key)
            interrupted = subprocess.Popen([bd, "-C", str(repo), "update", writer_id, "--set-metadata", "gc33.kill_phase=prepared", "--set-metadata", "gc33.kill_receipt=pair"], text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE); time.sleep(0.002)
            if interrupted.poll() is None: interrupted.terminate()
            interrupted.communicate(timeout=30); kill_pair = (metadata(issue(bd, repo, writer_id)).get("gc33.kill_phase"), metadata(issue(bd, repo, writer_id)).get("gc33.kill_receipt")); properties["kill_mid_update_recovery"] = {"observed": kill_pair in {(None, None), ("prepared", "pair")}, "pair": list(kill_pair), "process_returncode": interrupted.returncode}
            boundary = bd_call(bd, repo, "update", writer_id, "--set-metadata", f"{'k' * SAFE_METADATA_KEY_BYTES}={'v' * SAFE_METADATA_VALUE_BYTES}", check=False); properties["tested_safe_metadata_envelope"] = {"key_bytes": SAFE_METADATA_KEY_BYTES, "value_bytes": SAFE_METADATA_VALUE_BYTES, "observed": boundary.returncode == 0, "returncode": boundary.returncode, "claim": "accepted safe envelope; not a maximum limit"}

            create_discover = prepared_payload(90); created, created_from_existing = create_or_discover_child(bd, repo, create_discover); rediscovered, rediscovered_from_existing = create_or_discover_child(bd, repo, create_discover); listed = json.loads(bd_call(bd, repo, "list", "--all", "--id", create_discover["expected_delivery_bead_id"], "--json").stdout); properties["deterministic_successor_create_or_discover"] = {"observed": not created_from_existing and rediscovered_from_existing and len(listed) == 1 and child_identity_matches(create_discover, created) and child_identity_matches(create_discover, rediscovered), "handoff_id": create_discover["handoff_id"], "epoch": 1, "external_ref": create_discover["expected_external_ref"], "successor_id": create_discover["expected_delivery_bead_id"], "same_id_count": len(listed), "created_after_absent_discovery": not created_from_existing, "positive_rediscovery": rediscovered_from_existing and child_identity_matches(create_discover, rediscovered), "same_identity_race": "not separately issued; deterministic reread is non-mutating"}

            bd_call(bd, repo, "close", create_discover["expected_delivery_bead_id"]); late = bd_call(bd, repo, "update", create_discover["expected_delivery_bead_id"], "--set-metadata", "gc33.late=accepted", check=False); properties["terminal_semantic_immutability"] = {"result": "NOT_ENFORCED_BY_BD" if late.returncode == 0 else "CLI_REJECTED", "late_duplicate_returncode": late.returncode}
            before = bd_call(bd, repo, "list", "--all", "--json").stdout; cleanup["first_server_stopped"] = stop_server(server); server = start_server(dolt, data_dir, port, log); OWNED_SERVER = server; after = bd_call(bd, repo, "list", "--all", "--json").stdout; properties["cross_process_visibility_and_clean_restart"] = {"observed": json.loads(before) == json.loads(after), "before_count": len(json.loads(before)), "after_count": len(json.loads(after))}

            cuts = []
            for cut in range(5):
                handoff_root = root / f"handoff-cut-{cut}"; handoff_root.mkdir(); prepared = prepared_payload(cut); validate_payload("handoff-prepared.v1.schema.json", prepared); canonical_write(handoff_root / "prepared.json", prepared); create(bd, repo, prepared["semantic_bead_id"], "semantic handoff")
                if cut >= 1: terminalize_handoff(bd, repo, prepared, prepared["admission_certificate_digest"])
                for _ in range(max(0, cut - 1)): reconcile_handoff(handoff_root, bd, repo)
                steps = []
                for _ in range(6):
                    step = reconcile_handoff(handoff_root, bd, repo); steps.append(step)
                    if step["action"] == "await_terminal": terminalize_handoff(bd, repo, prepared, prepared["admission_certificate_digest"])
                    if step["action"] == "converged": break
                count = len(json.loads(bd_call(bd, repo, "list", "--all", "--id", prepared["expected_delivery_bead_id"], "--json").stdout))
                artifacts = {}
                for label, path, schema in (("prepared", handoff_root / "prepared.json", "handoff-prepared.v1.schema.json"), ("nonroutable_delivery", handoff_root / "delivery.nonroutable.json", "delivery.v1.schema.json"), ("published_delivery", handoff_root / "delivery.published.json", "delivery.v1.schema.json"), ("committed", handoff_root / "committed.json", "handoff-committed.v1.schema.json")):
                    payload, artifact_error = load_valid(path, schema)
                    artifacts[label] = payload is not None and artifact_error is None
                cuts.append({"cut": cut, "steps": steps, "exactly_one_delivery_bead": count == 1, "converged": steps[-1]["action"] == "converged", "precommit_authorizations": sum(item["effect_authorized"] for item in steps[:-1]), "schema_valid_artifacts": artifacts})
            conflict_root = root / "handoff-conflict"; conflict_root.mkdir(); conflict = prepared_payload(91); validate_payload("handoff-prepared.v1.schema.json", conflict); canonical_write(conflict_root / "prepared.json", conflict); create(bd, repo, conflict["semantic_bead_id"], "semantic conflict handoff"); create(bd, repo, conflict["expected_delivery_bead_id"], "conflicting successor", external_ref=external_ref("b" * 64, 1)); terminalize_handoff(bd, repo, conflict, conflict["admission_certificate_digest"]); conflict_step = reconcile_handoff(conflict_root, bd, repo); conflict_count = len(json.loads(bd_call(bd, repo, "list", "--all", "--id", conflict["expected_delivery_bead_id"], "--json").stdout)); negative = {"action": conflict_step["action"], "effect_authorized": conflict_step["effect_authorized"], "delivery_count": conflict_count, "published": (conflict_root / "delivery.published.json").exists(), "committed": (conflict_root / "committed.json").exists()}; negative_ok = negative == {"action": "refuse_conflicting_delivery_identity", "effect_authorized": False, "delivery_count": 1, "published": False, "committed": False}
            bd_call(bd, repo, "update", "cap-semantic-4", "--set-metadata", f"gc33.terminal={json.dumps({'verdict': 'PASS', 'certificate_digest': 'c' * 64})}")
            late_conflict = reconcile_handoff(root / "handoff-cut-4", bd, repo)
            late_conflict_ok = late_conflict == {"action": "refuse_conflicting_terminal", "effect_authorized": False}
            properties["marker_first_cross_store_handoff"] = {"observed": handoff_selection_observed(cuts, negative_ok, late_conflict_ok), "per_cut": cuts, "negative_preexisting_conflicting_child": negative, "negative_preexisting_conflicting_child_observed": negative_ok, "late_conflicting_terminal": late_conflict, "late_conflicting_terminal_observed": late_conflict_ok}
            properties["event_single_flight"] = {"result": "NOT_PROVEN", "reason": "GC runtime/event was not exercised; event wake remains disabled"}
        finally:
            cleanup["no_attributed_process_remains"] = stop_server(server); OWNED_SERVER = None; signal.signal(signal.SIGTERM, prior_term); signal.signal(signal.SIGINT, prior_int)
    required = ("claim_race_exclusive", "tested_safe_metadata_envelope", "cross_process_visibility_and_clean_restart", "deterministic_successor_create_or_discover", "marker_first_cross_store_handoff")
    if not all(properties[name].get("observed") for name in required) or not properties["stale_claimant_can_mutate"]["observed"] or not cleanup["no_attributed_process_remains"]:
        raise ProbeError(f"capability result did not support selected representation: {json.dumps(properties, sort_keys=True)}")
    receipt = {"schema_version": "beads-capability-selection.v1", "toolchain": {**runtime, "harness_sha256": sha256(Path(__file__)), "lock_sha256": sha256(LOCK), "managed_store": {"topology": "private external Dolt sql-server", "host": "127.0.0.1", "port": port}}, "selected_representation": "B-successor-delivery-bead", "selection_reason": "A stale former claimant can mutate ordinary metadata, so it is not a fence. Exact immutable successor id plus exact external_ref/handoff/epoch rediscovery supports one non-routable delivery successor; delivery remains marker-first and effects remain blocked until committed.", "properties": properties, "cleanup": cleanup}
    output.parent.mkdir(parents=True, exist_ok=True); canonical_write(output, receipt); return receipt


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path); parser.add_argument("--bd-bin", required=True, type=Path); parser.add_argument("--dolt-bin", required=True, type=Path); parser.add_argument("--trials", type=int, default=3)
    args = parser.parse_args()
    try:
        probe(args.output, args.bd_bin, args.dolt_bin, args.trials)
    except (ProbeError, subprocess.TimeoutExpired, OSError, json.JSONDecodeError) as exc:
        print(f"beads capability probe: {exc}", file=sys.stderr); return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
