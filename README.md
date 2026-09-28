# GophProfile

GophProfile — микросервис для загрузки, хранения, обработки и раздачи пользовательских аватарок. Пользователь загружает одно изображение, после чего сервис сохраняет оригинал и асинхронно создаёт миниатюры размером 100×100 и 300×300.

## Из чего состоит проект

- `server` — принимает HTTP-запросы, проверяет загружаемые файлы, сохраняет метаданные и публикует задания на обработку.
- `worker` — получает задания из RabbitMQ, скачивает оригинал из MinIO, создаёт миниатюры и обновляет статус в PostgreSQL.
- PostgreSQL — хранит метаданные: владельца, имя и размер файла, ключи объектов, размеры изображения и статус обработки.
- MinIO — S3-совместимое хранилище оригиналов и миниатюр. Бинарные файлы не сохраняются в базе данных.
- RabbitMQ — отделяет загрузку изображения от ресурсоёмкой обработки. Благодаря этому API не ждёт создания миниатюр.
- Docker Compose — поднимает все компоненты одной командой и связывает их общей сетью.

## Как проходит загрузка

1. Клиент отправляет изображение на `POST /api/v1/avatars`.
2. Server проверяет `X-User-ID`, размер файла и MIME-тип по содержимому.
3. Оригинал сохраняется в MinIO, а метаданные — в PostgreSQL.
4. Server публикует задание в RabbitMQ и возвращает ответ со статусом `pending`.
5. Worker получает задание и атомарно помечает его как обрабатываемое.
6. Worker создаёт варианты 100×100 и 300×300, сохраняет их в MinIO и устанавливает статус `completed`.
7. Повторно доставленное завершённое задание пропускается. Ошибочные задания повторяются с экспоненциальной задержкой.

## Структура каталогов

```text
cmd/
├── server/             точка входа HTTP-сервера
└── worker/             точка входа фонового обработчика
internal/
├── api/                маршруты, HTTP-обработчики и веб-страницы
├── broker/             адаптер RabbitMQ
├── config/             чтение настроек из переменных окружения
├── domain/             основные сущности и доменные ошибки
├── mocks/              сгенерированные gomock-моки
├── ports/              интерфейсы БД, хранилища и брокера
├── repository/         адаптер PostgreSQL
├── service/            бизнес-логика аватарок
├── storage/            адаптер MinIO/S3
└── worker/             обработка изображений и миниатюр
migrations/             схема PostgreSQL и индексы
Dockerfile              multi-stage сборка server и worker
docker-compose.yml      локальное окружение целиком
.env.example            безопасный пример конфигурации
```

Разделение на `domain`, `ports`, `service` и инфраструктурные адаптеры позволяет тестировать бизнес-логику без реальных PostgreSQL, MinIO и RabbitMQ.

## Настройка окружения

Создайте локальный файл настроек:

```bash
cp .env.example .env
```

На Windows PowerShell:

```powershell
Copy-Item .env.example .env
```

`.env` находится в `.gitignore` и не попадёт в Git. Не записывайте реальные пароли в `.env.example`.

Переменные:

| Переменная | Для чего нужна |
|---|---|
| `POSTGRES_USER` | Пользователь PostgreSQL |
| `POSTGRES_PASSWORD` | Пароль PostgreSQL |
| `POSTGRES_DB` | Имя базы метаданных |
| `MINIO_ROOT_USER` | Логин MinIO и ключ доступа S3 |
| `MINIO_ROOT_PASSWORD` | Пароль MinIO и секретный ключ S3 |
| `RABBITMQ_USER` | Пользователь RabbitMQ |
| `RABBITMQ_PASSWORD` | Пароль RabbitMQ |

Приложения внутри Compose получают из этих значений `DATABASE_URL`, настройки S3 и `RABBITMQ_URL`.

## Запуск

```bash
docker compose up --build
```

После запуска доступны:

- веб-интерфейс загрузки: <http://localhost:8080/web/upload>;
- API: <http://localhost:8080/api/v1>;
- RabbitMQ Management: <http://localhost:15672>;
- MinIO Console: <http://localhost:9001>.

Логины и пароли RabbitMQ и MinIO берутся из `.env`.

Остановить контейнеры, сохранив данные:

```bash
docker compose down
```

Остановить контейнеры и удалить локальные данные PostgreSQL и MinIO:

```bash
docker compose down -v
```

