#!/usr/bin/env python3
"""One-shot unix socket that answers a single inspect JSON-line request.

The CLI dials, writes one Request, and reads one Response. This is that
other end: bind, accept, discard the request, write argv's canned JSON.
"""
import os
import socket
import sys

path, response_path = sys.argv[1], sys.argv[2]
with open(response_path, encoding="utf-8") as fh:
    body = fh.read()
if not body.endswith("\n"):
    body += "\n"

try:
    os.unlink(path)
except FileNotFoundError:
    pass

srv = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
srv.bind(path)
srv.listen(1)
srv.settimeout(5)
conn, _ = srv.accept()
with conn:
    buf = b""
    while b"\n" not in buf:
        chunk = conn.recv(4096)
        if not chunk:
            break
        buf += chunk
    conn.sendall(body.encode("utf-8"))
srv.close()
