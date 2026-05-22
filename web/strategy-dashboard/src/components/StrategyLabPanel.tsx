import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type {
  StrategyLabAnalyzeResponse,
  StrategyLabCatalog,
  StrategyLabRunRow,
  StrategyLabStrategyMeta,
  StrategyLabVerdict,
} from '../api/types';
import { InstrumentSearchPicker, type SelectedInstrument } from './InstrumentSearchPicker';
import { Badge, Card, EmptyState } from './ui';
import { useI18n } from '../i18n';
import type { Lang } from '../i18n';
import { formatNumber } from '../format';

type Step = 1 | 2 | 3 | 4 | 5;

function paramsStr(p: Record<string, string> | undefined): string {
  if (!p) return '—';
  return Object.entries(p)
    .filter(([k]) => k !== 'interval' && k !== 'type')
    .map(([k, v]) => `${k}=${v}`)
    .join(', ');
}

function rowKey(r: StrategyLabRunRow): string {
  return `${r.strategy}|${r.mode ?? ''}|${JSON.stringify(r.params)}`;
}

export function StrategyLabPanel({ onDeployed }: { onDeployed?: () => void }) {
  const { t, lang } = useI18n();
  const [step, setStep] = useState<Step>(1);
  const [catalog, setCatalog] = useState<StrategyLabCatalog | null>(null);
  const [instrument, setInstrument] = useState<SelectedInstrument | null>(null);
  const [strategy, setStrategy] = useState<StrategyLabStrategyMeta | null>(null);
  const [days, setDays] = useState(90);
  const [analysis, setAnalysis] = useState<StrategyLabAnalyzeResponse | null>(null);
  const [selected, setSelected] = useState<StrategyLabRunRow | null>(null);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [catalogLoading, setCatalogLoading] = useState(true);
  const [applyMsg, setApplyMsg] = useState<string | null>(null);
  const [confirmLive, setConfirmLive] = useState(false);

  const loadCatalog = useCallback(() => {
    setCatalogLoading(true);
    setError(null);
    return api
      .strategyLabCatalog()
      .then(setCatalog)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setCatalogLoading(false));
  }, []);

  useEffect(() => {
    void loadCatalog();
  }, [loadCatalog]);

  const resultRows = useMemo(() => analysis?.top_rows ?? [], [analysis]);

  const deployRow = useMemo(() => {
    if (selected) return selected;
    return analysis?.candidate ?? null;
  }, [selected, analysis]);

  const runAnalyze = useCallback(async () => {
    if (!instrument) return;
    setBusy('analyze');
    setError(null);
    setApplyMsg(null);
    setAnalysis(null);
    try {
      const res = await api.strategyLabAnalyze({
        uid: instrument.uid,
        ticker: instrument.ticker,
        days,
        lang,
      });
      setAnalysis(res);
      setSelected(res.candidate ?? res.top_rows[0] ?? null);
      setStep(4);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  }, [instrument, days, lang]);

  const stageConfig = useCallback(async () => {
    if (!instrument || !deployRow) return;
    const typ = deployRow.strategy;
    if (!deployRow.live_eligible) {
      setError(t('labDeployBlockedResearch'));
      return;
    }
    setBusy('stage');
    setError(null);
    setApplyMsg(null);
    try {
      const params: Record<string, string> = { ...deployRow.params, quantity: '1' };
      if (typ === 'sma_crossover' || typ === 'sma') {
        params.interval = '1h';
      }
      if (typ === 'level_bounce') {
        params.interval = '15min';
      }
      const res = await api.strategyLabApply({
        phase: 'stage',
        type: typ,
        instance_id: analysis?.config_prod?.instance_id,
        account_id: analysis?.config_prod?.account_id,
        instrument_uid: instrument.uid,
        ticker: instrument.ticker,
        params,
        analysis_verdict: analysis?.verdict,
      });
      setApplyMsg(`${t('labStageOk')}: ${res.instance_id} (${res.status})`);
      onDeployed?.();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  }, [instrument, deployRow, analysis, t, onDeployed]);

  const enableLive = useCallback(async () => {
    if (!instrument || !deployRow || !analysis) return;
    if (!confirmLive) {
      setError(t('labConfirmLiveRequired'));
      return;
    }
    setBusy('enable');
    setError(null);
    try {
      const params: Record<string, string> = { ...deployRow.params, quantity: '1' };
      const typ = deployRow.strategy;
      if (typ === 'sma_crossover' || typ === 'sma') params.interval = '1h';
      if (typ === 'level_bounce') params.interval = '15min';
      const res = await api.strategyLabApply({
        phase: 'enable_live',
        confirm_live: true,
        analysis_verdict: analysis.verdict,
        type: typ,
        instance_id: analysis.config_prod?.instance_id,
        account_id: analysis.config_prod?.account_id,
        instrument_uid: instrument.uid,
        ticker: instrument.ticker,
        params,
        start: false,
      });
      if (res.blocked_reasons?.length) {
        setError(res.blocked_reasons.join('; '));
        return;
      }
      setApplyMsg(`${t('labEnableOk')}: ${res.instance_id} — ${res.status}`);
      onDeployed?.();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  }, [instrument, deployRow, analysis, confirmLive, t, onDeployed]);

  const steps = [
    t('labStep1'),
    t('labStep2'),
    t('labStep3'),
    t('labStep4'),
    t('labStep5'),
  ];

  return (
    <div className="space-y-6">
      <Card>
        <h2 className="text-lg font-semibold text-[var(--text)]">{t('labTitle')}</h2>
        <p className="text-sm text-[var(--muted)] mt-1">{t('labSubtitle')}</p>
        <div className="flex flex-wrap gap-2 mt-4">
          {steps.map((label, i) => (
            <button
              key={label}
              type="button"
              className={`ui-button text-xs ${step === i + 1 ? 'ui-button-primary' : 'ui-button-secondary'}`}
              onClick={() => setStep((i + 1) as Step)}
            >
              {i + 1}. {label}
            </button>
          ))}
        </div>
      </Card>

      {error && (
        <div className="rounded-lg border border-[var(--danger)] bg-[var(--danger)]/10 px-4 py-2 text-sm text-[var(--danger)] flex flex-wrap items-center gap-2">
          <span>{error}</span>
          <button type="button" className="ui-button ui-button-secondary text-xs" onClick={() => void loadCatalog()}>
            {t('labRetry')}
          </button>
        </div>
      )}
      {applyMsg && (
        <div className="rounded-lg border border-[var(--ok)] bg-[var(--ok)]/10 px-4 py-2 text-sm text-[var(--ok)]">
          {applyMsg}
        </div>
      )}

      {step === 1 && (
        <Card>
          <h3 className="font-medium mb-2">{t('labStep1')}</h3>
          <InstrumentSearchPicker value={instrument} onChange={setInstrument} />
          <div className="flex flex-wrap gap-2 mt-3">
            {catalog?.core_tickers?.map((c) => (
              <button
                key={c.uid}
                type="button"
                className="ui-button ui-button-secondary text-xs"
                onClick={() =>
                  setInstrument({ uid: c.uid, ticker: c.ticker, name: c.ticker, kind: 'future' })
                }
              >
                {c.ticker}
              </button>
            ))}
          </div>
          <div className="mt-4 flex gap-2 items-center">
            <label className="text-sm text-[var(--muted)]">{t('labDays')}</label>
            <input
              type="number"
              className="ui-input w-24"
              min={14}
              max={120}
              value={days}
              onChange={(e) => setDays(Number(e.target.value) || 90)}
            />
          </div>
          <button
            type="button"
            className="ui-button ui-button-primary mt-4"
            disabled={!instrument}
            onClick={() => setStep(2)}
          >
            {t('labNext')}
          </button>
        </Card>
      )}

      {step === 2 && (
        <Card>
          <h3 className="font-medium mb-2">{t('labStep2')}</h3>
          <p className="text-xs text-[var(--muted)] mb-3">
            {instrument?.ticker} · {t('labDays')} {days}
          </p>
          {catalogLoading && <p className="text-sm text-[var(--muted)]">{t('labLoading')}</p>}
          {!catalogLoading && !catalog?.strategies?.length && !error && (
            <EmptyState>{t('labCatalogEmpty')}</EmptyState>
          )}
          <div className="grid gap-2 sm:grid-cols-2">
            {catalog?.strategies?.map((s) => (
              <button
                key={s.id}
                type="button"
                className={`text-left rounded-lg border p-3 transition ${
                  strategy?.id === s.id
                    ? 'border-[var(--accent)] bg-[var(--accent)]/10'
                    : 'border-[var(--border)] hover:border-[var(--accent)]'
                }`}
                onClick={() => setStrategy(s)}
              >
                <div className="font-medium">{s.label}</div>
                <div className="text-xs text-[var(--muted)]">{s.interval}</div>
                {s.live_eligible ? (
                  <Badge tone="success">{t('labLiveOk')}</Badge>
                ) : (
                  <Badge tone="warning">{t('labResearchOnly')}</Badge>
                )}
              </button>
            ))}
          </div>
          <button
            type="button"
            className="ui-button ui-button-primary mt-4"
            disabled={!strategy}
            onClick={() => setStep(3)}
          >
            {t('labNext')}
          </button>
        </Card>
      )}

      {step === 3 && (
        <Card>
          <h3 className="font-medium mb-2">{t('labStep3')}</h3>
          <p className="text-sm text-[var(--muted)] mb-4">{t('labStep3AnalyzeHint')}</p>
          <label className="text-xs text-[var(--muted)] block mb-1">{t('labDays')}</label>
          <input
            type="number"
            className="ui-input w-24 mb-4"
            min={14}
            max={180}
            value={days}
            onChange={(e) => setDays(Number(e.target.value) || 90)}
          />
          <button
            type="button"
            className="ui-button ui-button-primary"
            disabled={!!busy || !instrument}
            onClick={() => void runAnalyze()}
          >
            {busy === 'analyze' ? t('labRunning') : t('labRunAnalyze')}
          </button>
        </Card>
      )}

      {step === 4 && (
        <Card>
          <h3 className="font-medium mb-2">{t('labStep4')}</h3>
          <p className="text-xs text-[var(--muted)] mb-2">{t('labPnlNote')}</p>
          {!analysis ? (
            <EmptyState>{t('labNoResults')}</EmptyState>
          ) : (
            <>
              <LabAnalysisBlock analysis={analysis} t={t} />
              <div className="overflow-x-auto mt-4">
                <ResultsTable rows={resultRows} selected={selected} onSelect={setSelected} t={t} lang={lang} />
              </div>
              {selected && (
                <div className="mt-4 p-3 rounded-lg bg-[var(--surface2)] text-sm">
                  <div className="font-medium">{selected.strategy} / {selected.mode}</div>
                  <div className="text-[var(--muted)]">{paramsStr(selected.params)}</div>
                </div>
              )}
              <button
                type="button"
                className="ui-button ui-button-primary mt-4"
                disabled={!analysis.rollout.can_stage_config}
                onClick={() => setStep(5)}
              >
                {t('labNext')}
              </button>
            </>
          )}
        </Card>
      )}

      {step === 5 && (
        <Card>
          <h3 className="font-medium mb-2">{t('labStep5')}</h3>
          {!analysis || !deployRow ? (
            <EmptyState>{t('labPickFirst')}</EmptyState>
          ) : (
            <>
              <LabRolloutChecklist analysis={analysis} />
              <p className="text-sm mt-4 mb-2">
                <strong>{instrument?.ticker}</strong> · {deployRow.strategy} · {paramsStr(deployRow.params)}
              </p>
              <button
                type="button"
                className="ui-button ui-button-primary mr-2"
                disabled={!!busy || !analysis.rollout.can_stage_config}
                onClick={() => void stageConfig()}
              >
                {busy === 'stage' ? t('labRunning') : t('labStageConfig')}
              </button>
              {analysis.rollout.can_enable_live && (
                <div className="mt-4 border-t border-[var(--border)] pt-4">
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={confirmLive}
                      onChange={(e) => setConfirmLive(e.target.checked)}
                    />
                    {t('labConfirmLiveLabel')}
                  </label>
                  <p className="text-xs text-[var(--muted)] mt-1">{t('labEnableLiveHint')}</p>
                  <button
                    type="button"
                    className="ui-button ui-button-secondary mt-2"
                    disabled={!!busy || !confirmLive}
                    onClick={() => void enableLive()}
                  >
                    {busy === 'enable' ? t('labRunning') : t('labEnableLive')}
                  </button>
                </div>
              )}
            </>
          )}
        </Card>
      )}
    </div>
  );
}

function LabAnalysisBlock({ analysis, t }: { analysis: StrategyLabAnalyzeResponse; t: (k: string) => string }) {
  const rec = analysis.verdict as StrategyLabVerdict;
  const tone: 'success' | 'warning' | 'neutral' =
    rec === 'deploy_candidate' ? 'success' : rec === 'research_only' ? 'warning' : 'neutral';
  return (
    <div className="space-y-3 text-sm border-b border-[var(--border)] pb-4">
      <div className="flex flex-wrap items-center gap-2">
        <Badge tone={tone}>{t(`labVerdict_${rec}`)}</Badge>
        <span className="text-[var(--muted)]">{analysis.verdict_reason}</span>
      </div>
      <div className="whitespace-pre-wrap leading-relaxed">{analysis.summary_md}</div>
      {analysis.vs_production_md && (
        <div className="whitespace-pre-wrap text-[var(--muted)]">{analysis.vs_production_md}</div>
      )}
      {analysis.warnings_md && (
        <div className="rounded-lg bg-[var(--warning)]/10 border border-[var(--warning)]/30 px-3 py-2 whitespace-pre-wrap">
          {analysis.warnings_md}
        </div>
      )}
    </div>
  );
}

function LabRolloutChecklist({ analysis }: { analysis: StrategyLabAnalyzeResponse }) {
  return (
    <ol className="list-decimal list-inside space-y-3 text-sm">
      {analysis.rollout.steps.map((s) => (
        <li key={s.id}>
          <span className="font-medium">{s.title}</span>
          {s.automated && <Badge tone="neutral">API</Badge>}
          <div className="text-[var(--muted)] whitespace-pre-wrap mt-1 ml-4">{s.detail_md}</div>
        </li>
      ))}
    </ol>
  );
}

function ResultsTable({
  rows,
  selected,
  onSelect,
  t,
  lang,
}: {
  rows: StrategyLabRunRow[];
  selected: StrategyLabRunRow | null;
  onSelect: (r: StrategyLabRunRow) => void;
  t: (k: string) => string;
  lang: Lang;
}) {
  return (
    <table className="ui-table text-xs w-full mt-2">
      <thead>
        <tr>
          <th>{t('labColStrategy')}</th>
          <th>{t('labColParams')}</th>
          <th>PnL</th>
          <th>Sharpe</th>
          <th>DD</th>
          <th>{t('labTrades')}</th>
          <th>{t('labColProd')}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr
            key={rowKey(r)}
            className={`cursor-pointer ${selected && rowKey(selected) === rowKey(r) ? 'bg-[var(--accent)]/15' : ''}`}
            onClick={() => onSelect(r)}
          >
            <td>{r.mode === 'prod' || r.mode === 'prod_baseline' ? t('labProdBaseline') : `${r.strategy}/${r.mode ?? '—'}`}</td>
            <td className="max-w-[14rem] truncate" title={paramsStr(r.params)}>{paramsStr(r.params)}</td>
            <td>{formatNumber(r.pnl, lang)}</td>
            <td>{formatNumber(r.sharpe, lang)}</td>
            <td>{formatNumber(r.max_drawdown, lang)}</td>
            <td>{r.trades}</td>
            <td>{r.live_eligible ? t('labYes') : t('labNo')}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
