
# UptimePing Platform

Продакшен-готовая платформа мониторинга для HTTP/HTTPS, gRPC и GraphQL с поддержкой мультитенантной архитектуры и расширенными уведомлениями.

## Состав платформы

### UptimePing Core
Продакшен-готовый мониторинг для:
- HTTP/HTTPS endpoints
- gRPC сервисов
- GraphQL API
- TCP/SSL проверок

**Возможности**:
- Мультитенантная архитектура (поддержка multiple организаций)
- Планирование проверок по расписанию (cron с поддержкой секундных интервалов)
- Настройка ожидаемых результатов и валидации
- Расширенные уведомления (Telegram, Slack, Email, Webhook)
- Сбор метрик Prometheus и история проверок
- Дедупликация инцидентов и автоматическая эскалация
- Распределенные блокировки для предотвращения дублирования задач
- Rate limiting на уровне API и уведомлений

### UptimePing Forge
Инструмент для автоматической генерации:
- Конфигураций проверок из .proto файлов
- Go кода для проверок gRPC методов
- Тестов и документации
- Интерактивная настройка через веб-интерфейс и CLI

### UptimePing CLI (НОВЫЙ)
Командный интерфейс для управления платформой:
- Управление проверками и конфигурациями через терминал
- Интерактивная аутентификация
- Поддержка всех функций API
- Автодополнение и цветной вывод
- Поддержка Linux, macOS, Windows

## Архитектура

Платформа построена на микросервисной архитектуре с использованием чистой архитектуры (Clean Architecture).

### Микросервисы (10 сервисов)

1. **API Gateway** - единая точка входа, маршрутизация, аутентификация, rate limiting, CORS
2. **Auth Service** - мультитенантная аутентификация (JWT, API Keys), управление пользователями и организациями
3. **Config Service** - управление конфигурациями проверок с валидацией и кешированием
4. **Scheduler Service** - распределенный планировщик проверок с поддержкой cron и приоритетов
5. **Core Service** - выполнение проверок мониторинга с пулом воркеров
6. **Incident Manager** - управление инцидентами с дедупликацией и эскалацией
7. **Notification Service** - мультиканальные уведомления (Telegram, Slack, Email, Webhook)
8. **Forge Service** - генерация конфигураций и кода из .proto файлов
9. **Metrics Service** - сбор, агрегация и экспорт метрик для Prometheus
10. **CLI Service** (НОВЫЙ) - командный интерфейс для управления платформой

### Технологический стек

- **Язык**: Go 1.21+
- **Базы данных**: PostgreSQL 14+, Redis 7+
- **Очереди**: RabbitMQ 3.11+ с поддержкой DLQ
- **API**: REST (HTTP), gRPC (внутренняя коммуникация)
- **Контейнеризация**: Docker, Docker Compose, Kubernetes
- **CI/CD**: GitLab CI/CD, ArgoCD (GitOps)
- **Мониторинг**: Prometheus, Grafana, Loki, Alertmanager
- **Безопасность**: JWT, API Keys, TLS, HashiCorp Vault (опционально)
- **CLI**: Cobra + Viper, поддержка Linux/macOS/Windows

## Быстрый старт

### Требования

- Go 1.21 или выше
- Docker и Docker Compose
- PostgreSQL 14+
- Redis 7+
- RabbitMQ 3.11+
- buf (для работы с protobuf)

### Локальный запуск (все сервисы)

1. Клонировать репозиторий:
```bash
git clone <repository-url>
cd uptimeping-platform
```

2. Запустить инфраструктуру:
```bash
cd deployments/docker-compose
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d
```

3. Применить миграции:
```bash
# Для каждого сервиса
cd services/auth-service
go run cmd/server/main.go migrate

# Или используйте скрипт для всех сервисов
./scripts/migrate.sh all
```

