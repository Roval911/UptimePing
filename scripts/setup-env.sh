#!/bin/bash

# Скрипт настройки переменных окружения

set -e

echo "Настройка переменных окружения..."

# Путь к файлу .env
ENV_FILE="${1:-.env}"

# Проверка существования файла
if [ ! -f "${ENV_FILE}" ]; then
  echo "Создание файла ${ENV_FILE} на основе .env.example"
  if [ -f ".env.example" ]; then
    cp .env.example "${ENV_FILE}"
    echo "Файл ${ENV_FILE} создан. Пожалуйста, обновите его с вашими значениями."
  else
    echo "Ошибка: .env.example не найден"
    exit 1
  fi
fi

# Загрузка переменных окружения
export $(grep -v '^#' "${ENV_FILE}" | xargs)

echo "Переменные окружения загружены из ${ENV_FILE}"