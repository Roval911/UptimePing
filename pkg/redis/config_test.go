package redis

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig_DefaultValues(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем значения по умолчанию
	assert.Equal(t, "localhost:6379", config.Addr)
	assert.Equal(t, "", config.Password)
	assert.Equal(t, 0, config.DB)
	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 2, config.MinIdleConn)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryInterval)
	assert.Equal(t, 30*time.Second, config.HealthCheck)
}

func TestGetConfig_EnvironmentVariables(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	// Устанавливаем переменные окружения
	os.Setenv("REDIS_ADDR", "redis:6379")
	os.Setenv("REDIS_PASSWORD", "secret123")
	os.Setenv("REDIS_DB", "1")
	os.Setenv("REDIS_POOL_SIZE", "20")
	os.Setenv("REDIS_MIN_IDLE_CONN", "5")
	os.Setenv("REDIS_MAX_RETRIES", "5")
	os.Setenv("REDIS_RETRY_INTERVAL", "2s")
	os.Setenv("REDIS_HEALTH_CHECK", "10s")
	
	defer clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем значения из переменных окружения
	assert.Equal(t, "redis:6379", config.Addr)
	assert.Equal(t, "secret123", config.Password)
	assert.Equal(t, 1, config.DB)
	assert.Equal(t, 20, config.PoolSize)
	assert.Equal(t, 5, config.MinIdleConn)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 2*time.Second, config.RetryInterval)
	assert.Equal(t, 10*time.Second, config.HealthCheck)
}

func TestGetConfig_InvalidValues(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	// Устанавливаем некорректные значения
	os.Setenv("REDIS_DB", "invalid")
	os.Setenv("REDIS_POOL_SIZE", "not_a_number")
	os.Setenv("REDIS_MIN_IDLE_CONN", "not_a_number")
	os.Setenv("REDIS_MAX_RETRIES", "not_a_number")
	os.Setenv("REDIS_RETRY_INTERVAL", "not_a_duration")
	os.Setenv("REDIS_HEALTH_CHECK", "not_a_duration")
	
	defer clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем, что остались значения по умолчанию
	assert.Equal(t, 0, config.DB)
	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 2, config.MinIdleConn)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryInterval)
	assert.Equal(t, 30*time.Second, config.HealthCheck)
}

func TestGetConfig_PartialEnvironment(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	// Устанавливаем только некоторые переменные
	os.Setenv("REDIS_ADDR", "redis-cluster:6379")
	os.Setenv("REDIS_PASSWORD", "cluster-password")
	os.Setenv("REDIS_MAX_RETRIES", "10")
	os.Setenv("REDIS_RETRY_INTERVAL", "5s")
	
	defer clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем, что только установленные значения изменились
	assert.Equal(t, "redis-cluster:6379", config.Addr)
	assert.Equal(t, "cluster-password", config.Password)
	assert.Equal(t, 10, config.MaxRetries)
	assert.Equal(t, 5*time.Second, config.RetryInterval)
	
	// Остальные должны остаться по умолчанию
	assert.Equal(t, 0, config.DB)
	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 2, config.MinIdleConn)
	assert.Equal(t, 30*time.Second, config.HealthCheck)
}

func TestGetConfig_DurationParsing(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	testCases := []struct {
		envValue  string
		expected time.Duration
	}{
		{"1s", 1 * time.Second},
		{"5m", 5 * time.Minute},
		{"1h", 1 * time.Hour},
		{"500ms", 500 * time.Millisecond},
	}
	
	for _, tc := range testCases {
		os.Setenv("REDIS_RETRY_INTERVAL", tc.envValue)
		config := GetConfig()
		assert.Equal(t, tc.expected, config.RetryInterval, "Value: %s", tc.envValue)
		os.Unsetenv("REDIS_RETRY_INTERVAL")
	}
}

func TestGetConfig_IntegerParsing(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	testCases := []struct {
		envValue  string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"10", 10},
		{"100", 100},
		{"999", 999},
	}
	
	for _, tc := range testCases {
		os.Setenv("REDIS_POOL_SIZE", tc.envValue)
		config := GetConfig()
		assert.Equal(t, tc.expected, config.PoolSize, "Value: %s", tc.envValue)
		os.Unsetenv("REDIS_POOL_SIZE")
	}
}

func clearEnvVars() {
	vars := []string{
		"REDIS_ADDR",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"REDIS_POOL_SIZE",
		"REDIS_MIN_IDLE_CONN",
		"REDIS_MAX_RETRIES",
		"REDIS_RETRY_INTERVAL",
		"REDIS_HEALTH_CHECK",
	}
	
	for _, v := range vars {
		os.Unsetenv(v)
	}
}
