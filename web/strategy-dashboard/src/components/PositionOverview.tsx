import type {
  IndicatorData,
  Instance,
  LedgerData,
  PortfolioPosition,
  PortfolioSnapshot,
  StopOrder,
} from '../api/types';
import { formatMoney, formatNumber } from '../format';
import { useI18n } from '../i18n';
import { Badge, Card, LabelWithHelp } from './ui';

interface Props {
  instance: Instance | null;
  indicator: IndicatorData | null;
  ledger: LedgerData | null;
  portfolio: PortfolioSnapshot | null;
  stopOrders?: StopOrder[];
}

function strategyPositionLabel(pos?: number): string {
  if (pos === 1) return 'LONG';
  if (pos === -1) return 'SHORT';
  return 'FLAT';
}

function strategyPositionColor(pos?: number): string {
  if (pos === 1) return 'text-green-400';
  if (pos === -1) return 'text-red-400';
  return 'text-gray-400';
}

function ledgerQuantity(ledger: LedgerData | null, uid: string): number {
  return ledger?.quantities?.[uid] ?? 0;
}

function ledgerAvg(ledger: LedgerData | null, uid: string): number {
  return ledger?.avg_prices?.[uid] ?? 0;
}

function hasQtyMismatch(p: PortfolioPosition, ledger: LedgerData | null): boolean {
  return Math.abs(p.quantity - ledgerQuantity(ledger, p.instrument_uid)) > 1e-6;
}

function brokerSyncTime(portfolio: PortfolioSnapshot | null): string {
  if (!portfolio?.last_broker_sync) return '—';
  return new Date(portfolio.last_broker_sync).toLocaleTimeString('ru-RU');
}

