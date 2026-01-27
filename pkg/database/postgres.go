package database

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	MaxConns    int
	MinConns    int
	MaxConnLife time.Duration
	MaxConnIdle time.Duration
	HealthCheck time.Duration
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
func GetConfig() *Config {
	config := NewConfig()
	
	// Загружаем значения из переменных окружения, если они установлены
	if host := os.Getenv("DB_HOST"); host != "" {
		config.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Port = p
		}
	}
	if user := os.Getenv("DB_USER"); user != "" {
		config.User = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		config.Password = password
	}
	if database := os.Getenv("DB_NAME"); database != "" {
		config.Database = database
	}
	if sslmode := os.Getenv("DB_SSLMODE"); sslmode != "" {
		config.SSLMode = sslmode
	}
	
	// Пул соединений
	if maxConns := os.Getenv("DB_MAX_CONNS"); maxConns != "" {
		if mc, err := strconv.Atoi(maxConns); err == nil {
			config.MaxConns = mc
		}
	}
	if minConns := os.Getenv("DB_MIN_CONNS"); minConns != "" {
		if mc, err := strconv.Atoi(minConns); err == nil {
			config.MinConns = mc
		}
	}
	
	// Таймауты
	if maxConnLife := os.Getenv("DB_MAX_CONN_LIFE"); maxConnLife != "" {
		if mcl, err := time.ParseDuration(maxConnLife); err == nil {
			config.MaxConnLife = mcl
		}
	}
	if maxConnIdle := os.Getenv("DB_MAX_CONN_IDLE"); maxConnIdle != "" {
		if mci, err := time.ParseDuration(maxConnIdle); err == nil {
			config.MaxConnIdle = mci
		}
	}
	if healthCheck := os.Getenv("DB_HEALTH_CHECK"); healthCheck != "" {
		if hc, err := time.ParseDuration(healthCheck); err == nil {
			config.HealthCheck = hc
		}
	}
	
	// Retry настройки
	if maxRetries := os.Getenv("DB_MAX_RETRIES"); maxRetries != "" {
		if mr, err := strconv.Atoi(maxRetries); err == nil {
			config.MaxRetries = mr
		}
	}
	if retryInterval := os.Getenv("DB_RETRY_INTERVAL"); retryInterval != "" {
		if ri, err := time.ParseDuration(retryInterval); err == nil {
			config.RetryInterval = ri
		}
	}
	
	// Поддержка DATABASE_URL для совместимости
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		// Парсим DATABASE_URL и извлекаем параметры
		if parsedConfig := parseDatabaseURL(databaseURL); parsedConfig != nil {
			// Применяем только те параметры, которые не были установлены через переменные окружения
			if config.Host == "localhost" && parsedConfig.Host != "" {
				config.Host = parsedConfig.Host
			}
			if config.Port == 5432 && parsedConfig.Port != 0 {
				config.Port = parsedConfig.Port
			}
			if config.User == "postgres" && parsedConfig.User != "" {
				config.User = parsedConfig.User
			}
			if config.Password == "postgres" && parsedConfig.Password != "" {
				config.Password = parsedConfig.Password
			}
			if config.Database == "postgres" && parsedConfig.Database != "" {
				config.Database = parsedConfig.Database
			}
			if config.SSLMode == "disable" && parsedConfig.SSLMode != "" {
				config.SSLMode = parsedConfig.SSLMode
			}
		}
	}
	
	return config
}

// parseDatabaseURL парсит DATABASE_URL и извлекает параметры подключения
func parseDatabaseURL(databaseURL string) *Config {
	// Простой парсер для postgres://user:password@host:port/database
	if !strings.HasPrefix(databaseURL, "postgres://") && !strings.HasPrefix(databaseURL, "postgresql://") {
		return nil
	}
	
	// Удаляем префикс
	url := strings.TrimPrefix(databaseURL, "postgres://")
	url = strings.TrimPrefix(url, "postgresql://")
	
	// Разделяем на части
	parts := strings.Split(url, "@")
	if len(parts) < 2 {
		return nil
	}
	
	// Парсим user:password
	authParts := strings.Split(parts[0], ":")
	if len(authParts) < 2 {
		return nil
	}
	
	// Парсим host:port/database
	hostParts := strings.Split(parts[1], "/")
	if len(hostParts) < 2 {
		return nil
	}
	
	hostPort := strings.Split(hostParts[0], ":")
	host := "localhost"
	port := 5432
	
	if len(hostPort) > 1 {
		host = hostPort[0]
		if p, err := strconv.Atoi(hostPort[1]); err == nil {
			port = p
		}
	} else {
		host = hostPort[0]
	}
	
	return &Config{
		Host:     host,
		Port:     port,
		User:     authParts[0],
		Password: authParts[1],
		Database: hostParts[1],
		SSLMode:  "disable",
	}
}
