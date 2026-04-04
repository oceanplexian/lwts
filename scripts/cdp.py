#!/usr/bin/env python3
"""CDP helper — connects to Chrome DevTools Protocol via IPv6 to avoid
   the regular-Chrome-on-IPv4 conflict. Usage:

     python3 cdp.py screenshot /tmp/out.png
     python3 cdp.py eval "document.title"
     python3 cdp.py navigate "http://localhost:8099/"
     python3 cdp.py nav_eval_screenshot "http://url" "jsExpr" /tmp/out.png [delay]
     python3 cdp.py full "http://url" "js1" "js2" ... -- /tmp/out.png [delay]
"""
import http.client, os, struct, json, time, base64, sys, socket

CDP_HOST = "::1"
CDP_PORT = 9222

# ── raw WebSocket over IPv6 ──────────────────────────────────────────

def _ws_connect(host, port, path):
    key = base64.b64encode(os.urandom(16)).decode()
    # Force IPv6
    sock = socket.socket(socket.AF_INET6, socket.SOCK_STREAM)
    sock.settimeout(10)
    sock.connect((host, port, 0, 0))
    req = (
        f"GET {path} HTTP/1.1\r\n"
        f"Host: localhost:{port}\r\n"
        f"Upgrade: websocket\r\n"
        f"Connection: Upgrade\r\n"
        f"Sec-WebSocket-Key: {key}\r\n"
        f"Sec-WebSocket-Version: 13\r\n\r\n"
    )
    sock.sendall(req.encode())
    resp = b""
    while b"\r\n\r\n" not in resp:
        resp += sock.recv(4096)
    status_line = resp.split(b"\r\n")[0].decode()
    if "101" not in status_line:
        raise ConnectionError(f"WebSocket handshake failed: {status_line}")
    return sock

def _ws_send(sock, data):
    payload = data.encode() if isinstance(data, str) else data
    mask = os.urandom(4)
    frame = bytearray([0x81])
    ln = len(payload)
    if ln < 126:
        frame.append(0x80 | ln)
    elif ln < 65536:
        frame.append(0x80 | 126)
        frame.extend(struct.pack(">H", ln))
    else:
        frame.append(0x80 | 127)
        frame.extend(struct.pack(">Q", ln))
    frame.extend(mask)
    frame.extend(bytes(b ^ mask[i % 4] for i, b in enumerate(payload)))
    sock.sendall(frame)

def _ws_recv(sock):
    def _read_exact(n):
        buf = bytearray()
        while len(buf) < n:
            chunk = sock.recv(n - len(buf))
            if not chunk:
                raise ConnectionError("socket closed")
            buf.extend(chunk)
        return bytes(buf)

    header = _read_exact(2)
    opcode = header[0] & 0x0F
    masked = bool(header[1] & 0x80)
    ln = header[1] & 0x7F
    if ln == 126:
        ln = struct.unpack(">H", _read_exact(2))[0]
    elif ln == 127:
        ln = struct.unpack(">Q", _read_exact(8))[0]
    if masked:
        mask_key = _read_exact(4)
    data = _read_exact(ln)
    if masked:
        data = bytes(b ^ mask_key[i % 4] for i, b in enumerate(data))
    return data.decode() if opcode == 1 else data


# ── CDP session via browser endpoint ─────────────────────────────────

