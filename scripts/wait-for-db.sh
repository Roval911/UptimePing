#!/bin/bash

# Скрипт ожидания готовности PostgreSQL

set -e

host="${1}"
port="${2}"
shift 2
cmd="${@}"

export PGPASSWORD="${POSTGRES_PASSWORD}"

until psql -h "${host}" -p "${port}" -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -c '\q'; do
  >&2 echo "Postgres is unavailable - sleeping"
  sleep 1
done

>&2 echo "Postgres is up - executing command ${cmd}"
exec ${cmd}