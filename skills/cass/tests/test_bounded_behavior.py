from __future__ import annotations

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


SKILL = Path(__file__).resolve().parents[1]
MULTI = SKILL / "scripts" / "multi_machine_search.sh"
QUICK = SKILL / "scripts" / "quick_analysis.sh"
RECOVER = SKILL / "scripts" / "recover.sh"
MINER = SKILL / "scripts" / "prompt_miner.py"


class CassBoundedBehaviorTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve()
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.calls = self.root / "calls"

    def tearDown(self) -> None:
        self.temp.cleanup()

    def executable(self, name: str, content: str) -> None:
        path = self.bin / name
        path.write_text(content, encoding="utf-8")
        path.chmod(0o755)

    def environment(self) -> dict[str, str]:
        env = os.environ.copy()
        env["PATH"] = os.pathsep.join([str(self.bin), "/opt/homebrew/bin", "/usr/bin", "/bin"])
        env["CALLS"] = str(self.calls)
        env["TMPDIR"] = str(self.root)
        return env

    def install_fake_commands(self) -> None:
        self.executable(
            "timeout",
            """#!/bin/sh
while [ "${1#--}" != "$1" ]; do shift; done
shift
printf 'timeout\\n' >> "$CALLS"
exec "$@"
""",
        )
        self.executable(
            "cass",
            """#!/bin/sh
printf 'cass:%s\\n' "$1" >> "$CALLS"
case "$1" in
  status) printf '%s\\n' '{"database":{"exists":true,"conversations":1,"messages":2},"index":{"fresh":true,"documents":2},"recommended_action":"none"}' ;;
  search) printf '%s\\n' '{"total_matches":3,"hits":[{"source_path":"/secret/machine/session.jsonl","line_number":1,"score":0.9,"title":"hit"}],"aggregations":{"agent":{"buckets":[{"key":"codex","count":1}]},"date":{"buckets":[{"key":"2026-01-01","count":1}]}}}' ;;
  index) printf '%s\\n' '{"indexed":1}' ;;
  doctor) printf '%s\\n' '{"healthy":true,"checks":[]}' ;;
  diag) printf '%s\\n' '{"database":{},"index":{},"paths":{"secret":"/private/value"}}' ;;
esac
""",
        )
        self.executable(
            "ssh",
            """#!/bin/sh
printf 'ssh\\n' >> "$CALLS"
cat >/dev/null
printf '%s\\n' '{"hits":[{"source_path":"/remote/private/session.jsonl","line_number":2,"score":0.8,"title":"remote"}]}'
""",
        )

    def test_fanout_authorized_host_succeeds_without_raw_paths(self) -> None:
        self.install_fake_commands()
        completed = subprocess.run(
            ["bash", str(MULTI), "needle", "css"],
            env=self.environment(),
            capture_output=True,
            text=True,
        )
        self.assertEqual(completed.returncode, 0, completed.stderr)
        result = json.loads(completed.stdout)
        self.assertEqual({row["source"] for row in result}, {"session.jsonl"})
        self.assertNotIn("/secret/", completed.stdout)
        self.assertNotIn("/remote/", completed.stdout)
        self.assertIn("ssh", self.calls.read_text(encoding="utf-8"))

    def test_fanout_behavior_matrix_rejects_unapproved_and_path_like_hosts(self) -> None:
        self.install_fake_commands()
        for host in ("css-prod", "../../escape", "css/other"):
            with self.subTest(host=host):
                self.calls.unlink(missing_ok=True)
                completed = subprocess.run(
                    ["bash", str(MULTI), "needle", host],
                    env=self.environment(),
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(completed.returncode, 2)
                self.assertNotIn("ssh", self.calls.read_text(encoding="utf-8") if self.calls.exists() else "")
        hosts = self.root / "hosts"
        hosts.write_text("new-host\n", encoding="utf-8")
        link = self.root / "hosts-link"
        link.symlink_to(hosts)
        completed = subprocess.run(
            ["bash", str(MULTI), "--hosts-file", str(link), "needle", "new-host"],
            env=self.environment(), capture_output=True, text=True,
        )
        self.assertEqual(completed.returncode, 2)
        approved = subprocess.run(
            ["bash", str(MULTI), "--hosts-file", str(hosts), "needle", "new-host"],
            env=self.environment(), capture_output=True, text=True,
        )
        self.assertEqual(approved.returncode, 0, approved.stderr)

    def test_quick_analysis_fails_closed_without_timeout_and_bounds_every_call(self) -> None:
        minimal = self.root / "minimal"
        minimal.mkdir()
        for tool in ("dirname",):
            target = shutil.which(tool)
            assert target
            (minimal / tool).symlink_to(target)
        no_timeout_env = os.environ.copy()
        no_timeout_env["PATH"] = str(minimal)
        failed = subprocess.run(
            ["/bin/bash", str(QUICK), str(self.root)],
            env=no_timeout_env, capture_output=True, text=True,
        )
        self.assertEqual(failed.returncode, 2)
        self.assertIn("refusing an uncapped", failed.stderr)

        self.install_fake_commands()
        completed = subprocess.run(
            ["bash", str(QUICK), str(self.root)],
            env=self.environment(), capture_output=True, text=True,
        )
        self.assertEqual(completed.returncode, 0, completed.stderr)
        calls = self.calls.read_text(encoding="utf-8").splitlines()
        self.assertEqual(calls.count("timeout"), len([line for line in calls if line.startswith("cass:")]))
        self.assertNotIn(str(self.root), completed.stdout)
        invalid = self.environment()
        invalid["CASS_INDEX_TIMEOUT"] = "0"
        rejected = subprocess.run(
            ["bash", str(QUICK), str(self.root)], env=invalid, capture_output=True, text=True,
        )
        self.assertEqual(rejected.returncode, 2)

    def test_recovery_has_finite_caps_and_retains_no_raw_logs(self) -> None:
        self.install_fake_commands()
        completed = subprocess.run(
            ["bash", str(RECOVER)], env=self.environment(), capture_output=True, text=True,
        )
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertIn("READY", completed.stderr)
        self.assertFalse(list(self.root.glob("cass-refresh*")))
        self.assertFalse(list(self.root.glob("cass-rebuild*")))
        bad_env = self.environment()
        bad_env["CASS_REBUILD_TIMEOUT"] = "999999"
        rejected = subprocess.run(
            ["bash", str(RECOVER)], env=bad_env, capture_output=True, text=True,
        )
        self.assertEqual(rejected.returncode, 2)

    def test_prompt_miner_redacts_and_confines_explicit_sources(self) -> None:
        sessions = self.root / "sessions"
        sessions.mkdir()
        record = {
            "type": "user",
            "timestamp": "2026-01-01T00:00:00Z",
            "message": {"content": "deploy /private/work/app api_key=super-secret"},
        }
        (sessions / "one.jsonl").write_text(
            json.dumps(record) + "\n" + json.dumps(record) + "\n", encoding="utf-8"
        )
        command = [
            sys.executable, str(MINER), "--input-root", str(sessions),
            "--max-files", "2", "--max-file-bytes", "10000",
            "--max-line-bytes", "5000", "--max-prompts", "10",
            "--max-excerpt-chars", "100", "--json", "one.jsonl",
        ]
        completed = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertNotIn(str(sessions), completed.stdout)
        self.assertNotIn("super-secret", completed.stdout)
        self.assertNotIn("/private/work/app", completed.stdout)
        result = json.loads(completed.stdout)
        self.assertEqual(result["repeated_prompts"][0]["count"], 2)
        outside = self.root / "outside.jsonl"
        outside.write_text(json.dumps(record) + "\n", encoding="utf-8")
        (sessions / "link.jsonl").symlink_to(outside)
        rejected = subprocess.run(command[:-1] + ["link.jsonl"], capture_output=True, text=True)
        self.assertEqual(rejected.returncode, 2)
        self.assertEqual(rejected.stdout, "")


if __name__ == "__main__":
    unittest.main()
