"""L2 tests for Phase 1 audit — scanner + classifier over a fixture home.

Hermetic: exercises only the filesystem-backed functions (no `bd`/`ao` CLI
dependency), per the repo testing rules. Builds a temp home that mirrors the
real fragmentation modes (authored knowledge, pending-extract dups, a bloated
MEMORY.md index, an authored topic file) and asserts exact verdicts.
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

_SCRIPTS = Path(__file__).resolve().parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS))

import audit_report  # noqa: E402
import audit_scan  # noqa: E402
import classify  # noqa: E402


@pytest.fixture
def home(tmp_path: Path) -> Path:
    """A fixture home reproducing the four fragmentation modes."""
    # authored knowledge — a keeper
    knowledge = tmp_path / ".agents" / "knowledge"
    knowledge.mkdir(parents=True)
    (knowledge / "real.md").write_text("# authored knowledge\nload-bearing.\n")

    # learnings — pending-extract dups + one unique authored learning
    learnings = tmp_path / ".agents" / "learnings"
    learnings.mkdir(parents=True)
    (learnings / "x-pend-2026-01-01-a.md").write_text("DUPED PAYLOAD\n")
    (learnings / "x-pend-2026-01-01-b.md").write_text("DUPED PAYLOAD\n")  # exact dup content
    (learnings / "2026-05-16-unique.md").write_text("a genuinely unique learning\n")

    # claude memory — a bloated index + an authored topic file
    mem = tmp_path / ".claude" / "projects" / "p" / "memory"
    mem.mkdir(parents=True)
    (mem / "MEMORY.md").write_text("\n".join(f"line {i}" for i in range(250)))
    (mem / "feedback_topic.md").write_text("---\nname: topic\n---\nbody\n")
    return tmp_path


def test_scanner_flags_bloated_memory_index(home: Path) -> None:
    rep = audit_scan.scan_claude_memory(home, sample_cap=100)
    assert rep.present is True
    assert rep.file_count == 2
    assert any("250 lines" in n for n in rep.notes), rep.notes


def test_scanner_counts_learnings_and_dups(home: Path) -> None:
    learnings = home / ".agents" / "learnings"
    rep = audit_scan.scan_dir_layer("agents_learnings", learnings, sample_cap=100)
    assert rep.file_count == 3
    assert rep.dup_files == 1  # one of the two identical -pend- files is a duplicate
    assert rep.dup_groups == 1


def test_classify_knowledge_all_keep(home: Path) -> None:
    items = classify.classify_knowledge(home / ".agents" / "knowledge")
    assert len(items) == 1
    assert items[0].verdict == "keep"
    assert items[0].reason == "authored knowledge"


def test_classify_learnings_drops_pend_and_dups(home: Path) -> None:
    keep, drop = classify.classify_learnings(home / ".agents" / "learnings", sample_cap=100)
    keep_names = {Path(i.path).name for i in keep}
    drop_reasons = sorted(i.reason for i in drop)
    assert keep_names == {"2026-05-16-unique.md"}
    # both -pend- files dropped: first as pending-extract, second as pending-extract too
    assert drop_reasons == ["pending-extract dump", "pending-extract dump"]


def test_classify_claude_memory_drops_bloated_keeps_topic(home: Path) -> None:
    keep, drop = classify.classify_claude_memory(home)
    assert {Path(i.path).name for i in keep} == {"feedback_topic.md"}
    assert len(drop) == 1
    assert "bloated index" in drop[0].reason


def test_audit_report_contains_gate_and_rollback(home: Path) -> None:
    report = audit_report.render(home, sample_cap=100)
    assert "Phase 1 audit report (Gate A)" in report
    assert "Reversibility plan" in report
    assert "Gate A" in report
    # every destructive action is named with a rollback column
    assert "hard-retire" in report
    assert "rollback test .26" in report
