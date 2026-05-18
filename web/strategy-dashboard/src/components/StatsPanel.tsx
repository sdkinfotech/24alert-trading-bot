import type { PnlData, LedgerData, PortfolioSnapshot, DailySummary, OrderRecord, ExecutionRecord, StopOrder } from '../api/types';
import { useI18n } from '../i18n';
import { formatMoney } from '../format';
import { Card, Stat } from './ui';

interface Props {
  pnl: PnlData | null;
  ledger: LedgerData | null;
  portfolio: PortfolioSnapshot | null;
  daily: DailySummary | null;
  orders?: OrderRecord[];
  executions?: ExecutionRecord[];
  stopOrders?: StopOrder[];
}

export function StatsPanel({ pnl, ledger, portfolio, daily, orders = [], executions = [], stopOrders = [] }: Props) {
  const { t, lang } = useI18n();
  const positions = ledger
    ? Object.entries(ledger.quantities).filter(([, q]) => q !== 0)
    : [];
  const brokerPositions = portfolio?.positions.filter((p) => p.in_instance) ?? [];

  return (
    <Card title="Stats">
      <div className="grid grid-cols-1 gap-3">
        <Stat label={t('totalPnl')} value={formatMoney(pnl?.total_rub, lang)} tone={(pnl?.total_rub ?? 0) < 0 ? 'danger' : 'success'} sub={pnl?.source} />
        <Stat label={t('realized')} value={formatMoney(pnl?.realized_rub, lang)} tone={(pnl?.realized_rub ?? 0) < 0 ? 'danger' : 'success'} />
        <Stat label={t('unrealized')} value={formatMoney(pnl?.unrealized_rub, lang)} tone={(pnl?.unrealized_rub ?? 0) < 0 ? 'danger' : 'success'} />
        <Stat label={t('runnerLedger')} help={t('runnerLedgerHelp')} value={positions.length > 0 ? positions.map(([uid, q]) => `${q > 0 ? '+' : ''}${q} (${uid.slice(0, 8)}…)`).join(', ') : t('flat')} tone={positions.length > 0 ? 'info' : 'neutral'} />
        <Stat label={t('brokerTruth')} help={t('brokerTruthHelp')} value={brokerPositions.length > 0 ? brokerPositions.map((p) => `${p.quantity > 0 ? '+' : ''}${p.quantity} (${p.ticker || `${p.instrument_uid.slice(0, 8)}…`})`).join(', ') : portfolio?.portfolio_error ? t('unavailable') : t('flat')} tone={brokerPositions.length > 0 ? 'info' : portfolio?.portfolio_error ? 'warning' : 'neutral'} sub={portfolio?.portfolio_error} />
      </div>

      {daily && (
        <div className="mt-4 rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3">
          <div className="text-xs text-[var(--muted)] mb-1">Today</div>
          <div className="flex flex-wrap gap-3 text-sm">
            <span>{t('signals')}: <b className="text-[var(--info)]">{daily.SignalsCount}</b></span>
            <span>{t('orders')}: <b>{orders.length || daily.OrdersCount}</b></span>
            <span>{t('executions')}: <b className="text-[var(--success)]">{executions.length || daily.ExecutionsCount}</b></span>
            <span>{t('stopOrders')}: <b className="text-[var(--warning)]">{stopOrders.length}</b></span>
          </div>
        </div>
      )}
    </Card>
  );
}
