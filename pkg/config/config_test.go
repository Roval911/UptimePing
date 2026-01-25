package config

import (
	"os"
	"testing"
)

// TestLoadConfig_DefaultValues проверяет загрузку значений по умолчанию
func TestLoadConfig_DefaultValues(t *testing.T) {
	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check default values
	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected server host to be \"0.0.0.0\", got %s", config.Server.Host)
	}
	if config.Server.Port != 8080 {
		t.Errorf("Expected server port to be 8080, got %d", config.Server.Port)
	}
	if config.Database.Host != "localhost" {
		t.Errorf("Expected database host to be \"localhost\", got %s", config.Database.Host)
	}
	if config.Database.Port != 5432 {
		t.Errorf("Expected database port to be 5432, got %d", config.Database.Port)
	}
	if config.Logger.Level != "info" {
		t.Errorf("Expected logger level to be \"info\", got %s", config.Logger.Level)
	}
	if config.Logger.Format != "json" {
		t.Errorf("Expected logger format to be \"json\", got %s", config.Logger.Format)
	}
	if config.Environment != "dev" {
		t.Errorf("Expected environment to be \"dev\", got %s", config.Environment)
	}
}

// TestLoadConfig_FileOverride проверяет возможность переопределения значений по умолчанию значениями из файла конфигурации
func TestLoadConfig_FileOverride(t *testing.T) {
	// Create a temporary config file
	tempFile := "/tmp/test_config.yaml"
	configContent := `server:
  host: "127.0.0.1"
  port: 9090
database:
  host: "prod-db"
  port: 5433
  name: "myapp"
  user: "myuser"
  password: "mypass"
logger:
  level: "debug"
  format: "text"
environment: "prod"
`

	err := os.WriteFile(tempFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tempFile)

	// Load config from file
	config, err := LoadConfig(tempFile)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check that file values override defaults
	if config.Server.Host != "127.0.0.1" {
		t.Errorf("Expected server host to be \"127.0.0.1\", got %s", config.Server.Host)
	}
	if config.Server.Port != 9090 {
		t.Errorf("Expected server port to be 9090, got %d", config.Server.Port)
	}
	if config.Database.Host != "prod-db" {
		t.Errorf("Expected database host to be \"prod-db\", got %s", config.Database.Host)
	}
	if config.Database.Port != 5433 {
		t.Errorf("Expected database port to be 5433, got %d", config.Database.Port)
	}
	if config.Database.Name != "myapp" {
		t.Errorf("Expected database name to be \"myapp\", got %s", config.Database.Name)
	}
	if config.Database.User != "myuser" {
		t.Errorf("Expected database user to be \"myuser\", got %s", config.Database.User)
	}
	if config.Database.Password != "mypass" {
		t.Errorf("Expected database password to be \"mypass\", got %s", config.Database.Password)
	}
	if config.Logger.Level != "debug" {
		t.Errorf("Expected logger level to be \"debug\", got %s", config.Logger.Level)
	}
	if config.Logger.Format != "text" {
		t.Errorf("Expected logger format to be \"text\", got %s", config.Logger.Format)
	}
	if config.Environment != "prod" {
		t.Errorf("Expected environment to be \"prod\", got %s", config.Environment)
	}
}

// TestLoadConfig_EnvironmentOverride проверяет возможность переопределения значений переменными окружения
func TestLoadConfig_EnvironmentOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("SERVER_HOST", "192.168.1.1")
	os.Setenv("SERVER_PORT", "7070")
	os.Setenv("DATABASE_HOST", "env-db")
	os.Setenv("DATABASE_PORT", "5434")
	os.Setenv("DATABASE_NAME", "envapp")
	os.Setenv("DATABASE_USER", "envuser")
	os.Setenv("DATABASE_PASSWORD", "envpass")
	os.Setenv("LOGGER_LEVEL", "warn")
	os.Setenv("LOGGER_FORMAT", "console")
	os.Setenv("ENVIRONMENT", "staging")
	defer func() {
		// Clean up environment variables
		os.Unsetenv("SERVER_HOST")
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("DATABASE_HOST")
		os.Unsetenv("DATABASE_PORT")
		os.Unsetenv("DATABASE_NAME")
		os.Unsetenv("DATABASE_USER")
		os.Unsetenv("DATABASE_PASSWORD")
		os.Unsetenv("LOGGER_LEVEL")
		os.Unsetenv("LOGGER_FORMAT")
		os.Unsetenv("ENVIRONMENT")
	}()

	// Load config with no file (only defaults and env)
	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check that environment variables override defaults
	if config.Server.Host != "192.168.1.1" {
		t.Errorf("Expected server host to be \"192.168.1.1\", got %s", config.Server.Host)
	}
	if config.Server.Port != 7070 {
		t.Errorf("Expected server port to be 7070, got %d", config.Server.Port)
	}
	if config.Database.Host != "env-db" {
		t.Errorf("Expected database host to be \"env-db\", got %s", config.Database.Host)
	}
	if config.Database.Port != 5434 {
		t.Errorf("Expected database port to be 5434, got %d", config.Database.Port)
	}
	if config.Database.Name != "envapp" {
		t.Errorf("Expected database name to be \"envapp\", got %s", config.Database.Name)
	}
	if config.Database.User != "envuser" {
		t.Errorf("Expected database user to be \"envuser\", got %s", config.Database.User)
	}
	if config.Database.Password != "envpass" {
		t.Errorf("Expected database password to be \"envpass\", got %s", config.Database.Password)
	}
	if config.Logger.Level != "warn" {
		t.Errorf("Expected logger level to be \"warn\", got %s", config.Logger.Level)
	}
	if config.Logger.Format != "console" {
		t.Errorf("Expected logger format to be \"console\", got %s", config.Logger.Format)
	}
	if config.Environment != "staging" {
		t.Errorf("Expected environment to be \"staging\", got %s", config.Environment)
	}
}

