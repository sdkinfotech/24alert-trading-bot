#!/usr/bin/env python3
import argparse
import json
import math
import os
from collections import defaultdict
from datetime import datetime, timezone


PROFILES = {
    "balanced": {
        "pnl": 0.25,
        "sharpe": 0.22,
        "profit_factor": 0.18,
        "trades": 0.14,
        "drawdown": 0.11,
        "liquidity": 0.10,
    },
    "conservative": {
        "pnl": 0.15,
        "sharpe": 0.25,
        "profit_factor": 0.22,
        "trades": 0.12,
        "drawdown": 0.16,
        "liquidity": 0.10,
    },
    "aggressive": {
        "pnl": 0.38,
        "sharpe": 0.16,
        "profit_factor": 0.12,
        "trades": 0.12,
        "drawdown": 0.08,
        "liquidity": 0.14,
    },
}


def clamp(value, low=0.0, high=1.0):
    return max(low, min(high, value))


def norm_positive(value, max_value):
    if max_value <= 0 or value <= 0:
        return 0.0
    return clamp(math.log1p(value) / math.log1p(max_value))


def score_row(row, max_pnl, max_volume, profile):
    pnl_score = norm_positive(row.get("pnl", 0), max_pnl)
    sharpe_score = clamp(row.get("sharpe", 0) / 5.0)
    pf_score = clamp(row.get("profit_factor", 0) / 3.0)
    trades_score = clamp(row.get("trades", 0) / 20.0)
    pnl = max(abs(row.get("pnl", 0)), 1e-9)
    dd_ratio = max(row.get("max_drawdown", 0), 0) / pnl if row.get("pnl", 0) > 0 else 9
    drawdown_score = clamp(1.0 - min(dd_ratio, 2.0) / 2.0)
    volume_score = norm_positive(max(row.get("avg_volume_15m", 0), row.get("avg_volume_1h", 0)), max_volume)
    if row.get("is_core"):
        volume_score = max(volume_score, 0.75)

    raw = (
        pnl_score * profile["pnl"]
        + sharpe_score * profile["sharpe"]
        + pf_score * profile["profit_factor"]
        + trades_score * profile["trades"]
        + drawdown_score * profile["drawdown"]
        + volume_score * profile["liquidity"]
    )
    penalties = 0.0
    if row.get("trades", 0) < 5:
        penalties += 0.30
    if row.get("pnl", 0) <= 0:
        penalties += 0.40
    if row.get("profit_factor", 0) < 1.2:
        penalties += 0.12
    if dd_ratio > 1.0:
        penalties += 0.10

    return {
        "weighted_score": round(clamp(raw - penalties) * 100, 2),
        "components": {
            "pnl": round(pnl_score, 4),
            "sharpe": round(sharpe_score, 4),
            "profit_factor": round(pf_score, 4),
            "trades": round(trades_score, 4),
            "drawdown": round(drawdown_score, 4),
            "liquidity": round(volume_score, 4),
            "penalties": round(penalties, 4),
        },
    }


def best_by(rows, key):
    grouped = defaultdict(list)
    for row in rows:
        grouped[key(row)].append(row)
    out = []
    for _, group in grouped.items():
        out.append(max(group, key=lambda r: (r["weighted_score"], r.get("pnl", 0), r.get("trades", 0))))
    return sorted(out, key=lambda r: r["weighted_score"], reverse=True)


def markdown_table(rows, limit=20):
    lines = [
        "| Rank | Ticker | Family | Strategy | Params | Score | PnL | Trades | Sharpe | PF | DD | Volume15m | Action |",
        "|---:|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    for idx, row in enumerate(rows[:limit], start=1):
        params = ", ".join(f"{k}={v}" for k, v in row.get("params", {}).items())
        action = row.get("action", "")
        lines.append(
            f"| {idx} | `{row['ticker']}` | `{row.get('family', '')}` | `{row['strategy']}` | "
            f"`{params}` | {row['weighted_score']} | {row.get('pnl', 0)} | {row.get('trades', 0)} | "
            f"{row.get('sharpe', 0)} | {row.get('profit_factor', 0)} | {row.get('max_drawdown', 0)} | "
            f"{row.get('avg_volume_15m', 0)} | `{action}` |"
        )
    return "\n".join(lines)


def decide_action(row):
    if row.get("trades", 0) < 5:
        return "LOW_SAMPLE"
    if row.get("pnl", 0) <= 0:
        return "REJECT"
    if row["weighted_score"] >= 70 and row.get("profit_factor", 0) >= 1.5 and row.get("sharpe", 0) >= 2:
        return "ADD_CANDIDATE"
    if row["weighted_score"] >= 55:
        return "WATCH"
    return "REJECT"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--profile", choices=sorted(PROFILES), default="balanced")
    args = parser.parse_args()

    with open(args.input, encoding="utf-8") as fh:
        data = json.load(fh)

    rows = [row for row in data.get("results", []) if row.get("trades", 0) >= 1]
    max_pnl = max([row.get("pnl", 0) for row in rows if row.get("pnl", 0) > 0] or [1])
    max_volume = max([max(row.get("avg_volume_15m", 0), row.get("avg_volume_1h", 0)) for row in rows] or [1])
    profile = PROFILES[args.profile]

    scored = []
    for row in rows:
        enriched = dict(row)
        enriched.update(score_row(row, max_pnl, max_volume, profile))
        enriched["action"] = decide_action(enriched)
        scored.append(enriched)

    scored.sort(key=lambda r: (r["weighted_score"], r.get("pnl", 0), r.get("trades", 0)), reverse=True)
    by_instrument = best_by(scored, lambda r: r["ticker"])
    by_family = best_by(scored, lambda r: (r["ticker"], r.get("family", "")))
    family_summary = []
    for family in sorted({row.get("family", "") for row in scored}):
        family_rows = [row for row in scored if row.get("family") == family]
        pass_rows = [row for row in family_rows if row.get("action") in {"ADD_CANDIDATE", "WATCH"}]
        family_summary.append({
            "family": family,
            "runs": len(family_rows),
            "watch_or_better": len(pass_rows),
            "avg_score": round(sum(row["weighted_score"] for row in family_rows) / len(family_rows), 2) if family_rows else 0,
            "best": family_rows[0] if family_rows else None,
        })

    os.makedirs(args.output_dir, exist_ok=True)
    output = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "profile": args.profile,
        "weights": profile,
        "max_pnl": max_pnl,
        "max_volume": max_volume,
        "ranked": scored,
        "best_by_instrument": by_instrument,
        "best_by_family": by_family,
        "family_summary": family_summary,
    }
    json_path = os.path.join(args.output_dir, "weighted-rankings.json")
    md_path = os.path.join(args.output_dir, "weighted-rankings.md")
    with open(json_path, "w", encoding="utf-8") as fh:
        json.dump(output, fh, ensure_ascii=False, indent=2, default=str)
    with open(md_path, "w", encoding="utf-8") as fh:
        fh.write(f"# Weighted Strategy Rankings\n\nProfile: `{args.profile}`\n\n")
        fh.write("Weights:\n\n")
        for key, value in profile.items():
            fh.write(f"- `{key}`: {value}\n")
        fh.write("\n## Best By Instrument\n\n")
        fh.write(markdown_table(by_instrument, limit=30))
        fh.write("\n\n## Best By Instrument And Family\n\n")
        fh.write(markdown_table(by_family, limit=50))
        fh.write("\n\n## Top Overall\n\n")
        fh.write(markdown_table(scored, limit=30))
        fh.write("\n")
    print(json.dumps({"profile": args.profile, "ranked": len(scored), "top": scored[:10]}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
