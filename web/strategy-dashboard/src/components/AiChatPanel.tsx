import { useState, useEffect, useRef, useCallback } from 'react';
import { api } from '../api/client';
import type { AiChatStatus } from '../api/types';

interface ChatMsg {
  role: 'user' | 'assistant' | 'error';
  text: string;
  time: Date;
}

export function AiChatPanel() {
  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState<AiChatStatus | null>(null);
  const [messages, setMessages] = useState<ChatMsg[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    api.aiChatStatus().then(setStatus).catch(() => {});
  }, []);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  useEffect(() => {
    if (open && inputRef.current) {
      inputRef.current.focus();
    }
  }, [open]);

  const send = useCallback(async () => {
    const msg = input.trim();
    if (!msg || loading) return;
    setInput('');
    setMessages((prev) => [...prev, { role: 'user', text: msg, time: new Date() }]);
    setLoading(true);
    try {
      const res = await api.aiChat(msg);
      if (res.error) {
        setMessages((prev) => [...prev, { role: 'error', text: res.error!, time: new Date() }]);
      } else {
        setMessages((prev) => [
          ...prev,
          { role: 'assistant', text: res.reply ?? '', time: new Date() },
        ]);
      }
    } catch (e: any) {
      setMessages((prev) => [
        ...prev,
        { role: 'error', text: e.message ?? 'Network error', time: new Date() },
      ]);
    } finally {
      setLoading(false);
    }
  }, [input, loading]);

  const handleReset = useCallback(async () => {
    await api.aiChatReset().catch(() => {});
    setMessages([]);
  }, []);

  if (status && !status.available) return null;

  return (
    <>
      <button
        onClick={() => setOpen(!open)}
        className="fixed bottom-5 right-5 z-50 w-12 h-12 rounded-full bg-blue-600 hover:bg-blue-500 text-white shadow-lg flex items-center justify-center transition-all hover:scale-105"
        title="AI Assistant"
      >
        {open ? (
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M5 5L15 15M15 5L5 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          </svg>
        ) : (
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
            <path
              d="M12 2C6.48 2 2 5.58 2 10c0 2.24 1.12 4.27 2.94 5.72L4 20l4.28-2.14C9.47 18.28 10.71 18.5 12 18.5c5.52 0 10-3.58 10-8S17.52 2 12 2z"
              fill="currentColor"
            />
          </svg>
        )}
      </button>

      {open && (
        <div className="fixed bottom-20 right-5 z-50 w-96 max-h-[70vh] bg-gray-900 border border-gray-700 rounded-xl shadow-2xl flex flex-col overflow-hidden">
          <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700 bg-gray-900/95">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full bg-green-500" />
              <span className="text-sm font-semibold text-gray-200">AI Assistant</span>
              <span className="text-[10px] text-gray-500">{status?.model ?? ''}</span>
            </div>
            <div className="flex items-center gap-1">
              {status?.scanner_cron && (
                <span className="text-[9px] bg-blue-900/50 text-blue-400 px-1.5 py-0.5 rounded" title="Autonomous scanner active">
                  CRON
                </span>
              )}
              <button
                onClick={handleReset}
                className="text-gray-500 hover:text-gray-300 text-xs px-1.5 py-0.5 rounded hover:bg-gray-800 transition-colors"
                title="Reset conversation"
              >
                Reset
              </button>
            </div>
          </div>

          <div ref={scrollRef} className="flex-1 overflow-y-auto p-3 space-y-2 min-h-[200px] max-h-[50vh] scrollbar-thin">
            {messages.length === 0 && (
              <div className="text-center text-gray-500 text-xs py-8">
                <p className="mb-2">Спроси о стратегиях, PnL, параметрах</p>
                <div className="space-y-1">
                  {[
                    'Какое текущее состояние стратегий?',
                    'Какой PnL за сегодня?',
                    'Как оптимизировать параметры SMA?',
                  ].map((hint) => (
                    <button
                      key={hint}
                      onClick={() => { setInput(hint); inputRef.current?.focus(); }}
                      className="block w-full text-left text-[11px] text-gray-400 hover:text-blue-400 hover:bg-gray-800/50 px-2 py-1 rounded transition-colors"
                    >
                      {hint}
                    </button>
                  ))}
                </div>
              </div>
            )}
            {messages.map((m, i) => (
              <div key={i} className={`flex ${m.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                <div
                  className={`max-w-[85%] px-3 py-2 rounded-lg text-sm whitespace-pre-wrap break-words ${
                    m.role === 'user'
                      ? 'bg-blue-600 text-white rounded-br-sm'
                      : m.role === 'error'
                        ? 'bg-red-900/40 border border-red-800 text-red-300 rounded-bl-sm'
                        : 'bg-gray-800 text-gray-200 rounded-bl-sm'
                  }`}
                >
                  {m.text}
                </div>
              </div>
            ))}
            {loading && (
              <div className="flex justify-start">
                <div className="bg-gray-800 text-gray-400 px-3 py-2 rounded-lg rounded-bl-sm text-sm">
                  <span className="animate-pulse">Думаю...</span>
                </div>
              </div>
            )}
          </div>

          <div className="px-3 py-2 border-t border-gray-700 bg-gray-900/95">
            <div className="flex gap-2">
              <input
                ref={inputRef}
                type="text"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }}
                placeholder="Напиши сообщение..."
                disabled={loading}
                className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-200 placeholder-gray-500 focus:outline-none focus:border-blue-500 disabled:opacity-50"
              />
              <button
                onClick={send}
                disabled={loading || !input.trim()}
                className="bg-blue-600 hover:bg-blue-500 disabled:opacity-30 disabled:hover:bg-blue-600 text-white px-3 py-2 rounded-lg text-sm transition-colors"
              >
                ➤
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