export function PositionOverview({ instance, indicator, ledger, portfolio, stopOrders = [] }: Props) {
  const { t, lang } = useI18n();
  const brokerPositions = portfolio?.positions ?? [];
  const instancePositions = brokerPositions.filter((p) => p.in_instance);
  const foreignPositions = brokerPositions.filter((p) => !p.in_instance);
  const classicEnabled = instance?.enabled_in_config !== false;
  const strategyPos = indicator?.position ?? 0;
  const strategyFlatButBrokerOpen = strategyPos === 0 && instancePositions.length > 0;
  const mismatches = classicEnabled ? instancePositions.filter((p) => hasQtyMismatch(p, ledger)) : [];
  const unprotectedPositions = classicEnabled
    ? instancePositions.filter(
        (p) => p.quantity !== 0 && !stopOrders.some((s) => s.instrument_uid === p.instrument_uid),
      )
    : [];
  const showClassicDanger =
    classicEnabled && (strategyFlatButBrokerOpen || mismatches.length > 0 || unprotectedPositions.length > 0);
  const showAiTraderHint = !classicEnabled && brokerPositions.some((p) => Math.abs(p.quantity) > 1e-6);

  return (
    <Card>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between mb-3">
        <div>
          <h2 className="text-lg font-semibold">{t('positions')}</h2>
          <p className="text-xs text-[var(--muted)]">
            <LabelWithHelp label={t('brokerTruth')} help={t('brokerTruthHelp')} /> ·{' '}
            <LabelWithHelp label={t('runnerLedger')} help={t('runnerLedgerHelp')} /> ·{' '}
            <LabelWithHelp label={t('strategyState')} help={t('strategyStateHelp')} />
          </p>
        </div>
        <div className="text-xs text-[var(--muted)] sm:text-right">
          <div>{t('account')}: <span className="font-mono text-[var(--text)]">{portfolio?.account_id ?? instance?.account_id ?? '—'}</span></div>
          <div>Broker sync: <span className="text-[var(--text)]">{brokerSyncTime(portfolio)}</span></div>
        </div>
      </div>

      {portfolio?.portfolio_error && (
        <div className="mb-3 rounded-lg border border-[var(--warning)] bg-[var(--warning-soft)] p-3 text-sm text-[var(--warning)]">
          Broker portfolio unavailable: {portfolio.portfolio_error}
        </div>
      )}

      {showAiTraderHint && (
        <div className="mb-3 rounded-lg border border-[var(--info)] bg-[var(--info-soft)] p-3 text-sm text-[var(--info)]">
          {t('classicOffBrokerPositionHint')}
        </div>
      )}

      {showClassicDanger && (
        <div className="mb-3 rounded-lg border border-[var(--danger)] bg-[var(--danger-soft)] p-3 text-sm text-[var(--danger)]">
          {t('positionSourcesMismatch')}
          {unprotectedPositions.length > 0
            ? ` ${t('protectiveStopMissing')}: ${unprotectedPositions.map((p) => p.ticker || p.instrument_uid.slice(0, 8)).join(', ')}.`
            : ''}
        </div>
      )}

      {instance && !classicEnabled && (
        <div className="mb-3">
          <Badge tone="neutral">{t('classicStrategyDisabled')}</Badge>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-4">
        <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3">
          <div className="text-xs text-[var(--muted)] mb-1">{t('brokerTruth')}</div>
          <div className={instancePositions.length > 0 ? 'text-[var(--info)] font-semibold' : 'text-[var(--muted)]'}>
            {instancePositions.length > 0 ? `${instancePositions.length} open` : 'flat'}
          </div>
        </div>
        <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3">
          <div className="text-xs text-[var(--muted)] mb-1">{t('strategyState')}</div>
          <div className={`font-semibold ${strategyPositionColor(strategyPos)}`}>
            {strategyPositionLabel(strategyPos)}
          </div>
          {indicator?.trailing_stop_pct && indicator.trailing_stop_pct > 0 ? (
            <div className="text-[10px] text-orange-300 mt-0.5">
              trail {(indicator.trailing_stop_pct * 100).toFixed(2)}%
              {indicator.trailing_stop_active && indicator.trailing_stop_price
                ? ` @ ${indicator.trailing_stop_price.toFixed(4)}`
                : ' waiting'}
            </div>
          ) : null}
        </div>
        <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3">
          <div className="text-xs text-[var(--muted)] mb-1">{t('protectiveStop')}</div>
          <div className={unprotectedPositions.length > 0 ? 'text-[var(--danger)] font-semibold' : 'text-[var(--success)] font-semibold'}>
            {stopOrders.length} broker stops
          </div>
          <div className={portfolio && portfolio.expected_yield >= 0 ? 'text-green-400 font-semibold' : 'text-red-400 font-semibold'}>
            {t('expectedYield')}: {portfolio ? formatMoney(portfolio.expected_yield, lang, 'RUB') : '—'}
          </div>
        </div>
      </div>

      <div className="overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-xs uppercase text-gray-500">
            <tr className="border-b border-gray-800">
              <th className="py-2 pr-3 text-left">Instrument</th>
              <th className="py-2 pr-3 text-right">Broker qty</th>
              <th className="py-2 pr-3 text-right">Broker avg</th>
              <th className="py-2 pr-3 text-right">Current</th>
              <th className="py-2 pr-3 text-right">Yield</th>
              <th className="py-2 pr-3 text-right">Runner qty</th>
              <th className="py-2 pr-3 text-right">Runner avg</th>
              <th className="py-2 text-left">Status</th>
            </tr>
          </thead>
          <tbody>
            {instancePositions.length === 0 && (
              <tr>
                <td colSpan={8} className="py-4 text-center text-gray-500">
                  По инструментам выбранной стратегии broker positions нет.
                </td>
              </tr>
            )}
            {instancePositions.map((p) => {
              const qtyMismatch = hasQtyMismatch(p, ledger);
              const currency = p.currency || 'RUB';
              return (
                <tr key={p.instrument_uid} className="border-b border-gray-900/80">
                  <td className="py-2 pr-3">
                    <div className="font-mono text-gray-200">{p.ticker || p.instrument_uid.slice(0, 8)}</div>
                    <div className="text-[10px] text-gray-600">{p.instrument_uid}</div>
                  </td>
                  <td className="py-2 pr-3 text-right font-mono text-blue-300">{formatNumber(p.quantity, lang, 4)}</td>
                  <td className="py-2 pr-3 text-right font-mono">{formatMoney(p.average_price, lang, currency)}</td>
                  <td className="py-2 pr-3 text-right font-mono">{formatMoney(p.current_price, lang, currency)}</td>
                  <td className={`py-2 pr-3 text-right font-mono ${p.expected_yield >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {formatMoney(p.expected_yield, lang, currency)}
                  </td>
                  <td className={`py-2 pr-3 text-right font-mono ${qtyMismatch ? 'text-amber-300' : 'text-gray-300'}`}>
                    {formatNumber(ledgerQuantity(ledger, p.instrument_uid), lang, 4)}
                  </td>
                  <td className="py-2 pr-3 text-right font-mono">{formatMoney(ledgerAvg(ledger, p.instrument_uid), lang, currency)}</td>
                  <td className="py-2 text-left">
                    {qtyMismatch ? (
                      <Badge tone="warning">ledger mismatch</Badge>
                    ) : (
                      <Badge tone="success">aligned</Badge>
                    )}
                    {stopOrders.some((s) => s.instrument_uid === p.instrument_uid) ? <span className="ml-1"><Badge tone="info">protected</Badge></span> : <span className="ml-1"><Badge tone="danger">no broker stop</Badge></span>}
                    {p.blocked && <span className="ml-1"><Badge tone="danger">blocked</Badge></span>}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {foreignPositions.length > 0 && (
        <p className="mt-3 text-xs text-gray-500">
          На этом же счёте есть позиции вне выбранной стратегии: {foreignPositions.map((p) => p.ticker || p.instrument_uid.slice(0, 8)).join(', ')}.
        </p>
      )}
    </Card>
  );
}
