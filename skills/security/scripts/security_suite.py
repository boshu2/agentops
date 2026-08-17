#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shlex
import shutil
import signal
import subprocess
import sys
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any


DEFAULT_PATH = "/usr/bin:/bin:/usr/sbin:/sbin"
MAX_COMMAND_OUTPUT_BYTES = 1024 * 1024
MAX_BINARY_BYTES = 256 * 1024 * 1024
MAX_TIMEOUT_SECONDS = 120
MAX_DYNAMIC_PIDS = 256
MAX_DYNAMIC_ENDPOINTS = 256
MAX_DYNAMIC_SAMPLES = 512
SECRET_PATTERNS = (
    (re.compile(r"(?i)\b(api[_-]?key|token|password|passwd|secret|authorization)\s*[:=]\s*\S+"), r"\1=[REDACTED]"),
    (re.compile(r"(?i)\bBearer\s+\S+"), "Bearer [REDACTED]"),
    (re.compile(r"-----BEGIN [^-\n]+-----[\s\S]*?-----END [^-\n]+-----"), "[REDACTED-PEM]"),
    (re.compile(r"(?<![\w@])(?:/[^\s/:]+){2,}"), "[REDACTED-PATH]"),
    (re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b"), "[REDACTED-EMAIL]"),
    (re.compile(r"\b(?:[A-Fa-f0-9]{32,}|[A-Za-z0-9+/]{40,}={0,2})\b"), "[REDACTED-TOKEN]"),
)


@dataclass
class CmdResult:
    returncode: int
    stdout: str
    stderr: str


def _now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def _ensure_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)


def _confined_path(root_raw: str, relative_raw: str, *, must_exist: bool = False) -> Path:
    root_arg = Path(root_raw)
    if not root_arg.is_absolute() or root_arg.is_symlink():
        raise ValueError("authorization root must be an absolute non-symlink directory")
    root = root_arg.resolve(strict=True)
    relative = Path(relative_raw)
    if relative.is_absolute() or ".." in relative.parts or relative.as_posix() != relative_raw:
        raise ValueError("authorized path must be canonical and root-relative")
    candidate = root / relative
    current = candidate
    while current != root:
        if current.is_symlink():
            raise ValueError("authorized path must not contain symlinks")
        current = current.parent
    if must_exist:
        resolved = candidate.resolve(strict=True)
        resolved.relative_to(root)
        return resolved
    ancestor = candidate
    while not ancestor.exists() and ancestor != root:
        ancestor = ancestor.parent
    ancestor.resolve(strict=True).relative_to(root)
    return candidate


def _write_json(path: Path, data: dict[str, Any]) -> None:
    _ensure_dir(path.parent)
    path.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def _write_text(path: Path, text: str) -> None:
    _ensure_dir(path.parent)
    path.write_text(text, encoding="utf-8")


def _truncate(text: str, limit: int = 20000) -> str:
    if len(text) <= limit:
        return text
    return text[:limit] + f"\n... [truncated {len(text) - limit} bytes]"


def _redact(text: str) -> str:
    result = text
    for pattern, replacement in SECRET_PATTERNS:
        result = pattern.sub(replacement, result)
    return result


def _kill_group(process: subprocess.Popen[bytes]) -> None:
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass


def _run(cmd: list[str], *, timeout: int = 10, cwd: Path | None = None, env: dict[str, str] | None = None) -> CmdResult:
    if not 0 < timeout <= MAX_TIMEOUT_SECONDS:
        raise ValueError("command timeout is outside the supported finite range")
    try:
        process = subprocess.Popen(
            cmd,
            cwd=str(cwd) if cwd else None,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=True,
        )
    except FileNotFoundError:
        return CmdResult(127, "", "[missing tool]")
    assert process.stdout and process.stderr
    stdout = bytearray(); stderr = bytearray(); total = 0
    lock = threading.Lock(); overflow = threading.Event()

    def drain(stream: Any, target: bytearray) -> None:
        nonlocal total
        while chunk := stream.read(8192):
            with lock:
                total += len(chunk)
                if total > MAX_COMMAND_OUTPUT_BYTES:
                    overflow.set()
                if len(target) <= MAX_COMMAND_OUTPUT_BYTES:
                    target.extend(chunk[: max(0, MAX_COMMAND_OUTPUT_BYTES + 1 - len(target))])

    threads = [
        threading.Thread(target=drain, args=(process.stdout, stdout), daemon=True),
        threading.Thread(target=drain, args=(process.stderr, stderr), daemon=True),
    ]
    for thread in threads: thread.start()
    deadline = time.monotonic() + timeout; timed_out = False
    while process.poll() is None:
        if overflow.is_set(): break
        if time.monotonic() >= deadline:
            timed_out = True; break
        time.sleep(0.005)
    if process.poll() is None or overflow.is_set(): _kill_group(process)
    return_code = process.wait(timeout=2)
    _kill_group(process)
    for thread in threads: thread.join(timeout=1)
    process.stdout.close(); process.stderr.close()
    out_text = _redact(bytes(stdout[:MAX_COMMAND_OUTPUT_BYTES]).decode("utf-8", "replace"))
    err_text = _redact(bytes(stderr[:MAX_COMMAND_OUTPUT_BYTES]).decode("utf-8", "replace"))
    if timed_out:
        return CmdResult(124, out_text, err_text + "\n[timeout]")
    if overflow.is_set():
        return CmdResult(125, "", "[output bound exceeded]")
    return CmdResult(return_code, out_text, err_text)


