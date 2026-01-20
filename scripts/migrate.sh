#!/bin/bash

# Скрипт запуска миграций базы данных

set -e

# Путь к миграциям
MIGRATIONS_DIR="${MIGRATIONS_DIR:-migrations}"

# Проверка наличия goose
if ! command -v goose &> /dev/null; then
  echo "goose не установлен. Установка..."
  go install github.com/pressly/goose/v3/cmd/goose@latest
fi

# Ожидание готовности базы данных
echo "Ожидание готовности базы данных..."
scripts/wait-for-db.sh "${POSTGRES_HOST}" "${POSTGRES_PORT}"

echo "Запуск миграций из директории ${MIGRATIONS_DIR}..."

goose -dir "${MIGRATIONS_DIR}" "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" up

echo "Миграции успешно применены"