import { useState } from 'react';
import type { AssistantLevel } from '../api/types';
import { useI18n } from '../i18n';
import { formatNumber } from '../format';
import { Badge } from './ui';

interface Props {
  levels: AssistantLevel[];
  selectedId?: string;
  onSelect: (id: string) => void;
}

export function AssistantLevelAccordion({ levels, selectedId, onSelect }: Props) {
  const { t, lang } = useI18n();
  const [open, setOpen] = useState<Record<string, boolean>>({});

  if (!levels.length) {
    return <p className="text-sm text-[var(--muted)]">{t('assistantNoLevels')}</p>;
  }

  return (
    <div className="space-y-2 max-h-[480px] overflow-y-auto pr-1">
      {levels.map((lv) => {
        const isOpen = open[lv.id] ?? selectedId === lv.id;
        const tone =
          lv.kind === 'mirror' ? 'warning' : lv.kind === 'support' ? 'success' : lv.kind === 'resistance' ? 'danger' : 'info';
        return (
          <div
            key={lv.id}
            className={`rounded-lg border p-3 text-sm cursor-pointer transition-colors ${
              selectedId === lv.id
                ? 'border-[var(--info)] bg-[var(--info-soft)]'
                : 'border-[var(--border)] bg-[var(--surface)]'
            }`}
            onClick={() => onSelect(lv.id)}
          >
            <button
              type="button"
              className="flex w-full items-center justify-between gap-2 text-left"
              onClick={(e) => {
                e.stopPropagation();
                setOpen((o) => ({ ...o, [lv.id]: !isOpen }));
                onSelect(lv.id);
              }}
            >
              <span className="font-mono font-semibold">
                {lv.id} · {formatNumber(lv.price, lang, 4)}
              </span>
              <Badge tone={tone}>{lv.kind}</Badge>
            </button>
            <div className="mt-1 text-xs text-[var(--muted)]">{lv.source}</div>
            <div className="mt-1 flex flex-wrap gap-2 text-xs">
              <span>{t('assistantStrength')}: {lv.strength}/5</span>
              <span>{t('assistantTouches')}: {lv.touches}</span>
            </div>
            {isOpen && (
              <div className="mt-2 space-y-2 border-t border-[var(--border)] pt-2 text-xs">
                {lv.volume_note && <p className="text-[var(--muted)]">{lv.volume_note}</p>}
                {lv.report_md ? (
                  <div className="prose prose-sm dark:prose-invert max-w-none whitespace-pre-wrap">{lv.report_md}</div>
                ) : (
                  <p className="text-[var(--muted)]">{t('assistantNoReport')}</p>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
