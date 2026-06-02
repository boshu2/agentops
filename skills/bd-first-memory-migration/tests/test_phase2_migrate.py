"""L2 tests for Phase 2 migrate — wrapper, recall, generator, importer, write-path.

Hermetic: uses mem.FakeStore (in-memory) and an explicit ``now`` so nothing
touches the real bd database or the wall clock.
"""

from __future__ import annotations

import json
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

_SCRIPTS = Path(__file__).resolve().parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS))

import gen_memory_md  # noqa: E402
import import_memories  # noqa: E402
import mem  # noqa: E402
import recall  # noqa: E402
import remember  # noqa: E402

NOW = datetime(2026, 6, 2, tzinfo=timezone.utc)


def test_parse_render_roundtrip() -> None:
    header = mem.MemHeader(type="feedback", source="commit:abc", utility=0.5, access_count=3)
    body = mem.render(header, "the lesson body")
    parsed, text = mem.parse(body)
    assert parsed.type == "feedback"
    assert parsed.source == "commit:abc"
    assert parsed.utility == 0.5
    assert parsed.access_count == 3
    assert text == "the lesson body"


def test_parse_missing_header_defaults_to_fact() -> None:
    header, text = mem.parse("a legacy plain memory with no header")
    assert header.type == "fact"
    assert header.maturity == "provisional"
    assert text == "a legacy plain memory with no header"


def test_rank_prefers_recent_and_high_access() -> None:
    old = mem.Memory("fact:old", mem.MemHeader(type="fact", created_at="2024-01-01T00:00:00+00:00"))
    fresh = mem.Memory(
        "fact:fresh",
        mem.MemHeader(type="fact", created_at="2026-06-01T00:00:00+00:00", access_count=5),
    )
    ranked = mem.rank([old, fresh], NOW)
    assert ranked[0].key == "fact:fresh"


def test_rank_excludes_superseded() -> None:
    live = mem.Memory("fact:a", mem.MemHeader(type="fact", created_at=NOW.isoformat()))
    dead = mem.Memory("fact:b", mem.MemHeader(type="fact", superseded_by="fact:a"))
    ranked = mem.rank([live, dead], NOW)
    assert [m.key for m in ranked] == ["fact:a"]


def test_episodic_decays_faster_than_feedback() -> None:
    ts = (NOW - timedelta(days=30)).isoformat()
    epi = mem.Memory("episodic:x", mem.MemHeader(type="episodic", created_at=ts))
    fb = mem.Memory("feedback:y", mem.MemHeader(type="feedback", created_at=ts))
    assert mem.score(fb, NOW) > mem.score(epi, NOW)


def test_recall_bumps_access_count() -> None:
    store = mem.FakeStore()
    body = mem.render(mem.MemHeader(type="fact", created_at=NOW.isoformat()), "body")
    store.remember("fact:a", body)
    results = recall.recall(store, NOW, bump=True)
    assert len(results) == 1
    after, _ = mem.parse(store.data["fact:a"])
    assert after.access_count == 1


def test_gen_memory_md_is_thin_and_grouped() -> None:
    store = mem.FakeStore()
    store.remember("feedback:a", mem.render(mem.MemHeader(type="feedback"), "be careful"))
    store.remember("fact:b", mem.render(mem.MemHeader(type="fact"), "the sky is blue"))
    index = gen_memory_md.render_index(store.list_memories(), max_lines=180)
    assert "do not hand-edit" in index
    assert "## feedback" in index
    assert "`feedback:a` — be careful" in index
    assert index.count("\n") < 200


def test_importer_is_idempotent(tmp_path: Path) -> None:
    src = tmp_path / "real.md"
    src.write_text("authored content")
    manifest = {"keep": [{"layer": "agents_knowledge", "path": str(src)}]}
    store = mem.FakeStore()
    led1 = import_memories.run_import(manifest, store, dry_run=False, sleep_s=0, now=NOW)
    led2 = import_memories.run_import(manifest, store, dry_run=False, sleep_s=0, now=NOW)
    assert led1["imported"] == 1
    assert len(store.data) == 1  # second run updates in place, no duplicate
    assert led2["entries"][0]["source"] == f"file:{src}"


def test_importer_dry_run_writes_nothing(tmp_path: Path) -> None:
    src = tmp_path / "k.md"
    src.write_text("x")
    manifest = {"keep": [{"layer": "agents_knowledge", "path": str(src)}]}
    store = mem.FakeStore()
    ledger = import_memories.run_import(manifest, store, dry_run=True, sleep_s=0, now=NOW)
    assert ledger["dry_run"] is True
    assert ledger["imported"] == 1
    assert store.data == {}


def test_write_path_stamps_provenance() -> None:
    store = mem.FakeStore()
    key = remember.write_memory(
        store, "feedback", "x", "body", "session:s1", "candidate", NOW
    )
    assert key == "feedback:x"
    header, text = mem.parse(store.data[key])
    assert header.source == "session:s1"
    assert header.maturity == "candidate"
    assert text == "body"


def test_recall_round_trips_through_listing() -> None:
    store = mem.FakeStore()
    store.remember("fact:a", mem.render(mem.MemHeader(type="fact"), "alpha"))
    listing = json.dumps({m.key: m.text for m in store.list_memories()})
    assert "alpha" in listing
