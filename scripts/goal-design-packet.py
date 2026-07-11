#!/usr/bin/env python3
"""Create and maintain goal-design packets.

The checker owns validation. This helper owns the boring but failure-prone
authoring mechanics: writing the two packet files, computing the intent digest,
and refreshing driver references after intent edits.
"""

from __future__ import annotations

import argparse
import hashlib
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover - dependency guard for operator clarity
    print("goal-design-packet: PyYAML is required", file=sys.stderr)
    raise SystemExit(2)


SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
SHA_RE = re.compile(r"^[0-9a-f]{64}$")


def fail(message: str, code: int = 1) -> None:
    print(f"goal-design-packet: {message}", file=sys.stderr)
    raise SystemExit(code)


def repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


def canonical_intent_ref(slug: str) -> str:
    return f".agents/goal-design/{slug}/intent.md"


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def split_frontmatter(path: Path) -> tuple[dict[str, Any], str]:
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---\n"):
        fail(f"{path} missing YAML frontmatter")
    end = text.find("\n---\n", 4)
    if end < 0:
        fail(f"{path} has unterminated YAML frontmatter")
    raw = text[4:end]
    data = yaml.safe_load(raw)
    if not isinstance(data, dict):
        fail(f"{path} frontmatter did not parse as a mapping")
    return data, text[end + len("\n---\n") :]


def render_markdown(data: dict[str, Any], body: str) -> str:
    frontmatter = yaml.safe_dump(data, sort_keys=False, allow_unicode=False)
    return f"---\n{frontmatter}---\n{body}"


def validate_slug(slug: str) -> None:
    if not SLUG_RE.match(slug):
        fail(f"invalid slug {slug!r}; expected lowercase kebab-case")


def intent_data(args: argparse.Namespace) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "kind": "goal-design.intent",
        "id": f"gd-intent-{args.slug}",
        "slug": args.slug,
        "created_at": args.created_at,
        "status": "draft",
        "objective": args.objective,
        "why_it_matters": args.why,
        "domain_terms": [
            {
                "term": "goal-design packet",
                "definition": "The two-artifact intent and driver contract that turns a goal into loop-ready work.",
                "source": "docs/contracts/goal-design-artifacts.md",
            }
        ],
        "bdd": {
            "feature": args.feature,
            "scenarios": [
                {
                    "id": args.scenario_id,
                    "name": args.scenario_name,
                    "given": [args.given],
                    "when": [args.when],
                    "then": [args.then],
                }
            ],
        },
        "boundaries": {
            "bounded_context": args.bounded_context,
            "in_scope": args.in_scope,
            "non_goals": args.non_goal,
            "rollback_or_containment": args.rollback,
        },
        "evidence_for_done": {
            "first_failing_proof": args.first_failing_proof,
            "validation_command": f"scripts/check-goal-design-packet.sh .agents/goal-design/{args.slug}",
            "evidence_path": str(Path(args.output_root) / args.slug),
            "independent_gate": "validate",
        },
        "inputs_to_recheck": {
            "repo_paths": args.repo_path,
            "prior_artifacts": args.prior_artifact,
            "live_surfaces": args.live_surface,
            "stale_assumptions": args.stale_assumption,
        },
        "hard_rules": [
            "Keep behavior slices small.",
            "Refresh driver intent_ref.sha256 after every intent.md edit.",
            "Run the checker and independent validation before the packet drives work.",
        ],
    }


def intent_body(args: argparse.Namespace) -> str:
    return f"""# Goal Design Intent: {args.slug}

## Objective

{args.objective}

## BDD Behavior

```gherkin
Feature: {args.feature}

  Scenario: {args.scenario_name}
    Given {args.given}
    When {args.when}
    Then {args.then}
```
"""


