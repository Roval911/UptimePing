#!/bin/bash

# Script to rollback database migrations

set -e

echo "Rolling back database migrations..."

# Rollback seed migrations
if goose -dir migrations/seed postgres "$DATABASE_URL" down; then
    echo "Seed data migrations rolled back successfully"
else
    echo "Failed to rollback seed data migrations"
    exit 1
fi

# Rollback main migrations
if goose -dir migrations postgres "$DATABASE_URL" down; then
    echo "Main migrations rolled back successfully"
else
    echo "Failed to rollback main migrations"
    exit 1
fi

echo "All migrations rolled back successfully!"