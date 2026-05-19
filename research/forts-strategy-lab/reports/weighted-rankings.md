# Weighted Strategy Rankings

Profile: `balanced`

Weights:

- `pnl`: 0.25
- `sharpe`: 0.22
- `profit_factor`: 0.18
- `trades`: 0.14
- `drawdown`: 0.11
- `liquidity`: 0.1

## Best By Instrument

| Rank | Ticker | Family | Strategy | Params | Score | PnL | Trades | Sharpe | PF | DD | Volume15m | Action |
|---:|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| 1 | `SIM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 97.0 | 8666.0 | 30 | 7.8853 | 4.1674 | 1020.0 | 15464.81 | `ADD_CANDIDATE` |
| 2 | `EUM6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 93.84 | 7226.0 | 24 | 6.2119 | 2.9116 | 894.0 | 1564.39 | `ADD_CANDIDATE` |
| 3 | `SIU6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 93.11 | 6672.0 | 23 | 6.3757 | 3.6175 | 882.0 | 868.85 | `ADD_CANDIDATE` |
| 4 | `PXM6` | `trend_following` | `sma_1h` | `fast=5, slow=17` | 92.03 | 6458.0 | 20 | 8.6067 | 6.9302 | 356.0 | 187.95 | `ADD_CANDIDATE` |
| 5 | `MGM6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 91.82 | 6999.0 | 32 | 5.8909 | 3.7287 | 602.0 | 90.75 | `ADD_CANDIDATE` |
| 6 | `SIZ6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 90.7 | 8779.0 | 22 | 7.1506 | 4.2648 | 1220.0 | 30.49 | `ADD_CANDIDATE` |
| 7 | `MXM6` | `trend_following` | `sma_1h` | `fast=9, slow=26` | 88.48 | 14100.0 | 13 | 5.1077 | 2.757 | 3775.0 | 1514.83 | `ADD_CANDIDATE` |
| 8 | `MCM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 88.23 | 1748.0 | 33 | 4.5997 | 3.1035 | 366.0 | 130.05 | `ADD_CANDIDATE` |
| 9 | `BTM6` | `trend_following` | `sma_1h` | `fast=5, slow=17` | 87.57 | 6886.0 | 13 | 6.6209 | 3.1937 | 1163.0 | 541.4 | `ADD_CANDIDATE` |
| 10 | `NAM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 86.9 | 3768.0 | 35 | 4.61 | 2.4219 | 1170.0 | 3742.61 | `ADD_CANDIDATE` |
| 11 | `TTM6` | `trend_following` | `sma_1h` | `fast=7, slow=17` | 86.71 | 12091.0 | 13 | 6.9552 | 3.9635 | 2127.0 | 38.04 | `ADD_CANDIDATE` |
| 12 | `BTK6` | `breakout_momentum` | `orb_15m` | `range_minutes=30, rr=2.0` | 86.62 | 2927.0 | 14 | 7.9035 | 3.0512 | 612.0 | 1546.34 | `ADD_CANDIDATE` |
| 13 | `RIM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=1.0, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 85.15 | 5493.9286 | 10 | 8.6082 | 5.2921 | 1250.0 | 671.14 | `ADD_CANDIDATE` |
| 14 | `RNM6` | `trend_following` | `sma_1h` | `fast=5, slow=17` | 84.58 | 4924.0 | 20 | 4.7484 | 2.338 | 2006.0 | 108.02 | `ADD_CANDIDATE` |
| 15 | `BTN6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 84.45 | 4839.0 | 12 | 8.5806 | 4.5871 | 649.0 | 62.1 | `ADD_CANDIDATE` |
| 16 | `PXU6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 84.16 | 5094.0 | 14 | 6.6327 | 5.1448 | 471.0 | 12.3 | `ADD_CANDIDATE` |
| 17 | `SRM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 82.85 | 1191.7714 | 11 | 8.1473 | 4.44 | 194.7214 | 1225.11 | `ADD_CANDIDATE` |
| 18 | `BMM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 82.31 | 51.9 | 38 | 6.2824 | 3.395 | 8.44 | 8954.08 | `ADD_CANDIDATE` |
| 19 | `CHM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 81.87 | 5514.15 | 9 | 9.627 | 5.8837 | 608.05 | 23.02 | `ADD_CANDIDATE` |
| 20 | `N2M6` | `breakout_momentum` | `orb_15m` | `range_minutes=60, rr=1.0` | 81.26 | 1423.0 | 12 | 8.8155 | 3.5053 | 327.0 | 108.75 | `ADD_CANDIDATE` |
| 21 | `SGU6` | `trend_following` | `sma_1h` | `fast=7, slow=17` | 80.83 | 5730.0 | 10 | 9.7895 | 5.6359 | 743.0 | 4.99 | `ADD_CANDIDATE` |
| 22 | `NAU6` | `trend_following` | `ema_1h` | `fast=12, slow=27` | 80.69 | 3032.0 | 9 | 5.9684 | 4.5839 | 769.0 | 103.73 | `ADD_CANDIDATE` |
| 23 | `TTU6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 80.19 | 9726.0 | 8 | 7.07 | 4.5163 | 1580.0 | 4.01 | `ADD_CANDIDATE` |
| 24 | `MXU6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.5, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 80.01 | 10947.3214 | 5 | 11.1044 | 7.7368 | 1625.0 | 15.26 | `ADD_CANDIDATE` |
| 25 | `MTM6` | `trend_following` | `sma_1h` | `fast=9, slow=26` | 80.01 | 1536.0 | 11 | 7.375 | 3.5729 | 311.0 | 37.77 | `ADD_CANDIDATE` |
| 26 | `NAH7` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 79.86 | 2697.0 | 12 | 7.3663 | 3.7325 | 826.0 | 8.82 | `ADD_CANDIDATE` |
| 27 | `MGU6` | `trend_following` | `ema_1h` | `fast=5, slow=17` | 79.71 | 3764.0 | 9 | 5.6602 | 4.2989 | 527.0 | 9.93 | `ADD_CANDIDATE` |
| 28 | `SPM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.5, tp_mult=1.0, cutoff_hour=23, cutoff_min=30` | 79.41 | 995.4286 | 11 | 8.5175 | 3.9689 | 182.2857 | 36.61 | `ADD_CANDIDATE` |
| 29 | `RNU6` | `trend_following` | `sma_1h` | `fast=9, slow=26` | 79.08 | 5803.0 | 8 | 5.9116 | 3.6437 | 1715.0 | 6.75 | `ADD_CANDIDATE` |
| 30 | `SRU6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 78.78 | 1442.2857 | 8 | 17.117 | 30.4344 | 49.0 | 43.16 | `ADD_CANDIDATE` |