def _sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def _count_zip_signatures(path: Path) -> int:
    sig = b"PK\x03\x04"
    count = 0
    with path.open("rb") as f:
        while True:
            block = f.read(4 * 1024 * 1024)
            if not block:
                break
            count += block.count(sig)
    return count


def _extract_strings(binary: Path, *, timeout: int = 90) -> tuple[list[str], str]:
    if not shutil_which("strings"):
        return [], ""
    r = _run(["strings", "-a", str(binary)], timeout=timeout)
    if r.returncode != 0:
        return [], ""
    lines = r.stdout.splitlines()
    return lines, r.stdout


def shutil_which(cmd: str) -> str | None:
    # Resolve against PATH directly. The previous implementation shelled out to
    # `bash -lc`, which sources the user's login profile inside a security tool —
    # arbitrary profile code on every lookup. shutil.which has no such surface.
    return shutil.which(cmd)


def _detect_runtimes(strings_blob: str, linked_blob: str, file_blob: str) -> list[str]:
    text = "\n".join([strings_blob, linked_blob, file_blob])
    runtimes: list[str] = []

    def hit(pattern: str) -> bool:
        return re.search(pattern, text, re.IGNORECASE) is not None

    if hit(r"runtime\.morestack|go\.buildid|\bgo1\.\d+|golang\.org/|\bGOROOT\b"):
        runtimes.append("Go")
    if hit(r"libpython|python\d+\.\d+|Py_Initialize|Python\.framework"):
        runtimes.append("Python")
    if hit(r"rustc/\d+\.\d+\.\d+|core::panicking|alloc::|std::panicking|cargo:"):
        runtimes.append("Rust")
    if hit(r"NODE_MODULE_VERSION|libnode|node:internal|npm_"):
        runtimes.append("Node.js")
    if hit(r"java/lang/|JNI_OnLoad|ClassNotFoundException|kotlin/"):
        runtimes.append("JVM")
    if hit(r"CoreCLR|clrjit|mscoree|System\.Collections|Microsoft\.NET"):
        runtimes.append(".NET")
    if hit(r"GLIBCXX_|CXXABI_|libstdc\+\+|libc\+\+|__cxa_throw"):
        runtimes.append("C/C++")

    return sorted(set(runtimes))


def _collect_static(binary: Path, out_dir: Path) -> dict[str, Any]:
    static_dir = out_dir / "static"
    _ensure_dir(static_dir)

    file_info = _run(["file", str(binary)], timeout=10).stdout.strip() if shutil_which("file") else ""

    linked = ""
    if shutil_which("otool"):
        linked = _run(["otool", "-L", str(binary)], timeout=10).stdout
    elif shutil_which("ldd"):
        linked = _run(["ldd", str(binary)], timeout=10).stdout

    strings_all, strings_blob = _extract_strings(binary)
    strings_lines = strings_all[:5000]

    ai_terms = ["mcp", "modelcontextprotocol", "openai", "anthropic", "claude", "system prompt", "tool call"]
    ai_hits: list[str] = []
    for ln in strings_lines:
        low = ln.lower()
        if any(t in low for t in ai_terms):
            ai_hits.append(hashlib.sha256(ln.encode("utf-8", errors="replace")).hexdigest())
        if len(ai_hits) >= 300:
            break

    runtimes = _detect_runtimes(strings_blob, linked, file_info)

    data = {
        "schema_version": 1,
        "generated_at": _now_iso(),
        "binary": binary.name,
        "size_bytes": binary.stat().st_size,
        "sha256": _sha256_file(binary),
        "file_info": file_info,
        "linked_libraries": [ln for ln in linked.splitlines() if ln.strip()],
        "runtime_guess": runtimes if runtimes else ["unknown"],
        "zip_local_header_count": _count_zip_signatures(binary),
        "ai_related_string_hit_sha256": ai_hits,
        "strings_sample_count": len(strings_lines),
        "strings_total_count": len(strings_all),
    }

    _write_json(static_dir / "static-analysis.json", data)

    md = [
        "# Static Analysis",
        "",
        f"- Generated: {data['generated_at']}",
        f"- Binary: `{binary.name}`",
        f"- SHA256: `{data['sha256']}`",
        f"- Size: `{data['size_bytes']}` bytes",
        f"- Runtime guess: `{', '.join(data['runtime_guess'])}`",
        f"- Embedded ZIP local headers: `{data['zip_local_header_count']}`",
        "",
        "## file(1)",
        "",
        "```",
        file_info or "(unavailable)",
        "```",
        "",
        "## Linked Libraries",
        "",
        "```",
        linked.strip() or "(none detected)",
        "```",
        "",
        "## AI-Related String Hit Hashes (sample)",
        "",
    ]
    if ai_hits:
        md.extend([f"- `{h}`" for h in ai_hits[:50]])
    else:
        md.append("- _None detected in sampled strings._")

    _write_text(static_dir / "static-analysis.md", "\n".join(md).rstrip() + "\n")
    return data


