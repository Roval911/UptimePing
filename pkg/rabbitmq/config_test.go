package rabbitmq

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
	assert.Equal(t, "amqp://guest:guest@localhost:5672/", config.URL)
	assert.Equal(t, "", config.Exchange)
	assert.Equal(t, "", config.RoutingKey)
	assert.Equal(t, "", config.Queue)
	assert.Equal(t, "dlx", config.DLX)
	assert.Equal(t, "dlq", config.DLQ)
	assert.Equal(t, 5*time.Second, config.ReconnectInterval)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1, config.PrefetchCount)
	assert.Equal(t, 0, config.PrefetchSize)
	assert.Equal(t, false, config.Global)
}

func TestGetConfig_EnvironmentVariables(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	// Устанавливаем переменные окружения
	os.Setenv("RABBITMQ_URL", "amqp://user:pass@rabbitmq:5672/")
	os.Setenv("RABBITMQ_EXCHANGE", "test_exchange")
	os.Setenv("RABBITMQ_ROUTING_KEY", "test_key")
	os.Setenv("RABBITMQ_QUEUE", "test_queue")
	os.Setenv("RABBITMQ_DLX", "test_dlx")
	os.Setenv("RABBITMQ_DLQ", "test_dlq")
	os.Setenv("RABBITMQ_RECONNECT_INTERVAL", "10s")
	os.Setenv("RABBITMQ_MAX_RETRIES", "5")
	os.Setenv("RABBITMQ_PREFETCH_COUNT", "10")
	os.Setenv("RABBITMQ_PREFETCH_SIZE", "100")
	os.Setenv("RABBITMQ_GLOBAL", "true")
	
	defer clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем значения из переменных окружения
	assert.Equal(t, "amqp://user:pass@rabbitmq:5672/", config.URL)
	assert.Equal(t, "test_exchange", config.Exchange)
	assert.Equal(t, "test_key", config.RoutingKey)
	assert.Equal(t, "test_queue", config.Queue)
	assert.Equal(t, "test_dlx", config.DLX)
	assert.Equal(t, "test_dlq", config.DLQ)
	assert.Equal(t, 10*time.Second, config.ReconnectInterval)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 10, config.PrefetchCount)
	assert.Equal(t, 100, config.PrefetchSize)
	assert.Equal(t, true, config.Global)
}

func TestGetConfig_InvalidValues(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	// Устанавливаем некорректные значения
	os.Setenv("RABBITMQ_RECONNECT_INTERVAL", "invalid")
	os.Setenv("RABBITMQ_MAX_RETRIES", "not_a_number")
	os.Setenv("RABBITMQ_PREFETCH_COUNT", "not_a_number")
	os.Setenv("RABBITMQ_PREFETCH_SIZE", "not_a_number")
	
	defer clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем, что остались значения по умолчанию
	assert.Equal(t, 5*time.Second, config.ReconnectInterval)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1, config.PrefetchCount)
	assert.Equal(t, 0, config.PrefetchSize)
}

func TestGetConfig_PartialEnvironment(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	// Устанавливаем только некоторые переменные
	os.Setenv("RABBITMQ_URL", "amqp://admin:admin@localhost:5672/")
	os.Setenv("RABBITMQ_MAX_RETRIES", "10")
	os.Setenv("RABBITMQ_GLOBAL", "false")
	
	defer clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем, что только установленные значения изменились
	assert.Equal(t, "amqp://admin:admin@localhost:5672/", config.URL)
	assert.Equal(t, 10, config.MaxRetries)
	assert.Equal(t, false, config.Global)
	
	// Остальные должны остаться по умолчанию
	assert.Equal(t, "", config.Exchange)
	assert.Equal(t, 5*time.Second, config.ReconnectInterval)
	assert.Equal(t, 1, config.PrefetchCount)
}

func TestGetConfig_GlobalVariants(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	testCases := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"", false},
		{"invalid", false},
	}
	
	for _, tc := range testCases {
		os.Setenv("RABBITMQ_GLOBAL", tc.value)
		config := GetConfig()
		assert.Equal(t, tc.expected, config.Global, "Value: %s", tc.value)
		os.Unsetenv("RABBITMQ_GLOBAL")
	}
}

func clearEnvVars() {
	vars := []string{
		"RABBITMQ_URL",
		"RABBITMQ_EXCHANGE",
		"RABBITMQ_ROUTING_KEY",
		"RABBITMQ_QUEUE",
		"RABBITMQ_DLX",
		"RABBITMQ_DLQ",
		"RABBITMQ_RECONNECT_INTERVAL",
		"RABBITMQ_MAX_RETRIES",
		"RABBITMQ_PREFETCH_COUNT",
		"RABBITMQ_PREFETCH_SIZE",
		"RABBITMQ_GLOBAL",
		"RABBITMQ_MAX_RETRY_ATTEMPTS",
		"RABBITMQ_RETRY_DELAY",
	}
	
	for _, v := range vars {
		os.Unsetenv(v)
	}
}
