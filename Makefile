# Makefile для UptimePing Platform

# Переменные
PROJECT_NAME = UptimePing Platform
VERSION = 0.1.0

# Пути
SCRIPTS_DIR = scripts
MIGRATIONS_DIR = migrations

# Цели по умолчанию
.PHONY: help build test start stop clean migrate init-db proto

proto:
	@echo "Генерация кода из proto файлов..."
	buf generate

setup:
	@echo "Настройка окружения..."
	${SCRIPTS_DIR}/setup-env.sh

init:
	@echo "Инициализация базы данных..."
	${SCRIPTS_DIR}/init-db.sh

migrate:
	@echo "Применение миграций..."
	${SCRIPTS_DIR}/migrate.sh

start:
	@echo "Запуск платформы..."
	${SCRIPTS_DIR}/setup-env.sh
	docker-compose -f deployments/docker-compose/docker-compose.yml up -d

stop:
	@echo "Остановка платформы..."
	docker-compose -f deployments/docker-compose/docker-compose.yml down

restart:
	make stop
	make start

logs:
	@echo "Логи платформы (нажмите Ctrl+C для выхода)..."
	docker-compose -f deployments/docker-compose/docker-compose.yml logs -f

logs-service:
	@echo "Логи сервиса $(service) (нажмите Ctrl+C для выхода)..."
	docker-compose -f deployments/docker-compose/docker-compose.yml logs -f $(service)

ps:
	@echo "Состояние сервисов:"
	docker-compose -f deployments/docker-compose/docker-compose.yml ps

config:
	@echo "Конфигурация docker-compose:"
	docker-compose -f deployments/docker-compose/docker-compose.yml config

pull:
	@echo "Обновление образов..."
	docker-compose -f deployments/docker-compose/docker-compose.yml pull

build:
	@echo "Сборка сервисов..."
	docker-compose -f deployments/docker-compose/docker-compose.yml build

build-no-cache:
	@echo "Сборка сервисов без кеша..."
	docker-compose -f deployments/docker-compose/docker-compose.yml build --no-cache

clean:
	@echo "Очистка..."
	docker-compose -f deployments/docker-compose/docker-compose.yml down -v --remove-orphans
	rm -f .env

# Алиасы
up: start
down: stop
status: ps
