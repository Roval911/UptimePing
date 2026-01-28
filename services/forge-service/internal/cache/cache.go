package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/forge-service/internal/service"
)

// Cache интерфейс для кеширования результатов Forge Service
type Cache interface {
	// GetProtoInfo получает информацию о proto файле из кеша
	GetProtoInfo(ctx context.Context, key string) (*service.ForgeServiceInfo, error)
	
	// SetProtoInfo сохраняет информацию о proto файле в кеш
	SetProtoInfo(ctx context.Context, key string, info *service.ForgeServiceInfo, ttl time.Duration) error
	
	// GetConfig получает конфигурацию из кеша
	GetConfig(ctx context.Context, key string) (*service.CheckConfig, error)
	
	// SetConfig сохраняет конфигурацию в кеш
	SetConfig(ctx context.Context, key string, config *service.CheckConfig, ttl time.Duration) error
	
	// GetCode получает сгенерированный код из кеша
	GetCode(ctx context.Context, key string) (*CodeCache, error)
	
	// SetCode сохраняет сгенерированный код в кеш
	SetCode(ctx context.Context, key string, code *CodeCache, ttl time.Duration) error
	
	// GetTemplates получает шаблоны из кеша
	GetTemplates(ctx context.Context, key string) ([]service.TemplateInfo, error)
	
	// SetTemplates сохраняет шаблоны в кеш
	SetTemplates(ctx context.Context, key string, templates []service.TemplateInfo, ttl time.Duration) error
	
	// Invalidate инвалидирует кеш по ключу
	Invalidate(ctx context.Context, key string) error
	
	// Clear очищает весь кеш
	Clear(ctx context.Context) error
}

// CodeCache представляет кешированный код
type CodeCache struct {
	Code     string `json:"code"`
	Filename string `json:"filename"`
	Language string `json:"language"`
}

// redisCache реализация Cache на основе Redis
type redisCache struct {
	redisClient *redis.Client
	logger      logger.Logger
	prefix      string
}

// NewRedisCache создает новый кеш на основе Redis
func NewRedisCache(redisClient *redis.Client, logger logger.Logger) Cache {
	return &redisCache{
		redisClient: redisClient,
		logger:      logger,
		prefix:      "forge:",
	}
}