def driver_data(args: argparse.Namespace, digest: str) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "kind": "goal-design.driver",
        "id": f"gd-driver-{args.slug}",
        "slug": args.slug,
        "created_at": args.created_at,
        "status": "draft",
        "intent_ref": {
            "path": canonical_intent_ref(args.slug),
            "sha256": digest,
            "schema_version": 1,
        },
        "loop_routing": {
            "delivery": "File or update one bead only after the packet validates.",
            "rpi": "Run one inner tick over one behavior and one first failing proof.",
            "promotion": "Promote only evidence-backed changes after validation.",
            "knowledge": "Capture checker or validator misses as future guardrails.",
        },
        "candidate_beads": [
            {
                "id": "B1",
                "behavior": args.behavior,
                "bounded_context": args.bounded_context,
                "first_failing_proof": args.first_failing_proof,
                "write_scope": args.write_scope,
                "close_signal": args.close_signal,
            }
        ],
        "small_batch_gate": {
            "one_behavior": True,
            "one_bounded_context": True,
            "one_primary_write_scope": True,
            "one_acceptance_proof": True,
            "split_required_if": [
                "The change starts mixing unrelated behavior, write scopes, or product surfaces."
            ],
        },
        "route_back_rules": {
            "validation_fails": "Patch the packet contract or artifacts before filing work. After 3 failed rounds or an oscillation, take ONE bounded helper pass (fresh context, cross-family model, or council) before any human escalation.",
            "bead_closes_with_new_signal": "Use the close verdict to choose or revise the next candidate.",
            "candidate_stale": "Re-read the named inputs, refresh the digest, and revalidate.",
            "promotion_contradicts_intent": "Revise intent.md, refresh driver.md, and revalidate. Route the fork to the helper tier (council); escalate to human only if it survives the pass or is refusal-lane.",
        },
        "execution_mode": {
            "default": "single-agent",
            "escalations": {
                "ntm_atm": "Only when durability, attach, or cross-model debate is required.",
                "workflow": "Only for deterministic structured DAG needs.",
            },
        },
        "artifact_validation": {
            "checker_command": f"scripts/check-goal-design-packet.sh .agents/goal-design/{args.slug}",
            "independent_validator": "validate",
            "required_verdict": "PASS",
        },
    }


def driver_body(args: argparse.Namespace, digest: str) -> str:
    intent_ref = canonical_intent_ref(args.slug)
    return f"""# Goal Design Driver: {args.slug}

## Source Intent

- Intent artifact: `{intent_ref}`
- Intent digest: `{digest}`
- Last validation verdict: none

## Candidate Beads

| Candidate | Behavior | Bounded context | First failing proof | Write scope | Close signal |
| --- | --- | --- | --- | --- | --- |
| B1 | {args.behavior} | {args.bounded_context} | {args.first_failing_proof} | {', '.join(args.write_scope)} | {args.close_signal} |

## Andon Router (class -> tier)

| One-way-door class | Tier | Machinery (reuse, never rebuild) |
| --- | --- | --- |
| Gate / validation failure | **auto** | AUTO-REDO + `ao gate check --fast --scope head` |
| Architecture fork / plan-shape one-way door | **helper** | `/council` + `ao plan-pawl decide` (PASS/REDO/BLOCKED) + `/converge` |
| Stuck: 3 failed validation rounds, oscillation, or a scope-creep flag | **helper** | one bounded helper pass - hand the blocker, the evidence, and what was tried to a fresh context or cross-family model (`codex exec`, `/council`); it returns UNSTUCK (a concrete next action) or ESCALATE. An advisor, never a second driver: it does not take over the work |
| Money / legal / irreversible-external (the refusal lane), an explicit judgment flag, or an exhausted time/cost budget | **human** | ESCALATE / HOLD - hand back to the operator; the helper is skipped |
| TODO: goal-specific rows (write-scope escapes, domain forks) - edit deliberately before dispatch | - | - |

Implicit final rows: a breaker trip routes to ONE bounded helper pass first;
**human** only when the blocker survives that pass, the helper itself returns
ESCALATE, or the class is refusal-lane / explicit-judgment / budget-exhausted.
Never a second helper pass on the same blocker class - stop and ask, never
guess through it.
"""


def command_new(args: argparse.Namespace) -> int:
    validate_slug(args.slug)
    if not args.behavior:
        args.behavior = f"{args.scenario_id}: {args.scenario_name}"

    packet_dir = Path(args.output_root) / args.slug
    intent_path = packet_dir / "intent.md"
    driver_path = packet_dir / "driver.md"
    if packet_dir.exists() and not args.force:
        fail(f"packet already exists: {packet_dir} (use --force to overwrite)")

    packet_dir.mkdir(parents=True, exist_ok=True)
    intent_path.write_text(render_markdown(intent_data(args), intent_body(args)), encoding="utf-8")
    digest = sha256_file(intent_path)
    driver_path.write_text(render_markdown(driver_data(args, digest), driver_body(args, digest)), encoding="utf-8")

    if args.check:
        return run_checker(packet_dir)
    print(packet_dir)
    return 0


