"""Disposable loopback lab for GOST v3.3.3. Run: python run.py PATH_TO_GOST --out NEW_DIR."""
import argparse
import contextlib
import json
import os
import select
import socket
import socketserver
import sqlite3
import struct
import subprocess
import threading
import time
import uuid
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

IDS = {"alice": "tenant-a", "bob": "tenant-b"}
HOST = "127.0.0.1"


def read(sock, size):
    data = bytearray()
    while len(data) < size:
        chunk = sock.recv(size - len(data))
        if not chunk:
            raise EOFError(f"got {len(data)} of {size} bytes")
        data.extend(chunk)
    return bytes(data)


class Target(socketserver.BaseRequestHandler):
    def handle(self):
        self.request.settimeout(5)
        try:
            up, down = struct.unpack("!II", read(self.request, 8))
            read(self.request, up - 8)
            self.request.sendall(b"D" * down)
        except EOFError:
            pass


class Upstream(socketserver.BaseRequestHandler):
    """Minimal test SOCKS5 upstream, forwarding only to the local fixture."""
    def handle(self):
        client = self.request
        client.settimeout(5)
        ver, n = read(client, 2)
        assert ver == 5 and 2 in read(client, n)
        client.sendall(b"\x05\x02")
        assert read(client, 1) == b"\x01"
        name = read(client, read(client, 1)[0])
        password = read(client, read(client, 1)[0])
        assert (name, password) == (b"test-upstream", b"test-only")
        client.sendall(b"\x01\x00")
        assert read(client, 4) == b"\x05\x01\x00\x01"
        addr = socket.inet_ntoa(read(client, 4))
        port = struct.unpack("!H", read(client, 2))[0]
        assert addr == HOST
        self.server.connects.append(port)
        reply = b"\x00\x01\x00\x00\x00\x00\x00\x00"
        if port != self.server.target:
            client.sendall(b"\x05\x05" + reply)
            return
        with socket.create_connection((HOST, port), timeout=5) as target:
            client.sendall(b"\x05\x00" + reply)
            while True:
                ready, _, _ = select.select([client, target], [], [], 6)
                if not ready:
                    return
                for src in ready:
                    data = src.recv(65536)
                    if not data:
                        return
                    (target if src is client else client).sendall(data)


def flow(gateway, name, target, up=0, down=0, *, rejected=False, fail=False):
    with socket.create_connection((HOST, gateway), timeout=5) as conn:
        conn.sendall(b"\x05\x01\x02")
        assert read(conn, 2) == b"\x05\x02"
        user = name.encode()
        conn.sendall(b"\x01" + bytes([len(user)]) + user + b"\x09test-only")
        status = read(conn, 2)
        if rejected:
            assert status != b"\x01\x00"
            return
        assert status == b"\x01\x00", status
        conn.sendall(b"\x05\x01\x00\x01" + socket.inet_aton(HOST) + struct.pack("!H", target))
        reply = read(conn, 10)
        if fail:
            assert reply[1] != 0
            return reply[1]
        assert reply[1] == 0, reply
        if up:
            conn.sendall(struct.pack("!II", up, down) + b"U" * (up - 8))
            assert read(conn, down) == b"D" * down
        return reply[1]


