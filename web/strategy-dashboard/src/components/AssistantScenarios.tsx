import type { AssistantScenario } from '../api/types';
import { useI18n } from '../i18n';
import { Badge, Card } from './ui';

interface Props {
  scenarios: AssistantScenario[];
}

function biasTone(bias: string): 'success' | 'danger' | 'warning' | 'info' | 'neutral' {
  if (bias === 'bounce') return 'success';
  if (bias === 'breakout') return 'danger';
  if (bias === 'range') return 'warning';
  return 'neutral';
}

export function AssistantScenarios({ scenarios }: Props) {
  const { t } = useI18n();
  if (!scenarios.length) {
    return null;
  }
  return (
    <Card title={t('assistantScenarios')} subtitle={t('assistantScenariosHelp')}>
      <div className="grid gap-4 md:grid-cols-2">
        {scenarios.map((s) => (
          <div key={s.id} className="rounded-lg border border-[var(--border)] p-4 space-y-2">
            <div className="flex items-start justify-between gap-2">
              <h3 className="font-semibold text-[var(--text)]">{s.title}</h3>
              <Badge tone={biasTone(s.bias)}>{s.bias}</Badge>
            </div>
            <p className="text-xs text-[var(--muted)]">
              {t('assistantProbability')}: {s.probability}
            </p>
            <p className="text-sm">
              <span className="font-medium">{t('assistantTrigger')}:</span> {s.trigger}
            </p>
            <p className="text-sm">
              <span className="font-medium">{t('assistantInvalidation')}:</span> {s.invalidation}
            </p>
            <div className="text-sm whitespace-pre-wrap border-t border-[var(--border)] pt-2">{s.playbook_md}</div>
          </div>
        ))}
      </div>
    </Card>
  );
}
