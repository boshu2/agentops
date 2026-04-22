"""Unit tests for scripts/rpi/generate-context-shards.py.

Uses vanilla unittest + importlib so it runs without pytest installed.
Focus: pure helpers (unit packing, file classification, manifest verification).
"""

from __future__ import annotations

import importlib.util
import os
import sys
import tempfile
import unittest
from dataclasses import asdict
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT_PATH = REPO_ROOT / "scripts" / "rpi" / "generate-context-shards.py"


def _load_module():
    name = "generate_context_shards"
    spec = importlib.util.spec_from_file_location(name, SCRIPT_PATH)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    # Register in sys.modules so @dataclass can resolve cls.__module__
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


mod = _load_module()


class TestIsBinary(unittest.TestCase):
    def test_text_file_is_not_binary(self):
        with tempfile.NamedTemporaryFile(delete=False, suffix=".txt") as f:
            f.write(b"hello world\n")
            path = Path(f.name)
        try:
            self.assertFalse(mod.is_binary(path))
        finally:
            path.unlink()

    def test_file_with_null_byte_is_binary(self):
        with tempfile.NamedTemporaryFile(delete=False, suffix=".bin") as f:
            f.write(b"abc\x00def")
            path = Path(f.name)
        try:
            self.assertTrue(mod.is_binary(path))
        finally:
            path.unlink()

    def test_missing_file_not_classified_binary(self):
        # OSError path: returns False
        self.assertFalse(mod.is_binary(Path("/nonexistent/xyz")))


class TestCountLines(unittest.TestCase):
    def test_empty_returns_one(self):
        with tempfile.NamedTemporaryFile(delete=False) as f:
            f.write(b"")
            path = Path(f.name)
        try:
            self.assertEqual(mod.count_lines(path), 1)
        finally:
            path.unlink()

    def test_counts_newlines(self):
        with tempfile.NamedTemporaryFile(delete=False) as f:
            f.write(b"a\nb\nc\n")
            path = Path(f.name)
        try:
            self.assertEqual(mod.count_lines(path), 3)
        finally:
            path.unlink()

    def test_counts_line_without_trailing_newline(self):
        with tempfile.NamedTemporaryFile(delete=False) as f:
            f.write(b"a\nb")
            path = Path(f.name)
        try:
            self.assertEqual(mod.count_lines(path), 2)
        finally:
            path.unlink()


class TestUnitsForFile(unittest.TestCase):
    def test_small_text_file_returns_single_full_unit(self):
        with tempfile.NamedTemporaryFile(delete=False, suffix=".txt") as f:
            f.write(b"one\ntwo\n")
            path = Path(f.name)
        try:
            units = mod.units_for_file(str(path), chunk_target_bytes=10_000)
            self.assertEqual(len(units), 1)
            u = units[0]
            self.assertEqual(u.kind, "text-full")
            self.assertEqual(u.chunk_index, 1)
            self.assertEqual(u.chunk_count, 1)
        finally:
            path.unlink()

    def test_large_text_splits_into_chunks(self):
        with tempfile.NamedTemporaryFile(delete=False, suffix=".txt") as f:
            # Write 200 lines
            for i in range(200):
                f.write(f"line {i}\n".encode())
            path = Path(f.name)
        try:
            units = mod.units_for_file(str(path), chunk_target_bytes=100)
            self.assertGreater(len(units), 1)
            for u in units:
                self.assertEqual(u.kind, "text-chunk")
                self.assertGreaterEqual(u.line_start, 1)
                self.assertGreaterEqual(u.line_end, u.line_start)
        finally:
            path.unlink()

    def test_binary_returns_metadata_unit(self):
        with tempfile.NamedTemporaryFile(delete=False, suffix=".bin") as f:
            f.write(b"a\x00b")
            path = Path(f.name)
        try:
            units = mod.units_for_file(str(path), chunk_target_bytes=100)
            self.assertEqual(len(units), 1)
            self.assertEqual(units[0].kind, "binary-metadata")
        finally:
            path.unlink()


