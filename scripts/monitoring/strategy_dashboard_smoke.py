#!/usr/bin/env python3
"""Smoke-check that strategy dashboard tiles follow strategy runner instances."""

from __future__ import annotations

import argparse
import base64
import json
import os
import sys
import time
import urllib.parse
import urllib.request
from pathlib import Path


def get_json(url: str, username: str | None = None, password: str | None = None) -> dict:
    req = urllib.request.Request(url)
    if username and password:
        token = base64.b64encode(f"{username}:{password}".encode()).decode()
        req.add_header("Authorization", f"Basic {token}")
    with urllib.request.urlopen(req, timeout=20) as resp:  # noqa: S310 - operator supplied URL.
        return json.loads(resp.read().decode("utf-8"))


def prom_query(base_url: str, query: str, username: str | None, password: str | None) -> list[dict]:
    encoded = urllib.parse.urlencode({"query": query})
    data = get_json(f"{base_url.rstrip('/')}/api/v1/query?{encoded}", username, password)
    if data.get("status") != "success":
        raise RuntimeError(f"Prometheus query failed: {data}")
    return data["data"]["result"]


def dashboard_json(path: str | None, grafana_url: str | None, username: str | None, password: str | None) -> dict:
    if path:
        return json.loads(Path(path).read_text(encoding="utf-8"))
    if not grafana_url:
        raise ValueError("Either --dashboard-json or --grafana-url is required")
    data = get_json(f"{grafana_url.rstrip('/')}/api/dashboards/uid/24alert-strategy", username, password)
    return data["dashboard"]


def assert_dynamic_dashboard(dashboard: dict) -> list[str]:
    errors: list[str] = []
    variables = {item.get("name"): item for item in dashboard.get("templating", {}).get("list", [])}
    strategy_var = variables.get("strategy_instance")
    if not strategy_var:
        errors.append("missing Grafana variable strategy_instance")
    elif "alert24_strategy_instance_enabled" not in str(strategy_var.get("query", "")):
        errors.append("strategy_instance variable must be sourced from alert24_strategy_instance_enabled")

    repeat_panels = [panel for panel in dashboard.get("panels", []) if panel.get("repeat") == "strategy_instance"]
    if len(repeat_panels) != 1:
        errors.append(f"expected exactly one repeated strategy tile panel, got {len(repeat_panels)}")
    else:
        panel = repeat_panels[0]
        target_text = json.dumps(panel.get("targets", []), ensure_ascii=False)
        if "$strategy_instance" not in target_text:
            errors.append("repeated strategy tile must use $strategy_instance in PromQL")
        for legacy_id in ("fut-brent-mini-lb", "fut-gas-mini-sma", "fut-mechel-lb"):
            if legacy_id in target_text:
                errors.append(f"repeated strategy tile still hardcodes {legacy_id}")
    return errors


def runner_instances(strategy_runner_url: str) -> dict[str, dict]:
    instances = get_json(f"{strategy_runner_url.rstrip('/')}/instances")
    return {item["id"]: item for item in instances}


def prometheus_instances(prometheus_url: str, username: str | None, password: str | None) -> dict[str, dict]:
    enabled = prom_query(prometheus_url, "alert24_strategy_instance_enabled", username, password)
    running = prom_query(prometheus_url, "alert24_strategy_instance_running", username, password)
    result: dict[str, dict] = {}
    for row in enabled:
        instance = row["metric"].get("exported_instance")
        if instance:
            result.setdefault(instance, {})["enabled"] = float(row["value"][1])
    for row in running:
        instance = row["metric"].get("exported_instance")
        if instance:
            result.setdefault(instance, {})["running"] = float(row["value"][1])
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--strategy-runner-url", default=os.getenv("STRATEGY_RUNNER_URL", "http://127.0.0.1:9020"))
    parser.add_argument("--prometheus-url", default=os.getenv("PROMETHEUS_URL"))
    parser.add_argument("--grafana-url", default=os.getenv("GRAFANA_URL"))
    parser.add_argument("--dashboard-json", default=None)
    parser.add_argument("--prometheus-user", default=os.getenv("PROMETHEUS_USER"))
    parser.add_argument("--prometheus-password", default=os.getenv("PROMETHEUS_PASSWORD"))
    parser.add_argument("--grafana-user", default=os.getenv("GRAFANA_USER"))
    parser.add_argument("--grafana-password", default=os.getenv("GRAFANA_PASSWORD"))
    parser.add_argument("--wait-seconds", type=int, default=0)
    parser.add_argument("--skip-runner", action="store_true", help="Only validate dashboard JSON structure")
    args = parser.parse_args()
    if not args.dashboard_json and not args.grafana_url:
        local_dashboard = Path("monitoring/dashboards/24alert-strategy-runner.json")
        if local_dashboard.exists():
            args.dashboard_json = str(local_dashboard)

    if args.wait_seconds:
        time.sleep(args.wait_seconds)

    errors = assert_dynamic_dashboard(
        dashboard_json(args.dashboard_json, args.grafana_url, args.grafana_user, args.grafana_password)
    )

    runner_ids: set[str] = set()
    if args.strategy_runner_url and not args.skip_runner:
        runner = runner_instances(args.strategy_runner_url)
        runner_ids = set(runner)
        stopped = [sid for sid, item in runner.items() if item.get("enabled_in_config") and not item.get("running")]
        if stopped:
            errors.append(f"enabled runner instances not running: {', '.join(sorted(stopped))}")

    prom_ids: set[str] = set()
    if args.prometheus_url:
        prom = prometheus_instances(args.prometheus_url, args.prometheus_user, args.prometheus_password)
        prom_ids = set(prom)
        if runner_ids and runner_ids != prom_ids:
            errors.append(
                "runner/Prometheus instance mismatch: "
                f"runner_only={sorted(runner_ids - prom_ids)} prometheus_only={sorted(prom_ids - runner_ids)}"
            )
        not_ready = [sid for sid, vals in prom.items() if vals.get("enabled") == 1 and vals.get("running") != 1]
        if not_ready:
            errors.append(f"Prometheus reports enabled but not running: {', '.join(sorted(not_ready))}")

    report = {
        "ok": not errors,
        "runner_instances": sorted(runner_ids),
        "prometheus_instances": sorted(prom_ids),
        "errors": errors,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if not errors else 1


if __name__ == "__main__":
    sys.exit(main())