def replace_or_append(body: str, pattern: str, replacement: str, fallback: str) -> str:
    updated, count = re.subn(pattern, replacement, body, count=1, flags=re.MULTILINE)
    if count:
        return updated
    if updated and not updated.endswith("\n"):
        updated += "\n"
    return f"{updated}\n{fallback}\n"


def command_refresh_digest(args: argparse.Namespace) -> int:
    packet_dir = Path(args.packet_dir)
    intent_path = packet_dir / "intent.md"
    driver_path = packet_dir / "driver.md"
    if not intent_path.is_file() or not driver_path.is_file():
        fail(f"packet must contain intent.md and driver.md: {packet_dir}", 2)

    intent, _ = split_frontmatter(intent_path)
    driver, body = split_frontmatter(driver_path)
    slug = str(intent.get("slug", ""))
    validate_slug(slug)
    digest = sha256_file(intent_path)
    if not SHA_RE.match(digest):
        fail(f"computed invalid digest for {intent_path}", 2)

    intent_ref = canonical_intent_ref(slug)
    driver.setdefault("intent_ref", {})
    driver["intent_ref"]["path"] = intent_ref
    driver["intent_ref"]["sha256"] = digest
    driver["intent_ref"]["schema_version"] = 1

    body = replace_or_append(
        body,
        r"(- Intent artifact: `)[^`]+(`)",
        rf"\g<1>{intent_ref}\2",
        f"- Intent artifact: `{intent_ref}`",
    )
    body = replace_or_append(
        body,
        r"(- Intent digest: `)[^`]+(`)",
        rf"\g<1>{digest}\2",
        f"- Intent digest: `{digest}`",
    )
    driver_path.write_text(render_markdown(driver, body), encoding="utf-8")

    if args.check:
        return run_checker(packet_dir)
    print(digest)
    return 0


def dispatch_prompt_text(packet_dir: Path, intent: dict[str, Any], driver: dict[str, Any], draft: bool) -> str:
    # Goal-API workers often run in fresh worktrees where gitignored .agents/
    # does not exist; absolute paths keep the packet reachable from anywhere.
    packet_dir = packet_dir.absolute()
    objective = str(intent.get("objective", "")).strip()
    beads = driver.get("candidate_beads") or []
    first = beads[0] if isinstance(beads, list) and beads else {}
    bead_id = str(first.get("id", "B1"))
    behavior = str(first.get("behavior", "")).strip()
    lines: list[str] = []
    if draft:
        lines.append("[DRAFT PACKET - NOT VALIDATED. Preview only; validate before dispatch.]")
    lines += [
        f"Execute the goal-design packet at {packet_dir}.",
        "",
        f"Read {packet_dir}/intent.md and {packet_dir}/driver.md FIRST - they are the",
        "contract and override everything else, including this prompt.",
        "",
        f"Objective: {objective}",
        f"Candidate: {bead_id} - {behavior}",
        "",
        "Rules: write ONLY within the candidate's write_scope; respect the intent's non_goals.",
        "Done means the driver's first_failing_proof command exits 0 AND its close_signal",
        "holds - run the proof verbatim; no proof, not done.",
        "When blocked, follow the driver's andon router (auto -> helper -> human):",
        "a breaker trip gets ONE bounded helper pass (fresh context, cross-family model,",
        "or council) before the operator; escalate to human only when the blocker",
        "survives that pass or the class is refusal-lane / explicit-judgment /",
        "budget-exhausted (those classes skip the helper - no consult on a spent ceiling).",
        "Stop and hand back rather than guess past a breaker.",
    ]
    return "\n".join(lines) + "\n"


GOAL_API_PROMPT_LIMIT = 4000  # goal-API hard ceiling; --max-chars may only tighten it


