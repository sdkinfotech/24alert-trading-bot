import type { Lang } from './i18n';

const MSK_TZ = 'Europe/Moscow';

/** Missing value placeholder (UTF-8 em dash). */
export const EM_DASH = '—';

export function localeFor(lang: Lang): string {
  return lang === 'ru' ? 'ru-RU' : 'en-US';
}

export function formatMoney(value: number | null | undefined, lang: Lang, currency = 'RUB') {
  if (value == null || !Number.isFinite(value)) return '—';
  return new Intl.NumberFormat(localeFor(lang), {
    style: 'currency',
    currency,
    signDisplay: value === 0 ? 'auto' : 'always',
    maximumFractionDigits: 2,
  }).format(value);
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
