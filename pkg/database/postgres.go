package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres представляет подключение к PostgreSQL
type Postgres struct {
	Pool *pgxpool.Pool
}

// Config представляет конфигурацию PostgreSQL
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	// Connection pool settings
	MaxConns     int
	MinConns     int
	MaxConnLife  time.Duration
	MaxConnIdle  time.Duration
	HealthCheck  time.Duration
	// Retry settings
	MaxRetries    int
	RetryInterval time.Duration
}

// NewConfig создает конфигурацию по умолчанию
func NewConfig() *Config {
	return &Config{
		Host:          "localhost",
		Port:          5432,
		User:          "postgres",
		Password:      "postgres",
		Database:      "postgres",
		SSLMode:       "disable",
		MaxConns:      20,
		MinConns:      5,
		MaxConnLife:   30 * time.Minute,
		MaxConnIdle:   5 * time.Minute,
		HealthCheck:   30 * time.Second,
		MaxRetries:    3,
		RetryInterval: 1 * time.Second,
	}
}

// Connect устанавливает подключение к PostgreSQL с retry логикой
func Connect(ctx context.Context, config *Config) (*Postgres, error) {
	var lastErr error

	// Пытаемся подключиться с retry
	for i := 0; i <= config.MaxRetries; i++ {
		// Создаем строку подключения
		connString := fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d&pool_min_conns=%d&pool_max_conn_lifetime=%s&pool_max_conn_idle_time=%s",
			config.User, config.Password, config.Host, config.Port, config.Database,
			config.SSLMode, config.MaxConns, config.MinConns, config.MaxConnLife, config.MaxConnIdle,
		)

		// Создаем конфигурацию пула
		poolConfig, err := pgxpool.ParseConfig(connString)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse pool config: %w", err)
			if i < config.MaxRetries {
				time.Sleep(config.RetryInterval)
			}
			continue
		}

		// Создаем пул подключений
		poolConfig.HealthCheckPeriod = config.HealthCheck
		// Устанавливаем лимиты соединений
		poolConfig.MaxConns = int32(config.MaxConns)
		poolConfig.MinConns = int32(config.MinConns)

		// Устанавливаем максимальное время жизни соединения
		poolConfig.MaxConnLifetime = config.MaxConnLife
		poolConfig.MaxConnIdleTime = config.MaxConnIdle

		// Устанавливаем максимальное время ожидания соединения
		poolConfig.MaxConnLifetimeJitter = 30 * time.Second

		// Пытаемся подключиться
		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to database: %w", err)
			if i < config.MaxRetries {
				time.Sleep(config.RetryInterval)
			}
			continue
		}

		// Проверяем подключение
		if err := pool.Ping(ctx); err != nil {
			lastErr = fmt.Errorf("failed to ping database: %w", err)
			pool.Close()
			if i < config.MaxRetries {
				time.Sleep(config.RetryInterval)
			}
			continue
		}

		return &Postgres{Pool: pool}, nil
	}

	return nil, fmt.Errorf("failed to connect to database after %d retries: %w", config.MaxRetries, lastErr)
}

// Close закрывает подключение к базе данных
func (p *Postgres) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}

// HealthCheck проверяет состояние подключения к базе данных
func (p *Postgres) HealthCheck(ctx context.Context) error {
	if p.Pool == nil {
		return fmt.Errorf("database pool is not initialized")
	}

	// Пытаемся выполнить простой запрос
	var result string
	return p.Pool.QueryRow(ctx, "SELECT 'healthy'").Scan(&result)
}

// GetConfig возвращает конфигурацию из переменных окружения
// В реальном приложении здесь будет интеграция с системой конфигурации
func GetConfig() *Config {
	// TODO: Реализовать загрузку из переменных окружения
	return NewConfig()
}