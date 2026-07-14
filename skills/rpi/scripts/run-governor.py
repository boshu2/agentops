#!/usr/bin/env python3
"""Persistent, fail-closed RPI run admission and breaker governor."""

from __future__ import annotations

import argparse
import fcntl
import json
import os
import re
import sys
import tempfile
from contextlib import contextmanager
from pathlib import Path
from typing import Any, Iterator


SCHEMA_VERSION = 1
DISPOSITIONS = {"NOTE", "REPAIR", "REPLAN", "HOLD", "ANDON"}
ORDINARY_DISPOSITIONS = {"NOTE", "REPAIR", "REPLAN"}
STUCK_BREAKERS = {"max-attempts", "oscillation", "no-progress"}
HARD_BREAKERS = {"human-judgment"}
ACTIONS = {"crank-wave", "semantic-review", "deterministic-proof"}
LIMIT_KEYS = (
    "waves",
    "reviewer_tokens",
    "elapsed_seconds",
    "review_contexts",
    "deterministic_executions",
)
TOP_LEVEL_KEYS = {
    "schema_version",
    "run_id",
    "limits",
    "usage",
    "disposition",
    "reason",
    "authorized",
    "admissions",
    "helper",
    "helper_history",
}
ADMISSION_KEYS = {"id", "sequence", "action", "charge", "status"}
HELPER_REQUIRED_KEYS = {"allowed"}
HELPER_OPTIONAL_KEYS = {"blocker_class", "result", "new_approach"}
HELPER_HISTORY_REQUIRED_KEYS = {"result"}
HELPER_HISTORY_OPTIONAL_KEYS = {"new_approach"}
RUN_ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$")


class GovernorError(Exception):
    def __init__(self, reason: str, *, exit_code: int = 2):
        super().__init__(reason)
        self.reason = reason
        self.exit_code = exit_code


def emit(payload: dict[str, Any]) -> None:
    print(json.dumps(payload, sort_keys=True, separators=(",", ":")))


def refusal(reason: str, *, helper_allowed: bool = False) -> dict[str, Any]:
    return {
        "schema_version": SCHEMA_VERSION,
        "disposition": "NOTE",
        "reason": reason,
        "authorized": False,
        "helper": {"allowed": helper_allowed},
    }


def require_run_id(run_id: str | None) -> str:
    if not run_id or not RUN_ID_RE.fullmatch(run_id):
        raise GovernorError("invalid-run-id")
    return run_id


def state_path(state_dir: str | None, run_id: str) -> Path:
    if not state_dir:
        raise GovernorError("missing-state-dir")
    return Path(state_dir).resolve() / f"{run_id}.json"


@contextmanager
def locked(path: Path) -> Iterator[None]:
    path.parent.mkdir(parents=True, exist_ok=True)
    lock_path = path.parent / f".{path.stem}.lock"
    with lock_path.open("a+b") as lock_file:
        fcntl.flock(lock_file.fileno(), fcntl.LOCK_EX)
        try:
            yield
        finally:
            fcntl.flock(lock_file.fileno(), fcntl.LOCK_UN)


def corrupt() -> None:
    raise GovernorError("corrupt-state")


def require_exact_keys(
    value: Any,
    required: set[str],
    optional: set[str] | None = None,
) -> dict[str, Any]:
    if not isinstance(value, dict):
        corrupt()
    allowed = required | (optional or set())
    if not required.issubset(value) or not set(value).issubset(allowed):
        corrupt()
    return value


def require_nonempty_string(value: Any) -> str:
    if not isinstance(value, str) or not value:
        corrupt()
    return value


def require_counter(value: Any, *, positive: bool = False) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        corrupt()
    if value < (1 if positive else 0):
        corrupt()
    return value