def _snapshot_tree(root: Path) -> dict[str, dict[str, int]]:
    out: dict[str, dict[str, int]] = {}
    if not root.exists():
        return out
    for index, p in enumerate(sorted(root.rglob("*"))):
        if index >= 10000:
            raise ValueError("observation workspace exceeds 10000 entries")
        if p.is_symlink() or not p.is_file():
            continue
        # Filenames are controlled by the analyzed program and can themselves
        # contain credentials. Retain only stable identities, never raw names.
        rel = hashlib.sha256(p.relative_to(root).as_posix().encode("utf-8", "replace")).hexdigest()
        st = p.stat()
        out[rel] = {"size": int(st.st_size), "mtime_ns": int(st.st_mtime_ns)}
    return out


def _diff_snapshots(before: dict[str, dict[str, int]], after: dict[str, dict[str, int]]) -> dict[str, list[str]]:
    b = set(before.keys())
    a = set(after.keys())
    created = sorted(a - b)
    removed = sorted(b - a)
    modified = sorted(k for k in (a & b) if before[k] != after[k])
    return {"created": created, "modified": modified, "removed": removed}


def _collect_process_table() -> dict[int, dict[str, Any]]:
    if not shutil_which("ps"):
        return {}
    r = _run(["ps", "-axo", "pid=,ppid=,command="], timeout=5)
    table: dict[int, dict[str, Any]] = {}
    for ln in r.stdout.splitlines():
        m = re.match(r"\s*(\d+)\s+(\d+)\s+(.*)$", ln)
        if not m:
            continue
        pid = int(m.group(1))
        ppid = int(m.group(2))
        cmd = m.group(3).strip()
        table[pid] = {"ppid": ppid, "command": cmd}
    return table


def _descendants(root_pid: int, table: dict[int, dict[str, Any]]) -> set[int]:
    out: set[int] = {root_pid}
    changed = True
    while changed:
        changed = False
        for pid, meta in table.items():
            if pid in out:
                continue
            if int(meta.get("ppid", -1)) in out:
                out.add(pid)
                changed = True
    return out


def _collect_network_endpoints(pids: set[int]) -> list[str]:
    if not pids or not shutil_which("lsof"):
        return []
    eps: set[str] = set()
    selected = sorted(pids)[:MAX_DYNAMIC_PIDS]
    # `-a` is essential: without it lsof ORs -i and -p and accidentally
    # inventories every network connection on the host.
    r = _run(["lsof", "-nP", "-a", "-i", "-p", ",".join(map(str, selected))], timeout=1)
    if r.returncode != 0:
        return []
    for ln in r.stdout.splitlines():
        if "->" in ln or "TCP" in ln or "UDP" in ln:
            eps.add(re.sub(r"\s+", " ", ln.strip()))
            if len(eps) >= MAX_DYNAMIC_ENDPOINTS:
                break
    return sorted(eps)


def _sandbox_command(binary: Path, run_args: list[str], workspace: Path) -> tuple[list[str], str]:
    """Return a real OS containment command or fail closed before execution."""
    sandbox_exec = shutil.which("sandbox-exec")
    if sandbox_exec:
        def quoted(path: Path | str) -> str:
            return json.dumps(str(path))

        # Unknown native programs need system- and runtime-specific read paths
        # that cannot be enumerated portably on macOS. Reads are therefore
        # allowed, while all writes and network operations remain deny-default.
        # Child output and created filenames are retained only as hashes below,
        # so readable secrets cannot be copied into evidence artifacts.
        profile_lines = [
            "(version 1)",
            "(deny default)",
            "(allow process*)",
            "(allow file-read*)",
        ]
        profile_lines.extend(
            [
                f"(allow file-write* (subpath {quoted(workspace)}))",
                '(allow file-write-data (literal "/dev/null"))',
                "(allow sysctl-read)",
                "(allow mach-lookup)",
            ]
        )
        profile = "\n".join(profile_lines)
        return [sandbox_exec, "-p", profile, str(binary), *run_args], "macos-sandbox-exec"
    raise RuntimeError("no supported OS containment backend; dynamic execution refused")


