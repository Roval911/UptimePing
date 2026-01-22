package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BaseRepository базовая структура для всех репозиториев PostgreSQL
type BaseRepository struct {
	Pool *pgxpool.Pool
}

// NewBaseRepository создает новый экземпляр базового репозитория
func NewBaseRepository(pool *pgxpool.Pool) *BaseRepository {
	return &BaseRepository{Pool: pool}
}

// ExecContext выполняет запрос с контекстом
func (r *BaseRepository) ExecContext(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return r.Pool.Query(ctx, query, args...)
}

// QueryRowContext выполняет запрос и возвращает одну строку
func (r *BaseRepository) QueryRowContext(ctx context.Context, query string, args ...interface{}) pgx.Row {
	return r.Pool.QueryRow(ctx, query, args...)
}

// QueryContext выполняет запрос и возвращает несколько строк
func (r *BaseRepository) QueryContext(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return r.Pool.Query(ctx, query, args...)
}
