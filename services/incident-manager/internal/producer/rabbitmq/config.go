package rabbitmq

import (
	"time"

	"UptimePingPlatform/pkg/config"
)

// IncidentProducerConfig конфигурация для producer событий инцидентов
type IncidentProducerConfig struct {
	// RabbitMQ connection settings
	URL        string        `json:"url" yaml:"url"`
	Exchange   string        `json:"exchange" yaml:"exchange"`
	
	// Retry settings
	MaxRetries        int           `json:"max_retries" yaml:"max_retries"`
	InitialDelay      time.Duration `json:"initial_delay" yaml:"initial_delay"`
	MaxDelay          time.Duration `json:"max_delay" yaml:"max_delay"`
	Multiplier        float64       `json:"multiplier" yaml:"multiplier"`
	
	// QoS settings
	PrefetchCount int `json:"prefetch_count" yaml:"prefetch_count"`
	PrefetchSize  int `json:"prefetch_size" yaml:"prefetch_size"`
	Global        bool `json:"global" yaml:"global"`
	
	// Connection settings
	ReconnectInterval time.Duration `json:"reconnect_interval" yaml:"reconnect_interval"`
	Heartbeat         time.Duration `json:"heartbeat" yaml:"heartbeat"`
	Timeout           time.Duration `json:"timeout" yaml:"timeout"`
}

// DefaultIncidentProducerConfig возвращает конфигурацию по умолчанию
func DefaultIncidentProducerConfig() *IncidentProducerConfig {
	return &IncidentProducerConfig{
		URL:        "amqp://guest:guest@localhost:5672/",
		Exchange:   "incident.events",
		
		MaxRetries:        3,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          5 * time.Second,
		Multiplier:        2.0,
		
		PrefetchCount: 10,
		PrefetchSize:  0,
		Global:        false,
		
		ReconnectInterval: 5 * time.Second,
		Heartbeat:         30 * time.Second,
		Timeout:           10 * time.Second,
	}
}

// LoadIncidentProducerConfig загружает конфигурацию из pkg/config
func LoadIncidentProducerConfig(cfg *config.Config) *IncidentProducerConfig {
	defaultConfig := DefaultIncidentProducerConfig()
	
	// Используем поля из pkg/config если они есть
	// В pkg/config нет поля RabbitMQ, поэтому используем переменные окружения или значения по умолчанию
	
	return defaultConfig
}

// Validate валидирует конфигурацию
func (c *IncidentProducerConfig) Validate() error {
	if c.URL == "" {
		return ErrInvalidURL
	}
	
	if c.Exchange == "" {
		return ErrInvalidExchange
	}
	
	if c.MaxRetries < 0 {
		return ErrInvalidMaxRetries
	}
	
	if c.InitialDelay < 0 {
		return ErrInvalidInitialDelay
	}
	
	if c.MaxDelay < 0 {
		return ErrInvalidMaxDelay
	}
	
	if c.Multiplier <= 0 {
		return ErrInvalidMultiplier
	}
	
	return nil
}
