#!/usr/bin/env python3
"""Small 24alert ops exporter for host, Docker and deployment metadata."""

import json
import os
import socket
import time
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

DOCKER_SOCK = "/var/run/docker.sock"
HOST_ROOT = Path("/host")
REPO_GIT = Path("/repo/.git")


def docker_get(path):
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
        sock.settimeout(3)
        sock.connect(DOCKER_SOCK)
        req = f"GET {path} HTTP/1.1\r\nHost: docker\r\nConnection: close\r\n\r\n"
        sock.sendall(req.encode("ascii"))
        chunks = []
        while True:
            chunk = sock.recv(65536)
            if not chunk:
                break
            chunks.append(chunk)
    raw = b"".join(chunks)
    _, _, body = raw.partition(b"\r\n\r\n")
    return json.loads(body.decode("utf-8"))


def label_value(value):
    return str(value or "").replace("\\", "\\\\").replace("\n", " ").replace('"', '\\"')


def parse_time(value):
    if not value:
        return 0
    try:
        normalized = value.replace("Z", "+00:00")
        return int(datetime.fromisoformat(normalized).timestamp())
    except ValueError:
        return 0


def git_commit():
    head = REPO_GIT / "HEAD"
    if not head.exists():
        return "unknown", "unknown"
    ref = head.read_text(encoding="utf-8").strip()
    if ref.startswith("ref: "):
        ref_name = ref[5:]
        ref_file = REPO_GIT / ref_name
        commit = ref_file.read_text(encoding="utf-8").strip() if ref_file.exists() else "unknown"
        return ref_name.replace("refs/heads/", ""), commit
    return "detached", ref


def host_fs_metrics():
    stat = os.statvfs(HOST_ROOT)
    size = stat.f_frsize * stat.f_blocks
    free = stat.f_frsize * stat.f_bavail
    used = size - free
    return size, free, used


def collect():
    lines = [
        "# HELP alert24_ops_exporter_up Whether the ops exporter is healthy.",
        "# TYPE alert24_ops_exporter_up gauge",
        "alert24_ops_exporter_up 1",
    ]

    size, free, used = host_fs_metrics()
    lines += [
        "# HELP alert24_ops_filesystem_size_bytes Host root filesystem size.",
        "# TYPE alert24_ops_filesystem_size_bytes gauge",
        f'alert24_ops_filesystem_size_bytes{{mountpoint="/"}} {size}',
        "# HELP alert24_ops_filesystem_free_bytes Host root filesystem free bytes.",
        "# TYPE alert24_ops_filesystem_free_bytes gauge",
        f'alert24_ops_filesystem_free_bytes{{mountpoint="/"}} {free}',
        "# HELP alert24_ops_filesystem_used_bytes Host root filesystem used bytes.",
        "# TYPE alert24_ops_filesystem_used_bytes gauge",
        f'alert24_ops_filesystem_used_bytes{{mountpoint="/"}} {used}',
    ]

    branch, commit = git_commit()
    lines += [
        "# HELP alert24_ops_git_info Current deployed git metadata.",
        "# TYPE alert24_ops_git_info gauge",
        f'alert24_ops_git_info{{branch="{label_value(branch)}",commit="{label_value(commit)}"}} 1',
    ]

    containers = docker_get("/containers/json?all=1")
    lines += [
        "# HELP alert24_ops_container_running Whether a 24alert Docker container is running.",
        "# TYPE alert24_ops_container_running gauge",
        "# HELP alert24_ops_container_restarts Docker restart count by container.",
        "# TYPE alert24_ops_container_restarts gauge",
        "# HELP alert24_ops_container_started_timestamp Container started timestamp.",
        "# TYPE alert24_ops_container_started_timestamp gauge",
        "# HELP alert24_ops_container_info Container image and status metadata.",
        "# TYPE alert24_ops_container_info gauge",
    ]
    for container in containers:
        names = container.get("Names") or []
        name = names[0].lstrip("/") if names else container.get("Id", "")[:12]
        if not name.startswith("24alert-"):
            continue
        cid = container.get("Id", "")
        image = container.get("Image", "")
        state = container.get("State", "")
        status = container.get("Status", "")
        running = 1 if state == "running" else 0
        detail = docker_get(f"/containers/{cid}/json")
        restart_count = detail.get("RestartCount", 0)
        started = parse_time((detail.get("State") or {}).get("StartedAt", ""))
        image_id = detail.get("Image", "")
        labels = (
            f'container="{label_value(name)}",image="{label_value(image)}",'
            f'image_id="{label_value(image_id)}",status="{label_value(status)}"'
        )
        lines.append(f"alert24_ops_container_running{{{labels}}} {running}")
        lines.append(f"alert24_ops_container_restarts{{{labels}}} {restart_count}")
        lines.append(f"alert24_ops_container_started_timestamp{{{labels}}} {started}")
        lines.append(f"alert24_ops_container_info{{{labels}}} 1")

    images = docker_get("/images/json")
    total_size = sum(int(image.get("Size") or 0) for image in images)
    lines += [
        "# HELP alert24_ops_docker_images_total Number of Docker images on host.",
        "# TYPE alert24_ops_docker_images_total gauge",
        f"alert24_ops_docker_images_total {len(images)}",
        "# HELP alert24_ops_docker_images_size_bytes Total Docker image size.",
        "# TYPE alert24_ops_docker_images_size_bytes gauge",
        f"alert24_ops_docker_images_size_bytes {total_size}",
    ]

    return "\n".join(lines) + "\n"


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path not in ("/metrics", "/metrics/"):
            self.send_response(404)
            self.end_headers()
            return
        try:
            body = collect()
            self.send_response(200)
        except Exception as exc:  # noqa: BLE001 - exporter should report scrape error as body.
            body = f"alert24_ops_exporter_up 0\n# error: {label_value(exc)}\n"
            self.send_response(500)
        encoded = body.encode("utf-8")
        self.send_header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, fmt, *args):
        return


if __name__ == "__main__":
    print(f"ops-exporter starting at {datetime.now(timezone.utc).isoformat()}", flush=True)
    HTTPServer(("0.0.0.0", 9140), Handler).serve_forever()
