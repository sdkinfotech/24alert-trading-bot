import type { Instance } from '../api/types';
import { useI18n } from '../i18n';
import { Badge } from './ui';

interface Props {
  instances: Instance[];
  selected: string;
  onSelect: (id: string) => void;
}

export function InstanceSelector({ instances, selected, onSelect }: Props) {
  const { t } = useI18n();
  return (
    <div className="flex flex-wrap items-center justify-end gap-3">
      <select
        value={selected}
        onChange={(e) => onSelect(e.target.value)}
        className="rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--text)] focus:outline-none focus:ring-2 focus:ring-[var(--accent)]"
      >
        {instances.map((inst) => (
          <option key={inst.id} value={inst.id}>
            {inst.tickers ? `${inst.tickers} · ` : ''}
            {inst.id} ({inst.type})
          </option>
        ))}
      </select>
      {instances.map((inst) =>
        inst.id === selected ? (
          <div key={inst.id} className="flex flex-wrap items-center gap-2 text-xs text-[var(--muted)]">
            <Badge tone={inst.running ? 'success' : 'danger'}>{inst.running ? t('running') : t('stopped')}</Badge>
            <Badge tone={inst.enabled_in_config ? 'info' : 'warning'}>{inst.enabled_in_config ? t('enabled') : t('disabled')}</Badge>
            <span>{t('account')}: <span className="font-mono text-[var(--text)]">{inst.account_id}</span></span>
            {inst.tickers && (
              <>
                <span className="text-[var(--warning)] font-medium">{inst.tickers}</span>
              </>
            )}
          </div>
        ) : null,
      )}
    </div>
  );
}