def command_prompt(args: argparse.Namespace) -> int:
    if args.max_chars < 1 or args.max_chars > GOAL_API_PROMPT_LIMIT:
        fail(
            f"--max-chars must be within 1..{GOAL_API_PROMPT_LIMIT} (the goal-API hard "
            "ceiling); it can tighten the budget, never raise or disable it"
        )
    packet_dir = Path(args.packet_dir)
    intent_path = packet_dir / "intent.md"
    driver_path = packet_dir / "driver.md"
    if not intent_path.is_file() or not driver_path.is_file():
        fail(f"packet must contain intent.md and driver.md: {packet_dir}", 2)

    if run_checker(packet_dir) != 0:
        fail(
            f"packet checker failed for {packet_dir}; repair the packet (e.g. "
            "refresh-digest) before emitting a dispatch prompt"
        )

    intent, _ = split_frontmatter(intent_path)
    driver, _ = split_frontmatter(driver_path)
    statuses = {str(intent.get("status", "draft")), str(driver.get("status", "draft"))}
    draft = statuses != {"validated"}
    if draft and not args.allow_draft:
        fail(
            f"packet is not validated (statuses: {sorted(statuses)}); run the checker "
            "plus independent validation first, or pass --allow-draft for a preview"
        )

    prompt = dispatch_prompt_text(packet_dir, intent, driver, draft)
    if len(prompt) > args.max_chars:
        fail(
            f"dispatch prompt is {len(prompt)} chars, over the max-chars budget of "
            f"{args.max_chars} (goal-API ceiling {GOAL_API_PROMPT_LIMIT}); tighten "
            "the packet's objective or behavior text"
        )
    print(prompt, end="")
    return 0


VERDICT_RE = re.compile(r"^(PASS|WARN)\b")


def command_mark_validated(args: argparse.Namespace) -> int:
    verdict = args.verdict.strip()
    if not VERDICT_RE.match(verdict):
        fail(
            "verdict must start with PASS or WARN (a FAIL or empty verdict cannot "
            "mark a packet validated); rerun the independent validator first"
        )
    packet_dir = Path(args.packet_dir)
    intent_path = packet_dir / "intent.md"
    driver_path = packet_dir / "driver.md"
    if not intent_path.is_file() or not driver_path.is_file():
        fail(f"packet must contain intent.md and driver.md: {packet_dir}", 2)

    original_intent = intent_path.read_bytes()
    original_driver = driver_path.read_bytes()

    # Parse BOTH files and render the stamped content fully in memory BEFORE
    # any write: a malformed driver must fail here, while the packet is still
    # byte-identical to its pre-transition state.
    intent, intent_body_text = split_frontmatter(intent_path)
    driver, body = split_frontmatter(driver_path)
    slug = str(intent.get("slug", ""))
    validate_slug(slug)

    statuses = {str(intent.get("status", "draft")), str(driver.get("status", "draft"))}
    if "closed" in statuses:
        fail(
            f"packet is closed (statuses: {sorted(statuses)}); closed is terminal — "
            "mark-validated must not reopen it; author a replacement packet instead"
        )

    intent["status"] = "validated"
    new_intent_text = render_markdown(intent, intent_body_text)
    digest = hashlib.sha256(new_intent_text.encode("utf-8")).hexdigest()
    intent_ref = canonical_intent_ref(slug)

    driver["status"] = "validated"
    driver.setdefault("intent_ref", {})
    driver["intent_ref"]["path"] = intent_ref
    driver["intent_ref"]["sha256"] = digest
    driver["intent_ref"]["schema_version"] = 1
    body = replace_or_append(
        body,
        r"(- Intent digest: `)[^`]+(`)",
        rf"\g<1>{digest}\2",
        f"- Intent digest: `{digest}`",
    )
    body = replace_or_append(
        body,
        r"^- Last validation verdict: .*$",
        f"- Last validation verdict: {verdict}",
        f"- Last validation verdict: {verdict}",
    )
    new_driver_text = render_markdown(driver, body)

    # The transition is checker-gated with NO opt-out: a packet may carry
    # status validated only if the checker accepts the exact stamped bytes.
    # ANY failure past this point — a write error or a checker rejection —
    # restores the originals so a broken packet is never left certified.
    try:
        intent_path.write_text(new_intent_text, encoding="utf-8")
        driver_path.write_text(new_driver_text, encoding="utf-8")
        checker_rc = run_checker(packet_dir)
    except BaseException:
        intent_path.write_bytes(original_intent)
        driver_path.write_bytes(original_driver)
        raise
    if checker_rc != 0:
        intent_path.write_bytes(original_intent)
        driver_path.write_bytes(original_driver)
        fail(
            f"checker rejected the stamped packet; {packet_dir} restored to its "
            "pre-transition state — repair the packet, then rerun mark-validated"
        )
    return 0


DISPOSITION_KINDS = ("closed", "dropped", "superseded")


