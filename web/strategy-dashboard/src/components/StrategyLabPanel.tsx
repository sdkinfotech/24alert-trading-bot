import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type {
  StrategyLabAnalyzeResponse,
  StrategyLabCatalog,
  StrategyLabFamilyGrade,
  StrategyLabFamilyLeader,
  StrategyLabRunRow,
  StrategyLabVerdict,
} from '../api/types';
import { InstrumentSearchPicker, type SelectedInstrument } from './InstrumentSearchPicker';
import { Badge, Card, EmptyState } from './ui';
import { useI18n } from '../i18n';
import type { Lang } from '../i18n';
import { formatNumber } from '../format';

type Step = 1 | 2 | 3 | 4;

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

function gradeTone(g: StrategyLabFamilyGrade): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  switch (g) {
    case 'excellent':
    case 'good':
      return 'success';
    case 'prod':
      return 'info';
    case 'mixed':
      return 'warning';
    case 'research':
      return 'neutral';
    default:
      return 'danger';
  }
}

export function StrategyLabPanel({ onDeployed }: { onDeployed?: () => void }) {
  const { t, lang } = useI18n();
  const [step, setStep] = useState<Step>(1);
  const [catalog, setCatalog] = useState<StrategyLabCatalog | null>(null);
  const [instrument, setInstrument] = useState<SelectedInstrument | null>(null);
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
      setStep(3);
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

  const steps = [t('labStep1'), t('labStep2Analyze'), t('labStep4'), t('labStep5')];

  return (
    <div className="space-y-6">
      <Card>
        <h2 className="text-lg font-semibold text-[var(--text)]">{t('labTitle')}</h2>
        <p className="text-sm text-[var(--muted)] mt-1">{t('labSubtitleNew')}</p>
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
          <p className="text-xs text-[var(--muted)] mt-3">{t('labFamiliesHint')}</p>
          {!catalogLoading && catalog?.strategies?.length ? (
            <ul className="text-xs text-[var(--muted)] mt-2 list-disc list-inside space-y-0.5">
              {catalog.strategies.map((s) => (
                <li key={s.id}>
                  {s.label}
                  {s.live_eligible ? '' : ` (${t('labResearchOnly')})`}
                </li>
              ))}
            </ul>
          ) : null}
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
          <h3 className="font-medium mb-2">{t('labStep2Analyze')}</h3>
          <p className="text-sm text-[var(--muted)] mb-4">
            {instrument?.ticker} · {t('labDays')} {days}
          </p>
          <p className="text-sm leading-relaxed mb-4">{t('labStep3AnalyzeHint')}</p>
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

      {step === 3 && (
        <Card>
          <h3 className="font-medium mb-2">{t('labStep4')}</h3>
          <p className="text-xs text-[var(--muted)] mb-2">{t('labPnlNote')}</p>
          {!analysis ? (
            <EmptyState>{t('labNoResults')}</EmptyState>
          ) : (
            <>
              <LabAnalysisBlock analysis={analysis} t={t} />
              <FamilyLeadersTable
                families={analysis.family_leaders ?? []}
                selected={selected}
                onSelect={(r) => setSelected(r)}
                t={t}
                lang={lang}
              />
              {selected && (
                <div className="mt-4 p-3 rounded-lg bg-[var(--surface2)] text-sm border border-[var(--border)]">
                  <div className="text-xs text-[var(--muted)] mb-1">{t('labSelectedForDeploy')}</div>
                  <div className="font-medium">{selected.strategy} / {selected.mode}</div>
                  <div className="text-[var(--muted)]">{paramsStr(selected.params)}</div>
                </div>
              )}
              <button
                type="button"
                className="ui-button ui-button-primary mt-4"
                disabled={!analysis.rollout.can_stage_config}
                onClick={() => setStep(4)}
              >
                {t('labNext')}
              </button>
            </>
          )}
        </Card>
      )}

      {step === 4 && (
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
  const recLabel =
    analysis.recommendation === 'apply'
      ? t('labRec_apply')
      : analysis.recommendation === 'research_only'
        ? t('labRec_research_only')
        : analysis.recommendation === 'wait'
          ? t('labRec_wait')
          : t('labRec_keep_prod');
  return (
    <div className="space-y-4 text-sm">
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface2)] p-4">
        <div className="flex flex-wrap items-center gap-2 mb-2">
          <Badge tone={tone}>{t(`labVerdict_${rec}`)}</Badge>
          <Badge tone={analysis.recommendation === 'apply' ? 'success' : 'neutral'}>{recLabel}</Badge>
        </div>
        <p className="text-[var(--text)] leading-relaxed font-medium">{analysis.verdict_reason}</p>
      </div>
      <MarkdownBlock text={analysis.summary_md} />
      {analysis.vs_production_md && (
        <div>
          <h4 className="text-xs font-semibold uppercase tracking-wide text-[var(--muted)] mb-2">
            {t('labInterpretVsProd')}
          </h4>
          <MarkdownBlock text={analysis.vs_production_md} className="text-[var(--muted)]" />
        </div>
      )}
      {analysis.action_md && <MarkdownBlock text={analysis.action_md} />}
      {analysis.warnings_md && (
        <div className="rounded-lg bg-[var(--warning)]/10 border border-[var(--warning)]/30 px-3 py-2">
          <MarkdownBlock text={analysis.warnings_md} />
        </div>
      )}
    </div>
  );
}

function MarkdownBlock({ text, className = '' }: { text: string; className?: string }) {
  return (
    <div
      className={`whitespace-pre-wrap leading-relaxed prose-invert max-w-none ${className}`}
    >
      {text}
    </div>
  );
}

function FamilyLeadersTable({
  families,
  selected,
  onSelect,
  t,
  lang,
}: {
  families: StrategyLabFamilyLeader[];
  selected: StrategyLabRunRow | null;
  onSelect: (r: StrategyLabRunRow) => void;
  t: (k: string) => string;
  lang: Lang;
}) {
  if (!families.length) return null;
  return (
    <div className="mt-6">
      <h4 className="font-medium mb-1">{t('labFamilyTableTitle')}</h4>
      <p className="text-xs text-[var(--muted)] mb-3">{t('labFamilyTableHint')}</p>
      <div className="overflow-x-auto">
        <table className="ui-table text-xs w-full">
          <thead>
            <tr>
              <th>{t('labColFamily')}</th>
              <th>{t('labColConfig')}</th>
              <th>PnL</th>
              <th>{t('labColVsProd')}</th>
              <th>{t('labTrades')}</th>
              <th>{t('labColGrade')}</th>
            </tr>
          </thead>
          <tbody>
            {families.map((f) => {
              const rk = rowKey(f.row);
              const isSel = selected && rowKey(selected) === rk;
              return (
                <tr
                  key={f.strategy_id + (f.is_production ? '-prod' : '')}
                  className={`cursor-pointer align-top ${isSel ? 'bg-[var(--accent)]/15' : ''}`}
                  onClick={() => onSelect(f.row)}
                  title={f.verdict_line}
                >
                  <td className="font-medium">{f.label}</td>
                  <td className="max-w-[12rem]">{f.params_summary}</td>
                  <td>{formatNumber(f.pnl, lang)}</td>
                  <td>
                    {f.delta_vs_prod != null ? (
                      <span className={f.delta_vs_prod >= 0 ? 'text-[var(--success)]' : 'text-[var(--danger)]'}>
                        {f.delta_vs_prod >= 0 ? '+' : ''}
                        {formatNumber(f.delta_vs_prod, lang)}
                      </span>
                    ) : (
                      '—'
                    )}
                  </td>
                  <td>{f.trades}</td>
                  <td>
                    <Badge tone={gradeTone(f.grade)}>{f.grade_label}</Badge>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <p className="text-xs text-[var(--muted)] mt-2">{t('labFamilyRowHint')}</p>
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
