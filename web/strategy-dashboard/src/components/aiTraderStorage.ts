const STORAGE_KEY = '24alert.aiTrader.last';

export interface AiTraderLastSelection {
  account_id: string;
  instrument_uid: string;
  ticker?: string;
  session_id?: string;
}

export function readAiTraderLast(): AiTraderLastSelection | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as AiTraderLastSelection;
    if (!parsed.account_id || !parsed.instrument_uid) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function writeAiTraderLast(sel: AiTraderLastSelection) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(sel));
  } catch {
    /* ignore quota */
  }
}

export function clearAiTraderLast() {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    /* ignore */
  }
}