def _collect_dynamic(binary: Path, out_dir: Path, run_args: list[str], timeout_s: int) -> dict[str, Any]:
    if not 1 <= timeout_s <= MAX_TIMEOUT_SECONDS:
        raise ValueError("dynamic timeout must be in [1,120]")
    dynamic_dir = out_dir / "dynamic"
    workspace = dynamic_dir / "observation-workspace"
    home = workspace / "home"
    work = workspace / "work"
    tmp = workspace / "tmp"
    for directory in [dynamic_dir, workspace, home, work, tmp]:
        _ensure_dir(directory)
    argv, backend = _sandbox_command(binary, run_args, workspace)
    before_home = _snapshot_tree(home)
    before_work = _snapshot_tree(work)
    env = {"PATH": DEFAULT_PATH, "HOME": str(home), "TMPDIR": str(tmp), "LANG": "C"}

    started = time.monotonic()
    timed_out = False
    overflow = threading.Event()
    proc = subprocess.Popen(
        argv,
        cwd=str(work),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        start_new_session=True,
    )
    assert proc.stdout and proc.stderr
    stdout = bytearray()
    stderr = bytearray()
    output_total = 0
    output_lock = threading.Lock()

    def drain(stream: Any, target: bytearray) -> None:
        nonlocal output_total
        while chunk := stream.read(8192):
            with output_lock:
                output_total += len(chunk)
                if output_total > MAX_COMMAND_OUTPUT_BYTES:
                    overflow.set()
                if len(target) <= MAX_COMMAND_OUTPUT_BYTES:
                    target.extend(chunk[: max(0, MAX_COMMAND_OUTPUT_BYTES + 1 - len(target))])

    readers = [
        threading.Thread(target=drain, args=(proc.stdout, stdout), daemon=True),
        threading.Thread(target=drain, args=(proc.stderr, stderr), daemon=True),
    ]
    for reader in readers:
        reader.start()

    seen_cmd_hashes: set[str] = set()
    seen_pids: set[int] = set()
    seen_ep_hashes: set[str] = set()
    sample_count = 0
    try:
        while proc.poll() is None:
            if overflow.is_set() or time.monotonic() - started >= timeout_s:
                timed_out = time.monotonic() - started >= timeout_s
                _kill_group(proc)
                break
            if sample_count < MAX_DYNAMIC_SAMPLES:
                table = _collect_process_table()
                pids = _descendants(proc.pid, table) if proc.pid in table else {proc.pid}
                bounded_pids = set(sorted(pids)[:MAX_DYNAMIC_PIDS])
                seen_pids.update(bounded_pids)
                for pid in bounded_pids:
                    command = str(table.get(pid, {}).get("command", ""))
                    if command and len(seen_cmd_hashes) < MAX_DYNAMIC_PIDS:
                        seen_cmd_hashes.add(hashlib.sha256(command.encode()).hexdigest())
                if len(seen_ep_hashes) < MAX_DYNAMIC_ENDPOINTS:
                    for endpoint in _collect_network_endpoints(bounded_pids):
                        seen_ep_hashes.add(hashlib.sha256(endpoint.encode()).hexdigest())
                        if len(seen_ep_hashes) >= MAX_DYNAMIC_ENDPOINTS:
                            break
                sample_count += 1
            time.sleep(0.05)
    finally:
        _kill_group(proc)
        for pid in sorted(seen_pids - {proc.pid}):
            try:
                os.kill(pid, signal.SIGKILL)
            except (ProcessLookupError, PermissionError):
                pass

    try:
        rc = proc.wait(timeout=2)
    except subprocess.TimeoutExpired:
        _kill_group(proc)
        rc = proc.wait(timeout=2)
    for reader in readers:
        reader.join(timeout=1)
    proc.stdout.close()
    proc.stderr.close()

    stdout_bytes = bytes(stdout[:MAX_COMMAND_OUTPUT_BYTES])
    stderr_bytes = bytes(stderr[:MAX_COMMAND_OUTPUT_BYTES])
    after_home = _snapshot_tree(home)
    after_work = _snapshot_tree(work)
    data = {
        "schema_version": 2,
        "generated_at": _now_iso(),
        "binary": binary.name,
        "argument_count": len(run_args),
        "argv_sha256": hashlib.sha256(json.dumps([binary.name, *run_args]).encode()).hexdigest(),
        "timeout_seconds": timeout_s,
        "duration_ms": int((time.monotonic() - started) * 1000),
        "exit_code": -9 if timed_out else rc,
        "timed_out": timed_out,
        "output_exceeded": overflow.is_set(),
        "stdout_bytes_observed": len(stdout_bytes),
        "stderr_bytes_observed": len(stderr_bytes),
        "stdout_sha256": hashlib.sha256(stdout_bytes).hexdigest(),
        "stderr_sha256": hashlib.sha256(stderr_bytes).hexdigest(),
        "containment": {
            "backend": backend,
            "network": "deny-default",
            "writes": "observation-workspace-only",
            "reads": "allowed; output retained as hashes only",
        },
        "process_command_sha256": sorted(seen_cmd_hashes),
        "pids_observed_count": len(seen_pids),
        "network_endpoint_sha256": sorted(seen_ep_hashes),
        "file_changes": {
            "home": _diff_snapshots(before_home, after_home),
            "work": _diff_snapshots(before_work, after_work),
        },
    }
    _write_json(dynamic_dir / "dynamic-analysis.json", data)
    files_created = len(data["file_changes"]["home"]["created"]) + len(data["file_changes"]["work"]["created"])
    md = [
        "# Dynamic Analysis",
        "",
        f"- Generated: {data['generated_at']}",
        f"- Containment backend: `{backend}`",
        f"- Exit code: `{data['exit_code']}`",
        f"- Timed out: `{data['timed_out']}`",
        f"- Output exceeded bound: `{data['output_exceeded']}`",
        f"- Files created in observation workspace: `{files_created}`",
        f"- Process command hashes observed: `{len(seen_cmd_hashes)}`",
        f"- Network endpoint hashes observed: `{len(seen_ep_hashes)}`",
        "",
    ]
    _write_text(dynamic_dir / "dynamic-analysis.md", "\n".join(md))
    return data


def _normalize_cmd_token(token: str) -> str | None:
    token = token.strip().strip("`\"'")
    token = token.strip("[]<>(){}")
    if not token:
        return None
    if token.startswith("-"):
        return None
    if token.lower() in {"help", "commands", "command", "flags", "options", "usage"}:
        return None
    if not re.match(r"^[a-zA-Z0-9][a-zA-Z0-9._:-]*$", token):
        return None
    return token


