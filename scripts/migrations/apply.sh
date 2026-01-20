#!/bin/bash

# Script to apply database migrations

set -e

echo "Applying database migrations..."

# Apply main migrations
if goose -dir migrations postgres "$DATABASE_URL" up; then
    echo "Main migrations applied successfully"
else
    echo "Failed to apply main migrations"
    exit 1
fi

# Apply seed migrations
if goose -dir migrations/seed postgres "$DATABASE_URL" up; then
    echo "Seed data migrations applied successfully"
else
    echo "Failed to apply seed data migrations"
    exit 1
fi

echo "All migrations applied successfully!"