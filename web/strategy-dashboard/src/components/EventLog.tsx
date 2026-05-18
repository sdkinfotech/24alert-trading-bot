import { useState } from 'react';
import type { TimelineEvent } from '../api/types';
import { useI18n } from '../i18n';
import { formatDateTime } from '../format';
import { Badge, EmptyState } from './ui';

interface Props {
  events: TimelineEvent[];
}

const TYPE_STYLES: Record<string, string> = {
  signal: 'bg-blue-900/50 border-blue-500',
  signal_cancelled: 'bg-amber-950/40 border-amber-500',
  order: 'bg-gray-800/50 border-gray-500',
  execution: 'bg-emerald-900/30 border-emerald-500',
};

const TYPE_LABELS: Record<string, string> = {
  signal: 'Signal',
  signal_cancelled: 'Signal cancelled',
  order: 'Order',
  execution: 'Execution',
};

function statusColor(status?: string): string {
  if (!status) return 'text-gray-400';
  const s = status.toLowerCase();
  if (s === 'filled') return 'text-green-400';
  if (s === 'partially_filled') return 'text-yellow-400';
  if (s === 'cancelled' || s === 'rejected') return 'text-red-400';
  if (s.includes('blocked') || s.includes('rejected') || s.includes('error')) return 'text-amber-400';
  if (s === 'submitted' || s === 'new') return 'text-gray-300';
  return 'text-gray-400';
}

function dirColor(dir?: string): string {
  if (dir === 'buy') return 'text-green-400';
  if (dir === 'sell') return 'text-red-400';
  return 'text-gray-400';
}

export function EventLog({ events }: Props) {
  const { t, lang } = useI18n();
  const [filter, setFilter] = useState<string>('all');

  const filtered = filter === 'all'
    ? events
    : events.filter((e) => e.type === filter);

  return (
    <div>
      <div className="flex items-center gap-2 mb-1">
        <h2 className="text-lg font-semibold">{t('events')}</h2>
        <div className="flex gap-1 ml-auto">
          {['all', 'signal', 'signal_cancelled', 'order', 'execution'].map((t) => (
            <button
              key={t}
              onClick={() => setFilter(t)}
              className={`ui-button px-2 py-1 text-xs ${filter === t ? 'ui-button-primary' : 'ui-button-ghost'}`}
            >
              {t === 'all' ? 'All' : TYPE_LABELS[t] ?? t}
            </button>
          ))}
        </div>
      </div>
      <p className="text-xs text-[var(--muted)] mb-3">
        Время — метка свечи биржи (UTC), в колонке показывается в часовом поясе браузера. Совпадает с осью графика.
      </p>
      <div className="space-y-1 max-h-[500px] overflow-y-auto pr-1 scrollbar-thin">
        {filtered.length === 0 && (
          <EmptyState>{t('noEvents')}</EmptyState>
        )}
        {filtered.map((e, i) => (
          <div
            key={`${e.type}-${e.time}-${i}`}
            className={`border-l-2 pl-3 py-1.5 rounded-r ${TYPE_STYLES[e.type] ?? 'border-gray-600 bg-gray-800/30'}`}
          >
            <div className="flex items-center gap-2 text-xs">
              <span className="text-[var(--muted)] font-mono">{formatDateTime(e.time, lang)}</span>
              <Badge tone={e.type === 'signal_cancelled' ? 'warning' : e.type === 'execution' ? 'success' : 'info'}>{TYPE_LABELS[e.type] ?? e.type}</Badge>
              {e.direction && (
                <span className={`font-semibold ${dirColor(e.direction)}`}>
                  {e.direction.toUpperCase()}
                </span>
              )}
              {e.status && e.type !== 'order' && (
                <span className={`${statusColor(e.status)}`}>{e.status}</span>
              )}
            </div>
            <div className="text-xs text-[var(--muted)] mt-0.5">
              {(e.type === 'signal' || e.type === 'signal_cancelled') && (
                <>
                  {e.reason} &middot; qty {e.quantity} &middot; ref ₽{e.ref_price?.toFixed(2)}
                  {e.message && <span className="text-gray-500"> &middot; {e.message}</span>}
                </>
              )}
              {e.type === 'order' && (
                <>
                  {e.order_id?.slice(0, 12)}… &middot; {e.order_type} &middot; qty {e.quantity} &middot; ref ₽{e.ref_price?.toFixed(2)}
                </>
              )}
              {e.type === 'execution' && (
                <>
                  {e.order_id?.slice(0, 12)}… &middot; filled {e.filled_qty} @ ₽{e.avg_price?.toFixed(2)}
                  {e.message && <span className="text-gray-500"> &middot; {e.message}</span>}
                </>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
