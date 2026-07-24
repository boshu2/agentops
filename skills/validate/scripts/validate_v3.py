#!/usr/bin/env python3
"""Command-line adapter for the exact RPI proof kernel.

Intent is always consumed from a pre-minted snapshot plus an expected digest.
No command in this adapter re-reads or re-snapshots a living intent source.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys

import kernel_v3 as kernel


def write_json(value: dict, output: str | None) -> None:
    if output:
        kernel.atomic_write_json(Path(output), value)
    else:
        sys.stdout.write(
            json.dumps(value, sort_keys=True, indent=2, ensure_ascii=False) + "\n"
        )


def arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)

    snapshot = commands.add_parser(
        "snapshot-intent",
        help="mint exact supplied bytes once under their SHA-256 digest",
    )
    snapshot.add_argument("--source", required=True, help="file path or - for stdin")
    snapshot.add_argument("--intent-dir", required=True)
    snapshot.add_argument("--expected-digest")

    consume = commands.add_parser(
        "consume-intent",
        help="verify a pre-minted snapshot against the expected exact digest",
    )
    consume.add_argument("--intent-snapshot", required=True)
    consume.add_argument("--expected-digest", required=True)

    manifest = commands.add_parser("manifest", help="compute subject-manifest.v2")
    manifest.add_argument("--root", required=True)
    manifest.add_argument("--observation-roots", required=True)
    manifest.add_argument("--exclude", action="append", default=[])
    manifest.add_argument("--output")

    verify_manifest = commands.add_parser(
        "verify-manifest",
        help="recompute a frozen subject-manifest.v2 and fail on mutation",
    )
    verify_manifest.add_argument("--root", required=True)
    verify_manifest.add_argument("--manifest", required=True)

    scope = commands.add_parser(
        "freeze-scope",
        help="freeze stable criterion IDs, scope classes, and prior exclusions",
    )
    scope.add_argument("--source", required=True)
    scope.add_argument("--intent-digest", required=True)
    scope.add_argument("--output")

    effect = commands.add_parser(
        "derive-effect",
        help="derive actual changes and deletions from before/final manifests",
    )
    effect.add_argument("--before-manifest", required=True)
    effect.add_argument("--final-manifest", required=True)
    effect.add_argument("--scope-index", required=True)
    effect.add_argument("--check-receipt-ref", action="append", default=[])
    effect.add_argument("--output")

    store = commands.add_parser(
        "store-verdict",
        help="bind runtime facts and atomically persist one verdict.v3",
    )
    store.add_argument("--draft", required=True)
    store.add_argument("--repository", default=".")
    store.add_argument("--verdict-dir", required=True)
    store.add_argument("--invocation-id", required=True)
    store.add_argument("--judgment-id", required=True)
    store.add_argument("--intent-snapshot", required=True)
    store.add_argument("--expected-intent-digest", required=True)
    store.add_argument("--before-manifest", required=True)
    store.add_argument("--final-manifest", required=True)
    store.add_argument("--scope-index", required=True)
    store.add_argument("--effect-receipt", required=True)
    store.add_argument("--author-context-id", required=True)
    store.add_argument("--validator-context-id", required=True)
    store.add_argument("--freshness-source", required=True, choices=("runtime", "caller"))
    store.add_argument("--freshness-attester-id", required=True)

    return parser.parse_args()


def repo_ref(repository: Path, raw: str) -> str:
    path = Path(raw)
    if path.is_absolute():
        try:
            return path.resolve().relative_to(repository.resolve()).as_posix()
        except ValueError as exc:
            raise kernel.ContractError(f"path is outside repository: {path}") from exc
    return kernel.normalize_rel(raw)


def main() -> int:
    args = arguments()
    try:
        if args.command == "snapshot-intent":
            payload = (
                sys.stdin.buffer.read()
                if args.source == "-"
                else Path(args.source).read_bytes()
            )
            path, digest, existed = kernel.mint_intent_snapshot(
                payload,
                Path(args.intent_dir),
                expected_digest=args.expected_digest,
            )
            write_json(
                {
                    "intent_ref": str(path),
                    "intent_digest": digest,
                    "idempotent": existed,
                },
                None,
            )
        elif args.command == "consume-intent":
            payload = kernel.consume_intent_snapshot(
                Path(args.intent_snapshot),
                args.expected_digest,
            )
            write_json(
                {"intent_digest": kernel.sha256(payload), "byte_length": len(payload)},
                None,
            )
        elif args.command == "manifest":
            roots = kernel.load_json(Path(args.observation_roots))
            if set(roots) != {"observation_roots"}:
                raise kernel.ContractError(
                    "observation roots file must contain only observation_roots"
                )
            value = kernel.build_manifest_v2(
                Path(args.root),
                roots["observation_roots"],
                args.exclude,
            )
            write_json(value, args.output)
        elif args.command == "verify-manifest":
            kernel.verify_manifest_v2(
                kernel.load_json(Path(args.manifest)),
                Path(args.root),
            )
            write_json({"result": "PASS"}, None)
        elif args.command == "freeze-scope":
            source = kernel.load_json(Path(args.source))
            if set(source) != {
                "frozen_at",
                "criteria",
                "scope_classes",
                "declared_exclusions",
            }:
                raise kernel.ContractError("scope freeze source has invalid fields")
            value = kernel.freeze_scope_index(
                intent_digest=args.intent_digest,
                frozen_at=source["frozen_at"],
                criteria=source["criteria"],
                scope_classes=source["scope_classes"],
                declared_exclusions=source["declared_exclusions"],
            )
            write_json(value, args.output)
        elif args.command == "derive-effect":
            references = []
            for raw in args.check_receipt_ref:
                reference, separator, digest = raw.partition("=")
                if separator != "=":
                    raise kernel.ContractError(
                        "check receipt refs use REF=DIGEST"
                    )
                references.append({"ref": reference, "digest": digest})
            value = kernel.derive_effect_receipt(
                kernel.load_json(Path(args.before_manifest)),
                kernel.load_json(Path(args.final_manifest)),
                kernel.load_json(Path(args.scope_index)),
                references,
            )
            write_json(value, args.output)
        elif args.command == "store-verdict":
            repository = Path(args.repository).resolve()
            artifact, path, existed = kernel.store_verdict_v3(
                kernel.load_json(Path(args.draft)),
                repository=repository,
                destination=Path(args.verdict_dir),
                invocation_id=args.invocation_id,
                judgment_id=args.judgment_id,
                intent_ref=repo_ref(repository, args.intent_snapshot),
                expected_intent_digest=args.expected_intent_digest,
                before_manifest_ref=repo_ref(repository, args.before_manifest),
                final_manifest_ref=repo_ref(repository, args.final_manifest),
                scope_index_ref=repo_ref(repository, args.scope_index),
                effect_receipt_ref=repo_ref(repository, args.effect_receipt),
                author_context_id=args.author_context_id,
                validator_context_id=args.validator_context_id,
                freshness_source=args.freshness_source,
                freshness_attester_id=args.freshness_attester_id,
            )
            write_json(
                {
                    "artifact_digest": artifact["artifact_digest"],
                    "idempotent": existed,
                    "path": str(path),
                    "verdict": artifact["verdict"],
                },
                None,
            )
        return 0
    except (kernel.ContractError, OSError, json.JSONDecodeError) as exc:
        print(f"validate-v3: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