// GetProtoInfo получает информацию о proto файле из кеша
func (c *redisCache) GetProtoInfo(ctx context.Context, key string) (*service.ForgeServiceInfo, error) {
	cacheKey := c.prefix + "proto:" + key
	
	data, err := c.redisClient.Client.Get(ctx, cacheKey).Result()
	if err != nil {
		c.logger.Error("Failed to get proto info from cache", 
			logger.String("key", cacheKey),
			logger.Error(err))
		return nil, err
	}

	if data == "" {
		return nil, nil // Cache miss
	}

	var info service.ForgeServiceInfo
	if err := json.Unmarshal([]byte(data), &info); err != nil {
		c.logger.Error("Failed to unmarshal proto info from cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return nil, err
	}

	c.logger.Info("Proto info retrieved from cache",
		logger.String("key", cacheKey))

	return &info, nil
}

// SetProtoInfo сохраняет информацию о proto файле в кеш
func (c *redisCache) SetProtoInfo(ctx context.Context, key string, info *service.ForgeServiceInfo, ttl time.Duration) error {
	cacheKey := c.prefix + "proto:" + key
	
	data, err := json.Marshal(info)
	if err != nil {
		c.logger.Error("Failed to marshal proto info for cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return err
	}

	err = c.redisClient.Client.Set(ctx, cacheKey, string(data), ttl).Err()
	if err != nil {
		c.logger.Error("Failed to set proto info in cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return err
	}

	c.logger.Info("Proto info saved to cache",
		logger.String("key", cacheKey),
		logger.Duration("ttl", ttl))

	return nil
}

// GetConfig получает конфигурацию из кеша
func (c *redisCache) GetConfig(ctx context.Context, key string) (*service.CheckConfig, error) {
	cacheKey := c.prefix + "config:" + key
	
	data, err := c.redisClient.Client.Get(ctx, cacheKey).Result()
	if err != nil {
		c.logger.Error("Failed to get config from cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return nil, err
	}

	if data == "" {
		return nil, nil // Cache miss
	}

	var config service.CheckConfig
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		c.logger.Error("Failed to unmarshal config from cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return nil, err
	}

	c.logger.Info("Config retrieved from cache",
		logger.String("key", cacheKey))

	return &config, nil
}

// SetConfig сохраняет конфигурацию в кеш
func (c *redisCache) SetConfig(ctx context.Context, key string, config *service.CheckConfig, ttl time.Duration) error {
	cacheKey := c.prefix + "config:" + key
	
	data, err := json.Marshal(config)
	if err != nil {
		c.logger.Error("Failed to marshal config for cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return err
	}

	err = c.redisClient.Client.Set(ctx, cacheKey, string(data), ttl).Err()
	if err != nil {
		c.logger.Error("Failed to set config in cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return err
	}

	c.logger.Info("Config saved to cache",
		logger.String("key", cacheKey),
		logger.Duration("ttl", ttl))

	return nil
}

// GetCode получает сгенерированный код из кеша
func (c *redisCache) GetCode(ctx context.Context, key string) (*CodeCache, error) {
	cacheKey := c.prefix + "code:" + key
	
	data, err := c.redisClient.Client.Get(ctx, cacheKey).Result()
	if err != nil {
		c.logger.Error("Failed to get code from cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return nil, err
	}

	if data == "" {
		return nil, nil // Cache miss
	}

	var codeCache CodeCache
	if err := json.Unmarshal([]byte(data), &codeCache); err != nil {
		c.logger.Error("Failed to unmarshal code from cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return nil, err
	}

	c.logger.Info("Code retrieved from cache",
		logger.String("key", cacheKey))

	return &codeCache, nil
}

// SetCode сохраняет сгенерированный код в кеш
func (c *redisCache) SetCode(ctx context.Context, key string, code *CodeCache, ttl time.Duration) error {
	cacheKey := c.prefix + "code:" + key
	
	data, err := json.Marshal(code)
	if err != nil {
		c.logger.Error("Failed to marshal code for cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return err
	}

	err = c.redisClient.Client.Set(ctx, cacheKey, string(data), ttl).Err()
	if err != nil {
		c.logger.Error("Failed to set code in cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return err
	}

	c.logger.Info("Code saved to cache",
		logger.String("key", cacheKey),
		logger.Duration("ttl", ttl))

	return nil
}

// GetTemplates получает шаблоны из кеша
func (c *redisCache) GetTemplates(ctx context.Context, key string) ([]service.TemplateInfo, error) {
	cacheKey := c.prefix + "templates:" + key
	
	data, err := c.redisClient.Client.Get(ctx, cacheKey).Result()
	if err != nil {
		c.logger.Error("Failed to get templates from cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return nil, err
	}

	if data == "" {
		return nil, nil // Cache miss
	}

	var templates []service.TemplateInfo
	if err := json.Unmarshal([]byte(data), &templates); err != nil {
		c.logger.Error("Failed to unmarshal templates from cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return nil, err
	}

	c.logger.Info("Templates retrieved from cache",
		logger.String("key", cacheKey))

	return templates, nil
}

// SetTemplates сохраняет шаблоны в кеш
func (c *redisCache) SetTemplates(ctx context.Context, key string, templates []service.TemplateInfo, ttl time.Duration) error {
	cacheKey := c.prefix + "templates:" + key
	
	data, err := json.Marshal(templates)
	if err != nil {
		c.logger.Error("Failed to marshal templates for cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return err
	}

	err = c.redisClient.Client.Set(ctx, cacheKey, string(data), ttl).Err()
	if err != nil {
		c.logger.Error("Failed to set templates in cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return err
	}

	c.logger.Info("Templates saved to cache",
		logger.String("key", cacheKey),
		logger.Duration("ttl", ttl))

	return nil
}

// Invalidate инвалидирует кеш по ключу
func (c *redisCache) Invalidate(ctx context.Context, key string) error {
	cacheKey := c.prefix + key
	
	err := c.redisClient.Client.Del(ctx, cacheKey).Err()
	if err != nil {
		c.logger.Error("Failed to invalidate cache",
			logger.String("key", cacheKey),
			logger.Error(err))
		return err
	}

	c.logger.Info("Cache invalidated",
		logger.String("key", cacheKey))

	return nil
}

// Clear очищает весь кеш
func (c *redisCache) Clear(ctx context.Context) error {
	pattern := c.prefix + "*"
	
	keys, err := c.redisClient.Client.Keys(ctx, pattern).Result()
	if err != nil {
		c.logger.Error("Failed to get keys for cache clearing",
			logger.String("pattern", pattern),
			logger.Error(err))
		return err
	}
	
	if len(keys) == 0 {
		return nil
	}
	
	err = c.redisClient.Client.Del(ctx, keys...).Err()
	if err != nil {
		c.logger.Error("Failed to clear cache",
			logger.String("pattern", pattern),
			logger.Error(err))
		return err
	}

	c.logger.Info("Cache cleared",
		logger.String("pattern", pattern),
		logger.Int("keys_cleared", len(keys)))

	return nil
}

// GenerateCacheKey генерирует ключ для кеша на основе параметров
func GenerateCacheKey(params ...string) string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// DefaultTTL значения по умолчанию для TTL
const (
	DefaultProtoInfoTTL = 1 * time.Hour
	DefaultConfigTTL    = 30 * time.Minute
	DefaultCodeTTL      = 2 * time.Hour
	DefaultTemplatesTTL = 4 * time.Hour
)
