# ugubot

XMPP-бот, который ведёт архив комнат и личных переписок, отвечает через OpenAI-совместимую модель и показывает историю в веб-интерфейсе.

## Архитектура

Четыре сервиса общаются через NATS JetStream, данные лежат в PostgreSQL. Все сервисы собраны в один бинарник `ugubot`: запускать можно по отдельности или одним процессом.

```
Браузер ──WebSocket──▶ web-gateway ──┐
                                     │      ┌──────────── NATS JetStream ────────────┐
XMPP-сервер ◀──▶ xmpp-gateway ───────┼─────▶│ ugubot.xmpp.event.*   события из чатов │
                                     │      │ ugubot.xmpp.cmd.send  команды отправки │
                 ai-worker ──────────┤      │ ugubot.msg.stored.*   сохранённые      │
                    │                │      │ ugubot.history.*      запросы к архиву │
                 OpenAI API          │      └────────────────────────────────────────┘
                                     │
                 history ────────────┘──▶ PostgreSQL (схемы chat и ai)
```

| Сервис | Что делает |
|---|---|
| `xmpp-gateway` | Единственная XMPP-сессия: комнаты (в том числе с паролем), личные сообщения, подписки, IQ version, переподключение. Превращает станзы в события и отправляет сообщения по командам. С базой не работает. |
| `history` | Владеет схемой `chat`: сохраняет события, публикует `msg.stored`, отвечает на запросы о чатах, сообщениях, датах и цветах ников. |
| `ai-worker` | Цепочка middleware с командами `~`, контекст по чатам, вызовы модели. Владеет схемой `ai`: usage, blocklist, прелюдии. |
| `web-gateway` | Вход по паролю, раздача фронтенда, WebSocket API, пуш новых сообщений. |

Порядок обработки сообщения:

1. `xmpp-gateway` публикует событие.
2. `history` сохраняет его и публикует `msg.stored`.
3. `web-gateway` рассылает сообщение в браузеры, а `ai-worker` решает, нужно ли отвечать.
4. Ответ AI уходит командой `xmpp.cmd.send`, проходит этот же путь и попадает в архив. Usage связывается с сообщением по `reply_for`.

### Даты в календаре

Список дней с сообщениями считается для таймзоны браузера одним запросом (`internal/history/store.go`, `Store.Dates`). Запрос прыгает по индексу `(chat_id, ts)` от одной локальной полуночи к следующей, поэтому его стоимость зависит от числа дней с сообщениями, а не от числа сообщений. Кэш не нужен, Redis тоже.

## Запуск

```sh
cp settings.toml.example settings.toml   # заполнить
```

**Одним процессом** со встроенным NATS (`nats.url = "embedded"`), нужен только PostgreSQL:

```sh
cd web && npm ci && npm run build && cd ..
go run ./cmd/ugubot all
```

**Сервисами в Docker**: postgres, nats и четыре контейнера из одного образа:

```sh
docker compose up -d --build
```

Отдельный сервис запускается так: `ugubot [-config settings.toml] history|xmpp-gateway|ai-worker|web-gateway`.

### Переезд с Python-версии

Нужно указать ту же базу PostgreSQL (`[database]` в старом формате тоже читается). При первом запуске `history` и `ai-worker` скопируют данные из таблиц Pony (`chat`, `message`, `nickcolor`, `aiusage`, `aiprelude`, `blockedusers`) в схемы `chat` и `ai`. Старые таблицы при этом не трогаются. Сессии веб-интерфейса остаются действительными. Redis больше не нужен.

Формат `settings.toml` прежний. Из нового:

- `xmpp.host` подключает к серверу напрямую, без SRV;
- `openai.base_url` позволяет использовать любой OpenAI-совместимый API;
- `openai.prices` задаёт цены моделей;
- `database.url` задаёт подключение к базе строкой;
- секции `[nats]` и `[log]`.

Секции `[redis]` и `[logging]` больше не используются.

## Разработка

```sh
go test ./...                       # unit-тесты
UGUBOT_TEST_DATABASE_URL=postgres://postgres:x@localhost/postgres go test ./internal/history/   # с PostgreSQL
test/e2e/run.sh                     # e2e в Docker: Prosody + PostgreSQL + бот + фейковый OpenAI
MODE=split test/e2e/run.sh          # то же, сервисы в отдельных контейнерах с внешним NATS
```

Фронтенд написан на Svelte 5 + TypeScript и лежит в `web/`. Сборка попадает в `internal/web/dist` и встраивается в бинарник. Для разработки запустите `npm run dev` в `web/`: Vite проксирует `/api` и `/ws` на `localhost:8000`. В этом режиме у бэкенда должен быть включён `webui.debug = true`, иначе он отклонит WebSocket с чужого origin.
