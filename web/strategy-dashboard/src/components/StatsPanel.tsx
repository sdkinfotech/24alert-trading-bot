import type { PnlData, LedgerData, DailySummary } from '../api/types';

interface Props {
  pnl: PnlData | null;
  ledger: LedgerData | null;
  daily: DailySummary | null;
}

function StatCard({
  label,
  value,
  color,
  sub,
}: {
  label: string;
  value: string;
  color?: string;
  sub?: string;
}) {
  return (
    <div className="bg-gray-900/60 rounded-lg p-3 border border-gray-800">
      <div className="text-xs text-gray-500 mb-1">{label}</div>
      <div className={`text-lg font-bold ${color ?? 'text-white'}`}>{value}</div>
      {sub && <div className="text-[10px] text-gray-500 mt-0.5">{sub}</div>}
    </div>
  );
}

function pnlColor(v: number): string {
  if (v > 0) return 'text-green-400';
  if (v < 0) return 'text-red-400';
  return 'text-gray-400';
}

function fmtRub(v: number): string {
  const sign = v >= 0 ? '+' : '';
  return `${sign}${v.toFixed(2)} ₽`;
}

export function StatsPanel({ pnl, ledger, daily }: Props) {
  const positions = ledger
    ? Object.entries(ledger.quantities).filter(([, q]) => q !== 0)
    : [];

  return (
    <div>
      <h2 className="text-lg font-semibold mb-3">Stats</h2>
      <div className="grid grid-cols-2 gap-2">
        <StatCard
          label="Total PnL"
          value={pnl ? fmtRub(pnl.total_rub) : '—'}
          color={pnl ? pnlColor(pnl.total_rub) : undefined}
        />
        <StatCard
          label="Realized"
          value={pnl ? fmtRub(pnl.realized_rub) : '—'}
          color={pnl ? pnlColor(pnl.realized_rub) : undefined}
        />
        <StatCard
          label="Unrealized"
          value={pnl ? fmtRub(pnl.unrealized_rub) : '—'}
          color={pnl ? pnlColor(pnl.unrealized_rub) : undefined}
        />
        <StatCard
          label="Positions"
          value={positions.length > 0
            ? positions.map(([uid, q]) =>
                `${q > 0 ? '+' : ''}${q} (${uid.slice(0, 8)}…)`
              ).join(', ')
            : 'flat'
          }
          color={positions.length > 0 ? 'text-blue-400' : 'text-gray-400'}
          sub={positions.length > 0 && ledger
            ? `avg: ${Object.values(ledger.avg_prices).map((p) => `₽${p.toFixed(2)}`).join(', ')}`
            : undefined
          }
        />
      </div>

      {daily && (
        <div className="mt-3 bg-gray-900/40 rounded-lg p-3 border border-gray-800">
          <div className="text-xs text-gray-500 mb-1">Today</div>
          <div className="flex gap-4 text-sm">
            <span>Signals: <b className="text-blue-400">{daily.SignalsCount}</b></span>
            <span>Orders: <b className="text-gray-300">{daily.OrdersCount}</b></span>
            <span>Fills: <b className="text-green-400">{daily.ExecutionsCount}</b></span>
          </div>
        </div>
      )}
    </div>
  );
}
