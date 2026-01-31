# UptimePing CLI - Полное руководство по командам

## 📋 Обзор

UptimePing CLI - мощный инструмент командной строки для управления платформой мониторинга доступности сервисов. Поддерживает управление аутентификацией, проверками, инцидентами, уведомлениями и конфигурацией системы.

## 🚀 Быстрый старт

### Установка и базовая настройка
```bash
# Регистрация нового пользователя
uptimeping auth register --email admin@example.com --password securePass --tenant "Production"

# Вход в систему
uptimeping auth login --email admin@example.com --password securePass

# Проверка статуса
uptimeping auth status
```

## 🔐 Auth - Управление аутентификацией

### Регистрация пользователя
```bash
# Базовая регистрация
uptimeping auth register --email user@example.com --password MyPassword123 --tenant "MyCompany"

# Регистрация с дополнительными параметрами
uptimeping auth register \
  --email admin@company.com \
  --password SecurePass123! \
  --tenant "Production" \
  --server https://api.company.com
```

### Вход в систему
```bash
# Стандартный вход
uptimeping auth login --email user@example.com --password MyPassword123

# Вход с указанием сервера
uptimeping auth login \
  --email admin@company.com \
  --password SecurePass123! \
  --server https://api.company.com
```

### Управление сессией
```bash
# Проверить статус авторизации
uptimeping auth status

# Выход из системы
uptimeping auth logout

# Обновить токен (если истекает)
uptimeping auth refresh
```

### Флаги Auth
- `--email`: Email адрес пользователя
- `--password`: Пароль пользователя
- `--tenant`: Имя тенанта
- `--server`: URL API сервера

## 📊 Checks - Управление проверками

### Просмотр проверок
```bash
# Список всех проверок
uptimeping checks list

# Список с фильтрацией по тегам
uptimeping checks list --tags production,critical

# Только активные проверки
uptimeping checks list --enabled-only

# Только неактивные проверки
uptimeping checks list --disabled-only

# Фильтрация по типу проверки
uptimeping checks list --type http

# Форматы вывода
uptimeping checks list --output json
uptimeping checks list --output yaml
uptimeping checks list --output table
```

### Создание проверок
```bash
# HTTP проверка
uptimeping checks create \
  --name "Google Homepage" \
  --url "https://google.com" \
  --type http \
  --interval 60 \
  --timeout 10

# HTTPS проверка с проверкой сертификата
uptimeping checks create \
  --name "API Endpoint" \
  --url "https://api.company.com/health" \
  --type https \
  --interval 30 \
  --timeout 5 \
  --verify-ssl

# TCP проверка порта
uptimeping checks create \
  --name "Database Port" \
  --host "db.company.com" \
  --port 5432 \
  --type tcp \
  --interval 60

# Ping проверка
uptimeping checks create \
  --name "Router" \
  --host "192.168.1.1" \
  --type ping \
  --interval 30

# gRPC проверка
uptimeping checks create \
  --name "User Service" \
  --host "grpc.company.com" \
  --port 50051 \
  --type grpc \
  --service "UserService" \
  --method "GetUser"
```

### Управление проверками
```bash
# Получить детали проверки
uptimeping checks get <check-id>

# Обновить проверку
uptimeping checks update <check-id> \
  --interval 120 \
  --timeout 15 \
  --tags updated,critical

# Включить проверку
uptimeping checks enable <check-id>

# Выключить проверку
uptimeping checks disable <check-id>

# Удалить проверку
uptimeping checks delete <check-id>

# Тестировать проверку
uptimeping checks test <check-id>
```

### Флаги Checks
- `--name`: Имя проверки
- `--url`: URL для HTTP/HTTPS проверок
- `--host`: Хост для TCP/ping проверок
- `--port`: Порт для TCP/gRPC проверок
- `--type`: Тип проверки (http, https, tcp, ping, grpc)
- `--interval`: Интервал проверки в секундах
- `--timeout`: Таймаут в секундах
- `--tags`: Теги для фильтрации
- `--enabled`: Включена ли проверка
- `--verify-ssl`: Проверять SSL сертификат

## 🚨 Incidents - Управление инцидентами

