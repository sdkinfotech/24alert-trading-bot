"""Parameter grids and matrix evaluation."""
from backtestlib.instruments import FIXED_PERIOD_PAIRS
from backtestlib.metrics import enrich_run
from backtestlib.strategies import (
    run_sma, run_level_bounce, run_ema, run_donchian, run_orb,
)


def period_pairs():
    """Fixed prod pairs + coarse grid (3 offsets per fast)."""
    pairs = set(FIXED_PERIOD_PAIRS)
    for fast in range(3, 13):
        for off in (4, 9, 17):
            pairs.add((fast, fast + off))
    return sorted(pairs)


TRAIL_GRID_FULL = [x / 1000.0 for x in range(3, 16)]
TRAIL_GRID_COARSE = [0.003, 0.005, 0.008, 0.010, 0.012, 0.015]


def run_matrix_for_ticker(inst: dict, c1h: list, c15: list, cdaily: list) -> list:
    ticker = inst["ticker"]
    swing_default = inst.get("swing_bars", 0)
    runs = []

    # Production baseline
    p = inst["prod"]
    trades = run_sma(
        c1h, p["fast_period"], p["slow_period"],
        p.get("trailing_stop_pct", 0), p.get("initial_stop_swing_bars", 0),
    )
    runs.append(enrich_run(
        "sma_crossover", dict(p), trades,
        live_eligible=True, mode="prod_baseline", interval="1h",
    ))

    pairs = period_pairs()
    sl_grid = [(sl / 1000.0, tp / 1000.0)
               for sl in range(5, 21, 3) for tp in range(10, 41, 5)]

    for fast, slow in pairs:
        is_fixed = (fast, slow) in FIXED_PERIOD_PAIRS
        trail_grid = TRAIL_GRID_FULL if is_fixed else TRAIL_GRID_COARSE

        trades = run_sma(c1h, fast, slow, 0, swing_default)
        runs.append(enrich_run(
            "sma_crossover",
            {"fast_period": fast, "slow_period": slow,
             "trailing_stop_pct": 0, "initial_stop_swing_bars": swing_default},
            trades, live_eligible=False,
            live_block_reason="trailing_stop_pct required for live SMA",
            mode="sma_no_trail", interval="1h",
        ))

        swings = [0, swing_default] if swing_default else [0]
        for trail in trail_grid:
            for swing in swings:
                trades = run_sma(c1h, fast, slow, trail, swing)
                runs.append(enrich_run(
                    "sma_crossover",
                    {"fast_period": fast, "slow_period": slow,
                     "trailing_stop_pct": trail,
                     "initial_stop_swing_bars": swing},
                    trades,
                    live_eligible=trail > 0,
                    live_block_reason="" if trail > 0 else "trailing required",
                    mode="sma_trailing", interval="1h",
                ))

        if is_fixed:
            for sl, tp in sl_grid:
                trades = run_sma(c1h, fast, slow, 0, swing_default, sl, tp)
                runs.append(enrich_run(
                    "sma_crossover",
                    {"fast_period": fast, "slow_period": slow,
                     "stop_loss_pct": sl, "take_profit_pct": tp,
                     "initial_stop_swing_bars": swing_default},
                    trades, live_eligible=False,
                    live_block_reason="fixed SL/TP not in Go SMA runner",
                    mode="sma_sl_tp", interval="1h",
                ))

    for fast, slow in FIXED_PERIOD_PAIRS:
        trades = run_ema(c1h, fast, slow)
        runs.append(enrich_run(
            "ema_1h",
            {"fast_period": fast, "slow_period": slow},
            trades, live_eligible=False,
            live_block_reason="ema not implemented in Go runner",
            mode="ema_cross", interval="1h",
        ))

    # Level bounce 15m
    if len(c15) >= 50 and len(cdaily) >= 10:
        for sl in [0.3, 0.5, 0.7, 1.0]:
            for tp in [1.0, 1.5, 2.0]:
                for ch, cm in [(17, 30), (18, 30), (23, 30)]:
                    trades = run_level_bounce(c15, cdaily, 0.0, sl, tp, ch, cm)
                    runs.append(enrich_run(
                        "level_bounce",
                        {"sl_mult": sl, "tp_mult": tp,
                         "cutoff_hour": ch, "cutoff_min": cm, "level_days": 10},
                        trades, live_eligible=True, mode="level_bounce", interval="15min",
                    ))

    # Donchian 15m
    if len(c15) >= 50:
        for lookback in [8, 12, 16, 20]:
            for atr_stop in [0.8, 1.0, 1.5]:
                trades = run_donchian(c15, lookback, atr_stop)
                runs.append(enrich_run(
                    "donchian_15m",
                    {"lookback": lookback, "atr_stop": atr_stop},
                    trades, live_eligible=False,
                    live_block_reason="donchian not in Go runner",
                    mode="donchian", interval="15min",
                ))

    # ORB 15m (Go-aligned, blocked live)
    if len(c15) >= 50:
        for rc in [1, 2, 4]:
            for ch, cm in [(18, 30), (23, 30)]:
                trades = run_orb(c15, rc, ch, cm)
                runs.append(enrich_run(
                    "orb_breakout",
                    {"range_candles": rc, "cutoff_hour": ch, "cutoff_min": cm,
                     "interval": "15min"},
                    trades, live_eligible=False,
                    live_block_reason="orb_breakout blocked for live until protective stop",
                    mode="orb_breakout", interval="15min",
                ))

    for r in runs:
        r["ticker"] = ticker
        r["instance_id"] = inst["id"]
    return runs


def dedupe_best(runs: list) -> list:
    by_key = {}
    for r in runs:
        key = (r["strategy"], r["mode"], str(r["params"]))
        if key not in by_key or r["risk_score"] > by_key[key]["risk_score"]:
            by_key[key] = r
    return sorted(by_key.values(), key=lambda x: x["risk_score"], reverse=True)
