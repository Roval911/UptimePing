package database

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig_DefaultValues(t *testing.T) {
	// Очищаем переменные окружения
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DB_SSLMODE")
	os.Unsetenv("DATABASE_URL")
	
	config := GetConfig()
	
	// Проверяем значения по умолчанию
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 5432, config.Port)
	assert.Equal(t, "postgres", config.User)
	assert.Equal(t, "postgres", config.Password)
	assert.Equal(t, "postgres", config.Database)
	assert.Equal(t, "disable", config.SSLMode)
	assert.Equal(t, 20, config.MaxConns)
	assert.Equal(t, 5, config.MinConns)
	assert.Equal(t, 30*time.Minute, config.MaxConnLife)
	assert.Equal(t, 5*time.Minute, config.MaxConnIdle)
	assert.Equal(t, 30*time.Second, config.HealthCheck)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryInterval)
}

func TestGetConfig_EnvironmentVariables(t *testing.T) {
	// Устанавливаем переменные окружения
	os.Setenv("DB_HOST", "test-host")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_USER", "test-user")
	os.Setenv("DB_PASSWORD", "test-pass")
	os.Setenv("DB_NAME", "test-db")
	os.Setenv("DB_SSLMODE", "require")
	os.Setenv("DB_MAX_CONNS", "50")
	os.Setenv("DB_MIN_CONNS", "10")
	os.Setenv("DB_MAX_CONN_LIFE", "1h")
	os.Setenv("DB_MAX_CONN_IDLE", "30m")
	os.Setenv("DB_HEALTH_CHECK", "10s")
	os.Setenv("DB_MAX_RETRIES", "5")
	os.Setenv("DB_RETRY_INTERVAL", "2s")
	
	defer func() {
		// Очищаем переменные окружения
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("DB_SSLMODE")
		os.Unsetenv("DB_MAX_CONNS")
		os.Unsetenv("DB_MIN_CONNS")
		os.Unsetenv("DB_MAX_CONN_LIFE")
		os.Unsetenv("DB_MAX_CONN_IDLE")
		os.Unsetenv("DB_HEALTH_CHECK")
		os.Unsetenv("DB_MAX_RETRIES")
		os.Unsetenv("DB_RETRY_INTERVAL")
	}()
	
	config := GetConfig()
	
	// Проверяем значения из переменных окружения
	assert.Equal(t, "test-host", config.Host)
	assert.Equal(t, 5433, config.Port)
	assert.Equal(t, "test-user", config.User)
	assert.Equal(t, "test-pass", config.Password)
	assert.Equal(t, "test-db", config.Database)
	assert.Equal(t, "require", config.SSLMode)
	assert.Equal(t, 50, config.MaxConns)
	assert.Equal(t, 10, config.MinConns)
	assert.Equal(t, 1*time.Hour, config.MaxConnLife)
	assert.Equal(t, 30*time.Minute, config.MaxConnIdle)
	assert.Equal(t, 10*time.Second, config.HealthCheck)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 2*time.Second, config.RetryInterval)
}

func TestGetConfig_DatabaseURL(t *testing.T) {
	// Очищаем переменные окружения
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DB_SSLMODE")
	
	// Устанавливаем DATABASE_URL
	os.Setenv("DATABASE_URL", "postgres://testuser:testpass@testhost:5433/testdb")
	defer os.Unsetenv("DATABASE_URL")
	
	config := GetConfig()
	
	// Проверяем значения из DATABASE_URL
	assert.Equal(t, "testhost", config.Host)
	assert.Equal(t, 5433, config.Port)
	assert.Equal(t, "testuser", config.User)
	assert.Equal(t, "testpass", config.Password)
	assert.Equal(t, "testdb", config.Database)
	assert.Equal(t, "disable", config.SSLMode) // значение по умолчанию
}

func TestGetConfig_DatabaseURLWithEnvOverride(t *testing.T) {
	// Устанавливаем переменные окружения
	os.Setenv("DB_HOST", "env-host")
	os.Setenv("DATABASE_URL", "postgres://testuser:testpass@testhost:5433/testdb")
	
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DATABASE_URL")
	}()
	
	config := GetConfig()
	
	// Переменные окружения должны иметь приоритет над DATABASE_URL
	assert.Equal(t, "env-host", config.Host)
	assert.Equal(t, 5433, config.Port)
	assert.Equal(t, "testuser", config.User)
	assert.Equal(t, "testpass", config.Password)
	assert.Equal(t, "testdb", config.Database)
}

func TestParseDatabaseURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *Config
	}{
		{
			name: "full postgres URL",
			url:  "postgres://user:pass@localhost:5432/database",
			expected: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Database: "database",
				SSLMode:  "disable",
			},
		},
		{
			name: "postgresql URL",
			url:  "postgresql://admin:secret@db.example.com:5433/mydb",
			expected: &Config{
				Host:     "db.example.com",
				Port:     5433,
				User:     "admin",
				Password: "secret",
				Database: "mydb",
				SSLMode:  "disable",
			},
		},
		{
			name: "URL without port",
			url:  "postgres://user:pass@localhost/database",
			expected: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Database: "database",
				SSLMode:  "disable",
			},
		},
		{
			name:     "invalid URL",
			url:      "invalid-url",
			expected: nil,
		},
		{
			name:     "missing parts",
			url:      "postgres://user@localhost",
			expected: nil,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDatabaseURL(tt.url)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Host, result.Host)
				assert.Equal(t, tt.expected.Port, result.Port)
				assert.Equal(t, tt.expected.User, result.User)
				assert.Equal(t, tt.expected.Password, result.Password)
				assert.Equal(t, tt.expected.Database, result.Database)
				assert.Equal(t, tt.expected.SSLMode, result.SSLMode)
			}
		})
	}
}