### Просмотр инцидентов
```bash
# Список всех инцидентов
uptimeping incidents list

# Фильтрация по статусу
uptimeping incidents list --status open
uptimeping incidents list --status acknowledged
uptimeping incidents list --status resolved

# Фильтрация по серьезности
uptimeping incidents list --severity critical
uptimeping incidents list --severity high
uptimeping incidents list --severity medium
uptimeping incidents list --severity low

# Фильтрация по временному диапазону
uptimeping incidents list --from "2024-01-01T00:00:00Z"
uptimeping incidents list --to "2024-01-31T23:59:59Z"

# Ограничение количества результатов
uptimeping incidents list --limit 50
```

### Управление инцидентами
```bash
# Получить детали инцидента
uptimeping incidents get <incident-id>

# Подтвердить инцидент
uptimeping incidents acknowledge <incident-id> \
  --comment "Начинаю расследование проблемы"

# Разрешить инцидент
uptimeping incidents resolve <incident-id> \
  --comment "Проблема исправлена, сервер перезапущен"

# Создать инцидент вручную
uptimeping incidents create \
  --title "API Server Down" \
  --description "API сервер не отвечает на запросы" \
  --severity critical \
  --check-id <check-id>
```

### Флаги Incidents
- `--status`: Статус инцидента (open, acknowledged, resolved)
- `--severity`: Серьезность (critical, high, medium, low)
- `--from`: Начало временного диапазона
- `--to`: Конец временного диапазона
- `--limit`: Ограничение количества результатов
- `--title`: Заголовок инцидента
- `--description`: Описание инцидента
- `--check-id`: ID связанной проверки
- `--comment`: Комментарий к действию

## ⚙️ Config - Управление конфигурацией

### Управление конфигурациями проверок
```bash
# Список всех конфигураций
uptimeping config list

# Создание новой конфигурации
uptimeping config create \
  --name "Production API" \
  --url "https://api.company.com" \
  --type https \
  --interval 60 \
  --timeout 10 \
  --tags production,api

# Получение конфигурации
uptimeping config get <config-id>

# Обновление конфигурации
uptimeping config update <config-id> \
  --interval 120 \
  --timeout 15 \
  --tags updated

# Просмотр конфигурации
uptimeping config view <config-id>

# Удаление конфигурации
uptimeping config delete <config-id>

# Инициализация конфигурации по умолчанию
uptimeping config init
```

### Управление глобальными настройками
```bash
# Просмотр глобальной конфигурации
uptimeping config view global

# Обновление глобальных настроек
uptimeping config update global \
  --default-timeout 30 \
  --default-retry-count 3 \
  --notification-email admin@company.com
```

### Флаги Config
- `--name`: Имя конфигурации
- `--url`: URL для проверки
- `--type`: Тип проверки
- `--interval`: Интервал проверки
- `--timeout`: Таймаут
- `--tags`: Теги
- `--default-timeout`: Таймаут по умолчанию
- `--default-retry-count`: Количество попыток по умолчанию

## 📢 Notification - Управление уведомлениями

### Управление каналами уведомлений
```bash
# Список всех каналов
uptimeping notification list

# Создание email канала
uptimeping notification create \
  --name "Email Alerts" \
  --type email \
  --target admin@company.com,dev@company.com \
  --enabled

# Создание Slack канала
uptimeping notification create \
  --name "Slack Notifications" \
  --type slack \
  --webhook "https://hooks.slack.com/services/..." \
  --channel "#alerts"

# Создание webhook канала
uptimeping notification create \
  --name "Custom Webhook" \
  --type webhook \
  --url "https://api.company.com/webhooks/alerts" \
  --headers "Authorization:Bearer token123"

# Тестирование канала
uptimeping notification test <channel-id> \
  --message "Test notification from UptimePing CLI"

# Обновление канала
uptimeping notification update <channel-id> \
  --enabled false

# Удаление канала
uptimeping notification delete <channel-id>
```

### Флаги Notification
- `--name`: Имя канала
- `--type`: Тип канала (email, slack, webhook)
- `--target`: Получатели (для email)
- `--webhook`: URL webhook
- `--channel`: Канал Slack
- `--headers`: Заголовки HTTP
- `--enabled`: Включен ли канал
- `--message`: Тестовое сообщение

## 🌐 Context - Управление окружениями

