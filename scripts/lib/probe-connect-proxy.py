#!/usr/bin/env python3
"""Harness-owned local CONNECT proxy for one sealed skill-probe capture.

WHY: the outer seatbelt profile is `(allow default)` and codex's own sandbox is
bypassed inside it (seatbelt does not nest), so before this proxy a sealed rep
could fetch the canonical SKILL.md straight off GitHub and the filesystem seal
proved nothing about what the rep read. The seal now denies `network*` except
outbound to this proxy, and the proxy allows CONNECT only to an explicit host
allowlist. Everything else is refused with 403 and logged; a refused CONNECT
degrades that rep (`network-egress`).

It speaks only the CONNECT half of HTTP proxying, which is what an HTTPS client
uses: an absolute-URI plain HTTP request is refused like any other destination.
Standard library only, no third-party dependency.

Usage:
  python3 probe-connect-proxy.py --log FILE --port-file FILE
      [--allow-host HOST]... [--allow-port PORT]... [--allow-any]
      [--rep-file FILE] [--allow-private-upstream]

An `--allow-host` beginning with a dot is an explicit domain suffix. Only the
ports named by `--allow-port` are admitted (default 443). A destination whose
name resolves into loopback, link-local or private space is refused even when
the name is on the allowlist, because otherwise a rebinding answer turns an
allowed name into a local service. `--allow-private-upstream` lifts that check
and exists only so the test suite can stand up a local upstream; a capture
never passes it.

`--allow-any` is DISCOVERY mode: it permits and logs every destination so an
operator can learn which hosts the producer needs before pinning the allowlist.
It is never used for a capture; a capture whose proxy ran in discovery mode is
recorded with `network.mode: proxy-discovery`, which is not coverage-eligible.
"""

from __future__ import annotations

import argparse
import json
import os
import ipaddress
import select
import socket
import sys
import threading
import time

BUFFER_BYTES = 65536
CONNECT_TIMEOUT_SECONDS = 20
IDLE_TIMEOUT_SECONDS = 300
MAX_REQUEST_BYTES = 8192


class ProxyLog:
    """Append-only JSONL: an `attempt` record, then the decision that followed.

    The rep is captured when the connection is ACCEPTED, not when the line is
    written: a decision can land after the rep that opened the connection has
    exited, and attributing it to whichever rep happened to be current then
    would put one rep's egress on another rep's record.
    """

    def __init__(self, path: str, rep_file: str | None) -> None:
        self._path = path
        self._rep_file = rep_file
        self._lock = threading.Lock()

    def current_rep(self) -> str | None:
        """The rep the harness says is running right now, read at accept time."""
        if not self._rep_file:
            return None
        try:
            with open(self._rep_file, encoding="utf-8") as handle:
                return handle.read().strip() or None
        except OSError:
            return None

    def write(
        self, host: str, port: int, decision: str, detail: str = "", rep: str | None = None
    ) -> None:
        record = {
            "ts": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "rep": rep,
            "host": host,
            "port": port,
            "decision": decision,
        }
        if detail:
            record["detail"] = detail
        line = json.dumps(record, ensure_ascii=False, sort_keys=True) + "\n"
        with self._lock:
            with open(self._path, "a", encoding="utf-8") as handle:
                handle.write(line)
                handle.flush()
                os.fsync(handle.fileno())


def host_allowed(host: str, allowed: frozenset[str]) -> bool:
    """Exact host match, or a leading-dot entry as an explicit domain suffix.

    `chatgpt.com` matches only that host. `.oaiusercontent.com` matches any host
    under that domain, which is needed because the OpenAI content hosts carry a
    rotating region prefix (sdmntprsouthcentralus, sdmntprcentralus, ...) that
    no fixed list survives. It is still an allowlist: one named vendor domain,
    written where a reader can see it, not a wildcard.
    """
    name = host.lower()
    for entry in allowed:
        if entry.startswith("."):
            if name.endswith(entry) or name == entry[1:]:
                return True
        elif name == entry:
            return True
    return False


def pump(client: socket.socket, upstream: socket.socket) -> None:
    """Relay both directions until either side closes or the pair goes idle."""
    sockets = [client, upstream]
    try:
        while True:
            readable, _, errored = select.select(sockets, [], sockets, IDLE_TIMEOUT_SECONDS)
            if errored or not readable:
                return
            for source in readable:
                target = upstream if source is client else client
                try:
                    chunk = source.recv(BUFFER_BYTES)
                except OSError:
                    return
                if not chunk:
                    return
                try:
                    target.sendall(chunk)
                except OSError:
                    return
    finally:
        for handle in sockets:
            try:
                handle.close()
            except OSError:
                pass


def read_request_line(client: socket.socket) -> tuple[str, bytes]:
    """The first line of the proxy request, with any already-buffered remainder."""
    buffered = b""
    while b"\r\n" not in buffered:
        if len(buffered) > MAX_REQUEST_BYTES:
            return "", buffered
        try:
            chunk = client.recv(BUFFER_BYTES)
        except OSError:
            return "", buffered
        if not chunk:
            return "", buffered
        buffered += chunk
    line, _, rest = buffered.partition(b"\r\n")
    return line.decode("latin-1"), rest


def refuse(client: socket.socket, status: str, message: str) -> None:
    body = message.encode("utf-8")
    response = (
        f"HTTP/1.1 {status}\r\n"
        "Content-Type: text/plain\r\n"
        f"Content-Length: {len(body)}\r\n"
        "Connection: close\r\n"
        "\r\n"
    ).encode("latin-1") + body
    try:
        client.sendall(response)
    except OSError:
        pass
    try:
        client.close()
    except OSError:
        pass


