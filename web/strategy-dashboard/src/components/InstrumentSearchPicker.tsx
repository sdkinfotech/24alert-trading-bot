import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api } from '../api/client';
import type { CatalogInstrument } from '../api/types';
import { useI18n } from '../i18n';

export interface SelectedInstrument {
  uid: string;
  ticker: string;
  name: string;
  kind: string;
}

interface Props {
  value: SelectedInstrument | null;
  onChange: (item: SelectedInstrument | null) => void;
  disabled?: boolean;
}

type KindFilter = 'all' | 'future' | 'share';

function kindLabel(kind: string, t: (k: string) => string): string {
  if (kind === 'future') return t('instrumentKindFuture');
  if (kind === 'share') return t('instrumentKindShare');
  return kind;
}

export function InstrumentSearchPicker({ value, onChange, disabled }: Props) {
  const { t } = useI18n();
  const [query, setQuery] = useState(value?.ticker ?? '');
  const [kind, setKind] = useState<KindFilter>('all');
  const [results, setResults] = useState<CatalogInstrument[]>([]);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const wrapRef = useRef<HTMLDivElement>(null);

  const search = useCallback(async (q: string, k: KindFilter) => {
    setLoading(true);
    try {
      const rows = await api.instrumentCatalog(q, k, 50);
      setResults(rows);
      setError(null);
    } catch (e) {
      setResults([]);
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    const timer = window.setTimeout(() => void search(query, kind), 250);
    return () => window.clearTimeout(timer);
  }, [open, query, kind, search]);

  useEffect(() => {
    if (value?.ticker) setQuery(value.ticker);
  }, [value?.uid, value?.ticker]);

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, []);

  const hint = useMemo(() => {
    if (loading) return t('instrumentSearchLoading');
    if (error) return error;
    if (open && results.length === 0) return t('instrumentSearchEmpty');
    return t('instrumentSearchHint');
  }, [loading, error, open, results.length, t]);

  function pick(item: CatalogInstrument) {
    onChange({
      uid: item.uid,
      ticker: item.ticker,
      name: item.name,
      kind: item.kind,
    });
    setQuery(item.ticker);
    setOpen(false);
  }

  return (
    <div ref={wrapRef} className="instrument-search relative">
      <div className="mb-2 flex flex-wrap gap-2">
        {(['all', 'future', 'share'] as const).map((k) => (
          <button
            key={k}
            type="button"
            disabled={disabled}
            onClick={() => {
              setKind(k);
              setOpen(true);
            }}
            className={`rounded-full border px-3 py-1 text-xs font-medium transition ${
              kind === k
                ? 'border-[var(--accent)] bg-[var(--accent)]/15 text-[var(--accent)]'
                : 'border-[var(--border)] text-[var(--muted)] hover:border-[var(--accent)]'
            }`}
          >
            {k === 'all' ? t('instrumentKindAll') : kindLabel(k, t)}
          </button>
        ))}
      </div>
      <input
        type="text"
        disabled={disabled}
        value={query}
        placeholder={t('instrumentSearchPlaceholder')}
        onFocus={() => {
          setOpen(true);
          void search(query, kind);
        }}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
          if (!e.target.value.trim()) onChange(null);
        }}
        className="w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--text)] outline-none focus:ring-2 focus:ring-[var(--accent)]"
      />
      <p className="mt-1 text-xs text-[var(--muted)]">{hint}</p>
      {value && (
        <p className="mt-1 text-xs text-[var(--text)]">
          <span className="font-semibold">{value.ticker}</span>
          {' · '}
          {kindLabel(value.kind, t)}
          {value.name ? ` · ${value.name}` : ''}
          <span className="ml-2 font-mono text-[var(--muted)]">{value.uid.slice(0, 12)}…</span>
        </p>
      )}
      {open && results.length > 0 && (
        <ul className="instrument-search-results">
          {results.map((item) => (
            <li key={item.uid}>
              <button type="button" className="instrument-search-item" onClick={() => pick(item)}>
                <span className="font-mono font-semibold">{item.ticker}</span>
                <span className="text-[var(--muted)]">{kindLabel(item.kind, t)}</span>
                <span className="truncate text-xs text-[var(--muted)]">{item.name || item.class_code}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