class Receiver(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        payload = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        lab = self.server.lab
        if self.path == "/auth":
            tenant = IDS.get(payload.get("username")) if payload.get("password") == "test-only" else None
            self.respond(200, {"ok": bool(tenant), "id": tenant or ""})
            return
        instance, epoch = self.path.split("/")[2:4]
        mode = lab["mode"]
        if mode != "reject":
            # Cumulative snapshots only. Checkpoint + raw record + totals in one transaction.
            with sqlite3.connect(lab["db"]) as db:
                now = utc_now()
                previous = db.execute("SELECT epoch, received_at FROM raw WHERE instance=? ORDER BY rowid DESC LIMIT 1",
                                      (instance,)).fetchone()
                if previous and previous[0] != epoch:
                    db.execute("INSERT INTO gap VALUES (?, ?, ?, ?, ?)",
                               (instance, previous[0], epoch, previous[1], now))
                for event in payload["events"]:
                    s = event["stats"]
                    key = (instance, epoch, event["service"], event["client"])
                    before = db.execute("SELECT up, down FROM checkpoint WHERE instance=? AND epoch=? AND service=? AND tenant=?", key).fetchone() or (0, 0)
                    current = s["inputBytes"], s["outputBytes"]
                    delta = tuple(a-b for a, b in zip(current, before))
                    assert min(delta) >= 0, (key, before, current)
                    db.execute("INSERT INTO raw VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
                               (*key, now, json.dumps(event), *delta, mode))
                    db.execute("INSERT OR REPLACE INTO checkpoint VALUES (?, ?, ?, ?, ?, ?)", (*key, *current))
        lab["seen"].append({"instance": instance, "epoch": epoch, "mode": mode,
                            "received_at": utc_now(), "events": payload["events"]})
        if mode == "drop_ack":
            lab["mode"] = "ok"
            self.close_connection = True
            return
        self.respond(503 if mode == "reject" else 200, {"ok": mode != "reject"})

    def respond(self, status, value):
        data = json.dumps(value).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def utc_now():
    return datetime.now(timezone.utc).isoformat()


def serve(stack, cls, handler):
    server = stack.enter_context(cls((HOST, 0), handler))
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    stack.callback(server.shutdown)
    return server


def port(server):
    return server.server_address[1]


def free_port():
    with socket.socket() as s:
        s.bind((HOST, 0))
        return s.getsockname()[1]


def wait_for(predicate, description, timeout=8):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        value = predicate()
        if value:
            return value
        time.sleep(.025)
    raise RuntimeError(f"Timed out: {description}")


def listening(p):
    try:
        with socket.create_connection((HOST, p), timeout=.2):
            return True
    except OSError:
        return False


@contextlib.contextmanager
def gost(exe, out, receiver, upstream, name):
    gateway = free_port()
    epoch = name + "-" + uuid.uuid4().hex[:8]
    base = f"http://{HOST}:{receiver}"
    cfg = {
        "log": {"level": "error"},
        "services": [{"name": "lab-socks5", "addr": f"{HOST}:{gateway}",
                      "listener": {"type": "tcp"},
                      "handler": {"type": "socks5", "chain": "lab-upstream", "auther": "lab-auth",
                                  "observer": "lab-observer", "metadata": {
                                      "observer.period": "1s", "observer.resetTraffic": False}}}],
        "authers": [{"name": "lab-auth", "plugin": {"type": "http", "addr": base + "/auth", "timeout": "2s"}}],
        "observers": [{"name": "lab-observer", "plugin": {"type": "http",
                       "addr": base + "/observer/gw-1/" + epoch, "timeout": "2s"}}],
        "chains": [{"name": "lab-upstream", "hops": [{"name": "local", "nodes": [
            {"name": "local-socks", "addr": f"{HOST}:{upstream}", "dialer": {"type": "tcp"},
             "connector": {"type": "socks5", "auth": {"username": "test-upstream", "password": "test-only"}}}
        ]}]}],
    }
    path = out / (name + ".json")
    path.write_text(json.dumps(cfg, indent=2), encoding="utf-8")
    with (out / (name + ".log")).open("wb") as log:
        flags = subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0
        p = subprocess.Popen([exe, "-C", str(path)], stdout=log, stderr=subprocess.STDOUT, creationflags=flags)
        try:
            wait_for(lambda: listening(gateway), "GOST listener, see " + name + ".log")
            yield gateway, epoch
        finally:
            if p.poll() is None:
                p.kill()
            p.wait(timeout=5)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("gost", type=Path)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    exe, out = str(args.gost.resolve()), args.out.resolve()
    out.mkdir()
    version = subprocess.check_output([exe, "-V"], stderr=subprocess.STDOUT, text=True).strip()
    assert "3.3.3" in version, version
    lab = {"mode": "ok", "seen": [], "db": out / "scratch.sqlite"}
    with sqlite3.connect(lab["db"]) as db:
        db.executescript("""
        CREATE TABLE checkpoint(instance, epoch, service, tenant, up, down,
                                PRIMARY KEY(instance, epoch, service, tenant));
        CREATE TABLE raw(instance, epoch, service, tenant, received_at, payload, up, down, mode);
        CREATE TABLE gap(instance, previous_epoch, new_epoch, last_received, first_new_received);
        """)
    report = {"version": version, "scenarios": {}}
    scenarios = report["scenarios"]

    def total(epoch, tenant):
        with sqlite3.connect(lab["db"]) as db:
            return db.execute("SELECT COALESCE(SUM(up),0), COALESCE(SUM(down),0) FROM raw WHERE epoch=? AND tenant=?",
                              (epoch, tenant)).fetchone()

    def row_count(epoch, tenant):
        with sqlite3.connect(lab["db"]) as db:
            return db.execute("SELECT COUNT(*) FROM raw WHERE epoch=? AND tenant=?", (epoch, tenant)).fetchone()[0]

    def changed(epoch, tenant, before):
        wait_for(lambda: total(epoch, tenant) != before, f"{epoch}/{tenant} new usage")
        return tuple(a-b for a,b in zip(total(epoch, tenant), before))

    def attempts(epoch, mode):
        return [x for x in lab["seen"] if x["epoch"] == epoch and x["mode"] == mode]

    def save():
        report["upstream_connects"] = len(upstream.connects)
        (out / "report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
        (out / "observer-attempts.json").write_text(json.dumps(lab["seen"], indent=2), encoding="utf-8")

    with contextlib.ExitStack() as stack:
        target = serve(stack, socketserver.ThreadingTCPServer, Target)
        upstream = serve(stack, socketserver.ThreadingTCPServer, Upstream)
        upstream.target, upstream.connects = port(target), []
        receiver = serve(stack, ThreadingHTTPServer, Receiver)
        receiver.lab = lab
        try:
            with gost(exe, out, port(receiver), port(upstream), "before") as (gw, epoch):
                report["before_epoch"] = epoch
                flow(gw, "bad-user", port(target), rejected=True)
                assert not upstream.connects
                flow(gw, "alice", port(target), 1024, 2048)
                flow(gw, "bob", port(target), 512, 256)
                scenarios["tenant_a"] = {"fixture": [1024, 2048], "observer": changed(epoch, "tenant-a", (0, 0))}
                scenarios["tenant_b"] = {"fixture": [512, 256], "observer": changed(epoch, "tenant-b", (0, 0))}
                assert len(upstream.connects) == 2
                before = total(epoch, "tenant-a")
                flow(gw, "alice", port(target))
                scenarios["empty"] = {"fixture": [0, 0], "observer": changed(epoch, "tenant-a", before)}
                before = total(epoch, "tenant-a")
                reply = flow(gw, "alice", 1, fail=True)
                scenarios["upstream_fail"] = {"fixture": [0, 0], "observer": changed(epoch, "tenant-a", before), "reply": reply}

                before, count = total(epoch, "tenant-a"), row_count(epoch, "tenant-a")
                lab["mode"] = "drop_ack"
                flow(gw, "alice", port(target), 111, 222)
                wait_for(lambda: row_count(epoch, "tenant-a") >= count + 2, "retry after committed response was lost")
                dropped = attempts(epoch, "drop_ack")[0]
                repeat = next(x for x in attempts(epoch, "ok") if x["events"] == dropped["events"])
                scenarios["ack_loss"] = {"fixture": [111, 222], "added_once": changed(epoch, "tenant-a", before),
                                         "identical_payload_retry": bool(repeat)}
                with sqlite3.connect(lab["db"]) as db:
                    last = db.execute("SELECT up,down FROM raw WHERE epoch=? AND tenant=? ORDER BY rowid DESC LIMIT 1",
                                      (epoch, "tenant-a")).fetchone()
                assert last == (0, 0), last

                before = total(epoch, "tenant-a")
                lab["mode"] = "reject"
                rejects = len(attempts(epoch, "reject"))
                flow(gw, "alice", port(target), 444, 777)
                wait_for(lambda: len(attempts(epoch, "reject")) >= rejects + 2, "two failed attempts")
                assert total(epoch, "tenant-a") == before
                lab["mode"] = "ok"
                scenarios["temporary_outage"] = {"fixture": [444, 777], "after_recovery": changed(epoch, "tenant-a", before),
                                                 "rejected_attempts": len(attempts(epoch, "reject"))-rejects}

                before = total(epoch, "tenant-a")
                lab["mode"] = "reject"
                rejects = len(attempts(epoch, "reject"))
                flow(gw, "alice", port(target), 321, 654)
                wait_for(lambda: len(attempts(epoch, "reject")) > rejects, "unacknowledged report before kill")
                rejected = attempts(epoch, "reject")[-1]["events"][0]["stats"]
                scenarios["restart_gap"] = {"fixture": [321, 654],
                                            "observed_but_not_durable": [rejected["inputBytes"]-before[0], rejected["outputBytes"]-before[1]],
                                            "persisted_old_epoch_before_kill": before}
            lab["mode"] = "ok"
            with gost(exe, out, port(receiver), port(upstream), "after") as (gw, new_epoch):
                report["after_epoch"] = new_epoch
                flow(gw, "alice", port(target), 123, 234)
                after = changed(new_epoch, "tenant-a", (0, 0))
                assert total(epoch, "tenant-a") == before
                with sqlite3.connect(lab["db"]) as db:
                    db.row_factory = sqlite3.Row
                    gap = db.execute("SELECT * FROM gap").fetchall()
                    assert len(gap) == 1 and gap[0]["previous_epoch"] == epoch and gap[0]["new_epoch"] == new_epoch
                    report["potential_gap_marker"] = dict(gap[0])
                scenarios["restart_gap"].update({"persisted_old_epoch_after_restart": total(epoch, "tenant-a"),
                                                 "new_epoch_fixture": [123, 234], "new_epoch_observer": after,
                                                 "old_pending_replayed": False})
        finally:
            save()
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
