import type { ButtonHTMLAttributes, ReactNode } from 'react';

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ');
}

export function Card({
  title,
  subtitle,
  actions,
  children,
  className,
}: {
  title?: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={cx('ui-card', className)}>
      {(title || subtitle || actions) && (
        <div className="mb-4 flex items-start justify-between gap-4">
          <div>
            {title && <h2 className="text-base font-semibold text-[var(--text)]">{title}</h2>}
            {subtitle && <p className="mt-1 text-xs text-[var(--muted)]">{subtitle}</p>}
          </div>
          {actions}
        </div>
      )}
      {children}
    </section>
  );
}

export function Button({
  variant = 'secondary',
  className,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'ghost' }) {
  return <button {...props} className={cx('ui-button', `ui-button-${variant}`, className)} />;
}

export function Badge({
  tone = 'neutral',
  children,
}: {
  tone?: 'neutral' | 'success' | 'danger' | 'warning' | 'info';
  children: ReactNode;
}) {
  return <span className={`ui-badge ui-badge-${tone}`}>{children}</span>;
}

export function Tooltip({ text }: { text: ReactNode }) {
  return (
    <span className="ui-tooltip" tabIndex={0} aria-label={typeof text === 'string' ? text : undefined}>
      ?
      <span className="ui-tooltip-panel">{text}</span>
    </span>
  );
}

export function LabelWithHelp({ label, help }: { label: ReactNode; help?: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1">
      {label}
      {help ? <Tooltip text={help} /> : null}
    </span>
  );
}

export function Stat({
  label,
  value,
  help,
  tone = 'neutral',
  sub,
}: {
  label: ReactNode;
  value: ReactNode;
  help?: ReactNode;
  tone?: 'neutral' | 'success' | 'danger' | 'warning' | 'info';
  sub?: ReactNode;
}) {
  return (
    <div className="ui-stat">
      <div className="text-xs text-[var(--muted)]"><LabelWithHelp label={label} help={help} /></div>
      <div className={`mt-1 text-xl font-semibold ui-tone-${tone}`}>{value}</div>
      {sub && <div className="mt-1 text-[11px] text-[var(--muted)]">{sub}</div>}
    </div>
  );
}

export function EmptyState({ children }: { children: ReactNode }) {
  return <div className="rounded-lg border border-dashed border-[var(--border)] p-6 text-center text-sm text-[var(--muted)]">{children}</div>;
}

export function Skeleton() {
  return <div className="h-24 animate-pulse rounded-lg bg-[var(--surface-muted)]" />;
}

export function Table({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-x-auto">
      <table className="ui-table">{children}</table>
    </div>
  );
}

export function Tabs<T extends string>({
  tabs,
  active,
  onChange,
}: {
  tabs: Array<{ id: T; label: ReactNode }>;
  active: T;
  onChange: (id: T) => void;
}) {
  return (
    <div className="ui-tabs">
      {tabs.map((tab) => (
        <Button
          key={tab.id}
          type="button"
          variant={active === tab.id ? 'primary' : 'ghost'}
          onClick={() => onChange(tab.id)}
        >
          {tab.label}
        </Button>
      ))}
    </div>
  );
}
