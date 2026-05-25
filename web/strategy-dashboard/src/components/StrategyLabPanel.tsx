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

function verdictTone(v: StrategyLabVerdict): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  if (v === 'deploy_candidate') return 'success';
  if (v === 'research_only') return 'warning';
  if (v === 'insufficient') return 'danger';
  return 'info';
}

/** Minimal markdown: headers and paragraphs only (no broken pipe tables). */
function SimpleMarkdown({ text }: { text: string }) {
  const blocks = text.split(/\n\n+/);
  return (
    <div className="space-y-3 text-sm leading-relaxed">
      {blocks.map((block, i) => {
        const line = block.trim();
        if (!line) return null;
        if (line.startsWith('### ')) {
          return (
            <h4 key={i} className="font-semibold text-[var(--text)] mt-2">
              {line.replace(/^###\s+/, '')}
            </h4>
          );
        }
        if (line.startsWith('## ')) {
          return (
            <h3 key={i} className="text-base font-semibold text-[var(--text)]">
              {line.replace(/^##\s+/, '')}
            </h3>
          );
        }
        const html = line
          .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
          .replace(/_([^_]+)_/g, '<em class="text-[var(--muted)]">$1</em>');
        if (line.startsWith('- ')) {
          return (
            <ul key={i} className="list-disc list-inside space-y-1 text-[var(--text)]">
              {line.split('\n').map((li, j) => (
                <li
                  key={j}
                  dangerouslySetInnerHTML={{
                    __html: li.replace(/^- /, '').replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>'),
                  }}
                />
              ))}
            </ul>
          );
        }
        return (
          <p
            key={i}
            className="text-[var(--text)]"
            dangerouslySetInnerHTML={{ __html: html }}
          />
        );
      })}
    </div>
  );
}

export function StrategyLabPanel({ onDeployed }: { onDeployed?: () => void }) {
  const { t, lang } = useI18n();
  const [catalog, setCatalog] = useState<StrategyLabCatalog | null>(null);
  const [instrument, setInstrument] = useState<SelectedInstrument | null>(null);
  const [days, setDays] = useState(30);
  const [analysis, setAnalysis] = useState<StrategyLabAnalyzeResponse | null>(null);
  const [selected, setSelected] = useState<StrategyLabRunRow | null>(null);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [catalogLoading, setCatalogLoading] = useState(true);
  const [applyMsg, setApplyMsg] = useState<string | null>(null);
  const [confirmLive, setConfirmLive] = useState(false);
  const [showDeploy, setShowDeploy] = useState(false);

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

  const families = analysis?.family_leaders ?? [];

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
    setShowDeploy(false);
    try {
      const res = await api.strategyLabAnalyze({
        uid: instrument.uid,
        ticker: instrument.ticker,
        days,
        lang,
      });
      setAnalysis(res);
      setSelected(res.candidate ?? res.top_rows?.[0] ?? null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  }, [instrument, days, lang]);

  const stageConfig = useCallback(async () => {
    if (!instrument || !deployRow) return;
    if (!deployRow.live_eligible) {
      setError(t('labDeployBlockedResearch'));
      return;
    }
    setBusy('stage');
    setError(null);
    setApplyMsg(null);
    try {
      const params: Record<string, string> = { ...deployRow.params, quantity: '1' };
      const typ = deployRow.strategy;
      if (typ === 'sma_crossover' || typ === 'sma') params.interval = '1h';
      if (typ === 'level_bounce') params.interval = '15min';
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

  const recLabel =
    analysis?.recommendation === 'apply'
      ? t('labRec_apply')
      : analysis?.recommendation === 'research_only'
        ? t('labRec_research_only')
        : analysis?.recommendation === 'wait'
          ? t('labRec_wait')
          : t('labRec_keep_prod');

  return (
    <div className="space-y-6">
      <Card>
        <h2 className="text-lg font-semibold text-[var(--text)]">{t('labTitle')}</h2>
        <p className="text-sm text-[var(--muted)] mt-1">{t('labSubtitleNew')}</p>
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

      <Card>
        <h3 className="font-medium mb-3">{t('labStep1')}</h3>
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
        <div className="mt-4 flex flex-wrap gap-4 items-end">
          <div>
            <label className="text-sm text-[var(--muted)] block mb-1">{t('labDays')}</label>
            <input
              type="number"
              className="ui-input w-24"
              min={14}
              max={120}
              value={days}
              onChange={(e) => setDays(Number(e.target.value) || 30)}
            />
            <p className="text-xs text-[var(--muted)] mt-1">{t('labDaysHint')}</p>
          </div>
          <button
            type="button"
            className="ui-button ui-button-primary"
            disabled={!!busy || !instrument}
            onClick={() => void runAnalyze()}
          >
            {busy === 'analyze' ? t('labRunningLong') : t('labRunAnalyze')}
          </button>
        </div>
        {catalogLoading && <p className="text-xs text-[var(--muted)] mt-2">{t('labLoading')}</p>}
      </Card>

      {busy === 'analyze' && (
        <Card>
          <p className="text-sm text-[var(--muted)] animate-pulse">{t('labRunningLong')}</p>
        </Card>
      )}

      {analysis && !busy && (
        <>
          <Card>
            <div className="flex flex-wrap items-center gap-2 mb-3">
              <Badge tone={verdictTone(analysis.verdict)}>{t(`labVerdict_${analysis.verdict}`)}</Badge>
              <Badge tone={analysis.recommendation === 'apply' ? 'success' : 'neutral'}>{recLabel}</Badge>
              <span className="text-xs text-[var(--muted)]">
                {analysis.ticker} · {analysis.days} {t('labDays').toLowerCase()}
              </span>
            </div>
            <p className="text-base font-medium text-[var(--text)] mb-4">{analysis.verdict_reason}</p>

            <FamilyLeadersTable
              families={families}
              selected={selected}
              onSelect={setSelected}
              t={t}
              lang={lang}
            />

            {selected && (
              <div className="mt-4 p-3 rounded-lg bg-[var(--surface2)] border border-[var(--border)] text-sm">
                <div className="text-xs text-[var(--muted)]">{t('labSelectedForDeploy')}</div>
                <div className="font-medium mt-1">
                  {selected.strategy} · {paramsStr(selected.params)}
                </div>
              </div>
            )}

            <details className="mt-4">
              <summary className="cursor-pointer text-sm text-[var(--muted)] hover:text-[var(--text)]">
                {t('labDetailsToggle')}
              </summary>
              <div className="mt-3 space-y-4 border-t border-[var(--border)] pt-4">
                <SimpleMarkdown text={analysis.summary_md} />
                {analysis.vs_production_md && (
                  <div>
                    <h4 className="text-xs font-semibold uppercase text-[var(--muted)] mb-2">
                      {t('labInterpretVsProd')}
                    </h4>
                    <SimpleMarkdown text={analysis.vs_production_md} />
                  </div>
                )}
                {analysis.action_md && <SimpleMarkdown text={analysis.action_md} />}
                {analysis.warnings_md && (
                  <div className="rounded-lg bg-[var(--warning)]/10 border border-[var(--warning)]/30 p-3">
                    <SimpleMarkdown text={analysis.warnings_md} />
                  </div>
                )}
              </div>
            </details>
          </Card>

          <Card>
            <button
              type="button"
              className="text-sm font-medium text-[var(--accent)]"
              onClick={() => setShowDeploy((v) => !v)}
            >
              {showDeploy ? '▼' : '▶'} {t('labStep5')}
            </button>
            {showDeploy && (
              <div className="mt-4">
                <LabRolloutChecklist analysis={analysis} />
                <div className="mt-4 flex flex-wrap gap-2">
                  <button
                    type="button"
                    className="ui-button ui-button-primary"
                    disabled={!!busy || !analysis.rollout.can_stage_config || !deployRow}
                    onClick={() => void stageConfig()}
                  >
                    {busy === 'stage' ? t('labRunning') : t('labStageConfig')}
                  </button>
                </div>
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
              </div>
            )}
          </Card>
        </>
      )}

      {!analysis && !busy && (
        <EmptyState>{t('labStartHint')}</EmptyState>
      )}
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
  if (!families.length) {
    return <p className="text-sm text-[var(--warning)]">{t('labNoFamilies')}</p>;
  }
  return (
    <div>
      <h4 className="font-medium mb-1">{t('labFamilyTableTitle')}</h4>
      <p className="text-xs text-[var(--muted)] mb-3">{t('labFamilyTableHint')}</p>
      <div className="overflow-x-auto rounded-lg border border-[var(--border)]">
        <table className="ui-table text-sm w-full">
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
                  key={`${f.strategy_id}-${f.is_production ? 'prod' : 'best'}`}
                  className={`cursor-pointer align-top ${f.is_production ? 'bg-[var(--surface-muted)]' : ''} ${isSel ? 'ring-1 ring-[var(--accent)]' : ''}`}
                  onClick={() => !f.is_production && onSelect(f.row)}
                  title={f.verdict_line}
                >
                  <td className="font-medium whitespace-nowrap">
                    {f.label}
                    {f.is_production && (
                      <span className="ml-1 text-xs text-[var(--muted)]">({t('labProdBaseline')})</span>
                    )}
                  </td>
                  <td className="max-w-[14rem] text-[var(--muted)]">{f.params_summary}</td>
                  <td className="whitespace-nowrap">{formatNumber(f.pnl, lang)}</td>
                  <td className="whitespace-nowrap">
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
      <p className="text-xs text-[var(--muted)] mt-2">{t('labPnlNote')}</p>
    </div>
  );
}

function LabRolloutChecklist({ analysis }: { analysis: StrategyLabAnalyzeResponse }) {
  return (
    <ol className="list-decimal list-inside space-y-2 text-sm text-[var(--muted)]">
      {analysis.rollout.steps.map((s) => (
        <li key={s.id}>
          <span className="font-medium text-[var(--text)]">{s.title}</span>
        </li>
      ))}
    </ol>
  );
}
