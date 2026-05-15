export function SystemGuidePage() {
  return (
    <article className="max-w-3xl space-y-8 text-sm leading-relaxed text-gray-300">
      <section>
        <h2 className="text-lg font-semibold text-white mb-2">Что это за система</h2>
        <p>
          <strong className="text-gray-200">24alert Strategy Dashboard</strong> — веб-интерфейс к
          процессу <code className="text-amber-400/90">strategy-runner</code>: он подключается к
          T-Invest, получает свечи по выбранным инструментам, считает сигналы торговых стратегий,
          проверяет риск и при необходимости выставляет заявки. На экране вы видите состояние
          запущенных инстансов стратегий в реальном времени (данные обновляются автоматически).
        </p>
      </section>

      <section>
        <h2 className="text-lg font-semibold text-white mb-2">Как пользоваться дашбордом</h2>
        <ul className="list-disc pl-5 space-y-2">
          <li>
            <strong className="text-gray-200">Выбор инстанса</strong> — в правом верхнем углу
            выпадающий список только тех стратегий, которые помечены как включённые в конфиге (
            <code className="text-amber-400/90">enabled: true</code>). После смены инстанса
            перезагружаются график, журнал и блок статистики.
          </li>
          <li>
            <strong className="text-gray-200">График</strong> — свечи и визуализация индикаторов
            (SMA, уровни поддержки/сопротивления, ATR и т.д. в зависимости от типа стратегии). Если
            стратегия не отдаёт индикаторные данные, блок может быть пустым.
          </li>
          <li>
            <strong className="text-gray-200">Журнал событий</strong> — хронология сигналов, ордеров
            и исполнений по выбранному инстансу. Удобно проверять, что стратегия «видела» рынок и
            что произошло после сигнала.
          </li>
          <li>
            <strong className="text-gray-200">Статистика</strong> — PnL (реализованный, нереализованный,
            суммарный), позиции по внутреннему ledger и сводка за текущий UTC-день по журналу
            (количество сигналов, ордеров, исполнений).
          </li>
          <li>
            <strong className="text-gray-200">AI-ассистент</strong> — плавающая кнопка чата. Перед
            каждым ответом бэкенд подставляет в контекст актуальные данные: портфель, параметры
            стратегий, цены, индикаторы, последние сигналы и сделки. Можно спросить про баланс,
            настройки стратегии или текущую картину по счёту — ответы опираются на эти данные, а не
            на догадки.
          </li>
        </ul>
      </section>

      <section>
        <h2 className="text-lg font-semibold text-white mb-2">Важно знать</h2>
        <ul className="list-disc pl-5 space-y-2">
          <li>
            Дашборд показывает только инстансы из конфигурации runner. Запуск/остановка и правка
            параметров делаются на стороне сервера (<code className="text-amber-400/90">config.yaml</code>
            , hot-reload API и т.д.) — см. документацию репозитория.
          </li>
          <li>
            Интервал автообновления страницы фиксирован (около 30 секунд). Для мгновенной проверки
            обновите вкладку браузера.
          </li>
          <li>
            Рыночные и брокерские данные зависят от доступности API и прав токена. Если что-то не
            подгрузилось, в интерфейсе или в ответе ассистента будет видно ограничение.
          </li>
        </ul>
      </section>

      <section className="rounded-lg border border-gray-800 bg-gray-900/50 p-4">
        <h2 className="text-lg font-semibold text-white mb-2">Типы стратегий (кратко)</h2>
        <dl className="space-y-2">
          <div>
            <dt className="font-medium text-amber-400/90">sma_crossover</dt>
            <dd>Пересечение скользящих средних по заданным периодам и таймфрейму свечей.</dd>
          </div>
          <div>
            <dt className="font-medium text-amber-400/90">level_bounce</dt>
            <dd>Внутридневная игра от уровней поддержки/сопротивления с ATR для стопов и тейков.</dd>
          </div>
          <div>
            <dt className="font-medium text-amber-400/90">orb_breakout</dt>
            <dd>Пробой утреннего диапазона с контролем фазы и срезом позиций к концу сессии.</dd>
          </div>
          <div>
            <dt className="font-medium text-amber-400/90">grpc</dt>
            <dd>Внешняя стратегия по gRPC — логика на отдельном сервисе.</dd>
          </div>
        </dl>
      </section>
    </article>
  );
}
