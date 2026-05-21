import { useCallback, useState } from 'react';
import { api } from '../api/client';
import type { Instance, InstanceStatus, PortfolioSnapshot, TimelineEvent } from '../api/types';
import { useI18n } from '../i18n';
import { formatDateTime, formatMoney, formatNumber } from '../format';
import { Badge, Button, Card } from './ui';

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

type Props = {
  instance: Instance | null;
  status: InstanceStatus | null;
  portfolio: PortfolioSnapshot | null;
  recentEvents: TimelineEvent[];
  onAction: () => void;
  onGoHistory: () => void;
};

export function BotControlBar({ instance, status, portfolio, recentEvents, onAction, onGoHistory }: Props) {
  const { t, lang } = useI18n();
  const [loading, setLoading] = useState<string | null>(null);
  const [message, setMessage] = useState<{ tone: 'success' | 'danger'; text: string } | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showParams, setShowParams] = useState(false);

  const run = useCallback(
    async (key: string, fn: () => Promise<void>) => {
      setLoading(key);
      setMessage(null);
      try {
        await fn();
        setMessage({ tone: 'success', text: t('controlActionOk') });
        onAction();
      } catch (e: unknown) {
        setMessage({ tone: 'danger', text: errorMessage(e) });
      } finally {
        setLoading(null);
      }
    },
    [onAction, t],
  );

  if (!instance) return null;

  const brokerPositions = portfolio?.positions.filter((p) => p.in_instance) ?? [];
  const canStart = instance.enabled_in_config && !instance.running;
  const canStop = instance.running;
  const canFlatten =
    instance.running && (status?.open_position ?? brokerPositions.some((p) => Math.abs(p.quantity) > 1e-9));

  const confirmStop = () => window.confirm(t('controlConfirmStop'));
  const confirmFlatten = () => {
    const lines = brokerPositions.map(
      (p) => `${p.ticker ?? p.instrument_uid.slice(0, 8)}: ${formatNumber(p.quantity, lang)}`,
    );
    return window.confirm(
      `${t('controlConfirmFlatten')}\n${lines.length ? lines.join('\n') : t('flat')}\n${t('account')}: ${instance.account_id}`,
    );
  };
  const confirmReload = () => window.confirm(t('controlConfirmReload'));

  const sessionBadge = status?.trading_window.active ? (
    <Badge tone="success">{t('controlInSession')}</Badge>
  ) : (
    <Badge tone="warning">{t('controlOutOfSession')}</Badge>
  );

  const nextChange = status?.trading_window.next_change_at
    ? formatDateTime(status.trading_window.next_change_at, lang)
    : '—';

  const lastEvents = recentEvents
    .filter((e) => !['broker_position_sync'].includes(e.type))
    .slice(0, 3);

  return (
    <Card
      title={t('controlTitle')}
      subtitle={t('controlSubtitle')}
      className="mb-6"
      actions={
        <div className="flex flex-wrap gap-2">
          <Button
            variant="primary"
            disabled={!canStart || loading !== null}
            title={!instance.enabled_in_config ? t('controlStartDisabledHint') : undefined}
            onClick={() => run('start', () => api.startInstance(instance.id))}
          >
            {loading === 'start' ? '…' : t('controlStart')}
          </Button>
          <Button
            variant="secondary"
            disabled={!canStop || loading !== null}
            onClick={() => {
              if (!confirmStop()) return;
              void run('stop', () => api.stopInstance(instance.id));
            }}
          >
            {loading === 'stop' ? '…' : t('controlStop')}
          </Button>
          <Button
            variant="ghost"
            disabled={!canFlatten || loading !== null}
            className="!text-[var(--danger)]"
            onClick={() => {
              if (!confirmFlatten()) return;
              void run('flatten', () => api.flattenInstance(instance.id).then(() => undefined));
            }}
          >
            {loading === 'flatten' ? '…' : t('controlFlatten')}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <Badge tone={instance.running ? 'success' : 'warning'}>
            {instance.running ? t('running') : t('stopped')}
          </Badge>
          {!instance.enabled_in_config && <Badge tone="neutral">{t('controlDisabledInConfig')}</Badge>}
          {sessionBadge}
          {status?.feed_stale_hint && <Badge tone="danger">{t('controlFeedStale')}</Badge>}
          {status?.ledger_mismatch && <Badge tone="danger">{t('positionSourcesMismatch')}</Badge>}
          {status?.open_position && status.protective_stops === 0 && (
            <Badge tone="danger">{t('protectiveStopMissing')}</Badge>
          )}
        </div>

        <div className="grid gap-3 text-sm md:grid-cols-2 xl:grid-cols-4">
          <div>
            <div className="text-xs text-[var(--muted)]">{t('controlSession')}</div>
            <div>
              {status?.trading_window.active ? t('controlInSession') : t('controlOutOfSession')}
              {status?.trading_window.label ? ` (${status.trading_window.label})` : ''}
            </div>
            <div className="text-xs text-[var(--muted)]">
              {t('controlNextChange')}: {nextChange}
            </div>
          </div>
          <div>
            <div className="text-xs text-[var(--muted)]">{t('controlNextSignal')}</div>
            <div>
              {status?.timeframe ?? instance.params?.interval ?? '—'}
              {status?.next_candle_close_at
                ? ` → ${formatDateTime(status.next_candle_close_at, lang)}`
                : status?.last_closed_candle_at
                  ? ` · ${t('controlLastCandle')}: ${formatDateTime(status.last_closed_candle_at, lang)}`
                  : !status?.indicator_available
                    ? ` · ${t('controlNoIndicator')}`
                    : ''}
            </div>
          </div>
          <div>
            <div className="text-xs text-[var(--muted)]">{t('controlDailyPnl')}</div>
            <div className={(status?.daily_pnl_rub ?? 0) < 0 ? 'text-[var(--danger)]' : 'text-[var(--success)]'}>
              {formatMoney(status?.daily_pnl_rub, lang)}
            </div>
          </div>
          <div>
            <div className="text-xs text-[var(--muted)]">{t('controlPosition')}</div>
            <div>
              {status?.open_position
                ? `${formatNumber(status.broker_position_qty, lang)} · ${t('protectiveStop')}: ${status.protective_stops}`
                : t('flat')}
            </div>
          </div>
        </div>

        {message && (
          <div
            className={
              message.tone === 'danger'
                ? 'rounded-lg border border-[var(--danger)] bg-[var(--danger-soft)] p-2 text-sm text-[var(--danger)]'
                : 'rounded-lg border border-[var(--success)] bg-[var(--success-soft)] p-2 text-sm text-[var(--success)]'
            }
          >
            {message.text}
          </div>
        )}

        {lastEvents.length > 0 && (
          <div className="text-xs">
            <div className="mb-1 font-medium text-[var(--muted)]">{t('controlRecentEvents')}</div>
            <ul className="space-y-1">
              {lastEvents.map((e) => (
                <li key={`${e.type}-${e.time}-${e.instrument_uid ?? ''}`}>
                  <span className="text-[var(--muted)]">{formatDateTime(e.time, lang)}</span>{' '}
                  <span className="font-mono">{e.type}</span>
                  {e.direction ? ` ${e.direction}` : ''}
                  {e.quantity ? ` ×${e.quantity}` : ''}
                </li>
              ))}
            </ul>
            <button type="button" className="mt-1 text-[var(--info)] underline" onClick={onGoHistory}>
              {t('controlAllHistory')}
            </button>
          </div>
        )}

        <div>
          <button
            type="button"
            className="text-xs text-[var(--muted)] underline"
            onClick={() => setShowParams((v) => !v)}
          >
            {showParams ? t('controlHideParams') : t('controlShowParams')}
          </button>
          {showParams && instance.params && Object.keys(instance.params).length > 0 && (
            <dl className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1 text-xs md:grid-cols-4">
              {Object.entries(instance.params).map(([k, v]) => (
                <div key={k}>
                  <dt className="text-[var(--muted)]">{k}</dt>
                  <dd className="font-mono">{v}</dd>
                </div>
              ))}
            </dl>
          )}
        </div>

        <div>
          <button
            type="button"
            className="text-xs text-[var(--muted)] underline"
            onClick={() => setShowAdvanced((v) => !v)}
          >
            {showAdvanced ? t('controlHideAdvanced') : t('controlShowAdvanced')}
          </button>
          {showAdvanced && (
            <div className="mt-2 space-y-2">
              <p className="text-xs text-[var(--muted)]">{t('controlReloadHint')}</p>
              <Button
                variant="secondary"
                disabled={loading !== null}
                onClick={() => {
                  if (!confirmReload()) return;
                  void run('reload', async () => {
                    const res = await api.reloadConfig();
                    const parts = [
                      res.added?.length ? `+${res.added.join(', ')}` : '',
                      res.removed?.length ? `-${res.removed.join(', ')}` : '',
                      res.changed?.length ? `~${res.changed.join(', ')}` : '',
                    ].filter(Boolean);
                    setMessage({
                      tone: 'success',
                      text: parts.length ? parts.join(' · ') : t('controlReloadEmpty'),
                    });
                    onAction();
                  });
                }}
              >
                {loading === 'reload' ? '…' : t('controlReload')}
              </Button>
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}
