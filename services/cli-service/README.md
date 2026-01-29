# UptimePing CLI Service

CLI инструмент для управления мониторингом доступности сервисов UptimePing Platform.

## Установка

```bash
go build -o uptimeping .
```

## Использование

### Инициализация конфигурации

```bash
# Создать конфигурацию по умолчанию
./uptimeping config init

# Просмотреть текущую конфигурацию
./uptimeping config view
```

### Управление проверками

#### Создание проверки

```bash
# Создать HTTP проверку
./uptimeping config create \
  --name "Google Homepage" \
  --type http \
  --target https://google.com \
  --interval 60 \
  --timeout 10 \
  --tags production,web

# Создать TCP проверку
./uptimeping config create \
  --name "Database Server" \
  --type tcp \
  --target localhost:5432 \
  --interval 30 \
  --timeout 5 \
  --tags database,production
```

#### Получение информации о проверке

```bash
# Получить проверку по ID
./uptimeping config get check-12345
```

#### Обновление проверки

```bash
# Обновить интервал проверки
./uptimeping config update check-12345 \
  --interval 120

# Изменить теги
./uptimeping config update check-12345 \
  --tags updated,production

# Отключить проверку
./uptimeping config update check-12345 \
  --enabled false
```

#### Список проверок

```bash
# Показать все проверки
./uptimeping config list

# Фильтрация по тегам
./uptimeping config list --tags production,web

# Только активные проверки
./uptimeping config list --enabled true

# Пагинация
./uptimeping config list --page 1 --limit 20
```

### Запуск и мониторинг проверок

#### Ручной запуск проверки

```bash
# Запустить проверку
./uptimeping checks run check-12345
```

#### Статус проверки

```bash
# Получить текущий статус
./uptimeping checks status check-12345
```

#### История проверок

```bash
# Показать историю выполнения
./uptimeping checks history check-12345

# С пагинацией
./uptimeping checks history check-12345 --page 1 --limit 10

# В формате JSON
./uptimeping checks history check-12345 --format json
```

#### Список всех проверок

```bash
# Показать список всех проверок
./uptimeping checks list

# С фильтрацией
./uptimeping checks list --tags web --enabled true
```

## Конфигурация

### Файл конфигурации `~/.uptimeping/config.yaml`

```yaml
api:
  base_url: "http://localhost:8080"
  timeout: 30

grpc:
  scheduler_address: "localhost:50051"  # Scheduler Service gRPC порт
  core_address: "localhost:50052"      # Core Service gRPC порт
  use_grpc: true                       # Включить gRPC режим
  timeout: 30                          # Таймаут gRPC вызов в секундах

auth:
  token_expiry: 3600        # Время жизни токена в секундах (1 час)
  refresh_threshold: 300  # Порог обновления токена в секундах (5 минут)

output:
  format: "table"  # Формат вывода: table, json, yaml
  colors: true     # Использовать цвета в выводе

current_tenant: ""  # Текущий тенант (опционально)
```

### Режимы работы

#### Mock режим (по умолчанию)
- Использует заглушки для всех операций
- Подходит для разработки и тестирования
- Не требует запущенных сервисов

#### gRPC режим
- Использует реальные gRPC вызовы к сервисам
- Требует запущенные Scheduler Service и Core Service
- Включается установкой `use_grpc: true` в конфигурации

## Аутентификация

```bash
# Вход в систему
./uptimeping auth login --email user@example.com --password password123

# Регистрация
./uptimeping auth register --email user@example.com --password password123 --tenant-name "My Company"

# Статус аутентификации
./uptimeping auth status

# Выход
./uptimeping auth logout
```

## Примеры использования

### Полный цикл создания и мониторинга проверки

```bash
# 1. Инициализация
./uptimeping config init

# 2. Вход в систему
./uptimeping auth login --email admin@example.com --password admin123

# 3. Создание проверки
./uptimeping config create \
  --name "API Endpoint" \
  --type http \
  --target https://api.example.com/health \
  --interval 60 \
  --timeout 10 \
  --tags api,production

# 4. Запуск проверки
./uptimeping checks run check-generated-id

# 5. Проверка статуса
./uptimeping checks status check-generated-id

# 6. Просмотр истории
./uptimeping checks history check-generated-id --limit 5

# 7. Обновление проверки
./uptimeping config update check-generated-id --interval 30

# 8. Просмотр всех проверок
./uptimeping checks list --tags production
```