def validate_state(state: Any, expected_run_id: str) -> dict[str, Any]:
    state = require_exact_keys(state, TOP_LEVEL_KEYS)
    schema_version = state.get("schema_version")
    if (
        isinstance(schema_version, bool)
        or not isinstance(schema_version, int)
        or schema_version != SCHEMA_VERSION
    ):
        corrupt()
    if state.get("run_id") != expected_run_id:
        raise GovernorError("state-identity-mismatch")
    if not RUN_ID_RE.fullmatch(expected_run_id):
        corrupt()
    if state.get("disposition") not in DISPOSITIONS:
        corrupt()
    require_nonempty_string(state.get("reason"))
    if not isinstance(state.get("authorized"), bool):
        corrupt()

    meter_keys = set(LIMIT_KEYS)
    limits = require_exact_keys(state.get("limits"), meter_keys)
    usage = require_exact_keys(state.get("usage"), meter_keys)
    for key in LIMIT_KEYS:
        require_counter(limits.get(key), positive=True)
        require_counter(usage.get(key))
        if usage[key] > limits[key]:
            corrupt()

    admissions = state.get("admissions")
    if not isinstance(admissions, list):
        corrupt()
    calculated_usage = {key: 0 for key in LIMIT_KEYS}
    for expected_sequence, admission_value in enumerate(admissions, start=1):
        admission = require_exact_keys(admission_value, ADMISSION_KEYS)
        sequence = require_counter(admission.get("sequence"), positive=True)
        if sequence != expected_sequence:
            corrupt()
        if admission.get("id") != f"{expected_run_id}:{expected_sequence}":
            corrupt()
        if admission.get("action") not in ACTIONS:
            corrupt()
        if admission.get("status") != "recorded":
            corrupt()
        charge = require_exact_keys(admission.get("charge"), meter_keys)
        for key in LIMIT_KEYS:
            calculated_usage[key] += require_counter(charge.get(key))
        expected_wave_charge = 1 if admission["action"] == "crank-wave" else 0
        if charge["waves"] != expected_wave_charge:
            corrupt()
    if calculated_usage != usage:
        corrupt()

    helper = require_exact_keys(
        state.get("helper"), HELPER_REQUIRED_KEYS, HELPER_OPTIONAL_KEYS
    )
    if not isinstance(helper.get("allowed"), bool):
        corrupt()
    if "blocker_class" in helper:
        require_nonempty_string(helper["blocker_class"])
    if "result" in helper and helper["result"] not in {"UNSTUCK", "ESCALATE"}:
        corrupt()
    if "new_approach" in helper:
        require_nonempty_string(helper["new_approach"])
    if helper.get("result") == "UNSTUCK" and "new_approach" not in helper:
        corrupt()

    helper_history = state.get("helper_history")
    if not isinstance(helper_history, dict):
        corrupt()
    for blocker_class, record_value in helper_history.items():
        require_nonempty_string(blocker_class)
        record = require_exact_keys(
            record_value,
            HELPER_HISTORY_REQUIRED_KEYS,
            HELPER_HISTORY_OPTIONAL_KEYS,
        )
        if record.get("result") not in {"UNSTUCK", "ESCALATE"}:
            corrupt()
        if "new_approach" in record:
            require_nonempty_string(record["new_approach"])
        if record["result"] == "UNSTUCK" and "new_approach" not in record:
            corrupt()

    disposition = state["disposition"]
    if disposition == "HOLD":
        blocker_class = helper.get("blocker_class")
        if (
            state["reason"] not in STUCK_BREAKERS
            or helper["allowed"] is not True
            or not blocker_class
            or blocker_class in helper_history
        ):
            corrupt()
    elif helper["allowed"] is not False:
        corrupt()

    if state["authorized"] and (
        disposition != "NOTE"
        or state["reason"] != "admitted-before-dispatch"
        or not admissions
    ):
        corrupt()
    if not state["authorized"] and state["reason"] == "admitted-before-dispatch":
        corrupt()
    if state["reason"].startswith("hard-ceiling:") and disposition != "ANDON":
        corrupt()
    if "result" in helper:
        blocker_class = helper.get("blocker_class")
        if not blocker_class or helper_history.get(blocker_class) != {
            key: helper[key]
            for key in ("result", "new_approach")
            if key in helper
        }:
            corrupt()
        expected_disposition = "REPAIR" if helper["result"] == "UNSTUCK" else "ANDON"
        if disposition != expected_disposition:
            corrupt()
    return state


def load_state(path: Path, run_id: str) -> dict[str, Any]:
    if not path.is_file():
        raise GovernorError("missing-state")
    try:
        with path.open("r", encoding="utf-8") as handle:
            state = json.load(handle)
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise GovernorError("corrupt-state") from exc
    return validate_state(state, run_id)


def atomic_write(path: Path, state: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=path.parent
    )
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(state, handle, sort_keys=True, separators=(",", ":"))
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary_name, path)
        directory_fd = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)
    finally:
        try:
            os.unlink(temporary_name)
        except FileNotFoundError:
            pass


def initial_state(args: argparse.Namespace, run_id: str) -> dict[str, Any]:
    limits = {
        "waves": 3 if args.max_waves is None else args.max_waves,
        "reviewer_tokens": args.max_reviewer_tokens,
        "elapsed_seconds": args.max_elapsed_seconds,
        "review_contexts": args.max_review_contexts,
        "deterministic_executions": args.max_deterministic_executions,
    }
    if any(value is None for value in limits.values()):
        raise GovernorError("missing-ceiling")
    for key, value in limits.items():
        if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
            raise GovernorError(f"invalid-ceiling:{key}")
    return {
        "schema_version": SCHEMA_VERSION,
        "run_id": run_id,
        "limits": limits,
        "usage": {key: 0 for key in LIMIT_KEYS},
        "disposition": "NOTE",
        "reason": "initialized",
        "authorized": False,
        "admissions": [],
        "helper": {"allowed": False},
        "helper_history": {},
    }


