package mocks

import (
	"context"
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/go-redis/redis/v8"
	
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/database"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/metrics"
)

// MockDatabase имитирует pkg/database.Postgres
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) Connect(ctx context.Context, config *database.Config) (*database.Postgres, error) {
	args := m.Called(ctx, config)
	return args.Get(0).(*database.Postgres), args.Error(1)
}

func (m *MockDatabase) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabase) HealthCheck() error {
	args := m.Called()
	return args.Error(0)
}

// MockRedis имитирует pkg/redis.Client
type MockRedis struct {
	mock.Mock
}

func (m *MockRedis) Connect(ctx context.Context, config *pkg_redis.Config) (*redis.Client, error) {
	args := m.Called(ctx, config)
	return args.Get(0).(*redis.Client), args.Error(1)
}

func (m *MockRedis) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRedis) HealthCheck() error {
	args := m.Called()
	return args.Error(0)
}

// MockRabbitMQ имитирует pkg/rabbitmq.Connection
type MockRabbitMQ struct {
	mock.Mock
}

func (m *MockRabbitMQ) Connect(ctx context.Context, config *rabbitmq.Config) (*rabbitmq.Connection, error) {
	args := m.Called(ctx, config)
	return args.Get(0).(*rabbitmq.Connection), args.Error(1)
}

func (m *MockRabbitMQ) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockRateLimiter имитирует pkg/ratelimit.RateLimiter
type MockRateLimiter struct {
	mock.Mock
}

func (m *MockRateLimiter) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	args := m.Called(ctx, key, limit, window)
	return args.Bool(0), args.Error(1)
}

// MockConnecter имитирует pkg/connection.Connecter
type MockConnecter struct {
	mock.Mock
}

func (m *MockConnecter) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockConnecter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConnecter) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockRetryConfig имитирует pkg/connection.RetryConfig
type MockRetryConfig struct {
	mock.Mock
}

func (m *MockRetryConfig) GetMaxAttempts() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockRetryConfig) GetInitialDelay() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *MockRetryConfig) GetMaxDelay() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *MockRetryConfig) GetMultiplier() float64 {
	args := m.Called()
	return float64(args.Get(0).(float64))
}

func (m *MockRetryConfig) GetJitter() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockLogger имитирует pkg/logger.Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	args := m.Called(fields)
	return args.Get(0).(logger.Logger)
}

func (m *MockLogger) Sync() error {
	args := m.Called()
	return args.Error(0)
}

// MockHealthChecker имитирует pkg/health.HealthChecker
type MockHealthChecker struct {
	mock.Mock
}

func (m *MockHealthChecker) Check() *health.HealthStatus {
	args := m.Called()
	return args.Get(0).(*health.HealthStatus)
}

// MockMetrics имитирует pkg/metrics.Metrics
type MockMetrics struct {
	mock.Mock
}

func (m *MockMetrics) NewMetrics(serviceName string) *metrics.Metrics {
	args := m.Called(serviceName)
	return args.Get(0).(*metrics.Metrics)
}

func (m *MockMetrics) GetHandler() http.Handler {
	args := m.Called()
	return args.Get(0).(http.Handler)
}

// MockValidator имитирует pkg/validation.Validator
type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) ValidateRequiredFields(fields map[string]interface{}) error {
	args := m.Called(fields)
	return args.Error(0)
}

func (m *MockValidator) ValidateURL(url string) error {
	args := m.Called(url)
	return args.Error(0)
}

func (m *MockValidator) ValidateHostPort(hostPort string) error {
	args := m.Called(hostPort)
	return args.Error(0)
}

func (m *MockValidator) ValidateTimeout(timeout interface{}) error {
	args := m.Called(timeout)
	return args.Error(0)
}

func (m *MockValidator) ValidateStringLength(value string, min, max int) error {
	args := m.Called(value, min, max)
	return args.Error(0)
}

func (m *MockValidator) ValidateEnum(value string, allowed []string) error {
	args := m.Called(value, allowed)
	return args.Error(0)
}

func (m *MockValidator) ValidateUUID(uuid string) error {
	args := m.Called(uuid)
	return args.Error(0)
}

func (m *MockValidator) ValidateTimestamp(timestamp interface{}) error {
	args := m.Called(timestamp)
	return args.Error(0)
}

// MockBaseHandler имитирует pkg/grpc.BaseHandler
type MockBaseHandler struct {
	mock.Mock
}

func (m *MockBaseHandler) ValidateRequiredFields(ctx context.Context, fields map[string]interface{}) error {
	args := m.Called(ctx, fields)
	return args.Error(0)
}

func (m *MockBaseHandler) LogOperationStart(ctx context.Context, operation string, metadata map[string]interface{}) {
	m.Called(ctx, operation, metadata)
}

func (m *MockBaseHandler) LogOperationSuccess(ctx context.Context, operation string, metadata map[string]interface{}) {
	m.Called(ctx, operation, metadata)
}

func (m *MockBaseHandler) LogError(ctx context.Context, operation string, err error) {
	m.Called(ctx, operation, err)
}