def _parse_subcommands(help_text: str) -> list[str]:
    lines = help_text.splitlines()
    out: list[str] = []
    in_commands = False
    for ln in lines:
        if re.match(r"^\s*(Available\s+Commands|Commands|Subcommands)\s*:", ln, flags=re.IGNORECASE):
            in_commands = True
            continue
        if not in_commands:
            continue
        if not ln.strip():
            in_commands = False
            continue
        if re.match(r"^\s*(Flags|Global Flags|Options|Arguments|Examples|Environment|Usage|USAGE)\s*:", ln):
            in_commands = False
            continue
        tok = ln.strip().split()[0] if ln.strip().split() else ""
        norm = _normalize_cmd_token(tok)
        if norm:
            out.append(norm)
    seen: set[str] = set()
    dedup: list[str] = []
    for c in out:
        if c not in seen:
            dedup.append(c)
            seen.add(c)
    return dedup


def _probe_help(binary: Path, path: tuple[str, ...], timeout_s: int) -> tuple[bool, str, str]:
    probes: list[tuple[str, list[str]]] = []
    if path:
        p = list(path)
        probes = [
            ("--help", p + ["--help"]),
            ("-h", p + ["-h"]),
            ("help-prefix", ["help", *p]),
            ("help-suffix", [*p, "help"]),
        ]
    else:
        probes = [
            ("--help", ["--help"]),
            ("-h", ["-h"]),
            ("help", ["help"]),
        ]

    for pname, args in probes:
        r = _run([str(binary), *args], timeout=timeout_s)
        txt = (r.stdout or "") + ("\n" + r.stderr if r.stderr else "")
        if re.search(r"Usage|USAGE|Commands|Subcommands|Flags|Options|help", txt):
            return True, pname, txt
    return False, "", ""


def _capture_command_surface(binary: Path, max_depth: int, per_cmd_timeout: int, total_timeout: int) -> dict[str, Any]:
    started = time.time()
    queue: list[tuple[str, ...]] = [tuple()]
    visited: set[tuple[str, ...]] = set()

    commands: set[str] = set()
    sections: list[dict[str, Any]] = []
    probes: set[str] = set()

    while queue:
        if len(visited) >= 256:
            break
        if time.time() - started > total_timeout:
            break
        path = queue.pop(0)
        if path in visited:
            continue
        visited.add(path)

        ok, probe, output = _probe_help(binary, path, timeout_s=per_cmd_timeout)
        if not ok:
            continue

        probes.add(probe)
        sections.append({"path": " ".join(path), "probe": probe, "line_count": len(output.splitlines())})

        if path:
            commands.add(" ".join(path))

        if len(path) >= max_depth:
            continue

        for sub in _parse_subcommands(output):
            child = (*path, sub)
            if child not in visited:
                queue.append(child)

    command_list = sorted(commands)
    top_level = sorted({c.split()[0] for c in command_list if c})

    return {
        "command_paths": command_list,
        "top_level_commands": top_level,
        "help_sections": sections,
        "probe_kinds": sorted(probes),
        "max_depth": max((len(c.split()) for c in command_list), default=0),
        "timed_out": bool(queue),
    }


def _collect_contract(binary: Path, out_dir: Path, *, max_depth: int, per_cmd_timeout: int, total_timeout: int) -> dict[str, Any]:
    contract_dir = out_dir / "contract"
    _ensure_dir(contract_dir)

    surface = _capture_command_surface(binary, max_depth=max_depth, per_cmd_timeout=per_cmd_timeout, total_timeout=total_timeout)

    static_json = out_dir / "static" / "static-analysis.json"
    dynamic_json = out_dir / "dynamic" / "dynamic-analysis.json"

    static_data: dict[str, Any] = json.loads(static_json.read_text(encoding="utf-8")) if static_json.exists() else {}
    dynamic_data: dict[str, Any] = json.loads(dynamic_json.read_text(encoding="utf-8")) if dynamic_json.exists() else {}

    contract = {
        "schema_version": 1,
        "generated_at": _now_iso(),
        "binary": str(binary),
        "binary_sha256": static_data.get("sha256"),
        "runtime_guess": static_data.get("runtime_guess", ["unknown"]),
        "command_paths": surface["command_paths"],
        "top_level_commands": surface["top_level_commands"],
        "max_depth": surface["max_depth"],
        "help_probe_kinds": surface["probe_kinds"],
        "help_section_count": len(surface["help_sections"]),
        "dynamic_summary": {
            "exit_code": dynamic_data.get("exit_code"),
            "timed_out": dynamic_data.get("timed_out"),
            "network_endpoint_count": len(dynamic_data.get("network_endpoint_sha256", [])),
            "sandbox_file_creates": len(dynamic_data.get("file_changes", {}).get("home", {}).get("created", []))
            + len(dynamic_data.get("file_changes", {}).get("work", {}).get("created", [])),
        },
    }

    _write_json(contract_dir / "contract.json", contract)

    md = [
        "# Behavior Contract",
        "",
        f"- Generated: {contract['generated_at']}",
        f"- Binary: `{binary}`",
        f"- SHA256: `{contract.get('binary_sha256', 'unknown')}`",
        f"- Runtime guess: `{', '.join(contract.get('runtime_guess', ['unknown']))}`",
        f"- Command paths: `{len(contract['command_paths'])}`",
        f"- Top-level commands: `{len(contract['top_level_commands'])}`",
        f"- Max depth: `{contract['max_depth']}`",
        f"- Help probes: `{', '.join(contract['help_probe_kinds']) if contract['help_probe_kinds'] else 'none'}`",
        "",
        "## Top-Level Commands",
        "",
    ]
    if contract["top_level_commands"]:
        md.extend([f"- `{c}`" for c in contract["top_level_commands"][:200]])
    else:
        md.append("- _No commands discovered._")

    _write_text(contract_dir / "contract.md", "\n".join(md).rstrip() + "\n")
    _write_json(contract_dir / "help-sections.json", {"sections": surface["help_sections"]})
    return contract


