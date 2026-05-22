#!/usr/bin/env python3
"""Print deployable winners from matrix JSON."""
import argparse
import json
from pathlib import Path


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("matrix_json")
    args = ap.parse_args()
    data = json.loads(Path(args.matrix_json).read_text(encoding="utf-8"))

    for inst in data.get("instruments", []):
        if inst.get("error"):
            continue
        print(f"\n=== {inst['ticker']} ===")
        prod = inst.get("production") or {}
        best = inst.get("best_deployable") or {}
        print(f"PROD:  {prod.get('strategy')} pnl={prod.get('pnl')} trades={prod.get('trades')}")
        print(f"       params={prod.get('params')}")
        print(f"BEST:  {best.get('strategy')}/{best.get('mode')} pnl={best.get('pnl')} "
              f"sharpe={best.get('sharpe')} risk={best.get('risk_score')}")
        print(f"       params={best.get('params')}")
        if best and prod:
            imp = best.get("risk_score", 0) - prod.get("risk_score", 0)
            print(f"DELTA risk_score: {imp:.2f}")


if __name__ == "__main__":
    main()
