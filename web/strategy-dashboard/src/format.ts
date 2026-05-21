import type { Lang } from './i18n';

const MSK_TZ = 'Europe/Moscow';

/** Missing value placeholder (UTF-8 em dash). */
export const EM_DASH = '—';

export function localeFor(lang: Lang): string {
  return lang === 'ru' ? 'ru-RU' : 'en-US';
}

/** MOEX futures often report currency as "pt." (points) — not valid for Intl currency. */
function normalizeCurrency(raw?: string): { iso?: string; suffix?: string } {
  const c = (raw || 'RUB').trim();
  if (!c) return { iso: 'RUB' };
  const key = c.toLowerCase().replace(/\.$/, '');
  if (key === 'pt' || key === 'point' || key === 'points' || key === 'пункт') {
    return { suffix: 'pts' };
  }
  if (key === 'rub' || key === 'rur' || c === '₽') return { iso: 'RUB' };
  try {
    new Intl.NumberFormat('en', { style: 'currency', currency: c });
    return { iso: c };
  } catch {
    /* fall through */
  }
  const upper = c.toUpperCase();
  try {
    new Intl.NumberFormat('en', { style: 'currency', currency: upper });
    return { iso: upper };
  } catch {
    return { suffix: c };
  }
}

export function formatMoney(value: number | null | undefined, lang: Lang, currency = 'RUB') {
  if (value == null || !Number.isFinite(value)) return EM_DASH;
  const locale = localeFor(lang);
  const norm = normalizeCurrency(currency);
  if (norm.iso) {
    return new Intl.NumberFormat(locale, {
      style: 'currency',
      currency: norm.iso,
      signDisplay: value === 0 ? 'auto' : 'always',
      maximumFractionDigits: 2,
    }).format(value);
  }
  const n = new Intl.NumberFormat(locale, {
    signDisplay: value === 0 ? 'auto' : 'always',
    maximumFractionDigits: norm.suffix === 'pts' ? 4 : 2,
  }).format(value);
  return norm.suffix ? `${n} ${norm.suffix}` : n;
}

export function formatNumber(value: number | null | undefined, lang: Lang, digits = 2) {
  if (value == null || !Number.isFinite(value)) return '—';
  return new Intl.NumberFormat(localeFor(lang), {
    maximumFractionDigits: digits,
  }).format(value);
}

export function formatDateTime(value: string | number | null | undefined, lang: Lang) {
  if (value == null) return '—';
  const date = typeof value === 'number' ? new Date(value * 1000) : new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat(localeFor(lang), {
    timeZone: MSK_TZ,
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date);
}
