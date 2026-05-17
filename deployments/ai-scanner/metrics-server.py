#!/usr/bin/env python3
"""Tiny Prometheus endpoint for ai-scanner cron run status."""

from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

METRICS_DIR = Path("/workspace/metrics")


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path not in ("/metrics", "/metrics/"):
            self.send_response(404)
            self.end_headers()
            return
        parts = []
        for path in sorted(METRICS_DIR.glob("*.prom")):
            parts.append(path.read_text(encoding="utf-8"))
        body = "\n".join(parts)
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        self.send_header("Content-Length", str(len(body.encode("utf-8"))))
        self.end_headers()
        self.wfile.write(body.encode("utf-8"))

    def log_message(self, fmt, *args):
        return


if __name__ == "__main__":
    METRICS_DIR.mkdir(parents=True, exist_ok=True)
    HTTPServer(("0.0.0.0", 9130), Handler).serve_forever()
