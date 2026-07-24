from __future__ import annotations

import fcntl
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest

import jsonschema


REPO_ROOT = Path(__file__).resolve().parents[2]
PUBLISHER = REPO_ROOT / "scripts" / "publish-generated-projections.py"


def canonical_bytes(value: object) -> bytes:
    return json.dumps(
        value,
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=False,
    ).encode("utf-8")


def finalize(value: dict) -> dict:
    result = dict(value)
    result["artifact_digest"] = hashlib.sha256(canonical_bytes(result)).hexdigest()
    return result


def filesystem_snapshot(root: Path) -> dict[str, tuple]:
    result: dict[str, tuple] = {}
    for path in sorted(root.rglob("*")):
        relative = path.relative_to(root).as_posix()
        if relative == ".agents" or relative.startswith(".agents/"):
            continue
        metadata = path.lstat()
        mode = stat.S_IMODE(metadata.st_mode)
        if path.is_symlink():
            result[relative] = ("symlink", mode, os.readlink(path))
        elif path.is_file():
            result[relative] = ("file", mode, path.read_bytes())
        elif path.is_dir():
            result[relative] = ("directory", mode)
    return result


class PublishGeneratedProjectionsTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.repository = Path(self.temporary.name)
        (self.repository / "scripts").mkdir()
        (self.repository / "source").mkdir()
        (self.repository / "source/value.txt").write_text("rendered-v1\n")
        (self.repository / "source/link-target.txt").write_text("target\n")
        render = self.repository / "scripts/render.py"
        render.write_text(
            """\
from pathlib import Path
import os
import shutil

root = Path.cwd()
generated = root / "generated"
generated.mkdir(exist_ok=True)
payload = (root / "source/value.txt").read_bytes()
(generated / "dirty.txt").write_bytes(payload)
tool = generated / "tool.sh"
tool.write_bytes(b"#!/bin/sh\\nprintf rendered\\\\n\\n")
tool.chmod(0o755)
link = generated / "link"
if link.exists() or link.is_symlink():
    link.unlink()
os.symlink("../source/link-target.txt", link)
(generated / "missing.txt").write_bytes(b"created\\n")
tree = generated / "tree"
if tree.exists() or tree.is_symlink():
    if tree.is_dir() and not tree.is_symlink():
        shutil.rmtree(tree)
    else:
        tree.unlink()
tree.mkdir()
(tree / "leaf.txt").write_bytes(payload)
""",
            encoding="utf-8",
        )
        generated = self.repository / "generated"
        generated.mkdir()
        dirty = generated / "dirty.txt"
        dirty.write_bytes(b"operator dirty bytes\n")
        dirty.chmod(0o640)
        tool = generated / "tool.sh"
        tool.write_bytes(b"old executable\n")
        tool.chmod(0o751)
        os.symlink("operator-owned-target", generated / "link")
        self.owner_map_path = self.repository / "owner-map.json"
        owner_map = finalize(
            {
                "schema_version": "generated-projection-owner-map.v1",
                "publication": {
                    "lock_ref": "owner-map.json",
                    "manifest_ref": ".agents/publication/current.json",
                    "receipt_dir_ref": ".agents/publication/receipts",
                    "transaction_dir_ref": ".agents/publication/transactions",
                },
                "owners": [
                    {
                        "id": "fixture",
                        "generator_refs": ["scripts/render.py"],
                        "source_refs": ["source"],
                        "commands": [["python3", "scripts/render.py"]],
                        "targets": [
                            {
                                "path": "generated/dirty.txt",
                                "kind": "file",
                                "allow_missing": False,
                            },
                            {
                                "path": "generated/link",
                                "kind": "symlink",
                                "allow_missing": False,
                            },
                            {
                                "path": "generated/missing.txt",
                                "kind": "file",
                                "allow_missing": False,
                            },
                            {
                                "path": "generated/tool.sh",
                                "kind": "file",
                                "allow_missing": False,
                            },
                            {
                                "path": "generated/tree",
                                "kind": "tree",
                                "allow_missing": False,
                            },
                        ],
                    }
                ],
            }
        )
        self.owner_map_path.write_bytes(canonical_bytes(owner_map) + b"\n")

    def run_publisher(
        self,
        *args: str,
        timeout: float = 30,
    ) -> subprocess.CompletedProcess[str]:
        environment = dict(os.environ)
        environment["PYTHONDONTWRITEBYTECODE"] = "1"
        return subprocess.run(
            [
                sys.executable,
                "-B",
                str(PUBLISHER),
                "--repository",
                str(self.repository),
                "--owner-map",
                "owner-map.json",
                *args,
            ],
            check=False,
            capture_output=True,
            text=True,
            env=environment,
            timeout=timeout,
        )

    def decoded_stdout(self, result: subprocess.CompletedProcess[str]) -> dict:
        self.assertTrue(result.stdout.strip(), result.stderr)
        return json.loads(result.stdout)

    def test_check_is_read_only_and_write_has_exact_parity_and_idempotence(
        self,
    ) -> None:
        before = filesystem_snapshot(self.repository)

        checked = self.run_publisher("--check")

        self.assertEqual(checked.returncode, 1, checked.stderr)
        checked_receipt = self.decoded_stdout(checked)
        self.assertEqual(checked_receipt["result"], "DRIFT")
        self.assertEqual(filesystem_snapshot(self.repository), before)

        written = self.run_publisher("--write")

        self.assertEqual(written.returncode, 0, written.stderr)
        written_receipt = self.decoded_stdout(written)
        self.assertEqual(written_receipt["result"], "PUBLISHED")
        self.assertEqual(
            written_receipt["rendered_digest"],
            checked_receipt["rendered_digest"],
        )
        generated = self.repository / "generated"
        self.assertEqual(
            (generated / "dirty.txt").read_bytes(),
            b"rendered-v1\n",
        )
        self.assertEqual(
            stat.S_IMODE((generated / "tool.sh").stat().st_mode),
            0o755,
        )
        self.assertEqual(
            os.readlink(generated / "link"),
            "../source/link-target.txt",
        )
        self.assertEqual((generated / "missing.txt").read_bytes(), b"created\n")
        manifest = self.repository / ".agents/publication/current.json"
        manifest_before = manifest.read_bytes()
        manifest_value = json.loads(manifest_before)
        self.assertEqual(
            manifest_value["schema_version"],
            "generated-projection-publication.v1",
        )
        self.assertEqual(
            manifest_value["artifact_digest"],
            written_receipt["publication_manifest_digest"],
        )
        publication_schema = json.loads(
            (
                REPO_ROOT / "schemas/generated-projection-publication.v1.schema.json"
            ).read_text()
        )
        jsonschema.validate(manifest_value, publication_schema)
        receipt_path = self.repository / written_receipt["receipt_path"]
        receipt_value = json.loads(receipt_path.read_text())
        receipt_schema = json.loads(
            (REPO_ROOT / "schemas/publication-receipt.v1.schema.json").read_text()
        )
        jsonschema.validate(receipt_value, receipt_schema)

        second = self.run_publisher("--write")

        self.assertEqual(second.returncode, 0, second.stderr)
        self.assertEqual(self.decoded_stdout(second)["result"], "IDEMPOTENT")
        self.assertEqual(manifest.read_bytes(), manifest_before)
        clean = self.run_publisher("--check")
        self.assertEqual(clean.returncode, 0, clean.stderr)
        self.assertEqual(self.decoded_stdout(clean)["result"], "CLEAN")

    def test_unowned_tree_collision_aborts_before_any_live_mutation(self) -> None:
        tree = self.repository / "generated/tree"
        tree.mkdir()
        (tree / "intruder.txt").write_text("caller owned\n")
        before = filesystem_snapshot(self.repository)

        result = self.run_publisher("--write")

        self.assertEqual(result.returncode, 2)
        self.assertIn("unowned projection collision before mutation", result.stderr)
        self.assertEqual(filesystem_snapshot(self.repository), before)
        self.assertFalse(
            (self.repository / ".agents/publication/current.json").exists()
        )

    def test_injected_partial_failure_restores_bytes_modes_kinds_and_missing(
        self,
    ) -> None:
        before = filesystem_snapshot(self.repository)

        result = self.run_publisher("--write", "--fail-after-target", "5")

        self.assertEqual(result.returncode, 2)
        self.assertIn("injected failure after target 5", result.stderr)
        self.assertEqual(filesystem_snapshot(self.repository), before)
        self.assertFalse(
            (self.repository / ".agents/publication/current.json").exists()
        )
        transaction_root = self.repository / ".agents/publication/transactions"
        self.assertEqual(
            list(transaction_root.iterdir()) if transaction_root.exists() else [],
            [],
        )

    def test_abrupt_exit_is_recovered_from_bundle_without_git(self) -> None:
        before = filesystem_snapshot(self.repository)

        interrupted = self.run_publisher(
            "--write",
            "--abrupt-after-target",
            "3",
        )

        self.assertEqual(interrupted.returncode, 97)
        self.assertNotEqual(filesystem_snapshot(self.repository), before)
        self.assertFalse(
            (self.repository / ".agents/publication/current.json").exists()
        )
        recovered = self.run_publisher("--recover-only")
        self.assertEqual(recovered.returncode, 0, recovered.stderr)
        recovery_receipt = self.decoded_stdout(recovered)
        self.assertEqual(recovery_receipt["result"], "RECOVERED")
        self.assertEqual(filesystem_snapshot(self.repository), before)
        transactions = self.repository / ".agents/publication/transactions"
        self.assertEqual(list(transactions.iterdir()), [])

    def test_lock_serializes_publishers_without_changing_lock_file(self) -> None:
        lock_before = self.owner_map_path.read_bytes()
        descriptor = os.open(self.owner_map_path, os.O_RDONLY)
        self.addCleanup(os.close, descriptor)
        fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        self.addCleanup(fcntl.flock, descriptor, fcntl.LOCK_UN)

        result = self.run_publisher(
            "--check",
            "--lock-timeout",
            "0.01",
        )

        self.assertEqual(result.returncode, 2)
        self.assertIn("publication lock timed out", result.stderr)
        self.assertEqual(self.owner_map_path.read_bytes(), lock_before)

    def test_recovery_journal_cannot_expand_owner_scope(self) -> None:
        interrupted = self.run_publisher(
            "--write",
            "--abrupt-after-target",
            "1",
        )
        self.assertEqual(interrupted.returncode, 97)
        transaction_root = self.repository / ".agents/publication/transactions"
        journal_path = next(transaction_root.iterdir()) / "recovery.json"
        journal = json.loads(journal_path.read_text())
        journal["targets"][0]["path"] = "source/value.txt"
        journal.pop("artifact_digest")
        journal = finalize(journal)
        journal_path.write_bytes(canonical_bytes(journal) + b"\n")
        before_refusal = filesystem_snapshot(self.repository)

        refused = self.run_publisher("--recover-only")

        self.assertEqual(refused.returncode, 2)
        self.assertIn(
            "pending recovery does not bind the current owner map",
            refused.stderr,
        )
        self.assertEqual(filesystem_snapshot(self.repository), before_refusal)

    def test_owner_map_unknown_field_fails_closed(self) -> None:
        owner_map = json.loads(self.owner_map_path.read_text())
        owner_map["unexpected"] = True
        owner_map.pop("artifact_digest")
        self.owner_map_path.write_bytes(canonical_bytes(finalize(owner_map)) + b"\n")

        result = self.run_publisher("--validate-owner-map")

        self.assertEqual(result.returncode, 2)
        self.assertIn("unknown fields", result.stderr)


if __name__ == "__main__":
    unittest.main()