def parse_dispositions(specs: list[str]) -> dict[str, tuple[str, str]]:
    dispositions: dict[str, tuple[str, str]] = {}
    for spec in specs:
        bead_id, eq, rest = spec.partition("=")
        kind, colon, detail = rest.partition(":")
        if not eq or not colon or kind not in DISPOSITION_KINDS:
            fail(
                f"malformed --candidate {spec!r}; expected <id>=closed:<evidence>, "
                "<id>=dropped:<reason>, or <id>=superseded:<by>"
            )
        if not detail.strip():
            fail(f"--candidate {spec!r} has empty disposition detail (evidence/reason/target)")
        if bead_id in dispositions:
            fail(f"duplicate disposition for candidate {bead_id}")
        dispositions[bead_id] = (kind, detail.strip())
    return dispositions


def command_close(args: argparse.Namespace) -> int:
    dispositions = parse_dispositions(args.candidate or [])
    packet_dir = Path(args.packet_dir)
    intent_path = packet_dir / "intent.md"
    driver_path = packet_dir / "driver.md"
    if not intent_path.is_file() or not driver_path.is_file():
        fail(f"packet must contain intent.md and driver.md: {packet_dir}", 2)

    original_intent = intent_path.read_bytes()
    original_driver = driver_path.read_bytes()

    # Parse BOTH files and render the closed content fully in memory BEFORE
    # any write: every refusal below leaves the packet byte-identical to its
    # pre-transition state.
    intent, intent_body_text = split_frontmatter(intent_path)
    driver, body = split_frontmatter(driver_path)
    slug = str(intent.get("slug", ""))
    validate_slug(slug)

    intent_status = str(intent.get("status", "draft"))
    driver_status = str(driver.get("status", "draft"))
    statuses = {intent_status, driver_status}
    if statuses - {"draft", "validated"}:
        fail(
            f"close is legal only from draft or validated (statuses: {sorted(statuses)}); "
            "a closed or superseded packet is terminal"
        )

    candidates = driver.get("candidate_beads") or []
    candidate_ids = [str(item.get("id", "")) for item in candidates if isinstance(item, dict)]
    if not candidate_ids:
        fail(f"driver declares no candidate beads to close: {driver_path}")
    unknown = sorted(set(dispositions) - set(candidate_ids))
    if unknown:
        fail(
            f"unknown candidate ids: {', '.join(unknown)} "
            f"(driver declares: {', '.join(candidate_ids)})"
        )
    missing = [cid for cid in candidate_ids if cid not in dispositions]
    if missing:
        fail(
            "every candidate bead needs an evidence-bound disposition; "
            f"missing: {', '.join(missing)}"
        )

    prior_status = (
        intent_status
        if intent_status == driver_status
        else f"intent={intent_status}, driver={driver_status}"
    )

    intent["status"] = "closed"
    new_intent_text = render_markdown(intent, intent_body_text)
    digest = hashlib.sha256(new_intent_text.encode("utf-8")).hexdigest()
    intent_ref = canonical_intent_ref(slug)

    driver["status"] = "closed"
    driver.setdefault("intent_ref", {})
    driver["intent_ref"]["path"] = intent_ref
    driver["intent_ref"]["sha256"] = digest
    driver["intent_ref"]["schema_version"] = 1
    body = replace_or_append(
        body,
        r"(- Intent digest: `)[^`]+(`)",
        rf"\g<1>{digest}\2",
        f"- Intent digest: `{digest}`",
    )
    stamp_lines = [f"- Closed: {utc_now()} (prior status: {prior_status})"]
    for cid in candidate_ids:
        kind, detail = dispositions[cid]
        stamp_lines.append(f"- Disposition {cid}: {kind} - {detail}")
    if not body.endswith("\n"):
        body += "\n"
    body += "\n" + "\n".join(stamp_lines) + "\n"
    new_driver_text = render_markdown(driver, body)

    # Same no-opt-out transactional shape as mark-validated: a packet may carry
    # status closed only if the checker accepts the exact stamped bytes. ANY
    # failure past this point restores the originals.
    try:
        intent_path.write_text(new_intent_text, encoding="utf-8")
        driver_path.write_text(new_driver_text, encoding="utf-8")
        checker_rc = run_checker(packet_dir)
    except BaseException:
        intent_path.write_bytes(original_intent)
        driver_path.write_bytes(original_driver)
        raise
    if checker_rc != 0:
        intent_path.write_bytes(original_intent)
        driver_path.write_bytes(original_driver)
        fail(
            f"checker rejected the closed packet; {packet_dir} restored to its "
            "pre-transition state — repair the packet, then rerun close"
        )
    return 0


def run_checker(packet_dir: Path) -> int:
    checker = repo_root() / "scripts" / "check-goal-design-packet.sh"
    return subprocess.run([str(checker), str(packet_dir)], check=False).returncode


