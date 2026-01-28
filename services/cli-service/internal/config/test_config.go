package config

import (
	"os"
)

// GetTestConfig возвращает тестовую конфигурацию без аутентификации
func GetTestConfig() *Config {
	config := &Config{}
	
	// API настройки
	config.API.BaseURL = "http://localhost:8080"
	config.API.Timeout = 30
	
	// gRPC настройки - отключаем для тестов
	config.GRPC.UseGRPC = false
	config.GRPC.SchedulerAddress = "localhost:50051"
	config.GRPC.CoreAddress = "localhost:50052"
	config.GRPC.AuthAddress = "localhost:50053"
	config.GRPC.Timeout = 30
	
	// Аутентификация
	config.Auth.TokenExpiry = 3600
	config.Auth.RefreshThreshold = 300
	config.Auth.AccessSecret = "test-access-secret"
	config.Auth.RefreshSecret = "test-refresh-secret"
	
	// Настройки вывода
	config.Output.Format = "table"
	config.Output.Colors = true
	
	// Текущий тенант
	config.CurrentTenant = "test-tenant"
	
	return config
}

// LoadTestConfig загружает тестовую конфигурацию
func LoadTestConfig() (*Config, error) {
	// Проверяем наличие тестового конфигурационного файла
	configPath := "config.yaml"
	if _, err := os.Stat(configPath); err == nil {
		return LoadConfig(configPath)
	}
	
	// Если файла нет, используем тестовую конфигурацию по умолчанию
	config := GetTestConfig()
	
	// Загружаем переменные окружения (имеют приоритет)
	loadConfigFromEnv(config)
	
	return config, nil
}
