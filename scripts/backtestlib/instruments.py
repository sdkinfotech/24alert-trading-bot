"""Production futures for Strategy Lab."""

CORE_INSTRUMENTS = [
    {
        "id": "fut-brent-mini-lb",
        "ticker": "BMM6",
        "uid": "dc1ffa30-70a4-4a7b-807a-4f31c2951f7e",
        "swing_bars": 5,
        "prod": {
            "type": "sma_crossover",
            "interval": "1h",
            "fast_period": 4,
            "slow_period": 9,
            "trailing_stop_pct": 0.008,
            "initial_stop_swing_bars": 5,
        },
    },
    {
        "id": "fut-gas-mini-sma",
        "ticker": "NGM6",
        "uid": "117a1408-431f-4ba0-a041-5bba3123d4a8",
        "swing_bars": 0,
        "prod": {
            "type": "sma_crossover",
            "interval": "1h",
            "fast_period": 5,
            "slow_period": 17,
            "trailing_stop_pct": 0.005,
            "initial_stop_swing_bars": 0,
        },
    },
    {
        "id": "fut-mechel-lb",
        "ticker": "MCM6",
        "uid": "6f4563c0-e853-46f2-98c7-3abce3cc7517",
        "swing_bars": 0,
        "prod": {
            "type": "sma_crossover",
            "interval": "1h",
            "fast_period": 4,
            "slow_period": 9,
            "trailing_stop_pct": 0.005,
            "initial_stop_swing_bars": 0,
        },
    },
]

FIXED_PERIOD_PAIRS = [(4, 9), (5, 17), (7, 17), (8, 19), (9, 26), (12, 27)]