def _load_contract(path: Path) -> dict[str, Any]:
    c1 = path / "contract" / "contract.json"
    c2 = path / "contract.json"
    target = c1 if c1.exists() else c2
    if not target.exists():
        raise FileNotFoundError(f"contract not found under {path}")
    return json.loads(target.read_text(encoding="utf-8"))


def _compare_baseline(current_dir: Path, baseline_dir: Path, out_dir: Path) -> dict[str, Any]:
    compare_dir = out_dir / "compare"
    _ensure_dir(compare_dir)

    cur = _load_contract(current_dir)
    base = _load_contract(baseline_dir)

    cur_cmds = set(cur.get("command_paths", []))
    base_cmds = set(base.get("command_paths", []))

    added = sorted(cur_cmds - base_cmds)
    removed = sorted(base_cmds - cur_cmds)
    overlap = sorted(cur_cmds & base_cmds)

    status = "pass" if not removed else "fail"

    data = {
        "schema_version": 1,
        "generated_at": _now_iso(),
        "status": status,
        "current_count": len(cur_cmds),
        "baseline_count": len(base_cmds),
        "overlap_count": len(overlap),
        "added": added,
        "removed": removed,
        "runtime_changed": cur.get("runtime_guess") != base.get("runtime_guess"),
        "current_runtime": cur.get("runtime_guess"),
        "baseline_runtime": base.get("runtime_guess"),
        "current_sha256": cur.get("binary_sha256"),
        "baseline_sha256": base.get("binary_sha256"),
    }

    _write_json(compare_dir / "baseline-diff.json", data)

    md = [
        "# Baseline Diff",
        "",
        f"- Generated: {data['generated_at']}",
        f"- Status: **{data['status'].upper()}**",
        f"- Current commands: `{data['current_count']}`",
        f"- Baseline commands: `{data['baseline_count']}`",
        f"- Overlap: `{data['overlap_count']}`",
        "",
        "## Added Commands",
        "",
    ]
    md.extend([f"- `{c}`" for c in added[:200]] if added else ["_None._"])
    md.extend(["", "## Removed Commands", ""])
    md.extend([f"- `{c}`" for c in removed[:200]] if removed else ["_None._"])
    if len(added) > 200:
        md.append(f"- ... ({len(added) - 200} more)")
    if len(removed) > 200:
        md.append(f"- ... ({len(removed) - 200} more)")

    _write_text(compare_dir / "baseline-diff.md", "\n".join(md).rstrip() + "\n")
    return data


def _match_any(patterns: list[str], value: str) -> bool:
    for p in patterns:
        if re.search(p, value):
            return True
    return False