def command_init(args: argparse.Namespace) -> int:
    run_id = require_run_id(args.run_id)
    path = state_path(args.state_dir, run_id)
    with locked(path):
        if path.exists():
            raise GovernorError("state-already-exists")
        state = initial_state(args, run_id)
        validate_state(state, run_id)
        atomic_write(path, state)
    emit(state)
    return 0


def admission_charge(args: argparse.Namespace) -> dict[str, int]:
    meters = {
        "reviewer_tokens": args.reviewer_tokens,
        "elapsed_seconds": args.elapsed_seconds,
        "review_contexts": args.review_contexts,
        "deterministic_executions": args.deterministic_executions,
    }
    if any(value is None for value in meters.values()):
        raise GovernorError("missing-meter")
    for key, value in meters.items():
        if isinstance(value, bool) or not isinstance(value, int) or value < 0:
            raise GovernorError(f"invalid-meter:{key}")
    meters["waves"] = 1 if args.action == "crank-wave" else 0
    return {key: meters[key] for key in LIMIT_KEYS}


def command_admit(args: argparse.Namespace) -> int:
    run_id = require_run_id(args.run_id)
    if args.action not in ACTIONS:
        raise GovernorError("invalid-action")
    charge = admission_charge(args)
    path = state_path(args.state_dir, run_id)
    with locked(path):
        state = load_state(path, run_id)
        if state["disposition"] in {"HOLD", "ANDON"}:
            emit(state)
            return 4
        exceeded = next(
            (
                key
                for key in LIMIT_KEYS
                if charge[key] > 0
                and state["usage"][key] + charge[key] > state["limits"][key]
            ),
            None,
        )
        if exceeded is not None:
            state["disposition"] = "ANDON"
            state["reason"] = f"hard-ceiling:{exceeded}"
            state["authorized"] = False
            state["helper"] = {"allowed": False}
            validate_state(state, run_id)
            atomic_write(path, state)
            emit(state)
            return 3

        sequence = len(state["admissions"]) + 1
        for key, value in charge.items():
            state["usage"][key] += value
        state["admissions"].append(
            {
                "id": f"{run_id}:{sequence}",
                "sequence": sequence,
                "action": args.action,
                "charge": charge,
                "status": "recorded",
            }
        )
        state["disposition"] = "NOTE"
        state["reason"] = "admitted-before-dispatch"
        state["authorized"] = True
        state["helper"] = {"allowed": False}
        validate_state(state, run_id)
        atomic_write(path, state)
    emit(state)
    return 0


def mutate_state(args: argparse.Namespace, mutation: Any) -> tuple[dict[str, Any], int]:
    run_id = require_run_id(args.run_id)
    path = state_path(args.state_dir, run_id)
    with locked(path):
        state = load_state(path, run_id)
        exit_code = mutation(state)
        validate_state(state, run_id)
        atomic_write(path, state)
    return state, exit_code


def command_transition(args: argparse.Namespace) -> int:
    if args.disposition not in ORDINARY_DISPOSITIONS:
        raise GovernorError("invalid-disposition")

    def transition(state: dict[str, Any]) -> int:
        if state["disposition"] in {"HOLD", "ANDON"}:
            raise GovernorError("protected-disposition")
        state["disposition"] = args.disposition
        state["reason"] = args.reason or "explicit-transition"
        state["authorized"] = False
        state["helper"] = {"allowed": False}
        return 0

    state, exit_code = mutate_state(args, transition)
    emit(state)
    return exit_code


def command_break(args: argparse.Namespace) -> int:
    if args.kind not in STUCK_BREAKERS | HARD_BREAKERS:
        raise GovernorError("invalid-breaker")
    if not args.blocker_class:
        raise GovernorError("missing-blocker-class")

    def trip(state: dict[str, Any]) -> int:
        if state["disposition"] in {"HOLD", "ANDON"}:
            raise GovernorError("protected-disposition")
        if args.kind in HARD_BREAKERS:
            state["disposition"] = "ANDON"
            state["reason"] = args.kind
            state["helper"] = {
                "allowed": False,
                "blocker_class": args.blocker_class,
            }
        elif args.blocker_class in state["helper_history"]:
            state["disposition"] = "ANDON"
            state["reason"] = "helper-already-consumed"
            state["helper"] = {
                "allowed": False,
                "blocker_class": args.blocker_class,
            }
        else:
            state["disposition"] = "HOLD"
            state["reason"] = args.kind
            state["helper"] = {
                "allowed": True,
                "blocker_class": args.blocker_class,
            }
        state["authorized"] = False
        return 0

    state, exit_code = mutate_state(args, trip)
    emit(state)
    return exit_code


