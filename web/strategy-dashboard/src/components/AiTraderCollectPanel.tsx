import type { AiTraderCollectEvent, AiTraderSession } from '../api/types';
import { useI18n } from '../i18n';
import { formatDateTime, formatNumber } from '../format';
import { Card } from './ui';

interface Props {
  session: AiTraderSession;
}

function feedKindClass(kind: string): string {
  switch (kind) {
    case 'book':
      return 'ai-trader-feed-book';
    case 'print':
      return 'ai-trader-feed-print';
    case 'levels':
      return 'ai-trader-feed-levels';
    case 'phase':
    case 'advisor':
    case 'playbook':
      return 'ai-trader-feed-phase';
    default:
      return 'ai-trader-feed-default';
  }
}

function FeedLine({ ev, lang }: { ev: AiTraderCollectEvent; lang: 'ru' | 'en' }) {
  return (
    <li className={`ai-trader-feed-line ${feedKindClass(ev.kind)}`}>
      <time className="ai-trader-feed-time">{formatDateTime(ev.time, lang)}</time>
      <span className="ai-trader-feed-kind">{ev.kind}</span>
      <p className="ai-trader-feed-msg">{ev.message}</p>
      {ev.detail && <p className="ai-trader-feed-detail">{ev.detail}</p>}
    </li>
  );
}

export function AiTraderCollectPanel({ session }: Props) {
  const { t, lang } = useI18n();
  const stats = session.phase_progress?.buffer_stats;
  const feed = session.collect_feed ?? [];
  const prints = session.market_context?.recent_prints?.slice(-8) ?? [];

  return (
    <Card title={t('collectLiveTitle')} subtitle={t('collectLiveHelp')}>
      {stats && (
        <div className="ai-trader-buffer-stats">
          <span>{t('bufferBook')}: {stats.book_samples}</span>
          <span>{t('bufferPrints')}: {stats.print_samples}</span>
          <span>{t('bufferChart')}: {stats.chart_bars}</span>
          <span>{t('bufferLevels')}: {stats.level_count} (D{stats.daily_levels}/H{stats.hourly_levels})</span>
          {stats.mid != null && stats.mid > 0 && (
            <span>{t('bufferMid')}: {formatNumber(stats.mid, lang, 4)}</span>
          )}
          {stats.last_price != null && stats.last_price > 0 && (
            <span>{t('bufferLast')}: {formatNumber(stats.last_price, lang, 4)}</span>
          )}
        </div>
      )}
      {feed.length > 0 ? (
        <ul className="ai-trader-feed-list">
          {feed.map((ev, i) => (
            <FeedLine key={`${ev.time}-${ev.kind}-${i}`} ev={ev} lang={lang} />
          ))}
        </ul>
      ) : (
        <p className="text-sm text-[var(--muted)]">{t('collectFeedEmpty')}</p>
      )}
      {prints.length > 0 && (
        <div className="mt-3">
          <p className="text-xs font-semibold text-[var(--muted)] mb-1">{t('recentPrints')}</p>
          <ul className="text-xs font-mono space-y-0.5">
            {prints.map((p, i) => (
              <li key={`${p.time}-${i}`}>
                {formatDateTime(p.time, lang)} {p.direction} {p.quantity}@{formatNumber(p.price, lang, 4)}
              </li>
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}
