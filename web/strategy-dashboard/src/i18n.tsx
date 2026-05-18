/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';

export type Lang = 'ru' | 'en';

type Dict = Record<string, string>;

const ru: Dict = {
  appTitle: '24alert Strategy Dashboard',
  overview: 'Обзор',
  chart: 'График',
  portfolio: 'Портфель',
  history: 'История',
  guide: 'Справка',
  aiTrader: 'AI Trader',
  aiTraderHelp: 'Безопасный observe/paper режим: смотрит стакан, считает microstructure features и пишет решения без real orders.',
  startObserve: 'Старт observe',
  startPaper: 'Старт paper',
  stopSession: 'Остановить сессию',
  instruction: 'Инструкция',
  noAiTraderSession: 'AI Trader session ещё не запущена',
  lastDecision: 'Последнее решение',
  orderbookFeatures: 'Фичи стакана',
  imbalance: 'Imbalance',
  spread: 'Spread',
  freshness: 'Freshness',
  bidWall: 'Bid wall',
  askWall: 'Ask wall',
  language: 'Язык',
  theme: 'Тема',
  light: 'Светлая',
  dark: 'Тёмная',
  autoRefresh: 'Автообновление',
  lastUpdate: 'Последнее обновление',
  running: 'Работает',
  stopped: 'Остановлена',
  enabled: 'Включена',
  disabled: 'Выключена',
  brokerTruth: 'Broker truth',
  brokerTruthHelp: 'Фактические позиции и доходность, полученные напрямую из портфеля T-Invest.',
  runnerLedger: 'Runner ledger',
  runnerLedgerHelp: 'Локальная книга исполнений strategy-runner. Нужна для сверки, но broker truth главнее.',
  strategyState: 'Strategy state',
  strategyStateHelp: 'Внутреннее состояние стратегии: LONG, SHORT или FLAT.',
  totalPnl: 'Итог PnL',
  realized: 'Реализовано',
  unrealized: 'Открытый PnL',
  expectedYield: 'Ожидаемая доходность',
  expectedYieldHelp: 'Оценка брокера по открытому портфелю. Для фьючерсов это главный источник PnL.',
  positions: 'Позиции',
  noPositions: 'Открытых позиций нет',
  signals: 'Сигналы',
  orders: 'Ордера',
  executions: 'Исполнения',
  stopOrders: 'Стоп-заявки',
  events: 'События',
  all: 'Все',
  protectiveStop: 'Protective stop',
  protectiveStopHelp: 'Broker-side STOP_LOSS, который runner ставит после входа в позицию.',
  trailingStop: 'Trailing stop',
  trailingStopHelp: 'Software trailing внутри стратегии. Он закрывает позицию market-ордером при пробое уровня.',
  watchdogFlatten: 'Watchdog flatten',
  watchdogFlattenHelp: 'Аварийное закрытие broker position при превышении лимитов риска.',
  signalCancelled: 'Сигнал отменён',
  signalCancelledHelp: 'Сигнал был рассчитан стратегией, но не был отправлен брокеру из-за session/risk/order guard.',
  account: 'Счёт',
  instrument: 'Инструмент',
  quantity: 'Кол-во',
  average: 'Средняя',
  current: 'Текущая',
  status: 'Статус',
  source: 'Источник',
  flat: 'FLAT',
  open: 'Открыто',
  unavailable: 'Недоступно',
  noEvents: 'Событий пока нет',
  noStopOrders: 'Активных broker stop orders нет',
  riskWarnings: 'Риски и защита',
  portfolioTotals: 'Итоги портфеля',
  cash: 'Валюта',
  futures: 'Фьючерсы',
  shares: 'Акции',
  bonds: 'Облигации',
  etf: 'ETF',
  guideText: 'Эта панель показывает состояние стратегий, портфель брокера, локальный ledger, историю сигналов/ордеров/исполнений и защитные стопы.',
};