class TestPackShards(unittest.TestCase):
    def test_empty_units_returns_no_shards(self):
        shards = mod.pack_shards([], max_units=10, max_bytes=1000)
        self.assertEqual(shards, [])

    def test_single_shard_when_within_limits(self):
        units = [
            mod.ReadUnit(path=f"f{i}", kind="text-full", size_bytes=10, budget_bytes=10)
            for i in range(3)
        ]
        shards = mod.pack_shards(units, max_units=10, max_bytes=1000)
        self.assertEqual(len(shards), 1)
        self.assertEqual(shards[0].unit_count, 3)

    def test_splits_when_max_units_exceeded(self):
        units = [
            mod.ReadUnit(path=f"f{i}", kind="text-full", size_bytes=1, budget_bytes=1)
            for i in range(5)
        ]
        shards = mod.pack_shards(units, max_units=2, max_bytes=1000)
        self.assertGreaterEqual(len(shards), 3)
        for s in shards:
            self.assertLessEqual(s.unit_count, 2)

    def test_splits_when_max_bytes_exceeded(self):
        units = [
            mod.ReadUnit(path=f"f{i}", kind="text-full", size_bytes=100, budget_bytes=100)
            for i in range(5)
        ]
        shards = mod.pack_shards(units, max_units=100, max_bytes=150)
        self.assertGreaterEqual(len(shards), 3)

    def test_estimated_tokens_from_bytes(self):
        units = [mod.ReadUnit(path="f", kind="text-full", size_bytes=400, budget_bytes=400)]
        shards = mod.pack_shards(units, max_units=10, max_bytes=10000)
        self.assertEqual(shards[0].estimated_tokens, 100)  # 400 / 4


class TestVerifyManifest(unittest.TestCase):
    def _mk_manifest(self, files, shards_data):
        return {
            "shards": shards_data,
            "totals": {"files": len(files)},
        }

    def test_manifest_with_coverage_passes(self):
        files = ["a.txt", "b.txt"]
        shards = [
            {
                "shard_id": 1,
                "units": [
                    {"path": "a.txt", "budget_bytes": 10},
                    {"path": "b.txt", "budget_bytes": 10},
                ],
                "budget_bytes": 20,
            }
        ]
        manifest = self._mk_manifest(files, shards)
        # Should not raise
        mod.verify_manifest(manifest, files, max_units=10, max_bytes=1000)

    def test_raises_on_coverage_gap(self):
        files = ["a.txt", "b.txt"]
        shards = [
            {
                "shard_id": 1,
                "units": [{"path": "a.txt", "budget_bytes": 10}],
                "budget_bytes": 10,
            }
        ]
        with self.assertRaises(ValueError) as ctx:
            mod.verify_manifest(self._mk_manifest(files, shards), files, max_units=10, max_bytes=1000)
        self.assertIn("coverage gap", str(ctx.exception))

    def test_raises_on_empty_shards(self):
        with self.assertRaises(ValueError):
            mod.verify_manifest({"shards": []}, [], max_units=10, max_bytes=100)

    def test_raises_on_unknown_file(self):
        files = ["a.txt"]
        shards = [
            {
                "shard_id": 1,
                "units": [{"path": "unknown.txt", "budget_bytes": 10}],
                "budget_bytes": 10,
            }
        ]
        with self.assertRaises(ValueError):
            mod.verify_manifest(self._mk_manifest(files, shards), files, max_units=10, max_bytes=1000)

    def test_raises_when_max_units_exceeded(self):
        files = ["a.txt"]
        shards = [
            {
                "shard_id": 1,
                "units": [
                    {"path": "a.txt", "budget_bytes": 1},
                    {"path": "a.txt", "budget_bytes": 1},
                    {"path": "a.txt", "budget_bytes": 1},
                ],
                "budget_bytes": 3,
            }
        ]
        with self.assertRaises(ValueError) as ctx:
            mod.verify_manifest(self._mk_manifest(files, shards), files, max_units=2, max_bytes=1000)
        self.assertIn("max_units", str(ctx.exception))

    def test_raises_when_max_bytes_exceeded(self):
        files = ["a.txt"]
        shards = [
            {
                "shard_id": 1,
                "units": [{"path": "a.txt", "budget_bytes": 500}],
                "budget_bytes": 500,
            }
        ]
        with self.assertRaises(ValueError):
            mod.verify_manifest(self._mk_manifest(files, shards), files, max_units=10, max_bytes=100)


class TestBuildManifest(unittest.TestCase):
    def test_builds_manifest_with_expected_structure(self):
        tmp = tempfile.mkdtemp()
        try:
            a = Path(tmp) / "a.txt"
            b = Path(tmp) / "b.txt"
            a.write_text("aaa\nbbb\n")
            b.write_text("ccc\n")

            manifest = mod.build_manifest([str(a), str(b)], max_units=10, max_bytes=1_000_000)

            self.assertIn("generated_at", manifest)
            self.assertIn("shards", manifest)
            self.assertEqual(manifest["totals"]["files"], 2)
            self.assertGreater(len(manifest["shards"]), 0)
        finally:
            for f in Path(tmp).iterdir():
                f.unlink()
            os.rmdir(tmp)


class TestReadUnitDataclass(unittest.TestCase):
    def test_asdict_roundtrip(self):
        u = mod.ReadUnit(path="x", kind="text-full", size_bytes=10, budget_bytes=10)
        d = asdict(u)
        self.assertEqual(d["path"], "x")
        self.assertEqual(d["kind"], "text-full")
        self.assertIsNone(d["line_start"])


if __name__ == "__main__":
    unittest.main()
