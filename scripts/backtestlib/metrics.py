"""PnL, risk score, quality labels."""


def calc_pnl(trades: list) -> dict:
    entry = None
    pnl = 0.0
    wins = losses = 0
    max_dd = peak = equity = 0.0
    pnls = []

    for t in trades:
        if entry is None:
            entry = t
        elif t["dir"] != entry["dir"]:
            if entry["dir"] == "buy":
                p = t["price"] - entry["price"]
            else:
                p = entry["price"] - t["price"]
            pnl += p
            pnls.append(p)
            equity += p
            peak = max(peak, equity)
            max_dd = max(max_dd, peak - equity)
            if p > 0:
                wins += 1
            else:
                losses += 1
            entry = t if "reverse" in t.get("reason", "") else None
        else:
            entry = t

    total = wins + losses
    win_rate = wins / total if total else 0.0
    gross_profit = sum(p for p in pnls if p > 0)
    gross_loss = abs(sum(p for p in pnls if p <= 0))
    if gross_loss > 0:
        profit_factor = gross_profit / gross_loss
    elif gross_profit > 0:
        profit_factor = float("inf")
    else:
        profit_factor = 0.0

    if len(pnls) > 1:
        mean = sum(pnls) / len(pnls)
        var = sum((p - mean) ** 2 for p in pnls) / (len(pnls) - 1)
        std = var ** 0.5
        sharpe = (mean / std) * (252 ** 0.5) if std > 0 else 0.0
    else:
        sharpe = 0.0

    pf_out = round(profit_factor, 2) if profit_factor != float("inf") else 999.0
    return {
        "pnl": round(pnl, 4),
        "trades": total,
        "wins": wins,
        "losses": losses,
        "win_rate": round(win_rate, 4),
        "max_drawdown": round(max_dd, 4),
        "sharpe": round(sharpe, 4),
        "profit_factor": pf_out,
    }


def risk_score(stats: dict) -> float:
    if stats["trades"] < 5 or stats["pnl"] <= 0:
        return -1e9
    if stats["sharpe"] < 1.0 or stats["win_rate"] < 0.40:
        pf = stats["profit_factor"]
        if pf < 1.2 and pf < 999:
            return -1e6
    dd_ratio = stats["max_drawdown"] / max(abs(stats["pnl"]), 1e-6)
    pf = min(stats["profit_factor"], 20.0) if stats["profit_factor"] < 999 else 20.0
    return stats["sharpe"] * pf - 0.35 * dd_ratio


def quality_label(stats: dict) -> str:
    if stats["trades"] < 5:
        return "LOW_SAMPLE"
    if stats["pnl"] > 0 and stats["sharpe"] > 1 and stats["profit_factor"] > 1.3:
        return "PASS"
    if stats["pnl"] > 0:
        return "WATCH"
    return "REJECT"


def enrich_run(strategy: str, params: dict, trades: list, *,
               live_eligible: bool, live_block_reason: str = "",
               mode: str = "", interval: str = "1h") -> dict:
    stats = calc_pnl(trades)
    return {
        "strategy": strategy,
        "mode": mode or strategy,
        "interval": interval,
        "params": params,
        "live_eligible": live_eligible,
        "live_block_reason": live_block_reason,
        **stats,
        "risk_score": round(risk_score(stats), 4),
        "quality": quality_label(stats),
    }