## Best By Instrument And Family

| Rank | Ticker | Family | Strategy | Params | Score | PnL | Trades | Sharpe | PF | DD | Volume15m | Action |
|---:|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| 1 | `SIM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 97.0 | 8666.0 | 30 | 7.8853 | 4.1674 | 1020.0 | 15464.81 | `ADD_CANDIDATE` |
| 2 | `EUM6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 93.84 | 7226.0 | 24 | 6.2119 | 2.9116 | 894.0 | 1564.39 | `ADD_CANDIDATE` |
| 3 | `SIU6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 93.11 | 6672.0 | 23 | 6.3757 | 3.6175 | 882.0 | 868.85 | `ADD_CANDIDATE` |
| 4 | `PXM6` | `trend_following` | `sma_1h` | `fast=5, slow=17` | 92.03 | 6458.0 | 20 | 8.6067 | 6.9302 | 356.0 | 187.95 | `ADD_CANDIDATE` |
| 5 | `MGM6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 91.82 | 6999.0 | 32 | 5.8909 | 3.7287 | 602.0 | 90.75 | `ADD_CANDIDATE` |
| 6 | `SIZ6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 90.7 | 8779.0 | 22 | 7.1506 | 4.2648 | 1220.0 | 30.49 | `ADD_CANDIDATE` |
| 7 | `MXM6` | `trend_following` | `sma_1h` | `fast=9, slow=26` | 88.48 | 14100.0 | 13 | 5.1077 | 2.757 | 3775.0 | 1514.83 | `ADD_CANDIDATE` |
| 8 | `MCM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 88.23 | 1748.0 | 33 | 4.5997 | 3.1035 | 366.0 | 130.05 | `ADD_CANDIDATE` |
| 9 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=2.0, cutoff_hour=23, cutoff_min=30` | 88.11 | 14085.0 | 9 | 9.2068 | 6.9287 | 1250.3571 | 1514.83 | `ADD_CANDIDATE` |
| 10 | `BTM6` | `trend_following` | `sma_1h` | `fast=5, slow=17` | 87.57 | 6886.0 | 13 | 6.6209 | 3.1937 | 1163.0 | 541.4 | `ADD_CANDIDATE` |
| 11 | `NAM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 86.9 | 3768.0 | 35 | 4.61 | 2.4219 | 1170.0 | 3742.61 | `ADD_CANDIDATE` |
| 12 | `TTM6` | `trend_following` | `sma_1h` | `fast=7, slow=17` | 86.71 | 12091.0 | 13 | 6.9552 | 3.9635 | 2127.0 | 38.04 | `ADD_CANDIDATE` |
| 13 | `BTK6` | `breakout_momentum` | `orb_15m` | `range_minutes=30, rr=2.0` | 86.62 | 2927.0 | 14 | 7.9035 | 3.0512 | 612.0 | 1546.34 | `ADD_CANDIDATE` |
| 14 | `BTM6` | `breakout_momentum` | `orb_15m` | `range_minutes=60, rr=2.0` | 85.41 | 3681.0 | 12 | 9.1163 | 3.4721 | 548.0 | 541.4 | `ADD_CANDIDATE` |
| 15 | `RIM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=1.0, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 85.15 | 5493.9286 | 10 | 8.6082 | 5.2921 | 1250.0 | 671.14 | `ADD_CANDIDATE` |
| 16 | `RNM6` | `trend_following` | `sma_1h` | `fast=5, slow=17` | 84.58 | 4924.0 | 20 | 4.7484 | 2.338 | 2006.0 | 108.02 | `ADD_CANDIDATE` |
| 17 | `BTN6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 84.45 | 4839.0 | 12 | 8.5806 | 4.5871 | 649.0 | 62.1 | `ADD_CANDIDATE` |
| 18 | `PXU6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 84.16 | 5094.0 | 14 | 6.6327 | 5.1448 | 471.0 | 12.3 | `ADD_CANDIDATE` |
| 19 | `SRM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 82.85 | 1191.7714 | 11 | 8.1473 | 4.44 | 194.7214 | 1225.11 | `ADD_CANDIDATE` |
| 20 | `BMM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 82.31 | 51.9 | 38 | 6.2824 | 3.395 | 8.44 | 8954.08 | `ADD_CANDIDATE` |
| 21 | `BTN6` | `breakout_momentum` | `orb_15m` | `range_minutes=60, rr=1.0` | 81.98 | 2076.0 | 13 | 7.8962 | 3.2085 | 678.0 | 62.1 | `ADD_CANDIDATE` |
| 22 | `CHM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 81.87 | 5514.15 | 9 | 9.627 | 5.8837 | 608.05 | 23.02 | `ADD_CANDIDATE` |
| 23 | `N2M6` | `breakout_momentum` | `orb_15m` | `range_minutes=60, rr=1.0` | 81.26 | 1423.0 | 12 | 8.8155 | 3.5053 | 327.0 | 108.75 | `ADD_CANDIDATE` |
| 24 | `SGU6` | `trend_following` | `sma_1h` | `fast=7, slow=17` | 80.83 | 5730.0 | 10 | 9.7895 | 5.6359 | 743.0 | 4.99 | `ADD_CANDIDATE` |
| 25 | `NAU6` | `trend_following` | `ema_1h` | `fast=12, slow=27` | 80.69 | 3032.0 | 9 | 5.9684 | 4.5839 | 769.0 | 103.73 | `ADD_CANDIDATE` |
| 26 | `MGM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.5, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 80.34 | 1838.0 | 8 | 11.7267 | 26.1781 | 73.0 | 90.75 | `ADD_CANDIDATE` |
| 27 | `TTU6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 80.19 | 9726.0 | 8 | 7.07 | 4.5163 | 1580.0 | 4.01 | `ADD_CANDIDATE` |
| 28 | `MXU6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.5, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 80.01 | 10947.3214 | 5 | 11.1044 | 7.7368 | 1625.0 | 15.26 | `ADD_CANDIDATE` |
| 29 | `MTM6` | `trend_following` | `sma_1h` | `fast=9, slow=26` | 80.01 | 1536.0 | 11 | 7.375 | 3.5729 | 311.0 | 37.77 | `ADD_CANDIDATE` |
| 30 | `NAH7` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 79.86 | 2697.0 | 12 | 7.3663 | 3.7325 | 826.0 | 8.82 | `ADD_CANDIDATE` |
| 31 | `MGU6` | `trend_following` | `ema_1h` | `fast=5, slow=17` | 79.71 | 3764.0 | 9 | 5.6602 | 4.2989 | 527.0 | 9.93 | `ADD_CANDIDATE` |
| 32 | `SPM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.5, tp_mult=1.0, cutoff_hour=23, cutoff_min=30` | 79.41 | 995.4286 | 11 | 8.5175 | 3.9689 | 182.2857 | 36.61 | `ADD_CANDIDATE` |
| 33 | `RNU6` | `trend_following` | `sma_1h` | `fast=9, slow=26` | 79.08 | 5803.0 | 8 | 5.9116 | 3.6437 | 1715.0 | 6.75 | `ADD_CANDIDATE` |
| 34 | `SRU6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 78.78 | 1442.2857 | 8 | 17.117 | 30.4344 | 49.0 | 43.16 | `ADD_CANDIDATE` |
| 35 | `LKM6` | `trend_following` | `sma_1h` | `fast=12, slow=27` | 78.74 | 4415.0 | 12 | 4.6395 | 2.6066 | 2699.0 | 183.45 | `ADD_CANDIDATE` |
| 36 | `TTM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.5, tp_mult=1.0, cutoff_hour=23, cutoff_min=30` | 78.63 | 2350.0714 | 8 | 9.3159 | 4.7839 | 621.0714 | 38.04 | `ADD_CANDIDATE` |
| 37 | `W4G7` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 78.19 | 2450.0 | 10 | 5.1412 | 5.0164 | 560.0 | 5.38 | `ADD_CANDIDATE` |
| 38 | `EUU6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.0, cutoff_hour=23, cutoff_min=30` | 78.05 | 1975.6857 | 5 | 10.7953 | 7.4801 | 304.8857 | 154.7 | `ADD_CANDIDATE` |
| 39 | `N2M6` | `trend_following` | `ema_1h` | `fast=9, slow=26` | 77.85 | 6070.0 | 12 | 4.2957 | 2.539 | 2681.0 | 108.75 | `ADD_CANDIDATE` |
| 40 | `PXU6` | `breakout_momentum` | `orb_15m` | `range_minutes=60, rr=1.5` | 77.62 | 1444.5 | 9 | 9.4244 | 11.2447 | 106.0 | 12.3 | `ADD_CANDIDATE` |
| 41 | `SIH7` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 77.25 | 5396.0 | 20 | 4.4183 | 2.2066 | 3659.0 | 4.3 | `ADD_CANDIDATE` |
| 42 | `IRM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 77.21 | 1062.9143 | 10 | 6.5767 | 2.9419 | 261.9143 | 15.22 | `ADD_CANDIDATE` |
| 43 | `NAU6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.7, tp_mult=1.0, cutoff_hour=23, cutoff_min=30` | 76.82 | 976.1429 | 7 | 8.6014 | 3.7191 | 185.0 | 103.73 | `ADD_CANDIDATE` |
| 44 | `MGU6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=2.0, cutoff_hour=23, cutoff_min=30` | 76.75 | 2277.6857 | 7 | 7.9943 | 4.0719 | 443.9714 | 9.93 | `ADD_CANDIDATE` |
| 45 | `MXZ6` | `trend_following` | `ema_1h` | `fast=12, slow=27` | 76.39 | 9475.0 | 5 | 6.8347 | 2.9239 | 3275.0 | 3.23 | `ADD_CANDIDATE` |
| 46 | `MXU6` | `trend_following` | `sma_1h` | `fast=9, slow=26` | 76.22 | 13650.0 | 16 | 4.0621 | 1.9803 | 6475.0 | 15.26 | `ADD_CANDIDATE` |
| 47 | `NAM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.7, tp_mult=1.0, cutoff_hour=23, cutoff_min=30` | 76.16 | 583.6429 | 6 | 6.3063 | 2.758 | 184.0 | 3742.61 | `ADD_CANDIDATE` |
| 48 | `PXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.5, tp_mult=2.0, cutoff_hour=23, cutoff_min=30` | 76.03 | 1142.2857 | 12 | 4.6288 | 2.6693 | 574.2857 | 187.95 | `ADD_CANDIDATE` |
| 49 | `NAH7` | `breakout_momentum` | `orb_15m` | `range_minutes=60, rr=1.5` | 75.88 | 1268.5 | 9 | 6.4072 | 3.4583 | 389.0 | 8.82 | `ADD_CANDIDATE` |
| 50 | `LKU6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=2.0, cutoff_hour=23, cutoff_min=30` | 75.8 | 2266.0 | 5 | 11.955 | 13.1828 | 186.0 | 5.41 | `ADD_CANDIDATE` |

## Top Overall

| Rank | Ticker | Family | Strategy | Params | Score | PnL | Trades | Sharpe | PF | DD | Volume15m | Action |
|---:|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| 1 | `SIM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 97.0 | 8666.0 | 30 | 7.8853 | 4.1674 | 1020.0 | 15464.81 | `ADD_CANDIDATE` |
| 2 | `SIM6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 96.62 | 7410.0 | 27 | 6.4075 | 3.3412 | 847.0 | 15464.81 | `ADD_CANDIDATE` |
| 3 | `EUM6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 93.84 | 7226.0 | 24 | 6.2119 | 2.9116 | 894.0 | 1564.39 | `ADD_CANDIDATE` |
| 4 | `SIU6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 93.11 | 6672.0 | 23 | 6.3757 | 3.6175 | 882.0 | 868.85 | `ADD_CANDIDATE` |
| 5 | `SIM6` | `trend_following` | `ema_1h` | `fast=7, slow=17` | 92.2 | 5757.0 | 15 | 6.6679 | 3.5173 | 966.0 | 15464.81 | `ADD_CANDIDATE` |
| 6 | `PXM6` | `trend_following` | `sma_1h` | `fast=5, slow=17` | 92.03 | 6458.0 | 20 | 8.6067 | 6.9302 | 356.0 | 187.95 | `ADD_CANDIDATE` |
| 7 | `MGM6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 91.82 | 6999.0 | 32 | 5.8909 | 3.7287 | 602.0 | 90.75 | `ADD_CANDIDATE` |
| 8 | `MGM6` | `trend_following` | `ema_1h` | `fast=5, slow=17` | 91.4 | 6663.0 | 22 | 6.1992 | 3.521 | 935.0 | 90.75 | `ADD_CANDIDATE` |
| 9 | `PXM6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 91.37 | 5924.0 | 23 | 6.7077 | 5.268 | 800.0 | 187.95 | `ADD_CANDIDATE` |
| 10 | `PXM6` | `trend_following` | `ema_1h` | `fast=5, slow=17` | 91.26 | 4973.0 | 22 | 5.8591 | 4.3511 | 376.0 | 187.95 | `ADD_CANDIDATE` |
| 11 | `SIU6` | `trend_following` | `ema_1h` | `fast=5, slow=17` | 90.78 | 6120.0 | 17 | 7.0005 | 3.9941 | 825.0 | 868.85 | `ADD_CANDIDATE` |
| 12 | `SIZ6` | `trend_following` | `ema_1h` | `fast=4, slow=9` | 90.7 | 8779.0 | 22 | 7.1506 | 4.2648 | 1220.0 | 30.49 | `ADD_CANDIDATE` |
| 13 | `SIM6` | `trend_following` | `ema_1h` | `fast=5, slow=17` | 89.07 | 5606.0 | 15 | 4.5446 | 2.9364 | 1631.0 | 15464.81 | `ADD_CANDIDATE` |
| 14 | `MXM6` | `trend_following` | `sma_1h` | `fast=9, slow=26` | 88.48 | 14100.0 | 13 | 5.1077 | 2.757 | 3775.0 | 1514.83 | `ADD_CANDIDATE` |
| 15 | `MCM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 88.23 | 1748.0 | 33 | 4.5997 | 3.1035 | 366.0 | 130.05 | `ADD_CANDIDATE` |
| 16 | `PXM6` | `trend_following` | `sma_1h` | `fast=7, slow=17` | 88.15 | 5648.0 | 15 | 9.0728 | 7.6919 | 357.0 | 187.95 | `ADD_CANDIDATE` |
| 17 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=2.0, cutoff_hour=23, cutoff_min=30` | 88.11 | 14085.0 | 9 | 9.2068 | 6.9287 | 1250.3571 | 1514.83 | `ADD_CANDIDATE` |
| 18 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 87.87 | 13002.8571 | 9 | 9.4523 | 6.4732 | 1250.3571 | 1514.83 | `ADD_CANDIDATE` |
| 19 | `BTM6` | `trend_following` | `sma_1h` | `fast=5, slow=17` | 87.57 | 6886.0 | 13 | 6.6209 | 3.1937 | 1163.0 | 541.4 | `ADD_CANDIDATE` |
| 20 | `MGM6` | `trend_following` | `ema_1h` | `fast=12, slow=27` | 87.39 | 7239.0 | 14 | 7.0105 | 5.1603 | 1036.0 | 90.75 | `ADD_CANDIDATE` |
| 21 | `MGM6` | `trend_following` | `ema_1h` | `fast=7, slow=17` | 87.34 | 4484.0 | 22 | 4.7037 | 2.8257 | 1221.0 | 90.75 | `ADD_CANDIDATE` |
| 22 | `PXM6` | `trend_following` | `ema_1h` | `fast=9, slow=26` | 87.11 | 4356.0 | 15 | 6.7146 | 3.904 | 589.0 | 187.95 | `ADD_CANDIDATE` |
| 23 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.3, tp_mult=1.0, cutoff_hour=23, cutoff_min=30` | 87.07 | 10027.8571 | 9 | 9.3978 | 5.221 | 1250.3571 | 1514.83 | `ADD_CANDIDATE` |
| 24 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.5, tp_mult=2.0, cutoff_hour=23, cutoff_min=30` | 87.03 | 14610.7143 | 7 | 12.3723 | 21.8724 | 700.0 | 1514.83 | `ADD_CANDIDATE` |
| 25 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.7, tp_mult=2.0, cutoff_hour=23, cutoff_min=30` | 87.03 | 14610.7143 | 7 | 12.3723 | 21.8724 | 700.0 | 1514.83 | `ADD_CANDIDATE` |
| 26 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=1.0, tp_mult=2.0, cutoff_hour=23, cutoff_min=30` | 87.03 | 14610.7143 | 7 | 12.3723 | 21.8724 | 700.0 | 1514.83 | `ADD_CANDIDATE` |
| 27 | `NAM6` | `trend_following` | `sma_1h` | `fast=4, slow=9` | 86.9 | 3768.0 | 35 | 4.61 | 2.4219 | 1170.0 | 3742.61 | `ADD_CANDIDATE` |
| 28 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.5, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 86.82 | 13528.5714 | 7 | 13.0872 | 20.3265 | 700.0 | 1514.83 | `ADD_CANDIDATE` |
| 29 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=0.7, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 86.82 | 13528.5714 | 7 | 13.0872 | 20.3265 | 700.0 | 1514.83 | `ADD_CANDIDATE` |
| 30 | `MXM6` | `level_reversal` | `level_bounce_15m` | `sl_mult=1.0, tp_mult=1.5, cutoff_hour=23, cutoff_min=30` | 86.82 | 13528.5714 | 7 | 13.0872 | 20.3265 | 700.0 | 1514.83 | `ADD_CANDIDATE` |
