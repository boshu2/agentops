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


def test_rank_does_not_exceed_budget_for_oversized_first_item() -> None:
    oversized = mem.Memory(
        "fact:huge",
        mem.MemHeader(type="fact", created_at=NOW.isoformat()),
        "one two three four five six seven eight nine ten",
    )
    assert mem.rank([oversized], NOW, max_tokens=2) == []


def test_rank_skips_oversized_items_and_keeps_scanning_budget() -> None:
    oversized = mem.Memory(
        "feedback:huge",
        mem.MemHeader(type="feedback", created_at=NOW.isoformat(), access_count=10),
        "one two three four five six seven eight nine ten",
    )
    small = mem.Memory(
        "fact:small",
        mem.MemHeader(type="fact", created_at="2026-01-01T00:00:00+00:00"),
        "tiny",
    )
    assert [m.key for m in mem.rank([oversized, small], NOW, max_tokens=2)] == [
        "fact:small"
    ]


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
    key = led1["entries"][0]["key"]
    created = "2026-01-01T00:00:00+00:00"
    accessed = "2026-05-01T00:00:00+00:00"
    header, text = mem.parse(store.data[key])
    header.utility = 0.8
    header.access_count = 4
    header.created_at = created
    header.last_accessed = accessed
    store.remember(key, mem.render(header, text))

    led2 = import_memories.run_import(
        manifest, store, dry_run=False, sleep_s=0, now=NOW + timedelta(days=1)
    )
    header, _ = mem.parse(store.data[key])
    assert led1["imported"] == 1
    assert len(store.data) == 1  # second run updates in place, no duplicate
    assert led2["entries"][0]["source"] == f"file:{src}"
    assert header.utility == 0.8
    assert header.access_count == 4
    assert header.created_at == created
    assert header.last_accessed == accessed


def test_importer_keys_are_stable_for_duplicate_basenames(tmp_path: Path) -> None:
    src_a = tmp_path / "a" / "notes.md"
    src_b = tmp_path / "b" / "notes.md"
    src_a.parent.mkdir()
    src_b.parent.mkdir()
    src_a.write_text("a content")
    src_b.write_text("b content")
    item_a = {"layer": "agents_knowledge", "path": str(src_a)}
    item_b = {"layer": "agents_knowledge", "path": str(src_b)}
    store = mem.FakeStore()

    led1 = import_memories.run_import(
        {"keep": [item_a, item_b]}, store, dry_run=False, sleep_s=0, now=NOW
    )
    led2 = import_memories.run_import(
        {"keep": [item_b, item_a]}, store, dry_run=False, sleep_s=0, now=NOW
    )
    by_source1 = {entry["source"]: entry["key"] for entry in led1["entries"]}
    by_source2 = {entry["source"]: entry["key"] for entry in led2["entries"]}
    assert by_source1 == by_source2
    assert by_source1[f"file:{src_a}"] != by_source1[f"file:{src_b}"]

    import_memories.run_import({"keep": [item_b]}, store, dry_run=False, sleep_s=0, now=NOW)
    assert "a content" in store.data[by_source1[f"file:{src_a}"]]
    assert "b content" in store.data[by_source1[f"file:{src_b}"]]


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


def test_write_path_preserves_recall_metadata_on_upsert() -> None:
    store = mem.FakeStore()
    created = "2026-01-01T00:00:00+00:00"
    accessed = "2026-05-01T00:00:00+00:00"
    store.remember(
        "feedback:x",
        mem.render(
            mem.MemHeader(
                type="feedback",
                source="session:old",
                utility=0.8,
                access_count=4,
                created_at=created,
                last_accessed=accessed,
                maturity="established",
            ),
            "old body",
        ),
    )

    remember.write_memory(store, "feedback", "x", "new body", "session:new", "candidate", NOW)
    header, text = mem.parse(store.data["feedback:x"])
    assert header.utility == 0.8
    assert header.access_count == 4
    assert header.created_at == created
    assert header.last_accessed == accessed
    assert header.source == "session:new"
    assert header.maturity == "candidate"
    assert text == "new body"


def test_recall_round_trips_through_listing() -> None:
    store = mem.FakeStore()
    store.remember("fact:a", mem.render(mem.MemHeader(type="fact"), "alpha"))
    listing = json.dumps({m.key: m.text for m in store.list_memories()})
    assert "alpha" in listing


def test_parse_listing_keeps_markdown_headings_in_body() -> None:
    listing = "\n".join(
        [
            "### fact:a",
            mem.render(mem.MemHeader(type="fact"), "intro\n### Background\nbody"),
            "### feedback:b",
            mem.render(mem.MemHeader(type="feedback"), "second"),
        ]
    )
    parsed = mem.parse_listing(listing)
    assert [m.key for m in parsed] == ["fact:a", "feedback:b"]
    assert parsed[0].text == "intro\n### Background\nbody"


def test_parse_listing_keeps_legacy_untyped_keys() -> None:
    listing = "\n".join(
        [
            "### my-insight",
            "a legacy plain memory with no header",
            "### fact:a",
            mem.render(mem.MemHeader(type="fact"), "typed"),
        ]
    )
    parsed = mem.parse_listing(listing)
    assert [m.key for m in parsed] == ["my-insight", "fact:a"]
    assert parsed[0].header.type == "fact"
    assert parsed[0].text == "a legacy plain memory with no header"