class CDP:
    def __init__(self):
        info = self._http_get("/json/version")
        ws_url = info["webSocketDebuggerUrl"]
        browser_ws_path = ws_url.split(f":{CDP_PORT}", 1)[1]
        self.sock = _ws_connect(CDP_HOST, CDP_PORT, browser_ws_path)
        self._id = 0
        self.session_id = None
        self.target_id = None

    def _http_get(self, path):
        import subprocess
        r = subprocess.run(
            ["curl", "-s", f"http://[::1]:{CDP_PORT}{path}"],
            capture_output=True, text=True, timeout=5
        )
        return json.loads(r.stdout)

    def send(self, method, params=None):
        self._id += 1
        msg = {"id": self._id, "method": method}
        if params:
            msg["params"] = params
        if self.session_id:
            msg["sessionId"] = self.session_id
        _ws_send(self.sock, json.dumps(msg))
        # Read until we get our response (skip events)
        while True:
            raw = _ws_recv(self.sock)
            data = json.loads(raw)
            if data.get("id") == self._id:
                if "error" in data:
                    raise RuntimeError(f"CDP error: {data['error']}")
                return data.get("result", {})

    def create_tab(self, url="about:blank"):
        r = self.send("Target.createTarget", {"url": url})
        self.target_id = r["targetId"]
        r = self.send("Target.attachToTarget", {"targetId": self.target_id, "flatten": True})
        self.session_id = r["sessionId"]
        return self.target_id

    def navigate(self, url, wait=2):
        self.send("Page.navigate", {"url": url})
        time.sleep(wait)

    def evaluate(self, expr):
        r = self.send("Runtime.evaluate", {"expression": expr, "returnByValue": True})
        return r.get("result", {}).get("value")

    def screenshot(self, path):
        r = self.send("Page.captureScreenshot", {"format": "png"})
        with open(path, "wb") as f:
            f.write(base64.b64decode(r["data"]))
        return path

    def close(self):
        if self.target_id:
            try:
                self.session_id = None  # send on browser session
                self.send("Target.closeTarget", {"targetId": self.target_id})
            except Exception:
                pass
        try:
            self.sock.close()
        except Exception:
            pass


# ── CLI ──────────────────────────────────────────────────────────────

def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]
    cdp = CDP()

    if cmd == "screenshot":
        # Attach to first non-devtools page
        tabs = cdp._http_get("/json")
        tab = next(t for t in tabs if "devtools://" not in t["url"])
        cdp.target_id = tab["id"]
        r = cdp.send("Target.attachToTarget", {"targetId": tab["id"], "flatten": True})
        cdp.session_id = r["sessionId"]
        out = sys.argv[2] if len(sys.argv) > 2 else "/tmp/cdp-screenshot.png"
        cdp.screenshot(out)
        print(out)

    elif cmd == "eval":
        tabs = cdp._http_get("/json")
        tab = next(t for t in tabs if "devtools://" not in t["url"])
        r = cdp.send("Target.attachToTarget", {"targetId": tab["id"], "flatten": True})
        cdp.session_id = r["sessionId"]
        result = cdp.evaluate(sys.argv[2])
        print(result)

    elif cmd == "navigate":
        tabs = cdp._http_get("/json")
        tab = next(t for t in tabs if "devtools://" not in t["url"])
        r = cdp.send("Target.attachToTarget", {"targetId": tab["id"], "flatten": True})
        cdp.session_id = r["sessionId"]
        cdp.navigate(sys.argv[2])
        print("navigated")

    elif cmd == "nav_eval_screenshot":
        # Create a fresh tab, navigate, eval JS, screenshot, close
        url = sys.argv[2]
        js = sys.argv[3]
        out = sys.argv[4] if len(sys.argv) > 4 else "/tmp/cdp-screenshot.png"
        delay = float(sys.argv[5]) if len(sys.argv) > 5 else 2
        cdp.create_tab(url)
        time.sleep(delay)
        if js and js.strip():
            r = cdp.evaluate(js)
            print(f"eval: {r}")
            time.sleep(1)
        cdp.screenshot(out)
        print(out)
        cdp.close()

    elif cmd == "full":
        # full <url> <js1> <js2> ... -- <output.png> [delay]
        args = sys.argv[2:]
        if "--" in args:
            sep = args.index("--")
            js_exprs = args[:sep]
            rest = args[sep + 1:]
        else:
            js_exprs = args[:-1]
            rest = args[-1:]
        url = js_exprs.pop(0) if js_exprs else "about:blank"
        out = rest[0] if rest else "/tmp/cdp-screenshot.png"
        delay = float(rest[1]) if len(rest) > 1 else 2

        cdp.create_tab(url)
        time.sleep(delay)
        for js in js_exprs:
            if js.strip():
                r = cdp.evaluate(js)
                print(f"eval: {r}")
                time.sleep(0.5)
        time.sleep(0.5)
        cdp.screenshot(out)
        print(out)
        cdp.close()

    else:
        print(f"Unknown command: {cmd}")
        sys.exit(1)

if __name__ == "__main__":
    main()
