# 🏎️ SimTelemetry Hub

**SimTelemetry Hub** — высокопроизводительный backend-сервис на Go (Golang) для сбора, конкурентной обработки и аналитики телеметрии гоночных сессий (симуляторы Assetto Corsa, iRacing, rFactor 2 и др.).

Проект разработан с прицелом на высокий throughput (пропускную способность) при отправке телеметрии в реальном времени, использует конкурентную обработку с помощь Worker Pool, безопасный in-memory кэш с `sync.RWMutex`, базу данных PostgreSQL и чистую архитектуру.

---

## 🏗 Архитектура проекта

```text
[Race Simulator / Client]
         │
         │ POST /api/v1/telemetry (JSON)
         ▼
 ┌────────────────┐       Submit Job       ┌──────────────────────────────────────┐
 │  HTTP Router   │ ────────────────────►  │         Worker Pool (Goroutines)     │
 │  (chi / HTTP)  │  202 Accepted (Fast)   │ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
 └────────────────┘                        │ │ Worker 1 │ │ Worker 2 │ │ Worker N │ │
                                           │ └────┬─────┘ └────┬─────┘ └────┬─────┘ │
                                           └──────┼────────────┼────────────┼───────┘
                                                  │            │            │
                                     Lock / Write │            │            │ PostgreSQL
                                                  ▼            ▼            ▼
                                          ┌──────────────┐   ┌──────────────────────┐
                                          │ MemoryCache  │   │     PostgreSQL       │
                                          │ sync.RWMutex │   │ (telemetry, leader)  │
                                          └──────────────┘   └──────────────────────┘
```

### Структура папок

```text
cmd/
  api/
    main.go              # Точка входа приложения, сборка зависимостей, Graceful Shutdown
internal/
  config/
    config.go            # Чтение переменных окружения и конфигурация сервиса
  handler/
    telemetry.go         # HTTP контроллеры (POST telemetry, GET leaderboard, GET health)
    middleware.go        # Логирование, паник-рекавери, JSON заголовки
  service/
    telemetry.go         # Бизнес-логика и валидация
    workerpool.go        # Worker Pool, in-memory cache с sync.RWMutex, очередей задач
  repository/
    postgres.go          # Запросы к PostgreSQL (Telemetry & Leaderboards)
    models.go            # Data Models (TelemetryPayload, LeaderboardEntry)
pkg/
  database/
    postgres.go          # Инициализация пула соединений PostgreSQL (database/sql)
migrations/
  000001_init.up.sql     # SQL миграции базы данных
Dockerfile               # Многоэтапная сборка образа Go
docker-compose.yml       # Оркестрация сервисов Go API + PostgreSQL
.env.example             # Пример конфигурации переменных окружения
go.mod                   # Зависимости Go
README.md                # Документация проекта
```

---

## 🛠 Технологический стек

* **Язык:** Go 1.22+
* **HTTP Роутер:** `github.com/go-chi/chi/v5`
* **База данных:** PostgreSQL 16 (`database/sql` + драйвер `github.com/lib/pq`)
* **Конкурентность:** Кастомный Worker Pool на горутинах и буферизированных каналах (`sync.WaitGroup`, `sync.RWMutex`, `context.Context`)
* **Контейнеризация:** Docker & Docker Compose

---

## ⚙️ Особенности реализации многопоточности (Worker Pool)

1. **Неблокирующий прием запросов (`POST /api/v1/telemetry`):**
   HTTP-хэндлер мгновенно отправляет `TelemetryPayload` в буферизированный канал `jobs` и возвращает клиенту `202 Accepted`. Время ответа сервера — единицы миллисекунд.
2. **In-Memory Cache быстрых кругов (`sync.RWMutex`):**
   Воркеры проверяют и обновляют лучшие времена кругов в in-memory структуре `MemoryCache` с применением RLock/Lock паттерна. Это исключает частые лишние записи в БД.
3. **Асинхронное сохранение в PostgreSQL:**
   Воркеры параллельно выполняют запись исходных телеметрических данных и обновления таблицы `leaderboard`.
4. **Graceful Shutdown:**
   При перехвате сигналов `SIGINT` / `SIGTERM` сервер прекращает прием новых запросов (`http.Server.Shutdown`), закрывает канал задач `jobs`, а воркеры дорабатывают оставшиеся задачи из очереди благодаря `sync.WaitGroup`.

---

## 🚀 Быстрый запуск

### Требования
* Docker и Docker Compose

### 1. Запуск через Docker Compose

```bash
# Клонируйте репозиторий и перейдите в директорию
cd simtelemetry-hub

# Соберите и запустите контейнеры
docker-compose up --build
```

Сервер будет доступен по адресу `http://localhost:8080`.

---

## 🧪 Примеры API запросов (cURL)

### 1. Проверка работоспособности (`GET /api/v1/health`)

```bash
curl -X GET http://localhost:8080/api/v1/health
```

**Ответ `200 OK`:**
```json
{
  "status": "healthy",
  "message": "SimTelemetry Hub service is fully operational",
  "data": {
    "database": "connected",
    "pending_jobs": 0,
    "worker_pool": 10
  }
}
```

---

### 2. Отправка пакета телеметрии (`POST /api/v1/telemetry`)

```bash
curl -X POST http://localhost:8080/api/v1/telemetry \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "sess_98234",
    "driver_name": "Max Verstappen",
    "car_model": "Red Bull RB20",
    "track_name": "Monza",
    "lap_number": 5,
    "lap_time": 81.045,
    "speed": 342.50,
    "sector1_time": 26.120,
    "sector2_time": 27.310,
    "sector3_time": 27.615,
    "incident_flags": 0
  }'
```

**Ответ `202 Accepted`:**
```json
{
  "status": "processing",
  "message": "telemetry payload accepted for asynchronous processing"
}
```

---

### 3. Отправка дополнительных данных (для наполнения таблицы лидеров)

```bash
curl -X POST http://localhost:8080/api/v1/telemetry \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "sess_98235",
    "driver_name": "Charles Leclerc",
    "car_model": "Ferrari SF-24",
    "track_name": "Monza",
    "lap_number": 8,
    "lap_time": 80.912,
    "speed": 345.10,
    "sector1_time": 26.010,
    "sector2_time": 27.280,
    "sector3_time": 27.622,
    "incident_flags": 0
  }'
```

---

### 4. Получение таблицы лидеров трассы (`GET /api/v1/leaderboard`)

```bash
curl -X GET "http://localhost:8080/api/v1/leaderboard?track=Monza&limit=10"
```

**Ответ `200 OK`:**
```json
{
  "status": "success",
  "data": [
    {
      "rank": 1,
      "track_name": "Monza",
      "driver_name": "Charles Leclerc",
      "car_model": "Ferrari SF-24",
      "best_lap_time": 80.912,
      "updated_at": "2026-08-28T16:50:00Z"
    },
    {
      "rank": 2,
      "track_name": "Monza",
      "driver_name": "Max Verstappen",
      "car_model": "Red Bull RB20",
      "best_lap_time": 81.045,
      "updated_at": "2026-08-28T16:49:00Z"
    }
  ]
}
```

---

## 🔧 Локальный запуск без Docker (для разработки)

```bash
# Убедитесь, что PostgreSQL запущен локально
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=simuser
export DB_PASSWORD=simsecret
export DB_NAME=simtelemetry

# Запуск миграций вручную (или через psql)
psql -h localhost -U simuser -d simtelemetry -f migrations/000001_init.up.sql

# Запуск API сервера
go run cmd/api/main.go
```