def _enforce_policy(run_dir: Path, policy_file: Path, out_dir: Path) -> tuple[str, list[dict[str, Any]]]:
    policy_dir = out_dir / "policy"
    _ensure_dir(policy_dir)

    policy = json.loads(policy_file.read_text(encoding="utf-8"))
    contract = _load_contract(run_dir)

    dynamic_path = run_dir / "dynamic" / "dynamic-analysis.json"
    dynamic = json.loads(dynamic_path.read_text(encoding="utf-8")) if dynamic_path.exists() else {}

    compare_path = run_dir / "compare" / "baseline-diff.json"
    compare = json.loads(compare_path.read_text(encoding="utf-8")) if compare_path.exists() else {}

    findings: list[dict[str, Any]] = []

    req_top = policy.get("required_top_level_commands", [])
    top = set(contract.get("top_level_commands", []))
    missing = sorted([c for c in req_top if c not in top])
    if missing:
        findings.append({"severity": "fail", "code": "missing_required_commands", "message": f"missing required top-level commands: {', '.join(missing)}"})

    deny_cmd_patterns = policy.get("deny_command_patterns", [])
    for cmd in contract.get("command_paths", []):
        if _match_any(deny_cmd_patterns, cmd):
            findings.append({"severity": "fail", "code": "denied_command_pattern", "message": f"denied command pattern matched: {cmd}"})

    max_created = int(policy.get("max_created_files", 999999))
    created_files = dynamic.get("file_changes", {}).get("home", {}).get("created", []) + dynamic.get("file_changes", {}).get("work", {}).get("created", [])
    if len(created_files) > max_created:
        findings.append({"severity": "fail", "code": "too_many_created_files", "message": f"created files {len(created_files)} exceeds max {max_created}"})

    forbid_path_patterns = policy.get("forbid_file_path_patterns", [])
    for p in created_files:
        if _match_any(forbid_path_patterns, p):
            findings.append({"severity": "fail", "code": "forbidden_file_path", "message": f"forbidden created path: {p}"})

    endpoints = dynamic.get("network_endpoint_sha256", [])
    allow_net = policy.get("allow_network_endpoint_patterns", [])
    deny_net = policy.get("deny_network_endpoint_patterns", [])

    if allow_net:
        for ep in endpoints:
            if not _match_any(allow_net, ep):
                findings.append({"severity": "fail", "code": "network_not_allowlisted", "message": f"network endpoint not allowlisted: {ep}"})

    for ep in endpoints:
        if _match_any(deny_net, ep):
            findings.append({"severity": "fail", "code": "network_denylisted", "message": f"denylisted network endpoint observed: {ep}"})

    if bool(policy.get("block_if_removed_commands", False)) and compare.get("removed"):
        findings.append({"severity": "fail", "code": "removed_commands", "message": f"commands removed vs baseline: {len(compare.get('removed', []))}"})

    min_cmds = int(policy.get("min_command_count", 0))
    cmd_count = len(contract.get("command_paths", []))
    if cmd_count < min_cmds:
        findings.append({"severity": "warn", "code": "low_command_count", "message": f"command count {cmd_count} below expected minimum {min_cmds}"})

    verdict = "PASS"
    if any(f["severity"] == "fail" for f in findings):
        verdict = "FAIL"
    elif findings:
        verdict = "WARN"

    data = {
        "schema_version": 1,
        "generated_at": _now_iso(),
        "verdict": verdict,
        "policy_file": str(policy_file),
        "finding_count": len(findings),
        "findings": findings,
    }
    _write_json(policy_dir / "policy-verdict.json", data)

    md = [
        "# Policy Verdict",
        "",
        f"- Generated: {data['generated_at']}",
        f"- Verdict: **{verdict}**",
        f"- Policy file: `{policy_file}`",
        f"- Findings: `{len(findings)}`",
        "",
        "## Findings",
        "",
    ]
    if findings:
        for f in findings:
            md.append(f"- **{f['severity'].upper()}** `{f['code']}`: {f['message']}")
    else:
        md.append("- _No policy findings._")

    _write_text(policy_dir / "policy-verdict.md", "\n".join(md).rstrip() + "\n")
    return verdict, findings


def _suite_summary(out_dir: Path) -> dict[str, Any]:
    static = out_dir / "static" / "static-analysis.json"
    dynamic = out_dir / "dynamic" / "dynamic-analysis.json"
    contract = out_dir / "contract" / "contract.json"
    compare = out_dir / "compare" / "baseline-diff.json"
    policy = out_dir / "policy" / "policy-verdict.json"

    data: dict[str, Any] = {
        "schema_version": 1,
        "generated_at": _now_iso(),
        "artifacts": {
            "static": str(static) if static.exists() else None,
            "dynamic": str(dynamic) if dynamic.exists() else None,
            "contract": str(contract) if contract.exists() else None,
            "compare": str(compare) if compare.exists() else None,
            "policy": str(policy) if policy.exists() else None,
        },
    }

    if contract.exists():
        c = json.loads(contract.read_text(encoding="utf-8"))
        data["command_count"] = len(c.get("command_paths", []))
        data["runtime_guess"] = c.get("runtime_guess")
    if compare.exists():
        d = json.loads(compare.read_text(encoding="utf-8"))
        data["baseline_status"] = d.get("status")
        data["removed_commands"] = len(d.get("removed", []))
    if policy.exists():
        p = json.loads(policy.read_text(encoding="utf-8"))
        data["policy_verdict"] = p.get("verdict")

    _write_json(out_dir / "suite-summary.json", data)

    md = [
        "# Security Suite Summary",
        "",
        f"- Generated: {data['generated_at']}",
        f"- Command count: `{data.get('command_count', 'n/a')}`",
        f"- Runtime guess: `{', '.join(data.get('runtime_guess', ['n/a'])) if isinstance(data.get('runtime_guess'), list) else data.get('runtime_guess', 'n/a')}`",
        f"- Baseline status: `{data.get('baseline_status', 'n/a')}`",
        f"- Policy verdict: `{data.get('policy_verdict', 'n/a')}`",
    ]
    _write_text(out_dir / "suite-summary.md", "\n".join(md).rstrip() + "\n")
    return data


def _parse_run_args(raw: str | None) -> list[str]:
    if not raw:
        return ["--help"]
    if len(raw.encode("utf-8")) > 4096:
        raise ValueError("run arguments exceed 4096 bytes")
    parsed = shlex.split(raw)
    if len(parsed) > 64 or any(len(item.encode("utf-8")) > 1024 for item in parsed):
        raise ValueError("run arguments exceed count or token bounds")
    return parsed


