import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type {
  AdvisorAnalysisReport,
  AdvisorStrategyResponse,
  AdvisorTimeframe,
} from '../api/types';
import { useI18n } from '../i18n';
import { formatDateTime } from '../format';
import { Badge, Button, Card, EmptyState, Tabs } from './ui';

const TF_TABS: Array<{ id: AdvisorTimeframe; labelKey: string }> = [
  { id: '5m', labelKey: 'tf5m' },
  { id: '15m', labelKey: 'tf15m' },
  { id: '30m', labelKey: 'tf30m' },
  { id: '1h', labelKey: 'tf1h' },
  { id: '4h', labelKey: 'tf4h' },
  { id: '1d', labelKey: 'tfDay' },
  { id: 'strategy', labelKey: 'tfStrategy' },
];

interface Props {
  sessionId: string;
}

function Section({ title, items }: { title: string; items?: string[] }) {
  if (!items?.length) return null;
  return (
    <section className="advisor-section">
      <h4 className="advisor-section-title">{title}</h4>
      <ul className="advisor-bullets">
        {items.map((item, i) => (
          <li key={`${title}-${i}`}>{item}</li>
        ))}
      </ul>
    </section>
  );
}

function ReportBody({ report, t }: { report: AdvisorAnalysisReport; t: (k: string) => string }) {
  const s = report.structured;
  return (
    <div className="advisor-report-body">
      {report.status === 'failed' && (
        <p className="text-sm text-[var(--danger)]">{report.error_message ?? t('advisorReportFailed')}</p>
      )}
      {report.model && (
        <p className="text-xs text-[var(--muted)]">
          model: <Badge tone="neutral">{report.model}</Badge>
        </p>
      )}
      {report.summary_md && (
        <div className="advisor-summary-md whitespace-pre-wrap">{report.summary_md}</div>
      )}
      {s && (
        <>
          {s.market_regime && (
            <p className="text-sm text-[var(--muted)]">
              {t('advisorRegime')}: <Badge tone="info">{s.market_regime}</Badge>
            </p>
          )}
          <Section title={t('advisorParticipants')} items={s.participants?.map((p) => `${p.role}: ${p.notes}`)} />
          <Section title={t('advisorVolumes')} items={s.volume_notes} />
          <Section
            title={t('advisorLimits')}
            items={s.large_limits?.map((l) => `${l.side} ${l.price} × ${l.quantity} (${l.event})`)}
          />
          <Section title={t('advisorClouds')} items={s.mm_clouds} />
          <Section
            title={t('advisorDensities')}
            items={s.densities?.map((d) => `${d.side} @ ${d.price}: ${d.assessment} — ${d.reason}`)}
          />
          <Section title={t('advisorIcebergs')} items={s.iceberg_hints} />
          <Section title={t('advisorConclusions')} items={s.conclusions} />
          <Section title={t('nextWatch')} items={s.next_watch} />
        </>
      )}
    </div>
  );
}

export function AiTraderAnalysisPanel({ sessionId }: Props) {
  const { t, lang } = useI18n();
  const [tf, setTf] = useState<AdvisorTimeframe>('5m');
  const [reports, setReports] = useState<AdvisorAnalysisReport[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [strategy, setStrategy] = useState<AdvisorStrategyResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [finalizing, setFinalizing] = useState(false);

  const load = useCallback(async () => {
    if (!sessionId) return;
    try {
      if (tf === 'strategy') {
        const syn = await api.advisorStrategy(sessionId);
        setStrategy(syn);
        setReports(syn.reports ?? []);
        setError(null);
        return;
      }
      const list = await api.advisorAnalyses(sessionId, tf, 20);
      setStrategy(null);
      setReports(list);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [sessionId, tf]);

  const rebuildDay = useCallback(async () => {
    setFinalizing(true);
    try {
      await api.advisorFinalize(sessionId);
      setError(null);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setFinalizing(false);
    }
  }, [sessionId, load]);

  useEffect(() => {
    void load();
    const id = window.setInterval(() => void load(), 5000);
    return () => window.clearInterval(id);
  }, [load]);

  useEffect(() => {
    setSelectedId(null);
  }, [tf]);

  const active = useMemo(() => {
    if (!reports.length) return null;
    if (selectedId) {
      return reports.find((r) => r.id === selectedId) ?? reports[0];
    }
    return reports[0];
  }, [reports, selectedId]);

  const tabs = TF_TABS.map((tab) => ({ id: tab.id, label: t(tab.labelKey) }));

  return (
    <Card title={t('advisorAnalysisTitle')} subtitle={t('advisorAnalysisHelp')}>
      <div className="flex flex-wrap items-center gap-2 mb-3">
        <Tabs tabs={tabs} active={tf} onChange={setTf} />
        <Button type="button" variant="secondary" disabled={finalizing} onClick={() => void rebuildDay()}>
          {finalizing ? t('advisorRebuilding') : t('advisorRebuildDay')}
        </Button>
      </div>
      {error && <p className="mt-3 text-sm text-[var(--danger)]">{error}</p>}
      {tf === 'strategy' && strategy?.synthesis?.summary_md && (
        <div className="mt-4 advisor-summary-md whitespace-pre-wrap text-sm">{strategy.synthesis.summary_md}</div>
      )}
      {tf === 'strategy' && strategy?.synthesis?.drafts && strategy.synthesis.drafts.length > 0 && (
        <div className="mt-4 space-y-2">
          <h4 className="font-semibold text-sm">{t('advisorDrafts')}</h4>
          {strategy.synthesis.drafts.map((d) => (
            <article key={d.id} className="rounded-lg border border-[var(--border)] p-3 text-sm">
              <div className="flex items-center justify-between gap-2">
                <Badge tone="neutral">{d.kind}</Badge>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => void navigator.clipboard.writeText(d.body)}
                >
                  {t('advisorCopy')}
                </Button>
              </div>
              <p className="mt-1 font-medium">{d.title}</p>
              <pre className="mt-2 whitespace-pre-wrap text-xs text-[var(--muted)]">{d.body}</pre>
            </article>
          ))}
        </div>
      )}
      {reports.length === 0 && !error && <EmptyState>{t('advisorNoReports')}</EmptyState>}
      {active && (
        <div className="mt-4">
          <div className="flex flex-wrap gap-2 mb-3">
            {reports.map((r) => (
              <Button
                key={r.id}
                type="button"
                variant={r.id === active.id ? 'primary' : 'ghost'}
                onClick={() => setSelectedId(r.id)}
              >
                {formatDateTime(r.period_end, lang)}
                {r.status === 'failed' ? ' ⚠' : ''}
              </Button>
            ))}
          </div>
          <ReportBody report={active} t={t} />
        </div>
      )}
    </Card>
  );
}