def parse_authority(target: str) -> tuple[str, int] | None:
    if target.startswith("["):  # bracketed IPv6 literal
        closing = target.find("]")
        if closing < 0:
            return None
        host = target[1:closing]
        remainder = target[closing + 1 :]
        if not remainder.startswith(":"):
            return None
        port_text = remainder[1:]
    else:
        host, separator, port_text = target.partition(":")
        if not separator:
            return None
    if not host or not port_text.isdigit():
        return None
    port = int(port_text)
    if not 0 < port < 65536:
        return None
    return host, port


def local_address(text: str) -> bool:
    """True when an address sits in loopback, link-local or private space."""
    try:
        address = ipaddress.ip_address(text)
    except ValueError:
        return False
    return (
        address.is_loopback
        or address.is_link_local
        or address.is_private
        or address.is_reserved
        or address.is_multicast
        or address.is_unspecified
    )


def resolve_public(host: str, port: int) -> tuple[list[tuple], str]:
    """Resolve HOST, refusing any answer that points into local space.

    An allowlisted NAME is not an allowlisted destination: a rebinding answer
    for `chatgpt.com` would otherwise tunnel the rep into a local service.
    """
    try:
        infos = socket.getaddrinfo(host, port, proto=socket.IPPROTO_TCP)
    except OSError as exc:
        return [], f"could not resolve: {exc}"
    for info in infos:
        address = info[4][0]
        if local_address(str(address)):
            return [], f"resolves into local address space: {address}"
    return infos, ""


def connect_upstream(infos: list[tuple]) -> socket.socket:
    last: OSError | None = None
    for family, kind, proto, _canonical, sockaddr in infos:
        try:
            upstream = socket.socket(family, kind, proto)
            upstream.settimeout(CONNECT_TIMEOUT_SECONDS)
            upstream.connect(sockaddr)
            return upstream
        except OSError as exc:
            last = exc
    raise last if last is not None else OSError("no address to connect to")


def serve_client(
    client: socket.socket,
    allowed: frozenset[str],
    ports: frozenset[int],
    allow_any: bool,
    allow_private: bool,
    log: ProxyLog,
    rep: str | None,
) -> None:
    client.settimeout(CONNECT_TIMEOUT_SECONDS)
    request_line, _ = read_request_line(client)
    parts = request_line.split()
    if len(parts) != 3 or parts[0].upper() != "CONNECT":
        log.write("", 0, "refused", "not a CONNECT request", rep=rep)
        refuse(client, "405 Method Not Allowed", "probe proxy accepts CONNECT only\n")
        return
    authority = parse_authority(parts[1])
    if authority is None:
        log.write(parts[1], 0, "refused", "unparseable authority", rep=rep)
        refuse(client, "400 Bad Request", "probe proxy could not parse the authority\n")
        return
    host, port = authority
    # The attempt goes on the record BEFORE anything is resolved or dialed, so a
    # connection that dies mid-flight still leaves a trace of what was asked for.
    log.write(host, port, "attempt", rep=rep)
    if not (allow_any or host_allowed(host, allowed)):
        log.write(host, port, "refused", "host is not on the capture allowlist", rep=rep)
        refuse(client, "403 Forbidden", "probe proxy: host is not on the capture allowlist\n")
        return
    if port not in ports:
        log.write(host, port, "refused", "port is not on the capture allowlist", rep=rep)
        refuse(client, "403 Forbidden", "probe proxy: port is not on the capture allowlist\n")
        return
    if allow_private:
        infos, problem = socket.getaddrinfo(host, port, proto=socket.IPPROTO_TCP), ""
    else:
        infos, problem = resolve_public(host, port)
    if problem:
        log.write(host, port, "refused", problem, rep=rep)
        refuse(client, "403 Forbidden", "probe proxy: destination is not reachable policy\n")
        return
    try:
        upstream = connect_upstream(infos)
    except OSError as exc:
        log.write(host, port, "failed", f"upstream connect failed: {exc}", rep=rep)
        refuse(client, "502 Bad Gateway", "probe proxy could not reach the destination\n")
        return
    log.write(host, port, "allowed", rep=rep)
    try:
        client.sendall(b"HTTP/1.1 200 Connection established\r\n\r\n")
    except OSError:
        upstream.close()
        client.close()
        return
    client.settimeout(None)
    upstream.settimeout(None)
    pump(client, upstream)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--allow-host", action="append", default=[])
    parser.add_argument("--allow-port", action="append", type=int, default=[])
    parser.add_argument("--allow-any", action="store_true")
    parser.add_argument("--allow-private-upstream", action="store_true")
    parser.add_argument("--log", required=True)
    parser.add_argument("--port-file", required=True)
    parser.add_argument("--rep-file")
    args = parser.parse_args()

    allowed = frozenset(host.lower() for host in args.allow_host)
    ports = frozenset(args.allow_port or [443])
    if not allowed and not args.allow_any:
        print("probe-connect-proxy: an allowlist or --allow-any is required", file=sys.stderr)
        return 2
    log = ProxyLog(args.log, args.rep_file)

    listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listener.bind(("127.0.0.1", 0))
    listener.listen(64)
    port = listener.getsockname()[1]
    with open(args.port_file, "w", encoding="utf-8") as handle:
        handle.write(f"{port}\n")
    print(port, flush=True)

    while True:
        try:
            client, _ = listener.accept()
        except OSError:
            return 0
        worker = threading.Thread(
            target=serve_client,
            args=(
                client,
                allowed,
                ports,
                args.allow_any,
                args.allow_private_upstream,
                log,
                log.current_rep(),
            ),
            daemon=True,
        )
        worker.start()


if __name__ == "__main__":
    raise SystemExit(main())
