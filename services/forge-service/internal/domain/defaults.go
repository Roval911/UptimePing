package domain

// NewDefaultInteractiveConfig создает конфигурацию по умолчанию
func NewDefaultInteractiveConfig() *InteractiveConfig {
	return &InteractiveConfig{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Name:     "uptimeping",
			User:     "uptimeping",
			Password: "uptimeping",
		},
		Redis: RedisConfig{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		},
		Telegram: TelegramConfig{
			BotToken: "",
			ChatID:   "",
			Enabled:  false,
		},
		Email: EmailConfig{
			SMTPHost:    "smtp.gmail.com",
			SMTPPort:    587,
			Username:    "",
			Password:    "",
			FromAddress: "noreply@uptimeping.com",
			FromName:    "UptimePing Platform",
			Enabled:     false,
		},
		Logger: LoggerConfig{
			Level:  "info",
			Format: "json",
		},
		Environment: "dev",
		Services:    make(map[string]*ServiceConfig),
	}
}

// NewDefaultServiceConfig создает конфигурацию сервиса по умолчанию
func NewDefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		Host:            "localhost",
		Port:            50051,
		DefaultTimeout:  "30s",
		EnabledMethods:  []string{},
		DisabledMethods: []string{},
	}
}

// NewProductionConfig создает конфигурацию для production
func NewProductionConfig() *InteractiveConfig {
	config := NewDefaultInteractiveConfig()
	config.Environment = "prod"
	config.Logger.Level = "warn"
	config.Logger.Format = "json"
	
	// Production значения для БД
	config.Database.Host = "postgres"
	config.Database.User = "uptimeping"
	config.Database.Password = "" // Должен быть установлен из переменных окружения
	
	// Production значения для Redis
	config.Redis.Addr = "redis:6379"
	
	return config
}

// NewDevelopmentConfig создает конфигурацию для разработки
func NewDevelopmentConfig() *InteractiveConfig {
	config := NewDefaultInteractiveConfig()
	config.Environment = "development"
	config.Logger.Level = "debug"
	config.Logger.Format = "text"
	
	return config
}