const en: Dict = {
  appTitle: '24alert Strategy Dashboard',
  overview: 'Overview',
  chart: 'Chart',
  portfolio: 'Portfolio',
  history: 'History',
  guide: 'Guide',
  aiTrader: 'AI Trader',
  aiTraderHelp: 'Safe observe/paper mode: watches order book, computes microstructure features, and journals decisions without real orders.',
  startObserve: 'Start observe',
  startPaper: 'Start paper',
  stopSession: 'Stop session',
  instruction: 'Instruction',
  noAiTraderSession: 'No AI Trader session started yet',
  lastDecision: 'Last decision',
  orderbookFeatures: 'Order book features',
  imbalance: 'Imbalance',
  spread: 'Spread',
  freshness: 'Freshness',
  bidWall: 'Bid wall',
  askWall: 'Ask wall',
  language: 'Language',
  theme: 'Theme',
  light: 'Light',
  dark: 'Dark',
  autoRefresh: 'Auto refresh',
  lastUpdate: 'Last update',
  running: 'Running',
  stopped: 'Stopped',
  enabled: 'Enabled',
  disabled: 'Disabled',
  brokerTruth: 'Broker truth',
  brokerTruthHelp: 'Actual positions and yield received directly from the T-Invest portfolio.',
  runnerLedger: 'Runner ledger',
  runnerLedgerHelp: 'Local strategy-runner execution ledger. Useful for reconciliation, but broker truth wins.',
  strategyState: 'Strategy state',
  strategyStateHelp: 'Internal strategy position state: LONG, SHORT, or FLAT.',
  totalPnl: 'Total PnL',
  realized: 'Realized',
  unrealized: 'Open PnL',
  expectedYield: 'Expected yield',
  expectedYieldHelp: 'Broker-estimated portfolio yield. For futures this is the main PnL source.',
  positions: 'Positions',
  noPositions: 'No open positions',
  signals: 'Signals',
  orders: 'Orders',
  executions: 'Executions',
  stopOrders: 'Stop orders',
  events: 'Events',
  all: 'All',
  protectiveStop: 'Protective stop',
  protectiveStopHelp: 'Broker-side STOP_LOSS created by the runner after position entry.',
  trailingStop: 'Trailing stop',
  trailingStopHelp: 'Software trailing stop inside the strategy. It exits with a market order when breached.',
  watchdogFlatten: 'Watchdog flatten',
  watchdogFlattenHelp: 'Emergency broker-position close when risk limits are breached.',
  signalCancelled: 'Signal cancelled',
  signalCancelledHelp: 'The strategy produced a signal, but session/risk/order guard blocked broker dispatch.',
  account: 'Account',
  instrument: 'Instrument',
  quantity: 'Quantity',
  average: 'Average',
  current: 'Current',
  status: 'Status',
  source: 'Source',
  flat: 'FLAT',
  open: 'Open',
  unavailable: 'Unavailable',
  noEvents: 'No events yet',
  noStopOrders: 'No active broker stop orders',
  riskWarnings: 'Risk and protection',
  portfolioTotals: 'Portfolio totals',
  cash: 'Cash',
  futures: 'Futures',
  shares: 'Shares',
  bonds: 'Bonds',
  etf: 'ETF',
  guideText: 'This panel shows strategy status, broker portfolio, runner ledger, signal/order/fill history, and protective stops.',
};

const dictionaries: Record<Lang, Dict> = { ru, en };

interface I18nValue {
  lang: Lang;
  setLang: (lang: Lang) => void;
  t: (key: string) => string;
}

const I18nContext = createContext<I18nValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLang] = useState<Lang>(() => {
    const stored = localStorage.getItem('24alert.lang');
    return stored === 'en' ? 'en' : 'ru';
  });

  const value = useMemo<I18nValue>(() => ({
    lang,
    setLang: (next) => {
      localStorage.setItem('24alert.lang', next);
      setLang(next);
    },
    t: (key) => dictionaries[lang][key] ?? key,
  }), [lang]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) throw new Error('useI18n must be used inside I18nProvider');
  return value;
}