4. Запустить все сервисы:
```bash
# Используйте скрипт для запуска всех сервисов
./scripts/start.sh

# Или запустите отдельно:
# API Gateway (HTTP порт 8080)
cd services/api-gateway && go run cmd/server/main.go

# Auth Service (gRPC порт 50051)
cd services/auth-service && go run cmd/server/main.go

# Config Service (gRPC порт 50052)
cd services/config-service && go run cmd/server/main.go

# Scheduler Service (gRPC порт 50053)
cd services/scheduler-service && go run cmd/server/main.go

# Core Service (gRPC порт 50054)
cd services/core-service && go run cmd/server/main.go

# Incident Manager (gRPC порт 50055)
cd services/incident-manager && go run cmd/server/main.go

# Notification Service (gRPC порт 50056)
cd services/notification-service && go run cmd/server/main.go

# Forge Service (gRPC порт 50057)
cd services/forge-service && go run cmd/server/main.go

# Metrics Service (HTTP порт 9090)
cd services/metrics-service && go run cmd/server/main.go
```

5. Проверить состояние сервисов:
```bash
curl http://localhost:8080/health
curl http://localhost:9090/metrics
```

### Использование CLI (рекомендуемый способ)

1. Установите CLI:
```bash
# Из исходного кода
cd services/cli
go install

# Или скачайте готовый бинарник с GitHub Releases
```

2. Настройте подключение:
```bash
uptimeping config init
```

3. Войдите в систему:
```bash
uptimeping auth login
```

4. Создайте первую проверку:
```bash
# Интерактивный режим
uptimeping config create

# Или из файла
uptimeping config create --file=check.yaml
```

5. Проверьте статус:
```bash
uptimeping checks list
uptimeping checks status <check-id>
```

## Документация

- [Архитектура][1] - детальное описание архитектуры и коммуникации между сервисами
- [Структура сервисов][2] - детальная структура файлов всех сервисов и функции
- [План разработки][3] - детальный план разработки по дням и этапам
- [CLI руководство][4] - полное руководство по использованию CLI
- [API документация][5] - OpenAPI спецификация и примеры запросов
- [Руководство по настройке][6] - инструкции по настройке и развертыванию

## Разработка

### Структура проекта

```
uptimeping-platform/
├── services/              # Все микросервисы
│   ├── api-gateway/      # API Gateway Service
│   ├── auth-service/     # Auth Service
│   ├── config-service/   # Config Service
│   ├── scheduler-service/ # Scheduler Service
│   ├── core-service/     # Core Service (Worker)
│   ├── incident-manager/ # Incident Manager
│   ├── notification-service/ # Notification Service
│   ├── forge-service/    # Forge Service
│   ├── metrics-service/  # Metrics Service
│   └── cli/             # CLI Service (НОВЫЙ)
├── pkg/                  # Общие пакеты (библиотеки)
│   ├── config/          # Конфигурация
│   ├── logger/          # Логирование
│   ├── database/        # PostgreSQL утилиты
│   ├── redis/           # Redis клиент
│   ├── rabbitmq/        # RabbitMQ утилиты
│   ├── metrics/         # Prometheus метрики
│   ├── health/          # Health checks
│   ├── validator/       # Валидация данных
│   ├── security/        # Безопасность
│   ├── http/            # HTTP утилиты
│   ├── grpc/            # gRPC утилиты
│   └── utils/           # Разные утилиты
├── proto/               # gRPC контракты
│   └── api/
│       ├── auth/v1/
│       ├── config/v1/
│       ├── scheduler/v1/
│       ├── core/v1/
│       ├── incident/v1/
│       ├── notification/v1/
│       └── forge/v1/
├── deployments/          # Конфигурации деплоя
│   ├── docker-compose/  # Docker Compose для разработки
│   │   ├── docker-compose.yml
│   │   ├── docker-compose.dev.yml
│   │   ├── docker-compose.test.yml
│   │   └── docker-compose.prod.yml
│   └── k8s/             # Kubernetes манифесты
│       ├── namespaces/
│       ├── configs/
│       ├── networking/
│       ├── monitoring/
│       ├── services/
│       └── jobs/
├── scripts/             # Вспомогательные скрипты
│   ├── setup.sh         # Настройка окружения
│   ├── start.sh         # Запуск всех сервисов
│   ├── stop.sh          # Остановка всех сервисов
│   ├── migrate.sh       # Применение миграций
│   ├── wait-for-db.sh   # Ожидание БД
│   ├── health-check.sh  # Проверка здоровья
│   └── generate-proto.sh # Генерация gRPC кода
├── docs/                # Документация проекта
│   ├── api/            # API документация
│   ├── architecture/   # Архитектурная документация
│   ├── guides/         # Руководства
│   └── api-reference/  # API reference
└── tests/              # Тесты
    ├── unit/           # Unit тесты
    ├── integration/    # Интеграционные тесты
    └── e2e/            # End-to-end тесты
```

