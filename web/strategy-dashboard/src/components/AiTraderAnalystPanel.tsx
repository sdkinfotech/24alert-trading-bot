import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
import type { AiTraderInstrumentJournal, AiTraderSessionReport } from '../api/types';
import { useI18n } from '../i18n';
import { EM_DASH, formatDateTime, formatNumber } from '../format';
import { Button, Card, EmptyState, Stat } from './ui';

interface Props {
  sessionId: string;
  ticker?: string;
}

export function AiTraderAnalystPanel({ sessionId, ticker }: Props) {
  const { t, lang } = useI18n();
  const [report, setReport] = useState<AiTraderSessionReport | null>(null);
  const [journal, setJournal] = useState<AiTraderInstrumentJournal | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const rep = await api.aiTraderAnalystReport(sessionId).catch(() => null);
      setReport(rep);
      if (ticker) {
        const j = await api.aiTraderInstrumentJournal(ticker).catch(() => null);
        setJournal(j);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [sessionId, ticker]);

  useEffect(() => {
    void load();
  }, [load]);

  const runPostmarket = async () => {
    setLoading(true);
    try {
      const rep = await api.aiTraderRunPostmarket(sessionId);
      setReport(rep);
      if (ticker) {
        setJournal(await api.aiTraderInstrumentJournal(ticker));
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card
      title={t('analystTitle')}
      subtitle={t('analystHelp')}
      actions={
        <Button variant="secondary" disabled={loading} onClick={() => void runPostmarket()}>
          {t('analystRunPostmarket')}
        </Button>
      }
    >
      {error && <p className="text-sm text-[var(--danger)] mb-2">{error}</p>}
      {!report && !loading && <EmptyState>{t('analystNoReport')}</EmptyState>}
      {report && (
        <div className="space-y-3">
          <p className="text-sm text-[var(--text)]">{report.summary_ru}</p>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
            <Stat label={t('analystRounds')} value={String(report.trade_rounds?.length ?? 0)} />
            <Stat
              label={t('analystWinRate')}
              value={report.win_rate != null ? `${formatNumber(report.win_rate * 100, lang, 0)}%` : EM_DASH}
            />
            <Stat
              label={t('realized')}
              value={`${formatNumber(report.realized_rub ?? 0, lang, 2)} ₽`}
            />
            <Stat label={t('analystFrequency')} value={report.frequency?.verdict ?? EM_DASH} />
          </div>
          {report.recommendations && report.recommendations.length > 0 && (
            <ul className="text-xs text-[var(--muted)] list-disc pl-4">
              {report.recommendations.map((r, i) => (
                <li key={i}>{r}</li>
              ))}
            </ul>
          )}
          <p className="text-xs text-[var(--muted)]">
            {t('analystGenerated')}: {formatDateTime(report.generated_at, lang)}
          </p>
        </div>
      )}
      {journal?.stats && (
        <div className="mt-4 pt-3 border-t border-[var(--border)]">
          <h4 className="text-xs font-semibold text-[var(--muted)] mb-2">
            {t('analystInstrumentStats')} {journal.ticker}
          </h4>
          <p className="text-xs text-[var(--muted)]">
            {t('analystSessions')}: {journal.stats.sessions_analyzed} · {t('analystRounds')}:{' '}
            {journal.stats.total_rounds} · WR{' '}
            {formatNumber((journal.stats.win_rate ?? 0) * 100, lang, 0)}%
          </p>
          {journal.hints?.notes && journal.hints.notes.length > 0 && (
            <p className="text-xs mt-1">{journal.hints.notes[0]}</p>
          )}
        </div>
      )}
      <Button variant="secondary" className="mt-2" disabled={loading} onClick={() => void load()}>
        {t('refresh')}
      </Button>
    </Card>
  );
}