### Управление контекстами
```bash
# Список всех контекстов
uptimeping context list

# Текущий контекст
uptimeping context current

# Создание нового контекста
uptimeping context create staging \
  --server https://staging-api.company.com \
  --description "Staging environment"

# Создание production контекста
uptimeping context create production \
  --server https://api.company.com \
  --description "Production environment"

# Переключение контекста
uptimeping context set staging
uptimeping context set production

# Показать детали контекста
uptimeping context show production

# Удаление контекста
uptimeping context delete test

# Установка контекста с временными параметрами
uptimeping context set production \
  --timeout 30 \
  --verbose
```

### Флаги Context
- `--server`: URL API сервера
- `--description`: Описание окружения
- `--timeout`: Таймаут запросов
- `--verbose`: Подробный вывод

## 🔧 Forge - Управление Forge сервисом

### Управление задачами Forge
```bash
# Статус Forge сервиса
uptimeping forge status

# Список задач
uptimeping forge list

# Создание задачи
uptimeping forge create \
  --name "Build Application" \
  --type build \
  --repository "company/app" \
  --branch main

# Получение деталей задачи
uptimeping forge get <task-id>

# Отмена задачи
uptimeping forge cancel <task-id>
```

### Флаги Forge
- `--name`: Имя задачи
- `--type`: Тип задачи (build, deploy, test)
- `--repository`: Репозиторий
- `--branch`: Ветка

## 🛠️ Utility команды

### Автодополнение
```bash
# Установка автодополнения для bash
source <(uptimeping completion bash)

# Установка автодополнения для zsh
source <(uptimeping completion zsh)

# Генерация скрипта автодополнения
uptimeping completion bash > /etc/bash_completion.d/uptimeping
uptimeping completion zsh > /usr/local/share/zsh/site-functions/_uptimeping
```

### Экспорт конфигурации
```bash
# Экспорт в YAML формат
uptimeping export --format yaml > config.yaml

# Экспорт в JSON формат
uptimeping export --format json > config.json

# Экспорт только проверок
uptimeping export --type checks > checks.yaml

# Экспорт с фильтрацией
uptimeping export --tags production > production-config.yaml
```

### Системная информация
```bash
# Версия CLI
uptimeping --version

# Помощь по команде
uptimeping --help
uptimeping checks --help
uptimeping incidents --help
```

## 🌍 Глобальные флаги

### Основные флаги
```bash
# Указание сервера
uptimeping --server https://api.company.com checks list

# Указание конфигурационного файла
uptimeping --config ~/.uptimeping-prod.yaml auth login

# Отладочный режим
uptimeping --debug checks list

# Подробный вывод
uptimeping --verbose incidents list

# Тихий режим (минимум логов)
uptimeping --quiet checks list

# Формат вывода
uptimeping --output json checks list
uptimeping --output yaml incidents list
uptimeping --output table config list
```

### Флаги для всех команд
- `--config`: Путь к конфигурационному файлу
- `--server`: URL API сервера
- `--debug`: Включить отладочный режим
- `--verbose`: Подробный вывод
- `--output`: Формат вывода (table, json, yaml)
- `--help`: Помощь по команде
- `--version`: Версия CLI

## 📝 Примеры использования

### Полный рабочий процесс
```bash
# 1. Настройка окружений
uptimeping context create production --server https://api.company.com
uptimeping context create staging --server https://staging-api.company.com

# 2. Работа в production
uptimeping context set production
uptimeping auth login --email admin@company.com --password SecurePass123

# 3. Создание проверок
uptimeping checks create \
  --name "Company Website" \
  --url "https://company.com" \
  --type https \
  --interval 300 \
  --tags production,critical

uptimeping checks create \
  --name "API Health" \
  --url "https://api.company.com/health" \
  --type https \
  --interval 60 \
  --tags production,api

# 4. Настройка уведомлений
uptimeping notification create \
  --name "Production Alerts" \
  --type email \
  --target admin@company.com,devops@company.com

# 5. Переключение в staging
uptimeping context set staging
uptimeping auth login --email dev@company.com --password DevPass123

# 6. Создание тестовых проверок
uptimeping checks create \
  --name "Staging API" \
  --url "https://staging-api.company.com" \
  --type https \
  --interval 30 \
  --tags staging
```

