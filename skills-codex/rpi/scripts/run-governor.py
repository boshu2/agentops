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
STUCK_BREAKERS = {"max-attempts", "oscillation", "no-progress"}
HARD_BREAKERS = {"human-judgment"}
LIMIT_KEYS = (
    "waves",
    "reviewer_tokens",
    "elapsed_seconds",
    "review_contexts",
    "deterministic_executions",
)
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
        "disposition": "ANDON",
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


def require_counter(value: Any, name: str, *, positive: bool = False) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise GovernorError(f"invalid-state:{name}")
    if value < (1 if positive else 0):
        raise GovernorError(f"invalid-state:{name}")
    return value


def validate_state(state: Any, expected_run_id: str) -> dict[str, Any]:
    if not isinstance(state, dict):
        raise GovernorError("corrupt-state")
    if state.get("schema_version") != SCHEMA_VERSION:
        raise GovernorError("corrupt-state")
    if state.get("run_id") != expected_run_id:
        raise GovernorError("state-identity-mismatch")
    if state.get("disposition") not in DISPOSITIONS:
        raise GovernorError("corrupt-state")
    limits = state.get("limits")
    usage = state.get("usage")
    if not isinstance(limits, dict) or not isinstance(usage, dict):
        raise GovernorError("corrupt-state")
    for key in LIMIT_KEYS:
        require_counter(limits.get(key), f"limits.{key}", positive=True)
        require_counter(usage.get(key), f"usage.{key}")
        if usage[key] > limits[key]:
            raise GovernorError("corrupt-state")
    if not isinstance(state.get("admissions"), list):
        raise GovernorError("corrupt-state")
    if not isinstance(state.get("helper_history"), dict):
        raise GovernorError("corrupt-state")
    helper = state.get("helper")
    if not isinstance(helper, dict) or not isinstance(helper.get("allowed"), bool):
        raise GovernorError("corrupt-state")
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
    if args.action not in {"crank-wave", "semantic-review", "deterministic-proof"}:
        raise GovernorError("invalid-action")
    charge = admission_charge(args)
    path = state_path(args.state_dir, run_id)
    with locked(path):
        state = load_state(path, run_id)
        if state["disposition"] in {"HOLD", "ANDON"}:
            state["authorized"] = False
            emit(state)
            return 4
        exceeded = next(
            (
                key
                for key in LIMIT_KEYS
                if state["usage"][key] >= state["limits"][key]
                or state["usage"][key] + charge[key] > state["limits"][key]
            ),
            None,
        )
        if exceeded is not None:
            state["disposition"] = "ANDON"
            state["reason"] = f"hard-ceiling:{exceeded}"
            state["authorized"] = False
            state["helper"] = {"allowed": False}
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
    if args.disposition not in DISPOSITIONS:
        raise GovernorError("invalid-disposition")

    def transition(state: dict[str, Any]) -> int:
        state["disposition"] = args.disposition
        state["reason"] = args.reason or "explicit-transition"
        state["authorized"] = False
        state["helper"] = {"allowed": args.disposition == "HOLD"}
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

    def consult(state: dict[str, Any]) -> int:
        helper = state["helper"]
        already_used = args.blocker_class in state["helper_history"]
        wrong_hold = (
            state["disposition"] != "HOLD"
            or not helper.get("allowed", False)
            or helper.get("blocker_class") != args.blocker_class
        )
        if already_used or wrong_hold:
            state["disposition"] = "ANDON"
            state["reason"] = "helper-not-authorized"
            state["authorized"] = False
            state["helper"] = {
                "allowed": False,
                "blocker_class": args.blocker_class,
            }
            return 4
        if args.result == "UNSTUCK" and not args.new_approach:
            state["disposition"] = "ANDON"
            state["reason"] = "missing-new-approach"
            state["authorized"] = False
            state["helper"] = {
                "allowed": False,
                "blocker_class": args.blocker_class,
            }
            return 4

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