### Правила работы с Git

- **Ветки**: feature/... (для новой функциональности), bugfix/... (для исправлений), release/... (для релизов)
- **Коммиты**: Conventional Commits (feat:, fix:, chore:, docs:, test: и т.д.)
- **PR/MR**: Code review обязательно, все тесты должны проходить
- **CI/CD**: автоматический запуск тестов и линтеров при каждом пуше

**Процесс разработки**:
1. Создать ветку от `main`: `git checkout -b feature/service-name`
2. Внести изменения
3. Запустить тесты: `go test ./...`
4. Запустить линтеры: `golangci-lint run`
5. Сделать коммит с описательным сообщением
6. Отправить изменения: `git push origin feature/service-name`
7. Создать Pull/Merge Request
8. Пройти code review
9. Влить изменения в `main`

### Сборка и тестирование

```bash
# Генерация gRPC кода
./scripts/generate-proto.sh

# Запуск всех unit тестов
go test ./services/... ./pkg/...

# Запуск интеграционных тестов
cd tests/integration && go test ./...

# Сборка всех сервисов
go build ./services/...

# Сборка CLI
cd services/cli && go build -o uptimeping

# Линтинг кода
golangci-lint run
```

## Деплой

### Docker Compose (для разработки и тестирования)
```bash
cd deployments/docker-compose
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d
```

### Kubernetes (продакшен)
```bash
# Применение всех манифестов
kubectl apply -f deployments/k8s/

# Или с помощью Helm
helm install uptimeping ./charts/uptimeping
```

### Настройка мониторинга
```bash
# Запуск Prometheus, Grafana, Loki
cd deployments/docker-compose
docker-compose -f monitoring.yml up -d
```

## Особенности платформы

### Мультитенантность
- Полная изоляция данных между организациями
- Ролевая модель доступа (администратор, оператор, наблюдатель)
- Квотирование ресурсов на tenant

### Масштабируемость
- Горизонтальное масштабирование stateless сервисов
- Распределенные блокировки для stateful операций
- Репликация баз данных и очередей

### Отказоустойчивость
- Graceful shutdown всех сервисов
- Retry логика для всех внешних вызовов
- Dead letter queues для обработки ошибок
- Health checks и readiness probes

### Безопасность
- JWT токены с коротким сроком жизни
- API ключи с хешированием секретов
- Rate limiting на уровне API и уведомлений
- Audit logging всех критических операций

### CLI возможности
- Полная функциональность API через командную строку
- Автодополнение для bash, zsh, fish
- Поддержка JSON, YAML, табличного вывода
- Интерактивный режим и wizard для сложных операций
- Пакетирование для всех основных ОС

## Лицензия

MIT License. Смотрите файл [LICENSE][7] для деталей.

## Поддержка

- **Документация**: [docs.uptimeping.io][8]
- **Issues**: [GitHub Issues][9]
- **Discussions**: [GitHub Discussions][10]
- **Безопасность**: security@uptimeping.io

[1]:	./docs/ARCHITECTURE.md
[2]:	./docs/SERVICES_STRUCTURE.md
[3]:	./docs/DEVELOPMENT_PLAN.md
[4]:	./docs/CLI_GUIDE.md
[5]:	./docs/api/
[6]:	./docs/guides/
[7]:	./LICENSE
[8]:	https://docs.uptimeping.io
[9]:	https://github.com/uptimeping/platform/issues
[10]:	https://github.com/uptimeping/platform/discussions