### Мониторинг инцидентов
```bash
# Просмотр активных инцидентов
uptimeping incidents list --status open --severity critical

# Подтверждение инцидента
uptimeping incidents acknowledge <incident-id> \
  --comment "Команда оповещена, начинаем расследование"

# Разрешение инцидента
uptimeping incidents resolve <incident-id> \
  --comment "Проблема исправлена, сервис восстановлен"

# Создание отчета по инцидентам
uptimeping incidents list --from "2024-01-01T00:00:00Z" \
  --to "2024-01-31T23:59:59Z" \
  --output json > incidents-report.json
```

### Управление конфигурациями
```bash
# Экспорт текущей конфигурации
uptimeping export --format yaml > backup-config.yaml

# Импорт конфигурации из файла
uptimeping config import --file new-config.yaml

# Массовое создание проверок из файла
cat checks.txt | xargs -I {} uptimeping checks create --config {}

# Обновление таймаутов для всех проверок
uptimeping checks list --output json | \
  jq '.[] | select(.timeout < 10) | .id' | \
  xargs -I {} uptimeping checks update {} --timeout 10
```

## 🔍 Поиск и фильтрация

### Расширенные примеры фильтрации
```bash
# Поиск проверок по шаблону имени
uptimeping checks list --name "*API*"

# Проверки с определенными тегами
uptimeping checks list --tags production,api --enabled-only

# Инциденты за последние 24 часа
uptimeping incidents list \
  --from "$(date -d '1 day ago' -I seconds)" \
  --to "$(date -I seconds)"

# Проверки с интервалом более 5 минут
uptimeping checks list --output json | \
  jq '.[] | select(.interval > 300)'
```

## 🚀 Продвинутые сценарии

### Автоматизация с помощью скриптов
```bash
#!/bin/bash
# monitor.sh - Скрипт мониторинга

# Проверка статуса всех критичных проверок
CRITICAL_CHECKS=$(uptimeping checks list --tags critical --output json | \
  jq -r '.[] | select(.status != "healthy") | .id')

if [ -n "$CRITICAL_CHECKS" ]; then
  echo "❌ Обнаружены проблемы с критичными проверками:"
  echo "$CRITICAL_CHECKS"
  
  # Создание инцидента
  uptimeping incidents create \
    --title "Critical Checks Failed" \
    --description "Multiple critical checks are failing" \
    --severity critical
  
  # Отправка уведомления
  uptimeping notification test production-alerts \
    --message "Critical infrastructure issues detected"
else
  echo "✅ Все критические проверки в норме"
fi
```

### Резервное копирование конфигурации
```bash
#!/bin/bash
# backup.sh - Скрипт резервного копирования

DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/backup/uptimeping"

# Создание директории бэкапа
mkdir -p "$BACKUP_DIR"

# Экспорт конфигураций
uptimeping export --format yaml > "$BACKUP_DIR/config_$DATE.yaml"
uptimeping checks list --output json > "$BACKUP_DIR/checks_$DATE.json"
uptimeping incidents list --output json > "$BACKUP_DIR/incidents_$DATE.json"

# Архивирование
tar -czf "$BACKUP_DIR/uptimeping_backup_$DATE.tar.gz" \
  "$BACKUP_DIR/config_$DATE.yaml" \
  "$BACKUP_DIR/checks_$DATE.json" \
  "$BACKUP_DIR/incidents_$DATE.json"

# Удаление временных файлов
rm "$BACKUP_DIR/config_$DATE.yaml" \
  "$BACKUP_DIR/checks_$DATE.json" \
  "$BACKUP_DIR/incidents_$DATE.json"

echo "✅ Бэкап завершен: $BACKUP_DIR/uptimeping_backup_$DATE.tar.gz"
```

## 📚 Справочная информация

### Статусы проверок
- `healthy`: Проверка проходит успешно
- `unhealthy`: Проверка не проходит
- `unknown`: Статус неизвестен
- `disabled`: Проверка отключена

### Статусы инцидентов
- `open`: Инцидент открыт
- `acknowledged`: Инцидент подтвержден
- `resolved`: Инцидент решен

### Уровни серьезности
- `critical`: Критический
- `high`: Высокий
- `medium`: Средний
- `low`: Низкий

### Типы проверок
- `http`: HTTP запрос
- `https`: HTTPS с проверкой SSL
- `tcp`: TCP подключение
- `ping`: ICMP ping
- `grpc`: gRPC вызов

---

**UptimePing CLI предоставляет полный контроль над платформой мониторинга доступности сервисов!** 🚀