def main() -> int:
    ap = argparse.ArgumentParser(prog="security_suite.py")
    sub = ap.add_subparsers(dest="cmd", required=True)

    common = argparse.ArgumentParser(add_help=False)
    common.add_argument("--binary-root", required=True)
    common.add_argument("--binary", required=True)
    common.add_argument("--output-root", required=True)
    common.add_argument("--out-dir", required=True)

    _p_static = sub.add_parser("collect-static", parents=[common])
    p_dynamic = sub.add_parser("collect-dynamic", parents=[common])
    p_dynamic.add_argument("--authorized-dynamic", action="store_true")
    p_dynamic.add_argument("--run-args", default="--help", help="Arguments passed to the binary during dynamic run")
    p_dynamic.add_argument("--timeout", type=int, default=8)

    p_contract = sub.add_parser("collect-contract", parents=[common])
    p_contract.add_argument("--max-depth", type=int, default=4)
    p_contract.add_argument("--per-cmd-timeout", type=int, default=5)
    p_contract.add_argument("--total-timeout", type=int, default=120)

    p_compare = sub.add_parser("compare-baseline")
    p_compare.add_argument("--current-dir", required=True)
    p_compare.add_argument("--baseline-dir", required=True)
    p_compare.add_argument("--out-dir", required=True)
    p_compare.add_argument("--output-root", required=True)

    p_policy = sub.add_parser("enforce-policy")
    p_policy.add_argument("--run-dir", required=True)
    p_policy.add_argument("--policy-file", required=True)
    p_policy.add_argument("--out-dir", required=True)
    p_policy.add_argument("--output-root", required=True)

    p_run = sub.add_parser("run", parents=[common])
    p_run.add_argument("--authorized-dynamic", action="store_true")
    p_run.add_argument("--run-args", default="--help")
    p_run.add_argument("--timeout", type=int, default=8)
    p_run.add_argument("--max-depth", type=int, default=4)
    p_run.add_argument("--per-cmd-timeout", type=int, default=5)
    p_run.add_argument("--total-timeout", type=int, default=120)
    p_run.add_argument("--baseline-dir", default=None)
    p_run.add_argument("--policy-file", default=None)
    p_run.add_argument("--fail-on-removed", action="store_true", help="Exit non-zero if compare-baseline reports removed commands")
    p_run.add_argument("--fail-on-policy-fail", action="store_true", help="Exit non-zero if policy verdict is FAIL")

    args = ap.parse_args()

    for name in ("timeout", "per_cmd_timeout", "total_timeout"):
        if hasattr(args, name) and not 1 <= getattr(args, name) <= MAX_TIMEOUT_SECONDS:
            ap.error(f"--{name.replace('_', '-')} must be in [1,{MAX_TIMEOUT_SECONDS}]")
    if hasattr(args, "max_depth") and not 0 <= args.max_depth <= 5:
        ap.error("--max-depth must be in [0,5]")

    if args.cmd in {"collect-static", "collect-dynamic", "collect-contract", "run"}:
        try:
            binary = _confined_path(args.binary_root, args.binary, must_exist=True)
            out_dir = _confined_path(args.output_root, args.out_dir)
        except (OSError, ValueError) as exc:
            print(f"error: {exc}", file=sys.stderr)
            return 2
        if binary.is_symlink() or not binary.is_file() or binary.stat().st_size > MAX_BINARY_BYTES:
            print("error: binary must be a bounded regular non-symlink file", file=sys.stderr)
            return 2
    else:
        binary = Path("/")
        try:
            out_dir = _confined_path(args.output_root, args.out_dir)
        except (OSError, ValueError) as exc:
            print(f"error: {exc}", file=sys.stderr)
            return 2

    if args.cmd == "collect-static":
        _collect_static(binary, out_dir)
        return 0

    if args.cmd == "collect-dynamic":
        if not args.authorized_dynamic:
            print("error: --authorized-dynamic is required", file=sys.stderr)
            return 2
        try:
            run_args = _parse_run_args(args.run_args)
            _collect_dynamic(binary, out_dir, run_args, timeout_s=args.timeout)
        except (RuntimeError, ValueError) as exc:
            print(f"error: {exc}", file=sys.stderr)
            return 2
        return 0

    if args.cmd == "collect-contract":
        _collect_contract(binary, out_dir, max_depth=args.max_depth, per_cmd_timeout=args.per_cmd_timeout, total_timeout=args.total_timeout)
        return 0

    if args.cmd == "compare-baseline":
        _compare_baseline(Path(args.current_dir).resolve(), Path(args.baseline_dir).resolve(), out_dir)
        return 0

    if args.cmd == "enforce-policy":
        verdict, _ = _enforce_policy(Path(args.run_dir).resolve(), Path(args.policy_file).resolve(), out_dir)
        return 3 if verdict == "FAIL" else 0

    # run
    if not args.authorized_dynamic:
        print("error: --authorized-dynamic is required", file=sys.stderr)
        return 2
    _collect_static(binary, out_dir)
    try:
        run_args = _parse_run_args(args.run_args)
        _collect_dynamic(binary, out_dir, run_args, timeout_s=args.timeout)
    except (RuntimeError, ValueError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    _collect_contract(binary, out_dir, max_depth=args.max_depth, per_cmd_timeout=args.per_cmd_timeout, total_timeout=args.total_timeout)

    baseline_failed = False
    if args.baseline_dir:
        diff = _compare_baseline(out_dir, Path(args.baseline_dir).resolve(), out_dir)
        if args.fail_on_removed and diff.get("removed"):
            baseline_failed = True

    policy_failed = False
    if args.policy_file:
        verdict, _ = _enforce_policy(out_dir, Path(args.policy_file).resolve(), out_dir)
        if args.fail_on_policy_fail and verdict == "FAIL":
            policy_failed = True

    _suite_summary(out_dir)

    if baseline_failed or policy_failed:
        return 4
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
