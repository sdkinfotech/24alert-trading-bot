/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

export type ThemeMode = 'light' | 'dark';

export interface ChartTheme {
  background: string;
  text: string;
  grid: string;
  border: string;
  up: string;
  down: string;
  fast: string;
  slow: string;
  support: string;
  resistance: string;
  brokerAvg: string;
  trailing: string;
  fill: string;
}

const chartThemes: Record<ThemeMode, ChartTheme> = {
  light: {
    background: '#ffffff',
    text: '#475569',
    grid: '#e2e8f0',
    border: '#cbd5e1',
    up: '#15803d',
    down: '#dc2626',
    fast: '#2563eb',
    slow: '#d97706',
    support: '#16a34a',
    resistance: '#dc2626',
    brokerAvg: '#7c3aed',
    trailing: '#ea580c',
    fill: '#9333ea',
  },
  dark: {
    background: '#0f172a',
    text: '#94a3b8',
    grid: '#243044',
    border: '#334155',
    up: '#22c55e',
    down: '#f87171',
    fast: '#60a5fa',
    slow: '#fbbf24',
    support: '#4ade80',
    resistance: '#f87171',
    brokerAvg: '#c4b5fd',
    trailing: '#fb923c',
    fill: '#c084fc',
  },
};

interface ThemeValue {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
  chart: ChartTheme;
}

const ThemeContext = createContext<ThemeValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(() => {
    const stored = localStorage.getItem('24alert.theme');
    if (stored === 'light' || stored === 'dark') return stored;
    return window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  });

  useEffect(() => {
    document.documentElement.dataset.theme = mode;
    document.documentElement.style.colorScheme = mode;
  }, [mode]);

  const value = useMemo<ThemeValue>(() => ({
    mode,
    setMode: (next) => {
      localStorage.setItem('24alert.theme', next);
      setModeState(next);
    },
    chart: chartThemes[mode],
  }), [mode]);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const value = useContext(ThemeContext);
  if (!value) throw new Error('useTheme must be used inside ThemeProvider');
  return value;
}