Команда с `-v` необратимо удаляет Docker-тома этого проекта.

## API

| Метод и путь | Назначение |
|---|---|
| `POST /api/v1/avatars` | Загрузить аватарку |
| `GET /api/v1/avatars/{id}` | Получить оригинал или миниатюру |
| `GET /api/v1/avatars/{id}/metadata` | Получить метаданные и статус обработки |
| `DELETE /api/v1/avatars/{id}` | Удалить собственную аватарку |
| `GET /api/v1/users/{user_id}/avatars` | Получить список аватарок пользователя |
| `GET /api/v1/users/{user_id}/avatar` | Получить последнюю аватарку пользователя |
| `DELETE /api/v1/users/{user_id}/avatar` | Удалить последнюю аватарку пользователя |
| `GET /health` | Проверить PostgreSQL, MinIO и RabbitMQ |
| `GET /web/upload` | Открыть форму загрузки |
| `GET /web/gallery/{user_id}` | Открыть галерею пользователя |

### Загрузка

Поддерживаются JPEG, PNG и WebP размером до 10 МБ. Заголовок `X-User-ID` обязателен.

```bash
curl -F "file=@avatar.jpg" \
  -H "X-User-ID: user-1" \
  http://localhost:8080/api/v1/avatars
```

Сразу после загрузки статус обычно равен `pending` или `processing`. Итоговый статус можно получить через endpoint метаданных.

### Получение изображения

Оригинал:

```bash
curl http://localhost:8080/api/v1/avatars/AVATAR_ID --output avatar.jpg
```

Миниатюра:

```bash
curl "http://localhost:8080/api/v1/avatars/AVATAR_ID?size=100x100" --output avatar-small.jpg
```

Допустимые значения `size`: `original`, `100x100` и `300x300`.

### Удаление

Удалять аватарку может только её владелец:

```bash
curl -X DELETE \
  -H "X-User-ID: user-1" \
  http://localhost:8080/api/v1/avatars/AVATAR_ID
```

В PostgreSQL выполняется мягкое удаление, а физическое удаление файлов из MinIO поручается worker через RabbitMQ.

## Тесты и покрытие

Тесты используют `gomock`, поэтому реальные инфраструктурные сервисы для unit-тестов не нужны.

```bash
go test -coverpkg=./internal/... -coverprofile=coverage.out ./internal/...
go tool cover -func=coverage.out
```

HTML-отчёт покрытия:

```bash
go tool cover -html=coverage.out
```

Сгенерировать моки заново после изменения интерфейсов в `internal/ports`:

```bash
go generate ./internal/ports
```

Файл `internal/mocks/mock_ports.go` генерируется автоматически и не должен редактироваться вручную.

## Линтинг

```bash
golangci-lint run
```

Правила находятся в `.golangci.yml`. Также можно отдельно запустить стандартный анализатор Go:

```bash
go vet ./...
```

## Сборка без Docker

```bash
go build -o bin/server ./cmd/server
go build -o bin/worker ./cmd/worker
```

## Observability

The observability stack starts with the application via `docker compose up --build`:

- Grafana: <http://localhost:3000> (`admin`, password from `GRAFANA_PASSWORD`)
- Prometheus: <http://localhost:9090>
- Jaeger: <http://localhost:16686>
- Alertmanager: <http://localhost:9093>
- Loki API: <http://localhost:3100>

Grafana provisions Prometheus, Loki, and Jaeger data sources and the **GophProfile Service Overview** dashboard automatically. It includes RED metrics, business KPIs, PostgreSQL pool utilization, RabbitMQ queue depth, and correlated JSON logs. A log's `trace_id` links to the matching Jaeger trace.

The server exposes `GET /metrics`; the worker exposes metrics on its internal port `9091`. W3C Trace Context propagates from an HTTP request through PostgreSQL, MinIO, and RabbitMQ to the worker. `LOG_LEVEL` controls structured JSON log verbosity.

Prometheus alert rules for error rate, p95 latency, and unavailable targets are in `deploy/prometheus/alerts.yml`. The default Alertmanager receiver is intentionally local; configure a webhook, email, or chat receiver for production notifications.

Для запуска бинарников без Compose дополнительно нужны доступные PostgreSQL, MinIO и RabbitMQ, а также переменные `DATABASE_URL`, `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET` и `RABBITMQ_URL`.
