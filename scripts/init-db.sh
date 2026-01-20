#!/bin/bash

# Скрипт инициализации базы данных

set -e

export PGPASSWORD="${POSTGRES_PASSWORD}"

echo "Инициализация базы данных ${POSTGRES_DB}..."

# Создание базы данных, если она не существует
psql -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" -c "CREATE DATABASE ${POSTGRES_DB} OWNER ${POSTGRES_USER};" || echo "База данных уже существует"

# Создание расширений
psql -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -c "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";"
psql -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -c "CREATE EXTENSION IF NOT EXISTS \"pgcrypto\";"

echo "Инициализация базы данных завершена"