def command_check(args: argparse.Namespace) -> int:
    return run_checker(Path(args.packet_dir))


def parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Create and maintain goal-design packets.")
    sub = parser.add_subparsers(dest="command", required=True)

    new = sub.add_parser("new", help="Create a packet and compute its driver digest.")
    new.add_argument("slug")
    new.add_argument("--output-root", default=".agents/goal-design")
    new.add_argument("--objective", required=True)
    new.add_argument("--why", default="This goal should drive validated loop work.")
    new.add_argument("--feature", default=None)
    new.add_argument("--scenario-id", default="S1")
    new.add_argument("--scenario-name", default=None)
    new.add_argument("--given", default="A checked goal-design packet exists")
    new.add_argument("--when", default="An agent uses it to drive loop work")
    new.add_argument("--then", default="The packet validates before implementation starts")
    new.add_argument("--bounded-context", default="bc-loop")
    new.add_argument("--in-scope", action="append", default=None)
    new.add_argument("--non-goal", action="append", default=None)
    new.add_argument("--rollback", default="Delete or supersede the packet before it drives work.")
    new.add_argument("--first-failing-proof", default="Define the first failing proof before implementation.")
    new.add_argument("--behavior", default=None)
    new.add_argument("--write-scope", action="append", default=None)
    new.add_argument("--close-signal", default="Checker and independent validator pass.")
    new.add_argument("--repo-path", action="append", default=None)
    new.add_argument("--prior-artifact", action="append", default=None)
    new.add_argument("--live-surface", action="append", default=None)
    new.add_argument("--stale-assumption", action="append", default=None)
    new.add_argument("--created-at", default=utc_now())
    new.add_argument("--force", action="store_true")
    new.add_argument("--no-check", dest="check", action="store_false")
    new.set_defaults(check=True, func=command_new)

    refresh = sub.add_parser("refresh-digest", help="Refresh driver intent_ref from intent.md.")
    refresh.add_argument("packet_dir")
    refresh.add_argument("--no-check", dest="check", action="store_false")
    refresh.set_defaults(check=True, func=command_refresh_digest)

    check = sub.add_parser("check", help="Run the packet checker.")
    check.add_argument("packet_dir")
    check.set_defaults(func=command_check)

    mark = sub.add_parser(
        "mark-validated",
        help="Record an independent validation verdict: flip status, stamp the driver, refresh the digest, re-check.",
    )
    mark.add_argument("packet_dir")
    mark.add_argument("--verdict", required=True)
    mark.set_defaults(func=command_mark_validated)

    close = sub.add_parser(
        "close",
        help="Close a packet with per-candidate evidence: flip statuses to closed, stamp dispositions, refresh the digest, re-check.",
    )
    close.add_argument("packet_dir")
    close.add_argument(
        "--candidate",
        action="append",
        default=None,
        metavar="ID=KIND:DETAIL",
        help="Repeatable disposition: <id>=closed:<evidence>, <id>=dropped:<reason>, "
        "or <id>=superseded:<by>; every driver candidate bead needs exactly one.",
    )
    close.set_defaults(func=command_close)

    prompt = sub.add_parser(
        "prompt",
        help="Emit a small dispatch prompt pointing a goal-API worker (codex/claude goals) at the packet.",
    )
    prompt.add_argument("packet_dir")
    prompt.add_argument("--allow-draft", action="store_true")
    prompt.add_argument("--max-chars", type=int, default=4000)
    prompt.set_defaults(func=command_prompt)

    return parser


def normalize_args(args: argparse.Namespace) -> argparse.Namespace:
    if args.command == "new":
        if args.feature is None:
            args.feature = args.objective
        if args.scenario_name is None:
            args.scenario_name = args.objective
        args.in_scope = args.in_scope or [args.objective]
        args.non_goal = args.non_goal or ["Do not implement outside the declared packet scope."]
        args.write_scope = args.write_scope or ["TBD"]
        args.repo_path = args.repo_path or ["AGENTS.md"]
        args.prior_artifact = args.prior_artifact or ["none"]
        args.live_surface = args.live_surface or ["git status --short"]
        args.stale_assumption = args.stale_assumption or ["The target behavior may already exist."]
    return args


def main(argv: list[str] | None = None) -> int:
    args = normalize_args(parser().parse_args(argv))
    return int(args.func(args))


if __name__ == "__main__":
    raise SystemExit(main())
