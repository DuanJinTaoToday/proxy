"""Disposable loopback experiment using the official GOST executable."""

import contextlib
import json
from pathlib import Path
import socket
import socketserver
import struct
import subprocess
import sys
import tempfile
import threading
import time
import uuid


class Echo(socketserver.BaseRequestHandler):
    def handle(self):
        while data := self.request.recv(8192):
            self.request.sendall(data)


def exact(conn, size):
    data = bytearray()
    while len(data) < size:
        chunk = conn.recv(size - len(data))
        assert chunk, "unexpected EOF"
        data.extend(chunk)
    return bytes(data)


@contextlib.contextmanager
def gost(exe, root, rotate=False, invalid_path=False):
    directory = root / str(uuid.uuid4())
    directory.mkdir()
    path = directory / "sessions.jsonl"
    with socket.socket() as reserved:
        reserved.bind(("127.0.0.1", 0))
        port = reserved.getsockname()[1]
    file = {"path": str(path), "sep": "\n"}
    before = None
    if rotate:
        file["rotation"] = {"maxSize": 1}
        path.write_bytes(b'{"pad":"' + b'x' * ((1 << 20) - 13) + b'"}\n')
        before = path.stat()
        assert before.st_size == (1 << 20) - 2
    if invalid_path:
        path.mkdir()
    config = directory / "gost.json"
    config.write_text(json.dumps({
        "recorders": [{"name": "usage", "file": file}],
        "services": [{
            "name": "lab", "addr": f"127.0.0.1:{port}",
            "listener": {"type": "tcp"},
            "handler": {"type": "socks5", "metadata": {"recorder.period": "1s"}},
            "recorders": [{"name": "usage", "record": "recorder.service.handler"}],
        }],
    }), encoding="utf-8")
    with (directory / "gost.log").open("wb") as log:
        child = subprocess.Popen([exe, "-C", str(config)], stdout=log, stderr=subprocess.STDOUT)
        try:
            deadline = time.monotonic() + 10
            while True:
                try:
                    conn = socket.create_connection(("127.0.0.1", port), timeout=0.2)
                    break
                except OSError:
                    assert time.monotonic() < deadline and child.poll() is None, directory
                    time.sleep(0.05)
            with conn:
                conn.settimeout(5)
                yield conn, path, before
        finally:
            child.kill()
            child.wait(timeout=5)


def exchange(conn, target_port, path=None):
    conn.sendall(b"\x05\x01\x00")
    assert exact(conn, 2) == b"\x05\x00"
    conn.sendall(b"\x05\x01\x00\x01\x7f\x00\x00\x01" + struct.pack("!H", target_port))
    assert exact(conn, 10)[:2] == b"\x05\x00"
    for count, n in enumerate((32, 64), 1):
        conn.sendall(b"x" * n)
        assert exact(conn, n) == b"x" * n
        if path is not None:
            deadline = time.monotonic() + 5
            while True:
                data = path.read_bytes() if path.is_file() else b""
                if data.count(b'"phase":"interim"') == count and data.endswith(b"\n"):
                    break
                assert time.monotonic() < deadline, f"missing interim {count} in {path}"
                time.sleep(0.05)
    conn.shutdown(socket.SHUT_WR)
    assert conn.recv(1) == b""


def records(path):
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline:
        if path.is_file():
            data = path.read_bytes()
            if data.endswith(b"\n"):
                rows = [json.loads(line) for line in data.splitlines()]
                if rows[-1]["phase"] == "final":
                    return data, rows
        time.sleep(0.05)
    raise AssertionError(f"missing final in {path}")


def main(exe):
    root = Path(tempfile.mkdtemp(prefix="gost-recorder-binary-"))
    print(root)
    print(subprocess.check_output([exe, "-V"], stderr=subprocess.STDOUT, text=True).strip())
    with socketserver.ThreadingTCPServer(("127.0.0.1", 0), Echo) as target:
        server = threading.Thread(target=target.serve_forever, daemon=True)
        server.start()
        try:
            for rotate in (True, False, False):
                with gost(exe, root, rotate) as (conn, path, before):
                    exchange(conn, target.server_address[1], path)
                    data, rows = records(path)
                    assert [row["phase"] for row in rows] == ["interim", "interim", "final"], rows
                    for counter in ("inputBytes", "outputBytes"):
                        previous = 0
                        for index, row in enumerate(rows, 1):
                            assert row["recordIndex"] == index
                            total = row.get(counter, 0)
                            assert row[counter + "Delta"] == total - previous
                            previous = total
                    if rotate:
                        backup, = path.parent.glob("sessions-*.jsonl")
                        assert before is not None and before.st_ino != 0
                        assert same_identity(before, backup.stat())
                        assert not same_identity(before, path.stat())
                        print(f"renamed={backup.name} stable_identity={before.st_dev}:{before.st_ino}")
                        print(data.decode(), end="")
                    print(f"boot={path.parent.name} phases={[row['phase'] for row in rows]}")
            with gost(exe, root, invalid_path=True) as (conn, path, _):
                exchange(conn, target.server_address[1])
                assert path.is_dir() and not list(path.iterdir())
                print("invalid Recorder path: SOCKS5 exchange succeeded; no file created")
        finally:
            target.shutdown()
            server.join()


def same_identity(left, right):
    return (left.st_dev, left.st_ino) == (right.st_dev, right.st_ino)


if __name__ == "__main__":
    main(str(Path(sys.argv[1]).resolve()))
