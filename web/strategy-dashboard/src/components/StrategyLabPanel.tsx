import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type {
  StrategyLabCatalog,
  StrategyLabRunRow,
  StrategyLabMatrixResponse,
  StrategyLabStrategyMeta,
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
  const [optimizeResult, setOptimizeResult] = useState<StrategyLabRunRow[]>([]);
  const [matrix, setMatrix] = useState<StrategyLabMatrixResponse | null>(null);
  const [selected, setSelected] = useState<StrategyLabRunRow | null>(null);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [applyMsg, setApplyMsg] = useState<string | null>(null);

  useEffect(() => {
    void api.strategyLabCatalog().then(setCatalog).catch((e) => setError(String(e)));
  }, []);

  const instResult = useMemo(() => {
    if (!matrix?.instruments?.length || !instrument?.ticker) return null;
    return matrix.instruments.find((i) => i.ticker === instrument.ticker) ?? matrix.instruments[0];
  }, [matrix, instrument?.ticker]);

  const compareRows = useMemo(() => {
    if (!instResult) return [];
    const rows: StrategyLabRunRow[] = [];
    if (instResult.production) rows.push({ ...instResult.production, mode: 'prod' });
    for (const r of instResult.top10_overall ?? []) rows.push(r);
    const seen = new Set<string>();
    return rows.filter((r) => {
      const k = rowKey(r);
      if (seen.has(k)) return false;
      seen.add(k);
      return true;
    }).slice(0, 15);
  }, [instResult]);

  const runOptimize = useCallback(async () => {
    if (!instrument || !strategy) return;
    setBusy('optimize');
    setError(null);
    try {
      const raw = await api.strategyLabOptimize({
        uid: instrument.uid,
        ticker: instrument.ticker,
        strategy: strategy.id,
        days,
      });
      const rows: StrategyLabRunRow[] = [];
      if ('best' in raw && raw.best) rows.push(raw.best);
      if ('top10' in raw && raw.top10) rows.push(...raw.top10);
      const inst = (raw as StrategyLabMatrixResponse).instruments?.[0];
      if (inst?.top10_overall) rows.push(...inst.top10_overall);
      if (inst?.best_deployable) rows.unshift(inst.best_deployable);
      setOptimizeResult(rows.slice(0, 12));
      if (rows[0]) setSelected(rows[0]);
      setStep(4);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  }, [instrument, strategy, days]);

  const runCompare = useCallback(async () => {
    if (!instrument) return;
    setBusy('compare');
    setError(null);
    try {
      const res = await api.strategyLabCompare({ ticker: instrument.ticker, uid: instrument.uid, days });
      setMatrix(res);
      const inst = res.instruments?.[0];
      const pick = inst?.best_deployable ?? inst?.top10_overall?.[0] ?? null;
      if (pick) setSelected(pick);
      setStep(4);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  }, [instrument, days]);

  const deploy = useCallback(async () => {
    if (!instrument || !selected) return;
    if (!selected.live_eligible) {
      setError(t('labDeployBlocked'));
      return;
    }
    const typ = selected.strategy;
    if (typ === 'orb_breakout' || typ === 'ema_1h' || typ === 'donchian_15m') {
      setError(t('labDeployBlocked'));
      return;
    }
    setBusy('apply');
    setError(null);
    setApplyMsg(null);
    try {
      const params: Record<string, string> = { ...selected.params, quantity: '1' };
      if (typ === 'sma_crossover') {
        params.interval = '1h';
        if (!params.trailing_stop_pct || Number(params.trailing_stop_pct) <= 0) {
          setError(t('labTrailRequired'));
          return;
        }
      }
      if (typ === 'level_bounce') {
        params.interval = '15min';
      }
      const res = await api.strategyLabApply({
        type: typ,
        instrument_uid: instrument.uid,
        ticker: instrument.ticker,
        params,
        enabled: true,
        start: true,
      });
      setApplyMsg(`${t('labDeployOk')}: ${res.instance_id}`);
      onDeployed?.();
      setStep(5);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  }, [instrument, selected, t, onDeployed]);

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
        <div className="rounded-lg border border-[var(--danger)] bg-[var(--danger)]/10 px-4 py-2 text-sm text-[var(--danger)]">
          {error}
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
          <p className="text-sm text-[var(--muted)] mb-4">{t('labStep3Hint')}</p>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              className="ui-button ui-button-primary"
              disabled={!!busy}
              onClick={() => void runOptimize()}
            >
              {busy === 'optimize' ? t('labRunning') : t('labRunOptimize')}
            </button>
            <button
              type="button"
              className="ui-button ui-button-secondary"
              disabled={!!busy}
              onClick={() => void runCompare()}
            >
              {busy === 'compare' ? t('labRunning') : t('labRunCompareAll')}
            </button>
          </div>
          {optimizeResult.length > 0 && (
            <div className="mt-4 overflow-x-auto">
              <ResultsTable rows={optimizeResult} selected={selected} onSelect={setSelected} t={t} lang={lang} />
            </div>
          )}
        </Card>
      )}

      {step === 4 && (
        <Card>
          <h3 className="font-medium mb-2">{t('labStep4')}</h3>
          <p className="text-xs text-[var(--muted)] mb-2">{t('labPnlNote')}</p>
          {instResult?.production && (
            <p className="text-sm mb-3">
              {t('labProdBaseline')}: PnL {formatNumber(instResult.production.pnl, lang)} ·{' '}
              {instResult.production.trades} {t('labTrades')}
            </p>
          )}
          {compareRows.length === 0 ? (
            <EmptyState>{t('labNoResults')}</EmptyState>
          ) : (
            <ResultsTable rows={compareRows} selected={selected} onSelect={setSelected} t={t} lang={lang} />
          )}
          {selected && (
            <div className="mt-4 p-3 rounded-lg bg-[var(--surface2)] text-sm">
              <div className="font-medium">{selected.strategy} / {selected.mode}</div>
              <div className="text-[var(--muted)]">{paramsStr(selected.params)}</div>
              <div className="mt-2 grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
                <span>PnL: <strong>{formatNumber(selected.pnl, lang)}</strong></span>
                <span>Sharpe: <strong>{formatNumber(selected.sharpe, lang)}</strong></span>
                <span>DD: <strong>{formatNumber(selected.max_drawdown, lang)}</strong></span>
                <span>{t('labTrades')}: <strong>{selected.trades}</strong></span>
              </div>
            </div>
          )}
          <button
            type="button"
            className="ui-button ui-button-primary mt-4"
            disabled={!selected}
            onClick={() => setStep(5)}
          >
            {t('labNext')}
          </button>
        </Card>
      )}

      {step === 5 && (
        <Card>
          <h3 className="font-medium mb-2">{t('labStep5')}</h3>
          {!selected ? (
            <EmptyState>{t('labPickFirst')}</EmptyState>
          ) : (
            <>
              <p className="text-sm mb-3">
                <strong>{instrument?.ticker}</strong> · {selected.strategy} · {paramsStr(selected.params)}
              </p>
              {!selected.live_eligible && (
                <p className="text-sm text-[var(--warning)] mb-3">{t('labDeployBlocked')}</p>
              )}
              <button
                type="button"
                className="ui-button ui-button-primary"
                disabled={!!busy || !selected.live_eligible}
                onClick={() => void deploy()}
              >
                {busy === 'apply' ? t('labRunning') : t('labDeploy')}
              </button>
            </>
          )}
        </Card>
      )}
    </div>
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
            <td>{r.strategy}/{r.mode ?? '—'}</td>
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