// TestLoadConfig_Validation проверяет валидацию конфигурации на различных некорректных значениях
func TestLoadConfig_Validation(t *testing.T) {
	// Test invalid environment
	invalidConfig := &Config{
		Environment: "invalid",
	}
	if err := validateConfig(invalidConfig); err == nil {
		t.Error("Expected error for invalid environment, got nil")
	}

	// Test invalid server port
	invalidConfig = &Config{
		Environment: "dev",
		Server: ServerConfig{
			Port: 70000,
		},
	}
	if err := validateConfig(invalidConfig); err == nil {
		t.Error("Expected error for invalid server port, got nil")
	}

	// Test missing required fields
	invalidConfig = &Config{
		Environment: "dev",
	}
	// Missing server.host
	if err := validateConfig(invalidConfig); err == nil {
		t.Error("Expected error for missing server.host, got nil")
	}

	// Fix server.host, now missing database.host
	invalidConfig.Server.Host = "localhost"
	if err := validateConfig(invalidConfig); err == nil {
		t.Error("Expected error for missing database.host, got nil")
	}
}

// TestLoadConfig_FileDoesNotExist проверяет обработку ситуации, когда файл конфигурации не существует
func TestLoadConfig_FileDoesNotExist(t *testing.T) {
	_, err := LoadConfig("/non/existent/config.yaml")
	if err == nil {
		t.Fatal("Expected error for non-existent config file, got nil")
	}
	if err.Error() != "failed to load config from file: config file does not exist: /non/existent/config.yaml" {
		t.Errorf("Expected file not exist error, got %v", err)
	}
}

// TestLoadConfig_InvalidFileFormat проверяет обработку некорректного формата файла конфигурации
func TestLoadConfig_InvalidFileFormat(t *testing.T) {
	// Create a temporary file with invalid format
	tempFile := "/tmp/invalid_config.txt"
	err := os.WriteFile(tempFile, []byte("this is not yaml or json"), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tempFile)

	_, err = LoadConfig(tempFile)
	if err == nil {
		t.Fatal("Expected error for invalid config file format, got nil")
	}
}

// TestConfig_Save проверяет возможность сохранения конфигурации в файл
func TestConfig_Save(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Name:     "testdb",
			User:     "testuser",
			Password: "testpass",
		},
		Logger: LoggerConfig{
			Level:  "debug",
			Format: "json",
		},
		Environment: "dev",
		AuthService: ServiceConfig{
			Host: "localhost",
			Port: 50051,
		},
		ConfigService: ServiceConfig{
			Host: "localhost",
			Port: 50052,
		},
		CoreService: ServiceConfig{
			Host: "localhost",
			Port: 50053,
		},
		ForgeService: ServiceConfig{
			Host: "localhost",
			Port: 50054,
		},
		IncidentService: ServiceConfig{
			Host: "localhost",
			Port: 50055,
		},
		SchedulerService: ServiceConfig{
			Host: "localhost",
			Port: 50056,
		},
	}

	// Save to temp file
	tempFile := "/tmp/saved_config.yaml"
	defer os.Remove(tempFile)

	if err := config.Save(tempFile); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatalf("Saved config file does not exist: %s", tempFile)
	}

	// Load the saved config and verify it's the same
	savedConfig, err := LoadConfig(tempFile)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if savedConfig.Server.Host != config.Server.Host {
		t.Errorf("Saved config server host mismatch: expected %s, got %s", config.Server.Host, savedConfig.Server.Host)
	}
	if savedConfig.Server.Port != config.Server.Port {
		t.Errorf("Saved config server port mismatch: expected %d, got %d", config.Server.Port, savedConfig.Server.Port)
	}
}