## Интеграция с gRPC сервисами

### Запуск сервисов

```bash
# Запуск Scheduler Service (порт 50051)
go run services/scheduler-service/main.go --grpc-port=50051

# Запуск Core Service (порт 50052)  
go run services/core-service/main.go --grpc-port=50052
```

### Настройка CLI для gRPC

```bash
# Обновить конфигурацию для gRPC
./uptimeping config update --use-grpc true

# Или вручную отредактировать ~/.uptimeping/config.yaml
```

### Пример gRPC вызов

```bash
# Создание проверки через gRPC
./uptimeping config create \
  --name "gRPC Test" \
  --type http \
  --target https://example.com \
  --interval 60

# В логах будет видно:
# INFO: подключено к Scheduler Service {"service": "cli-service", "address": "localhost:50051"}
# INFO: подключено к Core Service {"service": "cli-service", "address": "localhost:50052"}
# INFO: создание проверки через gRPC {"service": "cli-service", "name": "gRPC Test", "type": "http"}
# INFO: проверка создана через gRPC {"service": "cli-service", "check_id": "check-12345"}
```

## Форматы вывода

### Table формат (по умолчанию)

```
✅ Проверка запущена!
🔍 ID проверки: check-12345
🆔 ID выполнения: exec-67890
⏰ Время запуска: 2026-01-28 16:30:15
📊 Статус: success
💬 Сообщение: Проверка выполнена успешно
```

### JSON формат

```json
{
  "execution_id": "exec-67890",
  "status": "success",
  "message": "Проверка выполнена успешно",
  "started_at": "2026-01-28T16:30:15Z",
  "check_id": "check-12345"
}
```

## Ошибки и устранение

### Ошибка: "gRPC не настроен"

**Причина**: Попытка использовать gRPC без настройки `use_grpc: true`

**Решение**:
```bash
# Включить gRPC режим
./uptimeping config update --use-grpc true

# Или использовать mock режим (по умолчанию)
```

### Ошибка: "Scheduler Service не доступен"

**Причина**: gRPC сервис не запущен или недоступен

**Решение**:
```bash
# Проверить, что сервис запущен
lsof -i :50051

# Запустить сервис
go run services/scheduler-service/main.go --grpc-port=50051
```

### Ошибка аутентификации

**Причина**: Токен истек или недействителен

**Решение**:
```bash
# Перелогиниться
./uptimeping auth login --email user@example.com --password password123
```

## Тестирование

### Запуск тестов

```bash
# Запустить все тесты
go test ./...

# Запустить тесты с покрытием
go test -cover ./...

# Запустить тесты конкретного пакета
go test ./internal/client/...
```

### Mock тесты

CLI включает comprehensive mock тесты для всех операций:

```bash
# Запустить тесты клиента
go test ./internal/client/ -v
```

## Разработка

### Структура проекта

```
services/cli-service/
├── cmd/                    # CLI команды
│   ├── auth.go            # Команды аутентификации
│   ├── checks.go          # Команды проверок
│   ├── config.go          # Команды конфигурации
│   └── root.go            # Корневая команда
├── internal/
│   ├── client/            # Клиенты API
│   │   ├── config_client.go      # Клиент конфигурации
│   │   ├── grpc_client.go        # gRPC клиент
│   │   └── config_client_test.go # Тесты клиента
│   ├── config/            # Конфигурация CLI
│   └── auth/              # Аутентификация
├── examples/              # Примеры использования
└── docs/                  # Документация
```

### Добавление новых команд

1. Создать файл в `cmd/`
2. Определить команду с Cobra
3. Добавить в `root.go`
4. Реализовать обработчики

### Добавление новых gRPC методов

1. Обновить `proto/` файлы
2. Сгенерировать код: `buf generate`
3. Добавить методы в `grpc_client.go`
4. Обновить `config_client.go`

## Версионирование

```bash
# Показать версию
./uptimeping version

# Вывод:
# UptimePing CLI v1.0.0
# Build: 2026-01-28T16:30:00Z
# Git: abc1234
```

## Лицензия

MIT License - см. файл LICENSE в корне проекта.
