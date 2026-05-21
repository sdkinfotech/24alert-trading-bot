import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api/client';
import type { AssistantAnalysis } from '../api/types';
import { useI18n } from '../i18n';
import { InstrumentSearchPicker, type SelectedInstrument } from './InstrumentSearchPicker';
import { AssistantChart } from './AssistantChart';
import { AssistantLevelAccordion } from './AssistantLevelAccordion';
import { AssistantScenarios } from './AssistantScenarios';
import { Badge, Button, Card } from './ui';

type ChartTF = '1d' | '1h' | '5m';

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

export function AssistantPanel() {
  const { t } = useI18n();
  const [pick, setPick] = useState<SelectedInstrument | null>(null);
  const [analysis, setAnalysis] = useState<AssistantAnalysis | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tf, setTf] = useState<ChartTF>('1h');
  const [highlight, setHighlight] = useState<string | undefined>();
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPoll = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  useEffect(() => () => stopPoll(), [stopPoll]);

  const pollAnalysis = useCallback(
    (id: string) => {
      stopPoll();
      pollRef.current = setInterval(async () => {
        try {
          const a = await api.getAssistantAnalysis(id);
          setAnalysis(a);
          if (a.status === 'done' || a.status === 'error') {
            stopPoll();
            setLoading(false);
            if (a.status === 'error') {
              setError(a.error ?? 'analysis failed');
            }
          }
        } catch (e: unknown) {
          stopPoll();
          setLoading(false);
          setError(errorMessage(e));
        }
      }, 2000);
    },
    [stopPoll],
  );

  const runAnalysis = async () => {
    if (!pick?.ticker) return;
    setLoading(true);
    setError(null);
    setAnalysis(null);
    setHighlight(undefined);
    try {
      const start = await api.startAssistantAnalysis(pick.ticker);
      pollAnalysis(start.analysis_id);
      const first = await api.getAssistantAnalysis(start.analysis_id);
      setAnalysis(first);
    } catch (e: unknown) {
      setLoading(false);
      setError(errorMessage(e));
    }
  };

  const chartPayload = analysis?.charts?.[tf];

  return (
    <div className="space-y-6">
      <Card title={t('assistantTitle')} subtitle={t('assistantSubtitle')}>
        <div className="flex flex-wrap items-end gap-4">
          <div className="min-w-[240px] flex-1">
            <InstrumentSearchPicker value={pick} onChange={setPick} />
          </div>
          <Button variant="primary" disabled={!pick || loading} onClick={() => void runAnalysis()}>
            {loading ? t('assistantRunning') : t('assistantAnalyze')}
          </Button>
        </div>
        {error && (
          <p className="mt-3 text-sm text-[var(--danger)]">{error}</p>
        )}
        {analysis && (
          <div className="mt-3 flex flex-wrap gap-2 text-sm">
            <Badge tone={analysis.status === 'done' ? 'success' : analysis.status === 'error' ? 'danger' : 'info'}>
              {analysis.status}
            </Badge>
            {analysis.status === 'running' && (
              <span className="text-[var(--muted)]">{analysis.progress_pct}%</span>
            )}
            {analysis.llm_model && (
              <span className="text-[var(--muted)]">{analysis.llm_model}</span>
            )}
            {analysis.llm_fallback && (
              <Badge tone="warning">{t('assistantFallback')}</Badge>
            )}
          </div>
        )}
      </Card>

      {analysis?.status === 'done' && (
        <>
          {analysis.summary_md && (
            <Card title={t('assistantSummary')}>
              <div className="text-sm whitespace-pre-wrap prose dark:prose-invert max-w-none">{analysis.summary_md}</div>
            </Card>
          )}

          <div className="grid gap-6 xl:grid-cols-[1fr_320px]">
            <Card
              title={t('assistantChart')}
              subtitle={chartPayload?.horizon ? `${tf} · ${chartPayload.horizon}` : tf}
              actions={
                <div className="flex gap-1">
                  {(['1d', '1h', '5m'] as ChartTF[]).map((x) => (
                    <button
                      key={x}
                      type="button"
                      className={`ui-button ui-button-secondary text-xs ${tf === x ? 'ring-2 ring-[var(--info)]' : ''}`}
                      onClick={() => setTf(x)}
                    >
                      {x}
                    </button>
                  ))}
                </div>
              }
            >
              <AssistantChart chart={chartPayload} timeframe={tf} highlightLevelId={highlight} />
            </Card>

            <Card title={t('assistantLevels')} subtitle={t('assistantLevelsHelp')}>
              <AssistantLevelAccordion
                levels={analysis.levels ?? []}
                selectedId={highlight}
                onSelect={setHighlight}
              />
            </Card>
          </div>

          <AssistantScenarios scenarios={analysis.scenarios ?? []} />
        </>
      )}
    </div>
  );
}
