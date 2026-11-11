"""Local IPCrown catalog and simulated checkout."""

import argparse
import getpass
import hashlib
import json
import os
from decimal import Decimal, InvalidOperation
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import secrets
import time
from urllib.request import HTTPRedirectHandler, Request, build_opener


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *args):
        return None


def fetch_catalog(partner_id, secret):
    params = {"cid": 200001, "nonce": secrets.token_hex(16), "timestamp": int(time.time())}
    signed = "&".join(f"{key}={params[key]}" for key in sorted(params))
    params["sign"] = hashlib.md5(f"{signed}&key={secret}".encode()).hexdigest().upper()
    request = Request(
        "http://www.ipcrown.com/open/api/nodes-and-continents",
        data=json.dumps(params).encode(),
        headers={"Content-Type": "application/json", "X-Partner-ID": partner_id},
    )
    with build_opener(NoRedirect).open(request, timeout=10) as response:
        result = json.load(response)
    if result["code"] != 0:
        raise ValueError("IPCrown rejected catalog query")

    nodes = []
    for node in result["data"]["nodes"]:
        prices = []
        for price in node["prices"]:
            if price["price_type"] not in ("yp1", "yp3", "yp6", "np"):
                continue
            amount = Decimal(str(price["price"]))
            if amount.is_finite() and amount >= 0:
                prices.append({"period": price["price_type"], "unitPrice": f"{amount:.2f}"})
        nodes.append({
            "id": int(node["id"]),
            "name": node["title"],
            "isp": node["isp"],
            "line": node.get("line") or "",
            "continentId": int(node["continent_id"]),
            "available": max(0, int(node["max"])),
            "prices": prices,
        })
    continents = [
        {"id": int(item["id"]), "name": item["name"]}
        for item in result["data"]["continents"]
    ]
    return {"nodes": nodes, "continents": continents, "fetchedAt": int(time.time())}


class Handler(BaseHTTPRequestHandler):
    def send_content(self, status, content_type, content):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(content)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(content)

    def send_json(self, status, data):
        self.send_content(status, "application/json; charset=utf-8", json.dumps(data).encode())

    def catalog(self):
        try:
            return fetch_catalog(*self.server.credentials)
        except (OSError, ValueError, KeyError, TypeError, InvalidOperation):
            self.send_json(502, {"error": "IPCrown 目录读取失败，请检查网络与凭证后重试。"})
            return None

    def do_GET(self):
        if self.path == "/":
            page = Path(__file__).with_name("index.html").read_bytes()
            self.send_content(200, "text/html; charset=utf-8", page)
        elif self.path == "/api/catalog":
            catalog = self.catalog()
            if catalog is not None:
                self.send_json(200, catalog)
        else:
            self.send_json(404, {"error": "接口不存在。"})

    def do_POST(self):
        if self.path != "/api/simulated-orders":
            self.send_json(404, {"error": "接口不存在。"})
            return
        try:
            size = int(self.headers.get("Content-Length", "0"))
            if not 0 < size <= 2048 or self.headers.get_content_type() != "application/json":
                raise ValueError
            order = json.loads(self.rfile.read(size))
            node_id, period, quantity = (order[key] for key in ("nodeId", "period", "quantity"))
            if type(node_id) is not int or type(quantity) is not int or quantity < 1:
                raise ValueError
        except (ValueError, KeyError, TypeError):
            self.send_json(400, {"error": "请选择节点、周期，并填写正整数数量。"})
            return

        catalog = self.catalog()
        if catalog is None:
            return
        node = next((item for item in catalog["nodes"] if item["id"] == node_id), None)
        price = next((item for item in node["prices"] if item["period"] == period), None) if node else None
        if price is None or quantity > node["available"]:
            self.send_json(409, {"error": "节点库存或周期已变化，请刷新后重试。"})
            return
        if order.get("unitPrice") != price["unitPrice"]:
            self.send_json(409, {"error": "节点价格已变化，请刷新后确认新价格。"})
            return
        total = Decimal(price["unitPrice"]) * quantity
        self.send_json(201, {
            "orderId": f"SIM-{secrets.token_hex(5).upper()}",
            "node": node["name"],
            "period": period,
            "quantity": quantity,
            "total": f"{total:.2f}",
            "charged": "0.00",
        })


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--port", type=int, default=8787)
    args = parser.parse_args()
    partner_id = os.environ.get("IPCROWN_PARTNER_ID") or input("IPCrown Partner ID: ").strip()
    secret = os.environ.get("IPCROWN_API_SECRET") or getpass.getpass("IPCrown API Secret: ").strip()
    with ThreadingHTTPServer(("127.0.0.1", args.port), Handler) as server:
        server.credentials = (partner_id, secret)
        print(f"http://127.0.0.1:{args.port}", flush=True)
        server.serve_forever()