def command_helper(args: argparse.Namespace) -> int:
    if args.result not in {"UNSTUCK", "ESCALATE"}:
        raise GovernorError("invalid-helper-result")
    if not args.blocker_class:
        raise GovernorError("missing-blocker-class")
    if args.result == "UNSTUCK" and not args.new_approach:
        raise GovernorError("missing-new-approach")

    def consult(state: dict[str, Any]) -> int:
        helper = state["helper"]
        already_used = args.blocker_class in state["helper_history"]
        wrong_hold = (
            state["disposition"] != "HOLD"
            or not helper.get("allowed", False)
            or helper.get("blocker_class") != args.blocker_class
        )
        if already_used or wrong_hold:
            raise GovernorError("helper-not-authorized", exit_code=4)

        record = {"result": args.result}
        if args.new_approach:
            record["new_approach"] = args.new_approach
        state["helper_history"][args.blocker_class] = record
        state["disposition"] = "REPAIR" if args.result == "UNSTUCK" else "ANDON"
        state["reason"] = (
            "helper-unstuck-new-approach"
            if args.result == "UNSTUCK"
            else "helper-escalate"
        )
        state["authorized"] = False
        state["helper"] = {
            "allowed": False,
            "blocker_class": args.blocker_class,
            **record,
        }
        return 0

    state, exit_code = mutate_state(args, consult)
    emit(state)
    return exit_code


def command_human(args: argparse.Namespace) -> int:
    if args.disposition not in ORDINARY_DISPOSITIONS:
        raise GovernorError("invalid-disposition")
    if not args.reason:
        raise GovernorError("missing-human-reason")

    def authorize(state: dict[str, Any]) -> int:
        if state["disposition"] != "ANDON":
            raise GovernorError("human-authority-not-required")
        if state["reason"].startswith("hard-ceiling:"):
            raise GovernorError("spent-hard-ceiling")
        state["disposition"] = args.disposition
        state["reason"] = f"human-authority:{args.reason}"
        state["authorized"] = False
        state["helper"] = {"allowed": False}
        return 0

    state, exit_code = mutate_state(args, authorize)
    emit(state)
    return exit_code


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command")

    def state_args(command: argparse.ArgumentParser) -> None:
        command.add_argument("--state-dir")
        command.add_argument("--run-id")

    init = subparsers.add_parser("init")
    state_args(init)
    init.add_argument("--max-waves", type=int)
    init.add_argument("--max-reviewer-tokens", type=int)
    init.add_argument("--max-elapsed-seconds", type=int)
    init.add_argument("--max-review-contexts", type=int)
    init.add_argument("--max-deterministic-executions", type=int)
    init.set_defaults(handler=command_init)

    admit = subparsers.add_parser("admit")
    state_args(admit)
    admit.add_argument("--action")
    admit.add_argument("--reviewer-tokens", type=int)
    admit.add_argument("--elapsed-seconds", type=int)
    admit.add_argument("--review-contexts", type=int)
    admit.add_argument("--deterministic-executions", type=int)
    admit.set_defaults(handler=command_admit)

    transition = subparsers.add_parser("transition")
    state_args(transition)
    transition.add_argument("--disposition")
    transition.add_argument("--reason")
    transition.set_defaults(handler=command_transition)

    breaker = subparsers.add_parser("break")
    state_args(breaker)
    breaker.add_argument("--kind")
    breaker.add_argument("--blocker-class")
    breaker.set_defaults(handler=command_break)

    helper = subparsers.add_parser("helper")
    state_args(helper)
    helper.add_argument("--blocker-class")
    helper.add_argument("--result")
    helper.add_argument("--new-approach")
    helper.set_defaults(handler=command_helper)

    human = subparsers.add_parser("human")
    state_args(human)
    human.add_argument("--disposition")
    human.add_argument("--reason")
    human.set_defaults(handler=command_human)
    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    if not hasattr(args, "handler"):
        emit(refusal("missing-command"))
        return 2
    try:
        return args.handler(args)
    except GovernorError as exc:
        emit(refusal(exc.reason))
        return exc.exit_code
    except Exception:
        emit(refusal("internal-governor-error"))
        return 2


if __name__ == "__main__":
    sys.exit(main())
