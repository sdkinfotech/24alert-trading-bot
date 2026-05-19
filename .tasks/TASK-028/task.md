---
ticket_id: TASK-028
slug: TASK-028
title: "AI Trader level intraday integration"
status: done
priority: high
complexity: L
phase: 2
created: 2026-05-19
---

# TASK-028 · AI Trader: уровневая intraday-интеграция

## Цель

Перестроить AI Trader в пофазный режим «уровни + стакан + лента»:

`collecting` (≥60s буфер) → `analyzing` (micro-LLM + advisor 5m/15m/1h) → `ready` (`trading_ready` + level playbook) → `trading` (paper-лимитки у уровней).

Убрать UX `observe` / `Start Paper`. Дефолтный инструмент в UI: **BMM6** (Brent mini).

## Критерии готовности

- [x] Фазы сессии в API + runner tick
- [x] Блокировка micro-LLM до 60s collect
- [x] `POST .../start-trading` только из `ready`
- [x] Level playbook (LB daily + DOM/footprint)
- [x] Advisor readiness + retry только running sessions
- [x] Paper engine + `alert24_ai_trader_orders_total`
- [x] Dashboard: мониторинг + пульсирующая «Начать торговлю»
- [x] `docs/AI_TRADER_SCALPER.md` обновлён

## Non-goals (v2)

- Live broker limits через `order.Service`
- Отдельный `advisor-svc` перенос AI Trader control plane
