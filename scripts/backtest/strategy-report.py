#!/usr/bin/env python3
"""Generate Russian markdown report from strategy-matrix JSON."""
import argparse
import json
import sys
from pathlib import Path


def fmt_params(p: dict) -> str:
    if not p:
        return "—"
    parts = []
    for k, v in sorted(p.items()):
        if k == "interval":
            continue
        if k == "trailing_stop_pct":
            parts.append(f"{k}={v * 100:.2f}%")
        elif isinstance(v, float) and k.endswith("_mult"):
            parts.append(f"{k}={v}")
        elif isinstance(v, float) and v < 1:
            parts.append(f"{k}={v:.4f}")
        else:
            parts.append(f"{k}={v}")
    return ", ".join(parts)[:80]


def row_line(label: str, r: dict | None, prod_ok: str = "") -> str:
    if not r:
        return f"| {label} | — | — | — | — | — | {prod_ok} |"
    prod = "да" if r.get("live_eligible") else f"нет ({r.get('live_block_reason', '')[:30]})"
    return (
        f"| {label} | {r.get('strategy', '')} / {r.get('mode', '')} | {fmt_params(r.get('params', {}))} | "
        f"{r.get('pnl', 0):.2f} | {r.get('trades', 0)} | {r.get('max_drawdown', 0):.2f} | {prod} |"
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("matrix_json", help="Path to matrix JSON")
    ap.add_argument("--out", default="", help="Markdown output path")
    args = ap.parse_args()

    data = json.loads(Path(args.matrix_json).read_text(encoding="utf-8"))
    lines = [
        "# Strategy Lab — отчёт (BMM6 / NGM6 / MCM6)",
        "",
        f"Дата: {data.get('generated_at', '')}",
        f"История: ~{data.get('days', 90)} дней. **PnL в пунктах цены фьючерса**, не рубли на счёте.",
        "",
        "Расписание: FORTS пн–пт, 10:00–14:00, 14:05–18:50, 19:00–23:50 MSK.",
        "",
    ]

    for inst in data.get("instruments", []):
        if inst.get("error"):
            lines.append(f"## {inst.get('ticker')} — ошибка: {inst['error']}")
            continue
        ticker = inst["ticker"]
        lines.extend([
            f"## {ticker}",
            "",
            "| | Стратегия | Параметры | PnL | Сделок | Просадка | На прод? |",
            "|---|-----------|-----------|-----|--------|----------|----------|",
            row_line("**Сейчас на прод**", inst.get("production")),
            row_line("**Лучше (можно на прод)**", inst.get("best_deployable")),
            row_line("**Лучше в тесте (нельзя)**", inst.get("best_research")),
            "",
            "### Топ-5 deployable",
            "",
            "| Стратегия | PnL | Sharpe | PF | Сделок | Параметры |",
            "|-----------|-----|--------|-----|--------|-----------|",
        ])
        for r in inst.get("top10_deployable", [])[:5]:
            lines.append(
                f"| {r.get('strategy')}/{r.get('mode')} | {r.get('pnl')} | {r.get('sharpe')} | "
                f"{r.get('profit_factor')} | {r.get('trades')} | {fmt_params(r.get('params', {}))} |"
            )
        lines.append("")

    text = "\n".join(lines)
    out_path = args.out or str(Path(args.matrix_json).with_suffix(".md"))
    Path(out_path).write_text(text, encoding="utf-8")
    print(text)
    print(f"\nWrote {out_path}", file=sys.stderr)


if __name__ == "__main__":
    main()
